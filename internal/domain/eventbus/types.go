package eventbus

const (
	TopicAddMessage = "add_message"
	TopicSessionEnd = "session_end"
	TopicExitChat   = "exit_chat" // exitchatevent

	// chat history relevant events (already deprecated, unified use TopicAddMessage)
	// Deprecated: use TopicAddMessage instead
	TopicChatHistoryUserMessage      = "chat_history_user_message"      // user message (after ASR) - already deprecated
	TopicChatHistoryAssistantMessage = "chat_history_assistant_message" // bot reply (after LLM+TTS) - already deprecated
)
