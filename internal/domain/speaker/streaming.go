package speaker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// StreamingClient WebSocket streaming recognize client
type StreamingClient struct {
	wsURL      string
	conn       *websocket.Conn
	sampleRate int
	mutex      sync.Mutex
	writeMu    sync.Mutex
	peekMu     sync.Mutex
	finishWait chan finishResponse
	peekWaits  map[string]chan peekResponse
	lastPeekAt time.Time
}

type finishResponse struct {
	result *IdentifyResult
	err    error
}

type peekResponse struct {
	result    *IdentifyResult
	throttled bool
	err       error
}

// NewStreamingClient create streaming recognize client
func NewStreamingClient(baseURL string) *StreamingClient {
	wsURL := deriveWebSocketURL(baseURL)
	return &StreamingClient{
		wsURL: wsURL,
	}
}

// deriveWebSocketURL derive WebSocket URL from HTTP base_url
func deriveWebSocketURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		log.Errorf("parse base_url failed: %v, use default value", err)
		return "ws://localhost:8080/api/v1/speaker/identify_ws"
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	return fmt.Sprintf("%s://%s/api/v1/speaker/identify_ws", scheme, u.Host)
}

// Connect connect to voiceprint recognition service WebSocket
func (sc *StreamingClient) Connect(sampleRate int, agentId string, threshold float32) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.sampleRate = sampleRate

	// if connection already exists, use Ping to check if connection is still valid
	if sc.conn != nil {
		if sc.pingConnectionLocked() {
			// connection valid, reuse existing connection
			return nil
		}
		// connection already disconnected, close old connection and prepare to reconnect
		log.Debugf("detected old connection already disconnected, will re-establish connection")
		sc.closeConnectionLocked()
	}

	// build WebSocket URL, include sample rate, agent_id and threshold parameter
	wsURL := fmt.Sprintf("%s?sample_rate=%d", sc.wsURL, sampleRate)
	if agentId != "" {
		wsURL += fmt.Sprintf("&agent_id=%s", url.QueryEscape(agentId))
	}
	// if threshold greater than 0, then pass threshold parameter
	if threshold > 0 {
		wsURL += fmt.Sprintf("&threshold=%.6f", threshold)
	}

	// connect WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket connection failed: %v", err)
	}

	sc.conn = conn
	sc.finishWait = nil
	sc.peekWaits = make(map[string]chan peekResponse)

	// set read timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// receive connection acknowledge message
	var connectionMsg map[string]interface{}
	if err := conn.ReadJSON(&connectionMsg); err != nil {
		conn.Close()
		sc.conn = nil
		return fmt.Errorf("read connection acknowledge message failed: %v", err)
	}

	if msgType, ok := connectionMsg["type"].(string); !ok || msgType != "connection" {
		conn.Close()
		sc.conn = nil
		return fmt.Errorf("unexpected connection message: %v", connectionMsg)
	}
	conn.SetReadDeadline(time.Time{})

	log.Debugf("voiceprint recognition WebSocket connection successful, sample rate: %d Hz, agent_id: %s, threshold: %.4f", sampleRate, agentId, threshold)
	go sc.readLoop(conn)
	return nil
}

// SendAudioChunk send audio data block
func (sc *StreamingClient) SendAudioChunk(audioData []float32) error {
	conn := sc.getConn()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// convert float32 array to binary bytes
	chunkBytes := float32ToBytes(audioData)

	// send binary message
	sc.writeMu.Lock()
	err := conn.WriteMessage(websocket.BinaryMessage, chunkBytes)
	sc.writeMu.Unlock()
	if err != nil {
		// when send failed, close connection
		sc.failConnection(conn, fmt.Errorf("send audio data failed: %v", err))
		return fmt.Errorf("send audio data failed: %v", err)
	}

	return nil
}

// FinishAndIdentify complete input and get recognize result
func (sc *StreamingClient) FinishAndIdentify(ctx context.Context) (*IdentifyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	resultCh := make(chan finishResponse, 1)

	sc.mutex.Lock()
	if sc.conn == nil {
		sc.mutex.Unlock()
		return nil, fmt.Errorf("not connected")
	}
	if sc.finishWait != nil {
		sc.mutex.Unlock()
		return nil, fmt.Errorf("finish already in progress")
	}
	sc.finishWait = resultCh
	conn := sc.conn
	sc.mutex.Unlock()

	// send complete command
	finishCmd := map[string]interface{}{
		"action": "finish",
	}
	sc.writeMu.Lock()
	err := conn.WriteJSON(finishCmd)
	sc.writeMu.Unlock()
	if err != nil {
		sc.clearFinishWait(resultCh)
		sc.failConnection(conn, fmt.Errorf("send complete command failed: %v", err))
		return nil, fmt.Errorf("send complete command failed: %v", err)
	}

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case resp := <-resultCh:
		return resp.result, resp.err
	case <-ctx.Done():
		sc.clearFinishWait(resultCh)
		return nil, ctx.Err()
	case <-timer.C:
		sc.clearFinishWait(resultCh)
		return nil, fmt.Errorf("wait for final recognize result timeout")
	}
}

// PeekAndIdentify get middle recognize result (not ending current round)
// return: recognize result, whether server-side debounce, error
func (sc *StreamingClient) PeekAndIdentify(ctx context.Context, requestID string) (*IdentifyResult, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestID == "" {
		requestID = fmt.Sprintf("peek_%d", time.Now().UnixNano())
	}

	sc.peekMu.Lock()
	peekStarted := false
	defer func() {
		if peekStarted {
			sc.lastPeekAt = time.Now()
		}
		sc.peekMu.Unlock()
	}()
	if !sc.lastPeekAt.IsZero() && time.Since(sc.lastPeekAt) < 200*time.Millisecond {
		return nil, true, nil
	}
	peekStarted = true

	respCh := make(chan peekResponse, 1)

	sc.mutex.Lock()
	if sc.conn == nil {
		sc.mutex.Unlock()
		return nil, false, fmt.Errorf("not connected")
	}
	if sc.peekWaits == nil {
		sc.peekWaits = make(map[string]chan peekResponse)
	}
	sc.peekWaits[requestID] = respCh
	conn := sc.conn
	sc.mutex.Unlock()

	peekCmd := map[string]interface{}{
		"action": "peek",
	}
	peekCmd["request_id"] = requestID
	sc.writeMu.Lock()
	err := conn.WriteJSON(peekCmd)
	sc.writeMu.Unlock()
	if err != nil {
		sc.removePeekWait(requestID, respCh)
		sc.failConnection(conn, fmt.Errorf("send peek command failed: %v", err))
		return nil, false, fmt.Errorf("send peek command failed: %v", err)
	}

	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		return resp.result, resp.throttled, resp.err
	case <-ctx.Done():
		sc.removePeekWait(requestID, respCh)
		return nil, false, ctx.Err()
	case <-timer.C:
		sc.removePeekWait(requestID, respCh)
		return nil, false, fmt.Errorf("wait peek result timeout")
	}
}

// Close close connection
func (sc *StreamingClient) Close() error {
	sc.mutex.Lock()
	conn := sc.conn
	sc.conn = nil
	finishWait, peekWaits := sc.takePendingLocked()
	sc.mutex.Unlock()

	if conn != nil {
		if err := conn.Close(); err != nil {
			sc.signalPending(finishWait, peekWaits, fmt.Errorf("connection already closed: %v", err))
			return err
		}
	}
	sc.signalPending(finishWait, peekWaits, fmt.Errorf("connection already closed"))
	return nil
}

// closeConnectionLocked close connection (must be called while holding mutex)
func (sc *StreamingClient) closeConnectionLocked() error {
	if sc.conn != nil {
		err := sc.conn.Close()
		sc.conn = nil
		return err
	}
	return nil
}

// IsConnected check if already connected
func (sc *StreamingClient) IsConnected() bool {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.conn != nil
}

// pingConnectionLocked use Ping to check if connection is valid (must be called while holding mutex)
func (sc *StreamingClient) pingConnectionLocked() bool {
	if sc.conn == nil {
		return false
	}

	// use Ping message to detect connection activity
	sc.writeMu.Lock()
	sc.conn.SetWriteDeadline(time.Now().Add(1000 * time.Millisecond))
	err := sc.conn.WriteMessage(websocket.PingMessage, nil)
	sc.conn.SetWriteDeadline(time.Time{})
	sc.writeMu.Unlock()

	return err == nil
}

func (sc *StreamingClient) getConn() *websocket.Conn {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	return sc.conn
}

func (sc *StreamingClient) clearFinishWait(waitCh chan finishResponse) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	if sc.finishWait == waitCh {
		sc.finishWait = nil
	}
}

func (sc *StreamingClient) removePeekWait(requestID string, waitCh chan peekResponse) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	if existing, ok := sc.peekWaits[requestID]; ok && existing == waitCh {
		delete(sc.peekWaits, requestID)
	}
}

func (sc *StreamingClient) takePendingLocked() (chan finishResponse, []chan peekResponse) {
	finishWait := sc.finishWait
	sc.finishWait = nil

	peekWaits := make([]chan peekResponse, 0, len(sc.peekWaits))
	for requestID, waitCh := range sc.peekWaits {
		peekWaits = append(peekWaits, waitCh)
		delete(sc.peekWaits, requestID)
	}
	return finishWait, peekWaits
}

func (sc *StreamingClient) signalPending(finishWait chan finishResponse, peekWaits []chan peekResponse, err error) {
	if finishWait != nil {
		select {
		case finishWait <- finishResponse{err: err}:
		default:
		}
	}
	for _, waitCh := range peekWaits {
		if waitCh == nil {
			continue
		}
		select {
		case waitCh <- peekResponse{err: err}:
		default:
		}
	}
}

func (sc *StreamingClient) failConnection(conn *websocket.Conn, err error) {
	sc.mutex.Lock()
	if sc.conn != conn {
		sc.mutex.Unlock()
		return
	}
	_ = sc.closeConnectionLocked()
	finishWait, peekWaits := sc.takePendingLocked()
	sc.mutex.Unlock()
	sc.signalPending(finishWait, peekWaits, err)
}

func (sc *StreamingClient) readLoop(conn *websocket.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			sc.failConnection(conn, fmt.Errorf("read message failed: %v", err))
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Warnf("parse voiceprint message failed: %v", err)
			continue
		}

		if !sc.dispatchMessage(msg) {
			sc.failConnection(conn, parseServerError(msg))
			return
		}
	}
}

func (sc *StreamingClient) dispatchMessage(msg map[string]interface{}) bool {
	msgType, _ := msg["type"].(string)
	switch msgType {
	case "partial_result":
		requestID := getString(msg, "request_id")
		throttled := getBool(msg, "throttled")

		sc.mutex.Lock()
		waitCh := sc.peekWaits[requestID]
		if waitCh != nil {
			delete(sc.peekWaits, requestID)
		}
		sc.mutex.Unlock()
		if waitCh == nil {
			return true
		}

		var result *IdentifyResult
		if resultData, ok := msg["result"].(map[string]interface{}); ok && resultData != nil {
			result = identifyResultFromMap(resultData)
		}
		select {
		case waitCh <- peekResponse{result: result, throttled: throttled}:
		default:
		}
		return true
	case "result":
		sc.mutex.Lock()
		waitCh := sc.finishWait
		sc.finishWait = nil
		sc.mutex.Unlock()
		if waitCh == nil {
			return true
		}

		var result *IdentifyResult
		if resultData, ok := msg["result"].(map[string]interface{}); ok && resultData != nil {
			result = identifyResultFromMap(resultData)
		}
		select {
		case waitCh <- finishResponse{result: result}:
		default:
		}
		return true
	case "error":
		return false
	default:
		// audio_received/connection/ready/cancelled/closing etc messages only used for state hint, ignore here
		return true
	}
}

func parseServerError(msg map[string]interface{}) error {
	if errMsg, ok := msg["message"].(string); ok && errMsg != "" {
		return fmt.Errorf("server error: %s", errMsg)
	}
	return fmt.Errorf("server error: %v", msg)
}

// float32ToBytes convert float32 array to binary bytes (little-endian)
func float32ToBytes(samples []float32) []byte {
	buf := make([]byte, len(samples)*4)
	for i, sample := range samples {
		bits := math.Float32bits(sample)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

// auxiliary function: safely get value from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getFloat32(m map[string]interface{}, key string) float32 {
	if v, ok := m[key].(float64); ok {
		return float32(v)
	}
	return 0.0
}

func identifyResultFromMap(resultData map[string]interface{}) *IdentifyResult {
	return &IdentifyResult{
		Identified:  getBool(resultData, "identified"),
		SpeakerID:   getString(resultData, "speaker_id"),
		SpeakerName: getString(resultData, "speaker_name"),
		Confidence:  getFloat32(resultData, "confidence"),
		Threshold:   getFloat32(resultData, "threshold"),
	}
}
