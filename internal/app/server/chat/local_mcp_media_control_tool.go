package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	log "xiaozhi-esp32-server-golang/logger"
)

const localMcpMusicControlToolName = "control_music_playback"

func init() {
	if err := RegisterLocalMcpFunc(
		localMcpMusicControlToolName,
		"when user wants to control current device playing music or audio must use. For \"continue play\" \"resume play\" \"continue listen\" \"continue play\" \"pause\" \"stop\" \"previous one\" \"next one\" \"play playlist\" \"play playlist songs\" \"play playlist\" \"add current play to playlist\" etc commands, must call this tool, cannot only do text reply. only when user wants to play new song, search song or play specific music, do not use this tool.",
		MusicPlaybackControlParams{},
		musicPlaybackControlHandler,
	); err != nil {
		log.Errorf("register media control local MCP tool failed: %v", err)
	}
}

func musicPlaybackControlHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Infof("execute media control tool, args=%s", argumentsInJSON)

	var params MusicPlaybackControlParams
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse(localMcpMusicControlToolName, "parameter parsing failed", "PARSE_ERROR", "please inspect action parameter format whether correct")
			return response.ToJSON()
		}
	}

	chatSessionOperatorValue := ctx.Value("chat_session_operator")
	if chatSessionOperatorValue == nil {
		return "", fmt.Errorf("chat_session_operator not found in context")
	}

	chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator)
	if !ok {
		return "", fmt.Errorf("chat_session_operator obtained from context is not of type ChatSessionOperator")
	}

	result, err := chatSessionOperator.LocalMcpControlMusicPlayback(ctx, &params)
	if err != nil {
		log.Errorf("media control failed: %v", err)
		response := NewErrorResponse(localMcpMusicControlToolName, fmt.Sprintf("media control failed: %v", err), "MEDIA_CONTROL_FAILED", "please inspect current play state after retry")
		return response.ToJSON()
	}
	if result == nil {
		result = &MusicPlaybackControlResult{
			Action:          normalizeMusicPlaybackAction(params.Action),
			Status:          "unknown",
			SilenceResponse: true,
		}
	}

	action := normalizeMusicPlaybackAction(params.Action)
	if result != nil && result.Action != "" {
		action = result.Action
	}

	response := NewActionResponse(
		localMcpMusicControlToolName,
		action,
		buildMusicPlaybackControlMessage(result),
		result.Status,
		false,
	)
	response.NoFurtherResponse = result.SilenceResponse
	response.SilenceLLM = result.SilenceResponse
	response.Metadata = buildMusicPlaybackControlMetadata(result)

	return response.ToJSON()
}

func buildMusicPlaybackControlMessage(result *MusicPlaybackControlResult) string {
	if result == nil {
		return "media control already complete"
	}

	switch result.Action {
	case "resume":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already continue play: %s", result.CurrentTitle)
		}
		return "already continue play"
	case "pause":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already pause: %s", result.CurrentTitle)
		}
		return "already pause play"
	case "stop":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already stop: %s", result.CurrentTitle)
		}
		return "already stop play"
	case "prev":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already switch to previous: %s", result.CurrentTitle)
		}
		return "already switch to previous"
	case "next":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already switch to next: %s", result.CurrentTitle)
		}
		return "already switch to next"
	case "play_playlist":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already start play playlist: %s", result.CurrentTitle)
		}
		return "already start play playlist"
	case "enqueue_current":
		if result.AddedTitle != "" {
			return fmt.Sprintf("already add current play source to playlist: %s", result.AddedTitle)
		}
		return "already add current play source to playlist"
	default:
		return "media control already complete"
	}
}

func buildMusicPlaybackControlMetadata(result *MusicPlaybackControlResult) map[string]string {
	if result == nil {
		return nil
	}

	metadata := map[string]string{
		"action":          result.Action,
		"status":          result.Status,
		"current_title":   result.CurrentTitle,
		"current_index":   strconv.Itoa(result.CurrentIndex),
		"playlist_length": strconv.Itoa(result.PlaylistLength),
		"current_source":  result.CurrentSource,
		"position_ms":     strconv.FormatInt(result.PositionMs, 10),
	}
	if result.AddedTitle != "" {
		metadata["added_title"] = result.AddedTitle
	}
	return metadata
}
