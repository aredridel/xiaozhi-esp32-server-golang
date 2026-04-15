package chat

import (
	"context"

	"xiaozhi-esp32-server-golang/internal/domain/speaker"
)

// SpeakerManager voiceprintrecognizemanage器（package装 SpeakerProvider）
type SpeakerManager struct {
	provider speaker.SpeakerProvider
}

type peekableSpeakerProvider interface {
	PeekAndIdentify(ctx context.Context, requestID string) (*speaker.IdentifyResult, bool, error)
}

// NewSpeakerManager createvoiceprintmanage器
func NewSpeakerManager(provider speaker.SpeakerProvider) *SpeakerManager {
	return &SpeakerManager{
		provider: provider,
	}
}

// StartStreaming startstreaming recognize
func (sm *SpeakerManager) StartStreaming(ctx context.Context, sampleRate int, agentId string) error {
	return sm.provider.StartStreaming(ctx, sampleRate, agentId)
}

// SendAudioChunk sendaudio chunk
func (sm *SpeakerManager) SendAudioChunk(ctx context.Context, pcmData []float32) error {
	return sm.provider.SendAudioChunk(ctx, pcmData)
}

// FinishAndIdentify completerecognizeandgetresult
func (sm *SpeakerManager) FinishAndIdentify(ctx context.Context) (*speaker.IdentifyResult, error) {
	return sm.provider.FinishAndIdentify(ctx)
}

// Close closevoiceprintmanage器
func (sm *SpeakerManager) Close() error {
	return sm.provider.Close()
}

// IsActive check if处于activatestate
func (sm *SpeakerManager) IsActive() bool {
	return sm.provider.IsActive()
}

// PeekAndIdentify getvoiceprintmiddlerecognizeresult（noendcurrent轮times）
// return: recognizeresult, whetherbeserver-sidedebounce, error
func (sm *SpeakerManager) PeekAndIdentify(ctx context.Context, requestID string) (*speaker.IdentifyResult, bool, error) {
	if sm == nil || sm.provider == nil {
		return nil, false, nil
	}
	peekProvider, ok := sm.provider.(peekableSpeakerProvider)
	if !ok {
		return nil, false, nil
	}
	return peekProvider.PeekAndIdentify(ctx, requestID)
}
