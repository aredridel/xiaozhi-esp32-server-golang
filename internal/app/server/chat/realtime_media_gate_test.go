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
		{text: "continue play for me", want: "resume"},
		{text: "pause first.", want: "pause"},
		{text: "stop playing", want: "stop"},
		{text: "next one", want: "next"},
		{text: "previous one", want: "prev"},
		{text: "play songs in playlist", want: "play_playlist"},
		{text: "add current play to playlist", want: "enqueue_current"},
		{text: "tell me a joke", want: ""},
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
		{text: "then exit conversation", want: true},
		{text: "bye bye", want: true},
		{text: "continueplay", want: false},
		{text: "how is the weather today", want: false},
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
