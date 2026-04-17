package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// DeviceMcpSession represents a device MCP session, aggregating multiple MCP connections
type DeviceMcpSession struct {
	deviceID              string
	Ctx                   context.Context
	cancel                context.CancelFunc
	wsEndPointMcp         sync.Map
	iotOverMcpByTransport map[string]*McpClientInstance
	iotMux                sync.RWMutex
}

type mcpClientInitState uint32

const (
	mcpClientInitStateIdle mcpClientInitState = iota
	mcpClientInitStateInitializing
	mcpClientInitStateReady
)

func normalizeDeviceTransportType(transportType string) string {
	transportType = strings.TrimSpace(transportType)
	if transportType == "" {
		return "unknown"
	}
	return transportType
}

func buildIotServerName(deviceID, transportType string) string {
	return fmt.Sprintf("iot_over_mcp_%s_%s", deviceID, normalizeDeviceTransportType(transportType))
}

func (dcs *DeviceMcpSession) AddWsEndPointMcp(mcpClient *McpClientInstance) {
	dcs.wsEndPointMcp.Store(mcpClient.serverName, mcpClient)

	// set close callback
	mcpClient.SetOnCloseHandler(dcs.handleMcpClientClose)

	mcpClient.refreshTools()
}

func (dcs *DeviceMcpSession) SetIotOverMcp(transportType string, mcpClient *McpClientInstance) {
	transportType = normalizeDeviceTransportType(transportType)
	if mcpClient != nil {
		mcpClient.SetOnCloseHandler(dcs.handleMcpClientClose)
	}

	var old *McpClientInstance
	dcs.iotMux.Lock()
	// same device + transportType keeps single instance
	if existing := dcs.iotOverMcpByTransport[transportType]; existing != nil && existing != mcpClient {
		old = existing
	}
	dcs.iotOverMcpByTransport[transportType] = mcpClient
	dcs.iotMux.Unlock()

	// close old instance outside the lock, avoid executing cancel logic inside session lock
	if old != nil {
		old.setConnected(false)
		old.cancel()
	}
}

func (dcs *DeviceMcpSession) RemoveWsEndPointMcp(mcpClient *McpClientInstance) {
	dcs.wsEndPointMcp.Delete(mcpClient.serverName)
}

// GetDeviceID gets device ID
func (dcs *DeviceMcpSession) GetDeviceID() string {
	return dcs.deviceID
}

// handleMcpClientClose handles MCP client close event
func (dcs *DeviceMcpSession) handleMcpClientClose(instance *McpClientInstance, reason string) {
	logger.Infof("MCP client %s for device %s closed, reason: %s", instance.serverName, dcs.deviceID, reason)

	// remove closed client from session
	dcs.RemoveWsEndPointMcp(instance)
	dcs.removeIotOverMcpByInstance(instance)

	if !dcs.hasAnyClient() {
		logger.Infof("all MCP connections for device %s closed, cleaning up session", dcs.deviceID)
		dcs.cancel()
		mcpClientPool.RemoveMcpClient(dcs.deviceID)
	}
}

func (dcs *DeviceMcpSession) removeIotOverMcpByInstance(instance *McpClientInstance) {
	dcs.iotMux.Lock()
	defer dcs.iotMux.Unlock()
	for transportType, iotClient := range dcs.iotOverMcpByTransport {
		if iotClient == instance {
			delete(dcs.iotOverMcpByTransport, transportType)
		}
	}
}

func (dcs *DeviceMcpSession) hasAnyClient() bool {
	hasWsClient := false
	dcs.wsEndPointMcp.Range(func(_, _ interface{}) bool {
		hasWsClient = true
		return false
	})
	if hasWsClient {
		return true
	}

	dcs.iotMux.RLock()
	defer dcs.iotMux.RUnlock()
	return len(dcs.iotOverMcpByTransport) > 0
}

// McpClientInstance represents a specific MCP client connection
type McpClientInstance struct {
	serverName string
	mcpClient  *client.Client // MCP server connected from ws endpoint
	tools      map[string]tool.InvokableTool
	toolsState atomic.Value // map[string]tool.InvokableTool, replaced atomically on refresh, reads use snapshot
	serverInfo *mcp.InitializeResult
	Ctx        context.Context
	cancel     context.CancelFunc
	conn       ConnInterface
	initState  uint32
	lastPing   atomic.Int64
	connected  atomic.Bool

	// close callback
	onCloseHandler func(instance *McpClientInstance, reason string)
	closeOnce      sync.Once
}

// NewDeviceMCPSession creates new MCP client session
func NewDeviceMCPSession(deviceID string) *DeviceMcpSession {
	ctx, cancel := context.WithCancel(context.Background())

	deviceMcpClient := &DeviceMcpSession{
		deviceID:              deviceID,
		Ctx:                   ctx,
		cancel:                cancel,
		iotOverMcpByTransport: make(map[string]*McpClientInstance),
		iotMux:                sync.RWMutex{},
		// wsEndPointMcp: make(map[string]*McpClientInstance),
	}

	go deviceMcpClient.refreshToolsAndPing()

	return deviceMcpClient
}

func NewWsEndPointMcpClient(ctx context.Context, deviceID string, conn *websocket.Conn) *McpClientInstance {
	ctx, cancel := context.WithCancel(ctx)

	wsTransport, err := NewWebsocketTransport(conn)
	if err != nil {
		logger.Errorf("failed to create MCP client: %v", err)
		return nil
	}
	mcpClient := client.NewClient(wsTransport)

	wsEndPointMcp := &McpClientInstance{
		serverName: fmt.Sprintf("ws_endpoint_mcp_%s_%s", deviceID, conn.RemoteAddr().String()),
		mcpClient:  mcpClient,
		Ctx:        ctx,
		cancel:     cancel,
		initState:  uint32(mcpClientInitStateReady),
	}
	wsEndPointMcp.storeToolsSnapshot(make(map[string]tool.InvokableTool))
	wsEndPointMcp.setConnected(true)
	wsEndPointMcp.setLastPing(time.Now())
	mcpClient.OnNotification(wsEndPointMcp.handleJSONRPCNotification)

	// set transport close callback
	wsTransport.SetOnCloseHandler(wsEndPointMcp.handleTransportClose)

	wsEndPointMcp.sendInitlize(ctx)
	wsEndPointMcp.mcpClient.Start(ctx)
	return wsEndPointMcp
}

func NewIotOverMcpClient(deviceID string, transportType string, conn ConnInterface) *McpClientInstance {
	ctx, cancel := context.WithCancel(context.Background())

	wsTransport, err := NewIotOverMcpTransport(conn)
	if err != nil {
		logger.Errorf("failed to create MCP client: %v", err)
		return nil
	}
	mcpClient := client.NewClient(wsTransport)

	iotOverMcp := &McpClientInstance{
		serverName: buildIotServerName(deviceID, transportType),
		mcpClient:  mcpClient,
		Ctx:        ctx,
		cancel:     cancel,
		conn:       conn,
		initState:  uint32(mcpClientInitStateInitializing),
	}
	iotOverMcp.storeToolsSnapshot(make(map[string]tool.InvokableTool))
	iotOverMcp.setConnected(true)
	iotOverMcp.setLastPing(time.Now())
	wsTransport.SetNotificationHandler(iotOverMcp.handleJSONRPCNotification)

	// set transport close callback
	wsTransport.SetOnCloseHandler(iotOverMcp.handleTransportClose)

	return iotOverMcp
}

func (dc *McpClientInstance) startIotOverMcp() error {
	if err := dc.sendInitlize(dc.Ctx); err != nil {
		return err
	}
	dc.mcpClient.Start(dc.Ctx)
	return dc.refreshTools()
}

// refreshTools common tool list refresh logic
func (dc *McpClientInstance) refreshTools() error {
	if dc == nil || dc.mcpClient == nil {
		return fmt.Errorf("mcp client not initialized")
	}
	if dc.serverInfo == nil {
		return fmt.Errorf("client not initialized")
	}

	tools, err := dc.mcpClient.ListTools(dc.Ctx, mcp.ListToolsRequest{})
	if err != nil {
		logger.Errorf("failed to refresh tool list: %v", err)
		return err
	}

	// tool conversion can be heavy, complete outside lock to avoid blocking tool list reads
	convertedTools := ConvertMcpToolListToInvokableToolList(tools.Tools, dc.serverName, dc.mcpClient)

	dc.storeToolsSnapshot(convertedTools)

	logger.Infof("tool list refreshed successfully: %s got %d tools", dc.serverName, len(convertedTools))
	return nil
}

func (dc *McpClientInstance) GetServerName() string {
	return dc.serverName
}

func (dc *McpClientInstance) IsInitialized() bool {
	return dc != nil && dc.serverInfo != nil
}

func (dc *McpClientInstance) storeToolsSnapshot(tools map[string]tool.InvokableTool) {
	if dc == nil {
		return
	}
	if tools == nil {
		tools = make(map[string]tool.InvokableTool)
	}
	dc.tools = tools
	dc.toolsState.Store(tools)
}

func (dc *McpClientInstance) loadToolsSnapshot() map[string]tool.InvokableTool {
	if dc == nil {
		return nil
	}
	if snapshot := dc.toolsState.Load(); snapshot != nil {
		if tools, ok := snapshot.(map[string]tool.InvokableTool); ok {
			return tools
		}
	}
	return dc.tools
}

func (dc *McpClientInstance) copyToolsInto(dst map[string]tool.InvokableTool) {
	if dc == nil {
		return
	}
	for name, invokable := range dc.loadToolsSnapshot() {
		dst[name] = invokable
	}
}

func (dc *McpClientInstance) toolCount() int {
	return len(dc.loadToolsSnapshot())
}

func (dc *McpClientInstance) getToolByName(toolName string) (tool.InvokableTool, bool) {
	tools := dc.loadToolsSnapshot()
	invokable, ok := tools[toolName]
	return invokable, ok
}

func (dc *McpClientInstance) setConnected(connected bool) {
	if dc == nil {
		return
	}
	dc.connected.Store(connected)
}

func (dc *McpClientInstance) setLastPing(ts time.Time) {
	if dc == nil {
		return
	}
	if ts.IsZero() {
		dc.lastPing.Store(0)
		return
	}
	dc.lastPing.Store(ts.UnixNano())
}

func (dc *McpClientInstance) LastPing() time.Time {
	if dc == nil {
		return time.Time{}
	}
	unixNano := dc.lastPing.Load()
	if unixNano == 0 {
		return time.Time{}
	}
	return time.Unix(0, unixNano)
}

func (dc *McpClientInstance) getInitState() mcpClientInitState {
	if dc == nil {
		return mcpClientInitStateIdle
	}
	return mcpClientInitState(atomic.LoadUint32(&dc.initState))
}

func (dc *McpClientInstance) setInitState(state mcpClientInitState) {
	if dc == nil {
		return
	}
	atomic.StoreUint32(&dc.initState, uint32(state))
}

func (dc *McpClientInstance) IsInitializing() bool {
	return dc.getInitState() == mcpClientInitStateInitializing
}

func (dc *McpClientInstance) IsReady() bool {
	return dc.getInitState() == mcpClientInitStateReady
}

func (dc *McpClientInstance) closeWithReason(reason string) {
	if dc == nil {
		return
	}
	dc.closeOnce.Do(func() {
		logger.Infof("MCP client %s closed, reason: %s", dc.serverName, reason)

		dc.setConnected(false)
		dc.setInitState(mcpClientInitStateIdle)
		dc.cancel()

		if dc.onCloseHandler != nil {
			dc.onCloseHandler(dc, reason)
		}
	})
}

func (dc *DeviceMcpSession) snapshotWsEndpointClients() []*McpClientInstance {
	clients := make([]*McpClientInstance, 0)
	dc.wsEndPointMcp.Range(func(_, value interface{}) bool {
		mcpInstance, ok := value.(*McpClientInstance)
		if ok && mcpInstance != nil {
			clients = append(clients, mcpInstance)
		}
		return true
	})
	return clients
}

func (dc *DeviceMcpSession) snapshotIotClients() []*McpClientInstance {
	dc.iotMux.RLock()
	defer dc.iotMux.RUnlock()

	clients := make([]*McpClientInstance, 0, len(dc.iotOverMcpByTransport))
	for _, instance := range dc.iotOverMcpByTransport {
		if instance != nil {
			clients = append(clients, instance)
		}
	}
	return clients
}

type iotTransportClientSnapshot struct {
	transportType string
	client        *McpClientInstance
}

func (dc *DeviceMcpSession) snapshotIotTransports() []iotTransportClientSnapshot {
	dc.iotMux.RLock()
	defer dc.iotMux.RUnlock()

	clients := make([]iotTransportClientSnapshot, 0, len(dc.iotOverMcpByTransport))
	for transportType, instance := range dc.iotOverMcpByTransport {
		if instance != nil {
			clients = append(clients, iotTransportClientSnapshot{
				transportType: transportType,
				client:        instance,
			})
		}
	}
	return clients
}

func (dc *DeviceMcpSession) heartbeatMcpInstance(mcpInstance *McpClientInstance) {
	if mcpInstance == nil || !mcpInstance.IsInitialized() {
		return
	}
	if err := mcpInstance.refreshTools(); err != nil {
		logger.Warnf("device %s heartbeat tool list refresh failed, actively destroying runtime: %v", mcpInstance.serverName, err)
		mcpInstance.closeWithReason("refresh_tools_failed")
		return
	}
	err := mcpInstance.mcpClient.Ping(mcpInstance.Ctx)
	if err == nil {
		mcpInstance.setLastPing(time.Now())
		logger.Debugf("device %s ping succeeded", mcpInstance.serverName)
	} else {
		logger.Warnf("device %s ping failed: %v", mcpInstance.serverName, err)
	}
}

func (dc *DeviceMcpSession) refreshToolsAndPing() {
	// only fetch tool list once during initialization
	findTools := func(mcpInstance *McpClientInstance) {
		if mcpInstance == nil || !mcpInstance.IsInitialized() {
			return
		}
		mcpInstance.refreshTools()
	}

	// fetch tool list during initialization
	for _, instance := range dc.snapshotWsEndpointClients() {
		findTools(instance)
	}

	for _, instance := range dc.snapshotIotClients() {
		findTools(instance)
	}

	// ping every 2 minutes
	pingTick := time.NewTicker(2 * time.Minute)
	defer pingTick.Stop()

	for {
		select {
		case <-dc.Ctx.Done():
			logger.Infof("device %s session cancelled, stopping ping", dc.deviceID)
			return
		case <-pingTick.C:
			for _, instance := range dc.snapshotWsEndpointClients() {
				dc.heartbeatMcpInstance(instance)
			}
			for _, instance := range dc.snapshotIotClients() {
				dc.heartbeatMcpInstance(instance)
			}
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
		logger.Errorf("failed to get tool list: %v", err)
		return nil, err
	}
	return tools, nil
}

// handleJSONRPCNotification handles JSON-RPC notification
func (dc *McpClientInstance) handleJSONRPCNotification(notification mcp.JSONRPCNotification) {
	switch notification.Method {
	case "notifications/progress":
		//handleProgressNotification(notification)
	case "notifications/message":
		//handleMessageNotification(notification)
	case "notifications/resources/updated":
		//handleResourceUpdateNotification(notification)
	case "notifications/tools/updated":
		// received tool update notification, refresh tool list
		logger.Infof("received tool update notification, refreshing tool list")
		go dc.refreshToolsOnNotification()
	default:
		log.Printf("Unknown notification: %s", notification.Method)
	}
}

// refreshToolsOnNotification refreshes tool list on notification
func (dc *McpClientInstance) refreshToolsOnNotification() {
	// add short delay to avoid frequent refresh
	time.Sleep(100 * time.Millisecond)
	dc.refreshTools()
}

// handleJSONRPCError handles JSON-RPC error
func (dc *McpClientInstance) handleJSONRPCError(errMsg mcp.JSONRPCError) error {
	logger.Errorf("received MCP server error: %+v", errMsg.Error)
	return nil
}

// handleTransportClose handles transport layer close event
func (dc *McpClientInstance) handleTransportClose(reason string) {
	dc.closeWithReason(reason)
}

// SetOnCloseHandler sets close callback
func (dc *McpClientInstance) SetOnCloseHandler(handler func(instance *McpClientInstance, reason string)) {
	dc.onCloseHandler = handler
}

// IsConnected checks if connection is still active
func (dc *McpClientInstance) IsConnected() bool {
	if dc == nil {
		return false
	}
	return dc.connected.Load()
}

func (dc *DeviceMcpSession) ShouldScheduleIotInit(transportType string, conn ConnInterface) bool {
	transportType = normalizeDeviceTransportType(transportType)
	if transportType == "unknown" || conn == nil {
		return false
	}

	dc.iotMux.RLock()
	existing := dc.iotOverMcpByTransport[transportType]
	dc.iotMux.RUnlock()
	if existing == nil {
		return true
	}
	if existing.conn != conn {
		return true
	}

	switch existing.getInitState() {
	case mcpClientInitStateReady, mcpClientInitStateInitializing:
		return false
	default:
		return true
	}
}

// GetConnectionStatus gets connection status info
func (dc *McpClientInstance) GetConnectionStatus() map[string]interface{} {
	toolsCount := dc.toolCount()

	initState := "idle"
	switch dc.getInitState() {
	case mcpClientInitStateInitializing:
		initState = "initializing"
	case mcpClientInitStateReady:
		initState = "ready"
	}

	return map[string]interface{}{
		"server_name": dc.serverName,
		"connected":   dc.IsConnected(),
		"init_state":  initState,
		"last_ping":   dc.LastPing(),
		"tools_count": toolsCount,
	}
}

func (dc *McpClientInstance) RawCallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	if dc == nil || dc.mcpClient == nil {
		return "", fmt.Errorf("MCP client not initialized")
	}
	if !dc.IsConnected() || !dc.IsInitialized() {
		return "", fmt.Errorf("MCP client not ready")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := dc.mcpClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %v", err)
	}

	resultBytes, err := result.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool call result: %v", err)
	}
	return string(resultBytes), nil
}

// GetTools gets tool list
func (dc *DeviceMcpSession) GetTools() map[string]tool.InvokableTool {
	tools := make(map[string]tool.InvokableTool)
	for _, mcpInstance := range dc.snapshotWsEndpointClients() {
		mcpInstance.copyToolsInto(tools)
	}

	for _, iotClient := range dc.snapshotIotClients() {
		iotClient.copyToolsInto(tools)
	}
	return tools
}

func (dc *DeviceMcpSession) GetWsEndpointMcpTools() map[string]tool.InvokableTool {
	tools := make(map[string]tool.InvokableTool)
	for _, mcpInstance := range dc.snapshotWsEndpointClients() {
		mcpInstance.copyToolsInto(tools)
	}
	return tools
}

// GetPreferredIotTransportType returns the transport best suited for device-level MCP queries/calls.
// Prefers connected transport with recent heartbeat; falls back to the most recent existing transport.
func (dc *DeviceMcpSession) GetPreferredIotTransportType() string {
	preferredTransport := ""
	var preferredClient *McpClientInstance
	isSupportedTransport := func(transportType string) bool {
		switch normalizeDeviceTransportType(transportType) {
		case "websocket", "udp", "mqtt_udp":
			return true
		default:
			return false
		}
	}

	selectPreferred := func(connectedOnly bool) string {
		preferredTransport = ""
		preferredClient = nil
		for _, snapshot := range dc.snapshotIotTransports() {
			transportType := snapshot.transportType
			iotClient := snapshot.client
			transportType = normalizeDeviceTransportType(transportType)
			if iotClient == nil {
				continue
			}
			if !isSupportedTransport(transportType) {
				continue
			}
			if connectedOnly && !iotClient.IsConnected() {
				continue
			}
			if preferredClient == nil {
				preferredTransport = transportType
				preferredClient = iotClient
				continue
			}
			currentLastPing := iotClient.LastPing()
			preferredLastPing := preferredClient.LastPing()
			if currentLastPing.After(preferredLastPing) {
				preferredTransport = transportType
				preferredClient = iotClient
				continue
			}
			if currentLastPing.Equal(preferredLastPing) && transportType < preferredTransport {
				preferredTransport = transportType
				preferredClient = iotClient
			}
		}
		return preferredTransport
	}

	if transportType := selectPreferred(true); transportType != "" {
		return transportType
	}
	return selectPreferred(false)
}

func (dc *DeviceMcpSession) GetIotToolsByTransport(transportType string) map[string]tool.InvokableTool {
	transportType = strings.TrimSpace(transportType)
	tools := make(map[string]tool.InvokableTool)
	if transportType == "" {
		return tools
	}

	dc.iotMux.RLock()
	iotClient := dc.iotOverMcpByTransport[transportType]
	dc.iotMux.RUnlock()
	if iotClient == nil {
		return tools
	}

	iotClient.copyToolsInto(tools)

	return tools
}

func (dc *DeviceMcpSession) GetIotToolByTransportAndName(transportType, toolName string) (tool.InvokableTool, bool) {
	transportType = strings.TrimSpace(transportType)
	if transportType == "" {
		return nil, false
	}

	dc.iotMux.RLock()
	iotClient := dc.iotOverMcpByTransport[transportType]
	dc.iotMux.RUnlock()
	if iotClient == nil {
		return nil, false
	}

	return iotClient.getToolByName(toolName)
}

func (dc *DeviceMcpSession) RawCallIotToolByTransport(ctx context.Context, transportType, toolName string, arguments map[string]interface{}) (string, bool, error) {
	transportType = strings.TrimSpace(transportType)
	if transportType == "" {
		return "", false, nil
	}

	dc.iotMux.RLock()
	iotClient := dc.iotOverMcpByTransport[transportType]
	dc.iotMux.RUnlock()
	if iotClient == nil || !iotClient.IsConnected() || !iotClient.IsInitialized() {
		return "", false, nil
	}

	result, err := iotClient.RawCallTool(ctx, toolName, arguments)
	return result, true, err
}

func (dc *DeviceMcpSession) RawCallWsEndpointTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, bool, error) {
	var selected *McpClientInstance
	for _, mcpInstance := range dc.snapshotWsEndpointClients() {
		if mcpInstance == nil || !mcpInstance.IsConnected() || !mcpInstance.IsInitialized() {
			continue
		}
		selected = mcpInstance
		break
	}
	if selected == nil {
		return "", false, nil
	}

	result, err := selected.RawCallTool(ctx, toolName, arguments)
	return result, true, err
}

func (dc *DeviceMcpSession) GetToolByName(toolName string) (tool tool.InvokableTool, ok bool) {
	for _, mcpInstance := range dc.snapshotWsEndpointClients() {
		if tool, ok = mcpInstance.getToolByName(toolName); ok {
			return tool, true
		}
	}
	if ok {
		return tool, true
	}

	for _, iotClient := range dc.snapshotIotClients() {
		if tool, ok = iotClient.getToolByName(toolName); ok {
			return tool, true
		}
	}
	return nil, false
}
