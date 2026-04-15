package msg

import (
	"encoding/json"

	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
)

const (
	MDeviceMockPubTopicPrefix = "device-server"
	MDeviceMockSubTopicPrefix = "null"
	MDeviceSubTopicPrefix     = "/p2p/device_sub/"
	MDevicePubTopicPrefix     = "/p2p/device_public/"
	MServerSubTopicPrefix     = "/p2p/device_public/#"
	MServerPubTopicPrefix     = MDeviceSubTopicPrefix
)

// message type constants
const (
	MessageTypeHello   = "hello"   // handshake message
	MessageTypeAbort   = "abort"   // abort message
	MessageTypeListen  = "listen"  // listen message
	MessageTypeIot     = "iot"     // IoT message
	MessageTypeMcp     = "mcp"     // MCP message
	MessageTypeGoodBye = "goodbye" // goodbye message
)

// server message type constants
const (
	ServerMessageTypeHello   = "hello"   // handshake message
	ServerMessageTypeStt     = "stt"     // voice to text
	ServerMessageTypeTts     = "tts"     // text to voice
	ServerMessageTypeIot     = "iot"     // IoT message
	ServerMessageTypeLlm     = "llm"     // large language model
	ServerMessageTypeText    = "text"    // text message
	ServerMessageTypeGoodBye = "goodbye" // goodbye message
)

// message state constants
const (
	MessageStateStart         = "start"          // start state
	MessageStateSentenceStart = "sentence_start" // sentence start state
	MessageStateSentenceEnd   = "sentence_end"   // sentence end state
	MessageStateStop          = "stop"           // stop state
	MessageStateDetect        = "detect"         // detect state
	MessageStateAbort         = "abort"          // abort state
	MessageStateSuccess       = "success"        // success state
)

type UdpConfig struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
	Key    string `json:"key"`
	Nonce  string `json:"nonce"`
}

// ServerMessage indicates server message
type ServerMessage struct {
	Type        string                   `json:"type"`
	Text        string                   `json:"text,omitempty"`
	SessionID   string                   `json:"session_id,omitempty"`
	Version     int                      `json:"version"`
	State       string                   `json:"state,omitempty"`
	Transport   string                   `json:"transport,omitempty"`
	AudioFormat *types_audio.AudioFormat `json:"audio_params,omitempty"`
	Emotion     string                   `json:"emotion,omitempty"`
	Udp         *UdpConfig               `json:"udp,omitempty"`
	PayLoad     json.RawMessage          `json:"payload,omitempty"`
}
