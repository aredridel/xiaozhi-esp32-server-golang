package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/constants"
	"xiaozhi-esp32-server-golang/internal/app/server/chat/plugins"
	types_conn "xiaozhi-esp32-server-golang/internal/app/server/types"
	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	chathooks "xiaozhi-esp32-server-golang/internal/domain/chat/hooks"
	"xiaozhi-esp32-server-golang/internal/domain/chat/streamtransform"
	userconfig "xiaozhi-esp32-server-golang/internal/domain/config"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/openclaw"
	pkghooks "xiaozhi-esp32-server-golang/internal/pkg/hooks"
	log "xiaozhi-esp32-server-golang/logger"
)

type ChatManager struct {
	DeviceID  string
	transport types_conn.IConn

	clientState *ClientState
	session     *ChatSession
	ctx         context.Context
	cancel      context.CancelFunc

	// Close protection, prevents multiple closes
	closeOnce sync.Once
	closed    bool
}

type ChatManagerOption func(*ChatManager)

var (
	chatHookAsyncExecutorOnce sync.Once
	chatHookAsyncExecutor     *pkghooks.AsyncExecutor
)

func sharedChatHookAsyncExecutor() *pkghooks.AsyncExecutor {
	chatHookAsyncExecutorOnce.Do(func() {
		asyncCfg := pkghooks.AsyncConfig{
			QueueSize:    viper.GetInt("chat_hooks.async.queue_size"),
			WorkerCount:  viper.GetInt("chat_hooks.async.worker_count"),
			DropWhenFull: viper.GetBool("chat_hooks.async.drop_when_full"),
			Timeout:      time.Duration(viper.GetInt("chat_hooks.async.timeout_ms")) * time.Millisecond,
		}
		chatHookAsyncExecutor = pkghooks.NewAsyncExecutor(context.Background(), asyncCfg)
		log.Infof("Initializing global shared chat hook observer executor: queue_size=%d worker_count=%d drop_when_full=%v timeout=%s", asyncCfg.QueueSize, asyncCfg.WorkerCount, asyncCfg.DropWhenFull, asyncCfg.Timeout)
	})
	return chatHookAsyncExecutor
}

func newChatHookHub(parent context.Context) *chathooks.Hub {
	asyncCfg := pkghooks.AsyncConfig{
		QueueSize:    viper.GetInt("chat_hooks.async.queue_size"),
		WorkerCount:  viper.GetInt("chat_hooks.async.worker_count"),
		DropWhenFull: viper.GetBool("chat_hooks.async.drop_when_full"),
		Timeout:      time.Duration(viper.GetInt("chat_hooks.async.timeout_ms")) * time.Millisecond,
	}
	hub := chathooks.NewHub(parent, pkghooks.WithAsyncConfig(asyncCfg), pkghooks.WithAsyncExecutor(sharedChatHookAsyncExecutor()))
	stats := hub.Stats()
	log.Infof("Initializing chat hook hub: queue_size=%d worker_count=%d drop_when_full=%v timeout=%s dropped_async=%d", asyncCfg.QueueSize, asyncCfg.WorkerCount, asyncCfg.DropWhenFull, asyncCfg.Timeout, stats.DroppedAsync)
	return hub
}

func chatHookBuiltinOverrides() map[string]chathooks.BuiltinPluginConfig {
	overrides := map[string]chathooks.BuiltinPluginConfig{}
	for _, reg := range chathooks.BuiltinRegistrations() {
		path := "chat_hooks.plugins." + reg.Meta.Name
		cfg := chathooks.BuiltinPluginConfig{}
		if viper.IsSet(path + ".enabled") {
			enabled := viper.GetBool(path + ".enabled")
			cfg.Enabled = &enabled
		}
		if viper.IsSet(path + ".priority") {
			cfg.Priority = viper.GetInt(path + ".priority")
		}
		overrides[reg.Meta.Name] = cfg
	}
	return overrides
}

func NewChatManager(deviceID string, transport types_conn.IConn, options ...ChatManagerOption) (*ChatManager, error) {

	cm := &ChatManager{
		DeviceID:  deviceID,
		transport: transport,
	}

	for _, option := range options {
		option(cm)
	}

	ctx := context.WithValue(context.Background(), "chat_session_operator", ChatSessionOperator(cm))

	cm.ctx, cm.cancel = context.WithCancel(ctx)

	// Create clientState first, then register OnClose callback to avoid race conditions
	clientState, err := GenClientState(cm.ctx, cm.DeviceID)
	if err != nil {
		log.Errorf("Failed to initialize client state: %v", err)
		cm.transport.Close()
		return nil, err
	}
	cm.clientState = clientState

	// Register OnClose callback after clientState is created
	cm.transport.OnClose(cm.OnClose)

	serverTransport := NewServerTransport(cm.transport, clientState)
	hookHub := newChatHookHub(cm.ctx)
	if !viper.IsSet("chat_hooks.enabled") || viper.GetBool("chat_hooks.enabled") {
		if err := chathooks.RegisterBuiltinPlugins(hookHub, chatHookBuiltinOverrides()); err != nil {
			log.Errorf("Failed to register chat hook builtin plugins: %v", err)
			cm.transport.Close()
			return nil, err
		}
		log.Infof("Loaded chat hook plugins: %+v", hookHub.PluginMetas())
	}
	transformRegistry := streamtransform.NewRegistry()
	plugins.Init(transformRegistry)

	cm.session = NewChatSession(
		clientState,
		serverTransport,
		hookHub,
		transformRegistry,
	)

	return cm, nil
}

func GenClientState(pctx context.Context, deviceID string) (*ClientState, error) {
	configProvider, err := userconfig.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Failed to get user config provider: %+v", err)
		return nil, err
	}
	deviceConfig, err := configProvider.GetUserConfig(pctx, deviceID)
	if err != nil {
		log.Errorf("Failed to get device %s configuration: %+v", deviceID, err)
		return nil, err
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)
	deviceConfig.SpeakerChatMode = NormalizeSpeakerChatMode(deviceConfig.SpeakerChatMode)

	// Create context with cancellation capability
	ctx, cancel := context.WithCancel(pctx)

	maxSilenceDuration := viper.GetInt64("chat.chat_max_silence_duration")
	if !viper.IsSet("chat.chat_max_silence_duration") {
		maxSilenceDuration = 400
	}

	isDeviceActivated, err := configProvider.IsDeviceActivated(ctx, deviceID, "")
	if err != nil {
		log.Errorf("Failed to check device activation status: %v", err)
	}

	clientState := &ClientState{
		IsActivated:       isDeviceActivated,
		Dialogue:          &Dialogue{},
		Abort:             false,
		ListenMode:        "auto",
		ListenPhase:       ListenPhaseIdle,
		DeviceID:          deviceID,
		AgentID:           deviceConfig.AgentId,
		Ctx:               ctx,
		Cancel:            cancel,
		SystemPrompt:      deviceConfig.SystemPrompt,
		DeviceConfig:      deviceConfig,
		OutputAudioFormat: types_audio.AudioFormat{},
		OpusAudioBuffer:   make(chan []byte, 100),
		AsrAudioBuffer: &AsrAudioBuffer{
			PcmData:          make([]float32, 0),
			AudioBufferMutex: sync.RWMutex{},
		},
		VoiceStatus: VoiceStatus{
			HaveVoice:            false,
			HaveVoiceLastTime:    0,
			VoiceStop:            false,
			SilenceThresholdTime: maxSilenceDuration,
		},
		SessionCtx: Ctx{},
	}
	applyOutputAudioFormatForTTS(clientState)

	return clientState, nil
}

func applyOutputAudioFormatForTTS(clientState *ClientState) {
	clientState.OutputAudioFormat = types_audio.AudioFormat{
		SampleRate:    types_audio.SampleRate,
		Channels:      types_audio.Channels,
		FrameDuration: types_audio.FrameDuration,
		Format:        types_audio.Format,
	}
	ttsType := clientState.DeviceConfig.Tts.Provider
	// If using xiaozhi tts, fixed to 24000hz, 20ms frame length
	if ttsType == constants.TtsTypeXiaozhi {
		clientState.OutputAudioFormat.SampleRate = 24000
		clientState.OutputAudioFormat.FrameDuration = 20
	}
}

// ReloadDeviceConfig reloads device configuration and applies to current session
func (c *ChatManager) ReloadDeviceConfig(ctx context.Context) error {
	configProvider, err := userconfig.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return fmt.Errorf("failed to get config provider: %w", err)
	}

	deviceConfig, err := configProvider.GetUserConfig(ctx, c.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to get device configuration: %w", err)
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)
	deviceConfig.SpeakerChatMode = NormalizeSpeakerChatMode(deviceConfig.SpeakerChatMode)

	oldAgentID := c.clientState.AgentID
	c.clientState.AgentID = deviceConfig.AgentId
	c.clientState.DeviceConfig = deviceConfig
	c.clientState.SystemPrompt = deviceConfig.SystemPrompt
	// Clear speaker temporary TTS config after role switch to avoid old config pollution
	c.clientState.SpeakerTTSConfig = nil
	// OpenClaw mode status is maintained by openclaw manager per agent session, actively exit mode on config refresh.
	openclaw.GetManager().ExitMode(oldAgentID, c.DeviceID)
	openclaw.GetManager().ExitMode(c.clientState.AgentID, c.DeviceID)
	applyOutputAudioFormatForTTS(c.clientState)
	log.Infof("Device %s configuration refreshed, current agent=%s", c.DeviceID, deviceConfig.AgentId)
	return nil
}

func (c *ChatManager) Start() error {
	err := c.session.Start(c.ctx)
	if err != nil {
		log.Errorf("ChatManager startup failed: %v", err)
		return err
	}
	select {
	case <-c.ctx.Done():
	}
	return nil
}

// Close actively closes the connection
func (c *ChatManager) Close() error {
	c.closeOnce.Do(func() {
		if c.clientState != nil {
			log.Infof("Actively closing connection, device %s", c.clientState.DeviceID)
		}
		// Close session-level resources first
		if c.session != nil {
			c.session.Close()
		}

		// Finally cancel manager-level context
		c.cancel()
	})
	return nil
}

func (c *ChatManager) OnClose(deviceId string) {
	log.Infof("Device %s disconnected", deviceId)
	if c.session != nil && c.session.mediaPlayer != nil {
		c.session.mediaPlayer.DetachSession(true)
	}
	c.cancel()
	if c.clientState != nil {
		eventbus.Get().Publish(eventbus.TopicSessionEnd, c.clientState)
	}
	return
}

func (c *ChatManager) GetClientState() *ClientState {
	return c.clientState
}

func (c *ChatManager) GetDeviceId() string {
	return c.clientState.DeviceID
}

// GetSession gets ChatSession
func (c *ChatManager) GetSession() *ChatSession {
	return c.session
}

// InjectMessage injects message to device
func (c *ChatManager) InjectMessage(message string, skipLlm bool) error {
	if skipLlm {
		// Send text message directly to device, skip LLM processing
		return c.session.AddTextToTTSQueue(message)
	} else {
		// Process message through LLM
		return c.session.AddAsrResultToQueue(message, nil)
	}
}

func (c *ChatManager) InjectOpenClawResponse(event openclaw.ResponseDelivery) error {
	return c.session.InjectOpenClawResponse(event)
}
