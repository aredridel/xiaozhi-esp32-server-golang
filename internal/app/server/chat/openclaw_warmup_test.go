package chat

import (
	"strings"
	"testing"
)

func TestParseOpenClawWarmupPlanObjects(t *testing.T) {
	got := parseOpenClawWarmupPlan(`[{"text":"Let me check the weather."},{"text":"Still following up on the weather."},{"text":"I'm working on this issue."},{"text":"Weather results coming soon."},{"text":"Still monitoring the weather."},{"text":"Continuing to verify the weather here."},{"text":"Weather data is being updated."},{"text":"Still watching the latest forecast."},{"text":"Will do final confirmation here."},{"text":"Will tell you when results are out."},{"text":"Let me check the weather again."}]`)

	if len(got) != openClawWarmupPlanSize {
		t.Fatalf("unexpected plan size: got %d want %d", len(got), openClawWarmupPlanSize)
	}
	if got[0] != "Let me check the weather." {
		t.Fatalf("unexpected first line: %q", got[0])
	}
	if got[4] != "Still monitoring the weather." {
		t.Fatalf("unexpected fifth line: %q", got[4])
	}
	if got[9] != "Will tell you when results are out." {
		t.Fatalf("unexpected tenth line: %q", got[9])
	}
	if got[10] != "Let me check the weather again." {
		t.Fatalf("unexpected eleventh line: %q", got[10])
	}
}

func TestParseOpenClawWarmupPlanReturnsEmptyOnInvalidJSON(t *testing.T) {
	got := parseOpenClawWarmupPlan("not-json")

	for idx, line := range got {
		if line != "" {
			t.Fatalf("expected empty line at %d, got %q", idx, line)
		}
	}
}

func TestBuildOpenClawWarmupHint(t *testing.T) {
	got := buildOpenClawWarmupHint("Help me check Shanghai weather today?")
	if got == "" {
		t.Fatal("expected non-empty hint")
	}
	if strings.Contains(got, "help me") || strings.Contains(got, "help") {
		t.Fatalf("hint should not contain user command: %q", got)
	}
	if len([]rune(got)) > 10 {
		t.Fatalf("hint too long: %q", got)
	}
}

func TestBuildOpenClawWarmupHintWeatherTopic(t *testing.T) {
	got := buildOpenClawWarmupHint("How's the weather in Tianjin the day after tomorrow?")
	if got != "Tianjin weather" {
		t.Fatalf("unexpected weather hint: %q", got)
	}
}

func TestBuildOpenClawWarmupUserPromptIncludesTimeline(t *testing.T) {
	got := buildOpenClawWarmupUserPrompt("How's the weather in Tianjin the day after tomorrow?")
	if !strings.Contains(got, "User's current task:") {
		t.Fatalf("task label missing from prompt: %q", got)
	}
	if !strings.Contains(got, "If need to mention topic, can only extract into noun short phrase") {
		t.Fatalf("topic hint missing from prompt: %q", got)
	}
	if !strings.Contains(got, "1st second, 10th second, 20th second, 30th second, 40th second, 50th second, 60th second, 70th second, 80th second, 90th second, 100th second") {
		t.Fatalf("timeline missing from prompt: %q", got)
	}
}

func TestFormatOpenClawWarmupTopicWeather(t *testing.T) {
	got := formatOpenClawWarmupTopic("Tianjin weather")
	if got != "Tianjin weather" {
		t.Fatalf("unexpected formatted topic: %q", got)
	}
}

func TestSanitizeOpenClawWarmupTextRejectsUserCommandEcho(t *testing.T) {
	got := sanitizeOpenClawWarmupText("Let me see help me check.")
	if got != "" {
		t.Fatalf("expected invalid warmup text to be rejected, got %q", got)
	}
}

func TestTakeWarmupSegmentStartFlagOnlyMarksFirstWarmupSentence(t *testing.T) {
	task := &openClawWarmupTask{nextWarmupSegmentIsStart: true}

	if !task.takeWarmupSegmentStartFlag() {
		t.Fatal("expected first warmup sentence to carry start flag")
	}
	if task.takeWarmupSegmentStartFlag() {
		t.Fatal("expected subsequent warmup sentence to clear start flag")
	}
}
