package streaming

// SentenceSignalType indicatea段audiobeforeneedsendof句child级controlsignaltype。
type SentenceSignalType string

const (
	SentenceSignalStart SentenceSignalType = "sentence_start"
	SentenceSignalEnd   SentenceSignalType = "sentence_end"
)

// SentenceSignal indicateandcurrentaudio chunkbindofhave序句childboundarysignal。
type SentenceSignal struct {
	Type SentenceSignalType
	Text string
}

// SynthesisEvent indicatea段dual-stream TTS output。
// Audio iscurrentaudio chunk；SentenceSignals indicateatsendthisaudio chunkbeforeneedfirstsendof句childboundarysignal。
type SynthesisEvent struct {
	Audio           []byte
	SentenceSignals []SentenceSignal
}
