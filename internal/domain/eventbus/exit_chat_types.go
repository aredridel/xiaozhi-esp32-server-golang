package eventbus

import (
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
)

// ExitChatEvent exit chat event
type ExitChatEvent struct {
	// client-side state
	ClientState *ClientState

	// exit reason
	Reason string // "user actively exited", "tool call exit", "timeout exit" etc

	// exit trigger method
	TriggerType string // "exit_words" (exit word detect), "tool_call" (tool call), "timeout" (timeout) etc

	// user input of original text (if have)
	UserText string

	// timestamp
	Timestamp time.Time
}
