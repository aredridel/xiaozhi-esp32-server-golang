package eventbus

import (
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"

	"github.com/cloudwego/eino/schema"
)

// AddMessageEvent unifiedofmessageaddevent
type AddMessageEvent struct {
	// client-sidestate
	ClientState *ClientState

	// messageinside容（unifieduse schema.Message）
	// schema.Message yesstandardofLLM messageformat，include：
	// - Role: messagerole（User/Assistant/System/Tool）
	// - Content: messagetextinside容
	// - ToolCalls: toolcalllist（optional）
	// - ToolCallID: toolcallID（Tool roleuse）
	Msg schema.Message

	// messageID（used forrelate两阶段save）
	MessageID string

	// audio data（optional，nobelong to schema.Message standardformat）
	// first阶段：AudioData = nil（onlysavetext）
	// nth二阶段：AudioData != nil（updateaudio）
	AudioData [][]byte // TTS/ASR audio framearray（OpusformatorPCMformat）
	AudioSize int      // audiosize（byte）

	// audioformatinfo（nobelong to schema.Message standardformat）
	SampleRate int // sampling率
	Channels   int // channelcount

	// 元data（nobelong to schema.Message standardformat）
	Timestamp   time.Time
	TTSDuration int // TTS time consumption（毫second）

	// 阶段标识
	IsUpdate bool // true=updateaudio，false=新增message
}
