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
	requestHandler func(*WebSocketRequest) // processreceiveofrequest
	mu             sync.RWMutex
	writeMu        sync.Mutex // protectedWebSocketwrite操as，preventconcurrentwrite
	isConnected    bool
	connectMu      sync.Mutex
	messageQueue   chan *WebSocketRequest
	workers        sync.WaitGroup

	messageHandle cmap.ConcurrentMap[string, MessageHandleFunc]
	uuid          string

	// reconnectrelevantfield
	retryStopChan  chan struct{}  // reconnectgoroutinestopsignal
	retryWg        sync.WaitGroup // reconnectgoroutinewaitgroup
	retryMu        sync.Mutex     // protectedreconnectrelevant操as
	isRetrying     bool           // whetherisreconnect
	isShuttingDown bool           // whetherisclose（main动disconnect，noreconnect）
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

// SetSystemConfigPushHandler setreceive system_config pushwhenofcallback（mainprogramused formergeto viper etc），by user_config at Init when注入
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
	// priorityfromenvironmentvariableget，ifenvironmentvariableno存atthenfromconfigget
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

	// willHTTP URLconvertisWebSocket URL
	wsURL := "ws://" + c.baseURL[7:] + "/ws" // 去掉 "http://" andadd "/ws"
	wsToken, err := c.generateWSToken()
	if err != nil {
		return fmt.Errorf("generateWebSocketauthenticatetokenfailed: %v", err)
	}

	// 建立WebSocketjoin
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{c.baseURL},
		"UUID":   []string{c.uuid},
		"Authorization": []string{
			"Bearer " + wsToken,
		},
	})
	if err != nil {
		return fmt.Errorf("WebSocketjoinfailed: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	// setpingprocess器
	conn.SetPongHandler(func(appData string) error {
		log.Debugf("receivepongmessage")
		return nil
	})

	// startmessageprocessloop
	go c.handleMessages()

	// startmessagesend工asthread
	c.startWorkers()

	// start心跳detect
	go c.startHeartbeat()

	log.Debugf("WebSocketclient-sidealreadyjointo: %s", wsURL)
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

// disconnect internaldisconnect joinmethod
// manualDisconnect: trueindicatemain动disconnect（notriggerreconnect），falseindicateerrordisconnect（triggerreconnect）
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
			log.Debugf("closeWebSocketjoinwhenout错: %v", err)
		}
		c.conn = nil
	}

	c.isConnected = false
	c.mu.Lock()
	// closeallrespondchannel
	for _, ch := range c.responseChans {
		close(ch)
	}
	c.responseChans = make(map[string]chan *WebSocketResponse)
	c.callbacks = make(map[string]func(*WebSocketResponse))
	c.mu.Unlock()

	// stop工asthread
	close(c.messageQueue)
	c.workers.Wait()
	// recreatemessagequeue
	c.messageQueue = make(chan *WebSocketRequest, 100)

	log.Debugf("WebSocketjoinalreadydisconnect")
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
			return nil, fmt.Errorf("joinfailed: %v", err)
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
		return nil, fmt.Errorf("sendrequestfailed: %v", err)
	}

	// waitrespond
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(c.requestTimeout):
		return nil, fmt.Errorf("requesttimeout")
	case <-ctx.Done():
		return nil, fmt.Errorf("contextcancel")
	}
}

// 便捷method - useWebSocket原生ping
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

// global便捷method
func ConnectManagerWebSocket(ctx context.Context) error {
	return GetDefaultClient().Connect(ctx)
}

func DisconnectManagerWebSocket() error {
	client := GetDefaultClient()
	client.StopReconnect()
	return client.disconnect(true) // main动disconnect，notriggerreconnect
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

// startWorkers startmessagesend工asthread
func (c *WebSocketClient) startWorkers() {
	workerCount := 3 // start3个工asthread

	for i := 0; i < workerCount; i++ {
		c.workers.Add(1)
		go func(workerID int) {
			defer c.workers.Done()

			log.Debugf("Manager WebSocket工asthread %d alreadystart", workerID)

			for request := range c.messageQueue {
				if !c.IsConnected() {
					log.Debugf("工asthread %d: WebSocketnotjoin，discardrequest", workerID)
					continue
				}

				// sendrequest（usewritelockprotected）
				c.writeMu.Lock()
				err := c.conn.WriteJSON(request)
				c.writeMu.Unlock()
				if err != nil {
					log.Debugf("工asthread %d: sendrequestfailed: %v", workerID, err)
					// joinmayalreadydisconnect，triggerreconnect
					c.handleConnectionError()
					continue
				}

				log.Debugf("工asthread %d: alreadysendrequest %s", workerID, request.ID)
			}

			log.Debugf("Manager WebSocket工asthread %d alreadystop", workerID)
		}(i)
	}
}

// handleConnectionError processjoinerror
func (c *WebSocketClient) handleConnectionError() {
	if c.IsConnected() {
		log.Warn("detecttoWebSocketjoinerror，isdisconnect join...")
		c.disconnect(false) // errordisconnect，willtriggerreconnect
		// triggerreconnect
		c.triggerReconnect()
	}
}

// startHeartbeat start心跳detect
func (c *WebSocketClient) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second) // 每30secondsend aping
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
				log.Warnf("sendpingfailed，joinmayalreadydisconnect: %v", err)
				c.disconnect(false) // errordisconnect，willtriggerreconnect
				// triggerreconnect
				c.triggerReconnect()
				return
			}
			log.Debugf("sendpingmessagesuccessful")

		case <-c.retryStopChan:
			return
		}
	}
}

// triggerReconnect triggerreconnect（non-blocking）
func (c *WebSocketClient) triggerReconnect() {
	c.retryMu.Lock()
	defer c.retryMu.Unlock()

	// ifisclose，notriggerreconnect
	if c.isShuttingDown {
		log.Debug("isclosein，notriggerreconnect")
		return
	}

	// ifalreadyatreconnect，no重复trigger
	if c.isRetrying {
		return
	}

	c.isRetrying = true
	// startreconnectgoroutine
	c.retryWg.Add(1)
	go c.startReconnectLoop()
}

// startReconnectLoop startreconnectloop（use指countbackoffalgorithm）
func (c *WebSocketClient) startReconnectLoop() {
	defer func() {
		c.retryMu.Lock()
		c.isRetrying = false
		c.retryMu.Unlock()
		c.retryWg.Done()
	}()

	// 硬encodeofbackoffalgorithmparameter
	initialDelay := 3 * time.Second // initialdelay3second
	maxDelay := 1 * time.Minute     // maximumdelay1minute钟
	backoffMultiplier := 2.0        // backoff倍count

	delay := initialDelay
	retryCount := 0

	log.Infof("Manager WebSocketjoinretrygoroutinealreadystart")

	for {
		// check ifshouldstopreconnect
		select {
		case <-c.retryStopChan:
			log.Info("receivestopsignal，stopreconnect")
			return
		default:
		}

		// ifisclose，stopreconnect
		c.retryMu.Lock()
		shuttingDown := c.isShuttingDown
		c.retryMu.Unlock()
		if shuttingDown {
			log.Info("isclosein，stopreconnect")
			return
		}

		// ifalreadyjoin，stopreconnect
		if c.IsConnected() {
			log.Info("Manager WebSocketjoinalreadyrecovery，stopreconnect")
			return
		}

		retryCount++
		log.Warnf("Manager WebSocketjoinfailed (nth%dtimes)，wait %v afterretryjoin...", retryCount, delay)

		// waitdelaytime
		select {
		case <-time.After(delay):
			// continuereconnect
		case <-c.retryStopChan:
			log.Info("receivestopsignal，stopreconnect")
			return
		}

		// tryjoin
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Connect(ctx)
		cancel()

		if err != nil {
			log.Warnf("Manager WebSocketjoinfailed (nth%dtimes): %v", retryCount, err)
			// calculatedownatimesdelaytime（指countbackoff）
			delay = time.Duration(float64(delay) * backoffMultiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
			continue
		}

		// joinsuccessful
		log.Info("Manager WebSocketjoinsuccessful")
		return
	}
}

// StopReconnect stopreconnectgoroutine
func (c *WebSocketClient) StopReconnect() {
	c.retryMu.Lock()
	c.isShuttingDown = true
	shouldClose := c.retryStopChan != nil
	c.retryMu.Unlock()

	if shouldClose {
		// use select avoid重复closechannel
		select {
		case <-c.retryStopChan:
			// channelalreadyclose
		default:
			close(c.retryStopChan)
		}
		c.retryWg.Wait()
		log.Info("Manager WebSocketreconnectgoroutinealready优雅close")
	}
}

// SendRequestWithCallback sendrequestandusecallbackprocessrespond
func (c *WebSocketClient) SendRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return fmt.Errorf("joinfailed: %v", err)
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

	// registercallback
	c.mu.Lock()
	c.callbacks[requestID] = callback
	c.mu.Unlock()

	// cleanupcallback
	defer func() {
		c.mu.Lock()
		delete(c.callbacks, requestID)
		c.mu.Unlock()
	}()

	// willrequestplay入queue
	select {
	case c.messageQueue <- &request:
		log.Debugf("request %s alreadyadd toqueue", requestID)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("messagequeuealreadyfull，requesttimeout")
	case <-ctx.Done():
		return fmt.Errorf("contextcancel")
	}
}

// SendRequestAsync asynchronizationsendrequest
func (c *WebSocketClient) SendRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	if !c.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return "", fmt.Errorf("joinfailed: %v", err)
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

	// willrequestplay入queue
	select {
	case c.messageQueue <- &request:
		log.Debugf("asynchronizationrequest %s alreadyadd toqueue", requestID)
		return requestID, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("messagequeuealreadyfull，requesttimeout")
	case <-ctx.Done():
		return "", fmt.Errorf("contextcancel")
	}
}

// GetResponse getspecifyrequestIDofrespond（used forasynchronizationrequest）
func (c *WebSocketClient) GetResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	responseChan := make(chan *WebSocketResponse, 1)

	// register临whencallback
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
		return nil, fmt.Errorf("waitrespondtimeout")
	}
}

// handleSystemConfigPush processserver-sidepushofsystemconfigchange，asynchronizationcallalreadyregisterofcallback
func (c *WebSocketClient) handleSystemConfigPush(data map[string]interface{}) {
	if systemConfigPushHandler == nil {
		log.Debugf("receive system_config push，butnotregisterprocesscallback")
		return
	}
	go systemConfigPushHandler(data)
}

// handleMessages processreceivetoofWebSocketmessage
func (c *WebSocketClient) handleMessages() {
	for {
		if !c.isConnected {
			return
		}

		// readcancel息type
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

		// processnoat the same timetypeofmessage
		switch messageType {
		case websocket.TextMessage:
			// processJSONmessage
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Errorf("parseJSONmessagefailed: %v", err)
				continue
			}

			// according tomessagetypejudge：server-sidepush(system_config)、request、respond
			if msgType, _ := rawMessage["type"].(string); msgType == "system_config" {
				if data, ok := rawMessage["data"].(map[string]interface{}); ok {
					c.handleSystemConfigPush(data)
				} else {
					log.Warnf("receive system_config pushbut data formatinvalid")
				}
			} else if method, exists := rawMessage["method"]; exists && method != nil {
				// 这yesreceiveofrequest
				c.handleIncomingRequest(rawMessage)
			} else if status, exists := rawMessage["status"]; exists && status != nil {
				// 这yesreceiveofrespond
				c.handleIncomingResponse(rawMessage)
			} else {
				log.Warnf("receiveno法recognizeofWebSocketmessage: %+v", rawMessage)
			}

		case websocket.PingMessage:
			// processpingmessage，automatic回复pong（usewritelockprotected）
			log.Debugf("receivepingmessage，automatic回复pong")
			c.writeMu.Lock()
			err := c.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
			c.writeMu.Unlock()
			if err != nil {
				log.Errorf("sendpongfailed: %v", err)
			}

		case websocket.PongMessage:
			// processpongmessage
			log.Debugf("receivepongmessage")

		case websocket.CloseMessage:
			// processclosemessage
			log.Debugf("receiveclosemessage")
			c.disconnect(false) // errordisconnect，willtriggerreconnect
			// triggerreconnect
			c.triggerReconnect()
			return

		default:
			log.Warnf("receivenot知typeofWebSocketmessage: %d", messageType)
		}
	}
}

// handleIncomingRequest processreceiveofrequest
func (c *WebSocketClient) handleIncomingRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Errorf("parseWebSocketrequestfailed: %v", err)
		return
	}

	log.Debugf("receiverequest: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// ifhaveregisterofrequestprocess器，callit
	if c.requestHandler != nil {
		go c.requestHandler(&request)
	} else {
		// ifnoregisterprocess器，usedefaultprocess器processalready知path
		c.handleDefaultRequest(&request)
	}
}

func (c *WebSocketClient) RegisterMessageHandler(ctx context.Context, path string, handler types.EventHandler) {
	f := func(request *WebSocketRequest) (string, error) {
		return handler(ctx, request.Path, request.Body)
	}
	c.messageHandle.Set(path, f)
}

// handleDefaultRequest defaultrequestprocess器
func (c *WebSocketClient) handleDefaultRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/config/test":
		// configtestmayrelativelytime consumption（VAD/ASR/LLM/TTS serialexecute），play入independent goroutine avoidblockreadloop，support多requestconcurrent
		go c.handleConfigTestRequest(request)

	case "/api/mcp/tools":
		// processMCPtoollistrequest
		c.handleMcpToolListRequest(request)

	case "/api/mcp/call":
		// processMCPtoolcallrequest
		c.handleMcpToolCallRequest(request)

	case "/api/openclaw/status":
		c.handleOpenClawStatusRequest(request)

	case "/api/openclaw/chat":
		c.handleOpenClawChatRequest(request)

	case "/api/server/info":
		// returnserverinfo
		response := map[string]interface{}{
			"server_name": "xiaozhi-server",
			"version":     "1.0.0",
			"uptime":      time.Now().Format(time.RFC3339),
			"request_id":  request.ID,
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("sendserverinforespondfailed: %v", err)
		}

	case "/api/server/ping":
		// 简单ofpingrespond
		response := map[string]interface{}{
			"message": "pong from server",
			"time":    time.Now().Format(time.RFC3339),
		}

		if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
			log.Errorf("sendpingrespondfailed: %v", err)
		}
	default:
		handler, exists := c.messageHandle.Get(request.Path)
		if exists {
			// callprocess器andprocessreturnvalue
			result, err := handler(request)
			if err != nil {
				log.Errorf("processrequest %s failed: %v", request.Path, err)
				// senderrorrespond
				if err := c.SendResponse(request.ID, 500, nil, err.Error()); err != nil {
					log.Errorf("senderrorrespondfailed: %v", err)
				}
			} else {
				// sendsuccessfulrespond
				response := map[string]interface{}{
					"result": result,
				}
				if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
					log.Errorf("sendsuccessfulrespondfailed: %v", err)
				}
			}
		} else {
			log.Warnf("receivenot知ofWebSocketrequestpath: %s, ID: %s", request.Path, request.ID)

			// send404respond
			if err := c.SendResponse(request.ID, 404, nil, "Unknown endpoint"); err != nil {
				log.Errorf("senderrorrespondfailed: %v", err)
			}
		}
	}
}

// configTestTotalTimeout configtestbodybodytimeout（VAD+ASR+LLM+TTS 合计）
const configTestTotalTimeout = 90 * time.Second

// handleConfigTestRequest processconfigtestrequest：VAD/ASR/LLM/TTS usedown发ofconfigandfixed WAV/textexecute轻amounttest
func (c *WebSocketClient) handleConfigTestRequest(request *WebSocketRequest) {
	data, _ := request.Body["data"].(map[string]interface{})
	if data == nil {
		log.Debugf("[config_test] request ID=%s Missing data field", request.ID)
		_ = c.SendResponse(request.ID, 400, nil, "Missing data field")
		return
	}
	testText, _ := request.Body["test_text"].(string)
	// debug: requestin各typeconfigcount（no含 provider）
	log.Debugf("[config_test] request ID=%s test_text=%q data 各type条目count: vad=%d asr=%d llm=%d tts=%d",
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
		log.Warnf("[config_test] request ID=%s bodybodytimeout %v", request.ID, configTestTotalTimeout)
		body := map[string]interface{}{
			"vad": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "configtest总timeout"}},
			"asr": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "configtest总timeout"}},
			"llm": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "configtest总timeout"}},
			"tts": map[string]interface{}{"_error": map[string]interface{}{"ok": false, "message": "configtest总timeout"}},
		}
		_ = c.SendResponse(request.ID, 200, body, "")
		return
	}

	// requestin带某typebutno任何可测configwhen，return _none 便于beforeendpoint展示reason
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
	log.Debugf("[config_test] respond ID=%s 各typeresultcount: vad=%d asr=%d llm=%d tts=%d",
		request.ID, len(vadR), len(asrR), len(llmR), len(ttsR))
	_ = c.SendResponse(request.ID, 200, body, "")
}

// fillEmptyConfigTestResult whenrequestincludethistypebuttestresultisemptywhen，write _none 条目
func fillEmptyConfigTestResult(data map[string]interface{}, typ string, result map[string]interface{}) {
	if _, has := data[typ]; !has || len(result) > 0 {
		return
	}
	msg := "notconfigornot启use" + strings.ToUpper(typ)
	result["_none"] = map[string]interface{}{"ok": false, "message": msg}
	log.Debugf("[config_test] type %s noresult，alreadywrite _none: %s", typ, msg)
}

// countConfigKeys count data in除 provider outsideof config 条目count，used for debug
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

// handleIncomingResponse processreceiveofrespond
func (c *WebSocketClient) handleIncomingResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Errorf("parseWebSocketrespondfailed: %v", err)
		return
	}

	log.Debugf("receiverespond: ID=%s, Status=%d", response.ID, response.Status)

	// findcorrespondingrespondchannelandcallback
	c.mu.RLock()
	responseChan, exists := c.responseChans[response.ID]
	callback, callbackExists := c.callbacks[response.ID]
	c.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Debugf("respondchannelalreadyfull，discardrespond: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Debugf("receivenot知ofrespondID: %s", response.ID)
	}
}

// SendResponse sendrespond给receiveofrequest
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

	// usewritelockprotected
	c.writeMu.Lock()
	err := c.conn.WriteJSON(response)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("sendrespondfailed: %v", err)
	}

	log.Debugf("alreadysendrespond: ID=%s, Status=%d", requestID, status)
	return nil
}

// SetRequestHandler setrequestprocess器
func (c *WebSocketClient) SetRequestHandler(handler func(*WebSocketRequest)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestHandler = handler
}

// mapToStruct auxiliaryfunction：willmapconvertisstruct
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

	// ParamsOneOf internalfieldnotexport，direct json.Marshal may得to {}。
	// priority走官方 ToOpenAPIV3()，ensure能取torealparameter schema。
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
			"description": fmt.Sprintf("MCPtool: %s", name),
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
		log.Errorf("getdeviceup报MCPtoollistfailed: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

func getAgentMcpTools(agentID string) ([]map[string]interface{}, error) {
	reportedTools, err := mcp.GetReportedToolsByAgentID(agentID)
	if err != nil {
		log.Errorf("getagentup报MCPtoollistfailed: %v", err)
		return nil, err
	}

	return convertReportedToolsToToolList(reportedTools)
}

// handleMcpToolListRequest processMCPtoollistrequest
func (c *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
	// fromrequestbodyingetagent_id/device_id
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
		log.Warnf("receiveMCPtoollistrequest，butMissingagent_id/device_id")
		if err := c.SendResponse(request.ID, 400, nil, "Missingagent_idordevice_idparameter"); err != nil {
			log.Errorf("senderrorrespondfailed: %v", err)
		}
		return
	}

	log.Infof("processMCPtoollistrequest，agent_id: %s, device_id: %s", agentID, deviceID)

	if agentID != "" && deviceID != "" {
		if err := c.SendResponse(request.ID, 400, nil, "agent_idanddevice_idcannotat the same timewhen传入"); err != nil {
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
		log.Errorf("getMCPtoollistfailed: %v", err)
		if err := c.SendResponse(request.ID, 500, nil, fmt.Sprintf("gettoollistfailed: %v", err)); err != nil {
			log.Errorf("senderrorrespondfailed: %v", err)
		}
		return
	}

	// constructrespond
	response := map[string]interface{}{
		"agent_id":  agentID,
		"device_id": deviceID,
		"tools":     toolList,
		"count":     len(toolList),
	}

	// sendrespond
	if err := c.SendResponse(request.ID, 200, response, ""); err != nil {
		log.Errorf("sendMCPtoollistrespondfailed: %v", err)
	}
}

// global便捷method（asynchronizationversion）
func SendManagerRequestAsync(ctx context.Context, method, path string, body map[string]interface{}) (string, error) {
	return GetDefaultClient().SendRequestAsync(ctx, method, path, body)
}

func SendManagerRequestWithCallback(ctx context.Context, method, path string, body map[string]interface{}, callback func(*WebSocketResponse)) error {
	return GetDefaultClient().SendRequestWithCallback(ctx, method, path, body, callback)
}

func GetManagerResponse(requestID string, timeout time.Duration) (*WebSocketResponse, error) {
	return GetDefaultClient().GetResponse(requestID, timeout)
}

// dualto通信supportmethod
func SetManagerRequestHandler(handler func(*WebSocketRequest)) {
	GetDefaultClient().SetRequestHandler(handler)
}

func SendManagerResponse(requestID string, status int, body map[string]interface{}, errorMsg string) error {
	return GetDefaultClient().SendResponse(requestID, status, body, errorMsg)
}

// create带haverequestprocess器ofclient-side
func NewManagerClientWithHandler(handler func(*WebSocketRequest)) *WebSocketClient {
	return NewWebSocketClientWithHandler(handler)
}

// SendMcpToolListRequest sendMCPtoollistrequest
func SendMcpToolListRequest(ctx context.Context, agentID string) (*WebSocketResponse, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequest(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestAsync asynchronizationsendMCPtoollistrequest
func SendMcpToolListRequestAsync(ctx context.Context, agentID string) (string, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestAsync(ctx, "GET", "/api/mcp/tools", body)
}

// SendMcpToolListRequestWithCallback usecallbacksendMCPtoollistrequest
func SendMcpToolListRequestWithCallback(ctx context.Context, agentID string, callback func(*WebSocketResponse)) error {
	body := map[string]interface{}{
		"agent_id": agentID,
	}
	return SendManagerRequestWithCallback(ctx, "GET", "/api/mcp/tools", body, callback)
}

// Init initializeManagerconfigprovide者
// package括WebSocketjoinofinitializeandreconnectmechanism
func Init(ctx context.Context) error {
	log.Infof("Initializing Manager config provider with WebSocket client")

	// createWebSocketclient-side
	client := GetDefaultClient()

	// tryjointoWebSocketserver
	if err := client.Connect(ctx); err != nil {
		log.Warnf("initialjoinManager WebSocketfailed: %v，willstartreconnectmechanism", err)
		// even ifinitialjoinfailed，alsostartreconnectmechanism
		client.triggerReconnect()
	} else {
		log.Infof("Manager config provider initialized successfully")
	}

	return nil
}

// Close closeManagerconfigprovide者，cleanupresource
func Close() error {
	log.Infof("Closing Manager config provider")

	// stopreconnectgoroutine
	client := GetDefaultClient()
	client.StopReconnect()

	// main动disconnect join（notriggerreconnect）
	client.disconnect(true)

	return nil
}

// IsConnected inspectManagerconfigprovide者whetheralreadyjoin
func IsConnected() bool {
	return IsManagerWebSocketConnected()
}

// handleMcpToolCallRequest processMCPtoolcallrequest
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
		_ = c.SendResponse(request.ID, 400, nil, "agent_idanddevice_idcannotat the same timewhen传入")
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
		_ = c.SendResponse(request.ID, 404, nil, fmt.Sprintf("toolno存at: %s", toolName))
		return
	}

	argBytes, _ := json.Marshal(arguments)
	result, err := invokable.InvokableRun(context.Background(), string(argBytes))
	if err != nil {
		_ = c.SendResponse(request.ID, 500, nil, fmt.Sprintf("toolcallfailed: %v", err))
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
	// cleanuptestdevicehistorycache，avoidstringtoupa轮testresult。
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
		// cleanuptestdevice离线cache，avoidaccumulate。
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
