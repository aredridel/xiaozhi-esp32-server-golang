package webrtc_vad

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWebRTCVAD testcreate WebRTC VAD instance
func TestNewWebRTCVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, DefaultSampleRate, webrtcVAD.sampleRate)
	assert.Equal(t, DefaultMode, webrtcVAD.mode)
	assert.False(t, webrtcVAD.initialized)

	// cleanupresource
	err := vad.Close()
	assert.NoError(t, err)
}

// TestNewWebRTCVADWithConfig testuseconfigcreate WebRTC VAD instance
func TestNewWebRTCVADWithConfig(t *testing.T) {
	// testvalidconfig
	vad, err := NewWebRTCVADWithConfig(8000, 1)
	require.NoError(t, err)
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, 8000, webrtcVAD.sampleRate)
	assert.Equal(t, 1, webrtcVAD.mode)

	err = vad.Close()
	assert.NoError(t, err)

	// testinvalidsampling率
	vad, err = NewWebRTCVADWithConfig(22050, 1)
	assert.Error(t, err)
	assert.Nil(t, vad)

	// testinvalidpattern
	vad, err = NewWebRTCVADWithConfig(16000, 5)
	assert.Error(t, err)
	assert.Nil(t, vad)
}

// TestWebRTCVAD_IsVAD testvoice活动detect
func TestWebRTCVAD_IsVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// testemptydata
	isActive, err := vad.IsVAD([]float32{})
	assert.NoError(t, err)
	assert.False(t, isActive)

	// testsilencedata（全zero）
	silentData := make([]float32, 1600) // 100ms at 16kHz
	isActive, err = vad.IsVAD(silentData)
	assert.NoError(t, err)
	// silencedata通常nowillbedetectisvoice活动，but这取决于 VAD ofimplement

	// test合成voicedata（positive弦波）
	speechData := generateSineWave(16000, 440, 1.0, 0.5) // 1second 440Hz positive弦波
	isActive, err = vad.IsVAD(speechData)
	assert.NoError(t, err)
	// positive弦波maybedetectisvoice活动，but这取决于 VAD algorithm

	// testdataamountno足aframeofsituation
	shortData := make([]float32, 100) // 少于aframeofdata
	isActive, err = vad.IsVAD(shortData)
	assert.NoError(t, err)
	assert.False(t, isActive)
}

// TestWebRTCVAD_Reset testresetfunction
func TestWebRTCVAD_Reset(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// initialize之beforereset
	err := vad.Reset()
	assert.NoError(t, err)

	// firstuse VAD performinitialize
	testData := make([]float32, 1600) // 100ms at 16kHz
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	// initializeafterreset
	err = vad.Reset()
	assert.NoError(t, err)
}

// TestWebRTCVAD_Close testclosefunction
func TestWebRTCVAD_Close(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	// not initializedwhenclose
	err := vad.Close()
	assert.NoError(t, err)

	// initializeafterclose
	testData := make([]float32, 1600)
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	err = vad.Close()
	assert.NoError(t, err)

	// 重复close
	err = vad.Close()
	assert.NoError(t, err)
}

// TestWebRTCVAD_SetMode testsetpattern
func TestWebRTCVAD_SetMode(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// testvalidpattern
	for mode := 0; mode <= 3; mode++ {
		err := webrtcVAD.SetMode(mode)
		assert.NoError(t, err)
		assert.Equal(t, mode, webrtcVAD.GetMode())
	}

	// testinvalidpattern
	err := webrtcVAD.SetMode(-1)
	assert.Error(t, err)

	err = webrtcVAD.SetMode(4)
	assert.Error(t, err)
}

// TestWebRTCVAD_SetSampleRate testsetsampling率
func TestWebRTCVAD_SetSampleRate(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// testvalidsampling率
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		err := webrtcVAD.SetSampleRate(rate)
		assert.NoError(t, err)
		assert.Equal(t, rate, webrtcVAD.GetSampleRate())
	}

	// testinvalidsampling率
	err := webrtcVAD.SetSampleRate(22050)
	assert.Error(t, err)

	err = webrtcVAD.SetSampleRate(44100)
	assert.Error(t, err)
}

// TestFloat32ToPCMBytes testdatatypeconvert
func TestFloat32ToPCMBytes(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// testboundaryvalue
	testData := []float32{-1.0, 0.0, 1.0, 1.5, -1.5}
	pcmBytes := webrtcVAD.float32ToPCMBytes(testData)

	assert.Equal(t, len(testData)*2, len(pcmBytes))

	// inspectconvertresult
	// -1.0 -> -32768
	// 0.0 -> 0
	// 1.0 -> 32767
	// 1.5 -> 32767 (clipped)
	// -1.5 -> -32768 (clipped)
}

// TestIsValidSampleRate testsampling率validate
func TestIsValidSampleRate(t *testing.T) {
	// validsampling率
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		assert.True(t, isValidSampleRate(rate))
	}

	// invalidsampling率
	invalidRates := []int{11025, 22050, 44100, 96000}
	for _, rate := range invalidRates {
		assert.False(t, isValidSampleRate(rate))
	}
}

// generateSineWave generatepositive弦波dataused fortest
func generateSineWave(sampleRate int, frequency float64, duration float64, amplitude float64) []float32 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = float32(amplitude * math.Sin(2*math.Pi*frequency*t))
	}

	return samples
}
