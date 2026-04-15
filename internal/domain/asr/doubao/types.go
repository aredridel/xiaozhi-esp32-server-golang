package doubao

import "strings"

const (
	legacyDoubaoNonstreamPath = "bigmodel_nostream"
	doubaoStreamingPath       = "bigmodel_async"
)

// DoubaoV2Config doubao ASR config structure
type DoubaoV2Config struct {
	AppID             string // application ID
	AccessToken       string // access token
	WsURL             string // WebSocket URL
	ResourceID        string // resource ID
	ModelName         string // model name
	EndWindowSize     int    // end window size
	EnablePunc        bool   // whether to enable punctuation
	EnableITN         bool   // whether to enable ITN
	EnableDDC         bool   // whether to enable DDC
	ResultType        string // result return mode
	ShowUtterances    bool   // whether to return utterance info
	ForceToSpeechTime int    // minimum duration before forced speech conversion (ms)
	EnableNonstream   bool   // whether to enable dual-stream streaming optimized version
	ChunkDuration     int    // chunk duration (milliseconds)
	Timeout           int    // timeout time (seconds)
}

// DefaultConfig default config
var DefaultConfig = DoubaoV2Config{
	WsURL:             "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async",
	ResourceID:        "volc.bigasr.sauc.duration",
	ModelName:         "bigmodel",
	EndWindowSize:     800,
	EnablePunc:        true,
	EnableITN:         true,
	EnableDDC:         false,
	ResultType:        "full",
	ShowUtterances:    true,
	ForceToSpeechTime: 1000,
	EnableNonstream:   false,
	ChunkDuration:     200,
	Timeout:           30,
}

func normalizeDoubaoWsURL(wsURL string) string {
	if wsURL == "" || !strings.Contains(wsURL, legacyDoubaoNonstreamPath) {
		return wsURL
	}
	return strings.ReplaceAll(wsURL, legacyDoubaoNonstreamPath, doubaoStreamingPath)
}

// DoubaoV2Request doubao ASR request structure
type DoubaoV2Request struct {
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Audio struct {
		Format   string `json:"format"`
		Rate     int    `json:"rate"`
		Bits     int    `json:"bits"`
		Channel  int    `json:"channel"`
		Language string `json:"language"`
	} `json:"audio"`
	Request struct {
		ModelName         string `json:"model_name"`
		EndWindowSize     int    `json:"end_window_size"`
		EnablePunc        bool   `json:"enable_punc"`
		EnableITN         bool   `json:"enable_itn"`
		EnableDDC         bool   `json:"enable_ddc"`
		ResultType        string `json:"result_type"`
		ShowUtterances    bool   `json:"show_utterances"`
		ForceToSpeechTime int    `json:"force_to_speech_time"`
		EnableNonstream   bool   `json:"enable_nonstream"`
	} `json:"request"`
}

// DoubaoV2Response doubao ASR response structure
type DoubaoV2Response struct {
	Code   int `json:"code"`
	Result struct {
		Text string `json:"text"`
	} `json:"result,omitempty"`
	Error string `json:"error,omitempty"`
}
