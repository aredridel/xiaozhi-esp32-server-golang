package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	log "xiaozhi-esp32-server-golang/logger"
)

// EventWrapper event wrapper, used for unified processing of different types of events
type EventWrapper struct {
	Topic string      // topicname
	Data  interface{} // eventdata
}

// TopicHandler common topic processor interface
type TopicHandler interface {
	// Process process event
	Process(ctx context.Context, data interface{}) error
	// GetRoutingKey get key used for hash routing (usually DeviceID or SessionID)
	GetRoutingKey(data interface{}) string
}

// UnifiedWorkerPool unified worker pool, can process multiple topics
type UnifiedWorkerPool struct {
	workers   []chan *EventWrapper
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handlers  map[string]TopicHandler // topic -> handler map
	workerNum int
	mu        sync.RWMutex // protected handlers map
}

// NewUnifiedWorkerPool create unified worker pool
func NewUnifiedWorkerPool(workerNum int) *UnifiedWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &UnifiedWorkerPool{
		workers:   make([]chan *EventWrapper, workerNum),
		ctx:       ctx,
		cancel:    cancel,
		handlers:  make(map[string]TopicHandler),
		workerNum: workerNum,
	}

	// initialize each worker's channel and start goroutine
	for i := 0; i < workerNum; i++ {
		pool.workers[i] = make(chan *EventWrapper, 100) // buffer 100 messages
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}

	log.Infof("UnifiedWorkerPool initialize complete, start %d worker goroutines (can process multiple topics)", workerNum)
	return pool
}

// RegisterHandler register topic processor
func (p *UnifiedWorkerPool) RegisterHandler(topic string, handler TopicHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	log.Infof("UnifiedWorkerPool: register topic processor [%s]", topic)
}

// workerLoop each worker's process loop (guarantees sequential processing)
func (p *UnifiedWorkerPool) workerLoop(index int) {
	defer p.wg.Done()
	defer log.Infof("UnifiedWorkerPool worker %d exit", index)

	ch := p.workers[index]
	for {
		select {
		case <-p.ctx.Done():
			// cleanup remaining messages in channel
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

// processEvent process event (dispatch to corresponding handler according to topic)
func (p *UnifiedWorkerPool) processEvent(event *EventWrapper) {
	p.mu.RLock()
	handler, exists := p.handlers[event.Topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] no registered processor, skip", event.Topic)
		return
	}

	if err := handler.Process(context.Background(), event.Data); err != nil {
		log.Errorf("UnifiedWorkerPool: topic [%s] process failed: %v", event.Topic, err)
	}
}

// Route route event to corresponding worker (use hash distribution)
func (p *UnifiedWorkerPool) Route(topic string, data interface{}) bool {
	p.mu.RLock()
	handler, exists := p.handlers[topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] no registered processor, cannot route", topic)
		return false
	}

	// get route key
	key := handler.GetRoutingKey(data)
	if key == "" {
		log.Warnf("UnifiedWorkerPool: topic [%s] route key is empty, cannot route message", topic)
		return false
	}

	// calculate hash value, route to corresponding worker
	workerIndex := p.hashKey(key)

	// create event wrapper
	event := &EventWrapper{
		Topic: topic,
		Data:  data,
	}

	// non-blocking send to corresponding worker channel
	select {
	case p.workers[workerIndex] <- event:
		return true
	default:
		log.Warnf("UnifiedWorkerPool: topic [%s] worker %d channel already full, discard message, key: %s",
			topic, workerIndex, key)
		return false
	}
}

// hashKey calculate key's hash value, return worker index
func (p *UnifiedWorkerPool) hashKey(key string) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()
	return int(hash) % p.workerNum
}

// Close close worker pool
func (p *UnifiedWorkerPool) Close() {
	p.cancel()
	p.wg.Wait()

	// closeallworker channels
	for i := 0; i < p.workerNum; i++ {
		close(p.workers[i])
	}

	log.Info("UnifiedWorkerPool already closed")
}

type EventHandle struct {
	// unified worker pool, can process multiple topics
	workerPool *UnifiedWorkerPool
	// App reference, used for getting ChatManager
	app *App
}

// SessionEndHandler SessionEnd event processor
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

	// add message to long-term memory body
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

// ExitChatHandler ExitChat event processor
type ExitChatHandler struct {
	eventHandle *EventHandle // hold EventHandle reference, used for accessing App
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
		log.Warnf("EventHandle or App not initialized, cannot get ChatManager")
		return nil
	}

	chatManager, exists := h.eventHandle.app.GetChatManager(clientState.DeviceID)
	if !exists {
		log.Warnf("device %s ChatManager not found, may already closed", clientState.DeviceID)
		return nil
	}

	// get ChatSession and execute exit chat logic
	session := chatManager.GetSession()
	if session == nil {
		log.Warnf("ChatManager Session is empty, device: %s", clientState.DeviceID)
		return nil
	}

	// execute exit chat logic (send goodbye phrase and close session)
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
	// create unified worker pool
	workerPool := NewUnifiedWorkerPool(MessageWorkerNum)

	// register SessionEnd processor
	sessionEndHandler := &SessionEndHandler{}
	workerPool.RegisterHandler(eventbus.TopicSessionEnd, sessionEndHandler)

	handle := &EventHandle{
		workerPool: workerPool,
		app:        app,
	}

	// register ExitChat processor
	exitChatHandler := &ExitChatHandler{
		eventHandle: handle,
	}
	workerPool.RegisterHandler(eventbus.TopicExitChat, exitChatHandler)

	log.Infof("EventHandle initialize complete (use unified worker pool process multiple topics, Redis process already migrated to MessageWorker)")
	return handle, nil
}

func (s *EventHandle) Start() error {
	// subscribeSessionEndevent
	go s.HandleSessionEnd()

	// subscribeExitChatevent
	go s.HandleExitChat()

	// here can add other topic subscriptions
	// go s.HandleDeviceOnline()

	return nil
}

// HandleSessionEnd subscribe and process SessionEnd event
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

// HandleExitChat subscribe and process ExitChat event
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

// RegisterTopic register new topic processor (convenience method)
func (s *EventHandle) RegisterTopic(topic string, handler TopicHandler) {
	s.workerPool.RegisterHandler(topic, handler)
}

// Close close EventHandle, gracefully close worker pool
func (s *EventHandle) Close() {
	if s.workerPool != nil {
		s.workerPool.Close()
	}
	log.Info("EventHandle already closed")
}
