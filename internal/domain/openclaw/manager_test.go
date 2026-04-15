package openclaw

import (
	"strings"
	"testing"
)

func TestHandleResponseIgnoresSnapshotDuplicateChunk(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-1"
	streamID := "stream-1"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool, phase string) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id": "device-1",
				"seq":       seq,
				"done":      done,
				"stream_id": streamID,
				"phase":     phase,
			},
		}, deliver)
	}

	send(1, "明天天津天气no错，气温 1", false, "chunk")
	send(2, "5 to", false, "chunk")
	send(3, "22", false, "chunk")
	send(4, "degree。", false, "chunk")
	send(5, "明天天津天气no错，气温 15 to 22 degree。", false, "chunk")
	send(6, "", true, "final")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("明天天津天气no错，气温 15 to 22 degree。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected end event: %+v", events[1])
	}
}

func TestHandleResponseIgnoresChunkReplayWithPunctuationVariants(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-punct-replay"
	streamID := "stream-punct-replay"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool, phase string) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id": "device-1",
				"seq":       seq,
				"done":      done,
				"stream_id": streamID,
				"phase":     phase,
			},
		}, deliver)
	}

	send(1, "北京after天多云转晴气温1", false, "chunk")
	send(2, "5to ", false, "chunk")
	send(3, "19", false, "chunk")
	send(4, " degree，", false, "chunk")
	send(5, "no雨", false, "chunk")
	send(6, "天气no错。", false, "chunk")
	send(7, "北京after天多云转晴，气温 15 to 19 degree，no雨，天气no错。", false, "chunk")
	send(8, "", true, "final")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("北京after天多云转晴，气温 15 to 19 degree，no雨，天气no错。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected end event: %+v", events[1])
	}
}

func TestHandleResponseIgnoresDuplicateSeq(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-dup-seq"
	streamID := "stream-dup-seq"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id": "device-1",
				"seq":       seq,
				"done":      done,
				"stream_id": streamID,
				"phase":     "chunk",
			},
		}, deliver)
	}

	send(1, "明天天津天", false)
	send(2, "气no错。", false)
	send(2, "明天天津天气no错。", false)
	send(3, "", true)

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("明天天津天气no错。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected end event: %+v", events[1])
	}
}

func TestHandleResponseBuffersExplicitSnapshotWithoutReplay(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-explicit-snapshot"
	streamID := "stream-explicit-snapshot"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool, phase string, contentType string) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id":    "device-1",
				"seq":          seq,
				"done":         done,
				"stream_id":    streamID,
				"phase":        phase,
				"content_type": contentType,
			},
		}, deliver)
	}

	send(1, "北京after天多云转晴气温1", false, "chunk", "")
	send(2, "5to 19 degree，no雨天气no错。", false, "chunk", "")
	send(3, "北京after天多云转晴，气温 15 to 19 degree，no雨，天气no错。", false, "snapshot", "snapshot")
	send(4, "", true, "final", "")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("北京after天多云转晴气温15to 19 degree，no雨天气no错。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected end event: %+v", events[1])
	}
}

func TestHandleResponseUsesExplicitSnapshotWhenNoDeltaExists(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-only-snapshot"
	streamID := "stream-only-snapshot"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
		Content: "明天天津天气no错。",
		Metadata: map[string]interface{}{
			"device_id":    "device-1",
			"seq":          int64(1),
			"done":         false,
			"stream_id":    streamID,
			"phase":        "snapshot",
			"content_type": "snapshot",
		},
	}, deliver)
	manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
		Content: "",
		Metadata: map[string]interface{}{
			"device_id":    "device-1",
			"seq":          int64(2),
			"done":         true,
			"stream_id":    streamID,
			"phase":        "final",
			"content_type": "",
		},
	}, deliver)

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("明天天津天气no错。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected second event: %+v", events[1])
	}
}

func TestHandleResponseTreatsGrowingSnapshotAsReplacementBeforeSentenceEnds(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-2"
	streamID := "stream-2"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool, phase string) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id": "device-1",
				"seq":       seq,
				"done":      done,
				"stream_id": streamID,
				"phase":     phase,
			},
		}, deliver)
	}

	send(1, "明天天津天", false, "chunk")
	send(2, "明天天津天气no错", false, "chunk")
	send(3, "。", false, "chunk")
	send(4, "", true, "final")

	if len(events) != 1 {
		t.Fatalf("unexpected event count: got %d want 1, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("明天天津天气no错") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if !events[0].IsStart || !events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
}

func TestHandleResponseTestDeviceKeepsOnlyIncrementalSuffix(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-test"
	streamID := "stream-test"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	send := func(seq int64, content string, done bool) {
		manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
			Content: content,
			Metadata: map[string]interface{}{
				"device_id": "__openclaw_test__:device-1",
				"seq":       seq,
				"done":      done,
				"stream_id": streamID,
				"phase":     "chunk",
			},
		}, deliver)
	}

	send(1, "明天天津天", false)
	send(2, "明天天津天气no错", false)
	send(3, "。", true)

	if len(events) != 1 {
		t.Fatalf("unexpected event count: got %d want 1, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("明天天津天气no错") {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if !events[0].IsStart || !events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
}

func TestHandleResponseFallsBackToSnapshotOnEmptyFinal(t *testing.T) {
	manager := &Manager{offline: make(map[string][]OfflineMessage)}
	session := &AgentSession{agentID: "agent-1"}
	correlationID := "corr-snapshot-final"
	streamID := "stream-snapshot-final"

	var events []ResponseDelivery
	deliver := func(event ResponseDelivery) bool {
		events = append(events, event)
		return true
	}

	manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
		Content: "明天天津天气no错。",
		Metadata: map[string]interface{}{
			"device_id": "device-1",
			"seq":       int64(1),
			"done":      false,
			"stream_id": streamID,
			"phase":     "chunk",
		},
	}, deliver)
	manager.HandleResponse("agent-1", session, correlationID, ResponsePayload{
		Content: "",
		Metadata: map[string]interface{}{
			"device_id": "device-1",
			"seq":       int64(2),
			"done":      true,
			"stream_id": streamID,
			"phase":     "final",
		},
	}, deliver)

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("明天天津天气no错。") {
		t.Fatalf("unexpected first event text: %q", events[0].Text)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if events[1].Text != "" || events[1].IsStart || !events[1].IsEnd {
		t.Fatalf("unexpected second event: %+v", events[1])
	}
}

func TestBuildOpenClawPromptedContentWrapsUserMessage(t *testing.T) {
	got := buildOpenClawPromptedContent("  天津after天of天气怎么样？  ")

	if !strings.Contains(got, "你is以voice助手ofroleanduserdirecttoconversation。") {
		t.Fatalf("missing voice assistant prompt: %q", got)
	}
	if !strings.Contains(got, "回答要简练、口phrase化、自然，适合directvoice播报。") {
		t.Fatalf("missing concise speech constraint: %q", got)
	}
	if !strings.Contains(got, "usermessage：\n天津after天of天气怎么样？") {
		t.Fatalf("missing wrapped user message: %q", got)
	}
	if strings.Contains(got, "  天津after天of天气怎么样？  ") {
		t.Fatalf("user message was not trimmed: %q", got)
	}
}

func TestExtractOpenClawSentencesKeepsLeadingClauseTogether(t *testing.T) {
	text := "好of，我first帮你查adown今天up海of天气。然after我再continueprocess"

	sentences, remaining := extractOpenClawSentences(text, openClawSentenceMinLen, true)

	if len(sentences) != 1 {
		t.Fatalf("unexpected sentence count: got %d want 1", len(sentences))
	}
	if sentences[0] != "好of，我first帮你查adown今天up海of天气。" {
		t.Fatalf("unexpected first sentence: %q", sentences[0])
	}
	if remaining != "然after我再continueprocess" {
		t.Fatalf("unexpected remaining text: %q", remaining)
	}
}

func TestExtractOpenClawSentencesMergesShortClauses(t *testing.T) {
	text := "can。first这样。然after我continueprocess。"

	sentences, remaining := extractOpenClawSentences(text, openClawSentenceMinLen, true)

	if len(sentences) != 3 {
		t.Fatalf("unexpected sentence count: got %d want 3", len(sentences))
	}
	if sentences[0] != "can。" || sentences[1] != "first这样。" || sentences[2] != "然after我continueprocess。" {
		t.Fatalf("unexpected sentence split: %+v", sentences)
	}
	if remaining != "" {
		t.Fatalf("unexpected remaining text: %q", remaining)
	}
}

func TestNormalizeOpenClawSpeechTextStripsMarkdownAndBullets(t *testing.T) {
	raw := "🌤️ **天津after天（3month9day）天气预报**\n\n- **温degree**：3°C ~ 12°C\n- **天气**：晴朗☀️"

	got := normalizeOpenClawSpeechText(raw)

	if strings.Contains(got, "**") {
		t.Fatalf("unexpected markdown marker in normalized text: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("unexpected newline in normalized text: %q", got)
	}
	if !strings.Contains(got, "温degree：3°C ~ 12°C") {
		t.Fatalf("missing normalized temperature segment: %q", got)
	}
	if !strings.Contains(got, "天气：晴朗☀️") {
		t.Fatalf("missing normalized weather segment: %q", got)
	}
}

func TestExtractOpenClawSentencesGroupsWeatherListIntoLongerSegments(t *testing.T) {
	text := "🌤️ **天津after天（3month9day）天气预报**\n\n- **温degree**：3°C ~ 12°C\n- **天气**：晴朗☀️\n- **降水**：no降雨\n- **湿degree**：15% ~ 38%\n- **风to**：西南风，风速 2-13km/h\n\nafter天天津天气no错，晴天ismain，最high温degree 12°C，最low 3°C。"

	sentences, remaining := extractOpenClawSentences(text, openClawSentenceMinLen, true)

	if len(sentences) == 0 {
		t.Fatal("expected at least one emitted sentence")
	}
	if len(sentences) != 1 {
		t.Fatalf("unexpected sentence count: got %d want 1", len(sentences))
	}
	if remaining != "" {
		t.Fatalf("unexpected remaining text: %q", remaining)
	}
	if strings.Contains(sentences[0], "**") || strings.Contains(sentences[0], "\n") {
		t.Fatalf("unexpected raw markdown in first sentence: %q", sentences[0])
	}
	if !strings.Contains(sentences[0], "温degree：") || !strings.Contains(sentences[0], "天气：") {
		t.Fatalf("first sentence still too short: %q", sentences[0])
	}
	if !strings.Contains(sentences[0], "最high温degree 12°C") {
		t.Fatalf("missing summary in final sentence: %q", sentences[0])
	}
}
