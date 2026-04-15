package eventbus

const (
	TopicAddMessage = "add_message"
	TopicSessionEnd = "session_end"
	TopicExitChat   = "exit_chat" // exitchatevent

	// chat historyrelevantevent（already废弃，unifieduse TopicAddMessage）
	// Deprecated: use TopicAddMessage 替代
	TopicChatHistoryUserMessage      = "chat_history_user_message"      // usermessage(ASRafter) - already废弃
	TopicChatHistoryAssistantMessage = "chat_history_assistant_message" // 机器人回复(LLM+TTSafter) - already废弃
)
