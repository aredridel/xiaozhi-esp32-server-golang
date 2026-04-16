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

	send(1, "Tomorrow Tianjin weather is nice, temperature 1", false, "chunk")
	send(2, "5 to", false, "chunk")
	send(3, "22", false, "chunk")
	send(4, "degree.", false, "chunk")
	send(5, "Tomorrow Tianjin weather is nice, temperature 15 to 22 degree.", false, "chunk")
	send(6, "", true, "final")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("Tomorrow Tianjin weather is nice, temperature 15 to 22 degree.") {
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

	send(1, "Beijing tomorrow cloudy turning sunny temperature 1", false, "chunk")
	send(2, "5to ", false, "chunk")
	send(3, "19", false, "chunk")
	send(4, " degree，", false, "chunk")
	send(5, "no rain", false, "chunk")
	send(6, "weather is nice.", false, "chunk")
	send(7, "Beijing tomorrow cloudy turning sunny, temperature 15 to 19 degree，no rain, weather is nice.", false, "chunk")
	send(8, "", true, "final")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if !events[0].IsStart || events[0].IsEnd {
		t.Fatalf("unexpected first event flags: %+v", events[0])
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("Beijing tomorrow cloudy turning sunny, temperature 15 to 19 degree，no rain, weather is nice.") {
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

	send(1, "Tomorrow Tianjin weather", false)
	send(2, "is nice.", false)
	send(2, "Tomorrow Tianjin weather is nice.", false)
	send(3, "", true)

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("Tomorrow Tianjin weather is nice.") {
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

	send(1, "Beijing tomorrow cloudy turning sunny temperature 1", false, "chunk", "")
	send(2, "5to 19 degree，no rain weather is nice.", false, "chunk", "")
	send(3, "Beijing tomorrow cloudy turning sunny, temperature 15 to 19 degree，no rain, weather is nice.", false, "snapshot", "snapshot")
	send(4, "", true, "final", "")

	if len(events) != 2 {
		t.Fatalf("unexpected event count: got %d want 2, events=%+v", len(events), events)
	}
	if openClawComparableKey(events[0].Text) != openClawComparableKey("Beijing tomorrow cloudy turning sunny temperature 15to 19 degree，no rain weather is nice.") {
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
		Content: "Tomorrow Tianjin weather is nice.",
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
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("Tomorrow Tianjin weather is nice.") {
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

	send(1, "Tomorrow Tianjin weather", false, "chunk")
	send(2, "Tomorrow Tianjin weather is nice", false, "chunk")
	send(3, "。", false, "chunk")
	send(4, "", true, "final")

	if len(events) != 1 {
		t.Fatalf("unexpected event count: got %d want 1, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("Tomorrow Tianjin weather is nice") {
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

	send(1, "Tomorrow Tianjin weather", false)
	send(2, "Tomorrow Tianjin weather is nice", false)
	send(3, "。", true)

	if len(events) != 1 {
		t.Fatalf("unexpected event count: got %d want 1, events=%+v", len(events), events)
	}
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("Tomorrow Tianjin weather is nice") {
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
		Content: "Tomorrow Tianjin weather is nice.",
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
	if openClawCanonicalKey(events[0].Text) != openClawCanonicalKey("Tomorrow Tianjin weather is nice.") {
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
	got := buildOpenClawPromptedContent("  Tomorrow's weather in Tianjin?  ")

	if !strings.Contains(got, "You are a voice assistant having a direct conversation with the user.") {
		t.Fatalf("missing voice assistant prompt: %q", got)
	}
	if !strings.Contains(got, "Responses should be concise, colloquial, natural, and suitable for direct voice broadcast.") {
		t.Fatalf("missing concise speech constraint: %q", got)
	}
	if !strings.Contains(got, "User message:\nTomorrow's weather in Tianjin?") {
		t.Fatalf("missing wrapped user message: %q", got)
	}
	if strings.Contains(got, "  Tomorrow's weather in Tianjin?  ") {
		t.Fatalf("user message was not trimmed: %q", got)
	}
}

func TestExtractOpenClawSentencesKeepsLeadingClauseTogether(t *testing.T) {
	text := "Okay, I'll first check today's weather in Shanghai for you. Then I'll continue processing"

	sentences, remaining := extractOpenClawSentences(text, openClawSentenceMinLen, true)

	if len(sentences) != 1 {
		t.Fatalf("unexpected sentence count: got %d want 1", len(sentences))
	}
	if sentences[0] != "Okay, I'll first check today's weather in Shanghai for you." {
		t.Fatalf("unexpected first sentence: %q", sentences[0])
	}
	if remaining != "Then I'll continue processing" {
		t.Fatalf("unexpected remaining text: %q", remaining)
	}
}

func TestExtractOpenClawSentencesMergesShortClauses(t *testing.T) {
	text := "Okay. Like this first. Then I'll continue processing."

	sentences, remaining := extractOpenClawSentences(text, openClawSentenceMinLen, true)

	if len(sentences) != 3 {
		t.Fatalf("unexpected sentence count: got %d want 3", len(sentences))
	}
	if sentences[0] != "Okay." || sentences[1] != "Like this first." || sentences[2] != "Then I'll continue processing." {
		t.Fatalf("unexpected sentence split: %+v", sentences)
	}
	if remaining != "" {
		t.Fatalf("unexpected remaining text: %q", remaining)
	}
}

func TestNormalizeOpenClawSpeechTextStripsMarkdownAndBullets(t *testing.T) {
	raw := "🌤️ **Tomorrow's Weather in Tianjin (March 9)**\n\n- **Temperature**: 3°C ~ 12°C\n- **Weather**: Sunny☀️"

	got := normalizeOpenClawSpeechText(raw)

	if strings.Contains(got, "**") {
		t.Fatalf("unexpected markdown marker in normalized text: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("unexpected newline in normalized text: %q", got)
	}
	if !strings.Contains(got, "Temperature: 3°C ~ 12°C") {
		t.Fatalf("missing normalized temperature segment: %q", got)
	}
	if !strings.Contains(got, "Weather: Sunny☀️") {
		t.Fatalf("missing normalized weather segment: %q", got)
	}
}

func TestExtractOpenClawSentencesGroupsWeatherListIntoLongerSegments(t *testing.T) {
	text := "🌤️ **Tomorrow's Weather in Tianjin (March 9)**\n\n- **Temperature**: 3°C ~ 12°C\n- **Weather**: Sunny☀️\n- **Precipitation**: No rainfall\n- **Humidity**: 15% ~ 38%\n- **Wind**: Southwest wind, wind speed 2-13km/h\n\nTomorrow's weather in Tianjin is nice, mainly sunny, highest temperature 12°C, lowest 3°C."

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
	if !strings.Contains(sentences[0], "Temperature:") || !strings.Contains(sentences[0], "Weather:") {
		t.Fatalf("first sentence still too short: %q", sentences[0])
	}
	if !strings.Contains(sentences[0], "highest temperature 12°C") {
		t.Fatalf("missing summary in final sentence: %q", sentences[0])
	}
}
