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
	// MessageWorkerNum messageprocessworkercount（基于CPUcorecount，unifiedconfig，used forRedis+Historyprocess）
	// 必须yes2of幂times以便hashminute布
	MessageWorkerNum = getMessageWorkerNum()
)

// getMessageWorkerNum according toCPUcorecountcalculateworkercount，toup取to最近of2of幂times
// minimumis4，maximumis64
func getMessageWorkerNum() int {
	cpuNum := runtime.NumCPU()

	// minimumis4，maximumis64
	if cpuNum < 4 {
		return 4
	}
	if cpuNum > 64 {
		return 64
	}

	// toup取to最近of2of幂times
	power := 1
	for power < cpuNum {
		power <<= 1
	}
	return power
}

// MessageWorker messageprocess器
// usefixedcountofgoroutinepool，按SessionIDofhashvalueroute，保证at the same timeasessionofmessagesequentialprocess
// unifiedprocessRedis、MemoryProviderandHistorymessage
type MessageWorker struct {
	client  *history.HistoryClient
	workers []chan *eventbus.AddMessageEvent // 每个workerofchannel
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMessageWorker createmessageprocess器
func NewMessageWorker(cfg history.HistoryClientConfig) *MessageWorker {
	client := history.NewHistoryClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	worker := &MessageWorker{
		client:  client,
		workers: make([]chan *eventbus.AddMessageEvent, MessageWorkerNum),
		ctx:     ctx,
		cancel:  cancel,
	}

	// initialize每个workerofchannelandstartgoroutine
	for i := 0; i < MessageWorkerNum; i++ {
		worker.workers[i] = make(chan *eventbus.AddMessageEvent, 100) // buffer100个message
		worker.wg.Add(1)
		go worker.workerLoop(i)
	}

	worker.subscribeEvents()
	log.Infof("MessageWorkerinitializecomplete，start %d 个worker goroutine（unifiedprocessRedis+MemoryProvider+History）", MessageWorkerNum)
	return worker
}

// workerLoop 每个workerofprocessloop（保证sequentialprocess）
func (w *MessageWorker) workerLoop(index int) {
	defer w.wg.Done()
	defer log.Infof("MessageWorker worker %d exit", index)

	ch := w.workers[index]
	for {
		select {
		case <-w.ctx.Done():
			// cleanupchannelinofremainingmessage
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

// processMessage processmessage（atworker goroutineinsequentialexecute）
// unifiedprocessRedis、MemoryProviderandHistory，保证at the same timeadevice/sessionofmessagesequentialprocess
func (w *MessageWorker) processMessage(event *eventbus.AddMessageEvent) {
	// 1. process History（allmessage）
	// useindependentof context，no受 event.ClientState.Ctx 影响，ensurehistorymessagesaveno受toconversationcancel影响
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// judgeyes新增oryesupdate
	if event.IsUpdate {
		// nth二阶段：updateaudio
		w.updateMessageAudio(ctx, event)
	} else {
		// first阶段：savetextmessage（includeRedisprocess）
		w.saveMessageText(ctx, event)
	}

	// 2. process MemoryProvider（only!IsUpdatewhen，independent于redisandmanager）
	// long期记忆body（memobase/mem0）process，no管yesredisoryesmanagerscenarioareneed
	if !event.IsUpdate {
		w.processMemoryProvider(event)
	}
}

// processMemoryProvider processlong期记忆body（memobase/mem0）
// independent于redisandmanager，no管yesredisoryesmanagerscenarioareneedprocess
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

// hashSessionID calculateSessionIDofhashvalue，returnworkerindex
func (w *MessageWorker) hashSessionID(sessionID string) int {
	if sessionID == "" {
		return 0 // ifSessionIDisempty，usenthaworker
	}

	// useFNV-1ahashfunction
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	hash := h.Sum32()
	return int(hash) % MessageWorkerNum
}

// subscribeEvents subscribeEventBusevent
func (w *MessageWorker) subscribeEvents() {
	bus := eventbus.Get()
	// subscribeunifiedofmessageaddevent（and EventHandle listenat the same timea Topic）
	bus.Subscribe(eventbus.TopicAddMessage, w.handleAddMessage)
}

// handleAddMessage unifiedprocessmessageaddevent（routetocorrespondingworker）
func (w *MessageWorker) handleAddMessage(event *eventbus.AddMessageEvent) {
	if event == nil || event.ClientState == nil {
		return
	}

	// determineused forrouteofkey：priorityuseSessionID，ifisemptythenuseDeviceID
	key := event.ClientState.SessionID
	if key == "" {
		key = event.ClientState.DeviceID
	}
	if key == "" {
		log.Warnf("SessionIDandDeviceIDareisempty，no法routemessage")
		return
	}

	// calculatehashvalue，routetocorrespondingworker
	workerIndex := w.hashSessionID(key)

	// non-blockingsendtocorrespondingworker channel
	select {
	case w.workers[workerIndex] <- event:
		// successfulsend
	default:
		// channelalreadyfull，recordwarn（通常nowilloccur，becauseischannelhavebuffer）
		log.Warnf("worker %d ofchannelalreadyfull，discardmessage, session_id: %s, device_id: %s",
			workerIndex, event.ClientState.SessionID, event.ClientState.DeviceID)
	}
}

// saveMessageText savetextmessage（first阶段，oratimes性savetext+audio）
// includeRedisprocess（whenconfig_provider.typeisrediswhen）
func (w *MessageWorker) saveMessageText(ctx context.Context, event *eventbus.AddMessageEvent) {
	// process Redis（onlywhenconfig_provider.typeisrediswhen）
	// addto Redis messagelist（used forLLM context）
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

	// determinemessagerole
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
		log.Warnf("unsupportedofmessagerole: %s", event.Msg.Role)
		return
	}

	// convertaudioformat（if存at）
	var audioBase64 string
	var audioFormat string
	var audioSize int

	if len(event.AudioData) > 0 {
		// ASR message：textandaudioat the same timewhenget，atimes性save
		var wavData []byte
		var err error

		// according tomessageroleselectnoat the same timeofaudioconvertmethod
		if event.Msg.Role == schema.User {
			// User message（ASR）：PCM float32 format
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User messageonlyhaveaelement
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant message（TTS）：Opus format（理论upnoshouldat这in，becauseis Assistant yes两阶段save）
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("audioconvertfailed, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// degradationprocess：directconcatallframe
			var fallbackData []byte
			for _, frame := range event.AudioData {
				fallbackData = append(fallbackData, frame...)
			}
			audioBase64 = base64.StdEncoding.EncodeToString(fallbackData)
			audioSize = event.AudioSize
			audioFormat = "raw" // degradationprocessuseoriginal format
		} else {
			audioBase64 = base64.StdEncoding.EncodeToString(wavData)
			audioSize = len(wavData)
			audioFormat = "wav"
		}
	}

	// build Metadata（onlysavetimestamp）
	metadata := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}

	// preparetoolcallrelevantfield
	var toolCallID string
	var toolCallsJSON *string

	// Tool role：save tool_call_id
	if event.Msg.Role == schema.Tool && event.Msg.ToolCallID != "" {
		toolCallID = event.Msg.ToolCallID
	}

	// Assistant role：save ToolCalls（ifhave）
	if event.Msg.Role == schema.Assistant && len(event.Msg.ToolCalls) > 0 {
		// serialize ToolCalls is JSON charstring
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
		log.Errorf("savemessagefailed, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}

// updateMessageAudio updatemessageaudio（nth二阶段）
func (w *MessageWorker) updateMessageAudio(ctx context.Context, event *eventbus.AddMessageEvent) {
	// convertaudioformat
	var audioBase64 string
	var audioSize int

	if len(event.AudioData) > 0 {
		var wavData []byte
		var err error

		// according tomessageroleselectnoat the same timeofaudioconvertmethod
		// User message（ASR）：PCM float32 format，use PCMFloat32BytesToWav
		// Assistant message（TTS）：Opus format，use OpusFramesToWav
		if event.Msg.Role == schema.User {
			// User message：PCM float32 format
			// event.AudioData yes [][]byte，but User messageonlyhaveaelement（完bodyof PCM float32 bytearray）
			if len(event.AudioData) > 0 {
				wavData, err = util.PCMFloat32BytesToWav(
					event.AudioData[0], // User messageonlyhaveaelement
					event.SampleRate,
					event.Channels)
			}
		} else {
			// Assistant message：Opus format
			wavData, err = util.OpusFramesToWav(
				event.AudioData,
				event.SampleRate,
				event.Channels)
		}

		if err != nil {
			log.Errorf("audioconvertfailed, device_id: %s, message_id: %s, role: %s, error: %v",
				event.ClientState.DeviceID, event.MessageID, event.Msg.Role, err)
			// degradationprocess：directconcatallframe
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

	// buildupdaterequest
	req := &history.UpdateMessageAudioRequest{
		MessageID:   event.MessageID,
		AudioData:   audioBase64,
		AudioFormat: "wav",
		AudioSize:   audioSize,
		Metadata: map[string]interface{}{
			"tts_duration": event.TTSDuration,
		},
	}

	// callupdateinterface
	if err := w.client.UpdateMessageAudio(ctx, req); err != nil {
		log.Errorf("updatemessageaudio failed, device_id: %s, message_id: %s, error: %v",
			event.ClientState.DeviceID, event.MessageID, err)
	}
}
