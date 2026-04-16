package speaker

import (
	"context"
)

// SpeakerProvider voiceprint recognize provider interface
type SpeakerProvider interface {
	// StartStreaming start streaming recognize
	StartStreaming(ctx context.Context, sampleRate int, agentId string) error

	// SendAudioChunk send audio data block
	SendAudioChunk(ctx context.Context, audioData []float32) error

	// FinishAndIdentify complete input and get recognize result
	FinishAndIdentify(ctx context.Context) (*IdentifyResult, error)

	// IsActive check if in active state
	IsActive() bool

	// Close close connection
	Close() error
}

// GetSpeakerProvider get voiceprint recognize provider
func GetSpeakerProvider(config map[string]interface{}) (SpeakerProvider, error) {
	return NewAsrServerProvider(config)
}
