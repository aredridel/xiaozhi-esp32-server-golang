package inter

// VAD voice activity detection interface
type VAD interface {
	// IsVAD detect audio data voice activity
	IsVAD(pcmData []float32) (bool, error)

	IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error)
	// Reset reset detector state
	Reset() error
	// Close close and release resource
	Close() error
	// IsValid inspect resource whether valid
	IsValid() bool
}
