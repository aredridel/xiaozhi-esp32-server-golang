package speaker

import (
	"context"
)

// SpeakerProvider voiceprintrecognizeprovide者interface
type SpeakerProvider interface {
	// StartStreaming startstreaming recognize
	StartStreaming(ctx context.Context, sampleRate int, agentId string) error

	// SendAudioChunk sendaudio datablock
	SendAudioChunk(ctx context.Context, audioData []float32) error

	// FinishAndIdentify completeinputandgetrecognizeresult
	FinishAndIdentify(ctx context.Context) (*IdentifyResult, error)

	// IsActive check if处于activatestate
	IsActive() bool

	// Close closejoin
	Close() error
}

// GetSpeakerProvider getvoiceprintrecognizeprovide者
func GetSpeakerProvider(config map[string]interface{}) (SpeakerProvider, error) {
	return NewAsrServerProvider(config)
}
