package eventbus

import (
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"

	"github.com/cloudwego/eino/schema"
)

// AddMessageEvent unified of message add event
type AddMessageEvent struct {
	// client-side state
	ClientState *ClientState

	// message content (unified use schema.Message)
	// schema.Message is standard of LLM message format, include:
	// - Role: message role (User/Assistant/System/Tool)
	// - Content: message text content
	// - ToolCalls: tool call list (optional)
	// - ToolCallID: tool call ID (Tool role use)
	Msg schema.Message

	// message ID (used for relate two phase save)
	MessageID string

	// audio data (optional, not belong to schema.Message standard format)
	// first phase: AudioData = nil (only save text)
	// second phase: AudioData != nil (update audio)
	AudioData [][]byte // TTS/ASR audio frame array (Opus format or PCM format)
	AudioSize int      // audio size (byte)

	// audio format info (not belong to schema.Message standard format)
	SampleRate int // sampling rate
	Channels   int // channel count

	// meta data (not belong to schema.Message standard format)
	Timestamp   time.Time
	TTSDuration int // TTS time consumption (millisecond)

	// phase mark
	IsUpdate bool // true=update audio, false=add new message
}
