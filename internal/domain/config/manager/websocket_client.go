package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"

	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/openclaw"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

type MessageHandleFunc func(*WebSocketRequest) (string, error)

type WebSocketClient struct {
	conn           *websocket.Conn
	baseURL        string
	requestTimeout time.Duration
	responseChans  map[string]chan *WebSocketResponse
	callbacks      map[string]func(*WebSocketResponse)
	requestHandler func(*WebSocketRequest) // Process received request
	mu             sync.RWMutex
	writeMu        sync.Mutex // Protect WebSocket write operations, prevent concurrent writes
	isConnected    bool
	connectMu      sync.Mutex
	messageQueue   chan *WebSocketRequest
	workers        sync.WaitGroup

	messageHandle cmap.ConcurrentMap[string, MessageHandleFunc]
	uuid          string

	// Reconnect related fields
	retryStopChan  chan struct{}  // Reconnect goroutine stop signal
	retryWg        sync.WaitGroup // Reconnect goroutine wait group
	retryMu        sync.Mutex     // Protect reconnect related operations
	isRetrying     bool           // Whether is reconnecting
	isShuttingDown bool           // Whether is closing (manual disconnect, no reconnect)
}

type WebSocketRequest struct {
	ID      string                 `json:"id"`
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type WebSocketResponse struct {
	ID      string                 `json:"id"`
	Status  int                    `json:"status"`
	Headers map[string]string      `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type managerWSClientClaims struct {
	Purpose string `json:"purpose"`
	UUID    string `json:"uuid"`
	jwt.RegisteredClaims
}

var (
	defaultClient           *WebSocketClient
	clientOnce              sync.Once
	systemConfigPushHandler func(map[string]interface{})
)

// SetSystemConfigPushHandler sets the callback for receiving system_config push (used by main program to merge into viper, etc.), injected by user_config during Init
func SetSystemConfigPushHandler(fn func(map[string]interface{})) {
	systemConfigPushHandler = fn
}

func GetDefaultClient() *WebSocketClient {
	clientOnce.Do(func() {
		defaultClient = NewWebSocketClient()
	})
	return defaultClient
}

func NewWebSocketClient() *WebSocketClient {
	// Priority from environment variable, if not exists then from config
	baseURL := util.GetBackendURL()
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &WebSocketClient{
		baseURL:        baseURL,
		requestTimeout: 30 * time.Second,
		responseChans:  make(map[string]chan *WebSocketResponse),
		callbacks:      make(map[string]func(*WebSocketResponse)),
		messageQueue:   make(chan *WebSocketRequest, 100),
		messageHandle:  cmap.New[MessageHandleFunc](),
		uuid:           uuid.New().String(),
		retryStopChan:  make(chan struct{}),
		isRetrying:     false,
	}
}

func NewWebSocketClientWithHandler(requestHandler func(*WebSocketRequest)) *WebSocketClient {
	client := NewWebSocketClient()
	client.requestHandler = requestHandler
	return client
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.isConnected {
		return nil
	}

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws://" + c.baseURL[7:] + "/ws" // Remove "http://" and add "/ws"
	wsToken, err := c.generateWSToken()
	if err != nil {
		return fmt.Errorf("failed to generate WebSocket authentication token: %v", err)
	}

	// Establish WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{c.baseURL},
		"UUID":   []string{c.uuid},
		"Authorization": []string{
			"Bearer " + wsToken,
		},
	})
	if err != nil {
		return fmt.Errorf("WebSocket connection failed: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	// Set ping handler
	conn.SetPongHandler(func(appData string) error {
		log.Debugf("Received pong message")
		return nil
	})

	// Start message processing loop
	go c.handleMessages()

	// Start message sending worker threads
	c.startWorkers()

	// Start heartbeat detection
	go c.startHeartbeat()

	log.Debugf("WebSocket client already connected to: %s", wsURL)
	return nil
}

func (c *WebSocketClient) generateWSToken() (string, error) {
	claims := managerWSClientClaims{
		Purpose: "manager-ws-client",
		UUID:    c.uuid,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := []byte(util.GetManagerEndpointAuthToken())
	return token.SignedString(secret)
}

func (c *WebSocketClient) Disconnect() error {
	return c.disconnect(false)
}

// disconnect internal disconnect method
// manualDisconnect: true indicates manual disconnect (no trigger reconnect), false indicates error disconnect (trigger reconnect)
func (c *WebSocketClient) disconnect(manualDisconnect bool) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if !c.isConnected {
		return nil
	}

	if manualDisconnect {
		c.isShuttingDown = true
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Debugf("Error closing WebSocket connection: %v", err)
		}
		c.conn = nil
	}

	c.isConnected = false
	c.mu.Lock()
	// Close all response channels
	for _, ch := range c.responseChans {
		close(ch)
	}
	c.responseChans = make(map[string]chan *WebSocketResponse)
	c.callbacks = make(map[string]func(*WebSocketResponse))
	c.mu.Unlock()

	// Stop worker threads
	close(c.messageQueue)
	c.workers.Wait()
	// Recreate message queue
	c.messageQueue = make(chan *WebSocketRequest, 100)

	log.Debugf("WebSocket connection already disconnected")
	return nil
}

func (c *WebSocketClient) IsConnected() bool {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	return c.isConnected
}

func (c *WebSocketClient) SendRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connection failed: %v", err)
		}
	}

	// generateUUIDasisrequestID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// createrespondchannel
	responseChan := make(chan *WebSocketResponse, 1)
	c.mu.Lock()
	c.responseChans[requestID] = responseChan
	c.mu.Unlock()

	// cleanuprespondchannel
	defer func() {
		c.mu.Lock()
		delete(c.responseChans, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	// sendrequest（usewritelockprotected）
	c.writeMu.Lock()
	err := c.conn.WriteJSON(request)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("send request failed: %v", err)
	}

	// waitrespond
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(c.requestTimeout):
		return nil, fmt.Errorf("request timeout")
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled")
	}
}

// convenient method - use WebSocket native ping
func (c *WebSocketClient) Ping() error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocketnotjoin")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
}

func (c *WebSocketClient) GetStatus(ctx context.Context) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "GET", "/api/ws/status", nil)
}

func (c *WebSocketClient) Echo(ctx context.Context, message string) (*WebSocketResponse, error) {
	return c.SendRequest(ctx, "POST", "/api/ws/echo", map[string]interface{}{
		"message": message,
	})
}

// global helper method
func ConnectManagerWebSocket(ctx context.Context) error {
	return GetDefaultClient().Connect(ctx)
}

func DisconnectManagerWebSocket() error {
	client := GetDefaultClient()
	client.StopReconnect()
	return client.disconnect(true) // manual disconnect, not trigger reconnect
}

func SendManagerRequest(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, method, path, body)
}

func ManagerWebSocketPing(ctx context.Context) error {
	return GetDefaultClient().Ping()
}

func ManagerWebSocketStatus(ctx context.Context) (*WebSocketResponse, error) {
	return GetDefaultClient().GetStatus(ctx)
}

func ManagerWebSocketEcho(ctx context.Context, message string) (*WebSocketResponse, error) {
	return GetDefaultClient().Echo(ctx, message)
}

func IsManagerWebSocketConnected() bool {
	return GetDefaultClient().IsConnected()
}

func SendDeviceRequest(ctx context.Context, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return GetDefaultClient().SendRequest(ctx, "POST", path, body)
}

// startWorkers starts message send worker threads
func (c *WebSocketClient) startWorkers() {
	workerCount := 3 // start 3 worker threads

	for i := 0; i < workerCount; i++ {
		c.workers.Add(1)
		go func(workerID int) {
			defer c.workers.Done()

			log.Debugf("Manager WebSocket worker thread %d started", workerID)

			for request := range c.messageQueue {
				if !c.IsConnected() {
					log.Debugf("Worker thread %d: WebSocket not connected, discard request", workerID)
					continue
				}

				// sendrequest（usewritelockprotected）
				c.writeMu.Lock()
				err := c.conn.WriteJSON(request)
				c.writeMu.Unlock()
				if err != nil {
					log.Debugf("Worker thread %d: send request failed: %v", workerID, err)
					// joinmayalreadydisconnect，triggerreconnect
					c.handleConnectionError()
					continue
				}

				log.Debugf("Worker thread %d: already sent request %s", workerID, request.ID)
			}

			log.Debugf("Manager WebSocket worker thread %d stopped", workerID)
		}(i)
	}
}

// handleConnectionError processes connection error
func (c *WebSocketClient) handleConnectionError() {
	if c.IsConnected() {
		log.Warn("Detected WebSocket connection error, disconnecting...")
		c.disconnect(false) // errordisconnect，willtriggerreconnect
		// triggerreconnect
		c.triggerReconnect()
	}
}

// startHeartbeat starts heartbeat detection
func (c *WebSocketClient) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second) // Send ping every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !c.IsConnected() {
				return
			}

			// sendpingmessage
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()

			if err != nil {
				log.Warnf("Send ping failed, connection may already be disconnected: %v", err)
				c.disconnect(false) // Error disconnect, will trigger reconnect
				// triggerreconnect
				c.triggerReconnect()
				return
			}
			log.Debugf("Ping message sent successfully")

		case <-c.retryStopChan:
			return
		}
	}
}

// triggerReconnect triggers reconnect (non-blocking)
func (c *WebSocketClient) triggerReconnect() {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()

	// If closing, don't trigger reconnect
	if c.isShuttingDown {
		log.Debug("Closing, not triggering reconnect")
		return
	}

	// If already reconnecting, don't trigger again
	if c.isRetrying {
		return
	}

	c.isRetrying = true
	// Start reconnect goroutine
	c.retryWg.Add(1)
	go c.startReconnectLoop()
}

// startReconnectLoop starts reconnect loop (uses exponential backoff algorithm)
func (c *WebSocketClient) startReconnectLoop() {
	defer func() {
		c.retryMu.Lock()
		c.isRetrying = false
		c.retryMu.Unlock()
		c.retryWg.Done()
	}()

	// Hardcoded backoff algorithm parameters
	initialDelay := 3 * time.Second // Initial delay 3 seconds
	maxDelay := 1 * time.Minute     // Maximum delay 1 minute
	backoffMultiplier := 2.0        // Backoff multiplier

	delay := initialDelay
	retryCount := 0

	log.Infof("Manager WebSocket reconnect goroutine started")

	for {
		// Check if should stop reconnect
		select {
		case <-c.retryStopChan:
			log.Info("receivestopsignal，stopreconnect")
			return
		default:
		}

		// If closing, stop reconnect
		c.retryMu.Lock()
		shuttingDown := c.isShuttingDown
		c.retryMu.Unlock()
		if shuttingDown {
			log.Info("Closing, stopping reconnect")
			return
		}

		// If already connected, stop reconnect
		if c.IsConnected() {
			log.Info("Manager WebSocket connection recovered, stopping reconnect")
			return
		}

		retryCount++
		log.Warnf("Manager WebSocket connection failed (attempt %d), will retry after %v...", retryCount, delay)

		// Wait for delay time
		select {
		case <-time.After(delay):
			// Continue reconnect
		case <-c.retryStopChan:
			log.Info("receivestopsignal，stopreconnect")
			return
		}

		// Try to connect
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Connect(ctx)
		cancel()

		if err != nil {
			log.Warnf("Manager WebSocket connection failed (attempt %d): %v", retryCount, err)
			// Calculate next delay time (exponential backoff)
			delay = time.Duration(float64(delay) * backoffMultiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
			continue
		}

		// Connection successful
		log.Info("Manager WebSocket connection successful")
		return
	}
}

// StopReconnect stops reconnect goroutine
func (c *WebSocketClient) StopReconnect() {
	c.retryMu.Lock()
	c.isShuttingDown = true
	shouldClose := c.retryStopChan != nil
	c.retryMu.Unlock()

	if shouldClose {
		// Use select to avoid closing channel repeatedly
		select {
		case <-c.retryStopChan:
			// Channel already closed
		default:
			close(c.retryStopChan)
		}
		c.retryWg.Wait()
		log.Info("Manager WebSocket reconnect goroutine gracefully stopped")
	}
}

// SendRequestWithCallback sends request and uses callback to process response
func (c *WebSocketClient) SendRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
	}

	// generateUUIDasisrequestID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Register callback
	c.mu.Lock()
	c.callbacks[requestID] = callback
	c.mu.Unlock()

	// cleanupcallback
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
	}()

	// will request play into queue
	select {
	case c.messageQueue <- &request:
		log.Debugf("Request %s already added to queue", requestID)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("message queue full, request timeout")
	case <-ctx.Done():
		return fmt.Errorf("context cancelled")
	}
}

// SendRequestAsync sends request asynchronously
func (c *WebSocketClient) SendRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return "", fmt.Errorf("connection failed: %v", err)
		}
	}

	// generateUUIDasisrequestID
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// will request play into queue
	select {
	case c.messageQueue <- &request:
		log.Debugf("Async request %s already added to queue", requestID)
		return requestID, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("message queue full, request timeout")
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled")
	}
}

// GetResponse gets response for specified request ID (used for async request)
func (c *WebSocketClient) GetResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	responseChan := make(chan *WebSocketResponse, 1)

	// Register temporary callback
	c.mu.Lock()
	c.callbacks[requestID] = func(response *WebSocketResponse) {
		responseChan <- response
	}
	c.mu.Unlock()

	// cleanupcallback
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
		close(responseChan)
	}()

	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("wait response timeout")
	}
}

// handleSystemConfigPush processes server-side push of system config change, asynchronously calls registered callback
func (c *WebSocketClient) handleSystemConfigPush(data map[string]interface{}) {
	if systemConfigPushHandler == nil {
		log.Debugf("Received system_config push, but no process callback registered")
		return
	}
	go systemConfigPushHandler(data)
}

// handleMessages processes received WebSocket messages
func (c *WebSocketClient) handleMessages() {
	for {
		if !c.isConnected {
			return
		}

		// Read message type
		messageType, reader, err := c.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debugf("WebSocketreaderror: %v", err)
			}
			c.disconnect(false) // errordisconnect，willtriggerreconnect
			// triggerreconnect
			c.triggerReconnect()
			return
		}

		// Process different types of messages
		switch messageType {
		case websocket.TextMessage:
			// Process JSON message
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Errorf("parseJSONmessagefailed: %v", err)
				continue
			}

			// Judge by message type: server push (system_config), request, response
			if msgType, _ := rawMessage["type"].(string); msgType == "system_config" {
				if data, ok := rawMessage["data"].(map[string]interface{}); ok {
					c.handleSystemConfigPush(data)
				} else {
					log.Warnf("Received system_config push but data format invalid")
				}
			} else if method, exists := rawMessage["method"]; exists && method != nil {
				// This is received request
				c.handleIncomingRequest(rawMessage)
			} else if status, exists := rawMessage["status"]; exists && status != nil {
				// This is received response
				c.handleIncomingResponse(rawMessage)
			} else {
				log.Warnf("Received unrecognized WebSocket message: %+v", rawMessage)
			}

		case websocket.PingMessage:
			// Process ping message, auto reply pong (use write lock protection)
			log.Debugf("Received ping message, auto reply pong")
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()
			if err != nil {
				log.Errorf("Send pong failed: %v", err)
			}

		case websocket.PongMessage:
			// Process pong message
			log.Debugf("Received pong message")

		case websocket.CloseMessage:
			// Process close message
			log.Debugf("Received close message")
			c.disconnect(false) // errordisconnect，willtriggerreconnect
			// triggerreconnect
			c.triggerReconnect()
			return

		default:
			log.Warnf("Received unknown type of WebSocket message: %d", messageType)
		}
	}
}

// handleIncomingRequest processes received request
func (c *WebSocketClient) handleIncomingRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Errorf("Parse WebSocket request failed: %v", err)
		return
	}

	log.Debugf("Received request: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// If registered request handler exists, call it
	if c.requestHandler != nil {
		go c.requestHandler(&request)
	} else {
		// If no registered handler, use default handler to process known paths
		c.handleDefaultRequest(&request)
	}
}

func (c *WebSocketClient) RegisterMessageHandler(ctx context.Context, path string, handler types.EventHandler) {
	f := func(request *WebSocketRequest) (string, error) {
		return handler(ctx, request.Path, request.Body)
	}
	c.messageHandle.Set(path, f)
}

// handleDefaultRequest is the default request handler
func (c *WebSocketClient) handleDefaultRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/config/test":
		// Config test may be time consuming (VAD/ASR/LLM/TTS serial execution), put in independent goroutine to avoid blocking read loop, support multiple requests concurrently
		go c.handleConfigTestRequest(request)

	case "/api/mcp/tools":
		// Process MCP tool list request
		c.handleMcpToolListRequest(request)

	case "/api/mcp/call":
		// Process MCP tool call request
		c.handleMcpToolCallRequest(request)

	case "/api/openclaw/status":
		c.handleOpenClawStatusRequest(request)

	case "/api/openclaw/chat":
		c.handleOpenClawChatRequest(request)

	case "/api/server/info":
		// Return server info
		response := map[string]interface{}{
			"server_name": "xiaozhi-server",
			"version":     "1.0.0",
			"uptime":      time.Now().Format(time.RFC3339),
			"request_id":  request.ID,
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("Send server info response failed: %v", err)
		}

	case "/api/server/ping":
		// Simple ping response
		response := map[string]interface{}{
			"message": "pong from server",
			"time":    time.Now().Format(time.RFC3339),
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("Send ping response failed: %v", err)
		}
	default:
		handler, exists := c.messageHandle.Get(request.Path)
		if exists {
			// Call handler and process return value
			result, err := handler(request)
			if err != nil {
				log.Errorf("Process request %s failed: %v", request.Path, err)
				// Send error response
				if err := c.SendResponse(request.ID, 500, nil, err.Error()); err != nil {
					log.Errorf("Send error response failed: %v", err)
				}
			} else {
				// Send success response
				response := map[string]interface{}{
					"result": result,
				}
				if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
					log.Errorf("Send success response failed: %v", err)
				}
			}
		} else {
			log.Warnf("Received unknown WebSocket request path: %s, ID: %s", request.Path, request.ID)

			// Send 404 response
			if err := c.SendResponse(request.ID, 404, nil, "Unknown endpoint"); err != nil {
				log.Errorf("Send error response failed: %v", err)
			}
		}
	}
}

// configTestTotalTimeout is the total timeout for config test (VAD+ASR+LLM+TTS combined)
const configTestTotalTimeout = 90 * time.Second

// handleConfigTestRequest processes config test request: VAD/ASR/LLM/TTS use received config and fixed WAV/text to execute light test
func (c *WebSocketClient) handleConfigTestRequest(request *WebSocketRequest) {
	data, _ := request.Body["data"].(map[string]interface{})
	if data == nil {
		log.Debugf("[config_test] request ID=%s Missing data field", request.ID)
		_ = c.SendResponse(request.ID, 400, nil, "Missing data field")
		return
	}
	testText, _ := request.Body["test_text"].(string)
	// Debug: count of each type config in request (excluding provider)
	log.Debugf("[config_test] request ID=%s test_text=%q data count by type: vad=%d asr=%d llm=%d tts=%d",
		request.ID, testText,
		countConfigKeys(data["vad"]), countConfigKeys(data["asr"]),
		countConfigKeys(data["llm"]), countConfigKeys(data["tts"]))

	type configTestResult struct {
		vad, asr, llm, tts map[string]interface{}
	}
	done := make(chan configTestResult, 1)
	go func() {
		vadR, asrR, llmR, ttsR := RunConfigTest(data, testText)
		done <- configTestResult{vadR, asrR, llmR, ttsR}
	}()

	var vadR, asrR, llmR, ttsR map[string]interface{}
	select {
	case res := <-done:
		vadR, asrR, llmR, ttsR = res.vad, res.asr, res.llm, res.tts
	case <-time.After(configTestTotalTimeout):
		log.Warnf("[config_test] request ID=%s total timeout %v", request.ID, configTestTotalTimeout)
		body := map[string]interface{}{
			"vad": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "config test total timeout"}},
			"asr": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "config test total timeout"}},
			"llm": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "config test total timeout"}},
			"tts": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "config test total timeout"}},
		}
		_ = c.SendResponse(request.ID, 200, body, "")
		return
	}

	// When request includes a type but no testable configs, return _none for frontend display
	fillEmptyConfigTestResult(data, "vad", vadR)
	fillEmptyConfigTestResult(data, "asr", asrR)
	fillEmptyConfigTestResult(data, "llm", llmR)
	fillEmptyConfigTestResult(data, "tts", ttsR)
	body := map[string]interface{}{
		"vad": vadR,
		"asr": asrR,
		"llm": llmR,
		"tts": ttsR,
	}
	log.Debugf("[config_test] response ID=%s result count by type: vad=%d asr=%d llm=%d tts=%d",
		request.ID, len(vadR), len(asrR), len(llmR), len(ttsR))
	_ = c.SendResponse(request.ID, 200, body, "")
}

// fillEmptyConfigTestResult when request includes this type but test result is empty, write _none entry
func fillEmptyConfigTestResult(data map[string]interface{}, typ string, result map[string]interface{}) {
	if _, has := data[typ]; !has || len(result) > 0 {
		return
	}
	msg := "no config or not enabled " + strings.ToUpper(typ)
	result["_none"] = map[string]interface{}{"ok": false, "message": msg}
	log.Debugf("[config_test] type %s no result, wrote _none: %s", typ, msg)
}

// countConfigKeys counts config entries excluding provider, used for debug
func countConfigKeys(v interface{}) int {
	m, ok := v.(map[string]interface{})
	if !ok {
		return 0
	}
	n := 0
	for k := range m {
		if k != "provider" {
			n++
		}
	}
	return n
}

// handleIncomingResponse processes received response
func (c *WebSocketClient) handleIncomingResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Errorf("Parse WebSocket response failed: %v", err)
		return
	}

	log.Debugf("Received response: ID=%s, Status=%d", response.ID, response.Status)

	// Find corresponding response channel and callback
	c.mu.RLock()
	responseChan, exists := c.responseChans[response.ID]
	callback, callbackExists := c.callbacks[response.ID]
	c.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Debugf("Response channel full, discard response: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Debugf("Received unknown response ID: %s", response.ID)
	}
}

// SendResponse sends response to received request
func (c *WebSocketClient) SendResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocketnotjoin")
	}

	response := WebSocketResponse{
		ID:     requestID,
		Status: status,
		Body:   body,
		Error:  errorMsg,
	}

	// Use write lock protection
	c.writeMu.Lock()
	err := c.conn.WriteJSON(response)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("send response failed: %v", err)
	}

	log.Debugf("Already sent response: ID=%s, Status=%d", requestID, status)
	return nil
}

// SetRequestHandler sets request handler
func (c *WebSocketClient) SetRequestHandler(handler func(*WebSocketRequest)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestHandler = handler
}

// mapToStruct auxiliary function: converts map to struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

func toolInfoToSchemaMap(paramsOneOf interface{}) map[string]interface{} {
	if paramsOneOf == nil {
		return nil
	}

	// ParamsOneOf internal field not exported, direct json.Marshal may get {}.
	// Priority: use official ToOpenAPIV3(), ensure can get real parameter schema.
	if p, ok := paramsOneOf.(*einoschema.ParamsOneOf); ok && p != nil {
		if openAPISchema, err := p.ToOpenAPIV3(); err == nil && openAPISchema != nil {
			raw, err := json.Marshal(openAPISchema)
			if err == nil {
				decoded := map[string]interface{}{}
				if err = json.Unmarshal(raw, &decoded); err == nil {
					if len(decoded) > 0 {
						return decoded
					}
				}
			}
		}
	}

	raw, err := json.Marshal(paramsOneOf)
	if err != nil {
		return nil
	}

	decoded := map[string]interface{}{}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}

	if openAPIV3, ok := decoded["openAPIV3"].(map[string]interface{}); ok {
		return openAPIV3
	}
	if openAPIV3, ok := decoded["open_api_v3"].(map[string]interface{}); ok {
		return openAPIV3
	}
	if len(decoded) == 0 {
		return nil
	}
	return decoded
}

func convertReportedToolsToToolList(reportedTools map[string]tool.InvokableTool) ([]map[string]interface{}, error) {
	toolList := make([]map[string]interface{}, 0)

	names := make([]string, 0, len(reportedTools))
	for name := range reportedTools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		invokable := reportedTools[name]
		toolInfo := map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("MCP tool: %s", name),
			"schema":      true,
		}

		if info, err := invokable.Info(context.Background()); err == nil && info != nil {
			if info.Desc != "" {
				toolInfo["description"] = info.Desc
			}
			inputSchema := toolInfoToSchemaMap(info.ParamsOneOf)
			if inputSchema != nil {
				toolInfo["input_schema"] = inputSchema
			}
		}

		toolList = append(toolList, toolInfo)
	}

	return toolList, nil
}

func getDeviceMcpTools(deviceID string) ([]map[string]interface{}, error) {
	reportedTools, err := mcp.GetReportedToolsByDeviceID(deviceID)
	if err != nil {
		log.Errorf("Failed to get device reported MCP tool list: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

func getAgentMcpTools(agentID string) ([]map[string]interface{}, error) {
	reportedTools, err := mcp.GetReportedToolsByAgentID(agentID)
	if err != nil {
		log.Errorf("Failed to get agent reported MCP tool list: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

// handleMcpToolListRequest processes MCP tool list request
func (c *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
	// Get agent_id/device_id from request body
	agentID := ""
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = id
		}
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if agentID == "" && deviceID == "" {
		log.Warnf("Received MCP tool list request, but missing agent_id/device_id")
		if err := c.SendResponse(request.ID, 400, nil, "Missingagent_idordevice_idparameter"); err != nil {
			log.Errorf("senderrorrespondfailed: %v", err)
		}
		return
	}

	log.Infof("Processing MCP tool list request, agent_id: %s, device_id: %s", agentID, deviceID)

	if agentID != "" && deviceID != "" {
		if err := c.SendResponse(request.ID, 400, nil, "agent_id and device_id cannot be provided at the same time"); err != nil {
			log.Errorf("senderrorrespondfailed: %v", err)
		}
		return
	}

	var (
		toolList []map[string]interface{}
		err      error
	)
	if deviceID != "" {
		toolList, err = getDeviceMcpTools(deviceID)
	} else {
		toolList, err = getAgentMcpTools(agentID)
	}
	if err != nil {
		log.Errorf("Failed to get MCP tool list: %v", err)
		if err := c.SendResponse(request.ID, 500, nil, fmt.Sprintf("gettoollistfailed: %v", err)); err != nil {
			log.Errorf("senderrorrespondfailed: %v", err)
		}
		return
	}

	// Construct response
	response := map[string]interface{}{
		"agent_id":  agentID,
		"device_id": deviceID,
		"tools":     toolList,
		"count":     len(toolList),
	}

	// Send response
	if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
		log.Errorf("Send MCP tool list response failed: %v", err)
	}
}

// Global convenience methods (async version)
func SendManagerRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	return GetDefaultClient().SendRequestAsync(ctx, method, path, body)
}

func SendManagerRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	return GetDefaultClient().SendRequestWithCallback(ctx, method, path, body, callback)
}

func GetManagerResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	return GetDefaultClient().GetResponse(requestID, timeout)
}

// Bidirectional communication support methods
func SetManagerRequestHandler(handler func(*WebSocketRequest)) {
	GetDefaultClient().SetRequestHandler(handler)
}

func SendManagerResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	return GetDefaultClient().SendResponse(requestID, status, body, errorMsg)
}

// Create client with request handler
func NewManagerClientWithHandler(handler func(*WebSocketRequest)) *WebSocketClient {
	return NewWebSocketClientWithHandler(handler)
}

// SendMcpToolListRequest sends MCP tool list request
func SendMcpToolListRequest(ctx context.Context, agentID string) (*WebSocketResponse, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequest(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestAsync sends MCP tool list request asynchronously
func SendMcpToolListRequestAsync(ctx context.Context, agentID string) (string, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestAsync(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestWithCallback sends MCP tool list request with callback
func SendMcpToolListRequestWithCallback(ctx context.Context, agentID string, callback func(*WebSocketResponse)) error {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestWithCallback(ctx, "GET", "/api/mcp/tools", body, callback)
}

// Init initializes Manager config provider
// Includes WebSocket connection initialization and reconnect mechanism
func Init(ctx context.Context) error {
	log.Infof("Initializing Manager config provider with WebSocket client")

	// createWebSocketclient-side
	client := GetDefaultClient()

	// tryjointoWebSocketserver
	if err := client.Connect(ctx); err != nil {
		log.Warnf("Initial connection to Manager WebSocket failed: %v, will start reconnect mechanism", err)
		// Even if initial connection failed, also start reconnect mechanism
		client.triggerReconnect()
	} else {
		log.Infof("Manager config provider initialized successfully")
	}

	return nil
}

// Close closes Manager config provider, cleanup resources
func Close() error {
	log.Infof("Closing Manager config provider")

	// Stop reconnect goroutine
	client := GetDefaultClient()
	client.StopReconnect()

	// Manually disconnect (not trigger reconnect)
	client.disconnect(true)

	return nil
}

// IsConnected checks if Manager config provider is already connected
func IsConnected() bool {
	return IsManagerWebSocketConnected()
}

// handleMcpToolCallRequest processes MCP tool call request
func (c *WebSocketClient) handleMcpToolCallRequest(request *WebSocketRequest) {
	agentID := ""
	deviceID := ""
	toolName := ""
	arguments := map[string]interface{}{}
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = id
		}
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
		if t, ok := request.Body["tool_name"].(string); ok {
			toolName = t
		}
		if args, ok := request.Body["arguments"].(map[string]interface{}); ok {
			arguments = args
		}
	}

	if toolName == "" || (agentID == "" && deviceID == "") {
		_ = c.SendResponse(request.ID, 400, nil, "Missingtool_nameoragent_id/device_idparameter")
		return
	}

	if agentID != "" && deviceID != "" {
		_ = c.SendResponse(request.ID, 400, nil, "agent_id and device_id cannot be provided at the same time")
		return
	}

	var (
		invokable tool.InvokableTool
		ok        bool
	)
	if deviceID != "" {
		invokable, ok = mcp.GetReportedToolByDeviceIDAndName(deviceID, toolName)
	} else {
		invokable, ok = mcp.GetReportedToolByAgentIDAndName(agentID, toolName)
	}
	if !ok {
		_ = c.SendResponse(request.ID, 404, nil, fmt.Sprintf("tool not found: %s", toolName))
		return
	}

	argBytes, _ := json.Marshal(arguments)
	result, err := invokable.InvokableRun(context.Background(), string(argBytes))
	if err != nil {
		_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("tool call failed: %v", err))
		return
	}

	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":  agentID,
		"device_id": deviceID,
		"tool_name": toolName,
		"result":    result,
	}, "")
}

func (c *WebSocketClient) handleOpenClawStatusRequest(request *WebSocketRequest) {
	agentID := ""
	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = strings.TrimSpace(id)
		}
	}
	if agentID == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing agent_id")
		return
	}

	manager := openclaw.GetManager()
	connected := manager.GetAgentSession(agentID) != nil
	status := "offline"
	if connected {
		status = "online"
	}

	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":  agentID,
		"connected": connected,
		"status":    status,
	}, "")
}

const (
	defaultOpenClawChatTimeoutMs = 10 * 60 * 1000
	minOpenClawChatTimeoutMs     = 1000
	maxOpenClawChatTimeoutMs     = 10 * 60 * 1000
	openClawChatTestSessionID    = "openclaw-chat-test-global"
)

func buildOpenClawTestDeviceID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		trimmed = "unknown"
	}
	return "__openclaw_test__:" + trimmed
}

func buildOpenClawTestSessionID() string {
	return openClawChatTestSessionID
}

func parseOpenClawTimeoutMs(v interface{}) int {
	timeout := defaultOpenClawChatTimeoutMs
	switch x := v.(type) {
	case int:
		timeout = x
	case int32:
		timeout = int(x)
	case int64:
		timeout = int(x)
	case float64:
		timeout = int(x)
	case float32:
		timeout = int(x)
	}
	if timeout < minOpenClawChatTimeoutMs {
		timeout = minOpenClawChatTimeoutMs
	}
	if timeout > maxOpenClawChatTimeoutMs {
		timeout = maxOpenClawChatTimeoutMs
	}
	return timeout
}

func parseOpenClawStreamEvents(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes", "on":
			return true
		}
	case int:
		return x != 0
	case int32:
		return x != 0
	case int64:
		return x != 0
	case float32:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

func openClawStreamSnippet(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func (c *WebSocketClient) handleOpenClawChatRequest(request *WebSocketRequest) {
	agentID := ""
	message := ""
	sessionID := ""
	timeoutMs := defaultOpenClawChatTimeoutMs
	streamEvents := false

	if request.Body != nil {
		if id, ok := request.Body["agent_id"].(string); ok {
			agentID = strings.TrimSpace(id)
		}
		if msg, ok := request.Body["message"].(string); ok {
			message = strings.TrimSpace(msg)
		}
		if rawSessionID, ok := request.Body["session_id"].(string); ok && strings.TrimSpace(rawSessionID) != "" {
			sessionID = strings.TrimSpace(rawSessionID)
		}
		timeoutMs = parseOpenClawTimeoutMs(request.Body["timeout_ms"])
		streamEvents = parseOpenClawStreamEvents(request.Body["stream_events"])
	}

	if agentID == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing agent_id")
		return
	}
	if message == "" {
		_ = c.SendResponse(request.ID, 400, nil, "missing message")
		return
	}
	if sessionID == "" {
		sessionID = buildOpenClawTestSessionID()
	}

	manager := openclaw.GetManager()
	if manager.GetAgentSession(agentID) == nil {
		_ = c.SendResponse(request.ID, 409, nil, fmt.Sprintf("openclaw session not connected for agent %s", agentID))
		return
	}

	testDeviceID := buildOpenClawTestDeviceID(agentID)
	// Cleanup test device history cache, avoid string to accumulate test results.
	manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
		return nil
	})

	start := time.Now()
	messageID, err := manager.SendMessage(agentID, testDeviceID, message, sessionID)
	if err != nil {
		errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(errMsg, "session not found") {
			_ = c.SendResponse(request.ID, 409, nil, fmt.Sprintf("openclaw session not connected for agent %s", agentID))
			return
		}
		_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("openclaw send failed: %v", err))
		return
	}
	if streamEvents {
		log.Infof(
			"openclaw chat stream started: request_id=%s agent=%s message_id=%s session=%s timeout_ms=%d",
			request.ID,
			agentID,
			messageID,
			sessionID,
			timeoutMs,
		)
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	var replyBuilder strings.Builder
	chunks := make([]string, 0, 8)
	done := false
	firstChunkLatencyMs := -1
	for time.Now().Before(deadline) {
		manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
			correlationID := strings.TrimSpace(msg.CorrelationID)
			if correlationID != "" && correlationID != messageID {
				return nil
			}
			chunk := strings.TrimSpace(msg.Text)
			if chunk != "" {
				replyBuilder.WriteString(chunk)
				chunks = append(chunks, chunk)
				if firstChunkLatencyMs < 0 {
					firstChunkLatencyMs = int(time.Since(start).Milliseconds())
				}
				if streamEvents {
					log.Infof(
						"openclaw chat stream chunk received: request_id=%s agent=%s message_id=%s chunk_index=%d chunk_len=%d chunk_snippet=%q",
						request.ID,
						agentID,
						messageID,
						len(chunks),
						len(chunk),
						openClawStreamSnippet(chunk, 64),
					)
				}
				if streamEvents {
					partialBody := map[string]interface{}{
						"agent_id":    agentID,
						"message_id":  messageID,
						"chunk":       chunk,
						"chunk_index": len(chunks),
						"reply":       strings.TrimSpace(replyBuilder.String()),
						"latency_ms":  int(time.Since(start).Milliseconds()),
						"done":        false,
					}
					if firstChunkLatencyMs >= 0 {
						partialBody["first_chunk_latency_ms"] = firstChunkLatencyMs
					}
					if err := c.SendResponse(request.ID, http.StatusPartialContent, partialBody, ""); err != nil {
						log.Warnf("openclaw chat stream partial response send failed: request_id=%s, err=%v", request.ID, err)
					}
				}
			}
			if msg.IsEnd {
				if streamEvents {
					log.Infof(
						"openclaw chat stream end marker received: request_id=%s agent=%s message_id=%s chunk_count=%d partial_reply_len=%d elapsed_ms=%d",
						request.ID,
						agentID,
						messageID,
						len(chunks),
						len(strings.TrimSpace(replyBuilder.String())),
						int(time.Since(start).Milliseconds()),
					)
				}
				done = true
			}
			return nil
		})
		if done {
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	reply := strings.TrimSpace(replyBuilder.String())

	if !done {
		// Cleanup test device offline cache, avoid accumulation.
		manager.ReplayOfflineMessages(testDeviceID, func(msg openclaw.OfflineMessage) error {
			return nil
		})
		if reply == "" {
			if streamEvents {
				log.Warnf(
					"openclaw chat stream timeout without reply: request_id=%s agent=%s message_id=%s timeout_ms=%d",
					request.ID,
					agentID,
					messageID,
					timeoutMs,
				)
			}
			_ = c.SendResponse(request.ID, 504, nil, "openclaw response timeout")
			return
		}
		if streamEvents {
			log.Warnf(
				"openclaw chat stream timeout with partial reply: request_id=%s agent=%s message_id=%s chunk_count=%d reply_len=%d elapsed_ms=%d",
				request.ID,
				agentID,
				messageID,
				len(chunks),
				len(reply),
				int(time.Since(start).Milliseconds()),
			)
		}
		_ = c.SendResponse(request.ID, 504, map[string]interface{}{
			"agent_id":               agentID,
			"message_id":             messageID,
			"reply":                  reply,
			"chunks":                 chunks,
			"chunk_count":            len(chunks),
			"latency_ms":             int(time.Since(start).Milliseconds()),
			"first_chunk_latency_ms": firstChunkLatencyMs,
			"timeout_ms":             timeoutMs,
			"finished":               false,
		}, "openclaw response timeout (partial reply received)")
		return
	}

	latencyMs := int(time.Since(start).Milliseconds())
	if streamEvents {
		log.Infof(
			"openclaw chat stream completed: request_id=%s agent=%s message_id=%s chunk_count=%d reply_len=%d latency_ms=%d",
			request.ID,
			agentID,
			messageID,
			len(chunks),
			len(reply),
			latencyMs,
		)
	}
	var firstChunkLatency interface{}
	if firstChunkLatencyMs >= 0 {
		firstChunkLatency = firstChunkLatencyMs
	}
	_ = c.SendResponse(request.ID, 200, map[string]interface{}{
		"agent_id":               agentID,
		"message_id":             messageID,
		"reply":                  reply,
		"chunks":                 chunks,
		"chunk_count":            len(chunks),
		"latency_ms":             latencyMs,
		"first_chunk_latency_ms": firstChunkLatency,
		"timeout_ms":             timeoutMs,
		"finished":               true,
	}, "")
}
