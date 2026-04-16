package play_music

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"bytes"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
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
			//Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// PlayMusicStream from URL play music, return audio stream channel
// frameDuration: per frame duration (millisecond), default 20ms
// audioFormat: audio format, support "mp3"
func PlayMusicStream(ctx context.Context, url string, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameter verify and default values set
	if frameDuration <= 0 {
		frameDuration = 20 // default 20ms frame duration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // default MP3 format
	}

	startTs := time.Now().UnixMilli()

	// create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "audio/*")
	req.Header.Set("User-Agent", "MusicPlayer/1.0")

	// use join pool create client-side
	client := getHTTPClient()

	// create output channel
	outputChan = make(chan []byte, 100)

	// start goroutine process streaming respond
	go func() {
		// send request
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("send request failed: %v", err)
			close(outputChan)
			return
		}
		defer func() {
			resp.Body.Close()
		}()

		// inspect respond state code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("API request failed, state code: %d, respond: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// inspect respond content type and content length
		contentLength := resp.ContentLength

		// record respond length to log
		log.Debugf("receive music stream respond, Content-Length: %d", contentLength)

		// judge Content-Length whether reasonable
		if contentLength == 0 {
			log.Errorf("music stream return empty respond, Content-Length is 0")
			close(outputChan)
			return
		}

		// MP3 file header at least need 100 byte only then can normal parse
		// -1 indicate not know length (for example minute block transmission)
		if contentLength > 0 && contentLength < 100 {
			log.Errorf("music stream respond too small no way parse is MP3: %d byte", contentLength)
			close(outputChan)
			return
		}

		log.Infof("start playing music: %s", url)

		// according to audio format process streaming respond
		if audioFormat == "mp3" {
			// create MP3 decoder, pass in context instead of done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("create MP3 decoder failed: %v", err)
				close(outputChan)
				return
			}

			// start decode process
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3 decode failed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("music play cancel, URL: %s", url)
				return
			default:
				log.Infof("music play complete time consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("current only support MP3 format of streaming play, pass in format: %s", audioFormat)
			close(outputChan)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromAudioData(ctx context.Context, audioData []byte, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameter verify and default values set
	if frameDuration <= 0 {
		frameDuration = 20 // default 20ms frame duration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // default MP3 format
	}

	// add debug info
	log.Debugf("PlayMusicFromAudioData: audio data length=%d byte, sampling rate=%d, frame duration=%dms, format=%s",
		len(audioData), sampleRate, frameDuration, audioFormat)

	// inspect audio data whether is empty
	if len(audioData) == 0 {
		log.Errorf("audio data is empty, no way play")
		return nil, fmt.Errorf("audio data is empty")
	}

	startTs := time.Now().UnixMilli()

	// create output channel
	outputChan = make(chan []byte, 100)

	// start goroutine process streaming respond
	go func() {
		// from audioData create a io.ReadCloser
		audioReader := io.NopCloser(bytes.NewReader(audioData))

		// according to audio format process streaming respond
		if audioFormat == "mp3" {
			// create MP3 decoder, pass in context instead of done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, audioReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("create MP3 decoder failed: %v", err)
				return
			}

			// start decode process
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3 decode failed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("music play cancel")
				return
			default:
				log.Infof("music play complete time consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("current only support MP3 format of streaming play, pass in format: %s", audioFormat)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromPipe(ctx context.Context, pipeReader *io.PipeReader, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameter verify and default values set
	if frameDuration <= 0 {
		frameDuration = 20 // default 20ms frame duration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // default MP3 format
	}

	// add debug info
	log.Debugf("PlayMusicFromPipe: sampling rate=%d, frame duration=%dms, format=%s",
		sampleRate, frameDuration, audioFormat)

	startTs := time.Now().UnixMilli()

	// create output channel
	outputChan = make(chan []byte, 100)

	// start goroutine process streaming respond
	go func() {
		// according to audio format process streaming respond
		if audioFormat == "mp3" {
			// create MP3 decoder, pass in context instead of done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("create MP3 decoder failed: %v", err)
				return
			}

			// start decode process
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3 decode failed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("music play cancel")
				return
			default:
				log.Infof("music play complete time consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("current only support MP3 format of streaming play, pass in format: %s", audioFormat)
		}
	}()

	return outputChan, nil
}
