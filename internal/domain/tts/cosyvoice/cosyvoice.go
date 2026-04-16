package cosyvoice

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/data/audio"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

// globalHTTPclient-side，implementconnectionpool
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getconfigconnectionpoolofHTTPclient-side
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
			Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// CosyVoiceTTSProvider CosyVoice TTS provider
type CosyVoiceTTSProvider struct {
	APIURL        string
	SpeakerID     string
	FrameDuration int
	TargetSR      int
	AudioFormat   string
	InstructText  string
}

// respondstructurebody
type cosyVoiceResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []byte `json:"data"`
}

// NewCosyVoiceTTSProvider create newCosyVoice TTS provider
func NewCosyVoiceTTSProvider(config map[string]interface{}) *CosyVoiceTTSProvider {
	apiURL, _ := config["api_url"].(string)
	speakerID, _ := config["spk_id"].(string)
	frameDuration, _ := config["frame_duration"].(float64)
	targetSR, _ := config["target_sr"].(float64)
	audioFormat, _ := config["audio_format"].(string)
	instructText, _ := config["instruct_text"].(string)

	// setdefault values
	if apiURL == "" {
		apiURL = "https://tts.linkerai.cn/tts"
	}
	if speakerID == "" {
		speakerID = "OUeAo1mhq6IBExi"
	}
	if frameDuration == 0 {
		frameDuration = audio.FrameDuration
	}
	if targetSR == 0 {
		targetSR = audio.SampleRate
	}
	if audioFormat == "" {
		audioFormat = "mp3"
	}

	return &CosyVoiceTTSProvider{
		APIURL:        apiURL,
		SpeakerID:     speakerID,
		FrameDuration: int(frameDuration),
		TargetSR:      int(targetSR),
		AudioFormat:   audioFormat,
		InstructText:  instructText,
	}
}

// TextToSpeech willtextconverttovoice，returnaudio framedataanderror
func (p *CosyVoiceTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// buildqueryparameter
	params := url.Values{}
	params.Add("tts_text", text)
	params.Add("spk_id", p.SpeakerID)
	params.Add("frame_durition", fmt.Sprintf("%d", p.FrameDuration))
	params.Add("stream", "true") // streamingrequest
	params.Add("target_sr", fmt.Sprintf("%d", p.TargetSR))
	params.Add("audio_format", p.AudioFormat)

	startTs := time.Now().UnixMilli()

	// buildcompleteURL
	requestURL := fmt.Sprintf("%s?%s", p.APIURL, params.Encode())

	// createHTTPrequest
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")

	// useconnectionpoolsendrequest
	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sendrequestfailed: %v", err)
	}
	defer resp.Body.Close()

	// readresponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// inspectresponsestatuscode
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed, statuscode: %d, response: %s", resp.StatusCode, string(body))
	}

	// inspectresponsecontenttypeandcontentlength
	// contentType := resp.Header.Get("Content-Type")
	contentLength := resp.ContentLength

	// recordresponselengthtolog
	log.Debugf("receiveTTSresponse, Content-Length: %d", contentLength)

	// judgeContent-Lengthwhetherreasonable
	if contentLength == 0 {
		log.Errorf("APIreturnemptyresponse, Content-Lengthis0")
		return nil, fmt.Errorf("APIreturnemptyresponse, Content-Lengthis0")
	}

	// MP3fileheaderat leastneed100bytethennormalparse
	// -1indicateunknownlength（forexamplechunkedtransfer）
	if contentLength > 0 && contentLength < 100 {
		log.Errorf("APIreturnofresponsetoosmalltoparseisMP3: %dbyte", contentLength)
		return nil, fmt.Errorf("APIreturnofresponsetoosmalltoparseisMP3: %dbyte", contentLength)
	}

	// converttoOpusframe
	if p.AudioFormat == "mp3" {
		// create apipe
		doneChan := make(chan struct{})
		outputChan := make(chan []byte, 1000)

		// createMP3 decoder
		mp3Decoder, err := util.CreateAudioDecoder(ctx, resp.Body, outputChan, frameDuration, p.AudioFormat)
		if err != nil {
			close(doneChan)
			return nil, fmt.Errorf("createMP3 decoderfailed: %v", err)
		}
		// startdecodeprocess
		go func() {
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3decodefailed: %v", err)
			}
		}()

		// receiveallOpusframe
		var opusFrames [][]byte
		for frame := range outputChan {
			opusFrames = append(opusFrames, frame)
		}

		return opusFrames, nil
	}

	return nil, fmt.Errorf("unsupportedofaudioformat: %s", p.AudioFormat)
}

// TextToSpeechStream streamingvoicesynthesisimplement
func (p *CosyVoiceTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	// buildqueryparameter
	params := url.Values{}
	params.Add("tts_text", text)
	params.Add("spk_id", p.SpeakerID)
	params.Add("frame_durition", fmt.Sprintf("%d", frameDuration))
	params.Add("stream", "true") // streamingrequest
	params.Add("target_sr", fmt.Sprintf("%d", sampleRate))
	params.Add("audio_format", p.AudioFormat)

	startTs := time.Now().UnixMilli()

	// buildcompleteURL
	requestURL := fmt.Sprintf("%s?%s", p.APIURL, params.Encode())

	// createHTTPrequest
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")

	// useconnectionpoolcreateclient
	client := getHTTPClient()

	// createoutputchannel
	outputChan = make(chan []byte, 100)
	// startgoroutineprocessstreamingrespond
	go func() {
		// sendrequest
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("sendrequestfailed: %v", err)
			return
		}
		defer func() {
			resp.Body.Close()
		}()

		// inspectresponsestatuscode
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("API request failed, statuscode: %d, response: %s", resp.StatusCode, string(body))
			return
		}

		// inspectresponsecontenttypeandcontentlength
		// contentType := resp.Header.Get("Content-Type")
		contentLength := resp.ContentLength

		// recordresponselengthtolog
		log.Debugf("receiveTTSresponse, Content-Length: %d", contentLength)

		// judgeContent-Lengthwhetherreasonable
		if contentLength == 0 {
			log.Errorf("APIreturnemptyresponse, Content-Lengthis0")
			return
		}

		// MP3fileheaderat leastneed100bytethennormalparse
		// -1indicateunknownlength（forexamplechunkedtransfer）
		if contentLength > 0 && contentLength < 100 {
			log.Errorf("APIreturnofresponsetoosmalltoparseisMP3: %dbyte", contentLength)
			return
		}

		// accordingtoaudioformatprocessstreamingresponse
		if p.AudioFormat == "mp3" {
			// create MP3 decoder，pass context butno done channel
			mp3Decoder, err := util.CreateAudioDecoder(ctx, resp.Body, outputChan, frameDuration, p.AudioFormat)
			if err != nil {
				log.Errorf("createMP3 decoderfailed: %v", err)
				close(outputChan)
				return
			}

			// startdecodeprocess
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3decodefailed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("TTSstreamingsynthesiscancel, text: %s", text)
				return
			default:
				log.Infof("ttstime consumption: from input togetMP3dataendtime consumption: %d ms", time.Now().UnixMilli()-startTs)

			}
		} else {
			log.Errorf("currentlyonlysupportMP3formatofstreamingsynthesis")
		}
	}()

	return outputChan, nil
}

// SetVoice setvoiceparameter
func (p *CosyVoiceTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	if spkID, ok := voiceConfig["spk_id"].(string); ok && spkID != "" {
		p.SpeakerID = spkID
		return nil
	}
	return fmt.Errorf("invalidofvoiceconfig: Missing spk_id")
}

// Close closeresource（nostate Provider，noneedtoclose）
func (p *CosyVoiceTTSProvider) Close() error {
	return nil
}

// IsValid inspectresourcewhethervalid
func (p *CosyVoiceTTSProvider) IsValid() bool {
	return p != nil
}
