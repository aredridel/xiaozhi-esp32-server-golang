package ten_vad

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	log "xiaozhi-esp32-server-golang/logger"

	. "xiaozhi-esp32-server-golang/internal/domain/vad/inter"
)

// VADdefaultconfig
var defaultVADConfig = map[string]interface{}{
	"hop_size":  512,
	"threshold": 0.3,
}

// TenVAD TEN-VADmodelimplement
type TenVAD struct {
	handle    unsafe.Pointer
	hopSize   int
	threshold float32
	mu        sync.Mutex
}

// NewTenVAD createTenVADinstance
func NewTenVAD(config map[string]interface{}) (*TenVAD, error) {
	hopSize, ok := config["hop_size"].(int)
	if !ok {
		// tryfrom float64 convert
		if hopSizeFloat, ok := config["hop_size"].(float64); ok {
			hopSize = int(hopSizeFloat)
		} else {
			hopSize = 512 // default values
		}
	}

	threshold, ok := config["threshold"].(float64)
	if !ok {
		// tryfrom float32 convert
		if thresholdFloat32, ok := config["threshold"].(float32); ok {
			threshold = float64(thresholdFloat32)
		} else {
			threshold = 0.3 // default values
		}
	}

	// createTEN-VADinstance
	tenVAD := GetInstance()
	handle, err := tenVAD.CreateInstance(hopSize, float32(threshold))
	if err != nil {
		return nil, fmt.Errorf("createTEN-VADinstancefailed: %v", err)
	}

	log.Debugf("createTEN-VADinstancesuccessful, hopSize: %d, threshold: %f", hopSize, threshold)

	return &TenVAD{
		handle:    handle,
		hopSize:   hopSize,
		threshold: float32(threshold),
	}, nil
}

// IsVAD implementVADinterfaceofIsVADmethod
func (t *TenVAD) IsVAD(pcmData []float32) (bool, error) {
	return t.IsVADExt(pcmData, 16000, t.hopSize)
}

// IsVADExt implementVADinterfaceofIsVADExtmethod
func (t *TenVAD) IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle == nil {
		return false, errors.New("TEN-VADinstancenot initialized")
	}

	if len(pcmData) == 0 {
		return false, nil
	}

	// will float32 convertis int16
	// float32 range: -1.0 to 1.0
	// int16 range: -32768 to 32767
	int16Data := make([]int16, len(pcmData))
	for i, f := range pcmData {
		// limitrangeandconvert
		if f > 1.0 {
			f = 1.0
		} else if f < -1.0 {
			f = -1.0
		}
		int16Data[i] = int16(f * 32768.0)
	}

	// 按 hopSize minuteframeprocess
	tenVAD := GetInstance()
	hasVoice := false
	voiceFrameCount := 0

	for i := 0; i < len(int16Data); i += t.hopSize {
		end := i + t.hopSize
		if end > len(int16Data) {
			end = len(int16Data)
		}

		frame := int16Data[i:end]
		// ifframelengthno足 hopSize，need填充orskip
		if len(frame) < t.hopSize {
			// to于最afteraframe，iflengthno足，canselectskipor填充
			// 这inselectskipno足offrame
			continue
		}

		_, flag, err := tenVAD.ProcessAudio(t.handle, frame)
		if err != nil {
			log.Errorf("TEN-VADprocessaudio framefailed: %v", err)
			continue
		}

		// flag == 1 indicatedetected voice
		if flag == 1 {
			hasVoice = true
			voiceFrameCount++
		}
	}

	// ifat leasthaveaframedetected voice，then认ishavevoice活动
	return hasVoice, nil
}

// Reset resetVADdetect器state
func (t *TenVAD) Reset() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// TEN-VADnoneedreset，every timeprocessareyesindependentof
	// but我们canrecreateinstance来resetstate
	// 这innodo任何操as，becauseisTEN-VADyesnostateof
	return nil
}

// Close closeandreleaseresource
func (t *TenVAD) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle != nil {
		tenVAD := GetInstance()
		err := tenVAD.DestroyInstance(t.handle)
		if err != nil {
			return fmt.Errorf("destroyTEN-VADinstancefailed: %v", err)
		}
		t.handle = nil
	}
	return nil
}

// IsValid inspectresourcewhethervalid
func (t *TenVAD) IsValid() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handle != nil
}

// AcquireVAD createandreturn TEN-VAD instance（byglobalresourcepoolmanage）
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
