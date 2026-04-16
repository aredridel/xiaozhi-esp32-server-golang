package play_music

import (
	"context"
)

// MusicPlayerInterface music playerinterface
type MusicPlayerInterface interface {
	// PlayMusicStream fromURLplay music，returnaudio streamchannel
	PlayMusicStream(ctx context.Context, url string) (chan []byte, error)

	// GetPlayerInfo getplayerinfo
	GetPlayerInfo() map[string]interface{}

	// Stop stopplayer
	Stop() error
}

// MusicPlayerConfig music playerconfig
type MusicPlayerConfig struct {
	FrameDuration int    `json:"frame_duration"` // frameduration(ms)，default20ms
	AudioFormat   string `json:"audio_format"`   // audioformat，default"mp3"
}

// DefaultMusicPlayerConfig defaultmusic playerconfig
func DefaultMusicPlayerConfig() *MusicPlayerConfig {
	return &MusicPlayerConfig{
		FrameDuration: 20,    // 20ms
		AudioFormat:   "mp3", // MP3format
	}
}

// ToMap willconfigconvertismap
func (c *MusicPlayerConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"frame_duration": c.FrameDuration,
		"audio_format":   c.AudioFormat,
	}
}

// AudioStreamInfo audio streaminfo
type AudioStreamInfo struct {
	URL           string `json:"url"`
	Format        string `json:"format"`         // audioformat，如 "mp3", "wav"
	SampleRate    int    `json:"sample_rate"`    // sampling率
	Channels      int    `json:"channels"`       // 声道count
	Duration      int64  `json:"duration"`       // duration(毫second)
	ContentLength int64  `json:"content_length"` // inside容length(byte)
}

// PlaybackStatus playstate
type PlaybackStatus int

const (
	StatusIdle PlaybackStatus = iota
	StatusPlaying
	StatusPaused
	StatusStopped
	StatusError
)

// String returnstateofcharstringindicate
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

// PlaybackEvent playevent
type PlaybackEvent struct {
	Type      string      `json:"type"`      // eventtype: "started", "progress", "finished", "error"
	Timestamp int64       `json:"timestamp"` // timestamp
	Message   string      `json:"message"`   // eventmessage
	Data      interface{} `json:"data"`      // 额outsidedata
}

// StreamingStats streamingplaycountinfo
type StreamingStats struct {
	BytesDownloaded int64          `json:"bytes_downloaded"` // alreadydownloadbytecount
	BytesDecoded    int64          `json:"bytes_decoded"`    // alreadydecodebytecount
	FramesGenerated int64          `json:"frames_generated"` // alreadygenerateframecount
	StartTime       int64          `json:"start_time"`       // starttime
	FirstFrameTime  int64          `json:"first_frame_time"` // firstframetime
	Status          PlaybackStatus `json:"status"`           // currentstate
	ErrorCount      int            `json:"error_count"`      // errortimescount
}
