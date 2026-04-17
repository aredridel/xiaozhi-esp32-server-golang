package chat

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	chathooks "xiaozhi-esp32-server-golang/internal/domain/chat/hooks"
	"xiaozhi-esp32-server-golang/internal/domain/chat/streamtransform"
	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/pool"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

const (
	MaxMessageCount = 10

	McpReadResourcePageSize       = 100 * 1024
	McpReadResourceStreamDoneFlag = "[DONE]"
)

// Context key type to avoid collisions
type contextKey int

const (
	ttsStopDelayDuration time.Duration = 200 * time.Millisecond
	fullTextKey          contextKey    = iota
	toolRoundMessagesKey
	ttsTurnTrackerKey
	ttsPlaybackStartHookKey
)

const (
	interruptExtraKey      = "interrupt"
	interruptByExtraKey    = "interrupt_by"
	interruptStageExtraKey = "interrupt_stage"
	interruptContentSuffix = " [user interrupted]"
)

// GetLastMessageID gets the MessageID of the most recently saved message (for two-phase save)
func (l *LLMManager) GetLastMessageID(role string) (string, bool) {
	l.lastMessageIDMu.RLock()
	defer l.lastMessageIDMu.RUnlock()
	id, ok := l.lastMessageID[role]
	return id, ok
}

type LLMResponseChannelItem struct {
	ctx          context.Context
	userMessage  *schema.Message
	responseChan chan llm_common.LLMResponseStruct
	onStartFunc  func(args ...any)
	onEndFunc    func(err error, args ...any)
}

type llmHandleResult struct {
	ok                      bool
	suppressProtocolTtsStop bool
}

func llmHandleResultFromArgs(args []any) llmHandleResult {
	if len(args) == 0 {
		return llmHandleResult{}
	}
	result, ok := args[0].(llmHandleResult)
	if !ok {
		return llmHandleResult{}
	}
	return result
}

type llmResponseChannelOptions struct {
	disableTTSCommands bool
	onStartFunc        func(args ...any)
	onEndFunc          func(err error, args ...any)
	onTTSPlaybackStart func()
}

type ttsPlaybackStartHook func()

func withTTSPlaybackStartHook(ctx context.Context, hook func()) context.Context {
	if ctx == nil || hook == nil {
		return ctx
	}

	var once sync.Once
	return context.WithValue(ctx, ttsPlaybackStartHookKey, ttsPlaybackStartHook(func() {
		once.Do(hook)
	}))
}

func ttsPlaybackStartHookFromContext(ctx context.Context) func() {
	if ctx == nil {
		return nil
	}
	hook, ok := ctx.Value(ttsPlaybackStartHookKey).(ttsPlaybackStartHook)
	if !ok || hook == nil {
		return nil
	}
	return func() {
		hook()
	}
}

type ttsTurnTracker struct {
	mu      sync.Mutex
	pending int
	doneCh  chan struct{}
}

func newTTSTurnTracker() *ttsTurnTracker {
	doneCh := make(chan struct{})
	close(doneCh)
	return &ttsTurnTracker{doneCh: doneCh}
}

func (t *ttsTurnTracker) Add() func(error) {
	if t == nil {
		return func(error) {}
	}

	t.mu.Lock()
	if t.pending == 0 {
		t.doneCh = make(chan struct{})
	}
	t.pending++
	t.mu.Unlock()

	var once sync.Once
	return func(error) {
		once.Do(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if t.pending == 0 {
				return
			}
			t.pending--
			if t.pending == 0 {
				close(t.doneCh)
			}
		})
	}
}

func (t *ttsTurnTracker) Wait(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	t.mu.Lock()
	pending := t.pending
	doneCh := t.doneCh
	t.mu.Unlock()

	if pending == 0 {
		return nil
	}

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type LLMManager struct {
	clientState       *ClientState
	session           *ChatSession
	serverTransport   *ServerTransport
	ttsManager        *TTSManager
	transformRegistry *streamtransform.Registry

	einoTools []*schema.ToolInfo

	llmResponseQueue *util.Queue[LLMResponseChannelItem]

	// store the MessageID of the most recently saved message (for two-phase save)
	// key: role (user/assistant), value: MessageID
	lastMessageID   map[string]string
	lastMessageIDMu sync.RWMutex // protect concurrent access to lastMessageID
}

func NewLLMManager(clientState *ClientState, serverTransport *ServerTransport, ttsManager *TTSManager, session *ChatSession, transformRegistry *streamtransform.Registry) *LLMManager {
	return &LLMManager{
		clientState:       clientState,
		session:           session,
		serverTransport:   serverTransport,
		ttsManager:        ttsManager,
		transformRegistry: transformRegistry,
		llmResponseQueue:  util.NewQueue[LLMResponseChannelItem](10),
		lastMessageID:     make(map[string]string),
	}
}

func (l *LLMManager) openOutputPipeline(ctx context.Context) (*streamtransform.Pipeline, error) {
	if l == nil || l.transformRegistry == nil {
		return &streamtransform.Pipeline{}, nil
	}

	sessionID := ""
	deviceID := ""
	if l.clientState != nil {
		sessionID = l.clientState.SessionID
		deviceID = l.clientState.DeviceID
	}

	return l.transformRegistry.Open(streamtransform.Context{
		Ctx:       ctx,
		SessionID: sessionID,
		DeviceID:  deviceID,
		RequestID: fmt.Sprintf("%s-%d", sessionID, time.Now().UnixNano()),
	})
}

func (l *LLMManager) emitLLMOutputRaw(ctx context.Context, data chathooks.LLMOutputRawData) (chathooks.LLMOutputRawData, bool, error) {
	if l == nil || l.session == nil || l.session.hookHub == nil {
		return data, false, nil
	}
	return l.session.hookHub.EmitLLMOutputRaw(l.session.hookContext(ctx), data)
}

// handleLLMWithContextAndTools handles LLM response using context control (compatible with and without tools)
// internally manages LLM resource acquisition and release
func (l *LLMManager) handleLLMWithContextAndTools(
	ctx context.Context,
	dialogue []*schema.Message,
	tools []*schema.ToolInfo,
) (chan llm_common.LLMResponseStruct, error) {
	// acquire LLM resource
	llmWrapper, err := pool.Acquire[llm.LLMProvider](
		"llm",
		l.clientState.DeviceConfig.Llm.Provider,
		l.clientState.DeviceConfig.Llm.Config,
	)
	if err != nil {
		return nil, fmt.Errorf("acquire LLM resource failed: %w", err)
	}

	// get provider
	llmProvider := llmWrapper.GetProvider()

	// call LLM provider
	msgChan := llmProvider.ResponseWithContext(ctx, l.clientState.SessionID, dialogue, tools)

	pipeline, err := l.openOutputPipeline(ctx)
	if err != nil {
		pool.Release(llmWrapper)
		return nil, fmt.Errorf("create LLM output stream transform pipeline failed: %w", err)
	}

	// create response channel
	responseChannel := make(chan llm_common.LLMResponseStruct, 2)
	startTs := time.Now().UnixMilli()
	var firstSegment bool
	var rawFullText strings.Builder

	// start goroutine to process response
	go func() {
		defer func() {
			log.Debugf("full Response with %d tools, fullText: %s", len(tools), rawFullText.String())
			close(responseChannel)
			if closeErr := pipeline.Close(); closeErr != nil {
				log.Warnf("close LLM output stream transform pipeline failed: %v", closeErr)
			}
			// release resource
			pool.Release(llmWrapper)
			log.Debugf("LLM resource released")
		}()

		isFirstOutput := true
		llmFirstTokenMarked := false

		emitResponse := func(item streamtransform.Item) bool {
			response := llm_common.LLMResponseStruct{
				IsEnd: item.IsEnd,
			}

			switch item.Kind {
			case streamtransform.ItemKindToolCalls:
				response.ToolCalls = item.ToolCalls
				if len(item.ToolCalls) > 0 {
					response.IsStart = isFirstOutput
				}
			case streamtransform.ItemKindTextDelta, streamtransform.ItemKindTextSegment:
				response.Text = item.Text
				if strings.TrimSpace(item.Text) != "" {
					response.IsStart = isFirstOutput
					if !firstSegment {
						firstSegment = true
						log.Infof("timing: llm tool first sentence: %d ms", time.Now().UnixMilli()-startTs)
					}
					if isFirstOutput {
						isFirstOutput = false
					}
				}
			default:
				return true
			}

			if strings.TrimSpace(response.Text) == "" && len(response.ToolCalls) == 0 && !response.IsEnd {
				return true
			}

			select {
			case <-ctx.Done():
				log.Infof("context canceled, stopping LLM response processing: %v, context done, exit", ctx.Err())
				return false
			case responseChannel <- response:
				return true
			}
		}

		pushToPipeline := func(item streamtransform.Item) (bool, error) {
			items, stop, err := pipeline.Push(item)
			if err != nil {
				return false, err
			}
			for _, out := range items {
				if !emitResponse(out) {
					return true, nil
				}
			}
			return stop, nil
		}

		pushRawText := func(delta string, isEnd bool, errVal error) (bool, error) {
			payload, stop, hookErr := l.emitLLMOutputRaw(ctx, chathooks.LLMOutputRawData{
				Delta:    delta,
				FullText: rawFullText.String(),
				IsEnd:    isEnd,
				Err:      errVal,
			})
			if hookErr != nil {
				log.Warnf("LLM_OUTPUT_RAW hook execution failed: %v", hookErr)
			}
			if stop {
				log.Infof("LLM_OUTPUT_RAW hook requested to stop current flow")
				return true, nil
			}
			if payload.Delta != "" {
				rawFullText.WriteString(payload.Delta)
			}
			return pushToPipeline(streamtransform.Item{
				Kind:  streamtransform.ItemKindTextDelta,
				Text:  payload.Delta,
				IsEnd: payload.IsEnd,
			})
		}

		pushRawToolCalls := func(toolCalls []schema.ToolCall) (bool, error) {
			payload, stop, hookErr := l.emitLLMOutputRaw(ctx, chathooks.LLMOutputRawData{
				FullText:  rawFullText.String(),
				ToolCalls: toolCalls,
			})
			if hookErr != nil {
				log.Warnf("LLM_OUTPUT_RAW hook execution failed: %v", hookErr)
			}
			if stop {
				log.Infof("LLM_OUTPUT_RAW hook requested to stop current flow")
				return true, nil
			}
			if len(payload.ToolCalls) == 0 {
				return false, nil
			}
			return pushToPipeline(streamtransform.Item{
				Kind:      streamtransform.ItemKindToolCalls,
				ToolCalls: payload.ToolCalls,
			})
		}

		for {
			select {
			case <-ctx.Done():
				log.Infof("context canceled, stopping LLM response processing: %v, context done, exit", ctx.Err())
				return
			case message, ok := <-msgChan:
				if !ok {
					stop, pushErr := pushRawText("", true, nil)
					if pushErr != nil {
						log.Errorf("process LLM end stream failed: %v", pushErr)
					}
					if stop || pushErr != nil {
						return
					}
					return
				}
				if message == nil {
					continue
				}
				if llm.IsLLMErrorMessage(message) {
					errMsg := llm.LLMErrorMessage(message)
					log.Warnf("LLM returned error: %s", errMsg)
					stop, pushErr := pushRawText(errMsg, true, nil)
					if pushErr != nil {
						log.Errorf("process LLM error output failed: %v", pushErr)
					}
					if stop || pushErr != nil {
						return
					}
					return
				}
				if message.Content != "" {
					if !llmFirstTokenMarked {
						firstTokenTs := time.Now().UnixMilli()
						l.clientState.MarkLlmFirstToken()
						if l.session != nil {
							l.session.TraceLlmFirstToken(ctx, firstTokenTs)
						}
						llmFirstTokenMarked = true
					}
					stop, pushErr := pushRawText(message.Content, false, nil)
					if pushErr != nil {
						log.Errorf("process LLM text stream failed: %v", pushErr)
						return
					}
					if stop {
						return
					}
				}
				if len(message.ToolCalls) > 0 {
					log.Infof("processing tool calls: %+v", message.ToolCalls)
					stop, pushErr := pushRawToolCalls(message.ToolCalls)
					if pushErr != nil {
						log.Errorf("process LLM tool stream failed: %v", pushErr)
						return
					}
					if stop {
						return
					}
				}
			}
		}
	}()

	return responseChannel, nil
}

func (l *LLMManager) Start(ctx context.Context) {
	l.processLLMResponseQueue(ctx)
}

func (l *LLMManager) processLLMResponseQueue(ctx context.Context) {
	for {
		item, err := l.llmResponseQueue.Pop(ctx, 0) // blocking
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			// other errors
			continue
		}

		log.Debugf("processLLMResponseQueue item: %+v", item)
		if item.onStartFunc != nil {
			item.onStartFunc()
		}

		// call handleLLMResponse, it gets fullText and toolCalls from context and populates them
		result, err := l.handleLLMResponse(item.ctx, item.userMessage, item.responseChan)
		if waitErr := waitForTTSTurnDrainIfRoot(item.ctx); err == nil && waitErr != nil {
			err = waitErr
		}

		if item.onEndFunc != nil {
			item.onEndFunc(err, result)
		}
	}
}

func (l *LLMManager) ClearLLMResponseQueue() {
	l.llmResponseQueue.Clear()
}

func (l *LLMManager) AddTextToTTSQueue(text string) error {
	return l.AddTextToTTSQueueWithOptions(text, llmResponseChannelOptions{})
}

func (l *LLMManager) AddTextToTTSQueueWithOptions(text string, options llmResponseChannelOptions) error {
	log.Debugf("AddTextToTTSQueue text: %s", text)
	msg := &schema.Message{
		Role:    schema.User,
		Content: text,
	}
	llmResponseChan := make(chan llm_common.LLMResponseStruct, 10)
	llmResponseChan <- llm_common.LLMResponseStruct{
		IsStart: true,
		IsEnd:   true,
		Text:    text,
	}
	close(llmResponseChan)

	sessionCtx := l.clientState.SessionCtx.Get(l.clientState.Ctx)
	ctx := l.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)
	if err := l.HandleLLMResponseChannelAsyncWithOptions(ctx, msg, llmResponseChan, options); err != nil {
		log.Warnf("AddTextToTTSQueue enqueue failed: %v", err)
		return err
	}

	return nil
}

func chainLLMResponseStartHooks(hooks ...func(args ...any)) func(args ...any) {
	filtered := make([]func(args ...any), 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func(args ...any) {
		for _, hook := range filtered {
			hook(args...)
		}
	}
}

func chainLLMResponseEndHooks(hooks ...func(err error, args ...any)) func(err error, args ...any) {
	filtered := make([]func(err error, args ...any), 0, len(hooks))
	for _, hook := range hooks {
		if hook != nil {
			filtered = append(filtered, hook)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func(err error, args ...any) {
		for _, hook := range filtered {
			hook(err, args...)
		}
	}
}

func (l *LLMManager) HandleLLMResponseChannelAsync(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct) error {
	return l.handleLLMResponseChannelAsync(ctx, userMessage, responseChan, llmResponseChannelOptions{})
}

func (l *LLMManager) HandleLLMResponseChannelAsyncWithOptions(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct, options llmResponseChannelOptions) error {
	return l.handleLLMResponseChannelAsync(ctx, userMessage, responseChan, options)
}

func (l *LLMManager) handleLLMResponseChannelAsync(ctx context.Context, userMessage *schema.Message, responseChan chan llm_common.LLMResponseStruct, options llmResponseChannelOptions) error {
	ctx = ensureTTSTurnTrackerInContext(ctx)
	ctx = withTTSPlaybackStartHook(ctx, options.onTTSPlaybackStart)

	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}
	if options.disableTTSCommands {
		needSendTtsCmd = false
	}

	// initialize or reuse fullText in context (for chat history)
	// if fullText already exists in context (continuing LLM request after tool call), reuse; otherwise create new
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("reusing existing fullText, current length: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("created new fullText")
	}

	var onStartFunc func(...any)
	var onEndFunc func(err error, args ...any)

	if needSendTtsCmd {
		onStartFunc = func(...any) {
			// determine if this is the first LLM call (via context's nest value), only clear TTS audio cache on first call
			val := ctx.Value("nest")
			if nest, ok := val.(int); !ok || nest <= 1 {
				// first call or no nest value, clear TTS audio cache
				l.ttsManager.ClearAudioHistory()
				log.Debugf("onStartFunc first call, TTS audio cache cleared")
			}
			l.ttsManager.EnqueueTtsStart(ctx)
		}
		onEndFunc = func(err error, args ...any) {
			handleResult := llmHandleResultFromArgs(args)
			l.clientState.MarkLlmEnd()
			if l.session != nil {
				l.session.TraceLlmEnd(ctx, time.Now().UnixMilli(), err)
			}
			strFullText := fullText.String()

			if handleResult.suppressProtocolTtsStop {
				l.ttsManager.FinishTtsWithoutProtocolStop(ctx, err)
			} else if !l.clientState.IsRealTime() {
				// in non-realtime mode, TtsStop is sent uniformly by runSenderLoop
				l.ttsManager.EnqueueTtsStop(ctx)
			}
			l.ttsManager.RequestTurnEnd(ctx, err)

			// get fullText from closure
			audioData := l.ttsManager.GetAndClearAudioHistory()

			// calculate total audio size (sum of all frame bytes)
			audioSize := 0
			for _, frame := range audioData {
				audioSize += len(frame)
			}

			// only send event on first call (nest<=1)
			if nest <= 1 {
				// get MessageID from LLMManager (Assistant role)
				// if MessageID not found, means first-phase save is incomplete, skip second-phase update
				messageID, ok := l.GetLastMessageID(string(schema.Assistant))
				if !ok {
					log.Warnf("MessageID not found when TTS completed, skipping second phase audio update")
					return
				}

				// publish event: second phase (update audio)
				assistantMsg := schema.AssistantMessage(strFullText, nil)
				eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
					ClientState: l.clientState,
					Msg:         *assistantMsg,
					MessageID:   messageID,
					AudioData:   audioData, // second phase: has audio
					AudioSize:   audioSize,
					SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
					Channels:    l.clientState.OutputAudioFormat.Channels,
					Timestamp:   time.Now(),
					IsUpdate:    true, // update message
				})
			}
		}
	}

	onStartFunc = chainLLMResponseStartHooks(onStartFunc, options.onStartFunc)
	onEndFunc = chainLLMResponseEndHooks(onEndFunc, options.onEndFunc)

	item := LLMResponseChannelItem{
		ctx:          ctx,
		userMessage:  userMessage,
		responseChan: responseChan,
		onStartFunc:  onStartFunc,
		onEndFunc:    onEndFunc,
	}

	err := l.llmResponseQueue.Push(item)
	if err != nil {
		log.Warnf("llmResponseQueue full or closed, dropping message")
		return fmt.Errorf("llmResponseQueue full or closed, dropping message")
	}
	return nil
}

func (l *LLMManager) HandleLLMResponseChannelSync(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct, einoTools []*schema.ToolInfo) (bool, error) {
	ctx = ensureTTSTurnTrackerInContext(ctx)

	needSendTtsCmd := true
	val := ctx.Value("nest")
	nest := 0
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if n, ok := val.(int); ok {
		nest = n
		if nest > 1 {
			needSendTtsCmd = false
		}
	}

	// initialize or reuse fullText in context (for chat history)
	// if fullText already exists in context (continuing LLM request after tool call), reuse; otherwise create new
	var fullText *strings.Builder
	if existingFullText, ok := ctx.Value(fullTextKey).(*strings.Builder); ok && existingFullText != nil {
		fullText = existingFullText
		log.Debugf("reusing existing fullText, current length: %d", fullText.Len())
	} else {
		fullText = &strings.Builder{}
		ctx = context.WithValue(ctx, fullTextKey, fullText)
		log.Debugf("created new fullText")
	}

	if needSendTtsCmd {
		// determine if this is the first LLM call (via context's nest value), only clear TTS audio cache on first call
		if nest <= 1 {
			// first call or no nest value, clear TTS audio cache
			l.ttsManager.ClearAudioHistory()
			log.Debugf("HandleLLMResponseChannelSync first call, TTS audio cache cleared")
		}
		l.ttsManager.EnqueueTtsStart(ctx)
	}

	result, err := l.handleLLMResponse(ctx, userMessage, llmResponseChannel)
	if waitErr := waitForTTSTurnDrainIfRoot(ctx); err == nil && waitErr != nil {
		err = waitErr
	}
	l.clientState.MarkLlmEnd()
	if l.session != nil {
		l.session.TraceLlmEnd(ctx, time.Now().UnixMilli(), err)
	}
	strFullText := fullText.String()

	if needSendTtsCmd {
		if result.suppressProtocolTtsStop {
			l.ttsManager.FinishTtsWithoutProtocolStop(ctx, err)
		} else if !l.clientState.IsRealTime() {
			l.ttsManager.EnqueueTtsStop(ctx)
		}
		l.ttsManager.RequestTurnEnd(ctx, err)

		// collect TTS audio and send chat history event
		// note: LLM response after tool call (nest > 1) also accumulates audio to cache, but won't clear
		// only clear cache and send event on first call (nest<=1)
		audioData := l.ttsManager.GetAndClearAudioHistory()

		// calculate total audio size (sum of all frame bytes)
		audioSize := 0
		for _, frame := range audioData {
			audioSize += len(frame)
		}

		// only send event on first call (nest<=1)
		if nest <= 1 {
			// get MessageID from LLMManager (Assistant role)
			// if MessageID not found, means first-phase save is incomplete, skip second-phase update
			messageID, ok := l.GetLastMessageID(string(schema.Assistant))
			if !ok {
				log.Warnf("MessageID not found when TTS completed, skipping second phase audio update")
				return result.ok, err
			}

			// publish event: second phase (update audio)
			assistantMsg := schema.AssistantMessage(strFullText, nil)
			eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
				ClientState: l.clientState,
				Msg:         *assistantMsg,
				MessageID:   messageID,
				AudioData:   audioData, // second phase: has audio
				AudioSize:   audioSize,
				SampleRate:  l.clientState.OutputAudioFormat.SampleRate,
				Channels:    l.clientState.OutputAudioFormat.Channels,
				Timestamp:   time.Now(),
			})
		}
	} else {
		// when nest > 1: although no TTS command is sent, audio data still accumulates in cache
		// these audio data will be collected together when first response ends (nest <= 1)
		log.Debugf("LLM response after tool call (nest=%d), audio data will accumulate in cache", nest)
	}

	return result.ok, err
}

// handleLLMResponse handles LLM response
func (l *LLMManager) handleLLMResponse(ctx context.Context, userMessage *schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (llmHandleResult, error) {
	log.Debugf("handleLLMResponse start")
	defer log.Debugf("handleLLMResponse end")

	// get fullText from context (for chat history)
	fullText := ctx.Value(fullTextKey).(*strings.Builder)
	state := l.clientState
	// toolCalls uses local variable (internal tool call logic, not related to chat history)
	var toolCalls []schema.ToolCall
	toolExecCtx := context.WithValue(ctx, "nest", 2)
	toolExecCtx = context.WithValue(toolExecCtx, fullTextKey, fullText)
	if speechStartHook := ttsPlaybackStartHookFromContext(ctx); speechStartHook != nil {
		toolExecCtx = withTTSPlaybackStartHook(toolExecCtx, speechStartHook)
	}
	if l.clientState.GetMemoryMode() == MemoryModeNone && userMessage != nil {
		toolExecCtx = appendToolRoundMessagesToContext(toolExecCtx, []*schema.Message{userMessage})
	}
	ttsTracker := ttsTurnTrackerFromContext(ctx)
	var onTTSItemEnqueued func() func(error)
	onTTSPlaybackStart := ttsPlaybackStartHookFromContext(ctx)
	if ttsTracker != nil {
		onTTSItemEnqueued = ttsTracker.Add
	}
	toolExecutor := newToolCallExecutor(l, toolExecCtx)
	assistantSaved := false
	result := llmHandleResult{}

	saveInterruptedAssistant := func() {
		if assistantSaved {
			return
		}
		if ctx.Err() == nil {
			return
		}
		text := strings.TrimSpace(fullText.String())
		if text == "" {
			return
		}
		msg := schema.AssistantMessage(text, nil)
		msg.Extra = map[string]any{
			interruptExtraKey:      true,
			interruptByExtraKey:    "user",
			interruptStageExtraKey: "llm",
		}
		if err := l.AddLlmMessage(ctx, msg); err != nil {
			log.Errorf("save interrupted assistant message failed: %v", err)
			return
		}
		assistantSaved = true
	}

	select {
	case <-ctx.Done():
		saveInterruptedAssistant()
		log.Debugf("handleLLMResponse ctx done, return")
		return result, nil
	default:
	}

	for {
		select {
		case <-ctx.Done():
			// context canceled, prioritize cancellation logic
			saveInterruptedAssistant()
			log.Infof("%s context canceled, stopping LLM response processing, context done, exit", state.DeviceID)
			return result, nil
		default:
			// non-blocking check, if ctx is not Done, continue processing LLM response
			select {
			case llmResponse, ok := <-llmResponseChannel:
				if !ok {
					// channel closed, exit goroutine
					log.Infof("LLM response channel closed, exiting goroutine")
					result.ok = true
					return result, nil
				}

				log.Debugf("LLM response: %+v", llmResponse)

				if len(llmResponse.ToolCalls) > 0 {
					log.Debugf("got tools: %+v", llmResponse.ToolCalls)
					toolCalls = append(toolCalls, llmResponse.ToolCalls...)
					toolExecutor.Submit(llmResponse.ToolCalls)
				}

				hasText := strings.TrimSpace(llmResponse.Text) != ""
				if hasText || llmResponse.IsStart || llmResponse.IsEnd {
					// dual-stream ending relies on empty text's IsEnd signal, cannot only pass to TTS when there is text.
					if err := l.ttsManager.handleTextResponseWithHooks(ctx, llmResponse, false, onTTSItemEnqueued, onTTSPlaybackStart); err != nil {
						result.ok = true
						return result, err
					}
				}
				if hasText {
					fullText.WriteString(llmResponse.Text)
				}

				if llmResponse.IsEnd {
					if len(toolCalls) == 0 {
						//write to Redis
						if userMessage != nil {
							if userMessage.Role == schema.User {
								// check if user message already saved (saved during ASR processing)
								// determine by checking if last message is user message with matching content
								/*messages := l.clientState.GetMessages(1)
								shouldSave := true
								if len(messages) > 0 {
									lastMsg := messages[len(messages)-1]
									if lastMsg.Role == schema.User && lastMsg.Content == userMessage.Content {
										// user message already saved (during ASR processing), skip
										shouldSave = false
										log.Debugf("user message already saved during ASR processing, skip duplicate save: %s", userMessage.Content)
									}
								}
								if shouldSave {
									if err := l.AddLlmMessage(ctx, userMessage); err != nil {
										log.Errorf("failed to save user message: %v", err)
									}
								}*/
							}
						}
						strFullText := fullText.String()
						if strings.TrimSpace(strFullText) != "" || len(toolCalls) > 0 {
							if err := l.AddLlmMessage(ctx, schema.AssistantMessage(strFullText, toolCalls)); err != nil {
								log.Errorf("save assistant message failed: %v", err)
							} else {
								assistantSaved = true
							}
						}
					}
					if len(toolCalls) > 0 {
						toolSummary, err := l.handleToolCallResponse(toolExecCtx, schema.AssistantMessage(fullText.String(), toolCalls), toolCalls, toolExecutor)
						if err != nil {
							log.Errorf("handle tool call response failed: %v", err)
							result.ok = true
							return result, fmt.Errorf("handle tool call response failed: %v", err)
						}
						result.suppressProtocolTtsStop = toolSummary.hasMediaOutput
						if !toolSummary.invokeToolSuccess && strings.TrimSpace(llmResponse.Text) != "" {
							if err := l.ttsManager.handleTextResponseWithHooks(ctx, llmResponse, false, nil, onTTSPlaybackStart); err != nil {
								result.ok = true
								return result, err
							}
							fullText.WriteString(llmResponse.Text)
						}
					}

					result.ok = true
					return result, nil
				}
			case <-ctx.Done():
				// context canceled, exit goroutine
				saveInterruptedAssistant()
				log.Infof("%s context canceled, stopping LLM response processing, context done, exit", state.DeviceID)
				return result, nil
			}
		}
	}
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, userMessage *schema.Message, einoTools []*schema.ToolInfo, isSync bool, speakerResult *speaker.IdentifyResult) error {
	log.Debugf("sending LLM request with tools, sessionID: %s, requestEinoMessages: %+v", l.clientState.SessionID, userMessage)
	clientState := l.clientState

	l.einoTools = einoTools

	// assemble history messages and current user message
	requestMessages := l.GetMessages(ctx, userMessage, MaxMessageCount, speakerResult)

	if l.session != nil {
		payload, stop, hookErr := l.session.hookHub.EmitLLMInput(l.session.hookContext(ctx), chathooks.LLMInputData{
			UserMessage:     userMessage,
			RequestMessages: requestMessages,
			Tools:           einoTools,
		})
		if hookErr != nil {
			log.Warnf("LLM_INPUT hook execution failed: %v", hookErr)
		}
		userMessage = payload.UserMessage
		requestMessages = payload.RequestMessages
		einoTools = payload.Tools
		if stop {
			log.Infof("LLM_INPUT hook requested to stop current flow")
			return nil
		}
	}

	clientState.SetStartLlmTs()
	if l.session != nil {
		l.session.TraceLlmStart(ctx, time.Now().UnixMilli())
	}
	clientState.SetStatus(ClientStatusLLMStart)

	// call internal method to handle LLM response, resources managed inside method
	responseSentences, err := l.handleLLMWithContextAndTools(
		ctx,
		requestMessages,
		einoTools,
	)
	if err != nil {
		log.Errorf("send LLM request with tools failed, sessionID: %s, error: %v", l.clientState.SessionID, err)
		return fmt.Errorf("send LLM request with tools failed: %v", err)
	}

	log.Debugf("DoLLmRequest goroutine started - SessionID: %s, context state: %v", l.clientState.SessionID, ctx.Err())

	if isSync {
		// sync processing: resources auto-released in handleLLMWithContextAndTools's defer
		_, err := l.HandleLLMResponseChannelSync(ctx, userMessage, responseSentences, einoTools)
		if err != nil {
			log.Errorf("process LLM response failed, sessionID: %s, error: %v", l.clientState.SessionID, err)
			return err
		}
	} else {
		err = l.HandleLLMResponseChannelAsync(ctx, userMessage, responseSentences)
		if err != nil {
			log.Errorf("process LLM response failed, sessionID: %s, error: %v", l.clientState.SessionID, err)
		}
	}

	log.Debugf("DoLLmRequest finished - SessionID: %s", l.clientState.SessionID)

	return nil
}

// AddMessage adds message to chat history (unified entry, applies to all message types)
func (l *LLMManager) AddMessage(ctx context.Context, msg *schema.Message) error {
	if msg == nil {
		log.Warnf("attempting to add nil message to chat history")
		return fmt.Errorf("message cannot be nil")
	}

	// generate MessageID (use MD5 hash to shorten length, avoid exceeding database varchar(64) limit)
	// original format: {SessionID}-{Role}-{Timestamp}
	rawMessageID := fmt.Sprintf("%s-%s-%d",
		l.clientState.SessionID,
		msg.Role,
		time.Now().UnixMilli())
	// use MD5 hash to generate fixed 32-char hex string
	hash := md5.Sum([]byte(rawMessageID))
	messageID := hex.EncodeToString(hash[:])

	// add to memory synchronously
	l.clientState.AddMessage(msg)

	// Tool role message: save directly, no two-phase save (no audio)
	if msg.Role == schema.Tool {
		eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
			ClientState: l.clientState,
			Msg:         *msg,
			MessageID:   messageID,
			AudioData:   nil, // Tool role has no audio
			AudioSize:   0,
			SampleRate:  0,
			Channels:    0,
			Timestamp:   time.Now(),
			IsUpdate:    false, // one-time save
		})
		return nil
	}

	// User/Assistant role: two-phase save
	// store MessageID in LLMManager for subsequent audio update
	if msg.Role == schema.User || msg.Role == schema.Assistant {
		l.lastMessageIDMu.Lock()
		l.lastMessageID[string(msg.Role)] = messageID
		l.lastMessageIDMu.Unlock()
	}

	// publish event: first phase (text only, no audio)
	eventbus.Get().Publish(eventbus.TopicAddMessage, &eventbus.AddMessageEvent{
		ClientState: l.clientState,
		Msg:         *msg,
		MessageID:   messageID,
		AudioData:   nil, // first phase: no audio
		AudioSize:   0,
		SampleRate:  0,
		Channels:    0,
		Timestamp:   time.Now(),
		IsUpdate:    false, // new message
	})

	return nil
}

// AddLlmMessage keeps backward compatibility, delegates to AddMessage
func (l *LLMManager) AddLlmMessage(ctx context.Context, msg *schema.Message) error {
	return l.AddMessage(ctx, msg)
}

func (l *LLMManager) GetMessages(ctx context.Context, userMessage *schema.Message, count int, speakerResult *speaker.IdentifyResult) []*schema.Message {
	memoryMode := l.clientState.GetMemoryMode()
	includeHistory := memoryMode != MemoryModeNone

	// get context from dialogue; in none mode only allow temp messages from current tool call chain
	messageList := make([]*schema.Message, 0)
	if includeHistory {
		messageList = l.clientState.GetMessages(count)
		if userMessage != nil {
			messageList = trimTrailingUserMessages(messageList)
		}
	} else if toolRoundMessages := toolRoundMessagesFromContext(ctx); len(toolRoundMessages) > 0 {
		messageList = toolRoundMessages
	}

	// build system prompt
	systemPrompt := l.clientState.SystemPrompt
	globalSystemPrompt := strings.TrimSpace(viper.GetString("chat.global_system_prompt"))
	if globalSystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt = globalSystemPrompt + "\n\n" + systemPrompt
		} else {
			systemPrompt = globalSystemPrompt
		}
	}

	// add current time and date info
	now := time.Now()
	systemPrompt += fmt.Sprintf("\ncurrent time and date: %s %s", now.Format("2006-01-02 15:04:05"), now.Format("Monday"))

	if memoryMode == MemoryModeLong && l.clientState.MemoryContext != "" {
		systemPrompt += fmt.Sprintf("\nuser personalized info: \n%s", l.clientState.MemoryContext)
	}

	log.Debugf("speakerResult: %+v, voiceIdentify: %+v", speakerResult, l.clientState.DeviceConfig.VoiceIdentify)

	// integrate speaker recognition result into systemPrompt
	if speakerResult != nil && speakerResult.Identified {
		// match speakerGroup info in userConfig based on speakerResult
		if l.clientState.DeviceConfig.VoiceIdentify != nil {
			// prefer SpeakerName matching (VoiceIdentify's key is speakerGroup.Name)
			if speakerGroupInfo, found := l.clientState.DeviceConfig.VoiceIdentify[speakerResult.SpeakerName]; found {
				// if matching speakerGroup found, integrate description into systemPrompt
				if speakerGroupInfo.Prompt != "" {
					systemPrompt += fmt.Sprintf("\nbased on speaker recognition, identified speaker info: \n%s", speakerGroupInfo.Prompt)
				}
			}
		}
	}

	//search memory
	if memoryMode == MemoryModeLong && l.clientState.MemoryProvider != nil && userMessage != nil {
		memoryContext, err := l.clientState.MemoryProvider.Search(ctx, l.clientState.GetDeviceIDOrAgentID(), userMessage.Content, 10, 180)
		if err != nil {
			log.Errorf("search memory failed: %v", err)
		}
		log.Debugf("search memory succeeded, input content: %s, memory content: %s", userMessage.Content, memoryContext)
		if memoryContext != "" {
			systemPrompt += fmt.Sprintf("\nrelated historical info: \n%s", memoryContext)
		}
	}

	systemPrompt += buildKnowledgeSearchRoutingPolicy(l.clientState.DeviceConfig.KnowledgeBases)

	retMessage := make([]*schema.Message, 0)
	retMessage = append(retMessage, &schema.Message{
		Role:    schema.System,
		Content: systemPrompt,
	})
	// filter out empty assistant messages to avoid 400 error when sending to LLM API
	// empty assistant message (Content empty and ToolCalls empty) causes API error
	for _, msg := range messageList {
		if msg != nil && msg.Role == schema.Assistant && msg.Content == "" && len(msg.ToolCalls) == 0 {
			log.Debugf("filtered out empty assistant message to avoid LLM API 400 error")
			continue
		}
		msgCopy := cloneMessageForRequest(msg)
		if isInterruptedMessage(msgCopy) {
			msgCopy.Content = decorateInterruptedContent(msgCopy.Content)
		}
		retMessage = append(retMessage, msgCopy)
	}
	if userMessage != nil {
		// check if last message in retMessage is already the same user message, avoid duplicate addition
		shouldAdd := true
		if len(retMessage) > 0 {
			lastMsg := retMessage[len(retMessage)-1]
			if lastMsg.Role == schema.User && lastMsg.Content == userMessage.Content {
				// last message is already the same user message, skip adding
				shouldAdd = false
				//log.Debugf("last message is already the same user message, skip duplicate addition: %s", userMessage.Content)
			}
		}
		if shouldAdd {
			retMessage = append(retMessage, userMessage)
		}
	}
	return retMessage
}

func buildKnowledgeSearchRoutingPolicy(knowledgeBases []config_types.KnowledgeBaseRef) string {
	if len(knowledgeBases) == 0 {
		return ""
	}

	availableKBs := make([]string, 0, len(knowledgeBases))
	for _, kb := range knowledgeBases {
		if strings.EqualFold(strings.TrimSpace(kb.Status), "inactive") {
			continue
		}
		if strings.TrimSpace(kb.ExternalKBID) == "" {
			continue
		}
		name := strings.TrimSpace(kb.Name)
		if name == "" {
			name = strings.TrimSpace(kb.ExternalKBID)
		}
		if name == "" {
			continue
		}
		if kb.ID == 0 {
			continue
		}
		desc := strings.TrimSpace(kb.Description)
		if desc == "" {
			desc = "no description"
		}
		availableKBs = append(availableKBs, fmt.Sprintf("%d: name=%s; description=%s", kb.ID, name, desc))
		if len(availableKBs) >= 8 {
			break
		}
	}
	if len(availableKBs) == 0 {
		return ""
	}

	return fmt.Sprintf(
		"\nKnowledge base search rules (tool: search_knowledge):\navailable knowledge bases(id:name+description): %s\n"+
			"1. Trigger conditions: user asks about facts, processes, parameters, rules, definitions, clauses, comparisons, or other questions requiring document evidence, or user explicitly requests \"answer from knowledge base/document\".\n"+
			"2. Non-trigger conditions: casual greetings, emotional companionship, pure creative writing, pure subjective suggestions.\n"+
			"3. Invocation: at most 1 call per turn, query should extract core keywords from the user's question, top_k defaults to 5; if a specific knowledge base can be determined, pass knowledge_base_ids (can be multiple).\n"+
			"4. Selection rules: only pass knowledge base IDs semantically most relevant to the current question; if unsure, do not pass knowledge_base_ids.\n"+
			"5. Insufficient info: if evidence is insufficient, do not fabricate, ask the user to provide more specific keywords.\n"+
			"6. Output requirements: when answering, do not mention \"knowledge base\", \"retrieval\", \"MCP\", \"tool call\", \"hit results\" or other source/process information.",
		strings.Join(availableKBs, ", "),
	)
}

func trimTrailingUserMessages(messages []*schema.Message) []*schema.Message {
	end := len(messages)
	for end > 0 {
		msg := messages[end-1]
		if msg == nil || msg.Role != schema.User {
			break
		}
		end--
	}
	return messages[:end]
}

func isInterruptedMessage(msg *schema.Message) bool {
	if msg == nil || msg.Extra == nil {
		return false
	}
	v, ok := msg.Extra[interruptExtraKey]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func decorateInterruptedContent(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	if strings.HasSuffix(content, interruptContentSuffix) {
		return content
	}
	return content + interruptContentSuffix
}

func cloneMessagesForRequest(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		cloned = append(cloned, cloneMessageForRequest(msg))
	}

	return cloned
}

func toolRoundMessagesFromContext(ctx context.Context) []*schema.Message {
	if ctx == nil {
		return nil
	}

	messages, ok := ctx.Value(toolRoundMessagesKey).([]*schema.Message)
	if !ok || len(messages) == 0 {
		return nil
	}

	return cloneMessagesForRequest(messages)
}

func ttsTurnTrackerFromContext(ctx context.Context) *ttsTurnTracker {
	if ctx == nil {
		return nil
	}

	tracker, ok := ctx.Value(ttsTurnTrackerKey).(*ttsTurnTracker)
	if !ok {
		return nil
	}

	return tracker
}

func ensureTTSTurnTrackerInContext(ctx context.Context) context.Context {
	if ttsTurnTrackerFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, ttsTurnTrackerKey, newTTSTurnTracker())
}

func waitForTTSTurnDrainIfRoot(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if nest, ok := ctx.Value("nest").(int); ok && nest > 1 {
		return nil
	}

	tracker := ttsTurnTrackerFromContext(ctx)
	if tracker == nil {
		return nil
	}

	return tracker.Wait(ctx)
}

func appendToolRoundMessagesToContext(ctx context.Context, messages []*schema.Message) context.Context {
	if len(messages) == 0 {
		return ctx
	}

	combined := toolRoundMessagesFromContext(ctx)
	combined = append(combined, cloneMessagesForRequest(messages)...)
	if len(combined) == 0 {
		return ctx
	}

	return context.WithValue(ctx, toolRoundMessagesKey, combined)
}

func cloneMessageForRequest(msg *schema.Message) *schema.Message {
	if msg == nil {
		return nil
	}
	msgCopy := *msg

	if msg.ToolCalls != nil {
		msgCopy.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
	}
	if msg.MultiContent != nil {
		msgCopy.MultiContent = append([]schema.ChatMessagePart(nil), msg.MultiContent...)
	}
	if msg.Extra != nil {
		msgCopy.Extra = make(map[string]any, len(msg.Extra))
		for k, v := range msg.Extra {
			msgCopy.Extra[k] = v
		}
	}
	if msg.ResponseMeta != nil {
		respMetaCopy := *msg.ResponseMeta
		msgCopy.ResponseMeta = &respMetaCopy
	}

	return &msgCopy
}
