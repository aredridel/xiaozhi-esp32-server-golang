package zhipu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/data/audio"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	sse "github.com/tmaxmax/go-sse"
)

// global HTTP client, implement connection pool
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

const (
	zhipuDefaultSampleRate = 24000
	zhipuLeadingFadeInMs   = 5
)

// get config connection pool of HTTP client
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		}
	})
	return httpClient
}

// ZhipuTTSProvider Zhipu TTS provider
type ZhipuTTSProvider struct {
	APIKey         string
	APIURL         string
	Model          string
	Voice          string
	ResponseFormat string
	Speed          float64
	Volume         float64
	Stream         bool
	EncodeFormat   string // only used when streaming: base64 or hex
	FrameDuration  int
}

// request structure body (according to Zhipu API documentation)
type zhipuRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Volume         float64 `json:"volume,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
	EncodeFormat   string  `json:"encode_format,omitempty"` // onlystreamingwhenuse：base64 or hex
}

// Event Stream response structure body (similar to OpenAI format)
type zhipuEventStreamResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason,omitempty"`
		Delta        struct {
			Role             string `json:"role,omitempty"`
			Content          string `json:"content,omitempty"` // base64 encodeofaudio data
			ReturnSampleRate int    `json:"return_sample_rate,omitempty"`
			ReturnFormat     string `json:"return_format,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
}

// NewZhipuTTSProvider create new Zhipu TTS provider
func NewZhipuTTSProvider(config map[string]interface{}) *ZhipuTTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	responseFormat, _ := config["response_format"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["volume"].(float64)
	stream, _ := config["stream"].(bool)
	encodeFormat, _ := config["encode_format"].(string)
	frameDuration, _ := config["frame_duration"].(float64)

	// set default values
	if apiURL == "" {
		apiURL = "https://open.bigmodel.cn/api/paas/v4/audio/speech"
	}
	if model == "" {
		model = "glm-tts"
	}
	if voice == "" {
		voice = "tongtong" // default voice
	}
	if responseFormat == "" {
		responseFormat = "pcm" // Zhipu default pcm, also supports wav
	}
	if speed == 0 {
		speed = 1.0 // 0.5 to 2.0
	}
	if volume == 0 {
		volume = 1.0 // 0 to 10
	}
	if encodeFormat == "" {
		encodeFormat = "base64" // default base64, also supports hex
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &ZhipuTTSProvider{
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		Voice:          voice,
		ResponseFormat: responseFormat,
		Stream:         stream,
		Speed:          speed,
		Volume:         volume,
		EncodeFormat:   encodeFormat,
		FrameDuration:  int(frameDuration),
	}
}

// TextToSpeech convert text to voice, return audio frame data and error
func (p *ZhipuTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()

	// limit text length (Zhipu API maximum 1024 char)
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("text length exceeds 1024 char, already truncated")
	}

	// create request body
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: p.ResponseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         false, // non-streaming
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serialize request failed: %v", err)
	}

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// set request header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// use connection pool send request
	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	// check response status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	// check response content length
	contentLength := resp.ContentLength
	log.Debugf("receive Zhipu TTS response, Content-Length: %d", contentLength)

	// judge Content-Length whether reasonable
	if contentLength == 0 {
		log.Errorf("API returned empty response, Content-Length is 0")
		return nil, fmt.Errorf("API returned empty response, Content-Length is 0")
	}

	// according to audio format process response (Zhipu only supports wav and pcm)
	if p.ResponseFormat == "wav" || p.ResponseFormat == "pcm" {
		audioReader := io.ReadCloser(resp.Body)
		if strings.EqualFold(p.ResponseFormat, "pcm") {
			pcmData, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read Zhipu PCM data failed: %v", err)
			}
			audioReader = io.NopCloser(bytes.NewReader(
				applyPCM16MonoLeadingFadeIn(pcmData, leadingFadeInSampleCount(zhipuDefaultSampleRate, zhipuLeadingFadeInMs)),
			))
		}

		// create a channel to collect audio frame
		outputChan := make(chan []byte, 1000)

		// create audio decoder
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, audioReader, outputChan, frameDuration, p.ResponseFormat, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("create audio decoder failed: %v", err)
		}
		if strings.EqualFold(p.ResponseFormat, "pcm") {
			decoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(zhipuDefaultSampleRate),
				NumChannels: 1,
			})
		}

		// start decode process
		go func() {
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("audio decode failed: %v", err)
			}
		}()

		// collect all audio frames
		var audioFrames [][]byte
		for frame := range outputChan {
			audioFrames = append(audioFrames, frame)
		}

		log.Debugf("Zhipu TTS complete, from input to get audio data end time consumption: %d ms", time.Now().UnixMilli()-startTs)
		return audioFrames, nil
	}

	return nil, fmt.Errorf("unsupported audio format: %s, Zhipu only supports wav and pcm", p.ResponseFormat)
}

// TextToSpeechStream streaming voice synthesis implementation
func (p *ZhipuTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// limit text length (Zhipu API maximum 1024 char)
	if len(text) > 1024 {
		text = text[:1024]
		log.Warnf("text length exceeds 1024 char, already truncated")
	}

	// streaming only supports pcm and wav format
	responseFormat := p.ResponseFormat

	// create request body
	reqBody := zhipuRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: responseFormat,
		Speed:          p.Speed,
		Volume:         p.Volume,
		Stream:         true,           // streaming
		EncodeFormat:   p.EncodeFormat, // use config of encode format
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serialize request failed: %v", err)
	}

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// set request header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// use connection pool create client
	client := getHTTPClient()

	// create output channel
	outputChan = make(chan []byte, 100)

	// start goroutine process streaming response
	go func() {
		// send request
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("send Zhipu request failed: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		// check response status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Zhipu API request failed, status code: %d, response: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// check Content-Type whether is Event Stream
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			log.Warnf("Zhipu API returned Content-Type is not text/event-stream: %s", contentType)
		}

		// streaming only supports pcm and wav format
		//log.Debugf("Zhipu TTS streaming responseFormat(request): %s", responseFormat)
		if responseFormat == "pcm" || responseFormat == "wav" {
			// create pipe, used for passing decoded binary data to audio decoder
			pipeReader, pipeWriter := io.Pipe()

			// start goroutine parse Event Stream and decode
			go func() {
				defer func() {
					if err := pipeWriter.Close(); err != nil {
						log.Debugf("close pipe write endpoint failed: %v", err)
					}
				}()

				// call independent parse method
				if err := p.parseEventStream(ctx, resp.Body, pipeWriter, text); err != nil {
					log.Errorf("parse Event Stream failed: %v", err)
				}
			}()

			// create audio decoder, read decoded binary data from pipe
			decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, responseFormat, sampleRate)
			if err != nil {
				log.Errorf("create Zhipu audio decoder failed: %v", err)
				pipeReader.Close()
				close(outputChan)
				return
			}
			if strings.EqualFold(responseFormat, "pcm") {
				decoder.WithFormat(beep.Format{
					SampleRate:  beep.SampleRate(zhipuDefaultSampleRate),
					NumChannels: 1,
				})
			}

			// start decode process
			if err := decoder.Run(startTs); err != nil {
				log.Errorf("Zhipu audio decode failed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("Zhipu TTS streaming synthesis cancel, text: %s", text)
				return
			default:
				log.Debugf("Zhipu TTS time consumption: from input to get audio data end time consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("Zhipu streaming output only supports pcm format")
			close(outputChan)
		}
	}()

	return outputChan, nil
}

// parseEventStream use go-sse parse Zhipu Event Stream response, decode data and write pipe
// ctx: context, used for cancel operation
// reader: response body reader
// writer: pipe write endpoint, used for output decoded binary data
// text: original text, used for log record
func (p *ZhipuTTSProvider) parseEventStream(ctx context.Context, reader io.Reader, writer *io.PipeWriter, text string) error {
	// config go-sse of ReadConfig, set larger MaxEventSize to process long token
	// Zhipu TTS returned base64 encoded audio data may exceed default 64KB limit
	readConfig := &sse.ReadConfig{
		MaxEventSize: 4 * 1024 * 1024, // 4MB, enough to process large base64 encoded audio data
	}
	fadeTotalSamples := 0
	fadeSamplesRemaining := -1

	for ev, evErr := range sse.Read(reader, readConfig) {
		if evErr != nil {
			return fmt.Errorf("read Zhipu SSE event failed: %w", evErr)
		}

		select {
		case <-ctx.Done():
			log.Debugf("Zhipu TTS streaming synthesis cancel, text: %s", text)
			return ctx.Err()
		default:
		}

		// Event Stream format:
		// data: {"id":"...","choices":[{"delta":{"content":"base64_data"}}]}
		// data: {"choices":[{"finish_reason":"stop"}]}

		dataValue := strings.TrimSpace(ev.Data)
		if dataValue == "" {
			continue
		}

		// parse JSON
		var eventResp zhipuEventStreamResponse
		if err := json.Unmarshal([]byte(dataValue), &eventResp); err != nil {
			log.Warnf("parse Zhipu Event Stream JSON failed: %v, data: %s", err, previewString(dataValue, 200))
			continue
		}

		// check if have finish_reason, indicate stream end
		for _, choice := range eventResp.Choices {
			if choice.FinishReason == "stop" {
				log.Debugf("receive finish_reason: stop, Event Stream end")
				return nil
			}
		}

		// extract each choice of content field and independent process
		for _, choice := range eventResp.Choices {
			if choice.Delta.Content != "" {
				decodedData, err := p.decodeAudioContent(choice.Delta.Content)
				if err != nil {
					return fmt.Errorf("process content failed: %v", err)
				}

				returnFormat := strings.TrimSpace(choice.Delta.ReturnFormat)
				if returnFormat == "" {
					returnFormat = p.ResponseFormat
				}
				if strings.EqualFold(returnFormat, "pcm") {
					if fadeSamplesRemaining < 0 {
						sampleRate := choice.Delta.ReturnSampleRate
						if sampleRate < 1 {
							sampleRate = zhipuDefaultSampleRate
						}
						fadeTotalSamples = leadingFadeInSampleCount(sampleRate, zhipuLeadingFadeInMs)
						fadeSamplesRemaining = fadeTotalSamples
					}
					applyPCM16MonoLeadingFadeInInPlace(decodedData, fadeTotalSamples, &fadeSamplesRemaining)
				}

				if len(decodedData) > 0 {
					if _, err := writer.Write(decodedData); err != nil {
						return fmt.Errorf("write pipe failed: %v", err)
					}
				}
			}
		}
	}

	return nil
}

// previewString return string of first n chars used for log
func previewString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// decodeAudioContent decode single content field
// content: base64 or hex encoded audio data string
func (p *ZhipuTTSProvider) decodeAudioContent(content string) ([]byte, error) {
	if content == "" {
		return nil, nil
	}

	// according to encode_format decode
	var decodedData []byte
	var decodeErr error

	switch p.EncodeFormat {
	case "base64":
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	case "hex":
		decodedData, decodeErr = hex.DecodeString(content)
	default:
		log.Warnf("unknown encode format: %s, use base64", p.EncodeFormat)
		decodedData, decodeErr = base64.StdEncoding.DecodeString(content)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("decode audio data failed: %v, data length: %d", decodeErr, len(content))
	}

	return decodedData, nil
}

func leadingFadeInSampleCount(sampleRate int, fadeMs int) int {
	if sampleRate < 1 {
		sampleRate = zhipuDefaultSampleRate
	}
	if fadeMs < 1 {
		return 0
	}
	samples := sampleRate * fadeMs / 1000
	if samples < 1 {
		return 1
	}
	return samples
}

func applyPCM16MonoLeadingFadeIn(data []byte, remainingSamples int) []byte {
	if len(data) == 0 || remainingSamples <= 0 {
		return data
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	applyPCM16MonoLeadingFadeInInPlace(cloned, remainingSamples, &remainingSamples)
	return cloned
}

func applyPCM16MonoLeadingFadeInInPlace(data []byte, totalSamples int, remainingSamples *int) {
	if len(data) < 2 || totalSamples <= 0 || remainingSamples == nil || *remainingSamples <= 0 {
		return
	}

	samplePairs := len(data) / 2
	for i := 0; i < samplePairs && *remainingSamples > 0; i++ {
		offset := i * 2
		sample := int16(uint16(data[offset]) | uint16(data[offset+1])<<8)
		appliedIndex := totalSamples - *remainingSamples
		scaled := int32(sample) * int32(appliedIndex) / int32(totalSamples)
		binarySample := uint16(int16(scaled))
		data[offset] = byte(binarySample)
		data[offset+1] = byte(binarySample >> 8)
		*remainingSamples = *remainingSamples - 1
	}
}

// SetVoice set voice parameter
func (p *ZhipuTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("invalid voice config: missing voice")
}

// Close close resource (no state Provider, no need close)
func (p *ZhipuTTSProvider) Close() error {
	return nil
}

// IsValid check resource whether valid
func (p *ZhipuTTSProvider) IsValid() bool {
	return p != nil
}
