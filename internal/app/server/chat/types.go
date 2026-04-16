package chat

import (
	"context"

	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// ChatSessionOperator define local mcp tool need of ChatSession operation interface
// this interface used for decouple LLMManager and ChatSession, avoid loop dependency
type ChatSessionOperator interface {
	// LocalMcpCloseChat close chat session
	LocalMcpCloseChat() error

	// LocalMcpClearHistory clear history to conversation
	LocalMcpClearHistory() error

	// LocalMcpPlayMusic play music
	LocalMcpPlayMusic(ctx context.Context, params *PlayMusicParams) error

	// LocalMcpSwitchDeviceRole switch device role by role name (support fuzzy matching)
	LocalMcpSwitchDeviceRole(ctx context.Context, roleName string) (string, error)

	// LocalMcpRestoreDeviceDefaultRole restore device default role
	LocalMcpRestoreDeviceDefaultRole(ctx context.Context) error

	// LocalMcpSearchKnowledge retrieve current agent relate knowledge library
	LocalMcpSearchKnowledge(ctx context.Context, query string, topK int, knowledgeBaseIDs []uint) ([]config_types.KnowledgeSearchHit, error)

	// LocalMcpControlMusicPlayback control current session-level media play
	LocalMcpControlMusicPlayback(ctx context.Context, params *MusicPlaybackControlParams) (*MusicPlaybackControlResult, error)

	// can add other operations according to need
	// GetDeviceID() string
	// IsActive() bool
}
