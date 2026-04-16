package ten_vad

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	log "xiaozhi-esp32-server-golang/logger"

	. "xiaozhi-esp32-server-golang/internal/domain/vad/inter"
)

// VAD default config
var defaultVADConfig = map[string]interface{}{
	"hop_size":  512,
	"threshold": 0.3,
}

// TenVAD TEN-VAD model implementation
type TenVAD struct {
	handle    unsafe.Pointer
	hopSize   int
	threshold float32
	mu        sync.Mutex
}

// NewTenVAD create TenVAD instance
func NewTenVAD(config map[string]interface{}) (*TenVAD, error) {
	hopSize, ok := config["hop_size"].(int)
	if !ok {
		// try from float64 convert
		if hopSizeFloat, ok := config["hop_size"].(float64); ok {
			hopSize = int(hopSizeFloat)
		} else {
			hopSize = 512 // default values
		}
	}

	threshold, ok := config["threshold"].(float64)
	if !ok {
		// try from float32 convert
		if thresholdFloat32, ok := config["threshold"].(float32); ok {
			threshold = float64(thresholdFloat32)
		} else {
			threshold = 0.3 // default values
		}
	}

	// create TEN-VAD instance
	tenVAD := GetInstance()
	handle, err := tenVAD.CreateInstance(hopSize, float32(threshold))
	if err != nil {
		return nil, fmt.Errorf("create TEN-VAD instance failed: %v", err)
	}

	log.Debugf("create TEN-VAD instance successful, hopSize: %d, threshold: %f", hopSize, threshold)

	return &TenVAD{
		handle:    handle,
		hopSize:   hopSize,
		threshold: float32(threshold),
	}, nil
}

// IsVAD implement VAD interface IsVAD method
func (t *TenVAD) IsVAD(pcmData []float32) (bool, error) {
	return t.IsVADExt(pcmData, 16000, t.hopSize)
}

// IsVADExt implement VAD interface IsVADExt method
func (t *TenVAD) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle == nil {
		return false, errors.New("TEN-VAD instance not initialized")
	}

	if len(pcmData) == 0 {
		return false, nil
	}

	// convert float32 to int16
	// float32 range: -1.0 to 1.0
	// int16 range: -32768 to 32767
	int16Data := make([]int16, len(pcmData))
	for i, f := range pcmData {
		// limit range and convert
		if f > 1.0 {
			f = 1.0
		} else if f < -1.0 {
			f = -1.0
		}
		int16Data[i] = int16(f * 32768.0)
	}

	// process by hopSize frame
	tenVAD := GetInstance()
	hasVoice := false
	voiceFrameCount := 0

	for i := 0; i < len(int16Data); i += t.hopSize {
		end := i + t.hopSize
		if end > len(int16Data) {
			end = len(int16Data)
		}

		frame := int16Data[i:end]
		// if frame length not enough hopSize, need padding or skip
		if len(frame) < t.hopSize {
			// for last frame, if length not enough, can select skip or padding
			// here select skip insufficient frame
			continue
		}

		_, flag, err := tenVAD.ProcessAudio(t.handle, frame)
		if err != nil {
			log.Errorf("TEN-VAD process audio frame failed: %v", err)
			continue
		}

		// flag == 1 indicate detected voice
		if flag == 1 {
			hasVoice = true
			voiceFrameCount++
		}
	}

	// if at least have one frame detected voice, then consider has voice activity
	return hasVoice, nil
}

// Reset reset VAD detector state
func (t *TenVAD) Reset() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// TEN-VAD no need reset, every time process are independent
	// but we can recreate instance to reset state
	// here do nothing, because TEN-VAD has no state
	return nil
}

// Close close and release resource
func (t *TenVAD) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle != nil {
		tenVAD := GetInstance()
		err := tenVAD.DestroyInstance(t.handle)
		if err != nil {
			return fmt.Errorf("destroy TEN-VAD instance failed: %v", err)
		}
		t.handle = nil
	}
	return nil
}

// IsValid inspect resource whether valid
func (t *TenVAD) IsValid() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handle != nil
}

// AcquireVAD create and return TEN-VAD instance (by global resource pool manage)
func AcquireVAD(config map[string]interface{}) (VAD, error) {
	return NewTenVAD(config)
}

// ReleaseVAD release VAD instance
func ReleaseVAD(vad VAD) error {
	if vad != nil {
		return vad.Close()
	}
	return nil
}
