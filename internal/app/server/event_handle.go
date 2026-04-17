package server

import (
	"context"
	"hash/fnv"
	"sync"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	log "xiaozhi-esp32-server-golang/logger"
)

// EventWrapper event wrapper for unified handling of different event types
type EventWrapper struct {
	Topic string      // topic name
	Data  interface{} // event data
}

// TopicHandler generic topic handler interface
type TopicHandler interface {
	// Process process event
	Process(ctx context.Context, data interface{}) error
	// GetRoutingKey get key for hash routing (usually DeviceID or SessionID)
	GetRoutingKey(data interface{}) string
}

// UnifiedWorkerPool unified worker pool that can handle multiple topics
type UnifiedWorkerPool struct {
	workers   []chan *EventWrapper
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	handlers  map[string]TopicHandler // topic -> handler mapping
	workerNum int
	mu        sync.RWMutex // protect handlers map
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

	// Initialize each worker's channel and start goroutine
	for i := 0; i < workerNum; i++ {
		pool.workers[i] = make(chan *EventWrapper, 100) // buffer 100 messages
		pool.wg.Add(1)
		go pool.workerLoop(i)
	}

	log.Infof("UnifiedWorkerPool initialized, started %d worker goroutines (can handle multiple topics)", workerNum)
	return pool
}

// RegisterHandler register topic handler
func (p *UnifiedWorkerPool) RegisterHandler(topic string, handler TopicHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topic] = handler
	log.Infof("UnifiedWorkerPool: registered topic handler [%s]", topic)
}

// workerLoop each worker's processing loop (guarantees sequential processing)
func (p *UnifiedWorkerPool) workerLoop(index int) {
	defer p.wg.Done()
	defer log.Infof("UnifiedWorkerPool worker %d exited", index)

	ch := p.workers[index]
	for {
		select {
		case <-p.ctx.Done():
			// Drain remaining messages from channel
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

// processEvent process event (dispatch to corresponding handler by topic)
func (p *UnifiedWorkerPool) processEvent(event *EventWrapper) {
	p.mu.RLock()
	handler, exists := p.handlers[event.Topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] has no registered handler, skipping", event.Topic)
		return
	}

	if err := handler.Process(context.Background(), event.Data); err != nil {
		log.Errorf("UnifiedWorkerPool: topic [%s] processing failed: %v", event.Topic, err)
	}
}

// Route route event to corresponding worker (using hash distribution)
func (p *UnifiedWorkerPool) Route(topic string, data interface{}) bool {
	p.mu.RLock()
	handler, exists := p.handlers[topic]
	p.mu.RUnlock()

	if !exists {
		log.Warnf("UnifiedWorkerPool: topic [%s] has no registered handler, cannot route", topic)
		return false
	}

	// Get routing key
	key := handler.GetRoutingKey(data)
	if key == "" {
		log.Warnf("UnifiedWorkerPool: topic [%s] routing key is empty, cannot route message", topic)
		return false
	}

	// Calculate hash value, route to corresponding worker
	workerIndex := p.hashKey(key)

	// Create event wrapper
	event := &EventWrapper{
		Topic: topic,
		Data:  data,
	}

	// Non-blocking send to corresponding worker channel
	select {
	case p.workers[workerIndex] <- event:
		return true
	default:
		log.Warnf("UnifiedWorkerPool: topic [%s] worker %d channel full, message dropped, key: %s",
			topic, workerIndex, key)
		return false
	}
}

// hashKey calculate hash of key, return worker index
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

	// Close all worker channels
	for i := 0; i < p.workerNum; i++ {
		close(p.workers[i])
	}

	log.Info("UnifiedWorkerPool closed")
}

type EventHandle struct {
	// Unified worker pool that can handle multiple topics
	workerPool *UnifiedWorkerPool
	// App reference, used to get ChatManager
	app *App
}

// SessionEndHandler SessionEnd event handler
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

	// Flush messages to long-term memory
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

// ExitChatHandler ExitChat event handler
type ExitChatHandler struct {
	eventHandle *EventHandle // holds EventHandle reference to access App
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

	log.Debugf("handling exit chat event: device_id: %s, reason: %s, trigger: %s, user_text: %s",
		clientState.DeviceID, event.Reason, event.TriggerType, event.UserText)

	// Get ChatManager by deviceId
	if h.eventHandle == nil || h.eventHandle.app == nil {
		log.Warnf("EventHandle or App not initialized, unable to get ChatManager")
		return nil
	}

	chatManager, exists := h.eventHandle.app.GetChatManager(clientState.DeviceID)
	if !exists {
		log.Warnf("ChatManager for device %s not found, may already be closed", clientState.DeviceID)
		return nil
	}

	return chatManager.ExitChat()
}

func (h *ExitChatHandler) GetRoutingKey(data interface{}) string {
	event, ok := data.(*eventbus.ExitChatEvent)
	if !ok || event == nil || event.ClientState == nil {
		return ""
	}
	return event.ClientState.DeviceID
}

func NewEventHandle(app *App) (*EventHandle, error) {
	// Create unified worker pool
	workerPool := NewUnifiedWorkerPool(MessageWorkerNum)

	// Register SessionEnd handler
	sessionEndHandler := &SessionEndHandler{}
	workerPool.RegisterHandler(eventbus.TopicSessionEnd, sessionEndHandler)

	handle := &EventHandle{
		workerPool: workerPool,
		app:        app,
	}

	// Register ExitChat handler
	exitChatHandler := &ExitChatHandler{
		eventHandle: handle,
	}
	workerPool.RegisterHandler(eventbus.TopicExitChat, exitChatHandler)

	log.Infof("EventHandle initialized (using unified worker pool to handle multiple topics, Redis processing migrated to MessageWorker)")
	return handle, nil
}

func (s *EventHandle) Start() error {
	// Subscribe to SessionEnd event
	go s.HandleSessionEnd()

	// Subscribe to ExitChat event
	go s.HandleExitChat()

	// Other topic subscriptions can be added here
	// go s.HandleDeviceOnline()

	return nil
}

// HandleSessionEnd subscribe and handle SessionEnd events
func (s *EventHandle) HandleSessionEnd() error {
	eventbus.Get().Subscribe(eventbus.TopicSessionEnd, func(clientState *ClientState) {
		if clientState == nil {
			log.Warnf("HandleSessionEnd: clientState is nil, skipping")
			return
		}

		// Route to unified worker pool
		s.workerPool.Route(eventbus.TopicSessionEnd, clientState)
	})
	return nil
}

// HandleExitChat subscribe and handle ExitChat events
func (s *EventHandle) HandleExitChat() error {
	eventbus.Get().Subscribe(eventbus.TopicExitChat, func(event *eventbus.ExitChatEvent) {
		if event == nil {
			log.Warnf("HandleExitChat: event is nil, skipping")
			return
		}

		// Route to unified worker pool
		s.workerPool.Route(eventbus.TopicExitChat, event)
	})
	return nil
}

// RegisterTopic register handler for a new topic (convenience method)
func (s *EventHandle) RegisterTopic(topic string, handler TopicHandler) {
	s.workerPool.RegisterHandler(topic, handler)
}

// Close close EventHandle, gracefully shutdown worker pool
func (s *EventHandle) Close() {
	if s.workerPool != nil {
		s.workerPool.Close()
	}
	log.Info("EventHandle closed")
}
