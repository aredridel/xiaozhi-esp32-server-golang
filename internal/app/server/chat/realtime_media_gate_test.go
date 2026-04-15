package chat

import (
	"testing"

	"xiaozhi-esp32-server-golang/internal/domain/play_music"
)

func TestDetectRealtimeMcpAudioControlAction(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{text: "给我continueplay", want: "resume"},
		{text: "firstpauseadown。", want: "pause"},
		{text: "stopplay吧", want: "stop"},
		{text: "downafirst", want: "next"},
		{text: "upafirst", want: "prev"},
		{text: "playplaylistinofsong", want: "play_playlist"},
		{text: "把currentplayadd toplaylist", want: "enqueue_current"},
		{text: "帮我讲个笑conversation", want: ""},
	}

	for _, tc := range cases {
		got := detectRealtimeMcpAudioControlAction(tc.text)
		if got != tc.want {
			t.Fatalf("detectRealtimeMcpAudioControlAction(%q)=%q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestIsRealtimeMcpAudioExitCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "goodbye", want: true},
		{text: "那就exittoconversation", want: true},
		{text: "拜拜啦", want: true},
		{text: "continueplay", want: false},
		{text: "今天天气怎么样", want: false},
	}

	for _, tc := range cases {
		got := isRealtimeMcpAudioExitCommand(tc.text)
		if got != tc.want {
			t.Fatalf("isRealtimeMcpAudioExitCommand(%q)=%v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestIsRealtimeMcpAudioPlaybackState(t *testing.T) {
	if !isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPlaying,
		CurrentSourceType: MediaSourceTypeMCPResource,
	}) {
		t.Fatal("expected mcp playing state to be gated")
	}

	if !isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPaused,
		CurrentSourceType: MediaSourceTypeInlineAudio,
	}) {
		t.Fatal("expected inline paused state to be gated")
	}

	if isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusStopped,
		CurrentSourceType: MediaSourceTypeMCPResource,
	}) {
		t.Fatal("expected stopped state not to be gated")
	}

	if isRealtimeMcpAudioPlaybackState(MediaPlayerState{
		Status:            play_music.StatusPlaying,
		CurrentSourceType: MediaSourceTypeHTTPURL,
	}) {
		t.Fatal("expected non-mcp source not to be gated")
	}
}
