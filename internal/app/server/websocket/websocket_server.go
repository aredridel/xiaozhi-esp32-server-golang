package websocket

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	"xiaozhi-esp32-server-golang/internal/app/server/types"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/openclaw"
	log "xiaozhi-esp32-server-golang/logger"
)

// WebSocketServer represents WebSocket server
type WebSocketServer struct {
	// Config upgrader
	upgrader websocket.Upgrader
	// Client states, using sync.Map for concurrency safety
	clientStates sync.Map
	// Auth manager
	authManager *auth.AuthManager
	// Port
	port int
	// MCP manager
	globalMCPManager *mcp.GlobalMCPManager

	onNewConnection    types.OnNewConnection
	onOpenClawResponse func(event openclaw.ResponseDelivery) bool
}

// Option type definition
// WebSocketServerOption is used for configuring optional parameters of WebSocketServer
type WebSocketServerOption func(*WebSocketServer)

// WithAuthManager sets the auth manager
func WithAuthManager(authManager *auth.AuthManager) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.authManager = authManager
	}
}

// WithMCPManager sets the MCP manager
func WithMCPManager(mcpManager *mcp.GlobalMCPManager) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.globalMCPManager = mcpManager
	}
}

func WithOnNewConnection(onNewConnection types.OnNewConnection) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.onNewConnection = onNewConnection
	}
}

func WithOnOpenClawResponse(handler func(event openclaw.ResponseDelivery) bool) WebSocketServerOption {
	return func(s *WebSocketServer) {
		s.onOpenClawResponse = handler
	}
}

// NewWebSocketServer create new WebSocket server (WithOption pattern)
func NewWebSocketServer(port int, opts ...WebSocketServerOption) *WebSocketServer {
	s := &WebSocketServer{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from all origins
			},
		},
		// default values
		authManager:      auth.A(),
		port:             port,
		globalMCPManager: mcp.GetGlobalMCPManager(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start start WebSocket server
func (s *WebSocketServer) Start() error {
	// Start all MCP managers (via unified manager)
	if err := mcp.StartMCPManagers(); err != nil {
		log.Errorf("Failed to start MCP manager cluster: %v", err)
		return err
	}

	// Start session cleanup
	go s.cleanupSessions()

	// Register route handlers
	http.HandleFunc("/xiaozhi/mqtt_udp/v1/", s.handleMqttUdpChat)
	http.HandleFunc("/xiaozhi/v1/", s.handleChat)
	http.HandleFunc("/xiaozhi/ota/", s.handleOta)
	http.HandleFunc("/xiaozhi/ota/activate", s.handleOtaActivate)
	http.HandleFunc("/mcp", s.handleMCPWebSocket)
	http.HandleFunc("/ws/openclaw", s.handleOpenClawWebSocket)
	http.HandleFunc("/xiaozhi/api/mcp/tools/", s.handleMCPAPI)
	http.HandleFunc("/xiaozhi/api/vision", s.handleVisionAPI) // Image recognition API

	http.HandleFunc("/admin/inject_msg", s.handleInjectMsg)

	listenAddr := fmt.Sprintf("0.0.0.0:%d", s.port)
	log.Infof("WebSocket server started at ws://%s/xiaozhi/v1/", listenAddr)
	log.Infof("MCP WebSocket endpoint: ws://%s/mcp?token=xxx", listenAddr)
	log.Infof("OpenClaw WebSocket endpoint: ws://%s/ws/openclaw?token=xxx", listenAddr)
	log.Infof("MCP API endpoint: http://%s/xiaozhi/api/mcp/tools/{deviceId}", listenAddr)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Log().Fatalf("WebSocket server startup failed: %v", err)
		return err
	}
	return nil
}

// handleGetDeviceTools gets the device's tool list
func (s *WebSocketServer) handleGetDeviceTools(w http.ResponseWriter, r *http.Request, deviceID string) {

}

// cleanupSessions periodically cleans up expired sessions
func (s *WebSocketServer) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.authManager.CleanupSessions(30 * time.Minute)
	}
}

// handleWebSocket process WebSocket join
func (s *WebSocketServer) handleChat(w http.ResponseWriter, r *http.Request) {
	s.internalHandleChat(w, r, false)
}

// handleWebSocket process WebSocket join
func (s *WebSocketServer) handleMqttUdpChat(w http.ResponseWriter, r *http.Request) {
	s.internalHandleChat(w, r, true)
}

// handleWebSocket process WebSocket join
func (s *WebSocketServer) internalHandleChat(w http.ResponseWriter, r *http.Request, isMqttUdp bool) {
	deviceID, clientID := extractDeviceAndClientID(r)
	if deviceID == "" {
		log.Warn("Missing device-id, please pass it via Header or URL parameter")
		http.Error(w, "Missing device-id (supports Header or URL parameter)", http.StatusBadRequest)
		return
	}
	if clientID == "" {
		log.Debugf("连接未提供 client-id, device_id: %s", deviceID)
	}

	/*isAuth := viper.GetBool("auth.enable")
	if isAuth {
		token := r.Header.Get("Authorization")
		if token == "" {
			log.Warn("Missing Authorization request header")
			http.Error(w, "Missing Authorization request header", http.StatusUnauthorized)
			return
		}

		// validate token
		if !s.authManager.ValidateToken(token) {
			log.Warnf("invalid token: %s", token)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}*/

	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	// Adapt to IConn interface
	wsConn := NewWebSocketConn(conn, deviceID, isMqttUdp)
	if s.onNewConnection != nil {
		s.onNewConnection(wsConn)
	}

}

func extractDeviceAndClientID(r *http.Request) (string, string) {
	deviceKeys := []string{"Device-Id", "device-id", "DEVICE-ID", "device_id", "Device_Id", "deviceId"}
	clientKeys := []string{"Client-Id", "client-id", "CLIENT-ID", "client_id", "Client_Id", "clientId"}

	headerDeviceID, headerDeviceKey := findHeaderValue(r.Header, deviceKeys)
	queryDeviceID, queryDeviceKey := findQueryValue(r.URL.Query(), deviceKeys)
	headerClientID, headerClientKey := findHeaderValue(r.Header, clientKeys)
	queryClientID, queryClientKey := findQueryValue(r.URL.Query(), clientKeys)

	deviceID := headerDeviceID
	if deviceID == "" {
		deviceID = queryDeviceID
	} else if queryDeviceID != "" && queryDeviceID != headerDeviceID {
		log.Warnf("device-id mismatch between Header(%s) and URL parameter(%s), using Header value", headerDeviceKey, queryDeviceKey)
	}

	clientID := headerClientID
	if clientID == "" {
		clientID = queryClientID
	} else if queryClientID != "" && queryClientID != headerClientID {
		log.Warnf("client-id mismatch between Header(%s) and URL parameter(%s), using Header value", headerClientKey, queryClientKey)
	}

	return deviceID, clientID
}

func findHeaderValue(header http.Header, keys []string) (string, string) {
	for _, key := range keys {
		if value := header.Get(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func findQueryValue(values url.Values, keys []string) (string, string) {
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func (s *WebSocketServer) handleInjectMsg(w http.ResponseWriter, r *http.Request) {

}
