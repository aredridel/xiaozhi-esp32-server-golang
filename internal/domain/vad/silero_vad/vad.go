package silero_vad

import (
	"errors"
	"sync"
	log "xiaozhi-esp32-server-golang/logger"

	. "xiaozhi-esp32-server-golang/internal/domain/vad/inter"

	"github.com/streamer45/silero-vad-go/speech"
)

// VAD default config
var defaultVADConfig = map[string]interface{}{
	"threshold":               0.5,
	"min_silence_duration_ms": int64(100),
	"sample_rate":             16000,
	"channels":                1,
	"speech_pad_ms":           60,
}

// global variable and initialize
var (
	// global decoder instance pool
	opusDecoderMap sync.Map
	// global VAD detector instance pool
	vadDetectorMap sync.Map
	// global initialize lock
	initMutex sync.Mutex
	// initialize flag
	initialized = false
)

// SileroVAD Silero VAD model implementation
type SileroVAD struct {
	detector         *speech.Detector
	vadThreshold     float32
	silenceThreshold int64 // unit: millisecond
	sampleRate       int   // sampling rate
	channels         int   // channel count
	mu               sync.Mutex
}

// NewSileroVAD create SileroVAD instance
func NewSileroVAD(config map[string]interface{}) (*SileroVAD, error) {
	threshold, ok := config["threshold"].(float64)
	if !ok {
		threshold = 0.5 // default threshold value
	}

	silenceMs, ok := config["min_silence_duration_ms"].(int64)
	if !ok {
		silenceMs = 800 // default 500 millisecond
	}

	sampleRate, ok := config["sample_rate"].(int)
	if !ok {
		sampleRate = 16000 // default sampling rate
	}

	channels, ok := config["channels"].(int)
	if !ok {
		channels = 1 // default mono
	}

	speechPadMs, ok := config["speech_pad_ms"].(int)
	if !ok {
		speechPadMs = 30 // default voice before after padding
	}

	modelPath, ok := config["model_path"].(string)
	if !ok {
		return nil, errors.New("Missing model path config")
	}

	// create voice detector
	detector, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            modelPath,
		SampleRate:           sampleRate,
		Threshold:            float32(threshold),
		MinSilenceDurationMs: int(silenceMs),
		SpeechPadMs:          speechPadMs,
		LogLevel:             speech.LogLevelWarn,
	})
	if err != nil {
		return nil, err
	}

	return &SileroVAD{
		detector:         detector,
		vadThreshold:     float32(threshold),
		silenceThreshold: silenceMs,
		sampleRate:       sampleRate,
		channels:         channels,
	}, nil
}

func (s *SileroVAD) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	return s.IsVAD(pcmData)
}

// IsVAD implement VAD interface IsVAD method
func (s *SileroVAD) IsVAD(pcmData []float32) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	segments, err := s.detector.Detect(pcmData)
	if err != nil {
		log.Errorf("detect failed: %s", err)
		return false, err
	}

	for _, s := range segments {
		log.Debugf("speech starts at %0.2fs", s.SpeechStartAt)
		if s.SpeechEndAt > 0 {
			log.Debugf("speech ends at %0.2fs", s.SpeechEndAt)
		}
	}

	return len(segments) > 0, nil
}

// Close close and release resource
func (s *SileroVAD) Close() error {
	if s.detector != nil {
		return s.detector.Destroy()
	}
	return nil
}

// IsValid inspect resource whether valid
func (s *SileroVAD) IsValid() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detector != nil
}

// AcquireVAD create and return Silero VAD instance (by global resource pool manage)
func AcquireVAD(config map[string]interface{}) (VAD, error) {
	return NewSileroVAD(config)
}

// ReleaseVAD release VAD instance
func ReleaseVAD(vad VAD) error {
	if vad != nil {
		return vad.Close()
	}
	return nil
}

// Reset reset VAD detector state
func (s *SileroVAD) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.detector.Reset()
}

// SetThreshold set VAD detect threshold value
func (s *SileroVAD) SetThreshold(threshold float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vadThreshold = threshold
	// Note: silero-vad-go library detector does not directly provide SetThreshold method
	// can only modify instance threshold value, effective during next detection
}
