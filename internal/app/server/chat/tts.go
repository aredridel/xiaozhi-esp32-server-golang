package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	chathooks "xiaozhi-esp32-server-golang/internal/domain/chat/hooks"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/domain/tts"
	ttsstream "xiaozhi-esp32-server-golang/internal/domain/tts/streaming"
	"xiaozhi-esp32-server-golang/internal/pool"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

// session-level global audio queue element type constant
const (
	AudioQueueKindFrame         = 0
	AudioQueueKindSentenceStart = 1
	AudioQueueKindSentenceEnd   = 2
	AudioQueueKindTtsStart      = 3
	AudioQueueKindTtsStop       = 4
	AudioQueueKindMediaFrame    = 5
)

// AudioQueueElem session-level audio queue element, compatible with TTS/media audio frame and sentence_start/sentence_end, tts_start/tts_stop.
type AudioQueueElem struct {
	Kind       int    // AudioQueueKindFrame / MediaFrame / SentenceStart / SentenceEnd / TtsStart / TtsStop
	Data       []byte // Kind==Frame or MediaFrame used, enqueue after copy
	Text       string // SentenceStart/SentenceEnd used
	Err        error  // SentenceEnd optional, indicates this segment error
	IsStart    bool   // SentenceStart: whether it is first package (used for counting)
	Generation uint64 // generation identifier, old generation element will be discarded after interrupt
	OnStart    func()
	OnEnd      func(error)
	OnError    func(error)
}

type delayedSentenceTask struct {
	Elem      AudioQueueElem
	ExecuteAt time.Time
}

type interruptRequest struct {
	done chan struct{}
}

// SessionAudioQueueCap session-level audio queue capacity, large enough to absorb prefetch and avoid blocking
const SessionAudioQueueCap = 150

type TTSQueueItem struct {
	ctx         context.Context
	llmResponse llm_common.LLMResponseStruct        // single pattern use
	StreamChan  <-chan llm_common.LLMResponseStruct // streaming pattern: non-nil when priority read from this channel
	enqueueSeq  uint64
	generation  uint64
	metricCycle uint64
	onStartFunc func()
	onEndFunc   func(err error)
}

type ttsMetricState struct {
	cycleID          uint64
	pendingItems     int
	activeRequests   int
	turnEndRequested bool
	started          bool
	firstAudio       bool
	ttsStopped       bool
	turnEnded        bool
}

// TTSManager responsible for TTS related processing
// can extend fields as needed
// currently no state, but can be extended later

type TTSManagerOption func(*TTSManager)

type TTSManager struct {
	clientState               *ClientState
	session                   *ChatSession
	serverTransport           *ServerTransport
	ttsQueue                  *util.Queue[TTSQueueItem]
	sessionAudioQueue         chan AudioQueueElem // session-level global audio queue, compatible with frame and control message
	delayedSentenceQueue      chan delayedSentenceTask
	delayedSentenceReadyQueue chan AudioQueueElem
	interruptCh               chan interruptRequest // interrupt signal: after receiving runSenderLoop clear sessionAudioQueue and continue
	audioGeneration           atomic.Uint64         // session-level audio generation: interrupt when increment, old generation element will be discarded by send goroutine
	audioInterruptMu          sync.RWMutex
	audioInterruptCh          chan struct{}
	ttsActive                 atomic.Bool // whether there is a TTS segment that has started but not ended
	senderLoopActive          atomic.Bool
	senderLoopDone            chan struct{} // runSenderLoop exit when close, for synchronization interrupt quick return in close path

	ttsQueueSeq   atomic.Uint64
	droppedTTSSeq atomic.Uint64

	mediaPlaybackMu     sync.RWMutex
	mediaPlaybackActive bool
	mediaPlaybackWaitCh chan struct{}

	interruptStopMu          sync.Mutex
	interruptStopPending     bool
	interruptStopSendTtsStop bool
	interruptStopErr         error

	// chat history audio cache: continuously accumulate multiple TTS audio segments (Opus frame array)
	audioHistoryBuffer [][]byte
	audioMutex         sync.Mutex

	// dual-stream TTS internal StreamChan: by handleTextResponse create at IsStart, close at IsEnd
	dualStreamChan  chan llm_common.LLMResponseStruct
	dualStreamDone  chan struct{} // dual-stream used for isSync wait: StreamChan corresponding onEndFunc signal
	dualStreamMu    sync.Mutex
	dualStreamEpoch atomic.Uint64

	ttsMetricMu    sync.Mutex
	ttsMetricState ttsMetricState
}

// NewTTSManager only accept WithClientState
func NewTTSManager(clientState *ClientState, serverTransport *ServerTransport, session *ChatSession, opts ...TTSManagerOption) *TTSManager {
	t := &TTSManager{
		clientState:               clientState,
		session:                   session,
		serverTransport:           serverTransport,
		ttsQueue:                  util.NewQueue[TTSQueueItem](10),
		sessionAudioQueue:         make(chan AudioQueueElem, SessionAudioQueueCap),
		delayedSentenceQueue:      make(chan delayedSentenceTask, SessionAudioQueueCap),
		delayedSentenceReadyQueue: make(chan AudioQueueElem, SessionAudioQueueCap),
		interruptCh:               make(chan interruptRequest, 1),
		senderLoopDone:            make(chan struct{}),
		audioInterruptCh:          make(chan struct{}),
	}
	for _, opt := range opts {
		opt(t)
	}
	t.audioGeneration.Store(1)
	return t
}

// start TTS queue consumer goroutine and unified send goroutine (session-level global audio queue)
func (t *TTSManager) Start(ctx context.Context) {
	go t.runDelayedSentenceLoop(ctx)
	go t.runSenderLoop(ctx)
	t.processTTSQueue(ctx)
}

// runSenderLoop only send goroutine: take element from sessionAudioQueue distribute by type, flow control centralized here; exit only when ctx cancel; SessionCtx cancel or receive TurnAbort when clear queue and continue
func (t *TTSManager) runSenderLoop(ctx context.Context) {
	t.senderLoopActive.Store(true)
	defer func() {
		t.senderLoopActive.Store(false)
		close(t.senderLoopDone)
	}()

	frameDuration := time.Duration(t.clientState.OutputAudioFormat.FrameDuration) * time.Millisecond
	cacheFrameCount := 120 / t.clientState.OutputAudioFormat.FrameDuration
	totalFrames := 0
	currentSentenceFrames := 0
	playbackTail := time.Time{}

	handleDelayedSentence := func(elem AudioQueueElem) {
		if elem.Generation != t.currentAudioGeneration() {
			if elem.OnEnd != nil {
				elem.OnEnd(context.Canceled)
			}
			return
		}
		switch elem.Kind {
		case AudioQueueKindSentenceStart:
			if elem.OnStart != nil {
				elem.OnStart()
			}
			if elem.Text != "" {
				if err := t.serverTransport.SendSentenceStart(elem.Text); err != nil {
					log.Errorf("send TTS text failed: %s, %v", elem.Text, err)
					if elem.OnError != nil {
						elem.OnError(err)
					}
					if elem.OnEnd != nil {
						elem.OnEnd(err)
					}
				}
			}
		case AudioQueueKindSentenceEnd:
			callbackErr := elem.Err
			if elem.Text != "" {
				if err := t.serverTransport.SendSentenceEnd(elem.Text); err != nil {
					log.Errorf("send TTS text failed: %s, %v", elem.Text, err)
					if elem.OnError != nil {
						elem.OnError(err)
					}
					if callbackErr == nil {
						callbackErr = err
					}
				}
			}
			currentSentenceFrames = 0
			if elem.OnEnd != nil {
				elem.OnEnd(callbackErr)
			}
		}
	}

	handleInterrupt := func() {
		t.drainSessionAudioQueue()
		t.drainDelayedSentenceReadyQueue()
		if pending, sendTtsStop, stopErr := t.consumePendingInterruptStop(); pending {
			t.finishTtsStop(t.clientState.Ctx, sendTtsStop, stopErr)
		}
		totalFrames = 0
		currentSentenceFrames = 0
		playbackTail = time.Time{}
		log.Debugf("runSenderLoop interrupt, drained queue and continue")
	}

	for {
		select {
		case elem := <-t.delayedSentenceReadyQueue:
			handleDelayedSentence(elem)
			continue
		default:
		}

		select {
		case <-ctx.Done():
			t.drainSessionAudioQueue()
			t.drainDelayedSentenceReadyQueue()
			t.finishTtsStop(t.clientState.Ctx, true, ctx.Err())
			log.Debugf("runSenderLoop ctx done, drained queue and exit")
			return
		case req := <-t.interruptCh:
			handleInterrupt()
			if req.done != nil {
				close(req.done)
			}
			continue
		case elem := <-t.delayedSentenceReadyQueue:
			handleDelayedSentence(elem)
		case elem, ok := <-t.sessionAudioQueue:
			if !ok {
				return
			}
			if elem.Generation != t.currentAudioGeneration() {
				if elem.OnEnd != nil {
					elem.OnEnd(context.Canceled)
				}
				continue
			}
			switch elem.Kind {
			case AudioQueueKindSentenceStart:
				currentSentenceFrames = 0
				if !t.enqueueDelayedSentenceTask(ctx, elem) && elem.OnEnd != nil {
					elem.OnEnd(ctx.Err())
				}
			case AudioQueueKindFrame, AudioQueueKindMediaFrame:
				now := time.Now()
				if playbackTail.IsZero() || now.After(playbackTail) {
					playbackTail = now
				}
				allowedAhead := time.Duration(cacheFrameCount) * frameDuration
				sendAt := playbackTail.Add(-allowedAhead)
				if now.Before(sendAt) {
					waitResult, interruptReq := t.waitUntilSenderDeadline(ctx, sendAt, handleDelayedSentence)
					switch waitResult {
					case senderWaitContextDone:
						t.drainSessionAudioQueue()
						t.drainDelayedSentenceReadyQueue()
						t.finishTtsStop(t.clientState.Ctx, true, ctx.Err())
						return
					case senderWaitInterrupted:
						handleInterrupt()
						if interruptReq.done != nil {
							close(interruptReq.done)
						}
						continue
					}
					now = time.Now()
					if now.After(playbackTail) {
						playbackTail = now
					}
				}
				if err := t.serverTransport.SendAudio(elem.Data); err != nil {
					audioType := "TTS"
					if elem.Kind == AudioQueueKindMediaFrame {
						audioType = "media"
					}
					log.Errorf("send %s audio failed: len: %d, %v", audioType, len(elem.Data), err)
					if elem.OnError != nil {
						elem.OnError(err)
					}
					continue
				}
				if elem.Kind == AudioQueueKindFrame {
					t.audioMutex.Lock()
					frameCopy := make([]byte, len(elem.Data))
					copy(frameCopy, elem.Data)
					t.audioHistoryBuffer = append(t.audioHistoryBuffer, frameCopy)
					t.audioMutex.Unlock()
				}
				totalFrames++
				currentSentenceFrames++
				playbackTail = playbackTail.Add(frameDuration)
			case AudioQueueKindSentenceEnd:
				if !t.enqueueDelayedSentenceTask(ctx, elem) && elem.OnEnd != nil {
					elem.OnEnd(ctx.Err())
				}
			case AudioQueueKindTtsStart:
				if t.session != nil {
					hookErr := t.session.hookHub.EmitTTSOutputStart(t.session.hookContext(ctx))
					if hookErr != nil {
						log.Warnf("TTS_OUTPUT_START hook execute failed: %v", hookErr)
					}
				}
				t.ttsActive.Store(true)
				if err := t.serverTransport.SendTtsStart(); err != nil {
					log.Errorf("send TtsStart failed: %v", err)
				}
				// new voice segment: reset frame count and playback tail pointer
				totalFrames = 0
				playbackTail = time.Time{}
			case AudioQueueKindTtsStop:
				// wait for current playback tail pointer walk to last frame end then send TtsStop
				if !playbackTail.IsZero() {
					waitResult, interruptReq := t.waitUntilSenderDeadline(ctx, playbackTail, handleDelayedSentence)
					switch waitResult {
					case senderWaitContextDone:
						t.drainSessionAudioQueue()
						t.drainDelayedSentenceReadyQueue()
						t.finishTtsStop(t.clientState.Ctx, true, ctx.Err())
						return
					case senderWaitInterrupted:
						handleInterrupt()
						if interruptReq.done != nil {
							close(interruptReq.done)
						}
						continue
					}
				}
				// fixed 150ms wait, ensure client-side playback complete
				waitResult, interruptReq := t.waitUntilSenderDeadline(ctx, time.Now().Add(150*time.Millisecond), handleDelayedSentence)
				switch waitResult {
				case senderWaitContextDone:
					t.drainSessionAudioQueue()
					t.drainDelayedSentenceReadyQueue()
					t.finishTtsStop(t.clientState.Ctx, true, ctx.Err())
					return
				case senderWaitInterrupted:
					handleInterrupt()
					if interruptReq.done != nil {
						close(interruptReq.done)
					}
					continue
				}
				t.finishTtsStop(t.clientState.Ctx, true, nil)
				playbackTail = time.Time{}
				totalFrames = 0
				currentSentenceFrames = 0
			}
		}
	}
}

// drainSessionAudioQueue ctx cancel when clear queue, discard unsent element
func (t *TTSManager) drainSessionAudioQueue() {
	for {
		select {
		case elem, ok := <-t.sessionAudioQueue:
			if !ok {
				return
			}
			if elem.OnEnd != nil {
				elem.OnEnd(context.Canceled)
			}
		default:
			return
		}
	}
}

func (t *TTSManager) drainDelayedSentenceReadyQueue() {
	for {
		select {
		case elem := <-t.delayedSentenceReadyQueue:
			if elem.OnEnd != nil {
				elem.OnEnd(context.Canceled)
			}
		default:
			return
		}
	}
}

// ClearSessionAudioQueue clear session-level audio queue (can be called by external when ctx cancel)
func (t *TTSManager) ClearSessionAudioQueue() {
	t.drainSessionAudioQueue()
}

func (t *TTSManager) currentAudioGeneration() uint64 {
	return t.audioGeneration.Load()
}

func (t *TTSManager) nextAudioGeneration() uint64 {
	return t.audioGeneration.Add(1)
}

func (t *TTSManager) currentAudioInterruptCh() <-chan struct{} {
	t.audioInterruptMu.RLock()
	defer t.audioInterruptMu.RUnlock()
	return t.audioInterruptCh
}

func (t *TTSManager) rotateAudioInterruptCh() {
	t.audioInterruptMu.Lock()
	defer t.audioInterruptMu.Unlock()

	oldCh := t.audioInterruptCh
	t.audioInterruptCh = make(chan struct{})
	if oldCh == nil {
		return
	}

	select {
	case <-oldCh:
	default:
		close(oldCh)
	}
}

func (t *TTSManager) withAudioInterruptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	interruptCtx, cancel := context.WithCancel(ctx)
	interruptCh := t.currentAudioInterruptCh()
	if interruptCh == nil {
		return interruptCtx, cancel
	}

	go func() {
		select {
		case <-interruptCtx.Done():
		case <-interruptCh:
			cancel()
		}
	}()

	return interruptCtx, cancel
}

func (t *TTSManager) startTtsMetricCycle() uint64 {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	t.ttsMetricState = ttsMetricState{
		cycleID: t.ttsMetricState.cycleID + 1,
	}
	return t.ttsMetricState.cycleID
}

func (t *TTSManager) currentTtsMetricCycle() uint64 {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()
	return t.ttsMetricState.cycleID
}

type ttsMetricCompletion struct {
	err          error
	ttsStopTs    int64
	turnEndTs    int64
	traceTtsStop bool
	traceTurnEnd bool
}

func (t *TTSManager) emitTtsMetricCompletion(ctx context.Context, completion ttsMetricCompletion) {
	if t.session == nil {
		return
	}
	if completion.traceTtsStop {
		t.session.TraceTtsStop(ctx, completion.ttsStopTs, completion.err)
	}
	if completion.traceTurnEnd {
		t.session.TraceTurnEnd(ctx, completion.turnEndTs, completion.err)
	}
}

func (t *TTSManager) registerTtsMetricItem(cycleID uint64) {
	if cycleID == 0 {
		return
	}

	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	if t.ttsMetricState.cycleID != cycleID || t.ttsMetricState.turnEnded {
		return
	}
	t.ttsMetricState.pendingItems++
}

func (t *TTSManager) finishTtsMetricItem(ctx context.Context, cycleID uint64, err error) {
	t.emitTtsMetricCompletion(ctx, t.finishTtsMetricItemLocked(cycleID, err))
}

func (t *TTSManager) finishTtsMetricItemLocked(cycleID uint64, err error) ttsMetricCompletion {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	if t.ttsMetricState.cycleID != cycleID || cycleID == 0 {
		return ttsMetricCompletion{}
	}
	if t.ttsMetricState.pendingItems > 0 {
		t.ttsMetricState.pendingItems--
	}
	return t.maybeFinalizeTtsMetricLocked(err)
}

func (t *TTSManager) markTtsMetricRequestStart(ctx context.Context, cycleID uint64) {
	if cycleID == 0 {
		return
	}

	var startTs int64

	t.ttsMetricMu.Lock()
	if t.ttsMetricState.cycleID == cycleID && !t.ttsMetricState.turnEnded {
		t.ttsMetricState.activeRequests++
		if !t.ttsMetricState.started {
			t.clientState.MarkTtsStart()
			startTs = t.clientState.Statistic.TtsStartTs
			t.ttsMetricState.started = true
		}
	}
	t.ttsMetricMu.Unlock()

	if startTs > 0 && t.session != nil {
		t.session.TraceTtsStart(ctx, startTs)
	}
}

func (t *TTSManager) markTtsMetricFirstAudio(ctx context.Context, cycleID uint64) {
	if cycleID == 0 {
		return
	}

	var firstAudioTs int64

	t.ttsMetricMu.Lock()
	if t.ttsMetricState.cycleID == cycleID && t.ttsMetricState.started && !t.ttsMetricState.firstAudio && !t.ttsMetricState.turnEnded {
		t.clientState.MarkTtsFirstFrame()
		firstAudioTs = t.clientState.Statistic.TtsFirstFrameTs
		t.ttsMetricState.firstAudio = true
	}
	t.ttsMetricMu.Unlock()

	if firstAudioTs > 0 && t.session != nil {
		t.session.TraceTtsFirstFrame(ctx, firstAudioTs)
	}
}

func (t *TTSManager) finishTtsMetricRequest(ctx context.Context, cycleID uint64, err error) {
	t.emitTtsMetricCompletion(ctx, t.finishTtsMetricRequestLocked(cycleID, err))
}

func (t *TTSManager) finishTtsMetricRequestLocked(cycleID uint64, err error) ttsMetricCompletion {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	if t.ttsMetricState.cycleID != cycleID || cycleID == 0 {
		return ttsMetricCompletion{}
	}
	if t.ttsMetricState.activeRequests > 0 {
		t.ttsMetricState.activeRequests--
	}
	return t.maybeFinalizeTtsMetricLocked(err)
}

func (t *TTSManager) requestTurnEndLocked(err error) ttsMetricCompletion {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	if t.ttsMetricState.cycleID == 0 {
		return ttsMetricCompletion{}
	}
	t.ttsMetricState.turnEndRequested = true
	return t.maybeFinalizeTtsMetricLocked(err)
}

func (t *TTSManager) forceStopTtsMetric(ctx context.Context, err error) {
	t.emitTtsMetricCompletion(ctx, t.forceStopTtsMetricLocked(err))
}

func (t *TTSManager) forceStopTtsMetricLocked(err error) ttsMetricCompletion {
	t.ttsMetricMu.Lock()
	defer t.ttsMetricMu.Unlock()

	if t.ttsMetricState.cycleID == 0 || t.ttsMetricState.turnEnded {
		return ttsMetricCompletion{}
	}
	t.ttsMetricState.turnEndRequested = true
	return t.finalizeTtsMetricLocked(err)
}

func (t *TTSManager) maybeFinalizeTtsMetricLocked(err error) ttsMetricCompletion {
	if !t.ttsMetricState.turnEndRequested || t.ttsMetricState.turnEnded {
		return ttsMetricCompletion{}
	}
	if t.ttsMetricState.pendingItems > 0 || t.ttsMetricState.activeRequests > 0 {
		return ttsMetricCompletion{}
	}
	return t.finalizeTtsMetricLocked(err)
}

func (t *TTSManager) finalizeTtsMetricLocked(err error) ttsMetricCompletion {
	if t.ttsMetricState.turnEnded {
		return ttsMetricCompletion{}
	}

	completion := ttsMetricCompletion{
		err:          err,
		traceTurnEnd: true,
	}
	if t.ttsMetricState.started && !t.ttsMetricState.ttsStopped {
		t.clientState.MarkTtsStop()
		completion.ttsStopTs = t.clientState.Statistic.TtsStopTs
		completion.turnEndTs = completion.ttsStopTs
		completion.traceTtsStop = true
		t.ttsMetricState.ttsStopped = true
	} else {
		completion.turnEndTs = time.Now().UnixMilli()
	}
	t.ttsMetricState.turnEnded = true
	return completion
}

func (t *TTSManager) pushTTSQueueItem(item TTSQueueItem) error {
	item.enqueueSeq = t.ttsQueueSeq.Add(1)
	if err := t.ttsQueue.Push(item); err != nil {
		return err
	}
	t.registerTtsMetricItem(item.metricCycle)
	return nil
}

func (t *TTSManager) shouldDropTTSQueueItem(item TTSQueueItem) bool {
	if item.enqueueSeq == 0 {
		return false
	}
	return item.enqueueSeq <= t.droppedTTSSeq.Load()
}

func (t *TTSManager) dismissTTSQueueItem(item TTSQueueItem, err error) {
	if item.onEndFunc != nil {
		item.onEndFunc(err)
	}
	t.finishTtsMetricItem(item.ctx, item.metricCycle, err)
}

func (t *TTSManager) waitForMediaPlaybackRelease(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		t.mediaPlaybackMu.RLock()
		active := t.mediaPlaybackActive
		waitCh := t.mediaPlaybackWaitCh
		t.mediaPlaybackMu.RUnlock()

		if !active || waitCh == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-waitCh:
		}
	}
}

func (t *TTSManager) BeginExclusiveMediaPlayback(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	waitCh := make(chan struct{})

	t.mediaPlaybackMu.Lock()
	if t.mediaPlaybackActive {
		t.mediaPlaybackMu.Unlock()
		return fmt.Errorf("media playback is already in exclusive state")
	}
	t.mediaPlaybackActive = true
	t.mediaPlaybackWaitCh = waitCh
	t.mediaPlaybackMu.Unlock()

	t.ClearTTSQueue()
	// media takeover when only interrupt and clear current TTS, do not immediately send tts_stop.
	// real tts_stop by outer response after media playback complete unified cleanup phase send.
	if err := t.InterruptAndClearQueueSync(ctx); err != nil {
		t.EndExclusiveMediaPlayback()
		return err
	}

	return nil
}

func (t *TTSManager) EndExclusiveMediaPlayback() {
	t.mediaPlaybackMu.Lock()
	waitCh := t.mediaPlaybackWaitCh
	t.mediaPlaybackActive = false
	t.mediaPlaybackWaitCh = nil
	t.mediaPlaybackMu.Unlock()

	if waitCh == nil {
		return
	}

	select {
	case <-waitCh:
	default:
		close(waitCh)
	}
}

func (t *TTSManager) sentenceControlDelay() time.Duration {
	frameDurationMs := t.clientState.OutputAudioFormat.FrameDuration
	if frameDurationMs <= 0 {
		return 0
	}
	cacheFrameCount := 120 / frameDurationMs
	return time.Duration(cacheFrameCount*frameDurationMs) * time.Millisecond
}

func insertDelayedSentenceTask(tasks []delayedSentenceTask, task delayedSentenceTask) []delayedSentenceTask {
	insertAt := len(tasks)
	for insertAt > 0 && task.ExecuteAt.Before(tasks[insertAt-1].ExecuteAt) {
		insertAt--
	}
	tasks = append(tasks, delayedSentenceTask{})
	copy(tasks[insertAt+1:], tasks[insertAt:])
	tasks[insertAt] = task
	return tasks
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (t *TTSManager) enqueueDelayedSentenceTask(ctx context.Context, elem AudioQueueElem) bool {
	task := delayedSentenceTask{
		Elem:      elem,
		ExecuteAt: time.Now().Add(t.sentenceControlDelay()),
	}
	if ctx == nil {
		t.delayedSentenceQueue <- task
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case t.delayedSentenceQueue <- task:
		return true
	}
}

func (t *TTSManager) runDelayedSentenceLoop(ctx context.Context) {
	var (
		pending []delayedSentenceTask
		timer   *time.Timer
		timerCh <-chan time.Time
	)

	resetTimer := func() {
		stopTimer(timer)
		timer = nil
		timerCh = nil
		if len(pending) == 0 {
			return
		}
		waitDuration := time.Until(pending[0].ExecuteAt)
		if waitDuration < 0 {
			waitDuration = 0
		}
		timer = time.NewTimer(waitDuration)
		timerCh = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return
		case task := <-t.delayedSentenceQueue:
			pending = insertDelayedSentenceTask(pending, task)
			resetTimer()
		case <-timerCh:
			timer = nil
			timerCh = nil
			if len(pending) == 0 {
				continue
			}
			task := pending[0]
			pending = pending[1:]
			if task.Elem.Generation != t.currentAudioGeneration() {
				if task.Elem.OnEnd != nil {
					task.Elem.OnEnd(context.Canceled)
				}
				resetTimer()
				continue
			}
			select {
			case <-ctx.Done():
				if task.Elem.OnEnd != nil {
					task.Elem.OnEnd(ctx.Err())
				}
				return
			case t.delayedSentenceReadyQueue <- task.Elem:
			}
			resetTimer()
		}
	}
}

type senderWaitResult int

const (
	senderWaitReached senderWaitResult = iota
	senderWaitContextDone
	senderWaitInterrupted
)

func (t *TTSManager) waitUntilSenderDeadline(ctx context.Context, deadline time.Time, handleDelayed func(AudioQueueElem)) (senderWaitResult, interruptRequest) {
	for {
		now := time.Now()
		if !now.Before(deadline) {
			return senderWaitReached, interruptRequest{}
		}

		timer := time.NewTimer(deadline.Sub(now))
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return senderWaitContextDone, interruptRequest{}
		case req := <-t.interruptCh:
			stopTimer(timer)
			return senderWaitInterrupted, req
		case elem := <-t.delayedSentenceReadyQueue:
			stopTimer(timer)
			handleDelayed(elem)
		case <-timer.C:
			return senderWaitReached, interruptRequest{}
		}
	}
}

func (t *TTSManager) enqueueSessionElem(ctx context.Context, generation uint64, elem AudioQueueElem) bool {
	elem.Generation = generation
	if ctx == nil {
		t.sessionAudioQueue <- elem
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case t.sessionAudioQueue <- elem:
		return true
	}
}

// InterruptAndClearQueue trigger interrupt: notify runSenderLoop clear sessionAudioQueue after continue running (non-blocking)
func (t *TTSManager) InterruptAndClearQueue() {
	t.nextAudioGeneration()
	t.rotateAudioInterruptCh()
	if !t.senderLoopActive.Load() {
		return
	}
	select {
	case t.interruptCh <- interruptRequest{}:
	default:
	}
}

// InterruptAndStop used for scenarios requiring immediate end of current TTS.
// it only registers pending close state, real stop and metric close by runSenderLoop at clear queue after unified send.
func (t *TTSManager) InterruptAndStop(ctx context.Context, sendTtsStop bool, stopErr error) {
	t.recordPendingInterruptStop(sendTtsStop, stopErr)
	t.InterruptAndClearQueue()
	t.finishPendingInterruptStopIfSenderLoopExited(ctx)
}

// InterruptAndStopSync trigger synchronization interrupt, while keeping TtsStop/trace/hook only go through unified cleanup of runSenderLoop.
func (t *TTSManager) InterruptAndStopSync(ctx context.Context, sendTtsStop bool, stopErr error) error {
	t.recordPendingInterruptStop(sendTtsStop, stopErr)
	if err := t.InterruptAndClearQueueSync(ctx); err != nil {
		t.finishPendingInterruptStopIfSenderLoopExited(ctx)
		return err
	}
	t.finishPendingInterruptStopIfSenderLoopExited(ctx)
	return nil
}

func (t *TTSManager) recordPendingInterruptStop(sendTtsStop bool, stopErr error) {
	t.interruptStopMu.Lock()
	defer t.interruptStopMu.Unlock()

	if t.interruptStopPending {
		t.interruptStopSendTtsStop = t.interruptStopSendTtsStop || sendTtsStop
		if t.interruptStopErr == nil {
			t.interruptStopErr = stopErr
		}
	} else {
		t.interruptStopPending = true
		t.interruptStopSendTtsStop = sendTtsStop
		t.interruptStopErr = stopErr
	}
}

func (t *TTSManager) consumePendingInterruptStop() (bool, bool, error) {
	t.interruptStopMu.Lock()
	defer t.interruptStopMu.Unlock()

	if !t.interruptStopPending {
		return false, false, nil
	}

	sendTtsStop := t.interruptStopSendTtsStop
	stopErr := t.interruptStopErr
	t.interruptStopPending = false
	t.interruptStopSendTtsStop = false
	t.interruptStopErr = nil
	return true, sendTtsStop, stopErr
}

func (t *TTSManager) finishPendingInterruptStopIfSenderLoopExited(ctx context.Context) {
	if t.senderLoopActive.Load() {
		return
	}
	if pending, sendTtsStop, stopErr := t.consumePendingInterruptStop(); pending {
		t.finishTtsStop(ctx, sendTtsStop, stopErr)
	}
}

// InterruptAndClearQueueSync trigger interrupt and wait return after runSenderLoop completes clearing queue.
func (t *TTSManager) InterruptAndClearQueueSync(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	t.nextAudioGeneration()
	t.rotateAudioInterruptCh()
	if !t.senderLoopActive.Load() {
		return nil
	}

	req := interruptRequest{
		done: make(chan struct{}),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.senderLoopDone:
		return nil
	case t.interruptCh <- req:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.senderLoopDone:
		return nil
	case <-req.done:
		return nil
	}
}

func (t *TTSManager) finishTtsStop(ctx context.Context, sendTtsStop bool, stopErr error) bool {
	if !t.ttsActive.Swap(false) {
		return false
	}

	if ctx == nil {
		ctx = context.Background()
	}

	shouldSendTtsStop := sendTtsStop
	if shouldSendTtsStop && t.clientState.IsRealTime() {
		shouldSendTtsStop = false
		log.Debugf("realtime pattern skips sending TtsStop: stop_err=%v", stopErr)
	}

	if shouldSendTtsStop {
		if err := t.serverTransport.SendTtsStop(); err != nil {
			if stopErr == nil {
				stopErr = err
			}
			log.Errorf("send TtsStop failed: %v", err)
		}
	}
	if t.session != nil {
		hookErr := t.session.hookHub.EmitTTSOutputStop(t.session.hookContext(ctx), chathooks.TTSOutputStopData{Err: stopErr})
		if hookErr != nil {
			log.Warnf("TTS_OUTPUT_STOP hook execute failed: %v", hookErr)
		}
	}

	t.forceStopTtsMetric(ctx, stopErr)

	return true
}

func (t *TTSManager) FinishTtsWithoutProtocolStop(ctx context.Context, stopErr error) bool {
	return t.finishTtsStop(ctx, false, stopErr)
}

// EnqueueTtsStart deliver to session-level audio queue TtsStart, unifiedly sent by runSenderLoop; when queue is full block until enqueued or ctx.Done
func (t *TTSManager) EnqueueTtsStart(ctx context.Context) {
	t.startTtsMetricCycle()
	t.enqueueSessionElem(ctx, t.currentAudioGeneration(), AudioQueueElem{Kind: AudioQueueKindTtsStart})
}

// RequestTurnEnd mark current round logical output end; actual turn_end will at all TTS audio received after send.
func (t *TTSManager) RequestTurnEnd(ctx context.Context, err error) {
	t.emitTtsMetricCompletion(ctx, t.requestTurnEndLocked(err))
}

// EnqueueTtsStop deliver to session-level audio queue TtsStop, unifiedly sent by runSenderLoop; when queue is full block until enqueued or ctx.Done
func (t *TTSManager) EnqueueTtsStop(ctx context.Context) {
	t.enqueueSessionElem(ctx, t.currentAudioGeneration(), AudioQueueElem{Kind: AudioQueueKindTtsStop})
}

func (t *TTSManager) enqueueSessionElemWithError(ctx context.Context, generation uint64, elem AudioQueueElem) error {
	if t.enqueueSessionElem(ctx, generation, elem) {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return context.Canceled
}

func (t *TTSManager) EnqueueMediaSentenceStart(ctx context.Context, text string, onError func(error)) error {
	return t.enqueueSessionElemWithError(ctx, t.currentAudioGeneration(), AudioQueueElem{
		Kind:    AudioQueueKindSentenceStart,
		Text:    text,
		OnError: onError,
	})
}

func (t *TTSManager) EnqueueMediaSentenceEnd(ctx context.Context, text string, onError func(error), onEnd func(error)) error {
	err := t.enqueueSessionElemWithError(ctx, t.currentAudioGeneration(), AudioQueueElem{
		Kind:    AudioQueueKindSentenceEnd,
		Text:    text,
		OnEnd:   onEnd,
		OnError: onError,
	})
	if err != nil && onEnd != nil {
		onEnd(err)
	}
	return err
}

func (t *TTSManager) EnqueueMediaFrame(ctx context.Context, frame []byte, onError func(error)) error {
	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)
	return t.enqueueSessionElemWithError(ctx, t.currentAudioGeneration(), AudioQueueElem{
		Kind:    AudioQueueKindMediaFrame,
		Data:    frameCopy,
		OnError: onError,
	})
}

func (t *TTSManager) processTTSQueue(ctx context.Context) {
	for {
		item, err := t.ttsQueue.Pop(ctx, 0) // blocking
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			continue
		}

		if t.shouldDropTTSQueueItem(item) {
			t.dismissTTSQueueItem(item, context.Canceled)
			continue
		}

		itemCtx, cancel := t.withAudioInterruptContext(item.ctx)
		item.ctx = itemCtx
		waitErr := t.waitForMediaPlaybackRelease(item.ctx)
		if waitErr != nil {
			cancel()
			t.dismissTTSQueueItem(item, waitErr)
			continue
		}
		if t.shouldDropTTSQueueItem(item) {
			cancel()
			t.dismissTTSQueueItem(item, context.Canceled)
			continue
		}

		itemErr := error(nil)
		if item.StreamChan != nil {
			log.Debugf("processTTSQueue start, stream mode")
			itemErr = t.handleStreamTts(item)
			t.finishTtsMetricItem(item.ctx, item.metricCycle, itemErr)
			cancel()
			log.Debugf("processTTSQueue end, stream mode")
			continue
		}

		// non-streaming: by handleTts generate and push SentenceStart -> Frame... -> SentenceEnd
		log.Debugf("processTTSQueue start, text: %s", item.llmResponse.Text)
		itemErr = t.handleTts(item.ctx, item.generation, item.metricCycle, item.llmResponse, item.onStartFunc, item.onEndFunc)
		t.finishTtsMetricItem(item.ctx, item.metricCycle, itemErr)
		cancel()
		log.Debugf("processTTSQueue end, text: %s (pushed)", item.llmResponse.Text)
	}
}

func (t *TTSManager) ClearTTSQueue() {
	t.droppedTTSSeq.Store(t.ttsQueueSeq.Load())
	t.dualStreamEpoch.Add(1)

	t.dualStreamMu.Lock()
	dualStreamChan := t.dualStreamChan
	t.dualStreamChan = nil
	t.dualStreamDone = nil
	t.dualStreamMu.Unlock()
	safeCloseLLMResponseStream(dualStreamChan)

	drained := t.ttsQueue.ClearAndDrain()
	for _, item := range drained {
		t.dismissTTSQueueItem(item, context.Canceled)
	}
}

// handleTts single TTS: generate and to sessionAudioQueue push SentenceStart -> Frame... -> SentenceEnd
func (t *TTSManager) handleTts(ctx context.Context, generation uint64, metricCycle uint64, llmResponse llm_common.LLMResponseStruct, onStartFunc func(), onEndFunc func(error)) error {
	if strings.TrimSpace(llmResponse.Text) == "" {
		if onEndFunc != nil {
			onEndFunc(nil)
		}
		return nil
	}
	outChan, release, genErr := t.generateTtsOnly(ctx, metricCycle, llmResponse)
	if genErr != nil {
		log.Errorf("handleTts gen err, text: %s, err: %v", llmResponse.Text, genErr)
		if onEndFunc != nil {
			onEndFunc(genErr)
		}
		return genErr
	}
	if outChan == nil {
		if release != nil {
			release()
		}
		if onEndFunc != nil {
			onEndFunc(nil)
		}
		return nil
	}
	requestActive := true
	firstAudioReported := false
	finishRequest := func(err error) {
		if requestActive {
			t.finishTtsMetricRequest(ctx, metricCycle, err)
			requestActive = false
		}
	}
	if !t.enqueueSessionElem(ctx, generation, AudioQueueElem{
		Kind:    AudioQueueKindSentenceStart,
		Text:    llmResponse.Text,
		IsStart: llmResponse.IsStart,
		OnStart: onStartFunc,
	}) {
		if release != nil {
			release()
		}
		finishRequest(ctx.Err())
		if onEndFunc != nil {
			onEndFunc(ctx.Err())
		}
		return ctx.Err()
	}
	for {
		select {
		case <-ctx.Done():
			if release != nil {
				release()
			}
			finishRequest(ctx.Err())
			if onEndFunc != nil {
				onEndFunc(ctx.Err())
			}
			return ctx.Err()
		case frame, ok := <-outChan:
			if !ok {
				if release != nil {
					release()
				}
				finishRequest(nil)
				if !t.enqueueSessionElem(ctx, generation, AudioQueueElem{
					Kind:  AudioQueueKindSentenceEnd,
					Text:  llmResponse.Text,
					OnEnd: onEndFunc,
				}) && onEndFunc != nil {
					onEndFunc(ctx.Err())
				}
				return ctx.Err()
			}
			if !firstAudioReported {
				t.markTtsMetricFirstAudio(ctx, metricCycle)
				firstAudioReported = true
			}
			frameCopy := make([]byte, len(frame))
			copy(frameCopy, frame)
			if !t.enqueueSessionElem(ctx, generation, AudioQueueElem{Kind: AudioQueueKindFrame, Data: frameCopy}) {
				if release != nil {
					release()
				}
				finishRequest(ctx.Err())
				if onEndFunc != nil {
					onEndFunc(ctx.Err())
				}
				return ctx.Err()
			}
		}
	}
}

const ttsSyncWaitTimeout = 30 * time.Second

// signalDone to already buffer of done send a complete signal, only first call takes effect
func signalDone(done chan<- struct{}) {
	select {
	case done <- struct{}{}:
	default:
	}
}

func safeCloseLLMResponseStream(ch chan llm_common.LLMResponseStruct) {
	if ch == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	close(ch)
}

func sendLLMResponseToDualStream(ctx context.Context, ch chan llm_common.LLMResponseStruct, llmResponse llm_common.LLMResponseStruct) (err error) {
	if ch == nil {
		return nil
	}

	defer func() {
		if recover() != nil {
			err = nil
		}
	}()

	select {
	case ch <- llmResponse:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("TTS process context canceled")
	}
}

func chainTTSOnEndFuncs(funcs ...func(error)) func(error) {
	filtered := make([]func(error), 0, len(funcs))
	for _, fn := range funcs {
		if fn != nil {
			filtered = append(filtered, fn)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	return func(err error) {
		for _, fn := range filtered {
			fn(err)
		}
	}
}

// waitForSync synchronization wait complete signal, support ctx cancel and timeout
func (t *TTSManager) waitForSync(ctx context.Context, done <-chan struct{}) error {
	timer := time.NewTimer(ttsSyncWaitTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("TTS process context canceled")
	case <-timer.C:
		return fmt.Errorf("TTS process timeout")
	}
}

// handleTextResponse process text respond (asynchronization TTS enqueue). caller calls multiple times by sentence, internal according to SupportsDualStream() automatic decide:
//   - unsupported dual-stream: every time Push a single TTSQueueItem (consistent with original logic).
//   - support dual-stream: IsStart when create internal StreamChan and Push a streaming item, after continue call write this channel, IsEnd when close.
func (t *TTSManager) handleTextResponse(ctx context.Context, llmResponse llm_common.LLMResponseStruct, isSync bool) error {
	return t.handleTextResponseWithHooks(ctx, llmResponse, isSync, nil)
}

func (t *TTSManager) handleTextResponseWithHooks(ctx context.Context, llmResponse llm_common.LLMResponseStruct, isSync bool, onTTSItemEnqueued func() func(error)) error {
	hasText := strings.TrimSpace(llmResponse.Text) != ""
	if !hasText && !llmResponse.IsEnd && !llmResponse.IsStart {
		return nil
	}

	// TTS_INPUT hook
	if t.session != nil {
		payload, stop, hookErr := t.session.hookHub.EmitTTSInput(t.session.hookContext(ctx), chathooks.TTSInputData{Text: llmResponse.Text, IsStart: llmResponse.IsStart, IsEnd: llmResponse.IsEnd})
		if hookErr != nil {
			log.Warnf("TTS_INPUT hook execute failed: %v", hookErr)
		}
		llmResponse.Text = payload.Text
		llmResponse.IsStart = payload.IsStart
		llmResponse.IsEnd = payload.IsEnd
		if stop {
			log.Infof("TTS_INPUT hook request stop current flow")
			return nil
		}
	}

	// re-inspect hasText, because hook may modify text
	hasText = strings.TrimSpace(llmResponse.Text) != ""

	if !t.SupportsDualStream() {
		if !hasText {
			return nil
		}
		gen := t.currentAudioGeneration()
		metricCycle := t.currentTtsMetricCycle()
		var done chan struct{}
		onEndFunc := func(error) {}
		if onTTSItemEnqueued != nil {
			onEndFunc = onTTSItemEnqueued()
		}
		if isSync {
			done = make(chan struct{}, 1)
			onEndFunc = chainTTSOnEndFuncs(onEndFunc, func(error) { signalDone(done) })
		}
		if err := t.pushTTSQueueItem(TTSQueueItem{
			ctx:         ctx,
			llmResponse: llmResponse,
			generation:  gen,
			metricCycle: metricCycle,
			onEndFunc:   onEndFunc,
		}); err != nil {
			if onEndFunc != nil {
				onEndFunc(err)
			}
			return err
		}
		if isSync {
			return t.waitForSync(ctx, done)
		}
		return nil
	}

	// dual-stream pattern
	var streamChan chan llm_common.LLMResponseStruct
	if llmResponse.IsStart {
		streamEpoch := t.dualStreamEpoch.Load()
		t.dualStreamMu.Lock()
		oldStreamChan := t.dualStreamChan
		t.dualStreamChan = nil
		t.dualStreamDone = nil
		t.dualStreamMu.Unlock()
		safeCloseLLMResponseStream(oldStreamChan)

		streamChan = make(chan llm_common.LLMResponseStruct, 16)
		var done chan struct{}
		var onEndFunc func(error)
		if onTTSItemEnqueued != nil {
			onEndFunc = onTTSItemEnqueued()
		}
		if isSync {
			done = make(chan struct{}, 1)
			onEndFunc = chainTTSOnEndFuncs(onEndFunc, func(error) { signalDone(done) })
		}
		if err := t.pushTTSQueueItem(TTSQueueItem{
			ctx:         ctx,
			StreamChan:  streamChan,
			generation:  t.currentAudioGeneration(),
			metricCycle: t.currentTtsMetricCycle(),
			onEndFunc:   onEndFunc,
		}); err != nil {
			safeCloseLLMResponseStream(streamChan)
			if onEndFunc != nil {
				onEndFunc(err)
			}
			return err
		}
		if t.dualStreamEpoch.Load() != streamEpoch {
			safeCloseLLMResponseStream(streamChan)
			return nil
		}
		t.dualStreamMu.Lock()
		if t.dualStreamEpoch.Load() != streamEpoch {
			t.dualStreamMu.Unlock()
			safeCloseLLMResponseStream(streamChan)
			return nil
		}
		t.dualStreamChan = streamChan
		t.dualStreamDone = done
		t.dualStreamMu.Unlock()
		log.Debugf("handleTextResponse: dual stream, created StreamChan and pushed item")
	} else {
		t.dualStreamMu.Lock()
		streamChan = t.dualStreamChan
		t.dualStreamMu.Unlock()
	}

	if streamChan != nil && hasText {
		if err := sendLLMResponseToDualStream(ctx, streamChan, llmResponse); err != nil {
			return err
		}
	} else if streamChan == nil && hasText {
		// degradation: not received to IsStart come data, enqueue as single item
		gen := t.currentAudioGeneration()
		var done chan struct{}
		var onEndFunc func(error)
		if onTTSItemEnqueued != nil {
			onEndFunc = onTTSItemEnqueued()
		}
		if isSync {
			done = make(chan struct{}, 1)
			onEndFunc = chainTTSOnEndFuncs(onEndFunc, func(error) { signalDone(done) })
		}
		if err := t.pushTTSQueueItem(TTSQueueItem{
			ctx:         ctx,
			llmResponse: llmResponse,
			generation:  gen,
			metricCycle: t.currentTtsMetricCycle(),
			onEndFunc:   onEndFunc,
		}); err != nil {
			if onEndFunc != nil {
				onEndFunc(err)
			}
			return err
		}
		log.Debugf("handleTextResponse: dual stream fallback, no active stream, pushed single item")
		if isSync {
			return t.waitForSync(ctx, done)
		}
	}

	if llmResponse.IsEnd && streamChan != nil {
		var done chan struct{}
		t.dualStreamMu.Lock()
		if t.dualStreamChan == streamChan {
			done = t.dualStreamDone
			t.dualStreamChan = nil
			t.dualStreamDone = nil
		}
		t.dualStreamMu.Unlock()
		safeCloseLLMResponseStream(streamChan)
		if isSync && done != nil {
			return t.waitForSync(ctx, done)
		}
	}

	return nil
}

// getEffectiveTTSConfig return current effective of TTS config: use voiceprint config if voiceprint exists, else use device default TTS config (and getTTSProviderInstance consistent)
func (t *TTSManager) getEffectiveTTSConfig() map[string]interface{} {
	if t.clientState.SpeakerTTSConfig != nil && len(t.clientState.SpeakerTTSConfig) > 0 {
		config := make(map[string]interface{})
		for k, v := range t.clientState.SpeakerTTSConfig {
			config[k] = v
		}
		return config
	}
	return t.clientState.DeviceConfig.Tts.Config
}

// SupportsDualStream determine current TTS whether support dual-stream: both TTS input and output are streaming (synthesize output while receiving text), and LLM irrelevant; by config double_stream and TTS provider bind.
func (t *TTSManager) SupportsDualStream() bool {
	config := t.getEffectiveTTSConfig()
	if config == nil {
		return false
	}
	v, ok := config["double_stream"]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s == "true" || s == "1"
	}
	return false
}

// getTTSProviderInstance get TTS Provider instance (use provider+voice as resource pool only key)
func (t *TTSManager) getTTSProviderInstance() (*pool.ResourceWrapper[tts.TTSProvider], error) {
	// get TTS config and provider
	var ttsConfig map[string]interface{}
	var ttsProvider string

	if t.clientState.SpeakerTTSConfig != nil && len(t.clientState.SpeakerTTSConfig) > 0 {
		// use voiceprint TTS config
		if provider, ok := t.clientState.SpeakerTTSConfig["provider"].(string); ok {
			ttsProvider = provider
		} else {
			log.Warnf("missing provider in voiceprint TTS config, use default config")
			ttsProvider = t.clientState.DeviceConfig.Tts.Provider
			ttsConfig = t.clientState.DeviceConfig.Tts.Config
		}
		// deep copy config
		ttsConfig = make(map[string]interface{})
		for k, v := range t.clientState.SpeakerTTSConfig {
			ttsConfig[k] = v
		}
	} else {
		// use default TTS config
		ttsProvider = t.clientState.DeviceConfig.Tts.Provider
		ttsConfig = t.clientState.DeviceConfig.Tts.Config
	}

	// logical identifier (used for log and fingerprint calculation): provider or provider:voiceID
	voiceID := extractVoiceID(ttsConfig)
	providerLabel := ttsProvider
	if voiceID != "" {
		providerLabel = fmt.Sprintf("%s:%s", ttsProvider, voiceID)
	}

	// get TTS resource from resource pool (pool key by config fingerprint decide, host/voice etc change will automatic switch pool)
	ttsWrapper, err := pool.Acquire[tts.TTSProvider]("tts", providerLabel, ttsConfig)
	if err != nil {
		log.Errorf("get TTS resource failed: %v", err)
		return nil, fmt.Errorf("get TTS resource failed: %v", err)
	}

	return ttsWrapper, nil
}

// extractVoiceID extract voice ID from config
func extractVoiceID(config map[string]interface{}) string {
	if config == nil {
		return ""
	}

	// try to get provider type from config
	provider, _ := config["provider"].(string)

	// cosyvoice uses spk_id field
	if provider == "cosyvoice" {
		if spkID, ok := config["spk_id"].(string); ok && spkID != "" {
			return spkID
		}
		return ""
	}

	// minimax and other providers: use voice
	if voice, ok := config["voice"].(string); ok && voice != "" {
		return voice
	}

	return ""
}

// generateTtsOnly scheme C: only do TTS generate, do not send; return audio channel and send complete after need call of ReleaseFunc
func (t *TTSManager) generateTtsOnly(ctx context.Context, metricCycle uint64, llmResponse llm_common.LLMResponseStruct) (outputChan <-chan []byte, releaseFunc func(), err error) {
	if strings.TrimSpace(llmResponse.Text) == "" {
		return nil, nil, nil
	}
	ttsWrapper, err := t.getTTSProviderInstance()
	if err != nil {
		log.Errorf("get TTS Provider instance failed: %v", err)
		return nil, nil, err
	}
	ttsProviderInstance := ttsWrapper.GetProvider()
	t.markTtsMetricRequestStart(ctx, metricCycle)
	ch, err := ttsProviderInstance.TextToSpeechStream(ctx, llmResponse.Text, t.clientState.OutputAudioFormat.SampleRate, t.clientState.OutputAudioFormat.Channels, t.clientState.OutputAudioFormat.FrameDuration)
	if err != nil {
		pool.Release(ttsWrapper)
		t.finishTtsMetricRequest(ctx, metricCycle, err)
		log.Errorf("generate TTS audio failed: %v", err)
		return nil, nil, fmt.Errorf("generate TTS audio failed: %v", err)
	}
	return ch, func() { pool.Release(ttsWrapper) }, nil
}

// handleDualStreamTts real dual-stream TTS: will StreamChan in of text streaming input to TTS provider, at the same time when streaming output audio.
// return true indicate already process (successful or error), false indicate provider unsupported dual-stream needs degradation.
func (t *TTSManager) handleDualStreamTts(item TTSQueueItem) (bool, error) {
	ttsWrapper, err := t.getTTSProviderInstance()
	if err != nil {
		log.Errorf("dual-stream TTS get provider failed: %v", err)
		return false, nil
	}
	defer pool.Release(ttsWrapper)

	provider := ttsWrapper.GetProvider()
	adapter, ok := provider.(*tts.ContextTTSAdapter)
	if !ok {
		return false, nil
	}
	dp, ok := adapter.Provider.(tts.DualStreamProvider)
	if !ok {
		return false, nil
	}

	textChan := make(chan string, 16)
	t.markTtsMetricRequestStart(item.ctx, item.metricCycle)
	eventChan, err := dp.StreamingSynthesize(item.ctx, textChan,
		t.clientState.OutputAudioFormat.SampleRate,
		t.clientState.OutputAudioFormat.Channels,
		t.clientState.OutputAudioFormat.FrameDuration)
	if err != nil {
		close(textChan)
		t.finishTtsMetricRequest(item.ctx, item.metricCycle, err)
		log.Errorf("dual-stream TTS StreamingSynthesize failed: %v", err)
		return false, nil
	}
	requestActive := true
	firstAudioReported := false
	finishRequest := func(err error) {
		if requestActive {
			t.finishTtsMetricRequest(item.ctx, item.metricCycle, err)
			requestActive = false
		}
	}

	// from StreamChan read LLM respond text and feed TTS provider.
	go func() {
		defer close(textChan)
		for {
			select {
			case <-item.ctx.Done():
				return
			case resp, ok := <-item.StreamChan:
				if !ok {
					return
				}
				text := strings.TrimSpace(resp.Text)
				if text == "" {
					continue
				}
				select {
				case textChan <- text:
				case <-item.ctx.Done():
					return
				}
			}
		}
	}()

	firstSentence := true
	for event := range eventChan {
		for _, signal := range event.SentenceSignals {
			switch signal.Type {
			case ttsstream.SentenceSignalEnd:
				if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{
					Kind: AudioQueueKindSentenceEnd,
					Text: signal.Text,
				}) {
					finishRequest(item.ctx.Err())
					if item.onEndFunc != nil {
						item.onEndFunc(item.ctx.Err())
					}
					return true, item.ctx.Err()
				}
			case ttsstream.SentenceSignalStart:
				startElem := AudioQueueElem{
					Kind:    AudioQueueKindSentenceStart,
					Text:    signal.Text,
					IsStart: firstSentence,
				}
				if firstSentence {
					startElem.OnStart = item.onStartFunc
					firstSentence = false
				}
				if !t.enqueueSessionElem(item.ctx, item.generation, startElem) {
					finishRequest(item.ctx.Err())
					if item.onEndFunc != nil {
						item.onEndFunc(item.ctx.Err())
					}
					return true, item.ctx.Err()
				}
			}
		}

		if len(event.Audio) > 0 {
			if !firstAudioReported {
				t.markTtsMetricFirstAudio(item.ctx, item.metricCycle)
				firstAudioReported = true
			}
			frameCopy := make([]byte, len(event.Audio))
			copy(frameCopy, event.Audio)
			if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindFrame, Data: frameCopy}) {
				finishRequest(item.ctx.Err())
				if item.onEndFunc != nil {
					item.onEndFunc(item.ctx.Err())
				}
				return true, item.ctx.Err()
			}
		}
	}

	finishRequest(nil)
	if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceEnd, OnEnd: item.onEndFunc}) && item.onEndFunc != nil {
		item.onEndFunc(nil)
	}
	return true, item.ctx.Err()
}

// handleStreamTts streaming TTS: from item.StreamChan read and one by one generateTtsOnly, to sessionAudioQueue push SentenceStart -> Frame... -> SentenceEnd
func (t *TTSManager) handleStreamTts(item TTSQueueItem) error {
	if t.SupportsDualStream() {
		handled, err := t.handleDualStreamTts(item)
		if handled {
			return err
		}
	}

	firstSegment := true
	for {
		select {
		case <-item.ctx.Done():
			if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceEnd, OnEnd: item.onEndFunc, Err: item.ctx.Err()}) && item.onEndFunc != nil {
				item.onEndFunc(item.ctx.Err())
			}
			return item.ctx.Err()
		case resp, ok := <-item.StreamChan:
			if !ok {
				if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceEnd, OnEnd: item.onEndFunc}) && item.onEndFunc != nil {
					item.onEndFunc(nil)
				}
				return item.ctx.Err()
			}
			outChan, release, genErr := t.generateTtsOnly(item.ctx, item.metricCycle, resp)
			if genErr != nil {
				if firstSegment {
					if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceStart, OnStart: item.onStartFunc}) {
						if item.onEndFunc != nil {
							item.onEndFunc(item.ctx.Err())
						}
						return item.ctx.Err()
					}
				}
				if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceEnd, OnEnd: item.onEndFunc, Err: genErr}) && item.onEndFunc != nil {
					item.onEndFunc(genErr)
				}
				return genErr
			}
			if outChan == nil {
				if release != nil {
					release()
				}
				continue
			}
			requestActive := true
			firstAudioReported := false
			finishRequest := func(err error) {
				if requestActive {
					t.finishTtsMetricRequest(item.ctx, item.metricCycle, err)
					requestActive = false
				}
			}
			startElem := AudioQueueElem{
				Kind:    AudioQueueKindSentenceStart,
				Text:    resp.Text,
				IsStart: resp.IsStart,
			}
			if firstSegment {
				startElem.OnStart = item.onStartFunc
				firstSegment = false
			}
			if !t.enqueueSessionElem(item.ctx, item.generation, startElem) {
				if release != nil {
					release()
				}
				finishRequest(item.ctx.Err())
				if item.onEndFunc != nil {
					item.onEndFunc(item.ctx.Err())
				}
				return item.ctx.Err()
			}
			for {
				select {
				case <-item.ctx.Done():
					if release != nil {
						release()
					}
					finishRequest(item.ctx.Err())
					if item.onEndFunc != nil {
						item.onEndFunc(item.ctx.Err())
					}
					return item.ctx.Err()
				case frame, ok := <-outChan:
					if !ok {
						if release != nil {
							release()
						}
						finishRequest(nil)
						if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindSentenceEnd, Text: resp.Text}) && item.onEndFunc != nil {
							item.onEndFunc(item.ctx.Err())
						}
						goto nextResp
					}
					if !firstAudioReported {
						t.markTtsMetricFirstAudio(item.ctx, item.metricCycle)
						firstAudioReported = true
					}
					frameCopy := make([]byte, len(frame))
					copy(frameCopy, frame)
					if !t.enqueueSessionElem(item.ctx, item.generation, AudioQueueElem{Kind: AudioQueueKindFrame, Data: frameCopy}) {
						if release != nil {
							release()
						}
						finishRequest(item.ctx.Err())
						if item.onEndFunc != nil {
							item.onEndFunc(item.ctx.Err())
						}
						return item.ctx.Err()
					}
				}
			}
		nextResp:
		}
	}
}

// getAlignedDuration calculate current time and start time of difference, round up to frameDuration
func getAlignedDuration(startTime time.Time, frameDuration time.Duration) time.Duration {
	elapsed := time.Since(startTime)
	// round up to frameDuration
	alignedMs := ((elapsed.Milliseconds() + frameDuration.Milliseconds() - 1) / frameDuration.Milliseconds()) * frameDuration.Milliseconds()
	return time.Duration(alignedMs) * time.Millisecond
}

func (t *TTSManager) sendAudioStream(ctx context.Context, audioChan <-chan []byte, isStart bool, recordHistory bool) error {
	totalFrames := 0 // track total sent frames

	isStatistic := true
	// first send 180ms audio, calculate based on outputAudioFormat.FrameDuration
	cacheFrameCount := 120 / t.clientState.OutputAudioFormat.FrameDuration
	/*if cacheFrameCount > 20 || cacheFrameCount < 3 {
		cacheFrameCount = 5
	}*/

	// record start send of timestamp
	startTime := time.Now()

	// precise flow control based on absolute time
	frameDuration := time.Duration(t.clientState.OutputAudioFormat.FrameDuration) * time.Millisecond

	log.Debugf("SendTTSAudio start, cache frame count: %d, frame duration: %v", cacheFrameCount, frameDuration)

	// use sliding window mechanism, ensure to endpoint always cache cacheFrameCount frame data
	for {
		// calculate next frame should send of time point
		nextFrameTime := startTime.Add(time.Duration(totalFrames-cacheFrameCount) * frameDuration)
		now := time.Now()

		// if next frame time not yet reached, need to wait
		if now.Before(nextFrameTime) {
			sleepDuration := nextFrameTime.Sub(now)
			// log.Debugf("SendTTSAudio flow control wait: %v", sleepDuration)
			time.Sleep(sleepDuration)
		}

		// try to get and send next frame
		select {
		case <-ctx.Done():
			log.Debugf("SendTTSAudio context done, exit")
			return nil
		case frame, ok := <-audioChan:
			if !ok {
				// channel closed, all frames processed complete
				// to ensure terminal playback complete: wait total duration of sent frames difference with actual time elapsed since start send
				elapsed := time.Since(startTime)
				totalDuration := time.Duration(totalFrames) * frameDuration
				if totalDuration > elapsed {
					waitDuration := totalDuration - elapsed
					log.Debugf("SendTTSAudio wait client-side playback remaining buffer: %v (totalFrames=%d, frameDuration=%v)", waitDuration, totalFrames, frameDuration)
					time.Sleep(waitDuration)
				}

				log.Debugf("SendTTSAudio audioChan closed, exit, total sent %d frame", totalFrames)
				return nil
			}
			// send current frame
			if err := t.serverTransport.SendAudio(frame); err != nil {
				log.Errorf("send TTS audio failed: nth %d frame, len: %d, error: %v", totalFrames, len(frame), err)
				return fmt.Errorf("send TTS audio len: %d failed: %v", len(frame), err)
			}

			if recordHistory {
				// accumulate audio data to history cache (every frame as independent of []byte)
				t.audioMutex.Lock()
				frameCopy := make([]byte, len(frame))
				copy(frameCopy, frame)
				t.audioHistoryBuffer = append(t.audioHistoryBuffer, frameCopy)
				t.audioMutex.Unlock()
			}

			totalFrames++
			if totalFrames%100 == 0 {
				log.Debugf("SendTTSAudio already sent %d frame", totalFrames)
			}

			// count info record (only at start when record a times)
			if isStart && isStatistic && totalFrames == 1 {
				log.Debugf("from receive audio end asr->llm->tts first frame body body time consumption: %d ms", t.clientState.GetAsrLlmTtsDuration())
				isStatistic = false
			}
		}
	}
}

func (t *TTSManager) SendTTSAudio(ctx context.Context, audioChan <-chan []byte, isStart bool) error {
	if err := t.waitForMediaPlaybackRelease(ctx); err != nil {
		return err
	}
	return t.sendAudioStream(ctx, audioChan, isStart, true)
}

func (t *TTSManager) SendMediaAudio(ctx context.Context, audioChan <-chan []byte) error {
	return t.sendAudioStream(ctx, audioChan, false, false)
}

// ClearAudioHistory clear TTS audio history cache
func (t *TTSManager) ClearAudioHistory() {
	t.audioMutex.Lock()
	defer t.audioMutex.Unlock()
	t.audioHistoryBuffer = nil
}

// GetAndClearAudioHistory get and clear TTS audio history cache
func (t *TTSManager) GetAndClearAudioHistory() [][]byte {
	t.audioMutex.Lock()
	defer t.audioMutex.Unlock()
	data := t.audioHistoryBuffer
	t.audioHistoryBuffer = nil
	return data
}
