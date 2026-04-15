package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
	"gorm.io/gorm"

	"xiaozhi/manager/backend/models"
)

type WebSocketController struct {
	DB                *gorm.DB
	endpointAuthToken string
	upgrader          websocket.Upgrader
	clientsMap        cmap.ConcurrentMap[string, *WebSocketClient]
}

type WSClientClaims struct {
	Purpose string `json:"purpose"`
	UUID    string `json:"uuid"`
	jwt.RegisteredClaims
}

// WebSocketClient client connected to Manager Backend
type WebSocketClient struct {
	ID           string
	conn         *websocket.Conn
	controller   *WebSocketController
	requestChans map[string]chan *WebSocketResponse
	callbacks    map[string]func(*WebSocketResponse)
	mu           sync.RWMutex
	isConnected  bool
	stopChan     chan struct{} // Stop signal channel
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

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      bool                   `json:"schema"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

const (
	defaultBroadcastRequestTimeout = 30 * time.Second
	openClawChatDefaultTimeoutMs   = 10 * 60 * 1000
	openClawChatMinTimeoutMs       = 1000
	openClawChatMaxTimeoutMs       = 10 * 60 * 1000
)

// NewWebSocketController creates WebSocket controller
func NewWebSocketController(db *gorm.DB, endpointAuthToken string) *WebSocketController {
	return &WebSocketController{
		DB:                db,
		endpointAuthToken: strings.TrimSpace(endpointAuthToken),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins, production should restrict
			},
		},
		clientsMap: cmap.New[*WebSocketClient](),
	}
}

// HandleWebSocket handles WebSocket connection upgrade
func (ctrl *WebSocketController) HandleWebSocket(c *gin.Context) {
	tokenString := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(tokenString), "bearer ") {
		tokenString = strings.TrimSpace(tokenString[7:])
	}
	if tokenString == "" {
		tokenString = strings.TrimSpace(c.Query("token"))
	}
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing WebSocket authentication token"})
		return
	}

	claims, err := ctrl.parseWSClientToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid WebSocket authentication token"})
		return
	}
	if claims.Purpose != "manager-ws-client" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid WebSocket token purpose"})
		return
	}

	// Get UUID header
	clientUUID := c.GetHeader("UUID")
	if clientUUID == "" {
		log.Printf("WebSocket connection missing UUID header")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing UUID header"})
		return
	}
	if strings.TrimSpace(claims.UUID) != "" && strings.TrimSpace(claims.UUID) != strings.TrimSpace(clientUUID) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UUID does not match token"})
		return
	}

	// Upgrade HTTP connection to WebSocket connection
	conn, err := ctrl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Check if a connection with the same UUID already exists
	if existingClient, exists := ctrl.clientsMap.Get(clientUUID); exists {
		log.Printf("Disconnecting existing connection: %s", clientUUID)
		existingClient.conn.Close()
		existingClient.isConnected = false
	}

	// Create new client
	client := &WebSocketClient{
		ID:           clientUUID,
		conn:         conn,
		controller:   ctrl,
		requestChans: make(map[string]chan *WebSocketResponse),
		callbacks:    make(map[string]func(*WebSocketResponse)),
		isConnected:  true,
		stopChan:     make(chan struct{}),
	}

	// Store in clientsMap
	ctrl.clientsMap.Set(clientUUID, client)

	log.Printf("New WebSocket client connected: %s", clientUUID)

	// Start client message handling
	go client.handleMessages()

	// Start heartbeat detection
	go client.heartbeat()
}

func (ctrl *WebSocketController) parseWSClientToken(tokenString string) (*WSClientClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &WSClientClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(ctrl.endpointAuthToken), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*WSClientClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrInvalidKey
	}
	return claims, nil
}

// Remove client
func (ctrl *WebSocketController) removeClient(clientID string) {
	if client, exists := ctrl.clientsMap.Get(clientID); exists {
		// Send stop signal to heartbeat
		select {
		case client.stopChan <- struct{}{}:
			log.Printf("Stop signal sent to client: %s", clientID)
		default:
			// Channel may be full or closed, ignore
		}

		// Ensure client state is set correctly
		client.isConnected = false
		// Remove from map
		ctrl.clientsMap.Remove(clientID)
		log.Printf("WebSocket client disconnected: %s", clientID)
	}
}

// Get client by UUID
func (ctrl *WebSocketController) GetClient(uuid string) *WebSocketClient {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return client
	}
	return nil
}

// Check if client with specified UUID is connected
func (ctrl *WebSocketController) IsClientConnected(uuid string) bool {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return client.isConnected
	}
	return false
}

// GetFirstConnectedClientUUID returns the UUID of the first connected client, used for config testing and similar scenarios
func (ctrl *WebSocketController) GetFirstConnectedClientUUID() string {
	for item := range ctrl.clientsMap.IterBuffered() {
		if client := item.Val; client.isConnected {
			return client.ID
		}
	}
	return ""
}

// Send message to client with specified UUID
func (ctrl *WebSocketController) SendToClient(uuid string, message interface{}) error {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.conn.WriteJSON(message)
	}
	return fmt.Errorf("Client %s is not connected", uuid)
}

// Broadcast message to all connected clients
func (ctrl *WebSocketController) Broadcast(message interface{}) {
	for item := range ctrl.clientsMap.IterBuffered() {
		if client := item.Val; client.isConnected {
			if err := client.conn.WriteJSON(message); err != nil {
				log.Printf("Failed to broadcast message to client %s: %v", client.ID, err)
			}
		}
	}
}

// BroadcastSystemConfig pushes system config changes to all connected clients, format consistent with GET /api/system/configs: {"type":"system_config","data":{...}}
func (ctrl *WebSocketController) BroadcastSystemConfig(data gin.H) {
	ctrl.Broadcast(gin.H{"type": "system_config", "data": data})
}

// Client message handling
func (client *WebSocketClient) handleMessages() {
	defer func() {
		client.conn.Close()
		client.isConnected = false
		client.controller.removeClient(client.ID)
	}()

	for {
		if !client.isConnected {
			return
		}

		// Read message type
		messageType, reader, err := client.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		// Handle different message types
		switch messageType {
		case websocket.TextMessage:
			// Handle JSON message
			var rawMessage map[string]interface{}
			if err := json.NewDecoder(reader).Decode(&rawMessage); err != nil {
				log.Printf("Failed to parse JSON message: %v", err)
				continue
			}
			// Handle message
			client.handleMessage(rawMessage)

		case websocket.PingMessage:
			// Handle ping message, auto reply pong
			log.Printf("Received ping message, auto replying pong")
			if err := client.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				log.Printf("Failed to send pong: %v", err)
			}

		case websocket.PongMessage:
			// Handle pong message
			log.Printf("Received pong message")

		case websocket.CloseMessage:
			// Handle close message
			log.Printf("Received close message")
			return

		default:
			log.Printf("Received unknown WebSocket message type: %d", messageType)
		}
	}
}

// Handle received message
func (client *WebSocketClient) handleMessage(rawMessage map[string]interface{}) {
	// Check if it's a request message
	if method, exists := rawMessage["method"]; exists && method != nil {
		client.handleRequest(rawMessage)
		return
	}

	// Check if it's a response message
	if status, exists := rawMessage["status"]; exists && status != nil {
		client.handleResponse(rawMessage)
		return
	}

	log.Printf("Received unrecognized message: %+v", rawMessage)
}

// Handle request message
func (client *WebSocketClient) handleRequest(rawMessage map[string]interface{}) {
	var request WebSocketRequest
	if err := mapToStruct(rawMessage, &request); err != nil {
		log.Printf("Failed to parse request: %v", err)
		return
	}

	log.Printf("Received request: ID=%s, Method=%s, Path=%s", request.ID, request.Method, request.Path)

	// Process request and send response
	client.processRequest(&request)
}

// Handle response message
func (client *WebSocketClient) handleResponse(rawMessage map[string]interface{}) {
	var response WebSocketResponse
	if err := mapToStruct(rawMessage, &response); err != nil {
		log.Printf("Failed to parse response: %v", err)
		return
	}

	log.Printf("Received response: ID=%s, Status=%d", response.ID, response.Status)

	// Find corresponding response channel
	client.mu.RLock()
	responseChan, exists := client.requestChans[response.ID]
	callback, callbackExists := client.callbacks[response.ID]
	client.mu.RUnlock()

	if exists {
		select {
		case responseChan <- &response:
		default:
			log.Printf("Response channel is full, dropping response: %s", response.ID)
		}
	}

	if callbackExists {
		go callback(&response)
	}

	if !exists && !callbackExists {
		log.Printf("Received unknown response ID: %s", response.ID)
	}
}

// Process request
func (client *WebSocketClient) processRequest(request *WebSocketRequest) {
	switch request.Path {
	case "/api/server/info":
		client.handleServerInfoRequest(request)

	case "/api/server/ping":
		client.handlePingRequest(request)

	case "/api/device/active":
		client.handleDeviceActiveRequest(request)

	case "/api/device/inactive":
		client.handleDeviceInactiveRequest(request)

	default:
		log.Printf("Unknown request path: %s", request.Path)
		client.sendResponse(request.ID, 404, nil, "Unknown endpoint")
	}
}

// Handle server info request
func (client *WebSocketClient) handleServerInfoRequest(request *WebSocketRequest) {
	response := map[string]interface{}{
		"server_name": "xiaozhi-manager-backend",
		"version":     "1.0.0",
		"uptime":      time.Now().Format(time.RFC3339),
		"request_id":  request.ID,
		"client_id":   client.ID,
	}

	client.sendResponse(request.ID, 200, response, "")
}

// Handle ping request
func (client *WebSocketClient) handlePingRequest(request *WebSocketRequest) {
	response := map[string]interface{}{
		"message":   "pong from manager backend",
		"time":      time.Now().Format(time.RFC3339),
		"client_id": client.ID,
	}

	client.sendResponse(request.ID, 200, response, "")
}

// Handle device active time update request
func (client *WebSocketClient) handleDeviceActiveRequest(request *WebSocketRequest) {
	// Get device_id from request body
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if deviceID == "" {
		log.Printf("Received device active request but missing device_id")
		client.sendResponse(request.ID, 400, nil, "Missing device_id parameter")
		return
	}

	log.Printf("Processing device active time update request, device_id: %s", deviceID)

	// Update device last active time
	now := time.Now()
	result := client.controller.DB.Model(&models.Device{}).
		Where("device_name = ?", deviceID).
		Update("last_active_at", now)

	if result.Error != nil {
		log.Printf("Failed to update device active time: %v", result.Error)
		client.sendResponse(request.ID, 500, nil, fmt.Sprintf("Failed to update device active time: %v", result.Error))
		return
	}

	if result.RowsAffected == 0 {
		log.Printf("Device does not exist: %s", deviceID)
		client.sendResponse(request.ID, 404, nil, "Device does not exist")
		return
	}

	// Construct success response
	response := map[string]interface{}{
		"device_id":      deviceID,
		"last_active_at": now.Format(time.RFC3339),
		"message":        "Device active time updated successfully",
	}

	client.sendResponse(request.ID, 200, response, "")
	log.Printf("Device %s active time updated to: %s", deviceID, now.Format(time.RFC3339))
}

// Handle device offline request
func (client *WebSocketClient) handleDeviceInactiveRequest(request *WebSocketRequest) {
	// Get device_id from request body
	deviceID := ""
	if request.Body != nil {
		if id, ok := request.Body["device_id"].(string); ok {
			deviceID = id
		}
	}

	if deviceID == "" {
		log.Printf("Received device offline request but missing device_id")
		client.sendResponse(request.ID, 400, nil, "Missing device_id parameter")
		return
	}

	log.Printf("Processing device offline request, device_id: %s", deviceID)

	// Set device last active time to NULL (offline status)
	result := client.controller.DB.Model(&models.Device{}).
		Where("device_name = ?", deviceID).
		Update("last_active_at", nil) // Set to NULL to indicate offline

	if result.Error != nil {
		log.Printf("Failed to update device offline status: %v", result.Error)
		client.sendResponse(request.ID, 500, nil, fmt.Sprintf("Failed to update device offline status: %v", result.Error))
		return
	}

	if result.RowsAffected == 0 {
		log.Printf("Device does not exist: %s", deviceID)
		client.sendResponse(request.ID, 404, nil, "Device does not exist")
		return
	}

	// Construct success response
	response := map[string]interface{}{
		"device_id":      deviceID,
		"last_active_at": nil, // Offline status
		"message":        "Device offline status updated successfully",
	}

	client.sendResponse(request.ID, 200, response, "")
	log.Printf("Device %s has been set to offline status", deviceID)
}

// Send response
func (client *WebSocketClient) sendResponse(requestID string, status int, body map[string]interface{}, errorMsg string) {
	response := WebSocketResponse{
		ID:     requestID,
		Status: status,
		Body:   body,
		Error:  errorMsg,
	}

	if err := client.conn.WriteJSON(response); err != nil {
		log.Printf("Failed to send response: %v", err)
	} else {
		log.Printf("Response sent: ID=%s, Status=%d", requestID, status)
	}
}

// Heartbeat detection - using WebSocket native ping/pong
func (client *WebSocketClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Consecutive ping failure count
	pingFailCount := 0
	maxPingFailCount := 3 // Allow 3 consecutive failures

	for {
		select {
		case <-client.stopChan:
			log.Printf("Received stop signal, stopping heartbeat detection")
			return
		case <-ticker.C:
			if !client.isConnected {
				return
			}

			// Check if connection is still valid
			if client.conn == nil {
				log.Printf("WebSocket connection is nil, stopping heartbeat detection")
				return
			}

			// Send WebSocket native ping
			if err := client.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				pingFailCount++
				log.Printf("Failed to send ping (attempt %d): %v", pingFailCount, err)

				// Only disconnect after consecutive failures exceed threshold
				if pingFailCount >= maxPingFailCount {
					log.Printf("Ping failed %d consecutive times, disconnecting WebSocket", maxPingFailCount)
					client.conn.Close()
					return
				}
			} else {
				// Ping successful, reset failure count
				if pingFailCount > 0 {
					log.Printf("Ping recovered successfully, resetting failure count")
					pingFailCount = 0
				}
			}
		}
	}
}

// Send request to client (for active push)
func (client *WebSocketClient) SendRequest(method, path string, body map[string]interface{}) error {
	request := WebSocketRequest{
		ID:     uuid.New().String(),
		Method: method,
		Path:   path,
		Body:   body,
	}

	return client.conn.WriteJSON(request)
}

// Send request and wait for response
func (client *WebSocketClient) SendRequestWithResponse(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	requestID := uuid.New().String()

	request := WebSocketRequest{
		ID:     requestID,
		Method: method,
		Path:   path,
		Body:   body,
	}

	// Create response channel
	responseChan := make(chan *WebSocketResponse, 1)
	client.mu.Lock()
	client.requestChans[requestID] = responseChan
	client.mu.Unlock()

	// Clean up response channel
	defer func() {
		client.mu.Lock()
		delete(client.requestChans, requestID)
		client.mu.Unlock()
		close(responseChan)
	}()

	// Send request
	if err := client.conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}

	// Wait for response
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("Request timeout")
	case <-ctx.Done():
		return nil, fmt.Errorf("Context cancelled")
	}
}

// mapToStruct helper function: convert map to struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// Send request to client with specified UUID and wait for response
func (ctrl *WebSocketController) SendRequestToClient(ctx context.Context, uuid string, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.SendRequestWithResponse(ctx, method, path, body)
	}
	return nil, fmt.Errorf("Client %s is not connected", uuid)
}

// Request client MCP tools list (broadcast method, wait for first non-empty list response)
func (ctrl *WebSocketController) RequestMcpToolsFromClient(ctx context.Context, agentID string) ([]string, error) {
	toolDetails, err := ctrl.RequestMcpToolDetailsFromClient(ctx, agentID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, 0, len(toolDetails))
	for _, detail := range toolDetails {
		toolNames = append(toolNames, detail.Name)
	}

	return toolNames, nil
}

func (ctrl *WebSocketController) RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error) {
	log.Printf("Starting to request client MCP tools list, agentID: %s", agentID)
	return ctrl.requestMcpToolsByBody(ctx, map[string]interface{}{"agent_id": agentID})
}

// RequestDeviceMcpToolsFromClient requests device-level MCP tools list (broadcast method, wait for first non-empty list response)
func (ctrl *WebSocketController) RequestDeviceMcpToolsFromClient(ctx context.Context, deviceID string) ([]string, error) {
	toolDetails, err := ctrl.RequestDeviceMcpToolDetailsFromClient(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, 0, len(toolDetails))
	for _, detail := range toolDetails {
		toolNames = append(toolNames, detail.Name)
	}

	return toolNames, nil
}

func (ctrl *WebSocketController) RequestDeviceMcpToolDetailsFromClient(ctx context.Context, deviceID string) ([]MCPTool, error) {
	log.Printf("Starting to request device MCP tools list, deviceID: %s", deviceID)
	return ctrl.requestMcpToolsByBody(ctx, map[string]interface{}{"device_id": deviceID})
}

func (ctrl *WebSocketController) requestMcpToolsByBody(ctx context.Context, body map[string]interface{}) ([]MCPTool, error) {
	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "GET", "/api/mcp/tools", body)
	if err != nil {
		return nil, err
	}

	toolsData, ok := response.Body["tools"]
	if !ok {
		return []MCPTool{}, nil
	}

	tools := make([]MCPTool, 0)
	switch v := toolsData.(type) {
	case []interface{}:
		for _, item := range v {
			if toolStr, ok := item.(string); ok {
				tools = append(tools, MCPTool{Name: toolStr, Description: fmt.Sprintf("MCP Tool: %s", toolStr), Schema: true})
				continue
			}

			toolMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := toolMap["name"].(string)
			if name == "" {
				continue
			}

			description, _ := toolMap["description"].(string)
			if description == "" {
				description = fmt.Sprintf("MCP Tool: %s", name)
			}

			parsed := MCPTool{Name: name, Description: description, Schema: true}
			if inputSchema, ok := toolMap["input_schema"].(map[string]interface{}); ok {
				parsed.InputSchema = inputSchema
			} else if inputSchema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
				// Compatible with some clients returning camelCase field names
				parsed.InputSchema = inputSchema
			}
			tools = append(tools, parsed)
		}
	case []string:
		for _, name := range v {
			tools = append(tools, MCPTool{Name: name, Description: fmt.Sprintf("MCP Tool: %s", name), Schema: true})
		}
	}

	return tools, nil
}

// CallMcpToolFromClient requests client to execute MCP tool call
func (ctrl *WebSocketController) CallMcpToolFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "POST", "/api/mcp/call", body)
	if err != nil {
		return nil, err
	}

	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

// RequestOpenClawStatusFromClient requests client to return OpenClaw connection status
func (ctrl *WebSocketController) RequestOpenClawStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
	}

	response, err := ctrl.broadcastRequestAndWaitFirstSuccess(ctx, "GET", "/api/openclaw/status", body)
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

// CallOpenClawChatFromClient requests client to execute OpenClaw chat test
func (ctrl *WebSocketController) CallOpenClawChatFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	if body == nil {
		body = map[string]interface{}{}
	}
	timeoutMs := normalizeOpenClawChatTimeoutMs(body["timeout_ms"])
	body["timeout_ms"] = timeoutMs
	waitTimeout := time.Duration(timeoutMs)*time.Millisecond + 5*time.Second

	response, err := ctrl.broadcastRequestAndWaitFirstSuccessWithTimeout(ctx, "POST", "/api/openclaw/chat", body, waitTimeout)
	if err != nil {
		return nil, err
	}
	if response.Body == nil {
		return map[string]interface{}{}, nil
	}

	return response.Body, nil
}

type wsClientResponse struct {
	clientID string
	response *WebSocketResponse
}

// CallOpenClawChatStreamFromClient requests client to execute OpenClaw chat test (streaming callback)
func (ctrl *WebSocketController) CallOpenClawChatStreamFromClient(
	ctx context.Context,
	body map[string]interface{},
	onResponse func(*WebSocketResponse) error,
) (map[string]interface{}, error) {
	if body == nil {
		body = map[string]interface{}{}
	}
	timeoutMs := normalizeOpenClawChatTimeoutMs(body["timeout_ms"])
	body["timeout_ms"] = timeoutMs
	body["stream_events"] = true
	waitTimeout := time.Duration(timeoutMs)*time.Millisecond + 5*time.Second

	responseChan := make(chan wsClientResponse, 64)
	requestID := uuid.New().String()
	callbacksRegistered := 0

	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if !client.isConnected {
			continue
		}

		clientID := client.ID
		responseHandler := func(response *WebSocketResponse) {
			select {
			case responseChan <- wsClientResponse{clientID: clientID, response: response}:
			default:
				log.Printf("OpenClaw streaming response channel is full, dropping response: %s", requestID)
			}
		}

		client.mu.Lock()
		client.callbacks[requestID] = responseHandler
		client.mu.Unlock()
		callbacksRegistered++

		request := WebSocketRequest{
			ID:     requestID,
			Method: "POST",
			Path:   "/api/openclaw/chat",
			Body:   body,
		}
		if err := client.conn.WriteJSON(request); err != nil {
			log.Printf("Failed to send OpenClaw streaming request to client %s: %v", client.ID, err)
		}
	}

	if callbacksRegistered == 0 {
		return nil, fmt.Errorf("No connected clients")
	}

	defer func() {
		for item := range ctrl.clientsMap.IterBuffered() {
			client := item.Val
			client.mu.Lock()
			delete(client.callbacks, requestID)
			client.mu.Unlock()
		}
	}()

	selectedClientID := ""
	failedClients := map[string]bool{}
	firstError := ""
	timeout := time.After(waitTimeout)

	for {
		select {
		case event := <-responseChan:
			resp := event.response
			if resp == nil {
				continue
			}

			if selectedClientID == "" {
				if resp.Status >= http.StatusBadRequest {
					failedClients[event.clientID] = true
					if firstError == "" {
						msg := strings.TrimSpace(resp.Error)
						if msg != "" {
							firstError = msg
						}
					}
					if len(failedClients) >= callbacksRegistered {
						if firstError != "" {
							return nil, fmt.Errorf("%s", firstError)
						}
						return nil, fmt.Errorf("All clients returned failure")
					}
					continue
				}
				selectedClientID = event.clientID
			}

			if event.clientID != selectedClientID {
				continue
			}

			if onResponse != nil {
				if err := onResponse(resp); err != nil {
					return nil, err
				}
			}

			if resp.Status == http.StatusOK {
				if resp.Body == nil {
					return map[string]interface{}{}, nil
				}
				return resp.Body, nil
			}

			if resp.Status >= http.StatusBadRequest {
				msg := strings.TrimSpace(resp.Error)
				if msg == "" {
					msg = fmt.Sprintf("OpenClaw streaming request failed: status=%d", resp.Status)
				}
				return nil, fmt.Errorf("%s", msg)
			}
		case <-timeout:
			return nil, fmt.Errorf("Request timeout")
		case <-ctx.Done():
			return nil, fmt.Errorf("Context cancelled")
		}
	}
}

func (ctrl *WebSocketController) broadcastRequestAndWaitFirstSuccess(ctx context.Context, method, path string, body map[string]interface{}) (*WebSocketResponse, error) {
	return ctrl.broadcastRequestAndWaitFirstSuccessWithTimeout(ctx, method, path, body, defaultBroadcastRequestTimeout)
}

func normalizeOpenClawChatTimeoutMs(v interface{}) int {
	timeout := openClawChatDefaultTimeoutMs
	switch x := v.(type) {
	case int:
		timeout = x
	case int32:
		timeout = int(x)
	case int64:
		timeout = int(x)
	case float32:
		timeout = int(x)
	case float64:
		timeout = int(x)
	}

	if timeout < openClawChatMinTimeoutMs {
		timeout = openClawChatMinTimeoutMs
	}
	if timeout > openClawChatMaxTimeoutMs {
		timeout = openClawChatMaxTimeoutMs
	}
	return timeout
}

func (ctrl *WebSocketController) broadcastRequestAndWaitFirstSuccessWithTimeout(
	ctx context.Context,
	method, path string,
	body map[string]interface{},
	waitTimeout time.Duration,
) (*WebSocketResponse, error) {
	if waitTimeout <= 0 {
		waitTimeout = defaultBroadcastRequestTimeout
	}

	responseChan := make(chan *WebSocketResponse, 10)
	requestID := uuid.New().String()

	responseHandler := func(response *WebSocketResponse) {
		select {
		case responseChan <- response:
		default:
			log.Printf("Response channel is full, dropping response: %s", response.ID)
		}
	}

	callbacksRegistered := 0
	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if !client.isConnected {
			continue
		}

		client.mu.Lock()
		client.callbacks[requestID] = responseHandler
		client.mu.Unlock()
		callbacksRegistered++

		request := WebSocketRequest{ID: requestID, Method: method, Path: path, Body: body}
		if err := client.conn.WriteJSON(request); err != nil {
			log.Printf("Failed to send request to client %s: %v", client.ID, err)
		}
	}

	if callbacksRegistered == 0 {
		return nil, fmt.Errorf("No connected clients")
	}

	defer func() {
		for item := range ctrl.clientsMap.IterBuffered() {
			client := item.Val
			client.mu.Lock()
			delete(client.callbacks, requestID)
			client.mu.Unlock()
		}
	}()

	responsesReceived := 0
	firstError := ""
	timeout := time.After(waitTimeout)
	for {
		select {
		case response := <-responseChan:
			responsesReceived++
			if response != nil && response.Status == http.StatusOK {
				return response, nil
			}
			if response != nil && firstError == "" {
				msg := strings.TrimSpace(response.Error)
				if msg != "" {
					firstError = msg
				}
			}
			if responsesReceived >= callbacksRegistered {
				if firstError != "" {
					return nil, fmt.Errorf("%s", firstError)
				}
				return nil, fmt.Errorf("All clients returned failure")
			}
		case <-timeout:
			return nil, fmt.Errorf("Request timeout")
		case <-ctx.Done():
			return nil, fmt.Errorf("Context cancelled")
		}
	}
}

// Request client server info
func (ctrl *WebSocketController) RequestServerInfoFromClient(ctx context.Context, uuid string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/server/info", nil)
}

func (ctrl *WebSocketController) RequestDeviceActivation(ctx context.Context, uuid, deviceID string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/device/activation", map[string]interface{}{
		"device_id": deviceID,
	})
}

// Request client ping
func (ctrl *WebSocketController) RequestPingFromClient(ctx context.Context, uuid string) (*WebSocketResponse, error) {
	return ctrl.SendRequestToClient(ctx, uuid, "GET", "/api/server/ping", nil)
}

// InjectMessageToDevice injects message to device (broadcast method)
func (ctrl *WebSocketController) InjectMessageToDevice(ctx context.Context, deviceID, message string, skipLlm bool) error {
	body := map[string]interface{}{
		"device_id": deviceID,
		"message":   message,
		"skip_llm":  skipLlm,
	}

	// Create request
	request := WebSocketRequest{
		ID:     uuid.New().String(),
		Method: "POST",
		Path:   "/api/device/inject_msg",
		Body:   body,
	}

	// Broadcast to all connected clients
	var lastError error
	clientCount := 0

	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		if client.isConnected {
			clientCount++
			if err := client.conn.WriteJSON(request); err != nil {
				log.Printf("Failed to broadcast inject message to client %s: %v", client.ID, err)
				lastError = err
			} else {
				log.Printf("Successfully broadcast inject message to client %s", client.ID)
			}
		}
	}

	if clientCount == 0 {
		return fmt.Errorf("No connected clients")
	}

	return lastError
}

// Send request to client asynchronously (without waiting for response)
func (ctrl *WebSocketController) SendRequestToClientAsync(uuid string, method, path string, body map[string]interface{}) error {
	if client, exists := ctrl.clientsMap.Get(uuid); exists && client.isConnected {
		return client.SendRequest(method, path, body)
	}
	return fmt.Errorf("Client %s is not connected", uuid)
}

// Get all client connection status
func (ctrl *WebSocketController) GetClientConnectionStatus() map[string]interface{} {
	clients := make([]map[string]interface{}, 0)
	for item := range ctrl.clientsMap.IterBuffered() {
		client := item.Val
		clients = append(clients, map[string]interface{}{
			"uuid":      client.ID,
			"connected": client.isConnected,
		})
	}

	return map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	}
}

// GetClientStatus gets specified client connection status
func (ctrl *WebSocketController) GetClientStatus(uuid string) map[string]interface{} {
	if client, exists := ctrl.clientsMap.Get(uuid); exists {
		return map[string]interface{}{
			"uuid":      client.ID,
			"connected": client.isConnected,
			"message":   "Client is connected",
		}
	}

	return map[string]interface{}{
		"uuid":      uuid,
		"connected": false,
		"message":   "Client is not connected",
	}
}
