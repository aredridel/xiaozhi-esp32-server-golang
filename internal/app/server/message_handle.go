package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"runtime"
	"sync"
	"time"

	data_client "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/data/history"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/memory/llm_memory"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

var (
	// MessageWorkerNum message process worker count (based on CPU core count, unified config, used for Redis+History process)
	// must be power of 2 for hash distribution
	MessageWorkerNum = getMessageWorkerNum()
)

// getMessageWorkerNum according to CPU core count calculate worker count, round up to nearest power of 2
// minimum is 4, maximum is 64
func getMessageWorkerNum() int {
	cpuNum := runtime.NumCPU()

	// minimum is 4, maximum is 64
	if cpuNum < 4 {
		return 4
	}
	if cpuNum > 64 {
		return 64
	}

	// round up to nearest power of 2
	power := 1
	for power < cpuNum {
		power <<= 1
	}
	return power
}

// MessageWorker message processor
// use fixed count of goroutine pool, route by SessionID hash value, guarantee sequential processing of same session messages
// unified process Redis, MemoryProvider and History messages
type MessageWorker struct {
	client  *history.HistoryClient
	workers []chan *eventbus.AddMessageEvent // each worker's channel
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMessageWorker create message processor
func NewMessageWorker(cfg history.HistoryClientConfig) *MessageWorker {
	client := history.NewHistoryClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	worker := &MessageWorker{
		client:  client,
		workers: make([]chan *eventbus.AddMessageEvent, MessageWorkerNum),
		ctx:     ctx,
		cancel:  cancel,
	}

	// initialize each worker's channel and start goroutine
	for i := 0; i < MessageWorkerNum; i++ {
		worker.workers[i] = make(chan *eventbus.AddMessageEvent, 100) // buffer 100 messages
		worker.wg.Add(1)
		go worker.workerLoop(i)
	}

	worker.subscribeEvents()
	log.Infof("MessageWorker initialize complete, start %d worker goroutines (unified process Redis+MemoryProvider+History)", MessageWorkerNum)
	return worker
}

// workerLoop each worker's process loop (guarantees sequential processing)
func (w *MessageWorker) workerLoop(index int) {
	defer w.wg.Done()
	defer log.Infof("MessageWorker worker %d exit", index)

	ch := w.workers[index]
	for {
		select {
		case <-w.ctx.Done():
			// cleanup remaining messages in channel
			for {
				select {
				case event := <-ch:
					if event != nil {
						w.processMessage(event)
					}
				default:
					return
				}
			}
		case event, ok := <-ch:
			if !ok {
				// channel closed
				return
			}
			if event != nil {
				w.processMessage(event)
			}
		}
	}
}

// processMessage process message (execute sequentially in worker goroutine)
// unified process Redis, MemoryProvider and History, guarantee sequential processing of same device/session messages
func (w *MessageWorker) processMessage(event *eventbus.AddMessageEvent) {
	// 1. process History (all messages)
	// use independent context, not affected by event.ClientState.Ctx, ensure history message save not affected by conversation cancel
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// judge if new or update
	if event.IsUpdate {
		// nth stage: update audio
		w.updateMessageAudio(ctx, event)
	} else {
		// first stage: save text message (include Redis process)
		w.saveMessageText(ctx, event)
	}

	// 2. process MemoryProvider (only when !IsUpdate, independent of redis and manager)
	// long-term memory body (memobase/mem0) process, regardless of redis or manager scenario are needed
	if !event.IsUpdate {
		w.processMemoryProvider(event)
	}
}

// processMemoryProvider process long-term memory body (memobase/mem0)
// independent of redis and manager, regardless of redis or manager scenario are needed to process
func (w *MessageWorker) processMemoryProvider(event *eventbus.AddMessageEvent) {
	clientState := event.ClientState
	if clientState.MemoryProvider == nil {
		return
	}
	if clientState.GetMemoryMode() != data_client.MemoryModeLong {
		return
	}

	err := clientState.MemoryProvider.AddMessage(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID(),
		event.Msg)
	if err != nil {
		log.Errorf("add message to memory provider failed: %v", err)
	}
}

// hashSessionID calculate SessionID hash value, return worker index
func (w *MessageWorker) hashSessionID(sessionID string) int {
	if sessionID == "" {
		return 0 // if SessionID is empty, use nth worker
	}

	// use FNV-1a hash function
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	hash := h.Sum32()
	return int(hash) % MessageWorkerNum
}

// subscribeEvents subscribe EventBus event
func (w *MessageWorker) subscribeEvents() {
	bus := eventbus.Get()
	// subscribe unified message add event (and EventHandle listen to same Topic)
	bus.Subscribe(eventbus.TopicAddMessage, w.handleAddMessage)
}

// handleAddMessage unified process message add event (route to corresponding worker)
func (w *MessageWorker) handleAddMessage(event *eventbus.AddMessageEvent) {
	if event == nil || event.ClientState == nil {
		return
	}

	// determine key used for routing: priority use SessionID, if empty then use DeviceID
	key := event.ClientState.SessionID
	if key == "" {
		key = event.ClientState.DeviceID
	}
	if key == "" {
		log.Warnf("SessionID and DeviceID are empty, cannot route message")
		return
	}

	// calculate hash value, route to corresponding worker
	workerIndex := w.hashSessionID(key)

	// non-blocking send to corresponding worker channel
	select {
	case w.workers[workerIndex] <- event:
		// successful send
	default:
		// channel already full, record warn (usually won't occur, because channel has buffer)
		log.Warnf("worker %d channel already full, discard message, session_id: %s, device_id: %s",
			workerIndex, event.ClientState.SessionID, event.ClientState.DeviceID)
	}
}

// saveMessageText save text message (first stage, or one-time save text+audio)
// include Redis process (when config_provider.type is redis)
func (w *MessageWorker) saveMessageText(ctx context.Context, event *eventbus.AddMessageEvent) {
	// process Redis (only when config_provider.type is redis)
	// add to Redis message list (used for LLM context)
	providerType := viper.GetString("config_provider.type")
	if providerType == "redis" {
		clientState := event.ClientState
		llm_memory.Get().AddMessage(
			clientState.Ctx,
			clientState.DeviceID,
			clientState.AgentID,
			event.Msg)
		return
	}

	// determine message role
	var role history.MessageType
	switch event.Msg.Role {
	case schema.User:
		role = history.MessageTypeUser
	case schema.Assistant:
		role = history.MessageTypeAssistant
	case schema.Tool:
		role = history.MessageTypeTool
	case schema.System:
		role = history.MessageTypeSystem
	default:
		log.Warnf("unsupported message role: %s", event.Msg.Role)
		return
	}

	// convert audio format (if exists)
	var audioBase64 string
	var audioFormat string
	var audioSize int

	if len(event.AudioData) > 0 {
		// ASR message: text and audio get at the same time, one-time save
		var wavData []byte
		var err error

		// according to message role select different audio convert method
		if event.Msg.Role == schema.User {
			// User message (ASR): PCM float32 format
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User message only has one element
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant message (TTS): Opus format (theoretically should not be here, because Assistant is two-stage save)
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("audio convert failed, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// degradation process: direct concat all frames
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
			audioFormat = "raw" // degradation process use original format
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
			audioFormat = "wav"
		}
	}

	// build Metadata (only save timestamp)
	metadata := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}

	// prepare tool call relevant field
	var toolCallID string
	var toolCallsJSON *string

	// Tool role: save tool_call_id
	if event.Msg.Role == schema.Tool && event.Msg.ToolCallID != "" {
		toolCallID = event.Msg.ToolCallID
	}

	// Assistant role: save ToolCalls (if has)
	if event.Msg.Role == schema.Assistant && len(event.Msg.ToolCalls) > 0 {
		// serialize ToolCalls as JSON string
		toolCallsBytes, err := json.Marshal(event.Msg.ToolCalls)
		if err != nil {
			log.Warnf("serialize ToolCalls failed, device_id: %s, message_id: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, err)
		} else {
			jsonStr := string(toolCallsBytes)
			toolCallsJSON = &jsonStr
		}
	}

	req := &history.SaveMessageRequest{
		MessageID:     event.MessageID,
		DeviceID:      event.ClientState.DeviceID,
		AgentID:       event.ClientState.AgentID,
		SessionID:     event.ClientState.SessionID,
		Role:          role,
		Content:       event.Msg.Content,
		ToolCallID:    toolCallID,
		ToolCallsJSON: toolCallsJSON,
		AudioData:     audioBase64,
		AudioFormat:   audioFormat,
		AudioSize:     audioSize,
		Metadata:      metadata,
	}

	if err := w.client.SaveMessage(ctx, req); err != nil {
		log.Errorf("save message failed, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}

// updateMessageAudio update message audio (nth stage)
func (w *MessageWorker) updateMessageAudio(ctx context.Context, event *eventbus.AddMessageEvent) {
	// convert audio format
	var audioBase64 string
	var audioSize int

	if len(event.AudioData) > 0 {
		var wavData []byte
		var err error

		// according to message role select different audio convert method
		// User message (ASR): PCM float32 format, use PCMFloat32BytesToWav
		// Assistant message (TTS): Opus format, use OpusFramesToWav
		if event.Msg.Role == schema.User {
			// User message: PCM float32 format
			// event.AudioData is [][]byte, but User message only has one element (complete PCM float32 byte array)
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User message only has one element
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant message: Opus format
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("audio convert failed, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// degradation process: direct concat all frames
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
		}
	}

	// build update request
	req := &history.UpdateMessageAudioRequest{
		MessageID:   event.MessageID,
		AudioData:   audioBase64,
		AudioFormat: "wav",
		AudioSize:   audioSize,
		Metadata: map[string]interface{}{
			"tts_duration": event.TTSDuration,
		},
	}

	// call update interface
	if err := w.client.UpdateMessageAudio(ctx, req); err != nil {
		log.Errorf("update message audio failed, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}
