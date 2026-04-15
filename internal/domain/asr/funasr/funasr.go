package funasr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"xiaozhi-esp32-server-golang/constants"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"

	"xiaozhi-esp32-server-golang/internal/data/audio"
	"xiaozhi-esp32-server-golang/internal/domain/asr/types"
)

// FunasrConfig config structure
type FunasrConfig struct {
	Host          string // FunASR service host address
	Port          string // FunASR server port
	Mode          string // recognition mode, e.g., "online"
	SampleRate    int    // sampling rate
	ChunkSize     []int  // chunk size
	ChunkInterval int    // chunk interval
	Timeout       int    // connection timeout (seconds)
	AutoEnd       bool   // whether to auto-end after timeout xx ms, not dependent on isSpeaking being false
}

// DefaultConfig default config
var DefaultConfig = FunasrConfig{
	Host:          "localhost",
	Port:          "10095",
	Mode:          "online",
	SampleRate:    audio.SampleRate,
	ChunkInterval: 10,
	ChunkSize:     []int{5, 10, 5},
	Timeout:       30,
}

// Funasr implements ASR interface
type Funasr struct {
	config FunasrConfig

	// connection management
	conn      *websocket.Conn
	connMutex sync.RWMutex
	// send lock, ensure only one request uses connection at a time
	sendMutex sync.Mutex
}

var funasrStreamSeq atomic.Uint64
var funasrStreamPrefix = uuid.NewString()

type streamDebugState struct {
	audioChunkCount  atomic.Uint64
	audioSampleCount atomic.Uint64
}

// FunasrRequest FunASR WebSocket request structure
type FunasrRequest struct {
	Mode          string `json:"mode,omitempty"`           // recognition mode, e.g., "online"
	ChunkSize     []int  `json:"chunk_size,omitempty"`     // chunk size
	ChunkInterval int    `json:"chunk_interval,omitempty"` // chunk interval
	AudioFs       int    `json:"audio_fs,omitempty"`       // sampling rate
	WavName       string `json:"wav_name,omitempty"`       // audio name
	WavFormat     string `json:"wav_format,omitempty"`     // audio format
	IsSpeaking    bool   `json:"is_speaking"`              // whether speaking
	Hotwords      string `json:"hotwords,omitempty"`       // hotwords
	Itn           bool   `json:"itn,omitempty"`            // whether to perform text normalization
}

// FunasrResponse FunASR WebSocket response structure
type FunasrResponse struct {
	Text       string  `json:"text"`       // recognized text
	IsFinal    bool    `json:"is_final"`   // whether is final result
	WavName    string  `json:"wav_name"`   // audio name
	TimeStamp  string  `json:"timestamp"`  // timestamp
	Mode       string  `json:"mode"`       // mode
	Confidence float64 `json:"confidence"` // confidence degree
}

// NewFunasr creates a new Funasr instance
func NewFunasr(config FunasrConfig) (*Funasr, error) {
	if config.Host == "" {
		config = DefaultConfig
	}

	return &Funasr{
		config: config,
	}, nil
}

// getConnection gets connection, creates if not exists
func (f *Funasr) getConnection(ctx context.Context) (*websocket.Conn, error) {
	// first try to read existing connection
	f.connMutex.RLock()
	conn := f.conn
	f.connMutex.RUnlock()

	if conn != nil {
		log.Debugf("FunASR WebSocket reuse connection: conn=%p", conn)
		return conn, nil
	}

	// need to create new connection
	f.connMutex.Lock()
	defer f.connMutex.Unlock()

	// double-check, other goroutine may have already created connection
	if f.conn != nil {
		return f.conn, nil
	}

	// create new connection
	url := fmt.Sprintf("ws://%s:%s/", f.config.Host, f.config.Port)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to FunASR service failed: %v", err)
	}

	f.conn = conn
	log.Infof("FunASR WebSocket connection established: conn=%p", conn)
	return conn, nil
}

// clearConnection clears connection (used for reconnection)
func (f *Funasr) clearConnection() {
	f.connMutex.Lock()
	defer f.connMutex.Unlock()

	if f.conn != nil {
		log.Infof("FunASR WebSocket connection cleared: conn=%p", f.conn)
		f.conn.Close()
		f.conn = nil
	}
}

// StreamingResult streaming recognition result
type StreamingResult struct {
	Text    string // recognized text
	IsFinal bool   // whether is final result
}

// isTimeoutError determines if it's a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// check if it's a network timeout error
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// check error message for timeout keywords
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "i/o timeout")
}

// isConnectionClosedError determines if it's a connection closed error
func isConnectionClosedError(err error) bool {
	if err == nil {
		return false
	}

	// check if it's a WebSocket close error
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
		return true
	}

	// check error message for connection close keywords
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "connection closed") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "use of closed network connection")
}

// writeMessage safely writes message to WebSocket connection
func (f *Funasr) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	// use read lock to protect connection write operation, prevent concurrent write causing data corruption
	f.connMutex.RLock()
	defer f.connMutex.RUnlock()

	// check if connection is valid
	if conn == nil {
		return fmt.Errorf("connection already closed")
	}

	return conn.WriteMessage(messageType, data)
}

// StreamingRecognize implements streaming recognition
// receives audio data from audioStream, returns result through resultChan
// can control recognition process cancellation and timeout through ctx
func (f *Funasr) StreamingRecognize(ctx context.Context, audioStream <-chan []float32) (chan types.StreamingResult, error) {
	// use send lock to protect, ensure only one request uses connection at a time
	f.sendMutex.Lock()
	// Note: don't release lock when function returns, but release when goroutine completes

	// get connection (reuse or create)
	conn, err := f.getConnection(ctx)
	if err != nil {
		f.sendMutex.Unlock() // release lock immediately if get connection fails
		return nil, err
	}

	subCtx, cancelFunc := context.WithCancel(ctx)
	streamID := fmt.Sprintf("funasr-stream-%s-%d", funasrStreamPrefix, funasrStreamSeq.Add(1))
	wavName := streamID
	debugState := &streamDebugState{}

	// send initial message
	firstMessage := FunasrRequest{
		Mode:          f.config.Mode,
		ChunkSize:     []int{5, 10, 5},
		ChunkInterval: f.config.ChunkInterval,
		AudioFs:       f.config.SampleRate,
		WavName:       wavName,
		WavFormat:     "pcm",
		IsSpeaking:    true,
		Hotwords:      "{\"Alibaba\":20,\"hello world\":40}",
		Itn:           true,
	}

	log.Debugf(
		"funasr StreamingRecognize start: stream_id=%s, conn=%p, mode=%s, chunk_interval=%d, chunk_size=%v, wav_name=%s",
		streamID,
		conn,
		f.config.Mode,
		f.config.ChunkInterval,
		firstMessage.ChunkSize,
		firstMessage.WavName,
	)

	messageBytes, err := json.Marshal(firstMessage)
	if err != nil {
		cancelFunc()
		f.sendMutex.Unlock() // release lock immediately if serialize fails
		return nil, fmt.Errorf("serialize initial message failed: %v", err)
	}

	err = f.writeMessage(conn, websocket.TextMessage, messageBytes)
	if err != nil {
		// send failed, clear connection, will auto-reconnect next time
		log.Errorf("send initial message failed: %v, clear connection", err)
		f.clearConnection()
		cancelFunc()
		f.sendMutex.Unlock() // release lock immediately if send fails
		return nil, fmt.Errorf("send initial message failed: %v", err)
	}

	// create result channel, with buffer to avoid blocking
	resultChan := make(chan types.StreamingResult, 20)

	// use WaitGroup to wait for two goroutines to complete
	var wg sync.WaitGroup
	wg.Add(2)

	// start goroutine to receive and send data
	// release lock when goroutine completes
	go func() {
		defer wg.Done()
		f.recvResult(subCtx, conn, streamID, wavName, debugState, resultChan)
	}()

	go func() {
		defer wg.Done()
		f.forwardStreamAudio(subCtx, cancelFunc, conn, streamID, wavName, debugState, audioStream)
	}()

	// wait for goroutine to complete in background and release lock
	go func() {
		wg.Wait()
		f.clearConnection()
		f.sendMutex.Unlock()
		log.Debugf(
			"funasr StreamingRecognize goroutine complete, already released sendMutex: stream_id=%s, wav_name=%s, chunks=%d, samples=%d",
			streamID,
			wavName,
			debugState.audioChunkCount.Load(),
			debugState.audioSampleCount.Load(),
		)
	}()

	return resultChan, nil
}

func (f *Funasr) recvResult(ctx context.Context, conn *websocket.Conn, streamID string, wavName string, debugState *streamDebugState, resultChan chan types.StreamingResult) {
	defer func() {
		close(resultChan)
	}()

	for {
		select {
		case <-ctx.Done():
			// context cancelled, exit goroutine
			log.Debugf("funasr recvResult already cancelled: %v", ctx.Err())
			return
		default:
			// continue normal processing
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Debugf("funasr recvResult read recognition result failed: stream_id=%s, conn=%p, err=%v, clear connection", streamID, conn, err)
			// read failed, clear connection, will auto-reconnect next time
			f.clearConnection()
			return
		}
		log.Debugf(
			"funasr recvResult read recognition result: stream_id=%s, conn=%p, chunks=%d, samples=%d, payload=%v",
			streamID,
			conn,
			debugState.audioChunkCount.Load(),
			debugState.audioSampleCount.Load(),
			string(message),
		)

		var response FunasrResponse
		err = json.Unmarshal(message, &response)
		if err != nil {
			log.Debugf("funasr recvResult parse recognition result failed: %v", err)
			continue
		}

		if response.WavName != "" && response.WavName != wavName {
			log.Warnf(
				"funasr recvResult ignore non-current stream result: stream_id=%s, expected_wav=%s, actual_wav=%s, conn=%p, chunks=%d, samples=%d",
				streamID,
				wavName,
				response.WavName,
				conn,
				debugState.audioChunkCount.Load(),
				debugState.audioSampleCount.Load(),
			)
			continue
		}

		// only send result when there is text
		/*if response.Text == "" {
			continue
		}*/

		streamingResult := f.toStreamingResult(response)

		// sendrecognizeresult
		select {
		case <-ctx.Done():
			// contextcancel，exitgoroutine
			log.Debugf("funasr recvResult alreadycancel: %v", ctx.Err())
			return
		case resultChan <- streamingResult:
		}
		/*if f.config.AutoEnd {
			log.Debugf("funasr recvResult autoend")
			return
		}*/
		// resultsendsuccessful
		// ifyesfinallyresultandinputalreadyend，thenexit loop
		if streamingResult.IsFinal {
			log.Debugf(
				"funasr recvResult isfinal: stream_id=%s, conn=%p, response_mode=%s, raw_is_final=%v, text_len=%d, wav_name=%s, chunks=%d, samples=%d",
				streamID,
				conn,
				response.Mode,
				response.IsFinal,
				len([]rune(response.Text)),
				response.WavName,
				debugState.audioChunkCount.Load(),
				debugState.audioSampleCount.Load(),
			)
			return
		}
	}
}

func (f *Funasr) toStreamingResult(response FunasrResponse) types.StreamingResult {
	result := types.StreamingResult{
		Text:    response.Text,
		IsFinal: response.IsFinal,
		AsrType: constants.AsrTypeFunAsr,
		Mode:    response.Mode,
	}

	if strings.EqualFold(strings.TrimSpace(f.config.Mode), "2pass") {
		switch strings.ToLower(strings.TrimSpace(response.Mode)) {
		case "2pass-online":
			result.IsFinal = false
		case "2pass-offline":
			result.IsFinal = true
		}
	}

	if result.IsFinal && strings.TrimSpace(result.Text) == "" {
		result.EmptyReason = types.EmptyReasonProviderEmptyFinal
	}

	return result
}

func (f *Funasr) forwardStreamAudio(ctx context.Context, cancelFunc context.CancelFunc, conn *websocket.Conn, streamID string, wavName string, debugState *streamDebugState, audioStream <-chan []float32) {
	sendEndMsg := func() {
		// sendterminatemessage
		endMessage := FunasrRequest{
			Mode:          f.config.Mode,
			ChunkInterval: f.config.ChunkInterval,
			ChunkSize:     []int{5, 10, 5},
			WavName:       wavName,
			IsSpeaking:    false,
		}
		endMessageBytes, _ := json.Marshal(endMessage)
		log.Debugf(
			"funasr forwardStreamAudio sendendmessage: stream_id=%s, conn=%p, chunks=%d, samples=%d, payload=%v",
			streamID,
			conn,
			debugState.audioChunkCount.Load(),
			debugState.audioSampleCount.Load(),
			string(endMessageBytes),
		)
		err := f.writeMessage(conn, websocket.TextMessage, endMessageBytes)
		if err != nil {
			log.Debugf("funasr forwardStreamAudio sendendmessagefailed: stream_id=%s, conn=%p, err=%v，clearjoin", streamID, conn, err)
			f.clearConnection()
		}
	}
	// processinputaudio stream
	for {
		select {
		case <-ctx.Done():
			// contextcancel，sendendmessageandexit
			log.Debugf(
				"funasr forwardStreamAudio contextalreadycancel: stream_id=%s, conn=%p, chunks=%d, samples=%d, err=%v",
				streamID,
				conn,
				debugState.audioChunkCount.Load(),
				debugState.audioSampleCount.Load(),
				ctx.Err(),
			)
			// 注意：这innoneedcall cancelFunc()，becauseis ctx.Done() alreadybetriggerinstructioncontextalreadycancel
			sendEndMsg()
			return
		case pcmChunk, ok := <-audioStream:
			if !ok {
				// channel closed，endinput，neednotifyreceivegoroutinestop
				log.Debugf(
					"funasr forwardStreamAudio audio channelclose: stream_id=%s, conn=%p, chunks=%d, samples=%d",
					streamID,
					conn,
					debugState.audioChunkCount.Load(),
					debugState.audioSampleCount.Load(),
				)
				sendEndMsg()
				return
			}

			// convertPCMdataisbyte
			audioBytes := Float32SliceToBytes(pcmChunk)

			//log.Debugf("funasr forwardStreamAudio sendaudio data, pcmChunk len: %v, audioBytes len: %v", len(pcmChunk), len(audioBytes))

			// sendaudio data
			err := f.writeMessage(conn, websocket.BinaryMessage, audioBytes)
			if err != nil {
				log.Debugf("funasr forwardStreamAudio sendaudio datafailed: stream_id=%s, conn=%p, err=%v，clearjoin", streamID, conn, err)
				f.clearConnection()
				cancelFunc() // sendfailedwhencancelcontext，notify recvResult goroutine stop
				return
			}
			chunkCount := debugState.audioChunkCount.Add(1)
			sampleCount := debugState.audioSampleCount.Add(uint64(len(pcmChunk)))
			if chunkCount <= 3 || chunkCount%10 == 0 {
				log.Debugf(
					"funasr forwardStreamAudio alreadysendaudio chunk: stream_id=%s, conn=%p, chunk=%d, chunk_samples=%d, total_samples=%d, bytes=%d",
					streamID,
					conn,
					chunkCount,
					len(pcmChunk),
					sampleCount,
					len(audioBytes),
				)
			}
		}
	}
}

// Process processaudio dataandreturnrecognizeresult
func (f *Funasr) Process(pcmData []float32) (string, error) {
	ctx := context.Background()

	// usesendlockprotected，ensureat the same timeatimeonlyhavearequestatusejoin
	f.sendMutex.Lock()
	defer f.sendMutex.Unlock()

	// getjoin（复useorcreate）
	conn, err := f.getConnection(ctx)
	if err != nil {
		return "", err
	}

	audioBytes := Float32SliceToBytes(pcmData)

	// sendinitialmessage
	firstMessage := FunasrRequest{
		Mode:          f.config.Mode,
		ChunkSize:     []int{5, 10, 5},
		ChunkInterval: f.config.ChunkInterval,
		AudioFs:       f.config.SampleRate,
		WavName:       "stream",
		WavFormat:     "pcm",
		IsSpeaking:    true,
		Hotwords:      "",
		Itn:           true,
	}

	messageBytes, err := json.Marshal(firstMessage)
	if err != nil {
		return "", fmt.Errorf("serializeinitialmessagefailed: %v", err)
	}

	err = f.writeMessage(conn, websocket.TextMessage, messageBytes)
	if err != nil {
		// sendfailed，clearjoin，downtimesusewhenautomaticreconnect
		log.Errorf("sendinitialmessagefailed: %v，clearjoin", err)
		f.clearConnection()
		return "", fmt.Errorf("sendinitialmessagefailed: %v", err)
	}

	// willaudio data按blocksend
	chunkSize := int(audio.SampleRate * 0.1) // 每blocksizeabout100msofaudio (16000 * 0.1)
	for i := 0; i < len(audioBytes); i += chunkSize {
		end := i + chunkSize
		if end > len(audioBytes) {
			end = len(audioBytes)
		}
		chunk := audioBytes[i:end]

		err = f.writeMessage(conn, websocket.BinaryMessage, chunk)
		if err != nil {
			// sendfailed，clearjoin，downtimesusewhenautomaticreconnect
			log.Errorf("sendaudio datafailed: %v，clearjoin", err)
			f.clearConnection()
			return "", fmt.Errorf("sendaudio datafailed: %v", err)
		}
	}

	// sendterminatemessage
	endMessage := FunasrRequest{
		IsSpeaking: false,
	}
	endMessageBytes, _ := json.Marshal(endMessage)
	err = f.writeMessage(conn, websocket.TextMessage, endMessageBytes)
	if err != nil {
		// sendfailed，clearjoin，downtimesusewhenautomaticreconnect
		log.Errorf("sendterminatemessagefailed: %v，clearjoin", err)
		f.clearConnection()
		return "", fmt.Errorf("sendterminatemessagefailed: %v", err)
	}

	// setreadtimeout
	conn.SetReadDeadline(time.Now().Add(time.Duration(f.config.Timeout) * time.Second))

	// readresult
	var result string
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if isTimeoutError(err) {
				log.Debugf("funasr Process readresulttimeout: %v", err)
				f.clearConnection() // readtimeout，clearjoin
				return "", fmt.Errorf("readresulttimeout: %v", err)
			}
			if isConnectionClosedError(err) {
				log.Debugf("funasr Process readresultjoinalreadyclose: %v", err)
				f.clearConnection() // joinalreadyclose，clearjoin
				return "", fmt.Errorf("joinalreadyclose: %v", err)
			}
			// readfailed，clearjoin，downtimesusewhenautomaticreconnect
			log.Errorf("funasr Process readresultfailed: %v，clearjoin", err)
			f.clearConnection()
			return "", fmt.Errorf("readresultfailed: %v", err)
		}

		var response FunasrResponse
		err = json.Unmarshal(message, &response)
		if err != nil {
			continue
		}

		// check ifisfinallyresult
		if response.IsFinal {
			result = response.Text
			break
		}
	}

	return result, nil
}

func Float32ToInt16(sample float32) int16 {
	// limitat [-1, 1]，avoidoverflow
	if sample > 1.0 {
		sample = 1.0
	} else if sample < -1.0 {
		sample = -1.0
	}
	return int16(sample * 32767)
}

func Float32SliceToBytes(samples []float32) []byte {
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		i16 := Float32ToInt16(s)
		data[2*i] = byte(i16)
		data[2*i+1] = byte(i16 >> 8)
	}
	return data
}

// Close closeresource，releasejoin
func (f *Funasr) Close() error {
	f.clearConnection()
	return nil
}

// IsValid inspectresourcewhethervalid
func (f *Funasr) IsValid() bool {
	f.connMutex.RLock()
	conn := f.conn
	f.connMutex.RUnlock()
	return conn != nil
}

/*
errortypejudgeuseexample：

1. timeouterrorjudge：
   if isTimeoutError(err) {
       // processtimeoutsituation，mayneedretryor调bodytimeouttime
       log.Warnf("操astimeout: %v", err)
   }

2. joincloseerrorjudge：
   if isConnectionClosedError(err) {
       // processjoinclosesituation，mayneedre建立join
       log.Warnf("joinalreadyclose: %v", err)
   }

3. 综合errorprocess：
   _, message, err := conn.ReadMessage()
   if err != nil {
       if isTimeoutError(err) {
           // timeout：mayyesnetworkdelayorserverrespondslow
           // suggestion：调bodytimeouttimeorretry
       } else if isConnectionClosedError(err) {
           // joinclose：mayyesservermain动disconnectornetworkin断
           // suggestion：re建立join
       } else {
           // othererror：mayyesprotocolerrorordataformaterror
           // suggestion：inspectdataformatorprotocolimplement
       }
   }

常见errortype：
- timeouterror：i/o timeout, context deadline exceeded
- joinclose：connection closed, broken pipe, connection reset
- WebSocketclose：close 1000 (normal), close 1001 (going away)
*/
