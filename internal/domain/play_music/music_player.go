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
			//Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

// PlayMusicStream fromURLplay music，returnaudio streamchannel
// frameDuration: 每frameduration（毫second），default20ms
// audioFormat: audioformat，support "mp3"
func PlayMusicStream(ctx context.Context, url string, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameterverifyanddefault valuesset
	if frameDuration <= 0 {
		frameDuration = 20 // default20msframeduration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // defaultMP3format
	}

	startTs := time.Now().UnixMilli()

	// createHTTPrequest
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "audio/*")
	req.Header.Set("User-Agent", "MusicPlayer/1.0")

	// usejoinpoolcreateclient-side
	client := getHTTPClient()

	// createoutputchannel
	outputChan = make(chan []byte, 100)

	// startgoroutineprocessstreamingrespond
	go func() {
		// sendrequest
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf("sendrequestfailed: %v", err)
			close(outputChan)
			return
		}
		defer func() {
			resp.Body.Close()
		}()

		// inspectrespondstate码
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("API request failed，state码: %d, respond: %s", resp.StatusCode, string(body))
			close(outputChan)
			return
		}

		// inspectrespondinside容typeandinside容length
		contentLength := resp.ContentLength

		// recordrespondlengthtolog
		log.Debugf("receive音乐streamrespond，Content-Length: %d", contentLength)

		// judgeContent-Lengthwhether合理
		if contentLength == 0 {
			log.Errorf("音乐streamreturnemptyrespond，Content-Lengthis0")
			close(outputChan)
			return
		}

		// MP3fileheaderat leastneed100byteonly then能normalparse
		// -1indicatenot知length（例如minuteblock传输）
		if contentLength > 0 && contentLength < 100 {
			log.Errorf("音乐streamrespond太smallno法parseisMP3: %dbyte", contentLength)
			close(outputChan)
			return
		}

		log.Infof("start playing music: %s", url)

		// according toaudioformatprocessstreamingrespond
		if audioFormat == "mp3" {
			// create MP3 decoder，传入 context 而noyes done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, resp.Body, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("createMP3 decoderfailed: %v", err)
				close(outputChan)
				return
			}

			// startdecodepast程
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3decodefailed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("音乐playcancel, URL: %s", url)
				return
			default:
				log.Infof("音乐playcompletetime consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("currentonlysupportMP3formatofstreamingplay，传入format: %s", audioFormat)
			close(outputChan)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromAudioData(ctx context.Context, audioData []byte, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameterverifyanddefault valuesset
	if frameDuration <= 0 {
		frameDuration = 20 // default20msframeduration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // defaultMP3format
	}

	// adddebuginfo
	log.Debugf("PlayMusicFromAudioData: audio data length=%dbyte, sampling率=%d, frameduration=%dms, format=%s",
		len(audioData), sampleRate, frameDuration, audioFormat)

	// inspectaudio datawhetherisempty
	if len(audioData) == 0 {
		log.Errorf("audio dataisempty，no法play")
		return nil, fmt.Errorf("audio dataisempty")
	}

	startTs := time.Now().UnixMilli()

	// createoutputchannel
	outputChan = make(chan []byte, 100)

	// startgoroutineprocessstreamingrespond
	go func() {
		// from audioData create a io.ReadCloser
		audioReader := io.NopCloser(bytes.NewReader(audioData))

		// according toaudioformatprocessstreamingrespond
		if audioFormat == "mp3" {
			// create MP3 decoder，传入 context 而noyes done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, audioReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("createMP3 decoderfailed: %v", err)
				return
			}

			// startdecodepast程
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3decodefailed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("音乐playcancel")
				return
			default:
				log.Infof("音乐playcompletetime consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("currentonlysupportMP3formatofstreamingplay，传入format: %s", audioFormat)
		}
	}()

	return outputChan, nil
}

func PlayMusicFromPipe(ctx context.Context, pipeReader *io.PipeReader, sampleRate int, frameDuration int, audioFormat string) (outputChan chan []byte, err error) {
	// parameterverifyanddefault valuesset
	if frameDuration <= 0 {
		frameDuration = 20 // default20msframeduration
	}
	if audioFormat == "" {
		audioFormat = "mp3" // defaultMP3format
	}

	// adddebuginfo
	log.Debugf("PlayMusicFromPipe: sampling率=%d, frameduration=%dms, format=%s",
		sampleRate, frameDuration, audioFormat)

	startTs := time.Now().UnixMilli()

	// createoutputchannel
	outputChan = make(chan []byte, 100)

	// startgoroutineprocessstreamingrespond
	go func() {
		// according toaudioformatprocessstreamingrespond
		if audioFormat == "mp3" {
			// create MP3 decoder，传入 context 而noyes done channel
			mp3Decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, audioFormat, sampleRate)
			if err != nil {
				log.Errorf("createMP3 decoderfailed: %v", err)
				return
			}

			// startdecodepast程
			if err := mp3Decoder.Run(startTs); err != nil {
				log.Errorf("MP3decodefailed: %v", err)
				return
			}

			select {
			case <-ctx.Done():
				log.Debugf("音乐playcancel")
				return
			default:
				log.Infof("音乐playcompletetime consumption: %d ms", time.Now().UnixMilli()-startTs)
			}
		} else {
			log.Errorf("currentonlysupportMP3formatofstreamingplay，传入format: %s", audioFormat)
		}
	}()

	return outputChan, nil
}
