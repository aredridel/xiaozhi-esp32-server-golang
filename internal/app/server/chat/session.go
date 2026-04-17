package chat

import (
	"context"
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

const (
	chatSessionCloseReasonManagerShutdown = "manager_shutdown"
	chatSessionCloseReasonExplicitExit    = "explicit_exit"
	chatSessionCloseReasonFatalError      = "fatal_error"
)

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

	// speaker recognition result cache (lock-protected)
	speakerResultMu        sync.RWMutex
	pendingSpeakerResult   *speaker.IdentifyResult
	speakerResultReady     chan struct{} // used only for readiness notification, no data
	turnSpeakerInterrupted atomic.Bool

	vadLoopStarted bool
	listenStartSeq atomic.Uint64

	// when unactivated device triggers frequently, reuse recent “inactive” judgment within short time, avoid frequent API calls.
	activationCheckMu     sync.Mutex
	lastActivationFalseAt time.Time

	// Close protection, prevent multiple closes
	closeOnce sync.Once
	closing   atomic.Bool

	// stopSpeaking protection, prevent concurrent conflict with AddAsrResultToQueue/HandleWelcome
	stopSpeakingMu sync.Mutex

	openClawStreamMu sync.Mutex
	openClawStreams  map[string]chan llm_common.LLMResponseStruct

	openClawWarmupMu sync.Mutex
	openClawWarmup   *openClawWarmupTask

	hookHub      *chathooks.Hub
	closeHandler func(session *ChatSession, reason string)
}

type ChatSessionOption func(*ChatSession)

func WithChatSessionCloseHandler(handler func(session *ChatSession, reason string)) ChatSessionOption {
	return func(s *ChatSession) {
		s.closeHandler = handler
	}
}

func NewChatSession(clientState *ClientState, serverTransport *ServerTransport, hookHub *chathooks.Hub, transformRegistry *streamtransform.Registry, opts ...ChatSessionOption) *ChatSession {
	s := &ChatSession{
		clientState:        clientState,
		serverTransport:    serverTransport,
		chatTextQueue:      util.NewQueue[AsrResponseChannelItem](10),
		speakerResultReady: make(chan struct{}, 1), // buffer of 1, avoid blocking
		openClawStreams:    make(map[string]chan llm_common.LLMResponseStruct),
		hookHub:            hookHub,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.asrManager = NewASRManager(clientState, serverTransport)
	s.asrManager.session = s
	s.ttsManager = NewTTSManager(clientState, serverTransport, s)
	s.mediaPlayer = NewSessionMediaPlayer(s)
	s.llmManager = NewLLMManager(clientState, serverTransport, s.ttsManager, s, transformRegistry)

	if clientState.IsSpeakerEnabled() {
		baseURL := viper.GetString("voice_identify.base_url")
		if baseURL != "" {
			speakerConfig := map[string]interface{}{
				"base_url": baseURL,
			}
			if viper.IsSet("voice_identify.threshold") {
				threshold := viper.GetFloat64("voice_identify.threshold")
				speakerConfig["threshold"] = threshold
			}

			provider, err := speaker.GetSpeakerProvider(speakerConfig)
			if err != nil {
				log.Warnf("create speaker recognition provider failed: %v", err)
			} else {
				clientState.SpeakerProvider = provider
				s.speakerManager = NewSpeakerManager(provider)
				log.Debugf("device %s speaker recognition enabled", clientState.DeviceID)

				// set async callback for getting speaker result
				clientState.OnVoiceSilenceSpeakerCallback = func(ctx context.Context) {
					log.Debugf("[SpeakerRecognition] OnVoiceSilenceSpeakerCallback called, deviceID: %s", clientState.DeviceID)

					go func() {
						log.Debugf("[SpeakerRecognition] starting async speaker recognition result retrieval, deviceID: %s", clientState.DeviceID)

						if !s.speakerManager.IsActive() {
							return
						}
						s.speakerResultMu.Lock()
						oldResult := s.pendingSpeakerResult
						s.pendingSpeakerResult = nil
						s.speakerResultMu.Unlock()
						if oldResult != nil {
							log.Debugf("[SpeakerRecognition] cleared previous result: identified=%v, speaker_id=%s", oldResult.Identified, oldResult.SpeakerID)
						}

						select {
						case <-s.speakerResultReady:
							log.Debugf("[SpeakerRecognition] cleared ready notification channel")
						default:
							log.Debugf("[SpeakerRecognition] ready notification channel already empty")
						}

						result, err := s.speakerManager.FinishAndIdentify(ctx)
						if err != nil {
							log.Warnf("[SpeakerRecognition] get speaker recognition result failed: %v, deviceID: %s", err, clientState.DeviceID)
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = nil
							s.speakerResultMu.Unlock()
							log.Debugf("[SpeakerRecognition] stored nil result (recognition failed)")
						} else if result != nil && result.Identified {
							log.Infof("[SpeakerRecognition] identified speaker: %s (confidence: %.4f, threshold: %.4f), deviceID: %s",
								result.SpeakerName, result.Confidence, result.Threshold, clientState.DeviceID)
							log.Debugf("[SpeakerRecognition] result details: speaker_id=%s, speaker_name=%s, confidence=%.4f, threshold=%.4f",
								result.SpeakerID, result.SpeakerName, result.Confidence, result.Threshold)
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[SpeakerRecognition] stored result (identified)")
						} else {
							if result != nil {
								log.Debugf("[SpeakerRecognition] speaker not identified: identified=%v, confidence=%.4f, threshold=%.4f, deviceID: %s",
									result.Identified, result.Confidence, result.Threshold, clientState.DeviceID)
							} else {
								log.Debugf("[SpeakerRecognition] result is nil, deviceID: %s", clientState.DeviceID)
							}
							s.speakerResultMu.Lock()
							s.pendingSpeakerResult = result
							s.speakerResultMu.Unlock()
							log.Debugf("[SpeakerRecognition] stored result (not identified)")
						}

						select {
						case s.speakerResultReady <- struct{}{}:
							log.Debugf("[SpeakerRecognition] sent result ready notification, deviceID: %s", clientState.DeviceID)
						default:
							log.Warnf("[SpeakerRecognition] result ready notification channel full, cannot send notification, deviceID: %s", clientState.DeviceID)
						}
					}()
				}
			}
		}
	}

	// set callback for ASR first character return
	clientState.OnAsrFirstTextCallback = func(text string, isFinal bool) {
		clientState.Asr.MarkTextReceived()
		log.Debugf("ASR first text returned: device=%s, text=%s, isFinal=%v", clientState.DeviceID, text, isFinal)
		clientState.MarkAsrFirstText()
		s.TraceAsrFirstText(clientState.Ctx, time.Now().UnixMilli())
		if clientState.IsRealTime() && viper.GetInt("chat.realtime_mode") == 4 {
			if s.isRealtimeMcpAudioGateActive() {
				log.Debugf("device %s realtime media playback gate active, skipping ASR first text interrupt: text=%s", clientState.DeviceID, text)
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

	if s.clientState.InputAudioFormat.SampleRate <= 0 || s.clientState.InputAudioFormat.Channels <= 0 {
		return fmt.Errorf("input audio format not initialized, please complete hello handshake first")
	}

	err := s.InitAsrLlmTts()
	if err != nil {
		log.Errorf("initialize ASR/LLM/TTS failed: %v", err)
		return err
	}

	// async load history messages, non-blocking session start
	go func() {
		err := s.initHistoryMessages()
		if err != nil {
			log.Errorf("initialize chat history failed: %v", err)
		}
	}()

	if !s.vadLoopStarted {
		s.asrManager.ProcessVadAudio(s.ctx, func() {
			s.CloseWithReason(chatSessionCloseReasonFatalError)
		})
		s.vadLoopStarted = true
	}

	go s.processChatText(s.ctx)  // process dialogue messages after ASR
	go s.llmManager.Start(s.ctx) // process return messages after LLM
	go s.ttsManager.Start(s.ctx) // process TTS message queue
	if s.mediaPlayer != nil {
		s.mediaPlayer.AttachSession()
	}

	return nil
}

// initialize history dialogue records into memory
func (s *ChatSession) initHistoryMessages() error {
	var historyMessages []*schema.Message
	var err error

	if s.clientState.GetMemoryMode() == MemoryModeNone {
		log.Debugf("device %s memory mode=none, skipping history message loading", s.clientState.DeviceID)
		return nil
	}

	// select data source based on config (no priority, direct selection)
	useRedis := s.shouldUseRedis()
	useManager := s.shouldUseManager()

	// validate required fields: DeviceID cannot be empty
	if s.clientState.DeviceID == "" {
		log.Debugf("DeviceID is empty, skipping history message loading (may be called before hello message)")
		return nil
	}

	// select data source based on config (no priority, direct selection)
	if useRedis {
		// load from Redis
		historyMessages, err = llm_memory.Get().GetMessages(
			s.ctx,
			s.clientState.DeviceID,
			s.clientState.AgentID,
			20)
		if err != nil {
			log.Warnf("load history messages from Redis failed: %v", err)
			return err
		}
		log.Infof("loaded %d history messages from Redis", len(historyMessages))
	} else if useManager {
		// load from Manager
		historyMessages, err = s.loadFromManager()
		if err != nil {
			log.Warnf("load history messages from Manager failed: %v", err)
			return err
		}
		log.Infof("loaded %d history messages from Manager", len(historyMessages))
	} else {
		// neither data source configured, skip loading history
		log.Debugf("neither Redis nor Manager configured, skipping history message loading")
		return nil
	}

	if len(historyMessages) > 0 {
		s.clientState.InitMessages(historyMessages)
		log.Infof("successfully loaded %d history messages", len(historyMessages))
	} else {
		log.Debugf("no history messages loaded (may have no history)")
	}

	return nil
}

// shouldUseRedis determines whether to use Redis as data source
func (s *ChatSession) shouldUseRedis() bool {
	// determine by config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "redis"
}

// shouldUseManager determines whether to use Manager as data source
func (s *ChatSession) shouldUseManager() bool {
	// determine by config_provider.type
	providerType := viper.GetString("config_provider.type")
	return providerType == "manager"
}

// loadFromManager loads history messages from Manager database
func (s *ChatSession) loadFromManager() ([]*schema.Message, error) {
	// create HistoryClient
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

	// convert to schema.Message format
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
			log.Warnf("unknown message role: %s", item.Role)
			continue
		}

		messages = append(messages, msg)
	}

	for _, msg := range messages {
		log.Debugf("history message: %+v", msg)
	}

	return messages, nil
}

// called after receiving type: listen, state: start via MQTT
func (c *ChatSession) InitAsrLlmTts() error {
	// initialize ASR structure
	c.clientState.InitAsr()

	// initialize memory (memory is not in resource pool)
	memoryMode := c.clientState.GetMemoryMode()
	memoryConfig := c.clientState.DeviceConfig.Memory
	memoryType := memory.MemoryType(memoryConfig.Provider)
	if memoryMode != MemoryModeLong {
		memoryType = memory.MemoryTypeNone
	}

	memoryProvider, err := memory.GetProvider(memoryType, memoryConfig.Config)
	if err != nil {
		return fmt.Errorf("create Memory provider failed: %v", err)
	}
	c.clientState.MemoryProvider = memoryProvider

	if memoryMode == MemoryModeLong {
		// initialize memory context (long memory mode only)
		context, err := memoryProvider.GetContext(c.ctx, c.clientState.GetDeviceIDOrAgentID(), 500)
		if err != nil {
			log.Warnf("initialize memory context failed: %v", err)
		}
		c.clientState.MemoryContext = context
	} else {
		c.clientState.MemoryContext = ""
	}

	return nil
}

// HandleAudioMessage handles audio messages
func (c *ChatSession) HandleAudioMessage(data []byte) bool {
	select {
	case c.clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("audio buffer full, discarding audio data")
	}
	return false
}

// handleListenMessage handles listen messages
func (s *ChatSession) HandleListenMessage(msg *ClientMessage) error {
	// process based on state
	switch msg.State {
	case MessageStateStart:
		s.HandleListenStart(msg)
	case MessageStateStop:
		s.HandleListenStop()
	case MessageStateDetect:
		s.HandleListenDetect(msg)
	}

	// log
	log.Infof("device %s updated audio listen state: %s", msg.DeviceID, msg.State)
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
	// check device activation status
	if msg.Text != "" {
		isActivated, err := s.CheckDeviceActivated()
		if err != nil {
			log.Errorf("check device activation status failed: %v", err)
			return err
		}
		if !isActivated {
			return nil
		}
	}

	// stop current playback
	s.StopSpeaking(false)

	// if there is text, process it
	if msg.Text != "" {
		text := removePunctuation(msg.Text)

		enableGreeting := viper.GetBool("enable_greeting")
		// wake word + greeting enabled -> welcome mode
		if isWakeupWord(text) && enableGreeting {
			if !s.clientState.IsWelcomeSpeaking {
				s.HandleWelcome()
			}
			return nil
		}

		if enableGreeting {
			// default fallback: AddAsrResultToQueue
			if err := s.AddAsrResultToQueue(text, nil); err != nil {
				log.Errorf("start conversation failed: %v", err)
			}
		}
	}
	return nil
}

func (s *ChatSession) HandleNotActivated() {
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("get config provider failed: %v", err)
		return
	}

	code, challenge, message, timeoutMs := configProvider.GetActivationInfo(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
	if code == "" {
		log.Errorf("get activation info failed: %v", err)
		return
	}

	log.Infof("activation code: %s, challenge: %s, message: %s, timeout: %d", code, challenge, message, timeoutMs)

	s.ttsManager.EnqueueTtsStart(s.clientState.Ctx)
	defer s.ttsManager.EnqueueTtsStop(s.clientState.Ctx)

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	err = s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text: fmt.Sprintf("please add device in the backend, activation code: %s", code),
	}, false)
	s.ttsManager.RequestTurnEnd(ctx, err)

}

func (s *ChatSession) HandleWelcome() {
	greetingText := s.GetRandomGreeting()
	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)

	// check if session has been stopped (by trying to acquire lock)
	if !s.stopSpeakingMu.TryLock() {
		log.Debugf("HandleWelcome is executing StopSpeaking, skipping welcome message")
		return
	}
	s.stopSpeakingMu.Unlock()

	// check if sessionCtx is canceled
	if sessionCtx.Err() != nil {
		log.Debugf("HandleWelcome sessionCtx canceled, skipping welcome message")
		return
	}

	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	// check if afterAsrCtx is canceled
	if ctx.Err() != nil {
		log.Debugf("HandleWelcome afterAsrCtx canceled, skipping welcome message")
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
	exitWords := []string{"goodbye", "step down", "exit", "exit conversation"}
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
		return "hello, what's fun."
	}
	rand.Seed(time.Now().UnixNano())
	return greetingList[rand.Intn(len(greetingList))]
}

func (s *ChatSession) AddTextToTTSQueue(text string) error {
	return s.llmManager.AddTextToTTSQueue(text)
}

func (s *ChatSession) AddTextToTTSQueueWithOptions(text string, options llmResponseChannelOptions) error {
	return s.llmManager.AddTextToTTSQueueWithOptions(text, options)
}

func (s *ChatSession) IsTTSActive() bool {
	if s == nil || s.ttsManager == nil {
		return false
	}
	return s.ttsManager.ttsActive.Load()
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
			// warm-up took over start; need to add stop here when formal OpenClaw reply ends;
			// cannot send at warm-up switch point, otherwise main reply would be interrupted mid-way.
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

	// non-streaming fallback: inject as single sentence when no correlation_id.
	if correlationID == "" {
		if text == "" {
			return nil
		}
		return s.AddTextToTTSQueue(text)
	}

	// intermediate empty segments are meaningless, skip; ending empty segment kept for finalization.
	if text == "" && !event.IsEnd {
		return nil
	}

	streamChan, created, err := s.getOrCreateOpenClawStream(correlationID)
	if err != nil {
		return err
	}

	isStart := event.IsStart
	if created && !isStart {
		// if first segment has no start flag, fallback to start first segment.
		isStart = true
	}
	if isStart {
		if task := s.getOpenClawWarmupTask(correlationID); task != nil {
			if text != "" {
				// only stop warm-up when first truly playable content arrives, avoid premature takeover by very short leading segments.
				// warm-up's own start flag is only for warm-up TTS, cannot swallow the IsStart of formal reply's first segment,
				// otherwise formal reply would degrade to single-sentence TTS, subsequent snapshots would be treated as second sentence and played again.
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

// InterruptAndClearTTSQueue triggers TTS interrupt and clears send queue (called by realtime mode VAD interrupt and similar scenarios)
func (s *ChatSession) InterruptAndClearTTSQueue() {
	if s.mediaPlayer != nil {
		if err := s.mediaPlayer.Suspend(); err != nil && !errors.Is(err, context.Canceled) {
			log.Warnf("suspend media playback failed: %v", err)
		}
	}
	s.ttsManager.InterruptAndStop(s.clientState.Ctx, true, context.Canceled)
}

// handleAbortMessage handles abort messages
func (s *ChatSession) HandleAbortMessage(msg *ClientMessage) error {
	// set interrupt state
	s.clientState.Abort = true

	if s.clientState.IsRealTime() {
		s.StopSpeakingAfterAsr(true)
	} else {
		s.StopSpeaking(true)
	}

	// log
	log.Infof("device %s abort session", msg.DeviceID)
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
				log.Debugf("device %s activation status still not activated, skipping repeated real-time verification", s.clientState.DeviceID)
				return false, nil
			}

			configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
			if err != nil {
				log.Errorf("get config provider failed: %v", err)
				return false, err
			}
			// call API to reconfirm activation status
			isActivated, err := configProvider.IsDeviceActivated(s.clientState.Ctx, s.clientState.DeviceID, "client_id")
			if err != nil {
				log.Errorf("get activation status failed: %v", err)
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
	// first check activation status
	isActivated, err := s.CheckDeviceActivated()
	if err != nil {
		log.Errorf("check device activation status failed: %v", err)
		return err
	}
	if !isActivated {
		return nil
	}

	// realtime mode first start: skip welcome check and Destroy, enter listening directly
	if msg.Mode == "realtime" {

		if !s.clientState.IsWelcomePlaying {
			s.StopSpeaking(false)
		}

		s.clientState.ListenMode = msg.Mode
		log.Infof("device %s listen mode: %s", msg.DeviceID, msg.Mode)

		startSeq := s.beginListenStart()
		go func() {
			if err := s.OnListenStart(startSeq); err != nil {
				log.Errorf("device %s listen start failed: %v", msg.DeviceID, err)
			}
		}()
		return nil
	}

	if s.clientState.IsWelcomePlaying {
		log.Infof("device %s welcome message playing, ignoring listen start", msg.DeviceID)
		return nil
	}

	if s.clientState.GetListenPhase() == ListenPhaseStarting {
		log.Infof("device %s listen start already in progress, ignoring duplicate listen start", msg.DeviceID)
		return nil
	}

	// handle audio pickup mode
	s.clientState.ListenMode = msg.Mode
	log.Infof("device %s listen mode: %s", msg.DeviceID, msg.Mode)
	//if s.clientState.ListenMode == "manual" {
	s.StopSpeaking(false)
	//}

	startSeq := s.beginListenStart()
	go func() {
		if err := s.OnListenStart(startSeq); err != nil {
			log.Errorf("device %s listen start failed: %v", msg.DeviceID, err)
		}
	}()

	return nil
}

func (s *ChatSession) HandleListenStop() error {
	/*if s.clientState.ListenMode == "auto" {
		s.clientState.CancelSessionCtx()
	}*/

	// call
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

	// realtime mode: skip Destroy, keep ASR running, but clear AudioBuffer
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

	// initialize ASR related
	if s.clientState.ListenMode == "manual" {
		s.clientState.VoiceStatus.SetClientHaveVoice(true)
	}

	// start ASR streaming recognition, reuse restartAsrRecognition function
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
		s.CloseWithReason(chatSessionCloseReasonFatalError)
		return err
	}

	if !s.isCurrentListenStart(startSeq) {
		log.Debugf("OnListenStart stale after ASR restart, cancel current start")
		s.clientState.Asr.CancelWithReason("ChatSession.OnListenStart: stale listen start after ASR restart")
		return nil
	}

	s.clientState.SetListenPhase(ListenPhaseListening)

	// define message save callback
	onMessageSave := func(userMsg *schema.Message, messageID string, audioData []float32) {
		// ASR text and audio obtained simultaneously, one-time save (no two-phase)
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: s.clientState,
			Msg:         *userMsg,
			MessageID:   messageID,
			AudioData:   [][]byte{util.Float32SliceToBytes(audioData)}, // convert to byte array
			AudioSize:   len(audioData) * 4,                            // float32 = 4 bytes
			SampleRate:  s.clientState.InputAudioFormat.SampleRate,
			Channels:    s.clientState.InputAudioFormat.Channels,
			IsUpdate:    false, // one-time save (text + audio)
			Timestamp:   time.Now(),
		})
	}

	// define error callback
	onError := func(err error) {
		log.Errorf("ASR recognition loop error: %v", err)
		s.CloseWithReason(chatSessionCloseReasonFatalError)
	}

	// start ASR recognition result processing loop (resource management inside ASRManager)
	s.asrManager.StartAsrRecognitionLoop(ctx, onMessageSave, onError)

	return nil
}

// startChat starts conversation
func (s *ChatSession) AddAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	return s.AddAsrResultToQueueWithOptions(text, speakerResult, llmResponseChannelOptions{})
}

func (s *ChatSession) AddAsrResultToQueueWithOptions(text string, speakerResult *speaker.IdentifyResult, options llmResponseChannelOptions) error {
	log.Debugf("AddAsrResultToQueue text: %s", text)
	if speakerResult != nil && speakerResult.Identified {
		log.Debugf("AddAsrResultToQueue speaker: %s (confidence: %.2f)", speakerResult.SpeakerName, speakerResult.Confidence)
	}

	// check if session has been stopped (by trying to acquire lock)
	// if StopSpeaking is executing, will wait here; if already completed, tryLock returns immediately
	if !s.stopSpeakingMu.TryLock() {
		log.Debugf("AddAsrResultToQueue StopSpeaking in progress, discarding message")
		return nil
	}
	s.stopSpeakingMu.Unlock()

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	// check if sessionCtx is canceled
	if sessionCtx.Err() != nil {
		log.Debugf("AddAsrResultToQueue sessionCtx canceled, discarding message")
		return nil
	}
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)

	item := AsrResponseChannelItem{
		ctx:           ctx,
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
			log.Errorf("process conversation failed: %v", err)
			continue
		}
	}
}

func (s *ChatSession) ClearChatTextQueue() {
	s.chatTextQueue.Clear()
}

// DoExitChat executes exit chat logic (send goodbye message and close session)
func (s *ChatSession) DoExitChat() {
	// friendly goodbye message
	goodbyeText := "Okay, goodbye! Looking forward to chatting with you again~"

	// save an assistant role message
	goodbyeMsg := schema.AssistantMessage(goodbyeText, nil)
	if err := s.llmManager.AddLlmMessage(s.clientState.Ctx, goodbyeMsg); err != nil {
		log.Errorf("save goodbye message failed: %v", err)
	}

	// get context
	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	ctx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)

	// send TTS goodbye message
	s.ttsManager.EnqueueTtsStart(ctx)

	err := s.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
		Text:    goodbyeText,
		IsStart: true,
		IsEnd:   true,
	}, true) // sync processing, wait for TTS to complete

	if err != nil {
		log.Errorf("send goodbye message failed: %v", err)
	}

	s.ttsManager.RequestTurnEnd(ctx, err)
	s.ttsManager.EnqueueTtsStop(ctx)
	// close session
	s.CloseWithReason(chatSessionCloseReasonExplicitExit)
}

func (s *ChatSession) Close() {
	s.CloseWithReason(chatSessionCloseReasonManagerShutdown)
}

func (s *ChatSession) IsClosing() bool {
	if s == nil {
		return true
	}
	return s.closing.Load()
}

func (s *ChatSession) CloseWithReason(reason string) {
	s.closing.Store(true)
	s.closeOnce.Do(func() {
		// cleanup ASR resources (resource management inside ASRManager)
		if s.asrManager != nil {
			s.asrManager.Cleanup()
		}
		deviceID := ""
		if s.clientState != nil {
			deviceID = s.clientState.DeviceID
		}
		log.Debugf("ChatSession.Close() starting to clean up session resources, device %s", deviceID)

		if s.mediaPlayer != nil {
			s.mediaPlayer.DetachSession(true)
		}

		// cancel session-level context
		if s.cancel != nil {
			s.cancel()
		}
		s.finishOpenClawWarmup("", false)

		// cleanup chat text queue
		s.ClearChatTextQueue()
		s.clearOpenClawStreams()

		// stop speaking and cleanup audio related resources. Close path already called DetachSession(true),
		// do not Suspend media again here, otherwise resumeOnAttach would be cleared.
		s.stopSpeakingWithLock(true, true, false)

		if s.speakerManager != nil {
			s.speakerManager.Close()
		}

		if s.clientState != nil {
			eventbus.Get().Publish(eventbus.TopicSessionEnd, s.clientState)
		}

		log.Debugf("ChatSession.Close() session resource cleanup completed, device %s", deviceID)

		if s.closeHandler != nil {
			s.closeHandler(s, reason)
		}
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
				_ = s.AddTextToTTSQueue("Exited OpenClaw mode")
				log.Infof("device %s exited OpenClaw mode: agent=%s exited=%v", deviceID, agentID, exited)
				return nil
			}

			log.Infof(
				"OpenClaw send STT: agent=%s device=%s session=%s text_len=%d text_snippet=%q",
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
					"device %s OpenClaw message send failed, falling back to normal mode: agent=%s session=%s text_snippet=%q err=%v",
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
				log.Infof("OpenClaw send STT succeeded: agent=%s device=%s session=%s message_id=%s", agentID, deviceID, openclawSessionID, messageID)
			}
			return nil
		}

		if isEnterKeyword {
			if !openclawManager.EnterMode(agentID, deviceID) {
				_ = s.AddTextToTTSQueue("OpenClaw currently unavailable, please try again later")
				log.Warnf("device %s enter OpenClaw mode failed: agent=%s agent session not ready", deviceID, agentID)
				return nil
			}
			_ = s.AddTextToTTSQueue("Entered OpenClaw mode, please continue")
			log.Infof("device %s entered OpenClaw mode: agent=%s trigger=%q", deviceID, agentID, openClawLogSnippet(trimmedText, 32))
			return nil
		}
		log.Debugf(
			"OpenClaw not handling current STT: agent=%s device=%s mode=%v enter_keyword=%v text_snippet=%q",
			agentID,
			deviceID,
			isOpenClawMode,
			isEnterKeyword,
			openClawLogSnippet(trimmedText, 64),
		)
	} else {
		s.finishOpenClawWarmup("", false)
		if openclawManager.ExitMode(agentID, deviceID) {
			log.Debugf("OpenClaw config not enabled, forced exit mode: agent=%s device=%s", agentID, deviceID)
		}
	}

	if s.checkExitWords(text) {
		// publish exit chat event
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "user actively exited",
			TriggerType: "exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		return nil
	}

	clientState := s.clientState

	sessionID := clientState.SessionID

	// dynamically switch TTS after speaker recognition (restore default TTS when not recognized)
	if err := s.switchTTSForSpeaker(speakerResult); err != nil {
		log.Warnf("switch TTS failed: %v", err)
		// do not interrupt flow, continue using current TTS
	}

	// directly create Eino native message
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: text,
	}

	// get global MCP tool list
	mcpTools, err := mcp.GetToolsByDeviceIdWithTransport(
		clientState.DeviceID,
		clientState.AgentID,
		s.serverTransport.GetTransportType(),
		clientState.DeviceConfig.MCPServiceNames,
	)
	if err != nil {
		log.Errorf("get tools for device %s failed: %v", clientState.DeviceID, err)
		mcpTools = make(map[string]tool.InvokableTool)
	}
	if !hasAvailableKnowledgeBase(clientState.DeviceConfig.KnowledgeBases) {
		if _, ok := mcpTools["search_knowledge"]; ok {
			delete(mcpTools, "search_knowledge")
			log.Infof("device %s has no available knowledge base, removed tool search_knowledge", clientState.DeviceID)
		}
	}

	// convert MCP tools to interface format for passing to conversion function
	mcpToolsInterface := make(map[string]interface{})
	for name, tool := range mcpTools {
		mcpToolsInterface[name] = tool
	}

	// convert MCP tools to Eino ToolInfo format
	einoTools, err := llm.ConvertMCPToolsToEinoTools(ctx, mcpToolsInterface)
	if err != nil {
		log.Errorf("convert MCP tools failed: %v", err)
		einoTools = nil
	}

	toolNameList := make([]string, 0)
	for _, tool := range einoTools {
		toolNameList = append(toolNameList, tool.Name)
	}

	// send LLM request with tools
	log.Infof("sending LLM request with %d MCP tools, tools: %+v", len(einoTools), toolNameList)

	err = s.llmManager.DoLLmRequest(ctx, userMessage, einoTools, true, speakerResult)
	if err != nil {
		log.Errorf("send LLM request with tools failed, sessionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("send LLM request with tools failed: %v", err)
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

// switchTTSForSpeaker switches TTS for recognized speaker
func (s *ChatSession) switchTTSForSpeaker(speakerResult *speaker.IdentifyResult) error {
	s.clientState.SpeakerTTSConfig = nil

	// 1. check if speakerResult is nil
	if speakerResult == nil {
		log.Debug("speakerResult is nil, clearing speaker TTS config")
		return nil
	}

	// 2. find speaker group config
	speakerGroupInfo, found := s.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]
	if !found {
		// config not found, clear speaker TTS config
		log.Debugf("config not found for speaker group %s, clearing speaker TTS config", speakerResult.SpeakerName)
		return nil
	}

	// 3. check if custom voice is configured
	if speakerGroupInfo.TTSConfigID == nil || *speakerGroupInfo.TTSConfigID == "" {
		// no custom voice configured, clear speaker TTS config
		log.Debugf("speaker group %s has no custom TTS configured, clearing speaker TTS config", speakerResult.SpeakerName)
		return nil
	}

	// 4. find corresponding TTS config from system config (viper)
	var targetTTSConfig *types.TtsConfigItem
	ttsConfigsRaw := viper.Get("tts")
	if ttsConfigsRaw == nil {
		return fmt.Errorf("tts not found in system config")
	}

	// parse tts config (now a map with config_id as key)
	if ttsConfigsMap, ok := ttsConfigsRaw.(map[string]interface{}); ok {
		// find matching config_id
		if configItem, exists := ttsConfigsMap[*speakerGroupInfo.TTSConfigID]; exists {
			if configMap, ok := configItem.(map[string]interface{}); ok {
				// parse config item
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
				// other fields of config item used directly as config
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
		return fmt.Errorf("TTS config not found: %s", *speakerGroupInfo.TTSConfigID)
	}

	// 5. copy TTS config to avoid modifying original
	ttsConfig := make(map[string]interface{})
	for k, v := range targetTTSConfig.Config {
		ttsConfig[k] = v
	}

	// 6. if voice value configured, override in TTS config
	if speakerGroupInfo.Voice != nil && *speakerGroupInfo.Voice != "" {
		// set corresponding voice field based on provider
		if targetTTSConfig.Provider == "cosyvoice" {
			ttsConfig["spk_id"] = *speakerGroupInfo.Voice
		} else {
			ttsConfig["voice"] = *speakerGroupInfo.Voice
		}
		log.Debugf("set voice for speaker %s: %s", speakerResult.SpeakerName, *speakerGroupInfo.Voice)
	}
	if targetTTSConfig.Provider == "aliyun_qwen" &&
		speakerGroupInfo.VoiceModelOverride != nil &&
		strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride) != "" {
		overrideModel := strings.TrimSpace(*speakerGroupInfo.VoiceModelOverride)
		ttsConfig["model"] = overrideModel
		log.Debugf("override Qwen model for speaker %s: %s", speakerResult.SpeakerName, overrideModel)
	}

	// 7. save complete TTS config (deep copy)
	s.clientState.SpeakerTTSConfig = make(map[string]interface{})
	for k, v := range ttsConfig {
		s.clientState.SpeakerTTSConfig[k] = v
	}
	// ensure provider is in config
	s.clientState.SpeakerTTSConfig["provider"] = targetTTSConfig.Provider

	log.Infof("switched TTS config for speaker %s successfully - Provider: %s, ConfigID: %s, Voice: %v",
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
