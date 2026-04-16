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
			"play playlist songs",
			"play playlist",
			"playlist",
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
			"continue play",
			"resume play",
			"continue listen",
			"continue play",
			"continue playing",
		},
	},
	{
		action: "pause",
		keywords: []string{
			"pause",
			"first pause",
			"first stop",
		},
	},
	{
		action: "stop",
		keywords: []string{
			"stop play",
			"stop",
			"stop playing",
			"don't play",
		},
	},
	{
		action: "next",
		keywords: []string{
			"next one",
			"next song",
			"switch to next",
			"next song",
		},
	},
	{
		action: "prev",
		keywords: []string{
			"previous one",
			"previous song",
			"switch to previous",
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
	"quit",
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

func isRealtimeMcpAudioPlaybackState(state MediaPlayerState) bool {
	if state.CurrentSourceType != MediaSourceTypeMCPResource && state.CurrentSourceType != MediaSourceTypeInlineAudio {
		return false
	}

	return state.Status == play_music.StatusPlaying || state.Status == play_music.StatusPaused
}

func (s *ChatSession) isRealtimeMcpAudioGateActive() bool {
	if s == nil || s.clientState == nil || !s.clientState.IsRealTime() || s.mediaPlayer == nil {
		return false
	}

	state := s.mediaPlayer.GetState()
	return isRealtimeMcpAudioPlaybackState(state)
}

func (s *ChatSession) tryHandleRealtimeMcpAudioASR(ctx context.Context, text string) (bool, error) {
	if !s.isRealtimeMcpAudioGateActive() {
		return false, nil
	}

	if isRealtimeMcpAudioExitCommand(text) {
		eventbus.Get().Publish(eventbus.TopicExitChat, &eventbus.ExitChatEvent{
			ClientState: s.clientState,
			Reason:      "realtime media play user exit",
			TriggerType: "realtime_media_exit_words",
			UserText:    text,
			Timestamp:   time.Now(),
		})
		log.Infof("device %s realtime media play gate exit command: %s", s.clientState.DeviceID, text)
		return true, nil
	}

	action := detectRealtimeMcpAudioControlAction(text)
	if action != "" {
		_, err := controlMusicPlayback(ctx, s, &MusicPlaybackControlParams{Action: action})
		if err != nil {
			log.Warnf("device %s realtime media play gate execute control action failed: action=%s, text=%s, err=%v", s.clientState.DeviceID, action, text, err)
			return true, nil
		}
		log.Infof("device %s realtime media play gate execute control action: action=%s, text=%s", s.clientState.DeviceID, action, text)
		return true, nil
	}

	log.Debugf("device %s realtime media play gate ignore ASR text: %s", s.clientState.DeviceID, text)
	return true, nil
}
