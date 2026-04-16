package qwen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
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

const (
	defaultAPIURLBeijing    = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultAPIURLSingapore  = "https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultQwenModel        = "qwen3-tts-flash"
	defaultQwenVoice        = "Cherry"
	defaultQwenLanguageType = "Chinese"
)

// global HTTP client, implement connection pool
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// get config connection pool HTTP client
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

// QwenTTSProvider Alibaba Cloud Qwen TTS provider
type QwenTTSProvider struct {
	APIKey        string
	APIURL        string
	Model         string
	Voice         string
	LanguageType  string
	Stream        bool
	FrameDuration int
}

// qwenRequest request struct
type qwenRequest struct {
	Model string           `json:"model"`
	Input qwenRequestInput `json:"input"`
}

type qwenRequestInput struct {
	Text         string `json:"text"`
	Voice        string `json:"voice"`
	LanguageType string `json:"language_type,omitempty"`
}

// qwenResponse non-streaming/streaming unified response struct
type qwenResponse struct {
	StatusCode int        `json:"status_code"`
	RequestID  string     `json:"request_id"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Output     qwenOutput `json:"output"`
	Usage      qwenUsage  `json:"usage"`
}

type qwenOutput struct {
	Text         interface{}   `json:"text"`
	FinishReason string        `json:"finish_reason"`
	Choices      interface{}   `json:"choices"`
	Audio        qwenAudioInfo `json:"audio"`
}

type qwenAudioInfo struct {
	Data      string `json:"data"`       // streaming output Base64 audio data (16bit PCM)
	URL       string `json:"url"`        // non-streaming output WAV URL
	ID        string `json:"id"`         // audio ID
	ExpiresAt int64  `json:"expires_at"` // URL expiration timestamp
}

type qwenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Characters   int `json:"characters"`
}

// NewQwenTTSProvider create new Alibaba Cloud Qwen TTS provider
func NewQwenTTSProvider(config map[string]interface{}) *QwenTTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	languageType, _ := config["language_type"].(string)
	stream, _ := config["stream"].(bool)
	frameDuration, _ := config["frame_duration"].(float64)
	region, _ := config["region"].(string)

	// process API URL / region
	if apiURL == "" {
		if strings.EqualFold(region, "singapore") {
			apiURL = defaultAPIURLSingapore
		} else {
			apiURL = defaultAPIURLBeijing
		}
	}

	// default values
	if model == "" {
		model = defaultQwenModel
	}
	if voice == "" {
		voice = defaultQwenVoice
	}
	if languageType == "" {
		languageType = defaultQwenLanguageType
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &QwenTTSProvider{
		APIKey:        apiKey,
		APIURL:        apiURL,
		Model:         model,
		Voice:         voice,
		LanguageType:  languageType,
		Stream:        stream,
		FrameDuration: int(frameDuration),
	}
}

// TextToSpeech non-streaming text to speech: call HTTP interface, download WAV and decode to frames
func (p *QwenTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	startTs := time.Now().UnixMilli()

	// construct request body
	reqBody := qwenRequest{
		Model: p.Model,
		Input: qwenRequestInput{
			Text:         text,
			Voice:        p.Voice,
			LanguageType: p.LanguageType,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serialize request failed: %v", err)
	}

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var ttsResp qwenResponse
	if err := json.Unmarshal(body, &ttsResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %v, response body: %s", err, string(body))
	}

	if ttsResp.StatusCode != 200 {
		return nil, fmt.Errorf("Qwen TTS API error [%s]: %s", ttsResp.Code, ttsResp.Message)
	}

	if ttsResp.Output.Audio.URL == "" {
		return nil, fmt.Errorf("response does not include audio URL")
	}

	log.Debugf("Qwen TTS non-streaming, download audio URL: %s", ttsResp.Output.Audio.URL)

	// download WAV, and decode to frames through decoder
	wavReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ttsResp.Output.Audio.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create audio download request failed: %v", err)
	}

	wavResp, err := client.Do(wavReq)
	if err != nil {
		return nil, fmt.Errorf("download audio failed: %v", err)
	}
	defer wavResp.Body.Close()

	if wavResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(wavResp.Body)
		return nil, fmt.Errorf("download audio failed, status code: %d, response: %s", wavResp.StatusCode, string(body))
	}

	outputChan := make(chan []byte, 1000)

	decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, wavResp.Body, outputChan, frameDuration, "wav", sampleRate)
	if err != nil {
		return nil, fmt.Errorf("create Qwen audio decoder failed: %v", err)
	}

	// start decode
	go func() {
		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Qwen TTS non-streaming audio decode failed: %v", err)
		}
	}()

	var frames [][]byte
	for frame := range outputChan {
		frames = append(frames, frame)
	}

	log.Debugf("Qwen TTS non-streaming complete, time consumption from input to get audio data end: %d ms", time.Now().UnixMilli()-startTs)
	return frames, nil
}

// TextToSpeechStream streaming text to speech implementation
func (p *QwenTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {

	startTs := time.Now().UnixMilli()

	// construct request body
	reqBody := qwenRequest{
		Model: p.Model,
		Input: qwenRequestInput{
			Text:         text,
			Voice:        p.Voice,
			LanguageType: p.LanguageType,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serialize request failed: %v", err)
	}

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))
	req.Header.Set("X-DashScope-SSE", "enable") // enable streaming output

	client := getHTTPClient()

	outputChan = make(chan []byte, 100)

	go func() {

		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("send Qwen streaming request failed: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Qwen streaming API request failed, status code: %d, response: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			log.Warnf("Qwen streaming API returned Content-Type is not text/event-stream: %s", contentType)
			close(outputChan)
			return
		}

		// pipe: parse SSE -> PCM -> decode to frames
		pipeReader, pipeWriter := io.Pipe()

		// parse SSE, write original PCM data.
		// Qwen streaming returned audio.data may carry WAV header at times, need to strip first then process as PCM.
		go func() {
			defer func() {
				if err := pipeWriter.Close(); err != nil {
					log.Debugf("close Qwen pipe write endpoint failed: %v", err)
				}
			}()

			if err := p.parseEventStream(ctx, resp.Body, pipeWriter, text); err != nil {
				log.Errorf("parse Qwen Event Stream failed: %v", err)
			}
		}()

		// create audio decoder, read PCM from pipe, output opus frame
		decoder, err := util.CreateAudioDecoderWithSampleRate(
			ctx,
			pipeReader,
			outputChan,
			frameDuration,
			"pcm", // parseEventStream will strip WAV header when needed, output pure 16bit PCM
			sampleRate,
		)
		if err != nil {
			log.Errorf("create Qwen streaming audio decoder failed: %v", err)
			close(outputChan)
			pipeReader.Close()
			return
		}

		// tell decoder PCM sample rate/channel info
		decoder.WithFormat(beep.Format{
			SampleRate:  beep.SampleRate(24000),
			NumChannels: 1,
		})

		// decoder.Run() will close outputChan internally
		// use sync.Once to ensure even if decoder.Run() closes channel, defer won't close repeatedly
		if err := decoder.Run(startTs); err != nil {
			log.Errorf("Qwen streaming audio decode failed: %v", err)
			return
		}

		// if decoder.Run() completes successfully, it will close channel
		// so here need to cancel defer close operation (already processed through sync.Once)

		select {
		case <-ctx.Done():
			log.Debugf("Qwen TTS streaming synthesis cancelled, text: %s", text)
			return
		default:
			log.Debugf("Qwen TTS streaming time consumption: from input to get audio data end: %d ms", time.Now().UnixMilli()-startTs)
		}
	}()

	return outputChan, nil
}

// parseEventStream use go-sse to parse Alibaba Cloud Qwen SSE, decode Base64 PCM and write to pipe
func (p *QwenTTSProvider) parseEventStream(ctx context.Context, reader io.Reader, writer *io.PipeWriter, text string) error {
	var leadingAudio bytes.Buffer
	wroteLeadingAudio := false

	for ev, evErr := range sse.Read(reader, nil) {
		if evErr != nil {
			return fmt.Errorf("read Qwen SSE event failed: %w", evErr)
		}

		select {
		case <-ctx.Done():
			log.Debugf("Qwen TTS streaming synthesis cancelled, text: %s", text)
			return ctx.Err()
		default:
		}

		dataValue := strings.TrimSpace(ev.Data)
		if dataValue == "" {
			continue
		}

		var eventResp qwenResponse
		if err := json.Unmarshal([]byte(dataValue), &eventResp); err != nil {
			log.Warnf("parse Qwen Event Stream JSON failed: %v, data: %s", err, previewString(dataValue, 200))
			continue
		}

		// check business status code (streaming data may not include status_code, when not included is 0, treat as success)
		if eventResp.StatusCode != 0 && eventResp.StatusCode != 200 {
			return fmt.Errorf("Qwen streaming API error [%s]: %s", eventResp.Code, eventResp.Message)
		}

		// decode Base64 PCM data
		if eventResp.Output.Audio.Data != "" {
			encoded := cleanBase64(eventResp.Output.Audio.Data)
			audioBytes, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				log.Errorf("decode Qwen Base64 PCM failed: %v", err)
				continue
			}

			if len(audioBytes) > 0 {
				if !wroteLeadingAudio {
					leadingAudio.Write(audioBytes)
					normalized, needMore, detectedWAV, err := normalizeLeadingQwenAudio(leadingAudio.Bytes())
					if err != nil {
						return fmt.Errorf("parse Qwen streaming audio header failed: %w", err)
					}
					if needMore {
						continue
					}
					wroteLeadingAudio = true
					if detectedWAV {
						log.Infof("Qwen streaming audio detected WAV header, already stripped and processed as PCM")
					}
					if len(normalized) == 0 {
						continue
					}
					if _, err := writer.Write(normalized); err != nil {
						return fmt.Errorf("write PCM to pipe failed: %v", err)
					}
					continue
				}

				if _, err := writer.Write(audioBytes); err != nil {
					return fmt.Errorf("write PCM to pipe failed: %v", err)
				}
			}
		}

		// check if complete
		if eventResp.Output.FinishReason == "stop" {
			log.Debugf("Qwen streaming received finish_reason=stop, request ID: %s", eventResp.RequestID)
			return nil
		}
	}

	return nil
}

func normalizeLeadingQwenAudio(data []byte) (normalized []byte, needMore bool, detectedWAV bool, err error) {
	if len(data) < 12 {
		return nil, true, false, nil
	}

	if !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return data, false, false, nil
	}

	offset, needMore, err := qwenWAVDataOffset(data)
	if err != nil {
		return nil, false, true, err
	}
	if needMore {
		return nil, true, true, nil
	}
	if offset > len(data) {
		return nil, false, true, fmt.Errorf("WAV data offset out of bounds: %d > %d", offset, len(data))
	}
	return data[offset:], false, true, nil
}

func qwenWAVDataOffset(data []byte) (offset int, needMore bool, err error) {
	if len(data) < 12 {
		return 0, true, nil
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return 0, false, fmt.Errorf("not a valid WAV header")
	}

	offset = 12
	for {
		if len(data) < offset+8 {
			return 0, true, nil
		}

		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if chunkSize < 0 {
			return 0, false, fmt.Errorf("illegal WAV chunk size: %d", chunkSize)
		}
		offset += 8

		if chunkID == "data" {
			return offset, false, nil
		}

		nextOffset := offset + chunkSize
		if chunkSize%2 == 1 {
			nextOffset++
		}
		if len(data) < nextOffset {
			return 0, true, nil
		}
		offset = nextOffset
	}
}

// SetVoice set voice
func (p *QwenTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("invalid voice config: Missing voice")
}

// Close close resource (stateless Provider, no need to close)
func (p *QwenTTSProvider) Close() error {
	return nil
}

// IsValid check if resource is valid
func (p *QwenTTSProvider) IsValid() bool {
	return p != nil
}

// cleanBase64 remove all whitespace characters in Base64 string
func cleanBase64(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' {
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// previewString return first n characters of string for log
func previewString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
