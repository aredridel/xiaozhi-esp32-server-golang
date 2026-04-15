package edge_offline

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	"github.com/gorilla/websocket"
)

// EdgeOfflineTTSProvider WebSocket TTS provide者
type EdgeOfflineTTSProvider struct {
	ServerURL        string
	Timeout          time.Duration
	HandshakeTimeout time.Duration

	// joinmanage
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// sendlock，ensureat the same timeatimeonlyhavearequestatusejoin
	sendMutex sync.Mutex
}

// NewEdgeOfflineTTSProvider create new Edge Offline TTS provide者
func NewEdgeOfflineTTSProvider(config map[string]interface{}) *EdgeOfflineTTSProvider {
	serverURL, _ := config["server_url"].(string)
	timeout, _ := config["timeout"].(float64)
	handshakeTimeout, _ := config["handshake_timeout"].(float64)

	// setdefault values
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tts"
	}
	if timeout == 0 {
		timeout = 30 // default30secondtimeout
	}
	if handshakeTimeout == 0 {
		handshakeTimeout = 10 // default10second握手timeout
	}

	return &EdgeOfflineTTSProvider{
		ServerURL:        serverURL,
		Timeout:          time.Duration(timeout) * time.Second,
		HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
	}
}

// getConnection getjoin，ifno存atthencreate
func (p *EdgeOfflineTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// firsttryread现havejoin
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	if conn != nil {
		return conn, nil
	}

	// needcreate新join
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// dual重inspect，mayother goroutine alreadycreatejoin
	if p.conn != nil {
		return p.conn, nil
	}

	// create新join
	dialer := &websocket.Dialer{
		HandshakeTimeout: p.HandshakeTimeout,
	}
	conn, _, err := dialer.DialContext(ctx, p.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocketjoinfailed: %v", err)
	}

	p.conn = conn
	log.Infof("WebSocket joinalready建立")
	return conn, nil
}

// clearConnection clearjoin（used for断线reconnect）
func (p *EdgeOfflineTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("WebSocket joinalreadyclear，waitdowntimesreconnect")
	}
}

// writeMessage 安全地to WebSocket joinwritemessage
func (p *EdgeOfflineTTSProvider) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	// usereadlockprotectedjoinwrite操as，preventconcurrentwritecausedata混乱
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	// inspectjoinwhethervalid
	if conn == nil {
		return fmt.Errorf("joinalreadyclose")
	}

	return conn.WriteMessage(messageType, data)
}

// TextToSpeech willtextconvertisvoice，returnaudio framedata
func (p *EdgeOfflineTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	var frames [][]byte

	// usesendlockprotected，ensureat the same timeatimeonlyhavearequestatusejoin
	p.sendMutex.Lock()
	// 注意：noatfunctionreturnwhenreleaselock，而yesat goroutine completewhenrelease

	// getjoin（复useorcreate）
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock() // getjoinfailedwhenimmediatelyreleaselock
		return nil, err
	}

	// sendtext（use受protectedofwritemethod）
	err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
	if err != nil {
		// sendfailed，clearjoin，downtimesusewhenautomaticreconnect
		log.Errorf("sendtext failed: %v，clearjoin", err)
		p.clearConnection()
		p.sendMutex.Unlock() // sendfailedwhenimmediatelyreleaselock
		return nil, fmt.Errorf("sendtext failed: %v", err)
	}

	// createpipeused foraudio data传输
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	startTs := time.Now().UnixMilli()

	// createaudiodecode器
	audioDecoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		pipeReader.Close()
		p.sendMutex.Unlock() // createdecode器failedwhenimmediatelyreleaselock
		return nil, fmt.Errorf("createaudiodecode器failed: %v", err)
	}

	// startdecode器
	go func() {
		if err := audioDecoder.Run(startTs); err != nil {
			log.Errorf("audiodecodefailed: %v", err)
		}
	}()

	// use WaitGroup waitread goroutine complete
	var wg sync.WaitGroup
	wg.Add(1)

	// receiveWebSocketdataandwritepipe；lockhere goroutine insideunifiedby defer release，ensurewhethernormalend、erroror panic arewillrelease
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		defer p.sendMutex.Unlock()
		defer close(done)
		defer pipeWriter.Close()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				log.Errorf("readWebSocketmessagefailed: %v，clearjoin", err)
				// joindisconnect，clearjoin，downtimesusewhenautomaticreconnect
				p.clearConnection()
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := pipeWriter.Write(data); err != nil {
					log.Errorf("writeaudio datafailed: %v", err)
					return
				}
			}
		}
	}()

	// receive集allofOpusframe
	go func() {
		for frame := range outputChan {
			frames = append(frames, frame)
		}
	}()

	// waitcompleteortimeout
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("TTS合成timeoutorbecancel")
	case <-done:
		close(outputChan)
		return frames, nil
	}
}

// TextToSpeechStream streamingvoice合成
func (p *EdgeOfflineTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 100)

	go func() {
		// usesendlockprotected，ensureat the same timeatimeonlyhavearequestatusejoin
		p.sendMutex.Lock()

		// getjoin（复useorcreate）
		conn, err := p.getConnection(ctx)
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("getWebSocketjoinfailed: %v", err)
			return
		}

		// sendtext（use受protectedofwritemethod）
		err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("sendtext failed: %v，clearjoin", err)
			// sendfailed，clearjoin，downtimesusewhenautomaticreconnect
			p.clearConnection()
			return
		}

		// createpipeused foraudio data传输
		pipeReader, pipeWriter := io.Pipe()
		defer func() {
			pipeWriter.Close()
			// readcompleteafterreleaselock
			log.Debugf("TextToSpeechStream read completed, release sendMutex")
			p.sendMutex.Unlock()
		}()

		// startdecode器（decode器willat defer inautomaticclose outputChan）
		go func() {

			startTs := time.Now().UnixMilli()
			// createaudiodecode器
			audioDecoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, "pcm", sampleRate)
			if err != nil {
				log.Errorf("createaudiodecode器failed: %v", err)
				return
			}

			audioDecoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(24000),
				NumChannels: channels,
				Precision:   2,
			})

			// decode器willat defer inautomaticclose outputChan
			if err := audioDecoder.Run(startTs); err != nil {
				log.Errorf("audiodecodefailed: %v", err)
			}
		}()

		// receiveWebSocketdataandwritepipe（readpast程in持havelock，ensureserial化）
		for {
			select {
			case <-ctx.Done():
				log.Debugf("TextToSpeechStream context done, exit")
				// close pipeWriter，letdecode器自然endandclose channel
				return
			default:
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					// close pipeWriter，letdecode器自然endandclose channel
					pipeWriter.Close()
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return
					}
					log.Errorf("readWebSocketmessagefailed: %v，clearjoin", err)
					// joindisconnect，clearjoin，downtimesusewhenautomaticreconnect
					p.clearConnection()
					return
				}

				if messageType == websocket.BinaryMessage {
					if _, err := pipeWriter.Write(data); err != nil {
						log.Errorf("writeaudio datafailed: %v", err)
						return
					}
					return
				}
			}
		}
	}()

	return outputChan, nil
}

// SetVoice setvoiceparameter（EdgeOffline unsupporteddynamicsetvoice，butno报错）
func (p *EdgeOfflineTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	// EdgeOffline through WebSocket join，voicebyserver-sidecontrol，unsupportedclient-sidedynamicset
	// return nil indicate操assuccessful（虽然actualupnoexecute任何操as）
	return nil
}

// Close closeresource，releasejoin
func (p *EdgeOfflineTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid inspectresourcewhethervalid
func (p *EdgeOfflineTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	// inspectjoinwhether存at
	return conn != nil
}
