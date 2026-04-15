package eventbus

import (
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
)

// ExitChatEvent exitchatevent
type ExitChatEvent struct {
	// client-sidestate
	ClientState *ClientState

	// exitreason
	Reason string // "user actively exited"、"toolcallexit"、"timeoutexit" etc

	// exittrigger方式
	TriggerType string // "exit_words"（exit词detect）、"tool_call"（toolcall）、"timeout"（timeout）etc

	// userinputoforiginaltext（ifhave）
	UserText string

	// timestamp
	Timestamp time.Time
}
