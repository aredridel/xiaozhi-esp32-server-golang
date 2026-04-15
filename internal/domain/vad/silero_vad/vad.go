package silero_vad

import (
	"errors"
	"sync"
	log "xiaozhi-esp32-server-golang/logger"

	. "xiaozhi-esp32-server-golang/internal/domain/vad/inter"

	"github.com/streamer45/silero-vad-go/speech"
)

// VADdefaultconfig
var defaultVADConfig = map[string]interface{}{
	"threshold":               0.5,
	"min_silence_duration_ms": int64(100),
	"sample_rate":             16000,
	"channels":                1,
	"speech_pad_ms":           60,
}

// globalvariableandinitialize
var (
	// globaldecode器instancepool
	opusDecoderMap sync.Map
	// globalVADdetect器instancepool
	vadDetectorMap sync.Map
	// globalinitializelock
	initMutex sync.Mutex
	// initializeflag
	initialized = false
)

// SileroVAD Silero VADmodelimplement
type SileroVAD struct {
	detector         *speech.Detector
	vadThreshold     float32
	silenceThreshold int64 // 单bit:毫second
	sampleRate       int   // sampling率
	channels         int   // channelcount
	mu               sync.Mutex
}

// NewSileroVAD createSileroVADinstance
func NewSileroVAD(config map[string]interface{}) (*SileroVAD, error) {
	threshold, ok := config["threshold"].(float64)
	if !ok {
		threshold = 0.5 // default阈value
	}

	silenceMs, ok := config["min_silence_duration_ms"].(int64)
	if !ok {
		silenceMs = 800 // default500毫second
	}

	sampleRate, ok := config["sample_rate"].(int)
	if !ok {
		sampleRate = 16000 // defaultsampling率
	}

	channels, ok := config["channels"].(int)
	if !ok {
		channels = 1 // default单声道
	}

	speechPadMs, ok := config["speech_pad_ms"].(int)
	if !ok {
		speechPadMs = 30 // defaultvoicebeforeafter填充
	}

	modelPath, ok := config["model_path"].(string)
	if !ok {
		return nil, errors.New("Missingmodelpathconfig")
	}

	// createvoicedetect器
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

// IsVAD implementVADinterfaceofIsVADmethod
func (s *SileroVAD) IsVAD(pcmData []float32) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	segments, err := s.detector.Detect(pcmData)
	if err != nil {
		log.Errorf("detectfailed: %s", err)
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

// Close closeandreleaseresource
func (s *SileroVAD) Close() error {
	if s.detector != nil {
		return s.detector.Destroy()
	}
	return nil
}

// IsValid inspectresourcewhethervalid
func (s *SileroVAD) IsValid() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detector != nil
}

// AcquireVAD createandreturn Silero VAD instance（byglobalresourcepoolmanage）
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

// Reset resetVADdetect器state
func (s *SileroVAD) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.detector.Reset()
}

// SetThreshold setVADdetect阈value
func (s *SileroVAD) SetThreshold(threshold float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vadThreshold = threshold
	// 注意：silero-vad-go libraryof detector nodirectprovide SetThreshold method
	// only能modifyinstanceof阈value，atdowntimesdetectwheneffective
}
