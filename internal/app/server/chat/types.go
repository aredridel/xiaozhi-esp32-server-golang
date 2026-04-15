package chat

import (
	"context"

	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// ChatSessionOperator 定义 local mcp tool needof ChatSession 操asinterface
// 这个interfaceused for解耦LLMManager and ChatSession，avoidloop依赖
type ChatSessionOperator interface {
	// LocalMcpCloseChat closechatsession
	LocalMcpCloseChat() error

	// LocalMcpClearHistory clear historytoconversation
	LocalMcpClearHistory() error

	// LocalMcpPlayMusic play music
	LocalMcpPlayMusic(ctx context.Context, params *PlayMusicParams) error

	// LocalMcpSwitchDeviceRole 按rolenameswitchdevicerole（supportfuzzymatching）
	LocalMcpSwitchDeviceRole(ctx context.Context, roleName string) (string, error)

	// LocalMcpRestoreDeviceDefaultRole recoverydevicedefaultrole
	LocalMcpRestoreDeviceDefaultRole(ctx context.Context) error

	// LocalMcpSearchKnowledge retrievecurrentagentrelateknowledgelibrary
	LocalMcpSearchKnowledge(ctx context.Context, query string, topK int, knowledgeBaseIDs []uint) ([]config_types.KnowledgeSearchHit, error)

	// LocalMcpControlMusicPlayback controlcurrentsession-levelmediaplay
	LocalMcpControlMusicPlayback(ctx context.Context, params *MusicPlaybackControlParams) (*MusicPlaybackControlResult, error)

	// not来canaccording toneedaddother操as
	// GetDeviceID() string
	// IsActive() bool
}
