package inter

// VAD voice活动detectinterface
type VAD interface {
	// IsVAD detectaudio datainofvoice活动
	IsVAD(pcmData []float32) (bool, error)

	IsVADExt(pcmData []float32, sampleRate int, frameSize int) (bool, error)
	// Reset resetdetect器state
	Reset() error
	// Close closeandreleaseresource
	Close() error
	// IsValid inspectresourcewhethervalid
	IsValid() bool
}
