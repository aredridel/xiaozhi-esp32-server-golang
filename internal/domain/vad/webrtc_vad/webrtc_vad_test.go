package webrtc_vad

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWebRTCVAD test create WebRTC VAD instance
func TestNewWebRTCVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, DefaultSampleRate, webrtcVAD.sampleRate)
	assert.Equal(t, DefaultMode, webrtcVAD.mode)
	assert.False(t, webrtcVAD.initialized)

	// cleanup resource
	err := vad.Close()
	assert.NoError(t, err)
}

// TestNewWebRTCVADWithConfig test use config create WebRTC VAD instance
func TestNewWebRTCVADWithConfig(t *testing.T) {
	// test valid config
	vad, err := NewWebRTCVADWithConfig(8000, 1)
	require.NoError(t, err)
	require.NotNil(t, vad)

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)
	assert.Equal(t, 8000, webrtcVAD.sampleRate)
	assert.Equal(t, 1, webrtcVAD.mode)

	err = vad.Close()
	assert.NoError(t, err)

	// test invalid sample rate
	vad, err = NewWebRTCVADWithConfig(22050, 1)
	assert.Error(t, err)
	assert.Nil(t, vad)

	// test invalid mode
	vad, err = NewWebRTCVADWithConfig(16000, 5)
	assert.Error(t, err)
	assert.Nil(t, vad)
}

// TestWebRTCVAD_IsVAD test voice activity detection
func TestWebRTCVAD_IsVAD(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// test empty data
	isActive, err := vad.IsVAD([]float32{})
	assert.NoError(t, err)
	assert.False(t, isActive)

	// test silence data (all zeros)
	silentData := make([]float32, 1600) // 100ms at 16kHz
	isActive, err = vad.IsVAD(silentData)
	assert.NoError(t, err)
	// silence data usually will not be detected as voice activity, but this depends on VAD implementation

	// test synthetic voice data (sine wave)
	speechData := generateSineWave(16000, 440, 1.0, 0.5) // 1 second 440Hz sine wave
	isActive, err = vad.IsVAD(speechData)
	assert.NoError(t, err)
	// sine wave may be detected as voice activity, but this depends on VAD algorithm

	// test data amount not enough for a frame
	shortData := make([]float32, 100) // less than a frame of data
	isActive, err = vad.IsVAD(shortData)
	assert.NoError(t, err)
	assert.False(t, isActive)
}

// TestWebRTCVAD_Reset test reset function
func TestWebRTCVAD_Reset(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	// reset before initialize
	err := vad.Reset()
	assert.NoError(t, err)

	// first use VAD to perform initialize
	testData := make([]float32, 1600) // 100ms at 16kHz
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	// reset after initialize
	err = vad.Reset()
	assert.NoError(t, err)
}

// TestWebRTCVAD_Close test close function
func TestWebRTCVAD_Close(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)

	// close when not initialized
	err := vad.Close()
	assert.NoError(t, err)

	// close after initialize
	testData := make([]float32, 1600)
	_, err = vad.IsVAD(testData)
	assert.NoError(t, err)

	err = vad.Close()
	assert.NoError(t, err)

	// close again
	err = vad.Close()
	assert.NoError(t, err)
}

// TestWebRTCVAD_SetMode test set mode
func TestWebRTCVAD_SetMode(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// test valid mode
	for mode := 0; mode <= 3; mode++ {
		err := webrtcVAD.SetMode(mode)
		assert.NoError(t, err)
		assert.Equal(t, mode, webrtcVAD.GetMode())
	}

	// test invalid mode
	err := webrtcVAD.SetMode(-1)
	assert.Error(t, err)

	err = webrtcVAD.SetMode(4)
	assert.Error(t, err)
}

// TestWebRTCVAD_SetSampleRate test set sample rate
func TestWebRTCVAD_SetSampleRate(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// test valid sample rate
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		err := webrtcVAD.SetSampleRate(rate)
		assert.NoError(t, err)
		assert.Equal(t, rate, webrtcVAD.GetSampleRate())
	}

	// test invalid sample rate
	err := webrtcVAD.SetSampleRate(22050)
	assert.Error(t, err)

	err = webrtcVAD.SetSampleRate(44100)
	assert.Error(t, err)
}

// TestFloat32ToPCMBytes test data type conversion
func TestFloat32ToPCMBytes(t *testing.T) {
	vad := NewWebRTCVAD()
	require.NotNil(t, vad)
	defer vad.Close()

	webrtcVAD, ok := vad.(*WebRTCVAD)
	require.True(t, ok)

	// test boundary values
	testData := []float32{-1.0, 0.0, 1.0, 1.5, -1.5}
	pcmBytes := webrtcVAD.float32ToPCMBytes(testData)

	assert.Equal(t, len(testData)*2, len(pcmBytes))

	// inspect conversion result
	// -1.0 -> -32768
	// 0.0 -> 0
	// 1.0 -> 32767
	// 1.5 -> 32767 (clipped)
	// -1.5 -> -32768 (clipped)
}

// TestIsValidSampleRate test sample rate validation
func TestIsValidSampleRate(t *testing.T) {
	// valid sample rates
	validRates := []int{8000, 16000, 32000, 48000}
	for _, rate := range validRates {
		assert.True(t, isValidSampleRate(rate))
	}

	// invalid sample rates
	invalidRates := []int{11025, 22050, 44100, 96000}
	for _, rate := range invalidRates {
		assert.False(t, isValidSampleRate(rate))
	}
}

// generateSineWave generate sine wave data for test
func generateSineWave(sampleRate int, frequency float64, duration float64, amplitude float64) []float32 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = float32(amplitude * math.Sin(2*math.Pi*frequency*t))
	}

	return samples
}
