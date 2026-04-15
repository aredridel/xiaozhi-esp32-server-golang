package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/schema"

	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/pool"
	log "xiaozhi-esp32-server-golang/logger"
)

var openClawWarmupSchedule = []time.Duration{
	1 * time.Second,
	10 * time.Second,
	20 * time.Second,
	30 * time.Second,
	40 * time.Second,
	50 * time.Second,
	60 * time.Second,
	70 * time.Second,
	80 * time.Second,
	90 * time.Second,
	100 * time.Second,
}

const (
	openClawWarmupPlanTimeout = 8 * time.Second
	openClawWarmupPlanSize    = 11
)

const openClawWarmupSystemPrompt = `You are a warm-up assistant in voice conversations, not the main responder.

Your task is: before the main reply returns, generate 11 very short Chinese conversational fillers, making the waiting process sound like someone is always responding.

Hard requirements:
1. Only responsible for warm-up, cannot directly answer questions, cannot give facts, conclusions, suggestions, steps, analysis, explanations or speculations.
2. Tone should be like a real person softly responding in conversation: brief, natural, colloquial, patient.
3. Don't be like customer service, don't be like system prompts, don't be like notification broadcasts, don't be like copywriting.
4. Forbidden to repeat user's original words, especially don't splice user instructions like "help me check", "help me look", "help me query", "tell me" into the reply.
5. If need to mention topic, can only extract into assistant perspective noun short phrase, e.g. "Beijing weather tomorrow", "this arrangement"; don't use imperative sentences.
6. First 1-2 sentences should be as light as possible, not necessarily with topic words, e.g. "let me see", "wait a moment"; don't start with heavy comforting words.
7. After a few sentences gradually express "I'm looking" or "I acknowledge", but naturally, don't mechanically repeat.
8. Avoid using stiff expressions like "it's being processed", "please wait", "continuing to follow up", "retrieving data", "accessing service".
9. Each must be a single short Chinese sentence, suitable for voice broadcast, length controlled at 4-16 Chinese characters.
10. You will get actual broadcast time points. 11 conversational phrases must be strictly designed according to these time points in sequence:
    - 1st second: like just received the question, softly respond.
    - 10th second: naturally add a sentence, tone still light.
    - 20th, 30th second: start expressing "I'm looking", but not mechanically.
    - 40th, 50th, 60th second: continue comforting, allow more explicit "acknowledging".
    - 70th, 80th, 90th, 100th second: admit time is a bit long, but still natural, calm, no complaints.
11. Only output strict JSON array, length must be 11.
12. Each JSON item format must be: {"text":"warm-up phrase"}.
13. Forbidden to output numbering, Markdown, explanations, code blocks or any content outside JSON.`

type openClawWarmupTask struct {
	correlationID string
	sessionCtx    context.Context
	warmupCtx     context.Context
	cancelWarmup  context.CancelFunc

	linesMu sync.RWMutex
	lines   []string

	stateMu                  sync.Mutex
	speechStarted            bool
	speechEnded              bool
	nextWarmupSegmentIsStart bool
	planReadyAt              time.Time
	planReadySignaled        bool

	spokeAny    atomic.Bool
	planReadyCh chan struct{}
}

type openClawWarmupLine struct {
	Text string `json:"text"`
}

func (s *ChatSession) startOpenClawWarmup(correlationID string, userText string) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" || s == nil || s.clientState == nil {
		return
	}

	sessionCtx := s.clientState.SessionCtx.Get(s.clientState.Ctx)
	parentCtx := s.clientState.AfterAsrSessionCtx.Get(sessionCtx)
	warmupCtx, cancelWarmup := context.WithCancel(parentCtx)
	task := &openClawWarmupTask{
		correlationID:            correlationID,
		sessionCtx:               parentCtx,
		warmupCtx:                warmupCtx,
		cancelWarmup:             cancelWarmup,
		lines:                    make([]string, openClawWarmupPlanSize),
		nextWarmupSegmentIsStart: true,
		planReadyCh:              make(chan struct{}),
	}

	s.replaceOpenClawWarmup(task)
	log.Infof("OpenClaw warmup started: device=%s correlation_id=%s", s.clientState.DeviceID, correlationID)

	go s.runOpenClawWarmupTask(task, userText)
}

func (s *ChatSession) replaceOpenClawWarmup(task *openClawWarmupTask) {
	s.openClawWarmupMu.Lock()
	oldTask := s.openClawWarmup
	s.openClawWarmup = task
	s.openClawWarmupMu.Unlock()

	if oldTask != nil {
		oldTask.cancelWarmupOnly()
	}
}

func (task *openClawWarmupTask) cancelWarmupOnly() {
	if task == nil || task.cancelWarmup == nil {
		return
	}
	task.cancelWarmup()
}

func (task *openClawWarmupTask) markSpeechStarted() bool {
	if task == nil {
		return false
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if task.speechStarted || task.speechEnded {
		return false
	}
	task.speechStarted = true
	return true
}

func (task *openClawWarmupTask) markSpeechEnded() bool {
	if task == nil {
		return false
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if !task.speechStarted || task.speechEnded {
		return false
	}
	task.speechEnded = true
	return true
}

func (task *openClawWarmupTask) takeWarmupSegmentStartFlag() bool {
	if task == nil {
		return true
	}
	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	isStart := task.nextWarmupSegmentIsStart
	task.nextWarmupSegmentIsStart = false
	return isStart
}

func (task *openClawWarmupTask) markPlanReady(readyAt time.Time) {
	if task == nil {
		return
	}
	task.stateMu.Lock()
	if task.planReadySignaled {
		task.stateMu.Unlock()
		return
	}
	task.planReadyAt = readyAt
	task.planReadySignaled = true
	close(task.planReadyCh)
	task.stateMu.Unlock()
}

func (task *openClawWarmupTask) waitPlanReady(ctx context.Context) (time.Time, bool) {
	if task == nil {
		return time.Time{}, false
	}

	select {
	case <-ctx.Done():
		return time.Time{}, false
	case <-task.planReadyCh:
	}

	task.stateMu.Lock()
	defer task.stateMu.Unlock()
	if task.planReadyAt.IsZero() {
		return time.Time{}, false
	}
	return task.planReadyAt, true
}

func (task *openClawWarmupTask) hasSpokenAny() bool {
	if task == nil {
		return false
	}
	return task.spokeAny.Load()
}

func (s *ChatSession) getOpenClawWarmupTask(correlationID string) *openClawWarmupTask {
	if s == nil {
		return nil
	}
	correlationID = strings.TrimSpace(correlationID)
	s.openClawWarmupMu.Lock()
	defer s.openClawWarmupMu.Unlock()
	task := s.openClawWarmup
	if task == nil {
		return nil
	}
	if correlationID != "" && task.correlationID != correlationID {
		return nil
	}
	return task
}

func (s *ChatSession) takeOpenClawWarmupTask(correlationID string) *openClawWarmupTask {
	if s == nil {
		return nil
	}
	correlationID = strings.TrimSpace(correlationID)
	s.openClawWarmupMu.Lock()
	defer s.openClawWarmupMu.Unlock()
	task := s.openClawWarmup
	if task == nil {
		return nil
	}
	if correlationID != "" && task.correlationID != correlationID {
		return nil
	}
	s.openClawWarmup = nil
	return task
}

func (s *ChatSession) cancelOpenClawWarmup(correlationID string, interrupt bool) bool {
	if s == nil {
		return false
	}

	task := s.getOpenClawWarmupTask(correlationID)
	if task == nil {
		return false
	}
	if task.warmupCtx.Err() != nil {
		return false
	}

	task.cancelWarmupOnly()
	if interrupt && task.hasSpokenAny() {
		s.InterruptAndClearTTSQueue()
	}

	log.Infof(
		"OpenClaw warmup canceled: device=%s correlation_id=%s interrupt=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		interrupt,
		task.hasSpokenAny(),
	)
	return true
}

func (s *ChatSession) finishOpenClawWarmup(correlationID string, interrupt bool) bool {
	task := s.takeOpenClawWarmupTask(correlationID)
	if task == nil {
		return false
	}

	task.cancelWarmupOnly()
	if interrupt {
		s.InterruptAndClearTTSQueue()
	}
	s.endOpenClawSpeech(task)

	log.Infof(
		"OpenClaw warmup finished: device=%s correlation_id=%s interrupt=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		interrupt,
		task.hasSpokenAny(),
	)
	return true
}

func (s *ChatSession) beginOpenClawSpeech(task *openClawWarmupTask) {
	if task == nil {
		return
	}
	if !task.markSpeechStarted() {
		return
	}
	s.ttsManager.ClearAudioHistory()
	s.ttsManager.EnqueueTtsStart(task.sessionCtx)
}

func (s *ChatSession) endOpenClawSpeech(task *openClawWarmupTask) {
	if task == nil {
		return
	}
	if !task.markSpeechEnded() {
		return
	}
	s.ttsManager.GetAndClearAudioHistory()
}

func (s *ChatSession) runOpenClawWarmupTask(task *openClawWarmupTask, userText string) {
	planCtx, cancel := context.WithTimeout(task.warmupCtx, openClawWarmupPlanTimeout)
	defer cancel()
	defer log.Infof(
		"OpenClaw warmup task stopped: device=%s correlation_id=%s warmup_err=%v session_err=%v spoke_any=%v",
		s.clientState.DeviceID,
		task.correlationID,
		task.warmupCtx.Err(),
		task.sessionCtx.Err(),
		task.hasSpokenAny(),
	)

	go func() {
		lines, err := s.generateOpenClawWarmupPlan(planCtx, task.correlationID, userText)
		if err != nil {
			if planCtx.Err() == nil {
				log.Warnf("OpenClaw warmup plan generation failed: device=%s correlation_id=%s err=%v", s.clientState.DeviceID, task.correlationID, err)
			}
			task.markPlanReady(time.Time{})
			return
		}
		task.setLines(lines)
		task.markPlanReady(time.Now())
		log.Infof("OpenClaw warmup plan ready: device=%s correlation_id=%s line_count=%d", s.clientState.DeviceID, task.correlationID, len(lines))
	}()

	baseAt, ok := task.waitPlanReady(task.warmupCtx)
	if !ok {
		return
	}

	for idx, delay := range openClawWarmupSchedule {
		if !waitOpenClawWarmupUntil(task.warmupCtx, baseAt.Add(delay)) {
			return
		}
		if task.warmupCtx.Err() != nil {
			return
		}

		text := task.lineAt(idx)
		if text == "" {
			continue
		}

		log.Infof(
			"OpenClaw warmup speaking: device=%s correlation_id=%s slot=%d text=%q",
			s.clientState.DeviceID,
			task.correlationID,
			idx,
			text,
		)
		if err := s.speakOpenClawWarmupLine(task, text); err != nil && task.sessionCtx.Err() == nil {
			log.Warnf("OpenClaw warmup speak failed: device=%s correlation_id=%s slot=%d err=%v", s.clientState.DeviceID, task.correlationID, idx, err)
			return
		}
		task.spokeAny.Store(true)
	}

	// don't cleanup active task here: the last warm-up audio may still be sending/playing,
	// need to continue allowing OpenClaw first sentence to arrive when executing preemptive interrupt.
}

func waitOpenClawWarmupUntil(ctx context.Context, deadline time.Time) bool {
	wait := time.Until(deadline)
	if wait <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (task *openClawWarmupTask) setLines(lines []string) {
	if task == nil || len(lines) == 0 {
		return
	}

	task.linesMu.Lock()
	defer task.linesMu.Unlock()

	if task.lines == nil {
		task.lines = make([]string, openClawWarmupPlanSize)
	}
	for idx := 0; idx < openClawWarmupPlanSize && idx < len(lines); idx++ {
		if text := sanitizeOpenClawWarmupText(lines[idx]); text != "" {
			task.lines[idx] = text
		}
	}
}

func (task *openClawWarmupTask) lineAt(index int) string {
	if task == nil || index < 0 {
		return ""
	}

	task.linesMu.RLock()
	defer task.linesMu.RUnlock()

	if index >= len(task.lines) {
		return ""
	}
	return strings.TrimSpace(task.lines[index])
}

func (s *ChatSession) speakOpenClawWarmupLine(task *openClawWarmupTask, text string) error {
	text = sanitizeOpenClawWarmupText(text)
	if text == "" {
		return nil
	}
	if task == nil {
		return nil
	}
	if task.sessionCtx.Err() != nil {
		return task.sessionCtx.Err()
	}

	s.beginOpenClawSpeech(task)
	if task.sessionCtx.Err() != nil {
		return task.sessionCtx.Err()
	}

	resp := llm_common.LLMResponseStruct{
		Text:    text,
		IsStart: task.takeWarmupSegmentStartFlag(),
		IsEnd:   true,
	}
	// 暖场句needensurealreadyentersendchain路，avoidbeaftercontinuepositive式回复“看起来像没effective”。
	return s.ttsManager.handleTextResponse(task.sessionCtx, resp, true)
}

func (s *ChatSession) generateOpenClawWarmupPlan(ctx context.Context, correlationID string, userText string) ([]string, error) {
	llmWrapper, err := pool.Acquire[llm.LLMProvider](
		"llm",
		s.clientState.DeviceConfig.Llm.Provider,
		s.clientState.DeviceConfig.Llm.Config,
	)
	if err != nil {
		return nil, fmt.Errorf("acquire llm provider: %w", err)
	}
	defer pool.Release(llmWrapper)

	dialogue := []*schema.Message{
		schema.SystemMessage(openClawWarmupSystemPrompt),
		schema.UserMessage(buildOpenClawWarmupUserPrompt(userText)),
	}

	msgChan := llmWrapper.GetProvider().ResponseWithContext(
		ctx,
		buildOpenClawWarmupSessionID(s.clientState.SessionID, correlationID),
		dialogue,
		nil,
	)

	raw, err := collectOpenClawWarmupResponse(ctx, msgChan)
	if err != nil {
		return nil, err
	}
	lines := parseOpenClawWarmupPlan(raw)
	if countOpenClawWarmupLines(lines) == 0 {
		return nil, fmt.Errorf("empty warmup plan")
	}
	return lines, nil
}

func buildOpenClawWarmupUserPrompt(userText string) string {
	trimmed := strings.TrimSpace(userText)
	topic := formatOpenClawWarmupTopic(buildOpenClawWarmupHint(userText))
	topicLine := "Don't repeat user instructions like 'help me check'."
	if topic != "" {
		topicLine = fmt.Sprintf("If need to mention topic, can only extract into noun short phrase \"%s\", don't repeat user instructions like 'help me check'.", topic)
	}
	return fmt.Sprintf(
		"User's current task:\n%s\n\n%s\n\nActual broadcast time points in sequence are: 1st second, 10th second, 20th second, 30th second, 40th second, 50th second, 60th second, 70th second, 80th second, 90th second, 100th second.\nPlease output 11 warm-up phrases, corresponding to the above 11 time points.",
		trimmed,
		topicLine,
	)
}

func buildOpenClawWarmupSessionID(sessionID string, correlationID string) string {
	base := strings.TrimSpace(sessionID)
	if base == "" {
		base = "openclaw"
	}
	correlationID = strings.TrimSpace(correlationID)
	if len(correlationID) > 12 {
		correlationID = correlationID[:12]
	}
	if correlationID == "" {
		return base + ":warmup"
	}
	return base + ":warmup:" + correlationID
}

func collectOpenClawWarmupResponse(ctx context.Context, msgChan chan *schema.Message) (string, error) {
	var builder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return builder.String(), ctx.Err()
		case msg, ok := <-msgChan:
			if !ok {
				return builder.String(), nil
			}
			if msg == nil {
				continue
			}
			if llm.IsLLMErrorMessage(msg) {
				errMsg := strings.TrimSpace(llm.LLMErrorMessage(msg))
				if errMsg == "" {
					errMsg = "unknown llm error"
				}
				return builder.String(), fmt.Errorf("llm returned error: %s", errMsg)
			}
			if msg.Content != "" {
				builder.WriteString(msg.Content)
			}
		}
	}
}

func parseOpenClawWarmupPlan(raw string) []string {
	lines := make([]string, openClawWarmupPlanSize)

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return lines
	}

	candidate := raw
	start := strings.Index(candidate, "[")
	end := strings.LastIndex(candidate, "]")
	if start >= 0 && end > start {
		candidate = candidate[start : end+1]
	}

	var objectItems []openClawWarmupLine
	if err := json.Unmarshal([]byte(candidate), &objectItems); err == nil {
		return buildOpenClawWarmupPlanLines(objectItemsToStrings(objectItems))
	}

	var stringItems []string
	if err := json.Unmarshal([]byte(candidate), &stringItems); err == nil {
		return buildOpenClawWarmupPlanLines(stringItems)
	}

	log.Warnf("OpenClaw warmup plan parse failed, ignored: raw=%q", raw)
	return lines
}

func objectItemsToStrings(items []openClawWarmupLine) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.Text)
	}
	return lines
}

func buildOpenClawWarmupPlanLines(items []string) []string {
	lines := make([]string, openClawWarmupPlanSize)
	for idx := 0; idx < openClawWarmupPlanSize && idx < len(items); idx++ {
		if text := sanitizeOpenClawWarmupText(items[idx]); text != "" {
			lines[idx] = text
		}
	}
	return lines
}

func countOpenClawWarmupLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func sanitizeOpenClawWarmupText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'`[]{}")
	text = strings.TrimLeft(text, "0123456789.、- ")
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	if isInvalidOpenClawWarmupText(text) {
		return ""
	}

	runes := []rune(text)
	if len(runes) > 16 {
		return ""
	}
	return text
}

func isInvalidOpenClawWarmupText(text string) bool {
	for _, bad := range []string{
		"帮我",
		"给我",
		"告诉我",
		"please帮",
		"麻烦帮",
		"能帮我",
		"can帮我",
		"帮忙查",
		"帮忙看",
		"帮忙问",
	} {
		if strings.Contains(text, bad) {
			return true
		}
	}
	return false
}

func buildOpenClawWarmupHint(userText string) string {
	trimmed := strings.TrimSpace(userText)
	if trimmed == "" {
		return ""
	}

	normalized := removePunctuation(trimmed)
	if normalized == "" {
		return ""
	}
	normalized = trimOpenClawWarmupCommandPrefix(normalized)
	normalized = trimOpenClawWarmupQuestionSuffix(normalized)
	if normalized == "" {
		return ""
	}

	for _, keyword := range []string{"天气", "气温", "温degree", "预报"} {
		if idx := strings.Index(normalized, keyword); idx >= 0 {
			limit := idx + len([]rune(keyword))
			runes := []rune(normalized)
			if limit > len(runes) {
				limit = len(runes)
			}
			normalized = string(runes[:limit])
			break
		}
	}

	runes := []rune(normalized)
	if len(runes) > 10 {
		runes = runes[:10]
	}
	for len(runes) > 0 {
		last := runes[len(runes)-1]
		if last == '的' || last == '吗' || last == '呢' {
			runes = runes[:len(runes)-1]
			continue
		}
		break
	}
	return string(runes)
}

func trimOpenClawWarmupCommandPrefix(text string) string {
	trimmed := strings.TrimSpace(text)
	for {
		changed := false
		for _, prefix := range []string{
			"麻烦帮我queryadown",
			"麻烦帮我查adown",
			"麻烦帮我看adown",
			"please帮我queryadown",
			"please帮我查adown",
			"please帮我看adown",
			"帮我queryadown",
			"帮我查adown",
			"帮我看adown",
			"帮我问adown",
			"给我queryadown",
			"给我查adown",
			"给我看adown",
			"can帮我查adown",
			"can帮我看adown",
			"能帮我查adown",
			"能帮我看adown",
			"我want知道",
			"我want问adown",
			"我want问",
			"please问adown",
			"please问",
			"queryadown",
			"查adown",
			"看adown",
			"问adown",
			"帮我query",
			"帮我查",
			"帮我看",
			"帮我问",
			"给我query",
			"给我查",
			"给我看",
			"query",
			"查",
			"看",
			"问",
		} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}
	return trimmed
}

func trimOpenClawWarmupQuestionSuffix(text string) string {
	trimmed := strings.TrimSpace(text)
	for _, suffix := range []string{
		"怎么样",
		"如何",
		"多少",
		"yes什么",
		"yes啥",
		"吗",
		"呢",
		"呀",
		"吧",
	} {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
	}
	return trimmed
}

func formatOpenClawWarmupTopic(hint string) string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return ""
	}
	for _, keyword := range []string{"天气", "气温", "温degree", "预报"} {
		if idx := strings.Index(hint, keyword); idx > 0 {
			prefix := strings.TrimSpace(hint[:idx])
			if prefix == "" || strings.HasSuffix(prefix, "of") {
				return hint
			}
			return prefix + "of" + hint[idx:]
		}
	}
	return hint
}
