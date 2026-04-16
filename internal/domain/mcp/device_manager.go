package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// DeviceMcpSession represents a device MCP session, aggregates multiple MCP connections
type DeviceMcpSession struct {
	deviceID      string
	Ctx           context.Context
	cancel        context.CancelFunc
	wsEndPointMcp sync.Map
	iotOverMcp    *McpClientInstance
	iotMux        sync.RWMutex
}

func (dcs *DeviceMcpSession) AddWsEndPointMcp(mcpClient *McpClientInstance) {
	dcs.wsEndPointMcp.Store(mcpClient.serverName, mcpClient)

	// setclosecallback
	mcpClient.SetOnCloseHandler(dcs.handleMcpClientClose)

	mcpClient.refreshTools()
}

// todo
func (dcs *DeviceMcpSession) SetIotOverMcp(mcpClient *McpClientInstance) {
	dcs.iotMux.Lock()
	defer dcs.iotMux.Unlock()
	// if already exists iotOverMcp, first close it
	/*if dcs.iotOverMcp != nil {
		dcs.iotOverMcp.Close()
	}*/
	dcs.iotOverMcp = mcpClient

	// setclosecallback
	mcpClient.SetOnCloseHandler(dcs.handleMcpClientClose)

	mcpClient.refreshTools()
}

func (dcs *DeviceMcpSession) RemoveWsEndPointMcp(mcpClient *McpClientInstance) {
	dcs.wsEndPointMcp.Delete(mcpClient.serverName)
}

// GetDeviceID get device ID
func (dcs *DeviceMcpSession) GetDeviceID() string {
	return dcs.deviceID
}

// handleMcpClientClose process MCP client close event
func (dcs *DeviceMcpSession) handleMcpClientClose(instance *McpClientInstance, reason string) {
	logger.Infof("device %s MCP client %s already closed, reason: %s", dcs.deviceID, instance.serverName, reason)

	// from session remove already closed client
	dcs.RemoveWsEndPointMcp(instance)

	// if all WebSocket endpoints are closed, can consider cleanup whole session
	/*if len(dcs.wsEndPointMcp) == 0 && dcs.iotOverMcp == nil {
		logger.Infof("device %s ofallMCPjoinalreadyclose，cleanupsession", dcs.deviceID)
		dcs.cancel()
	}*/
}

// McpClientInstance represents a concrete MCP client connection
type McpClientInstance struct {
	serverName string
	mcpClient  *client.Client // from ws endpoint connected mcp server
	tools      map[string]tool.InvokableTool
	toolsMux   sync.RWMutex // protected tool list mutex lock
	serverInfo *mcp.InitializeResult
	lastPing   time.Time
	Ctx        context.Context
	cancel     context.CancelFunc
	connected  bool
	conn       ConnInterface

	// addclosecallback
	onCloseHandler func(instance *McpClientInstance, reason string)
}

// NewDeviceMCPClient create new MCP client
func NewDeviceMCPSession(deviceID string) *DeviceMcpSession {
	ctx, cancel := context.WithCancel(context.Background())

	deviceMcpClient := &DeviceMcpSession{
		deviceID: deviceID,
		Ctx:      ctx,
		cancel:   cancel,
		iotMux:   sync.RWMutex{},
		// wsEndPointMcp: make(map[string]*McpClientInstance),
	}

	go deviceMcpClient.refreshToolsAndPing()

	return deviceMcpClient
}

func NewWsEndPointMcpClient(ctx context.Context, deviceID string, conn *websocket.Conn) *McpClientInstance {
	ctx, cancel := context.WithCancel(ctx)

	wsTransport, err := NewWebsocketTransport(conn)
	if err != nil {
		logger.Errorf("createMCPclient-sidefailed: %v", err)
		return nil
	}
	mcpClient := client.NewClient(wsTransport)

	wsEndPointMcp := &McpClientInstance{
		serverName: fmt.Sprintf("ws_endpoint_mcp_%s_%s", deviceID, conn.RemoteAddr().String()),
		mcpClient:  mcpClient,
		tools:      make(map[string]tool.InvokableTool),
		Ctx:        ctx,
		cancel:     cancel,
		connected:  true,
		lastPing:   time.Now(),
	}
	mcpClient.OnNotification(wsEndPointMcp.handleJSONRPCNotification)

	// settransportofclosecallback
	wsTransport.SetOnCloseHandler(wsEndPointMcp.handleTransportClose)

	wsEndPointMcp.sendInitlize(ctx)
	wsEndPointMcp.mcpClient.Start(ctx)
	return wsEndPointMcp
}

func NewIotOverMcpClient(deviceID string, conn ConnInterface) *McpClientInstance {
	ctx, cancel := context.WithCancel(context.Background())

	wsTransport, err := NewIotOverMcpTransport(conn)
	if err != nil {
		logger.Errorf("createMCPclient-sidefailed: %v", err)
		return nil
	}
	mcpClient := client.NewClient(wsTransport)

	iotOverMcp := &McpClientInstance{
		serverName: fmt.Sprintf("iot_over_mcp_%s", deviceID),
		mcpClient:  mcpClient,
		tools:      make(map[string]tool.InvokableTool),
		Ctx:        ctx,
		cancel:     cancel,
		connected:  true,
		lastPing:   time.Now(),
	}
	wsTransport.SetNotificationHandler(iotOverMcp.handleJSONRPCNotification)

	// settransportofclosecallback
	wsTransport.SetOnCloseHandler(iotOverMcp.handleTransportClose)

	iotOverMcp.sendInitlize(ctx)
	iotOverMcp.mcpClient.Start(ctx)

	return iotOverMcp
}

// refreshToolsCommon common tool list refresh logic
func (dc *McpClientInstance) refreshTools() error {
	tools, err := dc.mcpClient.ListTools(dc.Ctx, mcp.ListToolsRequest{})
	if err != nil {
		logger.Errorf("refreshtoollistfailed: %v", err)
		return err
	}

	// use mutex lock protected tool list update
	dc.toolsMux.Lock()
	dc.tools = ConvertMcpToolListToInvokableToolList(tools.Tools, dc.serverName, dc.mcpClient)
	dc.toolsMux.Unlock()

	logger.Infof("refresh tool list successful: %s get %d tools", dc.serverName, len(dc.tools))
	return nil
}

func (dc *McpClientInstance) GetServerName() string {
	return dc.serverName
}

func (dc *DeviceMcpSession) refreshToolsAndPing() {
	// only at initialize when get a time tool list
	findTools := func(mcpInstance *McpClientInstance) {
		if mcpInstance == nil {
			return
		}
		mcpInstance.refreshTools()
	}

	ping := func(mcpInstance *McpClientInstance) {
		if mcpInstance == nil {
			return
		}
		err := mcpInstance.mcpClient.Ping(mcpInstance.Ctx)
		if err == nil {
			mcpInstance.lastPing = time.Now()
			logger.Debugf("device %s pingsuccessful", mcpInstance.serverName)
		} else {
			logger.Warnf("device %s pingfailed: %v", mcpInstance.serverName, err)
		}
	}

	// initializewhengettoollist
	dc.wsEndPointMcp.Range(func(_, mcpInstance interface{}) bool {
		findTools(mcpInstance.(*McpClientInstance))
		return true
	})

	findTools(dc.iotOverMcp)

	// every 2 minutes perform a ping
	pingTick := time.NewTicker(2 * time.Minute)
	defer pingTick.Stop()

	for {
		select {
		case <-dc.Ctx.Done():
			logger.Infof("device %s session already cancelled, stopping", dc.deviceID)
			return
		case <-pingTick.C:
			dc.wsEndPointMcp.Range(func(_, mcpInstance interface{}) bool {
				ping(mcpInstance.(*McpClientInstance))
				return true
			})
			//ping(dc.iotOverMcp)
		}
	}
}

func (dc *McpClientInstance) sendInitlize(ctx context.Context) error {
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "mcp-go",
				Version: "0.1.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	serverInfo, err := dc.mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		fmt.Printf("Failed to initialize: %v", err)
		return err
	}
	dc.serverInfo = serverInfo
	return nil
}

func (dc *McpClientInstance) findTools() (*mcp.ListToolsResult, error) {
	tools, err := dc.mcpClient.ListTools(dc.Ctx, mcp.ListToolsRequest{})
	if err != nil {
		logger.Errorf("gettoollistfailed: %v", err)
		return nil, err
	}
	return tools, nil
}

// handleJSONRPCNotification process JSON-RPC notify
func (dc *McpClientInstance) handleJSONRPCNotification(notification mcp.JSONRPCNotification) {
	switch notification.Method {
	case "notifications/progress":
		//handleProgressNotification(notification)
	case "notifications/message":
		//handleMessageNotification(notification)
	case "notifications/resources/updated":
		//handleResourceUpdateNotification(notification)
	case "notifications/tools/updated":
		// receive tool update notify, refresh tool list
		logger.Infof("receivetoolupdatenotify，refreshtoollist")
		go dc.refreshToolsOnNotification()
	default:
		log.Printf("Unknown notification: %s", notification.Method)
	}
}

// refreshToolsOnNotification based on notify refresh tool list
func (dc *McpClientInstance) refreshToolsOnNotification() {
	// add short delay avoid frequent refresh
	time.Sleep(100 * time.Millisecond)
	dc.refreshTools()
}

// handleJSONRPCError process JSON-RPC error
func (dc *McpClientInstance) handleJSONRPCError(errMsg mcp.JSONRPCError) error {
	logger.Errorf("receive MCP server error: %+v", errMsg.Error)
	return nil
}

// handleTransportClose process transport layer close event
func (dc *McpClientInstance) handleTransportClose(reason string) {
	logger.Infof("MCP client %s transport layer close, reason: %s", dc.serverName, reason)

	// mark connection already disconnect
	dc.connected = false

	// cancelcontext
	dc.cancel()

	// notify upper process
	if dc.onCloseHandler != nil {
		dc.onCloseHandler(dc, reason)
	}
}

// SetOnCloseHandler set close callback
func (dc *McpClientInstance) SetOnCloseHandler(handler func(instance *McpClientInstance, reason string)) {
	dc.onCloseHandler = handler
}

// IsConnected inspect connection whether still active
func (dc *McpClientInstance) IsConnected() bool {
	return dc.connected
}

// GetConnectionStatus get connection state info
func (dc *McpClientInstance) GetConnectionStatus() map[string]interface{} {
	dc.toolsMux.RLock()
	toolsCount := len(dc.tools)
	dc.toolsMux.RUnlock()

	return map[string]interface{}{
		"server_name": dc.serverName,
		"connected":   dc.connected,
		"last_ping":   dc.lastPing,
		"tools_count": toolsCount,
	}
}

// GetTools get tool list
func (dc *DeviceMcpSession) GetTools() map[string]tool.InvokableTool {
	tools := make(map[string]tool.InvokableTool)
	dc.wsEndPointMcp.Range(func(_, value interface{}) bool {
		mcpInstance := value.(*McpClientInstance)
		mcpInstance.toolsMux.RLock()
		for k, v := range mcpInstance.tools {
			tools[k] = v
		}
		mcpInstance.toolsMux.RUnlock()
		return true
	})

	dc.iotMux.RLock()
	if dc.iotOverMcp != nil {
		dc.iotOverMcp.toolsMux.RLock()
		for k, v := range dc.iotOverMcp.tools {
			tools[k] = v
		}
		dc.iotOverMcp.toolsMux.RUnlock()
	}
	dc.iotMux.RUnlock()
	return tools
}

func (dc *DeviceMcpSession) GetWsEndpointMcpTools() map[string]tool.InvokableTool {
	tools := make(map[string]tool.InvokableTool)
	dc.wsEndPointMcp.Range(func(_, value interface{}) bool {
		mcpInstance := value.(*McpClientInstance)
		mcpInstance.toolsMux.RLock()
		for k, v := range mcpInstance.tools {
			tools[k] = v
		}
		mcpInstance.toolsMux.RUnlock()
		return true
	})
	return tools
}

func (dc *DeviceMcpSession) GetToolByName(toolName string) (tool tool.InvokableTool, ok bool) {
	dc.wsEndPointMcp.Range(func(_, value interface{}) bool {
		mcpInstance := value.(*McpClientInstance)
		mcpInstance.toolsMux.RLock()
		logger.Infof("wsEndPointMcp toollist: %+v", mcpInstance.tools)
		if tool, ok = mcpInstance.tools[toolName]; ok {
			mcpInstance.toolsMux.RUnlock()
			return false
		}
		mcpInstance.toolsMux.RUnlock()
		return true
	})
	if ok {
		return tool, true
	}

	if dc.iotOverMcp != nil {
		dc.iotOverMcp.toolsMux.RLock()
		logger.Infof("iotOverMcp toollist: %+v", dc.iotOverMcp.tools)
		if tool, ok = dc.iotOverMcp.tools[toolName]; ok {
			dc.iotOverMcp.toolsMux.RUnlock()
			return tool, true
		}
		dc.iotOverMcp.toolsMux.RUnlock()
	}
	return nil, false
}
