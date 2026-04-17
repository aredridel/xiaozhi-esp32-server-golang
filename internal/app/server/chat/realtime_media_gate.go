package chat

import (
	"context"
	"strings"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/eventbus"
	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	log "xiaozhi-esp32-server-golang/logger"
)

type realtimeMusicControlRule struct {
	action   string
	keywords []string
}

var realtimeMcpAudioControlRules = []realtimeMusicControlRule{
	{
		action: "play_playlist",
		keywords: []string{
			"play playlist",
			"play songs in playlist",
			"play playlist songs",
			"play the playlist",
		},
	},
	{
		action: "enqueue_current",
		keywords: []string{
			"add to playlist",
			"add to playlist",
			"add to playlist",
			"add to playlist",
		},
	},
	{
		action: "resume",
		keywords: []string{
			"resume",
			"resume playing",
			"keep listening",
			"keep playing",
			"continue playing",
		},
	},
	{
		action: "pause",
		keywords: []string{
			"pause",
			"pause first",
			"stop a moment",
		},
	},
	{
		action: "stop",
		keywords: []string{
			"stop playing",
			"stop",
			"stop playing",
			"stop playing",
		},
	},
	{
		action: "next",
		keywords: []string{
			"next",
			"next song",
			"skip to next",
			"skip song",
		},
	},
	{
		action: "prev",
		keywords: []string{
			"previous",
			"previous song",
			"skip to previous",
		},
	},
}

var realtimeMcpAudioExitKeywords = []string{
	"goodbye",
	"bye bye",
	"bye",
	"see you",
	"exit",
	"exit conversation",
	"step down",
}

func normalizeRealtimeMcpAudioText(text string) string {
	return removePunctuation(strings.ToLower(strings.TrimSpace(text)))
}

func detectRealtimeMcpAudioControlAction(text string) string {
	normalizedText := normalizeRealtimeMcpAudioText(text)
	if normalizedText == "" {
		return ""
	}

	for _, rule := range realtimeMcpAudioControlRules {
		for _, keyword := range rule.keywords {
			normalizedKeyword := normalizeRealtimeMcpAudioText(keyword)
			if normalizedKeyword == "" {
				continue
			}
			if strings.Contains(normalizedText, normalizedKeyword) {
				return rule.action
			}
		}
	}

	return ""
}

func isRealtimeMcpAudioExitCommand(text string) bool {
	normalizedText := normalizeRealtimeMcpAudioText(text)
	if normalizedText == "" {
		return false
	}

	for _, keyword := range realtimeMcpAudioExitKeywords {
		normalizedKeyword := normalizeRealtimeMcpAudioText(keyword)
		if normalizedKeyword == "" {
			continue
		}
		if strings.Contains(normalizedText, normalizedKeyword) {
			return true
		}
	}

	return false
}

func isRealtimeMcpAudioSourceType(sourceType MediaSourceType) bool {
	return sourceType == MediaSourceTypeMCPResource || sourceType == MediaSourceTypeInlineAudio
}

func isRealtimeMcpAudioPlaybackState(state MediaPlayerState) bool {
	if !isRealtimeMcpAudioSourceType(state.CurrentSourceType) {
		return false
	}

	return state.Status == play_music.StatusPlaying
}

func (s *ChatSession) hasRealtimeMcpAudioControlContext() bool {
	if s == nil || s.clientState == nil || !s.clientState.IsRealTime() || s.mediaPlayer == nil {
		return false
	}

	return s.mediaPlayer.HasRealtimeMcpAudioControlContext()
}

func (s *ChatSession) isRealtimeMcpAudioGateActive() bool {
	if s == nil || s.clientState == nil || !s.clientState.IsRealTime() || s.mediaPlayer == nil {
		return false
	}

	return s.mediaPlayer.ShouldGateRealtimeMcpAudioASR()
}

func (s *ChatSession) tryHandleRealtimeMcpAudioASR(ctx context.Context, text string) (bool, error) {
	if !s.hasRealtimeMcpAudioControlContext() {
		return false, nil
	}

	if isRealtimeMcpAudioExitCommand(text) {
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "user exited during realtime media playback",
			TriggerType: "realtime_media_exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		log.Infof("device %s realtime media playback gate matched exit command: %s", s.clientState.DeviceID, text)
		return true, nil
	}

	action := detectRealtimeMcpAudioControlAction(text)
	if action != "" {
		_, err := controlMusicPlayback(ctx, s, &MusicPlaybackControlParams{Action: action})
		if err != nil {
			log.Warnf("device %s realtime media playback gate control action failed: action=%s, text=%s, err=%v", s.clientState.DeviceID, action, text, err)
			return true, nil
		}
		log.Infof("device %s realtime media playback gate executed control action: action=%s, text=%s", s.clientState.DeviceID, action, text)
		return true, nil
	}

	if !s.isRealtimeMcpAudioGateActive() {
		return false, nil
	}

	log.Debugf("device %s realtime media playback gate ignoring ASR text: %s", s.clientState.DeviceID, text)
	return true, nil
}
