package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"

	log "xiaozhi-esp32-server-golang/logger"
)

// MCPServerConfig MCP server config
type MCPServerConfig struct {
	Name         string            `json:"name" mapstructure:"name"`
	Type         string            `json:"type" mapstructure:"type"`
	Url          string            `json:"url" mapstructure:"url"`
	SSEUrl       string            `json:"sse_url" mapstructure:"sse_url"` // backward compatible sse_url field
	Enabled      bool              `json:"enabled" mapstructure:"enabled"`
	Provider     string            `json:"provider,omitempty" mapstructure:"provider"`
	ServiceID    string            `json:"service_id,omitempty" mapstructure:"service_id"`
	AuthRef      string            `json:"auth_ref,omitempty" mapstructure:"auth_ref"`
	Headers      map[string]string `json:"headers,omitempty" mapstructure:"headers"`
	AllowedTools []string          `json:"allowed_tools,omitempty" mapstructure:"allowed_tools"`
}

// GlobalMCPManager global MCP manager
type GlobalMCPManager struct {
	servers       map[string]*MCPServerConnection
	tools         map[string]tool.InvokableTool
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	reconnectConf ReconnectConfig
	httpClient    *http.Client
}

// ReconnectConfig reconnect config
type ReconnectConfig struct {
	Interval    time.Duration
	MaxAttempts int
}

// MCPServerConnection MCP server connection
type MCPServerConnection struct {
	config        MCPServerConfig
	client        *client.Client
	tools         map[string]tool.InvokableTool
	connected     bool
	mu            sync.RWMutex
	lastError     error
	retryCount    int
	lastPing      time.Time
	reconnecting  bool
	reconnectWait chan struct{}
}

var (
	globalManager *GlobalMCPManager
	once          sync.Once
)

// GetGlobalMCPManager gets global MCP manager singleton
func GetGlobalMCPManager() *GlobalMCPManager {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalManager = &GlobalMCPManager{
			servers: make(map[string]*MCPServerConnection),
			tools:   make(map[string]tool.InvokableTool),
			ctx:     ctx,
			cancel:  cancel,
			reconnectConf: ReconnectConfig{
				Interval:    time.Duration(viper.GetInt("mcp.global.reconnect_interval")) * time.Second,
				MaxAttempts: viper.GetInt("mcp.global.max_reconnect_attempts"),
			},
			httpClient: &http.Client{
				Timeout: 600 * time.Second,
			},
		}
	})
	return globalManager
}

// Start starts global MCP manager
func (g *GlobalMCPManager) Start() error {
	// hot-reload scenario: ctx is cancelled after Stop, need to rebuild for restart monitoring and reconnection
	if g.ctx != nil && g.ctx.Err() != nil {
		g.ctx, g.cancel = context.WithCancel(context.Background())
		g.reconnectConf = ReconnectConfig{
			Interval:    time.Duration(viper.GetInt("mcp.global.reconnect_interval")) * time.Second,
			MaxAttempts: viper.GetInt("mcp.global.max_reconnect_attempts"),
		}
	}

	// first check config
	CheckMCPConfig()

	if !viper.GetBool("mcp.global.enabled") {
		log.Info("global MCP manager is disabled")
		return nil
	}

	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("failed to parse MCP server config: %v", err)
		return fmt.Errorf("failed to parse MCP server config: %v", err)
	}

	log.Infof("read %d MCP server configs from config", len(serverConfigs))

	// log each server config in detail
	for i, config := range serverConfigs {
		log.Infof("MCP server[%d]: Type=%s, Name=%s, Url=%s, SSEUrl=%s, Enabled=%v",
			i+1, config.Type, config.Name, config.Url, config.SSEUrl, config.Enabled)
	}

	// connect to enabled servers
	connectedCount := 0
	for _, config := range serverConfigs {
		if config.Enabled {
			if err := g.connectToServer(config); err != nil {
				log.Errorf("failed to connect to MCP server %s: %v", config.Name, err)
			} else {
				connectedCount++
			}
		} else {
			log.Infof("MCP server %s is disabled, skipping connection", config.Name)
		}
	}

	log.Infof("successfully connected to %d MCP servers", connectedCount)

	// start monitoring goroutine
	go g.monitorConnections()

	log.Info("global MCP manager started")
	return nil
}

// Stop stops global MCP manager
func (g *GlobalMCPManager) Stop() error {
	g.cancel()

	g.mu.Lock()
	type serverEntry struct {
		name string
		conn *MCPServerConnection
	}
	servers := make([]serverEntry, 0, len(g.servers))
	for name, conn := range g.servers {
		if conn != nil {
			servers = append(servers, serverEntry{name: name, conn: conn})
		}
	}
	g.servers = make(map[string]*MCPServerConnection)
	g.tools = make(map[string]tool.InvokableTool)
	g.mu.Unlock()

	for _, server := range servers {
		if err := server.conn.disconnect(); err != nil {
			log.Errorf("failed to disconnect MCP server %s: %v", server.name, err)
		}
	}

	log.Info("global MCP manager stopped")
	return nil
}

// createFailedConnection creates a failed connection object for subsequent reconnection
func (g *GlobalMCPManager) createFailedConnection(config MCPServerConfig) {
	conn := &MCPServerConnection{
		config:     config,
		tools:      make(map[string]tool.InvokableTool),
		connected:  false,
		lastError:  fmt.Errorf("initial connection failed"),
		retryCount: 0,
	}

	g.mu.Lock()
	g.servers[config.Name] = conn
	g.mu.Unlock()

	log.Infof("created connection object for failed MCP server: %s", config.Name)
}

// connectToServer connects to MCP server
func (g *GlobalMCPManager) connectToServer(config MCPServerConfig) error {
	// validate config
	if config.Name == "" {
		return fmt.Errorf("MCP server name cannot be empty")
	}

	if !config.Enabled {
		log.Infof("MCP server %s is disabled, skipping connection", config.Name)
		return nil
	}

	_, endpoint, endpointErr := endpointForConfig(config)
	if endpointErr != nil {
		return endpointErr
	}
	log.Infof("connecting to MCP server: %s (URL: %s)", config.Name, endpoint)

	conn := &MCPServerConnection{
		config: config,
		tools:  make(map[string]tool.InvokableTool),
	}

	g.mu.Lock()
	g.servers[config.Name] = conn
	g.mu.Unlock()

	// connect to server
	if err := conn.connect(); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %v", err)
	}

	log.Infof("connected to MCP server: %s", config.Name)
	return nil
}

// connect connects to MCP server
func (conn *MCPServerConnection) connect() error {
	// use background context, no timeout, keep SSE connection long-term
	ctx := context.Background()

	transportInstance, endpoint, err := buildMCPTransport(conn.config)
	if err != nil {
		return err
	}

	// use client.NewClient to create MCP client
	mcpClient := client.NewClient(transportInstance)
	conn.mu.Lock()
	conn.client = mcpClient
	conn.mu.Unlock()

	log.Infof("start connecting to MCP server: %s, %s URL: %s", conn.config.Name, conn.config.Type, endpoint)

	// start client
	if err := mcpClient.Start(ctx); err != nil {
		log.Errorf("failed to start MCP client, server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("failed to start client: %v", err)
	}

	log.Infof("MCP client started successfully: %s", conn.config.Name)

	// initialize client
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "xiaozhi-esp32-server",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Experimental: make(map[string]any),
			},
		},
	}

	log.Infof("initializing MCP server: %s", conn.config.Name)
	initResult, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		log.Errorf("failed to initialize MCP server, server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("initialization failed: %v", err)
	}

	log.Infof("MCP server initialized successfully: %s, result: %+v", conn.config.Name, initResult)

	// get tool list
	if err := conn.refreshTools(ctx); err != nil {
		log.Errorf("failed to get tool list: %v", err)
		// don't return error directly, tool list fetch failure should not prevent connection establishment
	}

	conn.mu.Lock()
	conn.connected = true
	conn.lastError = nil
	conn.retryCount = 0
	conn.mu.Unlock()

	log.Infof("MCP server connection established: %s", conn.config.Name)
	return nil
}

func normalizeMCPTransportType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "sse":
		return "sse"
	case "streamable_http", "streamable-http", "http":
		return "streamablehttp"
	default:
		return strings.ToLower(strings.TrimSpace(t))
	}
}

func endpointForConfig(config MCPServerConfig) (string, string, error) {
	transportType := normalizeMCPTransportType(config.Type)
	if transportType == "" {
		if strings.TrimSpace(config.SSEUrl) != "" {
			transportType = "sse"
		} else if strings.TrimSpace(config.Url) != "" {
			transportType = "streamablehttp"
		}
	}

	switch transportType {
	case "sse":
		if strings.TrimSpace(config.SSEUrl) != "" {
			return transportType, strings.TrimSpace(config.SSEUrl), nil
		}
		if strings.TrimSpace(config.Url) != "" {
			return transportType, strings.TrimSpace(config.Url), nil
		}
		return "", "", fmt.Errorf("MCP server %s missing SSE URL", config.Name)
	case "streamablehttp":
		if strings.TrimSpace(config.Url) != "" {
			return transportType, strings.TrimSpace(config.Url), nil
		}
		if strings.TrimSpace(config.SSEUrl) != "" {
			return transportType, strings.TrimSpace(config.SSEUrl), nil
		}
		return "", "", fmt.Errorf("MCP server %s missing StreamableHTTP URL", config.Name)
	default:
		return "", "", fmt.Errorf("MCP server %s unsupported type: %s", config.Name, config.Type)
	}
}

func buildMCPTransport(config MCPServerConfig) (transport.Interface, string, error) {
	transportType, endpoint, err := endpointForConfig(config)
	if err != nil {
		return nil, "", err
	}

	headers := make(map[string]string)
	for k, v := range config.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		headers[strings.TrimSpace(k)] = v
	}

	switch transportType {
	case "sse":
		opts := make([]transport.ClientOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHeaders(headers))
		}
		sseTransport, err := transport.NewSSE(endpoint, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create SSE transport: %v", err)
		}
		return sseTransport, endpoint, nil
	case "streamablehttp":
		opts := make([]transport.StreamableHTTPCOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		httpTransport, err := transport.NewStreamableHTTP(endpoint, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create StreamableHTTP transport: %v", err)
		}
		return httpTransport, endpoint, nil
	default:
		return nil, "", fmt.Errorf("unsupported MCP transport type: %s", transportType)
	}
}

func buildAllowedToolSet(allowedTools []string) map[string]struct{} {
	if len(allowedTools) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		set[toolName] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func filterMCPToolsByAllowList(tools []mcp.Tool, allowedTools []string) []mcp.Tool {
	allowedSet := buildAllowedToolSet(allowedTools)
	if len(allowedSet) == 0 {
		return tools
	}

	filtered := make([]mcp.Tool, 0, len(tools))
	for _, item := range tools {
		if _, ok := allowedSet[strings.TrimSpace(item.Name)]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// refreshTools refreshes tool list
func (conn *MCPServerConnection) refreshTools(ctx context.Context) error {
	conn.mu.RLock()
	serverName := conn.config.Name
	allowedTools := append([]string(nil), conn.config.AllowedTools...)
	mcpClient := conn.client
	conn.mu.RUnlock()
	if mcpClient == nil {
		return fmt.Errorf("MCP client not initialized")
	}

	// get tool list
	listRequest := mcp.ListToolsRequest{}
	toolsResult, err := mcpClient.ListTools(ctx, listRequest)
	if err != nil {
		return fmt.Errorf("failed to get tool list: %v", err)
	}

	tools := filterMCPToolsByAllowList(toolsResult.Tools, allowedTools)
	convertedTools := ConvertMcpToolListToInvokableToolList(tools, serverName, mcpClient)

	conn.mu.Lock()
	conn.tools = convertedTools
	conn.mu.Unlock()

	// update global tools table outside conn.mu to avoid lock order inversion with g.mu
	globalManager.updateGlobalTools(serverName, convertedTools)

	log.Infof("MCP server %s tool list updated, %d tools total", serverName, len(convertedTools))
	return nil
}

func ConvertMcpToolListToInvokableToolList(tools []mcp.Tool, serverName string, client *client.Client) map[string]tool.InvokableTool {
	invokeTools := make(map[string]tool.InvokableTool)
	for _, tool := range tools {

		marshaledInputSchema, err := sonic.Marshal(tool.InputSchema)
		if err != nil {
			log.Errorf("convert mcp tool to invokeable tool err: %+v", err)
			continue
		}
		inputSchema := &openapi3.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			log.Errorf("convert mcp tool to invokeable tool err: %+v", err)
			continue
		}

		mcpToolInstance := &McpTool{
			info: &schema.ToolInfo{
				Name:        tool.Name,
				Desc:        tool.Description,
				ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(inputSchema),
			},
			serverName: serverName,
			client:     client,
		}
		invokeTools[tool.Name] = mcpToolInstance
	}
	return invokeTools
}

// disconnect closes connection
func (conn *MCPServerConnection) disconnect() error {
	conn.mu.Lock()
	serverName := conn.config.Name
	mcpClient := conn.client
	conn.client = nil
	conn.connected = false
	conn.tools = make(map[string]tool.InvokableTool)
	conn.mu.Unlock()

	if globalManager != nil {
		globalManager.removeGlobalTools(serverName)
	}

	if mcpClient != nil {
		// close client outside lock, avoid locking fast path
		if err := mcpClient.Close(); err != nil {
			log.Errorf("failed to close MCP client: %v", err)
		}
	}

	return nil
}

func (g *GlobalMCPManager) removeGlobalTools(serverName string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, mcpToolInterface := range g.tools {
		if mt, ok := mcpToolInterface.(*McpTool); ok && mt.serverName == serverName {
			delete(g.tools, name)
		}
	}
}

// updateGlobalTools updates global tool list
func (g *GlobalMCPManager) updateGlobalTools(serverName string, tools map[string]tool.InvokableTool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// remove old tools from this server
	for name, mcpToolInterface := range g.tools {
		if mt, ok := mcpToolInterface.(*McpTool); ok && mt.serverName == serverName {
			delete(g.tools, name)
		}
	}

	// add new tools
	for name, mcpToolInterface := range tools {
		g.tools[fmt.Sprintf("%s_%s", serverName, name)] = mcpToolInterface
	}
}

// GetAllTools gets all available tools
func (g *GlobalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpToolInterface := range g.tools {
		result[name] = mcpToolInterface
	}
	return result
}

// GetToolByName gets tool by name
func (g *GlobalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	//all servers
	for _, conn := range g.servers {
		sname := fmt.Sprintf("%s_%s", conn.config.Name, name)
		mcpToolInterface, exists := g.tools[sname]
		if exists {
			return mcpToolInterface, true
		}
	}
	return nil, false
}

func GetServerClientByName(serverName string) *client.Client {
	return GetGlobalMCPManager().GetServerClientByName(serverName)
}

func (g *GlobalMCPManager) GetServerClientByName(serverName string) *client.Client {
	g.mu.RLock()
	conn, ok := g.servers[serverName]
	g.mu.RUnlock()
	if !ok || conn == nil {
		return nil
	}

	conn.mu.RLock()
	defer conn.mu.RUnlock()
	return conn.client
}

func GetServerEndpointSnapshotByName(serverName string) string {
	return GetGlobalMCPManager().GetServerEndpointSnapshotByName(serverName)
}

func (g *GlobalMCPManager) GetServerEndpointSnapshotByName(serverName string) string {
	g.mu.RLock()
	conn, ok := g.servers[serverName]
	g.mu.RUnlock()
	if !ok || conn == nil {
		return ""
	}

	conn.mu.RLock()
	config := conn.config
	conn.mu.RUnlock()

	_, endpoint, err := endpointForConfig(config)
	if err != nil {
		if strings.TrimSpace(config.Url) != "" {
			return strings.TrimSpace(config.Url)
		}
		return strings.TrimSpace(config.SSEUrl)
	}
	return endpoint
}

func ReconnectServerByName(serverName string) (*client.Client, error) {
	return GetGlobalMCPManager().reconnectServer(serverName)
}

// isSessionClosedError checks if error is session closed
func isSessionClosedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "session closed")
}

// monitorConnections monitors connection status
func (g *GlobalMCPManager) monitorConnections() {
	pingTicker := time.NewTicker(20 * time.Second) // ping every 60 seconds
	defer pingTicker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-pingTicker.C:
			// execute ping check
			g.mu.RLock()
			for name, conn := range g.servers {
				go func(name string, conn *MCPServerConnection) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					if err := conn.ping(ctx); err != nil {
						log.Warnf("MCP server %s ping failed, starting reconnection: %v", name, err)
						// mark as disconnected on ping failure and trigger reconnection
						conn.mu.Lock()
						conn.connected = false
						conn.lastError = err
						conn.mu.Unlock()

						// trigger reconnection directly
						go g.reconnectServer(name)
					} else {
						//log.Debugf("MCP server %s ping succeeded", name)
					}
				}(name, conn)
			}
			g.mu.RUnlock()
		}
	}
}

// reconnectServer reconnects to server and returns new client
func (g *GlobalMCPManager) reconnectServer(serverName string) (*client.Client, error) {
	g.mu.RLock()
	var conn *MCPServerConnection
	for _, c := range g.servers {
		if c.config.Name == serverName {
			conn = c
			break
		}
	}
	g.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("server connection not found: %s", serverName)
	}

	conn.mu.Lock()
	if conn.reconnecting {
		wait := conn.reconnectWait
		conn.mu.Unlock()
		if wait != nil {
			<-wait
		}

		conn.mu.RLock()
		mcpClient := conn.client
		connected := conn.connected
		lastErr := conn.lastError
		conn.mu.RUnlock()
		if mcpClient != nil && connected {
			return mcpClient, nil
		}
		if lastErr != nil {
			return nil, fmt.Errorf("reconnection failed: %v", lastErr)
		}
		return nil, fmt.Errorf("reconnection failed: client not ready")
	}
	wait := make(chan struct{})
	conn.reconnecting = true
	conn.reconnectWait = wait
	conn.mu.Unlock()

	defer func() {
		conn.mu.Lock()
		conn.reconnecting = false
		if conn.reconnectWait == wait {
			close(wait)
			conn.reconnectWait = nil
		}
		conn.mu.Unlock()
	}()

	// disconnect
	if err := conn.disconnect(); err != nil {
		log.Errorf("failed to disconnect: %v", err)
	}

	// wait briefly to ensure resources are released
	time.Sleep(time.Second)

	// reconnect
	if err := conn.connect(); err != nil {
		conn.mu.Lock()
		conn.lastError = err
		conn.mu.Unlock()
		return nil, fmt.Errorf("reconnection failed: %v", err)
	}

	conn.mu.RLock()
	mcpClient := conn.client
	conn.mu.RUnlock()
	return mcpClient, nil
}

// ping sends ping request to check connection status
func (conn *MCPServerConnection) ping(ctx context.Context) error {
	conn.mu.RLock()
	mcpClient := conn.client
	conn.mu.RUnlock()
	if mcpClient == nil {
		return fmt.Errorf("client not initialized")
	}

	// use empty Ping request as ping
	err := mcpClient.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	conn.mu.Lock()
	conn.lastPing = time.Now()
	conn.mu.Unlock()

	return nil
}
