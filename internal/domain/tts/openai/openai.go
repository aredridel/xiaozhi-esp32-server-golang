package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"

	"xiaozhi-esp32-server-golang/internal/data/audio"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

// globalHTTPclient-side，implementjoinpool
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getconfigjoinpoolofHTTPclient-side
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
			Timeout:   60 * time.Second, // OpenAI TTS mayneed更longtime
		}
	})
	return httpClient
}

// OpenAITTSProvider OpenAI TTSprovide者
type OpenAITTSProvider struct {
	APIKey         string
	APIURL         string
	Model          string
	Voice          string
	ResponseFormat string
	Speed          float64
	Stream         bool
	FrameDuration  int
}

// requeststructurebody
type openAIRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	Stream         bool    `json:"stream,omitempty"`
}

// NewOpenAITTSProvider create newOpenAI TTSprovide者
func NewOpenAITTSProvider(config map[string]interface{}) *OpenAITTSProvider {
	apiKey, _ := config["api_key"].(string)
	apiURL, _ := config["api_url"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	responseFormat, _ := config["response_format"].(string)
	speed, _ := config["speed"].(float64)
	stream, _ := config["stream"].(bool)
	frameDuration, _ := config["frame_duration"].(float64)

	// setdefault values
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/audio/speech"
	}
	if model == "" {
		model = "tts-1" // tts-1 or tts-1-hd
	}
	if voice == "" {
		voice = "alloy" // alloy, echo, fable, onyx, nova, shimmer
	}
	if responseFormat == "" {
		responseFormat = "mp3" // mp3, opus, aac, flac, wav, pcm
	}
	if speed == 0 {
		speed = 1.0 // 0.25 to 4.0
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}

	return &OpenAITTSProvider{
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		Voice:          voice,
		ResponseFormat: responseFormat,
		Stream:         stream,
		Speed:          speed,
		FrameDuration:  int(frameDuration),
	}
}

// TextToSpeech willtextconvertisvoice，returnaudio framedataanderror
func (p *OpenAITTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	streamChan, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}

	audioFrames := make([][]byte, 0, 32)
	for frame := range streamChan {
		audioFrames = append(audioFrames, frame)
	}
	if len(audioFrames) == 0 {
		return nil, fmt.Errorf("OpenAI TTS returnaudioisempty")
	}
	return audioFrames, nil
}

// TextToSpeechStream streamingvoice合成implement
func (p *OpenAITTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// createrequestbody
	reqBody := openAIRequest{
		Model:          p.Model,
		Input:          text,
		Voice:          p.Voice,
		ResponseFormat: p.ResponseFormat,
		Speed:          p.Speed,
		Stream:         p.Stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serializerequestfailed: %v", err)
	}

	//log.Debugf("OpenAI TTSrequest: %s", string(jsonData))

	// createHTTPrequest
	req, err := http.NewRequestWithContext(ctx, "POST", p.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// setrequest header
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))
	}

	// usejoinpoolcreateclient-side
	client := getHTTPClient()

	// createoutputchannel
	outputChan = make(chan []byte, 100)

	// startgoroutineprocessstreamingrespond
	go func() {
		// sendrequest
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("sendOpenAIrequestfailed: %v", err)
			close(outputChan)
			return
		}
		defer resp.Body.Close()

		// inspectrespondstate码
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("OpenAI API request failed，state码: %d, respond: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// inspectrespondinside容length
		contentLength := resp.ContentLength
		log.Debugf("receiveOpenAI TTSrespond，Content-Length: %d", contentLength)

		// judgeContent-Lengthwhether合理
		if contentLength == 0 {
			log.Errorf("OpenAI APIreturnemptyrespond，Content-Lengthis0")
			close(outputChan)
			return
		}

		responseFormat := strings.ToLower(strings.TrimSpace(p.ResponseFormat))
		decoderFormat := responseFormat
		if responseFormat == "opus" {
			decoderFormat = "ogg_opus"
			contentTypeFormat := util.GetAudioFormatByMimeType(strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))))
			if contentTypeFormat == "ogg_opus" || contentTypeFormat == "opus" {
				decoderFormat = contentTypeFormat
			}
		}

		if decoderFormat != "mp3" && decoderFormat != "wav" && decoderFormat != "pcm" && decoderFormat != "opus" && decoderFormat != "ogg_opus" {
			log.Errorf("currentonlysupport mp3/wav/pcm/opus/ogg_opus formatofstreaming合成")
			close(outputChan)
			return
		}

		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, decoderFormat, sampleRate)
		if err != nil {
			log.Errorf("createOpenAIaudiodecode器failed: %v", err)
			close(outputChan)
			return
		}
		if decoderFormat == "opus" {
			sourceChannels := channels
			if sourceChannels < 1 {
				sourceChannels = 1
			}
			decoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(util.NormalizeOpusSampleRate(sampleRate)),
				NumChannels: sourceChannels,
			})
		}

		if err := decoder.Run(startTs); err != nil {
			log.Errorf("OpenAIaudiodecodefailed: %v", err)
			return
		}

		select {
		case <-ctx.Done():
			log.Debugf("OpenAI TTSstreaming合成cancel, text: %s", text)
			return
		default:
			log.Infof("OpenAI TTStime consumption: frominput至getaudio dataendtime consumption: %d ms", time.Now().UnixMilli()-startTs)
		}
	}()

	return outputChan, nil
}

// SetVoice setvoiceparameter
func (p *OpenAITTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if voice, ok := voiceConfig["voice"].(string); ok && voice != "" {
		p.Voice = voice
		return nil
	}
	return fmt.Errorf("invalidofvoiceconfig: Missing voice")
}

// Close closeresource（nostate Provider，noneedclose）
func (p *OpenAITTSProvider) Close() error {
	return nil
}

// IsValid inspectresourcewhethervalid
func (p *OpenAITTSProvider) IsValid() bool {
	return p != nil
}
