package streaming

// SentenceSignalType indicate a segment audio before need to send sentence level control signal type.
type SentenceSignalType string

const (
	SentenceSignalStart SentenceSignalType = "sentence_start"
	SentenceSignalEnd   SentenceSignalType = "sentence_end"
)

// SentenceSignal indicate and current audio chunk bind of ordered sentence boundary signal.
type SentenceSignal struct {
	Type SentenceSignalType
	Text string
}

// SynthesisEvent indicate a segment dual-stream TTS output.
// Audio is current audio chunk; SentenceSignals indicate at send this audio chunk before need first send sentence boundary signal.
type SynthesisEvent struct {
	Audio           []byte
	SentenceSignals []SentenceSignal
}
