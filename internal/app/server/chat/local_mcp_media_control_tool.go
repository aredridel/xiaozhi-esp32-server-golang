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
		"whenuser要controlcurrentdeviceisplayof音乐oraudiowhen必须use。to于“continueplay”“recoveryplay”“continuelisten”“接着play”“pause”“stop”“upafirst”“downafirst”“playplaylist”“playplaylistinofsong”“playplaylist”“把currentplayadd toplaylist”etc指令，必须callthistool，cannotonlydo文字回复。onlywhenuserwantplaynewsong、searchsongorpoint播concrete音乐when，no要usethistool。",
		MusicPlaybackControlParams{},
		musicPlaybackControlHandler,
	); err != nil {
		log.Errorf("registermediacontrollocalMCPtoolfailed: %v", err)
	}
}

func musicPlaybackControlHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Infof("executemediacontroltool, args=%s", argumentsInJSON)

	var params MusicPlaybackControlParams
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse(localMcpMusicControlToolName, "parameter parsing failed", "PARSE_ERROR", "pleaseinspect action parameterformatwhetherpositive确")
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
		log.Errorf("mediacontrolfailed: %v", err)
		response := NewErrorResponse(localMcpMusicControlToolName, fmt.Sprintf("mediacontrolfailed: %v", err), "MEDIA_CONTROL_FAILED", "pleaseinspectcurrentplaystateafterretry")
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
		return "mediacontrolalreadycomplete"
	}

	switch result.Action {
	case "resume":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("alreadycontinueplay：%s", result.CurrentTitle)
		}
		return "alreadycontinueplay"
	case "pause":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("alreadypause：%s", result.CurrentTitle)
		}
		return "alreadypauseplay"
	case "stop":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("alreadystop：%s", result.CurrentTitle)
		}
		return "alreadystopplay"
	case "prev":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already切toupafirst：%s", result.CurrentTitle)
		}
		return "already切toupafirst"
	case "next":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("already切todownafirst：%s", result.CurrentTitle)
		}
		return "already切todownafirst"
	case "play_playlist":
		if result.CurrentTitle != "" {
			return fmt.Sprintf("alreadystartplayplaylist：%s", result.CurrentTitle)
		}
		return "alreadystartplayplaylist"
	case "enqueue_current":
		if result.AddedTitle != "" {
			return fmt.Sprintf("alreadywillcurrentplay源add toplaylist：%s", result.AddedTitle)
		}
		return "alreadywillcurrentplay源add toplaylist"
	default:
		return "mediacontrolalreadycomplete"
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
