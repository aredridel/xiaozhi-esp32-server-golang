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

// EdgeOfflineTTSProvider WebSocket TTS provider
type EdgeOfflineTTSProvider struct {
	ServerURL        string
	Timeout          time.Duration
	HandshakeTimeout time.Duration

	// join manage
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// send lock, ensure at the same time only have a request at use join
	sendMutex sync.Mutex
}

// NewEdgeOfflineTTSProvider create new Edge Offline TTS provider
func NewEdgeOfflineTTSProvider(config map[string]interface{}) *EdgeOfflineTTSProvider {
	serverURL, _ := config["server_url"].(string)
	timeout, _ := config["timeout"].(float64)
	handshakeTimeout, _ := config["handshake_timeout"].(float64)

	// set default values
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tts"
	}
	if timeout == 0 {
		timeout = 30 // default 30 second timeout
	}
	if handshakeTimeout == 0 {
		handshakeTimeout = 10 // default 10 second handshake timeout
	}

	return &EdgeOfflineTTSProvider{
		ServerURL:        serverURL,
		Timeout:          time.Duration(timeout) * time.Second,
		HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
	}
}

// getConnection get join, if not exist then create
func (p *EdgeOfflineTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// first try read existing join
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	if conn != nil {
		return conn, nil
	}

	// need create new join
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// double check, may other goroutine already create join
	if p.conn != nil {
		return p.conn, nil
	}

	// create new join
	dialer := &websocket.Dialer{
		HandshakeTimeout: p.HandshakeTimeout,
	}
	conn, _, err := dialer.DialContext(ctx, p.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket join failed: %v", err)
	}

	p.conn = conn
	log.Infof("WebSocket join already establish")
	return conn, nil
}

// clearConnection clear join (used for disconnect reconnect)
func (p *EdgeOfflineTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("WebSocket join already clear, wait next time reconnect")
	}
}

// writeMessage safely to WebSocket join write message
func (p *EdgeOfflineTTSProvider) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	// use read lock protect join write operation, prevent concurrent write cause data confusion
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	// inspect join whether valid
	if conn == nil {
		return fmt.Errorf("join already close")
	}

	return conn.WriteMessage(messageType, data)
}

// TextToSpeech will text convert is voice, return audio frame data
func (p *EdgeOfflineTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	var frames [][]byte

	// use send lock protect, ensure at the same time only have a request at use join
	p.sendMutex.Lock()
	// Note: not at function return when release lock, but at goroutine complete when release

	// get join (reuse or create)
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock() // get join failed when immediately release lock
		return nil, err
	}

	// send text (use protected write method)
	err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
	if err != nil {
		// send failed, clear join, next time use when automatic reconnect
		log.Errorf("send text failed: %v, clear join", err)
		p.clearConnection()
		p.sendMutex.Unlock() // send failed when immediately release lock
		return nil, fmt.Errorf("send text failed: %v", err)
	}

	// create pipe used for audio data transmission
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	startTs := time.Now().UnixMilli()

	// create audio decoder
	audioDecoder, err := util.CreateAudioDecoder(ctx, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		pipeReader.Close()
		p.sendMutex.Unlock() // create decoder failed when immediately release lock
		return nil, fmt.Errorf("create audio decoder failed: %v", err)
	}

	// start decoder
	go func() {
		if err := audioDecoder.Run(startTs); err != nil {
			log.Errorf("audio decode failed: %v", err)
		}
	}()

	// use WaitGroup wait read goroutine complete
	var wg sync.WaitGroup
	wg.Add(1)

	// receive WebSocket data and write pipe; lock here goroutine inside unified by defer release, ensure whether normal end, error or panic are will release
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
				log.Errorf("read WebSocket message failed: %v, clear join", err)
				// join disconnect, clear join, next time use when automatic reconnect
				p.clearConnection()
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := pipeWriter.Write(data); err != nil {
					log.Errorf("write audio data failed: %v", err)
					return
				}
			}
		}
	}()

	// receive all of Opus frame
	go func() {
		for frame := range outputChan {
			frames = append(frames, frame)
		}
	}()

	// wait complete or timeout
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("TTS synthesis timeout or be cancel")
	case <-done:
		close(outputChan)
		return frames, nil
	}
}

// TextToSpeechStream streaming voice synthesis
func (p *EdgeOfflineTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 100)

	go func() {
		// use send lock protect, ensure at the same time only have a request at use join
		p.sendMutex.Lock()

		// get join (reuse or create)
		conn, err := p.getConnection(ctx)
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("get WebSocket join failed: %v", err)
			return
		}

		// send text (use protected write method)
		err = p.writeMessage(conn, websocket.TextMessage, []byte(text))
		if err != nil {
			p.sendMutex.Unlock()
			log.Errorf("send text failed: %v, clear join", err)
			// send failed, clear join, next time use when automatic reconnect
			p.clearConnection()
			return
		}

		// create pipe used for audio data transmission
		pipeReader, pipeWriter := io.Pipe()
		defer func() {
			pipeWriter.Close()
			// read complete after release lock
			log.Debugf("TextToSpeechStream read completed, release sendMutex")
			p.sendMutex.Unlock()
		}()

		// start decoder (decoder will at defer in automatic close outputChan)
		go func() {

			startTs := time.Now().UnixMilli()
			// create audio decoder
			audioDecoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, "pcm", sampleRate)
			if err != nil {
				log.Errorf("create audio decoder failed: %v", err)
				return
			}

			audioDecoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(24000),
				NumChannels: channels,
				Precision:   2,
			})

			// decoder will at defer in automatic close outputChan
			if err := audioDecoder.Run(startTs); err != nil {
				log.Errorf("audio decode failed: %v", err)
			}
		}()

		// receive WebSocket data and write pipe (read process in hold lock, ensure serialization)
		for {
			select {
			case <-ctx.Done():
				log.Debugf("TextToSpeechStream context done, exit")
				// close pipeWriter, let decoder natural end and close channel
				return
			default:
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					// close pipeWriter, let decoder natural end and close channel
					pipeWriter.Close()
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return
					}
					log.Errorf("read WebSocket message failed: %v, clear join", err)
					// join disconnect, clear join, next time use when automatic reconnect
					p.clearConnection()
					return
				}

				if messageType == websocket.BinaryMessage {
					if _, err := pipeWriter.Write(data); err != nil {
						log.Errorf("write audio data failed: %v", err)
						return
					}
					return
				}
			}
		}
	}()

	return outputChan, nil
}

// SetVoice set voice parameter (EdgeOffline unsupported dynamic set voice, but not error)
func (p *EdgeOfflineTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	// EdgeOffline through WebSocket join, voice by server-side control, unsupported client-side dynamic set
	// return nil indicate operation successful (although actual up no execute any operation)
	return nil
}

// Close close resource, release join
func (p *EdgeOfflineTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid inspect resource whether valid
func (p *EdgeOfflineTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	// inspect join whether exist
	return conn != nil
}
