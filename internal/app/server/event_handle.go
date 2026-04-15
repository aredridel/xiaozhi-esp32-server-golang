package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	log "xiaozhi-esp32-server-golang/logger"
)

// EventWrapper eventpackage装器，used forunifiedprocessnoat the same timetypeofevent
type EventWrapper struct {
	Topic string      // topicname
	Data  interface{} // eventdata
}

// TopicHandler 通usetopicprocess器interface
type TopicHandler interface {
	// Process processevent
	Process(ctx context.Context, data interface{}) error
	// GetRoutingKey getused forhashrouteofkey（通常yesDeviceIDorSessionID）
	GetRoutingKey(data interface{}) string
}

// UnifiedWorkerPool unifiedofworkerpool，canprocessmultipletopic
type UnifiedWorkerPool struct {
	workers   []chan *EventWrapper
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handlers  map[string]TopicHandler // topic -> handler map
	workerNum int
	mu        sync.RWMutex // protected handlers map
}

// NewUnifiedWorkerPool createunifiedofworkerpool
func NewUnifiedWorkerPool(workerNum int) *UnifiedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &UnifiedWorkerPool{
		workers:   make([]chan *EventWrapper, workerNum),
		ctx:       ctx,
		cancel:    cancel,
		handlers:  make(map[string]TopicHandler),
		workerNum: workerNum,
	}

	// initialize每个workerofchannelandstartgoroutine
	for i := 0; i < workerNum; i++ {
		pool.workers[i] = make(chan *EventWrapper, 100) // buffer100个message
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}

	log.Infof("UnifiedWorkerPoolinitializecomplete，start %d 个worker goroutine（可processmultipletopic）", workerNum)
	return pool
}

// RegisterHandler registertopicprocess器
func (p *UnifiedWorkerPool) RegisterHandler(topic string, handler TopicHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	log.Infof("UnifiedWorkerPool: registertopicprocess器 [%s]", topic)
}

// workerLoop 每个workerofprocessloop（保证sequentialprocess）
func (p *UnifiedWorkerPool) workerLoop(index int) {
	defer p.wg.Done()
	defer log.Infof("UnifiedWorkerPool worker %d exit", index)

	ch := p.workers[index]
	for {
		select {
		case <-p.ctx.Done():
			// cleanupchannelinofremainingmessage
			for {
				select {
				case event := <-ch:
					if event != nil {
						p.processEvent(event)
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
				p.processEvent(event)
			}
		}
	}
}

// processEvent processevent（according totopicminute发tocorrespondinghandler）
func (p *UnifiedWorkerPool) processEvent(event *EventWrapper) {
	p.mu.RLock()
	handler, exists := p.handlers[event.Topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] noregisterprocess器，skip", event.Topic)
		return
	}

	if err := handler.Process(context.Background(), event.Data); err != nil {
		log.Errorf("UnifiedWorkerPool: topic [%s] processfailed: %v", event.Topic, err)
	}
}

// Route routeeventtocorrespondingworker（usehashminute布）
func (p *UnifiedWorkerPool) Route(topic string, data interface{}) bool {
	p.mu.RLock()
	handler, exists := p.handlers[topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] noregisterprocess器，no法route", topic)
		return false
	}

	// getroutekey
	key := handler.GetRoutingKey(data)
	if key == "" {
		log.Warnf("UnifiedWorkerPool: topic [%s] routekeyisempty，no法routemessage", topic)
		return false
	}

	// calculatehashvalue，routetocorrespondingworker
	workerIndex := p.hashKey(key)

	// createeventpackage装器
	event := &EventWrapper{
		Topic: topic,
		Data:  data,
	}

	// non-blockingsendtocorrespondingworker channel
	select {
	case p.workers[workerIndex] <- event:
		return true
	default:
		log.Warnf("UnifiedWorkerPool: topic [%s] worker %d ofchannelalreadyfull，discardmessage, key: %s",
			topic, workerIndex, key)
		return false
	}
}

// hashKey calculatekeyofhashvalue，returnworkerindex
func (p *UnifiedWorkerPool) hashKey(key string) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()
	return int(hash) % p.workerNum
}

// Close closeworkerpool
func (p *UnifiedWorkerPool) Close() {
	p.cancel()
	p.wg.Wait()

	// closeallworker channels
	for i := 0; i < p.workerNum; i++ {
		close(p.workers[i])
	}

	log.Info("UnifiedWorkerPoolalreadyclose")
}

type EventHandle struct {
	// unifiedofworkerpool，canprocessmultipletopic
	workerPool *UnifiedWorkerPool
	// App reference，used forget ChatManager
	app *App
}

// SessionEndHandler SessionEndeventprocess器
type SessionEndHandler struct{}

func (h *SessionEndHandler) Process(ctx context.Context, data interface{}) error {
	clientState, ok := data.(*ClientState)
	if !ok || clientState == nil {
		return nil
	}

	if clientState.MemoryProvider == nil {
		return nil
	}
	if clientState.GetMemoryMode() != MemoryModeLong {
		return nil
	}

	log.Debugf("HandleSessionEnd: deviceId: %s", clientState.DeviceID)

	// willmessage加tolong期记忆bodyin
	err := clientState.MemoryProvider.Flush(
		clientState.Ctx,
		clientState.GetDeviceIDOrAgentID())
	if err != nil {
		log.Errorf("flush message to memory provider failed: %v", err)
		return err
	}
	return nil
}

func (h *SessionEndHandler) GetRoutingKey(data interface{}) string {
	clientState, ok := data.(*ClientState)
	if !ok || clientState == nil {
		return ""
	}
	return clientState.DeviceID
}

// ExitChatHandler ExitChateventprocess器
type ExitChatHandler struct {
	eventHandle *EventHandle // 持have EventHandle reference，used foraccess App
}

func (h *ExitChatHandler) Process(ctx context.Context, data interface{}) error {
	event, ok := data.(*eventbus.ExitChatEvent)
	if !ok || event == nil {
		return nil
	}

	clientState := event.ClientState
	if clientState == nil {
		return nil
	}

	log.Debugf("processexitchatevent: device_id: %s, reason: %s, trigger: %s, user_text: %s",
		clientState.DeviceID, event.Reason, event.TriggerType, event.UserText)

	// according to deviceId get ChatManager
	if h.eventHandle == nil || h.eventHandle.app == nil {
		log.Warnf("EventHandle or App not initialized，no法get ChatManager")
		return nil
	}

	chatManager, exists := h.eventHandle.app.GetChatManager(clientState.DeviceID)
	if !exists {
		log.Warnf("not找todevice %s of ChatManager，mayalreadyclose", clientState.DeviceID)
		return nil
	}

	// get ChatSession andexecuteexitchatlogical
	session := chatManager.GetSession()
	if session == nil {
		log.Warnf("ChatManager of Session isempty，device: %s", clientState.DeviceID)
		return nil
	}

	// executeexitchatlogical（sendgoodbyephraseandclosesession）
	session.DoExitChat()

	return nil
}

func (h *ExitChatHandler) GetRoutingKey(data interface{}) string {
	event, ok := data.(*eventbus.ExitChatEvent)
	if !ok || event == nil || event.ClientState == nil {
		return ""
	}
	return event.ClientState.DeviceID
}

func NewEventHandle(app *App) (*EventHandle, error) {
	// createunifiedofworkerpool
	workerPool := NewUnifiedWorkerPool(MessageWorkerNum)

	// registerSessionEndprocess器
	sessionEndHandler := &SessionEndHandler{}
	workerPool.RegisterHandler(eventbus.TopicSessionEnd, sessionEndHandler)

	handle := &EventHandle{
		workerPool: workerPool,
		app:        app,
	}

	// registerExitChatprocess器
	exitChatHandler := &ExitChatHandler{
		eventHandle: handle,
	}
	workerPool.RegisterHandler(eventbus.TopicExitChat, exitChatHandler)

	log.Infof("EventHandleinitializecomplete（useunifiedworkerpoolprocessmultipletopic，Redisprocessalreadymigrate至MessageWorker）")
	return handle, nil
}

func (s *EventHandle) Start() error {
	// subscribeSessionEndevent
	go s.HandleSessionEnd()

	// subscribeExitChatevent
	go s.HandleExitChat()

	// at这incanaddothertopicofsubscribe
	// go s.HandleDeviceOnline()

	return nil
}

// HandleSessionEnd subscribeandprocessSessionEndevent
func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}

		// routetounifiedofworkerpool
		s.workerPool.Route(eventbus.TopicSessionEnd, clientState)
	})
	return nil
}

// HandleExitChat subscribeandprocessExitChatevent
func (s *EventHandle) HandleExitChat() error {
	eventbus.Get().Subscribe(eventbus.TopicExitChat, func(event *eventbus.ExitChatEvent) {
		if event == nil {
			log.Warnf("HandleExitChat: event is nil, skipping")
			return
		}

		// routetounifiedofworkerpool
		s.workerPool.Route(eventbus.TopicExitChat, event)
	})
	return nil
}

// RegisterTopic register新topicofprocess器（便捷method）
func (s *EventHandle) RegisterTopic(topic string, handler TopicHandler) {
	s.workerPool.RegisterHandler(topic, handler)
}

// Close closeEventHandle，优雅closeworkerpool
func (s *EventHandle) Close() {
	if s.workerPool != nil {
		s.workerPool.Close()
	}
	log.Info("EventHandlealreadyclose")
}
