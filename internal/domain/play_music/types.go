package play_music

import (
	"context"
)

// MusicPlayerInterface music player interface
type MusicPlayerInterface interface {
	// PlayMusicStream from URL play music, return audio stream channel
	PlayMusicStream(ctx context.Context, url string) (chan []byte, error)

	// GetPlayerInfo get player info
	GetPlayerInfo() map[string]interface{}

	// Stop stop player
	Stop() error
}

// MusicPlayerConfig music player config
type MusicPlayerConfig struct {
	FrameDuration int    `json:"frame_duration"` // frame duration(ms), default 20ms
	AudioFormat   string `json:"audio_format"`   // audio format, default "mp3"
}

// DefaultMusicPlayerConfig default music player config
func DefaultMusicPlayerConfig() *MusicPlayerConfig {
	return &MusicPlayerConfig{
		FrameDuration: 20,    // 20ms
		AudioFormat:   "mp3", // MP3 format
	}
}

// ToMap will config convert is map
func (c *MusicPlayerConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"frame_duration": c.FrameDuration,
		"audio_format":   c.AudioFormat,
	}
}

// AudioStreamInfo audio stream info
type AudioStreamInfo struct {
	URL           string `json:"url"`
	Format        string `json:"format"`         // audio format, such as "mp3", "wav"
	SampleRate    int    `json:"sample_rate"`    // sampling rate
	Channels      int    `json:"channels"`       // channel count
	Duration      int64  `json:"duration"`       // duration(millisecond)
	ContentLength int64  `json:"content_length"` // content length(byte)
}

// PlaybackStatus play state
type PlaybackStatus int

const (
	StatusIdle PlaybackStatus = iota
	StatusPlaying
	StatusPaused
	StatusStopped
	StatusError
)

// String return state of char string indicate
func (s PlaybackStatus) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusPlaying:
		return "playing"
	case StatusPaused:
		return "paused"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PlaybackEvent play event
type PlaybackEvent struct {
	Type      string      `json:"type"`      // event type: "started", "progress", "finished", "error"
	Timestamp int64       `json:"timestamp"` // timestamp
	Message   string      `json:"message"`   // event message
	Data      interface{} `json:"data"`      // extra data
}

// StreamingStats streaming play count info
type StreamingStats struct {
	BytesDownloaded int64          `json:"bytes_downloaded"` // already download byte count
	BytesDecoded    int64          `json:"bytes_decoded"`    // already decode byte count
	FramesGenerated int64          `json:"frames_generated"` // already generate frame count
	StartTime       int64          `json:"start_time"`       // start time
	FirstFrameTime  int64          `json:"first_frame_time"` // first frame time
	Status          PlaybackStatus `json:"status"`           // current state
	ErrorCount      int            `json:"error_count"`      // error times count
}
