package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/mqtt_server"
	"xiaozhi-esp32-server-golang/internal/app/server/chat"
	"xiaozhi-esp32-server-golang/internal/app/server/mqtt_udp"
	"xiaozhi-esp32-server-golang/internal/app/server/types"
	"xiaozhi-esp32-server-golang/internal/app/server/websocket"
	"xiaozhi-esp32-server-golang/internal/data/history"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"
	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/openclaw"
	"xiaozhi-esp32-server-golang/internal/pool"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spf13/viper"
)

// App unifiedmanageallprotocolserviceand ChatManager

type App struct {
	wsServer       *websocket.WebSocketServer
	mqttUdpAdapter *mqtt_udp.MqttUdpAdapter
	mqttUdpMu      sync.RWMutex

	// ChatManagermanage - useconcurrent map
	chatManagers cmap.ConcurrentMap[string, *chat.ChatManager]
}

func NewApp() *App {
	var err error
	app := &App{
		chatManagers: cmap.New[*chat.ChatManager](),
	}
	app.wsServer = app.newWebSocketServer()
	app.mqttUdpAdapter, err = app.newMqttUdpAdapter()
	if err != nil {
		log.Errorf("newMqttUdpAdapter err: %+v", err)
		return nil
	}
	return app
}

func (a *App) Run() {
	go a.wsServer.Start()
	log.Infof("enter Run, mqtt_server.enable: %v", viper.GetBool("mqtt_server.enable"))
	if viper.GetBool("mqtt_server.enable") {
		go func() {
			err := a.startMqttServer()
			if err != nil {
				log.Errorf("startMqttServer err: %+v", err)
			}
		}()
	}
	a.mqttUdpMu.RLock()
	adapter := a.mqttUdpAdapter
	a.mqttUdpMu.RUnlock()
	if adapter != nil {
		go adapter.Start() // non-blocking，joinandretryat adapter internalafter台execute
	}

	// registerchatrelevantoflocalMCPtool
	a.registerChatMCPTools()

	a.registerHandler()

	a.initEventHandle()

	// startresourcepoolcountmonitor（每5minute钟outputatimestolog）
	ctx := context.Background()
	pool.StartStatsMonitor(ctx, 5*time.Minute)

	// startresourcepoolcountup报（每5secondup报atimesto manager backend）
	pool.StartStatsReporter(ctx)

	select {} // blockmainthread
}

func (app *App) initEventHandle() {
	eventHandle, err := NewEventHandle(app)
	if err != nil {
		log.Errorf("initialize EventHandle failed: %v", err)
		return
	}
	if err := eventHandle.Start(); err != nil {
		log.Errorf("start EventHandle failed: %v", err)
		return
	}

	// initializemessageprocess器（总yes启use，unifiedprocessRedis+MemoryProvider+History）
	historyCfg := history.HistoryClientConfig{
		BaseURL:   util.GetBackendURL(),
		AuthToken: util.GetManagerAuthToken(),
		Timeout:   viper.GetDuration("manager.history_timeout"),
		Enabled:   true, // 总yes启use
	}
	NewMessageWorker(historyCfg)
	log.Info("messageprocess器alreadyinitialize")
}

func (app *App) currentMqttConfig() *mqtt_udp.MqttConfig {
	if !viper.GetBool("mqtt.enable") {
		return nil
	}
	return &mqtt_udp.MqttConfig{
		Broker:   viper.GetString("mqtt.broker"),
		Type:     viper.GetString("mqtt.type"),
		Port:     viper.GetInt("mqtt.port"),
		ClientID: viper.GetString("mqtt.client_id"),
		Username: viper.GetString("mqtt.username"),
		Password: viper.GetString("mqtt.password"),
	}
}

func (app *App) newMqttUdpAdapter() (*mqtt_udp.MqttUdpAdapter, error) {
	mqttConfig := app.currentMqttConfig()
	if mqttConfig == nil {
		return nil, nil
	}

	udpServer, err := app.newUdpServer()
	if err != nil {
		return nil, err
	}

	return mqtt_udp.NewMqttUdpAdapter(
		mqttConfig,
		mqtt_udp.WithUdpServer(udpServer),
		mqtt_udp.WithOnNewConnection(app.OnNewConnection),
	), nil
}

func (app *App) newUdpServer() (*mqtt_udp.UdpServer, error) {
	udpPort := viper.GetInt("udp.listen_port")
	externalHost := viper.GetString("udp.external_host")
	externalPort := viper.GetInt("udp.external_port")

	udpServer := mqtt_udp.NewUDPServer(udpPort, externalHost, externalPort)
	err := udpServer.Start()
	if err != nil {
		log.Fatalf("udpServer.Start err: %+v", err)
		return nil, err
	}
	return udpServer, nil
}

func (app *App) newWebSocketServer() *websocket.WebSocketServer {
	port := viper.GetInt("websocket.port")
	return websocket.NewWebSocketServer(
		port,
		websocket.WithOnNewConnection(app.OnNewConnection),
		websocket.WithOnOpenClawResponse(app.OnOpenClawResponse),
	)
}

func (app *App) startMqttServer() error {
	return mqtt_server.StartMqttServer()
}

// ReloadMqttServer 热更 MQTT Server：first停，再according to mqtt_server.enable decidewhetherstart（not启usethenonlystopnostart）
func (app *App) ReloadMqttServer() {
	_ = mqtt_server.StopMqttServer()
	if !viper.GetBool("mqtt_server.enable") {
		return
	}
	if err := app.startMqttServer(); err != nil {
		log.Errorf("ReloadMqttServer start: %v", err)
	}
}

// ReloadMqttUdp 热更 MQTT+UDP：first停旧adapter，再according to mqtt.enable decidewhether新建andstart（not启usethenonlystopnostart）
func (app *App) ReloadMqttUdp() {
	app.mqttUdpMu.Lock()
	old := app.mqttUdpAdapter
	app.mqttUdpAdapter = nil
	app.mqttUdpMu.Unlock()
	if old != nil {
		old.Stop()
	}
	if !viper.GetBool("mqtt.enable") {
		return
	}
	adapter, err := app.newMqttUdpAdapter()
	if err != nil {
		log.Errorf("ReloadMqttUdp newMqttUdpAdapter: %v", err)
		return
	}
	app.mqttUdpMu.Lock()
	app.mqttUdpAdapter = adapter
	app.mqttUdpMu.Unlock()
	time.Sleep(500 * time.Millisecond)
	go adapter.Start()
}

// ReloadMqttUdpWithFlags according tochangemarkdecidewhether热更 MQTT+UDP
func (app *App) ReloadMqttUdpWithFlags(doMqttReload, doUdpReload bool) {
	if !doMqttReload && !doUdpReload {
		return
	}
	if !viper.GetBool("mqtt.enable") {
		log.Infof("ReloadMqttUdpWithFlags: mqtt disabled, stopping mqtt+udp")
		app.ReloadMqttUdp()
		return
	}

	app.mqttUdpMu.RLock()
	adapter := app.mqttUdpAdapter
	app.mqttUdpMu.RUnlock()

	if adapter == nil {
		log.Infof("ReloadMqttUdpWithFlags: mqtt enabled but adapter is nil, starting mqtt+udp")
		newAdapter, err := app.newMqttUdpAdapter()
		if err != nil {
			log.Errorf("ReloadMqttUdpWithFlags newMqttUdpAdapter: %v", err)
			return
		}
		if newAdapter == nil {
			return
		}
		app.mqttUdpMu.Lock()
		app.mqttUdpAdapter = newAdapter
		app.mqttUdpMu.Unlock()
		time.Sleep(500 * time.Millisecond)
		go newAdapter.Start()
		return
	}

	if doMqttReload && doUdpReload {
		log.Infof("ReloadMqttUdpWithFlags: mqtt & udp config changed, reloading mqtt+udp")
		app.ReloadMqttUdp()
		return
	}
	if doMqttReload {
		log.Infof("ReloadMqttUdpWithFlags: mqtt config changed, reloading mqtt only")
		mqttConfig := app.currentMqttConfig()
		if mqttConfig == nil {
			app.ReloadMqttUdp()
			return
		}
		adapter.ReloadMqttClient(mqttConfig)
		return
	}
	if doUdpReload {
		log.Infof("ReloadMqttUdpWithFlags: udp listen changed, reloading udp only")
		udpServer, err := app.newUdpServer()
		if err != nil {
			log.Errorf("ReloadMqttUdpWithFlags newUdpServer: %v", err)
			return
		}
		adapter.ReloadUdpServer(udpServer)
	}
}

// ReloadMCP 热更 MCP：禁usewhenonlystopglobal MCP；启usewhenalreadystartthenrestartglobal MCP，notstartthenstart MCP cluster
func (app *App) ReloadMCP() error {
	if !viper.GetBool("mcp.global.enabled") {
		// 禁use：only停no启，avoid依赖 Start() insidejudgeormergewhen序
		if err := mcp.GetGlobalMCPManager().Stop(); err != nil {
			return err
		}
		return nil
	}
	mgr := mcp.GetMCPManager()
	if mgr.IsStarted() {
		if err := mgr.RestartManager("global"); err != nil {
			return err
		}
		return nil
	}
	if err := mcp.StartMCPManagers(); err != nil {
		return err
	}
	return nil
}

// allprotocol新joinare走这in
func (a *App) OnNewConnection(transport types.IConn) {
	deviceID := transport.GetDeviceID()

	// check ifalready存atthisdeviceofChatManager
	if existingManager, exists := a.chatManagers.Get(deviceID); exists {
		log.Infof("device %s already存atChatManager，firstclose旧ofjoin", deviceID)
		// close旧ofChatManager
		existingManager.Close()
		a.chatManagers.Remove(deviceID)
	}

	// create newChatManager
	chatManager, err := chat.NewChatManager(deviceID, transport)
	if err != nil {
		log.Errorf("createchatManagerfailed: %v", err)
		return
	}

	// storeChatManager
	a.chatManagers.Set(deviceID, chatManager)

	a.DeviceOnline(deviceID)

	log.Infof("device %s ofChatManageralreadycreateandstore", deviceID)

	// OpenClaw离线message补发（delayretry，avoidjoin刚建立whensessionnot yetnot initialized）
	go a.replayOpenClawOfflineMessages(deviceID)

	// startChatManager
	go func() {
		defer func() {
			// ChatManagerendwhen，frommapinremove
			if storedManager, exists := a.chatManagers.Get(deviceID); exists && storedManager == chatManager {
				a.chatManagers.Remove(deviceID)
				log.Infof("device %s ofChatManageralreadyfrommapinremove", deviceID)
				a.DeviceOffline(deviceID)
			}
		}()

		if err := chatManager.Start(); err != nil {
			log.Errorf("ChatManagerstartfailed: %v", err)
		}
	}()
}

// OnOpenClawResponse OpenClaw实whenresponddown发callback
func (a *App) OnOpenClawResponse(event openclaw.ResponseDelivery) bool {
	deviceID := strings.TrimSpace(event.DeviceID)
	if deviceID == "" {
		return false
	}
	chatManager, exists := a.GetChatManager(deviceID)
	if !exists || chatManager == nil {
		return false
	}
	if err := chatManager.InjectOpenClawResponse(event); err != nil {
		log.Warnf(
			"OpenClaw实whenmessage注入failed, device=%s correlation_id=%s start=%v end=%v err=%v",
			deviceID,
			strings.TrimSpace(event.CorrelationID),
			event.IsStart,
			event.IsEnd,
			err,
		)
		return false
	}
	return true
}

func (a *App) replayOpenClawOfflineMessages(deviceID string) {
	manager := openclaw.GetManager()
	const maxRetry = 10
	for i := 0; i < maxRetry; i++ {
		time.Sleep(1 * time.Second)
		delivered, remaining := manager.ReplayOfflineMessages(deviceID, func(msg openclaw.OfflineMessage) error {
			chatManager, exists := a.GetChatManager(deviceID)
			if !exists || chatManager == nil {
				return fmt.Errorf("chat manager not ready")
			}
			if strings.TrimSpace(msg.Text) == "" {
				return nil
			}
			return chatManager.InjectMessage(msg.Text, true)
		})
		if delivered > 0 {
			log.Infof("OpenClaw离线message补发successful, device=%s delivered=%d remaining=%d", deviceID, delivered, remaining)
		}
		if remaining == 0 {
			return
		}
	}
}

// GetChatManager getspecifydeviceofChatManager
func (a *App) GetChatManager(deviceID string) (*chat.ChatManager, bool) {
	return a.chatManagers.Get(deviceID)
}

// CloseChatManager closespecifydeviceofChatManager
func (a *App) CloseChatManager(deviceID string) bool {
	if manager, exists := a.chatManagers.Get(deviceID); exists {
		manager.Close()
		a.chatManagers.Remove(deviceID)
		log.Infof("device %s ofChatManageralreadycloseandremove", deviceID)
		return true
	}
	return false
}

// GetAllChatManagers getallChatManagerofreplica
func (a *App) GetAllChatManagers() map[string]*chat.ChatManager {
	// returnreplica以avoidconcurrentaccess问题
	managers := make(map[string]*chat.ChatManager)
	for tuple := range a.chatManagers.IterBuffered() {
		managers[tuple.Key] = tuple.Val
	}
	return managers
}

// GetChatManagerCount get current活跃ofChatManagercount
func (a *App) GetChatManagerCount() int {
	return a.chatManagers.Count()
}

// CloseAllChatManagers closeallChatManager
func (a *App) CloseAllChatManagers() {
	for tuple := range a.chatManagers.IterBuffered() {
		tuple.Val.Close()
		log.Infof("device %s ofChatManageralreadyclose", tuple.Key)
	}

	// clearmap
	a.chatManagers.Clear()
	log.Info("allChatManageralreadyclose")
}

// registerChatMCPTools registerchatrelevantoflocalMCPtool
func (s *App) registerChatMCPTools() {
	// callchatpackageofregisterfunction
	chat.RegisterChatMCPTools()

	log.Info("chatrelevantoflocalMCPtoolregistercomplete")
}

func (s *App) DeviceOnline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOnline, eventData)
}

func (s *App) DeviceOffline(deviceID string) {
	eventData := map[string]interface{}{
		"device_id": deviceID,
	}
	providerType := viper.GetString("config_provider.type")
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.NotifyDeviceEvent(context.Background(), config_types.EventDeviceOffline, eventData)
}

func (a *App) registerHandler() {
	providerType := viper.GetString("config_provider.type")
	log.Infof("registerHandler: config_provider.type=%s", providerType)
	provider, err := user_config.GetProvider(providerType)
	if err != nil {
		log.Errorf("GetProvider err: %+v", err)
		return
	}
	provider.RegisterMessageEventHandler(context.Background(), config_types.EventHandleMessageInject, a.HandleInjectMsg)
	log.Infof("registerHandler: registered paths=[%s]", config_types.EventHandleMessageInject)
}

// toclient-side注入message
func (a *App) HandleInjectMsg(ctx context.Context, eventType string, eventData map[string]interface{}) (string, error) {
	type InjectMsg struct {
		SkipLlm  bool   `json:"skip_llm"`
		DeviceId string `json:"device_id"`
		Message  string `json:"message"`
	}
	bodyBytes, _ := json.Marshal(eventData)
	var msg InjectMsg
	err := json.Unmarshal(bodyBytes, &msg)
	if err != nil {
		log.Errorf("HandleInjectMsg error: %+v", err)
		return "", fmt.Errorf("HandleInjectMsg error")
	}

	// validate必要parameter
	if msg.DeviceId == "" {
		log.Errorf("HandleInjectMsg: device_id is required")
		return "", fmt.Errorf("device_id is required")
	}
	if msg.Message == "" {
		log.Errorf("HandleInjectMsg: message is required")
		return "", fmt.Errorf("message is required")
	}

	// getspecifydeviceofChatManager
	chatManager, exists := a.GetChatManager(msg.DeviceId)
	if !exists {
		log.Errorf("HandleInjectMsg: device %s not found or offline", msg.DeviceId)
		return "", fmt.Errorf("device %s not found or offline", msg.DeviceId)
	}

	log.Debugf("HandleInjectMsg: injecting message to device %s, skip_llm: %v, message: %s",
		msg.DeviceId, msg.SkipLlm, msg.Message)

	// useChatManagerof公开method注入message
	err = chatManager.InjectMessage(msg.Message, msg.SkipLlm)
	if err != nil {
		log.Errorf("HandleInjectMsg: failed to inject message to device %s: %v", msg.DeviceId, err)
		return "", fmt.Errorf("failed to inject message: %v", err)
	}

	return "message injected successfully", nil
}
