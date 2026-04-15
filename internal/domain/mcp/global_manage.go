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
	SSEUrl       string            `json:"sse_url" mapstructure:"sse_url"` // toafter兼容 sse_url field
	Enabled      bool              `json:"enabled" mapstructure:"enabled"`
	Provider     string            `json:"provider,omitempty" mapstructure:"provider"`
	ServiceID    string            `json:"service_id,omitempty" mapstructure:"service_id"`
	AuthRef      string            `json:"auth_ref,omitempty" mapstructure:"auth_ref"`
	Headers      map[string]string `json:"headers,omitempty" mapstructure:"headers"`
	AllowedTools []string          `json:"allowed_tools,omitempty" mapstructure:"allowed_tools"`
}

// GlobalMCPManager globalMCPmanage器
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

// GetGlobalMCPManager getglobalMCPmanage器singleton
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

// Start startglobalMCPmanage器
func (g *GlobalMCPManager) Start() error {
	// 热更scenario：Stop after ctx alreadycancel，need重建以便restartaftermonitorandreconnectnormal
	if g.ctx != nil && g.ctx.Err() != nil {
		g.ctx, g.cancel = context.WithCancel(context.Background())
		g.reconnectConf = ReconnectConfig{
			Interval:    time.Duration(viper.GetInt("mcp.global.reconnect_interval")) * time.Second,
			MaxAttempts: viper.GetInt("mcp.global.max_reconnect_attempts"),
		}
	}

	// firstfirstinspectconfig
	CheckMCPConfig()

	if !viper.GetBool("mcp.global.enabled") {
		log.Info("globalMCPmanage器already禁use")
		return nil
	}

	var serverConfigs []MCPServerConfig
	if err := viper.UnmarshalKey("mcp.global.servers", &serverConfigs); err != nil {
		log.Errorf("parseMCPserverconfigfailed: %v", err)
		return fmt.Errorf("parseMCPserverconfigfailed: %v", err)
	}

	log.Infof("fromconfiginreadto %d 个MCPserverconfig", len(serverConfigs))

	// 详细record每个serverconfig
	for i, config := range serverConfigs {
		log.Infof("MCPserver[%d]: Type=%s, Name=%s, Url=%s, SSEUrl=%s, Enabled=%v",
			i+1, config.Type, config.Name, config.Url, config.SSEUrl, config.Enabled)
	}

	// join启useofserver
	connectedCount := 0
	for _, config := range serverConfigs {
		if config.Enabled {
			if err := g.connectToServer(config); err != nil {
				log.Errorf("jointoMCPserver %s failed: %v", config.Name, err)
			} else {
				connectedCount++
			}
		} else {
			log.Infof("MCPserver %s already禁use，skipjoin", config.Name)
		}
	}

	log.Infof("successfuljoin %d 个MCPserver", connectedCount)

	// startmonitorgoroutine
	go g.monitorConnections()

	log.Info("globalMCPmanage器alreadystart")
	return nil
}

// Stop stopglobalMCPmanage器
func (g *GlobalMCPManager) Stop() error {
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()

	for name, conn := range g.servers {
		if err := conn.disconnect(); err != nil {
			log.Errorf("disconnectMCPserver %s joinfailed: %v", name, err)
		}
	}

	g.servers = make(map[string]*MCPServerConnection)
	g.tools = make(map[string]tool.InvokableTool)

	log.Info("globalMCPmanage器alreadystop")
	return nil
}

// createFailedConnection createfailedofjoinobjectused foraftercontinuereconnect
func (g *GlobalMCPManager) createFailedConnection(config MCPServerConfig) {
	conn := &MCPServerConnection{
		config:     config,
		tools:      make(map[string]tool.InvokableTool),
		connected:  false,
		lastError:  fmt.Errorf("initializejoinfailed"),
		retryCount: 0,
	}

	g.mu.Lock()
	g.servers[config.Name] = conn
	g.mu.Unlock()

	log.Infof("alreadyisfailedofMCPservercreatejoinobject: %s", config.Name)
}

// connectToServer jointoMCPserver
func (g *GlobalMCPManager) connectToServer(config MCPServerConfig) error {
	// validateconfig
	if config.Name == "" {
		return fmt.Errorf("MCPservernamecannot be empty")
	}

	if !config.Enabled {
		log.Infof("MCPserver %s already禁use，skipjoin", config.Name)
		return nil
	}

	_, endpoint, endpointErr := endpointForConfig(config)
	if endpointErr != nil {
		return endpointErr
	}
	log.Infof("isjoinMCPserver: %s (URL: %s)", config.Name, endpoint)

	conn := &MCPServerConnection{
		config: config,
		tools:  make(map[string]tool.InvokableTool),
	}

	g.mu.Lock()
	g.servers[config.Name] = conn
	g.mu.Unlock()

	// jointoserver
	if err := conn.connect(); err != nil {
		return fmt.Errorf("joinMCPserverfailed: %v", err)
	}

	log.Infof("alreadyjointoMCPserver: %s", config.Name)
	return nil
}

// connect jointoMCPserver
func (conn *MCPServerConnection) connect() error {
	// usebackgroundcontext，nosettimeout，letSSEjoinlong期keep
	ctx := context.Background()

	transportInstance, endpoint, err := buildMCPTransport(conn.config)
	if err != nil {
		return err
	}

	// use client.NewClient create MCP client-side
	mcpClient := client.NewClient(transportInstance)

	conn.client = mcpClient

	log.Infof("startjoinMCPserver: %s, %s URL: %s", conn.config.Name, conn.config.Type, endpoint)

	// startclient-side
	if err := conn.client.Start(ctx); err != nil {
		log.Errorf("startMCPclient-sidefailed，server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("startclient-sidefailed: %v", err)
	}

	log.Infof("MCPclient-sidestartsuccessful: %s", conn.config.Name)

	// initializeclient-side
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

	log.Infof("isinitializeMCPserver: %s", conn.config.Name)
	initResult, err := conn.client.Initialize(ctx, initRequest)
	if err != nil {
		log.Errorf("initializeMCPserverfailed，server: %s, error: %v", conn.config.Name, err)
		return fmt.Errorf("initializefailed: %v", err)
	}

	log.Infof("MCPserverinitializesuccessful: %s, result: %+v", conn.config.Name, initResult)

	// gettoollist
	if err := conn.refreshTools(ctx); err != nil {
		log.Errorf("gettoollistfailed: %v", err)
		// nodirectreturnerror，becauseistoollistgetfailednoshouldpreventjoin建立
	}

	conn.mu.Lock()
	conn.connected = true
	conn.lastError = nil
	conn.retryCount = 0
	conn.mu.Unlock()

	log.Infof("MCPserverjoin建立complete: %s", conn.config.Name)
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
		return "", "", fmt.Errorf("MCPserver %s MissingSSE URL", config.Name)
	case "streamablehttp":
		if strings.TrimSpace(config.Url) != "" {
			return transportType, strings.TrimSpace(config.Url), nil
		}
		if strings.TrimSpace(config.SSEUrl) != "" {
			return transportType, strings.TrimSpace(config.SSEUrl), nil
		}
		return "", "", fmt.Errorf("MCPserver %s MissingStreamableHTTP URL", config.Name)
	default:
		return "", "", fmt.Errorf("MCPserver %s typeunsupported: %s", config.Name, config.Type)
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
			return nil, "", fmt.Errorf("createSSE传输layerfailed: %v", err)
		}
		return sseTransport, endpoint, nil
	case "streamablehttp":
		opts := make([]transport.StreamableHTTPCOption, 0)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		httpTransport, err := transport.NewStreamableHTTP(endpoint, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("createStreamableHTTP传输layerfailed: %v", err)
		}
		return httpTransport, endpoint, nil
	default:
		return nil, "", fmt.Errorf("unsupportedofMCP传输type: %s", transportType)
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

// refreshTools refreshtoollist
func (conn *MCPServerConnection) refreshTools(ctx context.Context) error {
	// gettoollist
	listRequest := mcp.ListToolsRequest{}
	toolsResult, err := conn.client.ListTools(ctx, listRequest)
	if err != nil {
		return fmt.Errorf("gettoollistfailed: %v", err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	tools := filterMCPToolsByAllowList(toolsResult.Tools, conn.config.AllowedTools)
	conn.tools = ConvertMcpToolListToInvokableToolList(tools, conn.config.Name, conn.client)

	// updateglobaltoollist
	globalManager.updateGlobalTools(conn.config.Name, conn.tools)

	log.Infof("MCPserver %s toollistalreadyupdate，total %d 个tool", conn.config.Name, len(conn.tools))
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

// disconnect disconnect join
func (conn *MCPServerConnection) disconnect() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.client != nil {
		// closeclient-side
		if err := conn.client.Close(); err != nil {
			log.Errorf("closeMCPclient-sidefailed: %v", err)
		}
		conn.client = nil
	}

	conn.connected = false
	conn.tools = make(map[string]tool.InvokableTool)

	return nil
}

// updateGlobalTools updateglobaltoollist
func (g *GlobalMCPManager) updateGlobalTools(serverName string, tools map[string]tool.InvokableTool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// removethisserverof旧tool
	for name, mcpToolInterface := range g.tools {
		if mt, ok := mcpToolInterface.(*McpTool); ok && mt.serverName == serverName {
			delete(g.tools, name)
		}
	}

	// add新tool
	for name, mcpToolInterface := range tools {
		g.tools[fmt.Sprintf("%s_%s", serverName, name)] = mcpToolInterface
	}
}

// GetAllTools getallavailabletool
func (g *GlobalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpToolInterface := range g.tools {
		result[name] = mcpToolInterface
	}
	return result
}

// GetToolByName according tonamegettool
func (g *GlobalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	//allofserver
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

// isSessionClosedError determine ifissession closederror
func isSessionClosedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "session closed")
}

// monitorConnections monitorjoinstate
func (g *GlobalMCPManager) monitorConnections() {
	pingTicker := time.NewTicker(20 * time.Second) // 每60secondpingatimes
	defer pingTicker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-pingTicker.C:
			// executepingdetect
			g.mu.RLock()
			for name, conn := range g.servers {
				go func(name string, conn *MCPServerConnection) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					if err := conn.ping(ctx); err != nil {
						log.Warnf("MCPserver %s pingfailed，startreconnect: %v", name, err)
						// pingfailedwhendirectmarkisdisconnectandtriggerreconnect
						conn.mu.Lock()
						conn.connected = false
						conn.lastError = err
						conn.mu.Unlock()

						// directtriggerreconnect
						go g.reconnectServer(name)
					} else {
						//log.Debugf("MCPserver %s pingsuccessful", name)
					}
				}(name, conn)
			}
			g.mu.RUnlock()
		}
	}
}

// reconnectServer reconnectserverandreturnnewclient
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
		return nil, fmt.Errorf("not找toserverjoin: %s", serverName)
	}

	// disconnect join
	if err := conn.disconnect(); err != nil {
		log.Errorf("disconnect joinfailed: %v", err)
	}

	// waitasmall段timeensureresourcerelease
	time.Sleep(time.Second)

	// rejoin
	if err := conn.connect(); err != nil {
		return nil, fmt.Errorf("reconnectfailed: %v", err)
	}

	return conn.client, nil
}

// ping sendpingrequestdetectjoinstate
func (conn *MCPServerConnection) ping(ctx context.Context) error {
	if conn.client == nil {
		return fmt.Errorf("clientnot initialized")
	}

	// useemptyofPingrequestasisping
	err := conn.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pingfailed: %v", err)
	}

	conn.mu.Lock()
	conn.lastPing = time.Now()
	conn.mu.Unlock()

	return nil
}
