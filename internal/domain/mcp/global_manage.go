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

// MCPServerConfig MCPserverconfig
type MCPServerConfig struct {
	Name         string            `json:"name" mapstructure:"name"`
	Type         string            `json:"type" mapstructure:"type"`
	Url          string            `json:"url" mapstructure:"url"`
	SSEUrl       string            `json:"sse_url" mapstructure:"sse_url"` // for backward compatibility sse_url field
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

// ReconnectConfig reconnectconfig
type ReconnectConfig struct {
	Interval    time.Duration
	MaxAttempts int
}

// MCPServerConnection MCPserverjoin
type MCPServerConnection struct {
	config     MCPServerConfig
	client     *client.Client
	tools      map[string]tool.InvokableTool
	connected  bool
	mu         sync.RWMutex
	lastError  error
	retryCount int
	lastPing   time.Time
}

var (
	globalManager *GlobalMCPManager
	once          sync.Once
)

// GetGlobalMCPManager get global MCP manager singleton
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

// Start start global MCP manager
func (g *GlobalMCPManager) Start() error {
	// Hot reload scenario: Stop after ctx already cancelled, need rebuild so that restart after monitor and reconnect normal
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
		log.Info("global MCP manager already disabled")
		return nil
	}

	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("parse MCP server config failed: %v", err)
		return fmt.Errorf("parse MCP server config failed: %v", err)
	}

	log.Infof("read %d MCP server config from config", len(serverConfigs))

	// detailed record each server config
	for i, config := range serverConfigs {
		log.Infof("MCP server[%d]: Type=%s, Name=%s, Url=%s, SSEUrl=%s, Enabled=%v",
			i+1, config.Type, config.Name, config.Url, config.SSEUrl, config.Enabled)
	}

	// connect enabled servers
	connectedCount := 0
	for _, config := range serverConfigs {
		if config.Enabled {
			if err := g.connectToServer(config); err != nil {
				log.Errorf("connect to MCP server %s failed: %v", config.Name, err)
			} else {
				connectedCount++
			}
		} else {
			log.Infof("MCP server %s already disabled, skip connect", config.Name)
		}
	}

	log.Infof("successfully connected %d MCP servers", connectedCount)

	// start monitor goroutine
	go g.monitorConnections()

	log.Info("global MCP manager already started")
	return nil
}

// Stop stop global MCP manager
func (g *GlobalMCPManager) Stop() error {
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()

	for name, conn := range g.servers {
		if err := conn.disconnect(); err != nil {
			log.Errorf("disconnect MCP server %s connection failed: %v", name, err)
		}
	}

	g.servers = make(map[string]*MCPServerConnection)
	g.tools = make(map[string]tool.InvokableTool)

	log.Info("global MCP manager already stopped")
	return nil
}

// createFailedConnection create failed connection object for later reconnect
func (g *GlobalMCPManager) createFailedConnection(config MCPServerConfig) {
	conn := &MCPServerConnection{
		config:     config,
		tools:      make(map[string]tool.InvokableTool),
		connected:  false,
		lastError:  fmt.Errorf("initialize connection failed"),
		retryCount: 0,
	}

	g.mu.Lock()
	g.servers[config.Name] = conn
	g.mu.Unlock()

	log.Infof("create connection object for failed MCP server: %s", config.Name)
}

// connectToServer connect to MCP server
func (g *GlobalMCPManager) connectToServer(config MCPServerConfig) error {
	// validate config
	if config.Name == "" {
		return fmt.Errorf("MCP server name cannot be empty")
	}

	if !config.Enabled {
		log.Infof("MCP server %s already disabled, skip connect", config.Name)
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
		return fmt.Errorf("connect MCP server failed: %v", err)
	}

	log.Infof("already connected to MCP server: %s", config.Name)
	return nil
}

// connect connect to MCP server
func (conn *MCPServerConnection) connect() error {
	// use background context, no timeout, let SSE connection long-term keep
	ctx := context.Background()

	transportInstance, endpoint, err := buildMCPTransport(conn.config)
	if err != nil {
		return err
	}

	// use client.NewClient create MCP client
	mcpClient := client.NewClient(transportInstance)

	conn.client = mcpClient

	log.Infof("start connecting to MCP server: %s, %s URL: %s", conn.config.Name, conn.config.Type, endpoint)

	// start client
	if err := conn.client.Start(ctx); err != nil {
		log.Errorf("start MCP client failed, server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("start client failed: %v", err)
	}

	log.Infof("MCP client start successful: %s", conn.config.Name)

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
	initResult, err := conn.client.Initialize(ctx, initRequest)
	if err != nil {
		log.Errorf("initialize MCP server failed, server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("initialize failed: %v", err)
	}

	log.Infof("MCP server initialize successful: %s, result: %+v", conn.config.Name, initResult)

	// get tool list
	if err := conn.refreshTools(ctx); err != nil {
		log.Errorf("get tool list failed: %v", err)
		// don't return error directly, because tool list get failed should not prevent connection establishment
	}

	conn.mu.Lock()
	conn.connected = true
	conn.lastError = nil
	conn.retryCount = 0
	conn.mu.Unlock()

	log.Infof("MCP server connection establishment complete: %s", conn.config.Name)
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
		return "", "", fmt.Errorf("MCP server %s Missing SSE URL", config.Name)
	case "streamablehttp":
		if strings.TrimSpace(config.Url) != "" {
			return transportType, strings.TrimSpace(config.Url), nil
		}
		if strings.TrimSpace(config.SSEUrl) != "" {
			return transportType, strings.TrimSpace(config.SSEUrl), nil
		}
		return "", "", fmt.Errorf("MCP server %s Missing StreamableHTTP URL", config.Name)
	default:
		return "", "", fmt.Errorf("MCP server %s type unsupported: %s", config.Name, config.Type)
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
			return nil, "", fmt.Errorf("create SSE transport layer failed: %v", err)
		}
		return sseTransport, endpoint, nil
	case "streamablehttp":
		opts := make([]transport.StreamableHTTPCOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		httpTransport, err := transport.NewStreamableHTTP(endpoint, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("create StreamableHTTP transport layer failed: %v", err)
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

// refreshTools refresh tool list
func (conn *MCPServerConnection) refreshTools(ctx context.Context) error {
	// get tool list
	listRequest := mcp.ListToolsRequest{}
	toolsResult, err := conn.client.ListTools(ctx, listRequest)
	if err != nil {
		return fmt.Errorf("get tool list failed: %v", err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	tools := filterMCPToolsByAllowList(toolsResult.Tools, conn.config.AllowedTools)
	conn.tools = ConvertMcpToolListToInvokableToolList(tools, conn.config.Name, conn.client)

	// update global tool list
	globalManager.updateGlobalTools(conn.config.Name, conn.tools)

	log.Infof("MCP server %s tool list already updated, total %d tools", conn.config.Name, len(conn.tools))
	return nil
}

func ConvertMcpToolListToInvokableToolList(tools []mcp.Tool, serverName string, client *client.Client) map[string]tool.InvokableTool {
	invokeTools := make(map[string]tool.InvokableTool)
	for _, tool := range tools {

		marshaledInputSchema, err := sonic.Marshal(tool.InputSchema)
		if err != nil {
			log.Errorf("convert mcp tool to invokable tool err: %+v", err)
			continue
		}
		inputSchema := &openapi3.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			log.Errorf("convert mcp tool to invokable tool err: %+v", err)
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

// disconnect disconnect connection
func (conn *MCPServerConnection) disconnect() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.client != nil {
		// close client
		if err := conn.client.Close(); err != nil {
			log.Errorf("close MCP client failed: %v", err)
		}
		conn.client = nil
	}

	conn.connected = false
	conn.tools = make(map[string]tool.InvokableTool)

	return nil
}

// updateGlobalTools update global tool list
func (g *GlobalMCPManager) updateGlobalTools(serverName string, tools map[string]tool.InvokableTool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// remove this server's old tools
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

// GetAllTools get all available tools
func (g *GlobalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpToolInterface := range g.tools {
		result[name] = mcpToolInterface
	}
	return result
}

// GetToolByName get tool by name
func (g *GlobalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// all servers
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
	defer g.mu.RUnlock()

	conn, ok := g.servers[serverName]
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

// isSessionClosedError determine if is session closed error
func isSessionClosedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "session closed")
}

// monitorConnections monitor connection state
func (g *GlobalMCPManager) monitorConnections() {
	pingTicker := time.NewTicker(20 * time.Second) // ping every 20 seconds
	defer pingTicker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-pingTicker.C:
			// execute ping detect
			g.mu.RLock()
			for name, conn := range g.servers {
				go func(name string, conn *MCPServerConnection) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					if err := conn.ping(ctx); err != nil {
						log.Warnf("MCP server %s ping failed, start reconnect: %v", name, err)
						// when ping failed, directly mark as disconnected and trigger reconnect
						conn.mu.Lock()
						conn.connected = false
						conn.lastError = err
						conn.mu.Unlock()

						// directly trigger reconnect
						go g.reconnectServer(name)
					} else {
						//log.Debugf("MCP server %s ping successful", name)
					}
				}(name, conn)
			}
			g.mu.RUnlock()
		}
	}
}

// reconnectServer reconnect server and return new client
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
		return nil, fmt.Errorf("not found server connection: %s", serverName)
	}

	// disconnect connection
	if err := conn.disconnect(); err != nil {
		log.Errorf("disconnect connection failed: %v", err)
	}

	// wait a short time to ensure resource release
	time.Sleep(time.Second)

	// reconnect
	if err := conn.connect(); err != nil {
		return nil, fmt.Errorf("reconnect failed: %v", err)
	}

	return conn.client, nil
}

// ping send ping request to detect connection state
func (conn *MCPServerConnection) ping(ctx context.Context) error {
	if conn.client == nil {
		return fmt.Errorf("client not initialized")
	}

	// use empty Ping request as ping
	err := conn.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	conn.mu.Lock()
	conn.lastPing = time.Now()
	conn.mu.Unlock()

	return nil
}
