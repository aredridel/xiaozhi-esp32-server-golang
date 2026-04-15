package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	types_conn "xiaozhi-esp32-server-golang/internal/app/server/types"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/data/history"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	chathooks "xiaozhi-esp32-server-golang/internal/domain/chat/hooks"
	"xiaozhi-esp32-server-golang/internal/domain/chat/streamtransform"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/memory"
	"xiaozhi-esp32-server-golang/internal/domain/memory/llm_memory"
	"xiaozhi-esp32-server-golang/internal/domain/openclaw"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

type AsrResponseChannelItem struct {
	ctx           context.Context
	text          string
	speakerResult *speaker.IdentifyResult
}

type ChatSession struct {
	clientState     *ClientState
	asrManager      *ASRManager
	ttsManager      *TTSManager
	llmManager      *LLMManager
	speakerManager  *SpeakerManager
	mediaPlayer     *SessionMediaPlayer
	serverTransport *ServerTransport

	ctx    context.Context
	cancel context.CancelFunc

	chatTextQueue *util.Queue[AsrResponseChannelItem]

	// Speaker identification result cache (protected by lock)
	speakerResultMu        sync.RWMutex
	pendingSpeakerResult   *speaker.IdentifyResult
	speakerResultReady     chan struct{} // Only used for ready notification, no data passed
	turnSpeakerInterrupted atomic.Bool

	// Hello idempotency control: MQTT-UDP short-term offline reconnect will resend hello, avoid repeated initialization causing resource leaks.
	helloMu        sync.Mutex
	helloInited    bool
	vadLoopStarted bool
	mcpHelloInited bool
	listenStartSeq atomic.Uint64

	// When unactivated device triggers frequently, reuse recent "unactivated" judgment within short time to avoid frequent API calls.
	activationCheckMu     sync.Mutex
	lastActivationFalseAt time.Time

	// Close protection, prevents multiple closes
	closeOnce sync.Once
	closed    bool

	// stopSpeaking protection, prevents concurrent conflict with AddAsrResultToQueue/HandleWelcome
	stopSpeakingMu sync.Mutex

	openClawStreamMu sync.Mutex
	openClawStreams  map[string]chan llm_common.LLMResponseStruct

	openClawWarmupMu sync.Mutex
	openClawWarmup   *openClawWarmupTask

	hookHub *chathooks.Hub
}

type ChatSessionOption func(*ChatSession)

func NewChatSession(clientState *ClientState, serverTransport *ServerTransport, hookHub *chathooks.Hub, transformRegistry *streamtransform.Registry, opts ...ChatSessionOption) *ChatSession {
	s := &ChatSession{
		clientState:        clientState,
		serverTransport:    serverTransport,
		chatTextQueue:      util.NewQueue[AsrResponseChannelItem](10),
		speakerResultReady: make(chan struct{}, 1), // Buffer of 1 to avoid blocking
		openClawStreams:    make(map[string]chan llm_common.LLMResponseStruct),
		hookHub:            hookHub,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.asrManager = NewASRManager(clientState, serverTransport)
	s.asrManager.session = s // Set session reference
	s.ttsManager = NewTTSManager(clientState, serverTransport, s)
	s.mediaPlayer = NewSessionMediaPlayer(s)
	s.llmManager = NewLLMManager(clientState, serverTransport, s.ttsManager, s, transformRegistry)

	// If speaker identification is enabled, create speaker manager
	if clientState.IsSpeakerEnabled() {
		// Get speaker service address from system config (viper)
		baseURL := viper.GetString("voice_identify.base_url")
		if baseURL != "" {
			// Set service address and threshold in config
			speakerConfig := map[string]interface{}{
				"base_url": baseURL,
			}
			// Read threshold config, use default 0.6 if not configured
			if viper.IsSet("voice_identify.threshold") {
				threshold := viper.GetFloat64("voice_identify.threshold")
				speakerConfig["threshold"] = threshold
			}

			provider, err := speaker.GetSpeakerProvider(speakerConfig)
			if err != nil {
				log.Warnf("Failed to create speaker identification provider: %v", err)
			} else {
				clientState.SpeakerProvider = provider
				s.speakerManager = NewSpeakerManager(provider)
				log.Debugf("Device %s speaker identification enabled", clientState.DeviceID)

				// Set callback for async speaker identification result retrieval
				clientState.OnVoiceSilenceSpeakerCallback = func(ctx context.Context) {
					log.Debugf("[Speaker Identification] OnVoiceSilenceSpeakerCallback called, deviceID: %s", clientState.DeviceID)

					// Get speaker result asynchronously
					go func() {
						log.Debugf("[Speaker Identification] Starting async speaker identification result retrieval, deviceID: %s", clientState.DeviceID)

						// Check if speakerManager is active
						if !s.speakerManager.IsActive() {
							//log.Warnf("[Speaker Identification] speakerManager not active, cannot get identification result")
							return
						}
						// Clear previous results
						s.speakerResultMu.Lock()
						oldResult := s.pendingSpeakerResult
						s.pendingSpeakerResult = nil
						s.speakerResultMu.Unlock()
						if oldResult != nil {
							log.Debugf("[Speaker Identification] Cleared previous identification result: identified=%v, speaker_id=%s", oldResult.Identified, oldResult.SpeakerID)
						}

						// Clear ready notification (non-blocking)
						select {
						case <-s.speakerResultReady:
							log.Debugf("[Speaker Identification] Cleared ready notification channel")
						default:
							log.Debugf("[Speaker Identification] Ready notification channel already empty")
						}

						result, err := s.speakerManager.FinishAndIdentify(ctx)
						if err != nil {
							log.Warnf("[Speaker Identification] Failed to get speaker identification result: %v, deviceID: %s", err, clientState.DeviceID)
							// Speaker identification failure doesn't affect main flow, store nil result
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = nil
							s.speakerResultMu.Unlock()
							log.Debugf("[Speaker Identification] Stored nil result (identification failed)")
						} else if result != nil && result.Identified {
							log.Infof("[Speaker Identification] Speaker identified: %s (confidence: %.4f, threshold: %.4f), deviceID: %s",
								result.SpeakerName, result.Confidence, result.Threshold, clientState.DeviceID)
							log.Debugf("[Speaker Identification] Identification result details: speaker_id=%s, speaker_name=%s, confidence=%.4f, threshold=%.4f",
								result.SpeakerID, result.SpeakerName, result.Confidence, result.Threshold)
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[Speaker Identification] Stored identification result (identified)")
						} else {
							// No speaker identified, also store result
							if result != nil {
								log.Debugf("[Speaker Identification] No speaker identified: identified=%v, confidence=%.4f, threshold=%.4f, deviceID: %s",
									result.Identified, result.Confidence, result.Threshold, clientState.DeviceID)
							} else {
								log.Debugf("[Speaker Identification] Identification result is nil, deviceID: %s", clientState.DeviceID)
							}
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[Speaker Identification] Stored identification result (not identified)")
						}

						// Notify result ready
						select {
						case s.speakerResultReady <- struct{}{}:
							log.Debugf("[Speaker Identification] Sent result ready notification, deviceID: %s", clientState.DeviceID)
						default:
							log.Warnf("[Speaker Identification] Result ready notification channel full, cannot send notification, deviceID: %s", clientState.DeviceID)
						}
					}()
				}
			}
		}
	}

	// Set callback for ASR first text return
	clientState.OnAsrFirstTextCallback = func(text string, isFinal bool) {
		clientState.Asr.MarkTextReceived()
		log.Debugf("ASR first text returned: device=%s, text=%s, isFinal=%v", clientState.DeviceID, text, isFinal)
		clientState.MarkAsrFirstText()
		s.TraceAsrFirstText(clientState.Ctx, time.Now().UnixMilli())
		if clientState.IsRealTime() && viper.GetInt("chat.realtime_mode") == 4 {
			if s.isRealtimeMcpAudioGateActive() {
				log.Debugf("Device %s realtime media playback gate active, skipping ASR first text interrupt: text=%s", clientState.DeviceID, text)
				return
			}
			clientState.AfterAsrSessionCtx.CancelWithReason("ChatSession.OnAsrFirstTextCallback: realtime_mode=4")
			s.InterruptAndClearTTSQueue()
		}
	}

	return s
}

func (s *ChatSession) Start(pctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(pctx)

	err := s.InitAsrLlmTts()
	if err != nil {
		log.Errorf("Failed to initialize ASR/LLM/TTS: %v", err)
		return err
	}

	// Load historical messages asynchronously, don't block session startup
	go func() {
		err := s.initHistoryMessages()
		if err != nil {
			log.Errorf("Failed to initialize conversation history: %v", err)
		}
	}()

	go s.CmdMessageLoop(s.ctx)   // Process signaling messages
	go s.AudioMessageLoop(s.ctx) // Process audio data
	go s.processChatText(s.ctx)  // Process post-asr conversation messages
	go s.llmManager.Start(s.ctx) // Process post-llm series of response messages
	go s.ttsManager.Start(s.ctx) // Process tts message queue
	if s.mediaPlayer != nil {
		s.mediaPlayer.AttachSession()
	}

	return nil
}

// Initialize historical conversation records into memory
func (s *ChatSession) initHistoryMessages() error {
	var historyMessages []*schema.Message
	var err error

	if s.clientState.GetMemoryMode() == MemoryModeNone {
		log.Debugf("Device %s memory mode=none, skipping historical message loading", s.clientState.DeviceID)
		return nil
	}

	// Select data source based on configuration (no priority relationship, direct selection)
	useRedis := s.shouldUseRedis()
	useManager := s.shouldUseManager()

	// Validate required fields: DeviceID cannot be empty
	if s.clientState.DeviceID == "" {
		log.Debugf("DeviceID is empty, skipping historical message loading (may be called before hello message)")
		return nil
	}

	// Select data source based on configuration (no priority relationship, direct selection)
	if useRedis {
		// Load from Redis
		historyMessages, err = llm_memory.Get().GetMessages(
			s.ctx,
			s.clientState.DeviceID,
			s.clientState.AgentID,
			20)
		if err != nil {
			log.Warnf("Failed to load historical messages from Redis: %v", err)
			return err
		}
		log.Infof("Loaded %d historical messages from Redis", len(historyMessages))
	} else if useManager {
		// Load from Manager
		historyMessages, err = s.loadFromManager()
		if err != nil {
			log.Warnf("Failed to load historical messages from Manager: %v", err)
			return err
		}
		log.Infof("Loaded %d historical messages from Manager", len(historyMessages))
	} else {
		// Neither data source configured, skip historical message loading
		log.Debugf("Neither Redis nor Manager configured, skipping historical message loading")
		return nil
	}

	if len(historyMessages) > 0 {
		s.clientState.InitMessages(historyMessages)
		log.Infof("Successfully loaded %d historical messages", len(historyMessages))
	} else {
		log.Debugf("No historical messages loaded (may have no history)")
	}

	return nil
}

// shouldUseRedis determines whether to use Redis as data source
func (s *ChatSession) shouldUseRedis() bool {
	// Determine based on config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "redis"
}

// shouldUseManager determines whether to use Manager as data source
func (s *ChatSession) shouldUseManager() bool {
	// Determine based on config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "manager"
}

// loadFromManager loads historical messages from Manager database
func (s *ChatSession) loadFromManager() ([]*schema.Message, error) {
	// Create HistoryClient
	historyCfg := history.HistoryClientConfig{
		BaseURL:   util.GetBackendURL(),
		AuthToken: util.GetManagerAuthToken(),
		Timeout:   viper.GetDuration("manager.history_timeout"),
		Enabled:   true,
	}
	client := history.NewHistoryClient(historyCfg)

	if s.clientState.DeviceID == "" || s.clientState.AgentID == "" {
		return []*schema.Message{}, nil
	}

	req := &history.GetMessagesRequest{
		DeviceID:  s.clientState.DeviceID,
		AgentID:   s.clientState.AgentID,
		SessionID: s.clientState.SessionID,
		Limit:     20,
	}

	resp, err := client.GetMessages(s.ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to schema.Message format
	messages := make([]*schema.Message, 0, len(resp.Messages))
	for _, item := range resp.Messages {
		var msg *schema.Message
		switch item.Role {
		case "user":
			msg = schema.UserMessage(item.Content)
		case "assistant":
			msg = schema.AssistantMessage(item.Content, item.ToolCalls)
		case "tool":
			msg = schema.ToolMessage(item.Content, item.ToolCallID)
		case "system":
			msg = schema.SystemMessage(item.Content)
		default:
			log.Warnf("Unknown message role: %s", item.Role)
			continue
		}

		messages = append(messages, msg)
	}

	for _, msg := range messages {
		log.Debugf("Historical message: %+v", msg)
	}

	return messages, nil
}

// Called after mqtt receives type: listen, state: start
func (c *ChatSession) InitAsrLlmTts() error {
	// Initialize asr structure
	c.clientState.InitAsr()

	// Initialize memory (memory not in resource pool)
	memoryMode := c.clientState.GetMemoryMode()
	memoryConfig := c.clientState.DeviceConfig.Memory
	memoryType := memory.MemoryType(memoryConfig.Provider)
	if memoryMode != MemoryModeLong {
		memoryType = memory.MemoryTypeNone
	}

	memoryProvider, err := memory.GetProvider(memoryType, memoryConfig.Config)
	if err != nil {
		return fmt.Errorf("Failed to create Memory provider: %v", err)
	}
	c.clientState.MemoryProvider = memoryProvider

	if memoryMode == MemoryModeLong {
		// Initialize memory context (only for long memory mode)
		context, err := memoryProvider.GetContext(c.ctx, c.clientState.GetDeviceIDOrAgentID(), 500)
		if err != nil {
			log.Warnf("Failed to initialize memory context: %v", err)
		}
		c.clientState.MemoryContext = context
	} else {
		c.clientState.MemoryContext = ""
	}

	return nil
}

func (c *ChatSession) CmdMessageLoop(ctx context.Context) {
	recvFailCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Infof("Device %s recvCmd context cancel", c.clientState.DeviceID)
			return
		default:
		}

		if recvFailCount > 3 {
			log.Errorf("recv cmd timeout: %v", recvFailCount)
			return
		}

		message, err := c.serverTransport.RecvCmd(ctx, 120)
		if err != nil {
			log.Errorf("recv cmd error: %v", err)
			recvFailCount = recvFailCount + 1
			continue
		}
		if message == nil {
			continue
		}
		recvFailCount = 0
		log.Infof("Received text message: %s", string(message))
		if err := c.HandleTextMessage(message); err != nil {
			log.Errorf("Failed to process text message: %v, message content: %s", err, string(message))
			continue
		}
	}
}

func (c *ChatSession) AudioMessageLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debugf("Device %s recvCmd context cancel", c.clientState.DeviceID)
			return
		default:
		}
		message, err := c.serverTransport.RecvAudio(ctx, 600)
		if err != nil {
			log.Errorf("recv audio error: %v", err)
			return
		}
		if message == nil {
			continue
		}
		log.Debugf("Received audio data, size: %d bytes", len(message))
		isAuth := viper.GetBool("auth.enable")
		if isAuth {
			if !c.clientState.IsActivated {
				log.Debugf("Device %s not activated, skipping audio data", c.clientState.DeviceID)
				continue
			}
		}
		if c.clientState.GetClientVoiceStop() {
			log.Debug("Client stopped speaking, skipping audio data")
			continue
		}

		if ok := c.HandleAudioMessage(message); !ok {
			log.Errorf("Audio buffer full: %v", err)
		}
	}
}

// handleTextMessage processes text messages
func (c *ChatSession) HandleTextMessage(message []byte) error {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		log.Errorf("Failed to parse message: %v", err)
		return fmt.Errorf("Failed to parse message: %v", err)
	}

	// Process different message types
	switch clientMsg.Type {
	case MessageTypeHello:
		return c.HandleHelloMessage(&clientMsg)
	case MessageTypeListen:
		return c.HandleListenMessage(&clientMsg)
	case MessageTypeAbort:
		return c.HandleAbortMessage(&clientMsg)
	case MessageTypeIot:
		return c.HandleIoTMessage(&clientMsg)
	case MessageTypeMcp:
		return c.HandleMcpMessage(&clientMsg)
	case MessageTypeGoodBye:
		return c.HandleGoodByeMessage(&clientMsg)
	default:
		// Unknown message type, echo directly
		return fmt.Errorf("Unknown message type: %s", clientMsg.Type)
	}
}

// HandleAudioMessage processes audio messages
func (c *ChatSession) HandleAudioMessage(data []byte) bool {
	select {
	case c.clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("Audio buffer full, discarding audio data")
	}
	return false
}

// handleHelloMessage processes hello messages
func (s *ChatSession) HandleHelloMessage(msg *ClientMessage) error {
	if msg.Transport == types_conn.TransportTypeWebsocket {
		return s.HandleWebsocketHelloMessage(msg)
	} else if msg.Transport == types_conn.TransportTypeMqttUdp {
		return s.HandleMqttHelloMessage(msg)
	}
	return fmt.Errorf("Unsupported transport type: %s", msg.Transport)
}

func (s *ChatSession) HandleMqttHelloMessage(msg *ClientMessage) error {
	if err := s.HandleCommonHelloMessage(msg); err != nil {
		return err
	}

	clientState := s.clientState

	udpExternalHost := viper.GetString("udp.external_host")
	udpExternalPort := viper.GetInt("udp.external_port")

	aesKey, err := s.serverTransport.GetData("aes_key")
	if err != nil {
		return fmt.Errorf("Failed to get aes_key: %v", err)
	}
	fullNonce, err := s.serverTransport.GetData("full_nonce")
	if err != nil {
		return fmt.Errorf("Failed to get full_nonce: %v", err)
	}

	strAesKey, ok := aesKey.(string)
	if !ok {
		return fmt.Errorf("aes_key is not a string")
	}
	strFullNonce, ok := fullNonce.(string)
	if !ok {
		return fmt.Errorf("full_nonce is not a string")
	}

	udpConfig := &UdpConfig{
		Server: udpExternalHost,
		Port:   udpExternalPort,
		Key:    strAesKey,
		Nonce:  strFullNonce,
	}

	// Send response
	return s.serverTransport.SendHello("udp", &clientState.OutputAudioFormat, udpConfig)
}

func (s *ChatSession) HandleCommonHelloMessage(msg *ClientMessage) error {
	if msg.AudioParams == nil {
		return fmt.Errorf("hello message missing audio_params")
	}

	clientState := s.clientState

	s.helloMu.Lock()
	defer s.helloMu.Unlock()

	// Allow updating some runtime parameters when hello arrives (also effective for duplicate hello scenarios)
	clientState.InputAudioFormat = *msg.AudioParams

	isDuplicateHello := s.helloInited
	if isDuplicateHello {
		prevAgentID := clientState.AgentID
		// Only try to refresh device dimension configuration in duplicate hello scenario; fail with degraded handling, don't block hello
		if err := s.refreshDeviceConfigOnHello(); err != nil {
			log.Warnf("Device %s duplicate hello config refresh failed, degraded continue: %v", clientState.DeviceID, err)
		}
		s.resetOpenClawModeOnHello(prevAgentID, clientState.AgentID)
		if isMcp, ok := msg.Features["mcp"]; ok && isMcp && !s.mcpHelloInited {
			s.mcpHelloInited = true
			go initMcp(s.clientState, s.serverTransport)
		}
		log.Infof("Device %s received duplicate hello, skipping duplicate initialization", clientState.DeviceID)
		return nil
	}

	// First hello initialization
	s.resetOpenClawModeOnHello(clientState.AgentID)
	session, err := auth.A().CreateSession(msg.DeviceID)
	if err != nil {
		return fmt.Errorf("Failed to create session: %v", err)
	}

	// Update client state
	clientState.SessionID = session.ID

	if !s.vadLoopStarted {
		s.asrManager.ProcessVadAudio(clientState.Ctx, s.Close)
		s.vadLoopStarted = true
	}

	if isMcp, ok := msg.Features["mcp"]; ok && isMcp && !s.mcpHelloInited {
		s.mcpHelloInited = true
		go initMcp(s.clientState, s.serverTransport)
	}

	s.helloInited = true
	return nil
}

func (s *ChatSession) resetOpenClawModeOnHello(agentIDs ...string) {
	deviceID := strings.TrimSpace(s.clientState.DeviceID)
	if deviceID == "" {
		return
	}

	openclawManager := openclaw.GetManager()
	seen := make(map[string]struct{}, len(agentIDs))
	for _, agentID := range agentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		if openclawManager.ExitMode(agentID, deviceID) {
			log.Infof("Device %s reset OpenClaw mode after hello: agent=%s", deviceID, agentID)
		}
	}
}

func (s *ChatSession) refreshDeviceConfigOnHello() error {
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		return fmt.Errorf("Failed to get config provider: %w", err)
	}

	deviceConfig, err := configProvider.GetUserConfig(s.clientState.Ctx, s.clientState.DeviceID)
	if err != nil {
		return fmt.Errorf("Failed to get device configuration: %w", err)
	}
	deviceConfig.MemoryMode = NormalizeMemoryMode(deviceConfig.MemoryMode)

	prevAgentID := s.clientState.AgentID
	s.clientState.AgentID = deviceConfig.AgentId
	s.clientState.DeviceConfig = deviceConfig
	s.clientState.SystemPrompt = deviceConfig.SystemPrompt
	// Role may have switched, clear speaker temporary TTS config to avoid old config pollution
	s.clientState.SpeakerTTSConfig = nil
	applyOutputAudioFormatForTTS(s.clientState)

	log.Infof("Device %s hello config refresh successful, agent: %s -> %s", s.clientState.DeviceID, prevAgentID, deviceConfig.AgentId)
	return nil
}

func (s *ChatSession) HandleWebsocketHelloMessage(msg *ClientMessage) error {
	err := s.HandleCommonHelloMessage(msg)
	if err != nil {
		return err
	}

	return s.serverTransport.SendHello("websocket", &s.clientState.OutputAudioFormat, nil)
}

// handleListenMessage processes listen messages
func (s *ChatSession) HandleListenMessage(msg *ClientMessage) error {
	// Process based on state
	switch msg.State {
	case MessageStateStart:
		s.HandleListenStart(msg)
	case MessageStateStop:
		s.HandleListenStop()
	case MessageStateDetect:
		s.HandleListenDetect(msg)
	}

	// Log
	log.Infof("Device %s updated audio listen state: %s", msg.DeviceID, msg.State)
	return nil
}

func (s *ChatSession) beginListenStart() uint64 {
	startSeq := s.listenStartSeq.Add(1)
	s.clientState.SetListenPhase(ListenPhaseStarting)
	return startSeq
}

func (s *ChatSession) invalidateListenStart() {
	s.listenStartSeq.Add(1)
	s.clientState.SetListenPhase(ListenPhaseIdle)
}

func (s *ChatSession) isCurrentListenStart(startSeq uint64) bool {
	return startSeq == s.listenStartSeq.Load()
}

func (s *ChatSession) HandleListenDetect(msg *ClientMessage) error {
	// Check device activation status
	if msg.Text != "" {
		isActivated, err := s.CheckDeviceActivated()
		if err != nil {
			log.Errorf("Failed to check device activation status: %v", err)
			return err
		}
		if !isActivated {
			return nil
		}
	}

	// Stop current playback
	s.StopSpeaking(false)

	// If there's text, process it
	if msg.Text != "" {
		text := removePunctuation(msg.Text)

		enableGreeting := viper.GetBool("enable_greeting")
		// Wake word + greeting enabled -> go to welcome mode
		if isWakeupWord(text) && enableGreeting {
			if !s.clientState.IsWelcomeSpeaking {
				s.HandleWelcome()
			}
			return nil
		}

		if enableGreeting {
			// Default fallback to AddAsrResultToQueue
			if err := s.AddAsrResultToQueue(text, nil); err != nil {
				log.Errorf("Failed to start conversation: %v", err)
			}
		}
	}
	return nil
}

func (s *ChatSession) HandleNotActivated() {
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Failed to get config provider: %v", err)
		return
	}

	code, challenge, message, timeoutMs := configProvider.GetActivationInfo(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
	if code == "" {
		log.Errorf("Failed to get activation info: %v", err)
		return
	}

	log.Infof("Activation code: %s, Challenge code: %s, Message: %s, Timeout: %d", code, challenge, message, timeoutMs)

	s.ttsManager.EnqueueTtsStart(s.clientState.Ctx)
	defer s.ttsManager.EnqueueTtsStop(s.clientState.Ctx)

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	err = s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text: fmt.Sprintf("Please add device in backend, activation code: %s", code),
	}, false)
	s.ttsManager.RequestTurnEnd(ctx, err)

}

func (s *ChatSession) HandleWelcome() {
	greetingText := s.GetRandomGreeting()
	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)

	// Check if session has been stopped (by trying to acquire lock)
	if !s.stopSpeakingMu.TryLock() {
		log.Debugf("HandleWelcome StopSpeaking is executing, skipping welcome message")
		return
	}
	s.stopSpeakingMu.Unlock()

	// Check if sessionCtx is cancelled
	if sessionCtx.Err() != nil {
		log.Debugf("HandleWelcome sessionCtx cancelled, skipping welcome message")
		return
	}

	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	// Check if afterAsrCtx is cancelled
	if ctx.Err() != nil {
		log.Debugf("HandleWelcome afterAsrCtx cancelled, skipping welcome message")
		return
	}

	s.clientState.IsWelcomeSpeaking = true
	s.clientState.IsWelcomePlaying = true
	s.ttsManager.EnqueueTtsStart(s.clientState.Ctx)
	err := s.ttsManager.handleTts(ctx, s.ttsManager.currentAudioGeneration(), s.ttsManager.currentTtsMetricCycle(), llm_common.LLMResponseStruct{Text: greetingText}, nil, nil)
	s.ttsManager.RequestTurnEnd(ctx, err)
	s.ttsManager.EnqueueTtsStop(s.clientState.Ctx)
}

func (a *ChatSession) checkExitWords(text string) bool {
	exitWords := []string{"goodbye", "exitnow", "exit", "exittoconversation"}
	for _, word := range exitWords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func normalizeOpenClawKeywordText(text string) string {
	return removePunctuation(strings.ToLower(strings.TrimSpace(text)))
}

func containsOpenClawKeyword(text string, keywords []string) bool {
	normalizedText := normalizeOpenClawKeywordText(text)
	if normalizedText == "" {
		return false
	}
	for _, keyword := range keywords {
		normalizedKeyword := normalizeOpenClawKeywordText(keyword)
		if normalizedKeyword == "" {
			continue
		}
		if strings.Contains(normalizedText, normalizedKeyword) {
			return true
		}
	}
	return false
}

func (s *ChatSession) isOpenClawEnterKeyword(text string) bool {
	return containsOpenClawKeyword(text, s.clientState.DeviceConfig.OpenClaw.EnterKeywords)
}

func (s *ChatSession) isOpenClawExitKeyword(text string) bool {
	return containsOpenClawKeyword(text, s.clientState.DeviceConfig.OpenClaw.ExitKeywords)
}

func openClawLogSnippet(text string, maxRunes int) string {
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

func (s *ChatSession) GetRandomGreeting() string {
	greetingList := viper.GetStringSlice("greeting_list")
	if len(greetingList) == 0 {
		return "Hello, what fun things do you have?"
	}
	rand.Seed(time.Now().UnixNano())
	return greetingList[rand.Intn(len(greetingList))]
}

func (s *ChatSession) AddTextToTTSQueue(text string) error {
	return s.llmManager.AddTextToTTSQueue(text)
}

func (s *ChatSession) getOrCreateOpenClawStream(correlationID string) (chan llm_common.LLMResponseStruct, bool, error) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return nil, false, fmt.Errorf("missing correlation_id")
	}

	s.openClawStreamMu.Lock()
	if existing, ok := s.openClawStreams[correlationID]; ok {
		s.openClawStreamMu.Unlock()
		return existing, false, nil
	}
	streamChan := make(chan llm_common.LLMResponseStruct, 16)
	s.openClawStreams[correlationID] = streamChan
	s.openClawStreamMu.Unlock()

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	options := llmResponseChannelOptions{}
	hasWarmup := s.getOpenClawWarmupTask(correlationID) != nil
	if hasWarmup {
		options.disableTTSCommands = true
		options.onEndFunc = func(err error, args ...any) {
			// Warmup took over start, formal OpenClaw reply needs to send stop here;
			// Cannot send at warmup switch point, otherwise it would interrupt the main reply mid-way.
			if !s.clientState.IsRealTime() {
				s.ttsManager.EnqueueTtsStop(ctx)
			}
			s.ttsManager.RequestTurnEnd(ctx, err)
			s.finishOpenClawWarmup(correlationID, false)
		}
	}
	log.Infof("OpenClaw stream created: device=%s correlation_id=%s warmup_attached=%v", s.clientState.DeviceID, correlationID, hasWarmup)
	if err := s.llmManager.HandleLLMResponseChannelAsyncWithOptions(ctx, nil, streamChan, options); err != nil {
		if hasWarmup && !s.clientState.IsRealTime() {
			s.ttsManager.EnqueueTtsStop(ctx)
		}
		if hasWarmup {
			s.ttsManager.RequestTurnEnd(ctx, err)
		}
		s.openClawStreamMu.Lock()
		delete(s.openClawStreams, correlationID)
		s.openClawStreamMu.Unlock()
		close(streamChan)
		return nil, false, err
	}

	return streamChan, true, nil
}

func (s *ChatSession) closeOpenClawStream(correlationID string) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return
	}
	s.openClawStreamMu.Lock()
	delete(s.openClawStreams, correlationID)
	s.openClawStreamMu.Unlock()
}

func (s *ChatSession) clearOpenClawStreams() {
	s.openClawStreamMu.Lock()
	s.openClawStreams = make(map[string]chan llm_common.LLMResponseStruct)
	s.openClawStreamMu.Unlock()
}

func (s *ChatSession) InjectOpenClawResponse(event openclaw.ResponseDelivery) error {
	correlationID := strings.TrimSpace(event.CorrelationID)
	text := strings.TrimSpace(event.Text)

	// Non-streaming fallback: when no correlation_id, inject directly as single sentence.
	if correlationID == "" {
		if text == "" {
			return nil
		}
		return s.AddTextToTTSQueue(text)
	}

	// Intermediate empty chunks have no meaning, skip directly; end empty chunks are kept for cleanup.
	if text == "" && !event.IsEnd {
		return nil
	}

	streamChan, created, err := s.getOrCreateOpenClawStream(correlationID)
	if err != nil {
		return err
	}

	isStart := event.IsStart
	if created && !isStart {
		// If first arrived chunk doesn't have start marker, force start for first segment.
		isStart = true
	}
	if isStart {
		if task := s.getOpenClawWarmupTask(correlationID); task != nil {
			if text != "" {
				// Only stop warmup when first real playable content arrives, avoid being preempted by too short leading chunks.
				// Warmup's own start marker is only for warmup TTS, cannot swallow formal reply's IsStart,
				// otherwise formal reply would degrade to single-sentence TTS, subsequent snapshots would be treated as second sentence and announced again.
				s.cancelOpenClawWarmup(correlationID, false)
				s.beginOpenClawSpeech(task)
			} else {
				isStart = false
			}
		}
	} else if event.IsEnd {
		s.cancelOpenClawWarmup(correlationID, false)
	}

	resp := llm_common.LLMResponseStruct{
		Text:    text,
		IsStart: isStart,
		IsEnd:   event.IsEnd,
	}

	select {
	case <-s.ctx.Done():
		return fmt.Errorf("chat session closed")
	case streamChan <- resp:
	}

	if event.IsEnd {
		s.closeOpenClawStream(correlationID)
	}

	return nil
}

// InterruptAndClearTTSQueue triggers TTS interrupt and clears send queue (for realtime mode VAD interrupt and similar scenarios)
func (s *ChatSession) InterruptAndClearTTSQueue() {
	if s.mediaPlayer != nil {
		if err := s.mediaPlayer.Suspend(); err != nil && !errors.Is(err, context.Canceled) {
			log.Warnf("Failed to suspend media playback: %v", err)
		}
	}
	s.ttsManager.InterruptAndStop(s.clientState.Ctx, true, context.Canceled)
}

// handleAbortMessage processes abort messages
func (s *ChatSession) HandleAbortMessage(msg *ClientMessage) error {
	// Set interrupt status
	s.clientState.Abort = true

	if s.clientState.IsRealTime() {
		s.StopSpeakingAfterAsr(true)
	} else {
		s.StopSpeaking(true)
	}

	// Log
	log.Infof("Device %s aborted session", msg.DeviceID)
	return nil
}

// handleIoTMessage processes IoT messages
func (s *ChatSession) HandleIoTMessage(msg *ClientMessage) error {
	// Get client state
	//sessionID := clientState.SessionID

	// Verify device ID
	/*
		if _, err := s.authManager.GetSession(msg.DeviceID); err != nil {
			return fmt.Errorf("Session verification failed: %v", err)
		}*/

	// Send IoT response
	err := s.serverTransport.SendIot(msg)
	if err != nil {
		return fmt.Errorf("Failed to send response: %v", err)
	}

	// Log
	log.Infof("Device %s IoT command: %s", msg.DeviceID, msg.Text)
	return nil
}

func (s *ChatSession) HandleMcpMessage(msg *ClientMessage) error {
	mcpSession := mcp.GetDeviceMcpClient(s.clientState.DeviceID)
	if mcpSession != nil {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			return s.serverTransport.HandleMcpMessage(msg.PayLoad)
		}
	}
	return nil
}

// Release udp resources
func (s *ChatSession) HandleGoodByeMessage(msg *ClientMessage) error {
	s.serverTransport.transport.CloseAudioChannel()
	return nil
}

func (s *ChatSession) CheckDeviceActivated() (bool, error) {
	if viper.GetBool("auth.enable") {
		if !s.clientState.IsActivated {
			const falseCheckThrottle = time.Second
			s.activationCheckMu.Lock()
			lastFalseAt := s.lastActivationFalseAt
			s.activationCheckMu.Unlock()
			if !lastFalseAt.IsZero() && time.Since(lastFalseAt) < falseCheckThrottle {
				log.Debugf("Device %s activation status still unactivated, skipping duplicate real-time verification", s.clientState.DeviceID)
				return false, nil
			}

			configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
			if err != nil {
				log.Errorf("Failed to get config provider: %v", err)
				return false, err
			}
			// Call API to reconfirm activation status
			isActivated, err := configProvider.IsDeviceActivated(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
			if err != nil {
				log.Errorf("Failed to get activation status: %v", err)
				return false, err
			}
			if isActivated {
				s.clientState.IsActivated = true
				s.activationCheckMu.Lock()
				s.lastActivationFalseAt = time.Time{}
				s.activationCheckMu.Unlock()
			} else {
				s.activationCheckMu.Lock()
				s.lastActivationFalseAt = time.Now()
				s.activationCheckMu.Unlock()
				s.HandleNotActivated()
				return false, nil
			}
		}
	}
	return true, nil
}

func (s *ChatSession) HandleListenStart(msg *ClientMessage) error {
	// Check activation status first
	isActivated, err := s.CheckDeviceActivated()
	if err != nil {
		log.Errorf("Failed to check device activation status: %v", err)
		return err
	}
	if !isActivated {
		return nil
	}

	// Realtime mode first startup: skip welcome judgment and Destroy, directly enter listen
	if msg.Mode == "realtime" {

		if !s.clientState.IsWelcomePlaying {
			s.StopSpeaking(false)
		}

		s.clientState.ListenMode = msg.Mode
		log.Infof("Device %s listen mode: %s", msg.DeviceID, msg.Mode)

		startSeq := s.beginListenStart()
		go func() {
			if err := s.OnListenStart(startSeq); err != nil {
				log.Errorf("Device %s listen start failed: %v", msg.DeviceID, err)
			}
		}()
		return nil
	}

	if s.clientState.IsWelcomePlaying {
		log.Infof("Device %s welcome message playing, ignoring listen start", msg.DeviceID)
		return nil
	}

	if s.clientState.GetListenPhase() == ListenPhaseStarting {
		log.Infof("Device %s listen start already in progress, ignoring duplicate listen start", msg.DeviceID)
		return nil
	}

	// Process listen mode
	s.clientState.ListenMode = msg.Mode
	log.Infof("Device %s listen mode: %s", msg.DeviceID, msg.Mode)
	//if s.clientState.ListenMode == "manual" {
	s.StopSpeaking(false)
	//}

	startSeq := s.beginListenStart()
	go func() {
		if err := s.OnListenStart(startSeq); err != nil {
			log.Errorf("Device %s listen start failed: %v", msg.DeviceID, err)
		}
	}()

	return nil
}

func (s *ChatSession) HandleListenStop() error {
	/*if s.clientState.ListenMode == "auto" {
		s.clientState.CancelSessionCtx()
	}*/

	// Call
	s.clientState.OnManualStop()

	return nil
}

func (s *ChatSession) OnListenStart(startSeq uint64) error {
	log.Debugf("OnListenStart start")
	defer log.Debugf("OnListenStart end")

	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale before init, skip")
		return nil
	}

	select {
	case <-s.clientState.Ctx.Done():
		log.Debugf("OnListenStart Ctx done, return")
		if s.isCurrentListenStart(startSeq) {
			s.clientState.SetListenPhase(ListenPhaseIdle)
		}
		return nil
	default:
	}

	// Realtime mode: skip Destroy, keep ASR running continuously, but clear AudioBuffer
	if s.clientState.IsRealTime() {
		s.clientState.AsrAudioBuffer.ClearAsrAudioData()
	} else {
		s.clientState.Destroy()
		if !s.isCurrentListenStart(startSeq) {
			log.Debugf("OnListenStart stale after destroy, skip")
			return nil
		}
	}

	s.clientState.SetListenPhase(ListenPhaseStarting)

	s.clientState.SetStatus(ClientStatusListening)

	ctx := s.clientState.SessionCtx.Get(s.clientState.Ctx)

	// Initialize asr related
	if s.clientState.ListenMode == "manual" {
		s.clientState.VoiceStatus.SetClientHaveVoice(true)
	}

	// Start asr streaming recognition, reuse restartAsrRecognition function
	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale before ASR restart, skip")
		return nil
	}
	err := s.asrManager.RestartAsrRecognition(ctx)
	if err != nil {
		log.Errorf("ASR streaming recognition failed: %v", err)
		if s.isCurrentListenStart(startSeq) {
			s.clientState.SetListenPhase(ListenPhaseIdle)
		}
		s.Close()
		return err
	}

	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale after ASR restart, cancel current start")
		s.clientState.Asr.CancelWithReason("ChatSession.OnListenStart: stale listen start after ASR restart")
		return nil
	}

	s.clientState.SetListenPhase(ListenPhaseListening)

	// Define message save callback
	onMessageSave := func(userMsg *schema.Message, messageID string, audioData []float32) {
		// ASR text and audio obtained simultaneously, save in one go (no two-phase needed)
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: s.clientState,
			Msg:         *userMsg,
			MessageID:   messageID,
			AudioData:   [][]byte{util.Float32SliceToBytes(audioData)}, // Convert to byte array
			AudioSize:   len(audioData) * 4,                            // float32 = 4 bytes
			SampleRate:  s.clientState.InputAudioFormat.SampleRate,
			Channels:    s.clientState.InputAudioFormat.Channels,
			IsUpdate:    false, // One-time save (text + audio)
			Timestamp:   time.Now(),
		})
	}

	// Define error handling callback
	onError := func(err error) {
		log.Errorf("ASR recognition loop error: %v", err)
		s.Close()
	}

	// Start ASR recognition result processing loop (resource management inside ASRManager)
	s.asrManager.StartAsrRecognitionLoop(ctx, onMessageSave, onError)

	return nil
}

// startChat starts conversation
func (s *ChatSession) AddAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	log.Debugf("AddAsrResultToQueue text: %s", text)
	if speakerResult != nil && speakerResult.Identified {
		log.Debugf("AddAsrResultToQueue speaker: %s (confidence: %.2f)", speakerResult.SpeakerName, speakerResult.Confidence)
	}

	// Check if session has been stopped (by trying to acquire lock)
	// If StopSpeaking is executing, this will wait; if already completed, tryLock returns immediately
	if !s.stopSpeakingMu.TryLock() {
		log.Debugf("AddAsrResultToQueue StopSpeaking is executing, discarding message")
		return nil
	}
	s.stopSpeakingMu.Unlock()

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	// Check if sessionCtx is cancelled
	if sessionCtx.Err() != nil {
		log.Debugf("AddAsrResultToQueue sessionCtx cancelled, discarding message")
		return nil
	}

	item := AsrResponseChannelItem{
		ctx:           s.clientState.AfterAsrSessionCtx.Get(sessionCtx),
		text:          text,
		speakerResult: speakerResult,
	}
	err := s.chatTextQueue.Push(item)
	if err != nil {
		log.Warnf("chatTextQueue full or closed, discarding message")
	}
	return nil
}

func (s *ChatSession) processChatText(ctx context.Context) {
	log.Debugf("processChatText start")
	defer log.Debugf("processChatText end")

	for {
		item, err := s.chatTextQueue.Pop(ctx, 0)
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			continue
		}

		err = s.actionDoChat(item.ctx, item.text, item.speakerResult)
		if err != nil {
			log.Errorf("Failed to process conversation: %v", err)
			continue
		}
	}
}

func (s *ChatSession) ClearChatTextQueue() {
	s.chatTextQueue.Clear()
}

// DoExitChat executes exit chat logic (send goodbye message and close session)
func (s *ChatSession) DoExitChat() {
	// Friendly goodbye message
	goodbyeText := "Alright, goodbye! Looking forward to chatting with you next time ~"

	// Save an assistant role message
	goodbyeMsg := schema.AssistantMessage(goodbyeText, nil)
	if err := s.llmManager.AddLlmMessage(s.clientState.Ctx, goodbyeMsg); err != nil {
		log.Errorf("Failed to save goodbye message: %v", err)
	}

	// Get context
	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)

	// Send TTS goodbye message
	s.ttsManager.EnqueueTtsStart(ctx)

	err := s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text:    goodbyeText,
		IsStart: true,
		IsEnd:   true,
	}, true) // Synchronous processing, wait for TTS completion

	if err != nil {
		log.Errorf("Failed to send goodbye message: %v", err)
	}

	s.ttsManager.RequestTurnEnd(ctx, err)
	s.ttsManager.EnqueueTtsStop(ctx)
	// Close session
	s.Close()
}

func (s *ChatSession) Close() {
	s.closeOnce.Do(func() {
		// Cleanup ASR resources (resource management inside ASRManager)
		if s.asrManager != nil {
			s.asrManager.Cleanup()
		}
		deviceID := ""
		if s.clientState != nil {
			deviceID = s.clientState.DeviceID
		}
		log.Debugf("ChatSession.Close() starting session resource cleanup, device %s", deviceID)

		if s.mediaPlayer != nil {
			s.mediaPlayer.DetachSession(true)
		}

		// Cancel session-level context
		if s.cancel != nil {
			s.cancel()
		}
		s.finishOpenClawWarmup("", false)

		// Clear chat text queue
		s.ClearChatTextQueue()
		s.clearOpenClawStreams()

		// Stop speaking and cleanup audio related resources. Close path already DetachSession(true) earlier,
		// don't Suspend media again here, otherwise it would clear resumeOnAttach.
		s.stopSpeakingWithLock(true, true, false)

		// Close server transport
		if s.serverTransport != nil {
			s.serverTransport.Close()
		}

		if s.speakerManager != nil {
			s.speakerManager.Close()
		}

		if s.hookHub != nil {
			s.hookHub.Close()
		}

		if s.clientState != nil {
			eventbus.Get().Publish(eventbus.TopicSessionEnd, s.clientState)
		}

		log.Debugf("ChatSession.Close() session resource cleanup completed, device %s", deviceID)
	})
}

func (s *ChatSession) actionDoChat(ctx context.Context, text string, speakerResult *speaker.IdentifyResult) error {
	select {
	case <-ctx.Done():
		log.Debugf("actionDoChat ctx done, return")
		return nil
	default:
	}

	agentID := strings.TrimSpace(s.clientState.AgentID)
	deviceID := strings.TrimSpace(s.clientState.DeviceID)
	openclawSessionID := strings.TrimSpace(s.clientState.SessionID)
	trimmedText := strings.TrimSpace(text)

	handledByRealtimeGate, gateErr := s.tryHandleRealtimeMcpAudioASR(ctx, trimmedText)
	if handledByRealtimeGate {
		return gateErr
	}

	openclawManager := openclaw.GetManager()
	if s.clientState.DeviceConfig.OpenClaw.Allowed {
		isOpenClawMode := openclawManager.IsModeEnabled(agentID, deviceID)
		isEnterKeyword := s.isOpenClawEnterKeyword(text)
		isExitKeyword := false
		if isOpenClawMode {
			isExitKeyword = s.isOpenClawExitKeyword(text)
		}
		log.Debugf(
			"OpenClaw routing decision: agent=%s device=%s session=%s allowed=%v mode=%v enter_keyword=%v exit_keyword=%v text_len=%d text_trim_len=%d text_snippet=%q",
			agentID,
			deviceID,
			openclawSessionID,
			s.clientState.DeviceConfig.OpenClaw.Allowed,
			isOpenClawMode,
			isEnterKeyword,
			isExitKeyword,
			len(text),
			len(trimmedText),
			openClawLogSnippet(trimmedText, 64),
		)
		if isOpenClawMode {
			if isExitKeyword {
				s.finishOpenClawWarmup("", true)
				exited := openclawManager.ExitMode(agentID, deviceID)
				_ = s.AddTextToTTSQueue("alreadyexitOpenClawpattern")
				log.Infof("Device %s exited OpenClaw mode: agent=%s exited=%v", deviceID, agentID, exited)
				return nil
			}

			log.Infof(
				"OpenClaw sending STT: agent=%s device=%s session=%s text_len=%d text_snippet=%q",
				agentID,
				deviceID,
				openclawSessionID,
				len(trimmedText),
				openClawLogSnippet(trimmedText, 64),
			)
			s.finishOpenClawWarmup("", true)
			messageID, err := openclawManager.SendMessage(
				agentID,
				deviceID,
				text,
				openclawSessionID,
			)
			if err != nil {
				log.Warnf(
					"Device %s OpenClaw message send failed, fallback to normal mode: agent=%s session=%s text_snippet=%q err=%v",
					deviceID,
					agentID,
					openclawSessionID,
					openClawLogSnippet(trimmedText, 64),
					err,
				)
				openclawManager.ExitMode(agentID, deviceID)
				_ = s.AddTextToTTSQueue("OpenClaw currently unavailable, exited OpenClaw mode")
			} else {
				s.startOpenClawWarmup(messageID, text)
				log.Infof("OpenClaw STT send successful: agent=%s device=%s session=%s message_id=%s", agentID, deviceID, openclawSessionID, messageID)
			}
			return nil
		}

		if isEnterKeyword {
			if !openclawManager.EnterMode(agentID, deviceID) {
				_ = s.AddTextToTTSQueue("OpenClaw currently unavailable, please try again later")
				log.Warnf("Device %s failed to enter OpenClaw mode: agent=%s agent session not ready", deviceID, agentID)
				return nil
			}
			_ = s.AddTextToTTSQueue("Entered OpenClaw mode, please continue speaking")
			log.Infof("Device %s entered OpenClaw mode: agent=%s trigger=%q", deviceID, agentID, openClawLogSnippet(trimmedText, 32))
			return nil
		}
		log.Debugf(
			"OpenClaw didn't take over current STT: agent=%s device=%s mode=%v enter_keyword=%v text_snippet=%q",
			agentID,
			deviceID,
			isOpenClawMode,
			isEnterKeyword,
			openClawLogSnippet(trimmedText, 64),
		)
	} else {
		s.finishOpenClawWarmup("", false)
		if openclawManager.ExitMode(agentID, deviceID) {
			log.Debugf("OpenClaw configuration not enabled, forced exit mode: agent=%s device=%s", agentID, deviceID)
		}
	}

	if s.checkExitWords(text) {
		// Publish exit chat event
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "User actively exited",
			TriggerType: "exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		return nil
	}

	clientState := s.clientState

	sessionID := clientState.SessionID

	// Dynamic TTS switch after speaker identification (restore default TTS when not identified)
	if err := s.switchTTSForSpeaker(speakerResult); err != nil {
		log.Warnf("Failed to switch TTS: %v", err)
		// Don't interrupt flow, continue with current TTS
	}

	// Create Eino native message directly
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: text,
	}

	// Get global MCP tool list
	mcpTools, err := mcp.GetToolsByDeviceId(clientState.DeviceID, clientState.AgentID, clientState.DeviceConfig.MCPServiceNames)
	if err != nil {
		log.Errorf("Failed to get tools for device %s: %v", clientState.DeviceID, err)
		mcpTools = make(map[string]tool.InvokableTool)
	}
	if !hasAvailableKnowledgeBase(clientState.DeviceConfig.KnowledgeBases) {
		if _, ok := mcpTools["search_knowledge"]; ok {
			delete(mcpTools, "search_knowledge")
			log.Infof("Device %s has no associated available knowledge base, removed tool search_knowledge", clientState.DeviceID)
		}
	}

	// Convert MCP tools to interface format for passing to conversion function
	mcpToolsInterface := make(map[string]interface{})
	for name, tool := range mcpTools {
		mcpToolsInterface[name] = tool
	}

	// Convert MCP tools to Eino ToolInfo format
	einoTools, err := llm.ConvertMCPToolsToEinoTools(ctx, mcpToolsInterface)
	if err != nil {
		log.Errorf("Failed to convert MCP tools: %v", err)
		einoTools = nil
	}

	toolNameList := make([]string, 0)
	for _, tool := range einoTools {
		toolNameList = append(toolNameList, tool.Name)
	}

	// SendLLM request with tools
	log.Infof("SendingLLM request with %d MCP tools, tools: %+v", len(einoTools), toolNameList)

	err = s.llmManager.DoLLmRequest(ctx, userMessage, einoTools, true, speakerResult)
	if err != nil {
		log.Errorf("Failed to sendLLM request with tools, sessionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("Failed to sendLLM request with tools: %v", err)
	}
	return nil
}

func hasAvailableKnowledgeBase(knowledgeBases []types.KnowledgeBaseRef) bool {
	for _, kb := range knowledgeBases {
		if strings.EqualFold(strings.TrimSpace(kb.Status), "inactive") {
			continue
		}
		if strings.TrimSpace(kb.ExternalKBID) == "" {
			continue
		}
		return true
	}
	return false
}

func (s *ChatSession) MarkTurnSpeakerInterrupted() {
	if s == nil {
		return
	}
	s.turnSpeakerInterrupted.Store(true)
}

func (s *ChatSession) ConsumeTurnSpeakerInterrupted() bool {
	if s == nil {
		return false
	}
	return s.turnSpeakerInterrupted.Swap(false)
}

func (s *ChatSession) ResetTurnSpeakerInterrupted() {
	if s == nil {
		return
	}
	s.turnSpeakerInterrupted.Store(false)
}

func (s *ChatSession) ShouldAllowSpeakerChat(speakerResult *speaker.IdentifyResult, speakerInterrupted bool) (bool, string) {
	if s == nil || s.clientState == nil {
		return true, ""
	}

	matchedConfiguredSpeaker := s.clientState.HasMatchedConfiguredSpeaker(speakerResult)
	if speakerInterrupted && !matchedConfiguredSpeaker {
		return false, "speaker_interrupt_without_identify"
	}

	if s.clientState.RequireMatchedSpeakerForChat() && !matchedConfiguredSpeaker {
		return false, "speaker_chat_mode_identified_only_not_matched"
	}

	return true, ""
}

// switchTTSForSpeaker switches TTS for identified speaker
func (s *ChatSession) switchTTSForSpeaker(speakerResult *speaker.IdentifyResult) error {
	s.clientState.SpeakerTTSConfig = nil

	// 1. Check if speakerResult is nil
	if speakerResult == nil {
		log.Debug("speakerResult is nil, clearing speaker TTS config")
		return nil
	}

	// 2. Find speaker group configuration
	speakerGroupInfo, found := s.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]
	if !found {
		// Config not found, clear speaker TTS config
		log.Debugf("Speaker group %s config not found, clearing speaker TTS config", speakerResult.SpeakerName)
		return nil
	}

	// 3. Check if custom voice is configured
	if speakerGroupInfo.TTSConfigID == nil || *speakerGroupInfo.TTSConfigID == "" {
		// No custom voice configured, clear speaker TTS config
		log.Debugf("Speaker group %s has no custom TTS configured, clearing speaker TTS config", speakerResult.SpeakerName)
		return nil
	}

	// 4. Find corresponding TTS config from system config (viper)
	var targetTTSConfig *types.TtsConfigItem
	ttsConfigsRaw := viper.Get("tts")
	if ttsConfigsRaw == nil {
		return fmt.Errorf("tts not found in system config")
	}

	// Parse tts config (now a map, key is config_id)
	if ttsConfigsMap, ok := ttsConfigsRaw.(map[string]interface{}); ok {
		// Find matching config_id
		if configItem, exists := ttsConfigsMap[*speakerGroupInfo.TTSConfigID]; exists {
			if configMap, ok := configItem.(map[string]interface{}); ok {
				// Parse config item
				ttsItem := &types.TtsConfigItem{
					ConfigID: *speakerGroupInfo.TTSConfigID,
				}
				if name, ok := configMap["name"].(string); ok {
					ttsItem.Name = name
				}
				if provider, ok := configMap["provider"].(string); ok {
					ttsItem.Provider = provider
				}
				if isDefault, ok := configMap["is_default"].(bool); ok {
					ttsItem.IsDefault = isDefault
				}
				// Other config fields directly as config
				ttsItem.Config = make(map[string]interface{})
				for k, v := range configMap {
					if k != "name" && k != "provider" && k != "is_default" && k != "config_id" {
						ttsItem.Config[k] = v
					}
				}
				targetTTSConfig = ttsItem
			}
		}
	}

	if targetTTSConfig == nil {
		return fmt.Errorf("TTS config %s not found", *speakerGroupInfo.TTSConfigID)
	}

	// 5. Copy TTS config to avoid modifying original config
	ttsConfig := make(map[string]interface{})
	for k, v := range targetTTSConfig.Config {
		ttsConfig[k] = v
	}

	// 6. If voice value is configured, override in TTS config
	if speakerGroupInfo.Voice != nil && *speakerGroupInfo.Voice != "" {
		// Set corresponding voice field based on provider
		if targetTTSConfig.Provider == "cosyvoice" {
			ttsConfig["spk_id"] = *speakerGroupInfo.Voice
		} else {
			ttsConfig["voice"] = *speakerGroupInfo.Voice
		}
		log.Debugf("Setting voice for speaker %s: %s", speakerResult.SpeakerName, *speakerGroupInfo.Voice)
	}
	if targetTTSConfig.Provider == "aliyun_qwen" &&
		speakerGroupInfo.VoiceModelOverride != nil &&
		strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride) != "" {
		overrideModel := strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride)
		ttsConfig["model"] = overrideModel
		log.Debugf("Overriding Qwen model for speaker %s: %s", speakerResult.SpeakerName, overrideModel)
	}

	// 7. Save complete TTS config (deep copy)
	s.clientState.SpeakerTTSConfig = make(map[string]interface{})
	for k, v := range ttsConfig {
		s.clientState.SpeakerTTSConfig[k] = v
	}
	// Ensure provider is in config
	s.clientState.SpeakerTTSConfig["provider"] = targetTTSConfig.Provider

	log.Infof("✅ Successfully switched TTS config for speaker %s - Provider: %s, ConfigID: %s, Voice: %v",
		speakerResult.SpeakerName,
		targetTTSConfig.Provider,
		targetTTSConfig.ConfigID,
		speakerGroupInfo.Voice)

	return nil
}

func (s *ChatSession) hookContext(ctx context.Context) chathooks.Context {
	sessionID := ""
	deviceID := ""
	if s != nil && s.clientState != nil {
		sessionID = s.clientState.SessionID
		deviceID = s.clientState.DeviceID
	}

	return chathooks.Context{
		Ctx:       ctx,
		SessionID: sessionID,
		DeviceID:  deviceID,
	}
}

func (s *ChatSession) emitMetricStage(ctx context.Context, stage chathooks.MetricStage, ts int64, err error) {
	if s == nil {
		return
	}

	hookErr := s.hookHub.EmitMetric(s.hookContext(ctx), chathooks.MetricData{Stage: stage, Ts: ts, Err: err})
	if hookErr != nil {
		log.Warnf("METRIC hook execution failed: stage=%s err=%v", stage, hookErr)
	}
}

func (s *ChatSession) TraceTurnStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTurnStart, ts, nil)
}

func (s *ChatSession) TraceTurnEnd(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricTurnEnd, ts, err)
}

func (s *ChatSession) TraceAsrFirstText(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricAsrFirstText, ts, nil)
}

func (s *ChatSession) TraceAsrFinalText(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricAsrFinalText, ts, nil)
}

func (s *ChatSession) TraceLlmStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricLlmStart, ts, nil)
}

func (s *ChatSession) TraceLlmFirstToken(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricLlmFirstToken, ts, nil)
}

func (s *ChatSession) TraceLlmEnd(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricLlmEnd, ts, err)
}

func (s *ChatSession) TraceTtsStart(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTtsStart, ts, nil)
}

func (s *ChatSession) TraceTtsFirstFrame(ctx context.Context, ts int64) {
	s.emitMetricStage(ctx, chathooks.MetricTtsFirstFrame, ts, nil)
}

func (s *ChatSession) TraceTtsStop(ctx context.Context, ts int64, err error) {
	s.emitMetricStage(ctx, chathooks.MetricTtsStop, ts, err)
}
