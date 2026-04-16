package eventbus

import (
	"context"
	"time"
)

// UserMessageEvent user message event
// Deprecated: use AddMessageEvent instead, unified use TopicAddMessage event
type UserMessageEvent struct {
	Ctx       context.Context
	SessionID string
	DeviceID  string
	AgentID   string

	// ASR result
	Text      string
	AudioData []byte // original audio data (PCM float32 to byte)
	AudioSize int    // audio sample count

	// audio format info (used for convert to WAV)
	SampleRate int // sampling rate
	Channels   int // channel count

	// metadata
	Timestamp time.Time
}

// AssistantMessageEvent bot reply event
// Deprecated: use AddMessageEvent instead, unified use TopicAddMessage event
type AssistantMessageEvent struct {
	Ctx       context.Context
	SessionID string
	DeviceID  string
	AgentID   string

	// LLM result
	Text string

	// TTS result
	AudioData [][]byte // synthesized audio data (Opus format, audio frame array)
	AudioSize int      // audio size (byte)

	// audio format info (used for convert to WAV)
	SampleRate int // sampling rate
	Channels   int // channel count

	// metadata
	TTSDuration int // millisecond
	Timestamp   time.Time
}
