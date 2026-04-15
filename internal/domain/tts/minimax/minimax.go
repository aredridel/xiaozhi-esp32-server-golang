package minimax

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// constant definition
const (
	wsURL = "wss://api.minimaxi.com/ws/v1/t2a_v2"
)

// global WebSocket Dialer
var wsDialer = websocket.Dialer{
	ReadBufferSize:   16384, // 16KB read buffer
	WriteBufferSize:  16384, // 16KB write buffer
	HandshakeTimeout: 45 * time.Second,
}

// MinimaxTTSProvider Minimax TTS provider
type MinimaxTTSProvider struct {
	APIKey     string
	Model      string
	Voice      string
	Speed      float64
	Volume     float64
	Pitch      int
	SampleRate int
	Bitrate    int
	Format     string
	Channel    int

	// connection management
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// send lock, ensure only one request uses connection at a time
	sendMutex sync.Mutex
}

// WebSocket message structure
type minimaxMessage struct {
	Event           string        `json:"event,omitempty"`
	Model           string        `json:"model,omitempty"`
	VoiceSetting    *voiceSetting `json:"voice_setting,omitempty"`
	AudioSetting    *audioSetting `json:"audio_setting,omitempty"`
	ContinuousSound bool          `json:"continuous_sound,omitempty"`
	Text            string        `json:"text,omitempty"`
}

type minimaxResp struct {
	SessionId string            `json:"session_id,omitempty"`
	Event     string            `json:"event,omitempty"`
	TraceId   string            `json:"trace_id,omitempty"`
	Data      *minimaxData      `json:"data,omitempty"`
	IsFinal   bool              `json:"is_final,omitempty"`
	BaseResp  *minimaxBaseResp  `json:"base_resp,omitempty"`
	ExtraInfo *minimaxExtraInfo `json:"extra_info,omitempty"`
}

type minimaxExtraInfo struct {
	AudioLength     int    `json:"audio_length"`
	AudioSampleRate int    `json:"audio_sample_rate"`
	AudioDuration   int    `json:"audio_duration"`
	AudioSize       int    `json:"audio_size"`
	Bitrate         int    `json:"bitrate"`
	AudioFormat     string `json:"audio_format"`
	AudioChannel    int    `json:"audio_channel"`

	UsageCharacters int `json:"usage_characters"`
	WordCount       int `json:"word_count"`
}

type minimaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type voiceSetting struct {
	VoiceID              string  `json:"voice_id"`
	Speed                float64 `json:"speed"`
	Vol                  float64 `json:"vol"`
	Pitch                int     `json:"pitch"`
	EnglishNormalization bool    `json:"english_normalization"`
}

type audioSetting struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

type minimaxData struct {
	Audio string `json:"audio"`
}

// NewMinimaxTTSProvider create new Minimax TTS provider
func NewMinimaxTTSProvider(config map[string]interface{}) *MinimaxTTSProvider {
	apiKey, _ := config["api_key"].(string)
	model, _ := config["model"].(string)
	voice, _ := config["voice"].(string)
	speed, _ := config["speed"].(float64)
	volume, _ := config["vol"].(float64)
	if volume == 0 {
		volume, _ = config["volume"].(float64)
	}
	pitch, _ := config["pitch"].(float64)
	sampleRate, _ := config["sample_rate"].(float64)
	bitrate, _ := config["bitrate"].(float64)
	format, _ := config["format"].(string)
	channel, _ := config["channel"].(float64)

	// set default values
	if model == "" {
		model = "speech-2.8-hd"
	}
	if voice == "" {
		voice = "male-qn-qingse"
	}
	if speed == 0 {
		speed = 1.0
	}
	if volume == 0 {
		volume = 1.0
	}
	if sampleRate == 0 {
		sampleRate = 32000
	}
	if bitrate == 0 {
		bitrate = 128000
	}
	if format == "" {
		format = "mp3"
	}
	if channel == 0 {
		channel = 1
	}

	return &MinimaxTTSProvider{
		APIKey:     apiKey,
		Model:      model,
		Voice:      voice,
		Speed:      speed,
		Volume:     volume,
		Pitch:      int(pitch),
		SampleRate: int(sampleRate),
		Bitrate:    int(bitrate),
		Format:     format,
		Channel:    int(channel),
	}
}

// TextToSpeech one-time synthesis (temporarily unsupported, use streaming implementation)
func (p *MinimaxTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// Minimax mainly supports streaming, here we collect streaming data and return
	outputChan, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}

	var frames [][]byte
	for frame := range outputChan {
		frames = append(frames, frame)
	}

	return frames, nil
}

// TextToSpeechStream streaming voice synthesis implementation
func (p *MinimaxTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	startTs := time.Now().UnixMilli()

	// use send lock protection, ensure only one request uses connection at a time
	p.sendMutex.Lock()
	// Note: do not release lock when function returns, but release when goroutine completes

	// get connection (reuse or create)
	conn, err := p.getConnection(ctx)
	if err != nil {
		p.sendMutex.Unlock()
		return nil, fmt.Errorf("get WebSocket connection failed: %v", err)
	}

	// create output channel
	outputChan = make(chan []byte, 100)

	// create pipe for audio decode
	pipeReader, pipeWriter := io.Pipe()

	// start audio decoder goroutine
	go func() {
		decoder, err := util.CreateAudioDecoderWithSampleRate(ctx, pipeReader, outputChan, frameDuration, p.Format, sampleRate)
		if err != nil {
			log.Errorf("create audio decoder failed: %v", err)
			pipeReader.Close()
			close(outputChan)
			return
		}

		if err := decoder.Run(startTs); err != nil {
			log.Errorf("audio decode failed: %v", err)
		}
	}()

	// use WaitGroup wait read goroutine complete
	var wg sync.WaitGroup
	wg.Add(1)

	// start read and process goroutine; lock here goroutine inside unified by defer release, ensure whether normal end, error or panic will release
	go func() {
		defer wg.Done()
		defer p.sendMutex.Unlock()
		defer func() {
			pipeWriter.Close()
			pipeReader.Close()
		}()

		p.processStreamTTS(ctx, conn, text, pipeWriter)
	}()

	// in background wait goroutine complete and release lock
	go func() {
		wg.Wait()
		log.Debugf("Minimax TTS streaming synthesis complete, time consumption: %d ms", time.Now().UnixMilli()-startTs)
	}()

	return outputChan, nil
}

// processStreamTTS process streaming TTS synthesis flow
func (p *MinimaxTTSProvider) processStreamTTS(ctx context.Context, conn *websocket.Conn, text string, pipeWriter *io.PipeWriter) {
	// send task start message
	startMsg := minimaxMessage{
		Event: "task_start",
		Model: p.Model,
		VoiceSetting: &voiceSetting{
			VoiceID:              p.Voice,
			Speed:                p.Speed,
			Vol:                  p.Volume,
			Pitch:                p.Pitch,
			EnglishNormalization: false,
		},
		AudioSetting: &audioSetting{
			SampleRate: p.SampleRate,
			Bitrate:    p.Bitrate,
			Format:     p.Format,
			Channel:    p.Channel,
		},
		ContinuousSound: false,
	}

	log.Debugf("minimax send task start message: model=%s, voice=%s, format=%s", p.Model, p.Voice, p.Format)
	if err := p.sendMessage(conn, startMsg); err != nil {
		log.Errorf("send task start message failed: %v", err)
		p.clearConnection()
		return
	}

	// wait task start acknowledge
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := p.readMessage(conn)
	if err != nil {
		// check if timeout error
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			log.Errorf("read task start acknowledge timeout (not received response within 10 seconds)")
		} else {
			log.Errorf("read task start acknowledge failed: %v", err)
		}
		p.clearConnection()
		return
	}

	log.Debugf("receive task start acknowledge message: %+v", msg)

	if msg.Event != "task_started" {
		log.Errorf("task start failed, expected 'task_started', received: event=%s, full body message=%+v", msg.Event, msg)
		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("error details: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}
		p.clearConnection()
		return
	}
	// reset read timeout
	conn.SetReadDeadline(time.Time{})

	log.Debugf("task start acknowledge successful")

	// send text message
	continueMsg := minimaxMessage{
		Event: "task_continue",
		Text:  text,
	}

	if err := p.sendMessage(conn, continueMsg); err != nil {
		log.Errorf("send text message failed: %v", err)
		p.clearConnection()
		return
	}

	// read audio data
	chunkCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Debugf("Minimax TTS streaming synthesis cancel, text: %s", text)
			// send task end message
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// according to documentation, server will close WebSocket connection after receiving task_finish
			// try read task_finished response (if server sends conversation)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if finishResp, err := p.readMessage(conn); err == nil {
				log.Debugf("receive task end acknowledge: event=%s, full body message=%+v", finishResp.Event, finishResp)
			} else {
				// connection may already close, this is normal behavior
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugf("server already close connection (normal behavior)")
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("close frame details: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				} else {
					log.Debugf("read task end acknowledge failed: %v", err)
					if closeErr, ok := err.(*websocket.CloseError); ok {
						log.Debugf("close frame details: code=%d, text=%s", closeErr.Code, closeErr.Text)
					}
				}
			}

			// clear connection state, because server already close connection
			p.clearConnection()
			return
		default:
		}

		// set read timeout
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		msg, err := p.readMessage(conn)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("read WebSocket message failed: %v", err)
				// try get close frame info
				if closeErr, ok := err.(*websocket.CloseError); ok {
					log.Errorf("WebSocket close frame details: code=%d, text=%s", closeErr.Code, closeErr.Text)
				}
				p.clearConnection()
				return
			}
			// normal close or read error
			log.Debugf("WebSocket connection close or read error: %v", err)
			if closeErr, ok := err.(*websocket.CloseError); ok {
				log.Debugf("WebSocket close frame details: code=%d, text=%s", closeErr.Code, closeErr.Text)
			}
			return
		}

		if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
			log.Errorf("BaseResp: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
		}

		// check if have error message
		if msg.Event == "error" || msg.Event == "task_error" {
			log.Errorf("receive error message: %+v", msg)
			if msg.BaseResp != nil && msg.BaseResp.StatusCode != 0 {
				log.Errorf("error details: status_code=%d, status_msg=%s", msg.BaseResp.StatusCode, msg.BaseResp.StatusMsg)
			}
			p.clearConnection()
			return
		}

		// process audio data
		if msg.Data != nil && msg.Data.Audio != "" {
			chunkCount++

			// convert hex encoded audio data to binary
			audioBytes, err := hex.DecodeString(msg.Data.Audio)
			if err != nil {
				log.Errorf("decode audio data failed: %v", err)
				continue
			}

			// write pipe for decoder processing
			if _, err := pipeWriter.Write(audioBytes); err != nil {
				log.Errorf("write audio data to pipe failed: %v", err)
				p.clearConnection()
				return
			}
		}

		// check if complete
		if msg.IsFinal {
			log.Debugf("receive last audio chunk, total %d chunks", chunkCount)
			// send task end message
			finishMsg := minimaxMessage{Event: "task_finish"}
			p.sendMessage(conn, finishMsg)

			// clear connection state, because server already close connection
			// need to create new connection for next use
			p.clearConnection()
			return
		}
	}
}

// getConnection get connection, if not exist then create
func (p *MinimaxTTSProvider) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// first try read existing connection
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	if conn != nil {
		return conn, nil
	}

	// need create new connection
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// double check, may other goroutine already created connection
	if p.conn != nil {
		return p.conn, nil
	}

	// create HTTP header
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// create new connection
	conn, resp, err := wsDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			log.Errorf("WebSocket connection failed, status code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("WebSocket connection failed: %v", err)
	}

	// set message read limit
	conn.SetReadLimit(1024 * 1024) // 1MB maximum message size

	// set keep connection
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(1*time.Second))
	})

	// wait connection successful message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read connection acknowledge message failed: %v", err)
	}

	log.Debugf("receive connection acknowledge message (original): %s", string(message))

	var connectMsg minimaxResp
	if err := json.Unmarshal(message, &connectMsg); err != nil {
		conn.Close()
		log.Errorf("parse connection acknowledge message failed, original message: %s, error: %v", string(message), err)
		return nil, fmt.Errorf("parse connection acknowledge message failed: %v", err)
	}

	log.Debugf("receive connection acknowledge message (parsed): %+v", connectMsg)

	if connectMsg.Event != "connected_success" {
		conn.Close()
		log.Errorf("connection failed, expected 'connected_success', received: %+v", connectMsg)
		return nil, fmt.Errorf("connection failed, received: %+v", connectMsg)
	}

	p.conn = conn
	log.Infof("Minimax WebSocket connection already established")
	return conn, nil
}

// clearConnection clear connection (used for disconnect reconnect)
func (p *MinimaxTTSProvider) clearConnection() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		log.Infof("Minimax WebSocket connection already cleared, wait for next reconnect")
	}
}

// sendMessage send JSON message
func (p *MinimaxTTSProvider) sendMessage(conn *websocket.Conn, msg minimaxMessage) error {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection already closed")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("serialize message failed: %v", err)
	}

	log.Debugf("minimax send message: %s", string(data))

	return conn.WriteMessage(websocket.TextMessage, data)
}

// readMessage read JSON message
func (p *MinimaxTTSProvider) readMessage(conn *websocket.Conn) (*minimaxResp, error) {
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	_ = messageType
	//log.Debugf("minimax read WebSocket message: type=%d, original content length=%d, content=%s", messageType, len(message), string(message))

	var msg minimaxResp
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("parse message failed, original message: %s, error: %v", string(message), err)
		return nil, fmt.Errorf("parse message failed: %v", err)
	}

	return &msg, nil
}

// SetVoice set voice parameter
func (p *MinimaxTTSProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return nil
}

// Close close resource, release connection
func (p *MinimaxTTSProvider) Close() error {
	p.clearConnection()
	return nil
}

// IsValid check resource whether valid
func (p *MinimaxTTSProvider) IsValid() bool {
	p.connMutex.RLock()
	conn := p.conn
	p.connMutex.RUnlock()

	return conn != nil
}
