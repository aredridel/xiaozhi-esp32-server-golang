package chat

import (
	"strings"
	"testing"
)

func TestParseOpenClawWarmupPlanObjects(t *testing.T) {
	got := parseOpenClawWarmupPlan(`[{"text":"我first看adown天气。"},{"text":"天气situation我continue跟进。"},{"text":"这个问题我oratprocess。"},{"text":"天气resultorat路up。"},{"text":"我continue盯着天气。"},{"text":"这edgecontinue核to天气。"},{"text":"天气dataoratupdate。"},{"text":"我continue盯着最新预报。"},{"text":"这edgeoratdo最afteracknowledge。"},{"text":"resultato就告诉你。"},{"text":"我再看a眼天气。"}]`)

	if len(got) != openClawWarmupPlanSize {
		t.Fatalf("unexpected plan size: got %d want %d", len(got), openClawWarmupPlanSize)
	}
	if got[0] != "我first看adown天气。" {
		t.Fatalf("unexpected first line: %q", got[0])
	}
	if got[4] != "我continue盯着天气。" {
		t.Fatalf("unexpected last line: %q", got[4])
	}
	if got[9] != "resultato就告诉你。" {
		t.Fatalf("unexpected tenth line: %q", got[9])
	}
	if got[10] != "我再看a眼天气。" {
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
	got := buildOpenClawWarmupHint("帮我查adownup海今天天气怎么样？")
	if got == "" {
		t.Fatal("expected non-empty hint")
	}
	if strings.Contains(got, "帮我") {
		t.Fatalf("hint should not contain user command: %q", got)
	}
	if len([]rune(got)) > 10 {
		t.Fatalf("hint too long: %q", got)
	}
}

func TestBuildOpenClawWarmupHintWeatherTopic(t *testing.T) {
	got := buildOpenClawWarmupHint("天津after天of天气怎么样？")
	if got != "天津after天of天气" {
		t.Fatalf("unexpected weather hint: %q", got)
	}
}

func TestBuildOpenClawWarmupUserPromptIncludesTimeline(t *testing.T) {
	got := buildOpenClawWarmupUserPrompt("天津after天of天气怎么样？")
	if !strings.Contains(got, "user本轮task：") {
		t.Fatalf("task label missing from prompt: %q", got)
	}
	if !strings.Contains(got, "only能提炼成name词shortphrase“天津after天of天气”") {
		t.Fatalf("topic hint missing from prompt: %q", got)
	}
	if !strings.Contains(got, "nth1second、nth10second、nth20second、nth30second、nth40second、nth50second、nth60second、nth70second、nth80second、nth90second、nth100second") {
		t.Fatalf("timeline missing from prompt: %q", got)
	}
}

func TestFormatOpenClawWarmupTopicWeather(t *testing.T) {
	got := formatOpenClawWarmupTopic("天津after天of天气")
	if got != "天津after天of天气" {
		t.Fatalf("unexpected formatted topic: %q", got)
	}
}

func TestSanitizeOpenClawWarmupTextRejectsUserCommandEcho(t *testing.T) {
	got := sanitizeOpenClawWarmupText("我first看看帮我queryadown。")
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
