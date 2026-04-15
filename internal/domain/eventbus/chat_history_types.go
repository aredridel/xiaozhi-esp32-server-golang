package eventbus

import (
	"context"
	"time"
)

// UserMessageEvent usermessageevent
// Deprecated: use AddMessageEvent 替代，unifieduse TopicAddMessage event
type UserMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	// ASRresult
	Text      string
	AudioData []byte  // originalaudio data（PCM float32 转byte）
	AudioSize int     // audiosamplingcount

	// audioformatinfo（used forconvertisWAV）
	SampleRate int // sampling率
	Channels   int // channelcount

	// 元data
	Timestamp time.Time
}

// AssistantMessageEvent 机器人回复event
// Deprecated: use AddMessageEvent 替代，unifieduse TopicAddMessage event
type AssistantMessageEvent struct {
	Ctx         context.Context
	SessionID   string
	DeviceID    string
	AgentID     string

	//LLMresult
	Text string

	// TTSresult
	AudioData [][]byte // 合成audio data（Opusformat，audio framearray）
	AudioSize int      // audiosize(byte)

	// audioformatinfo（used forconvertisWAV）
	SampleRate int // sampling率
	Channels   int // channelcount

	// 元data
	TTSDuration int // 毫second
	Timestamp   time.Time
}
