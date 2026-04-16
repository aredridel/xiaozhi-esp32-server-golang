package aliyun_qwen3

import (
	"encoding/base64"
	"encoding/json"

	log "xiaozhi-esp32-server-golang/logger"
)

// ClientEvent client-side send event foundation structure
type ClientEvent struct {
	EventID string   `json:"event_id,omitempty"`
	Type    string   `json:"type"`
	Session *Session `json:"session,omitempty"`
	Audio   string   `json:"audio,omitempty"` // Base64 encoded audio data
}

// Session session.update event session config
type Session struct {
	Modalities              []string                 `json:"modalities"`
	InputAudioFormat        string                   `json:"input_audio_format,omitempty"`
	SampleRate              int                      `json:"sample_rate,omitempty"`
	InputAudioTranscription *InputAudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           *TurnDetection           `json:"turn_detection"`
}

// InputAudioTranscription audio transcription config
type InputAudioTranscription struct {
	Language string `json:"language,omitempty"`
}

// TurnDetection VAD config
type TurnDetection struct {
	Type              string  `json:"type,omitempty"`                // "server_vad" or not set
	Threshold         float64 `json:"threshold,omitempty"`           // VAD threshold value
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"` // silence duration (milliseconds)
}

// ServerEvent server-side response event foundation structure
type ServerEvent struct {
	Type            string     `json:"type"`
	EventID         string     `json:"event_id,omitempty"`
	PreviousEventID string     `json:"previous_event_id,omitempty"`
	Session         *Session   `json:"session,omitempty"`
	Item            *Item      `json:"item,omitempty"`
	Transcript      string     `json:"transcript,omitempty"`
	Error           *ErrorInfo `json:"error,omitempty"`
}

// Item session item (e.g. input audio transcription result)
type Item struct {
	ID            int            `json:"id,omitempty"`
	Type          string         `json:"type,omitempty"`
	Status        string         `json:"status,omitempty"`
	Transcription *Transcription `json:"transcription,omitempty"`
}

// Transcription transcription result
type Transcription struct {
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
}

// ErrorInfo error info
type ErrorInfo struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// NewSessionUpdateEvent create session.update event
func NewSessionUpdateEvent(config Config) *ClientEvent {
	session := &Session{
		Modalities:              []string{"text"},
		InputAudioFormat:        config.Format,
		SampleRate:              config.SampleRate,
		InputAudioTranscription: &InputAudioTranscription{Language: config.Language},
	}

	if config.AutoEnd {
		session.TurnDetection = &TurnDetection{
			Type:              "server_vad",
			Threshold:         config.VADThreshold,
			SilenceDurationMs: config.VADSilenceMs,
		}
	} else {
		session.TurnDetection = nil
	}

	event := &ClientEvent{
		EventID: "session_update",
		Type:    "session.update",
		Session: session,
	}

	// debug: print session.update event
	if jsonBytes, err := json.Marshal(event); err == nil {
		log.Debugf("[aliyun_qwen3] session.update JSON: %s", string(jsonBytes))
	}

	return event
}

// NewAudioAppendEvent create input_audio_buffer.append event
func NewAudioAppendEvent(audioData []byte) *ClientEvent {
	encoded := base64.StdEncoding.EncodeToString(audioData)
	return &ClientEvent{
		Type:  "input_audio_buffer.append",
		Audio: encoded,
	}
}

// NewAudioCommitEvent create input_audio_buffer.commit event
func NewAudioCommitEvent() *ClientEvent {
	return &ClientEvent{
		EventID: "audio_commit",
		Type:    "input_audio_buffer.commit",
	}
}

// NewSessionFinishEvent create session.finish event
func NewSessionFinishEvent() *ClientEvent {
	return &ClientEvent{
		EventID: "session_finish",
		Type:    "session.finish",
	}
}

// IsTranscriptionEvent determine if is transcription event
func IsTranscriptionEvent(event *ServerEvent) bool {
	return event.Type == "conversation.item.input_audio_transcription.text" ||
		event.Type == "conversation.item.input_audio_transcription.completed"
}

// IsFinalTranscription determine if is final transcription result
func IsFinalTranscription(event *ServerEvent) bool {
	return event.Type == "conversation.item.input_audio_transcription.completed"
}

// GetTranscriptionText get transcription text
func GetTranscriptionText(event *ServerEvent) string {
	if event == nil {
		return ""
	}
	if event.Item != nil && event.Item.Transcription != nil && event.Item.Transcription.Text != "" {
		return event.Item.Transcription.Text
	}
	if event.Transcript != "" {
		return event.Transcript
	}
	return ""
}
