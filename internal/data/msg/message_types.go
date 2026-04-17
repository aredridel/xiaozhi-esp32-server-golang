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
	MDeviceLifecycleTopic     = MDevicePubTopicPrefix + "_server/lifecycle"
	MServerSubTopicPrefix     = "/p2p/device_public/#"
	MServerPubTopicPrefix     = MDeviceSubTopicPrefix
)

const (
	MqttLifecycleType         = "mqtt_lifecycle"
	MqttLifecycleStateOnline  = "online"
	MqttLifecycleStateOffline = "offline"
)

// message type constants
const (
	MessageTypeHello      = "hello"       // handshake message
	MessageTypeAbort      = "abort"       // abort message
	MessageTypeListen     = "listen"      // listen message
	MessageTypeIot        = "iot"         // IoT message
	MessageTypeMcp        = "mcp"         // MCP message
	MessageTypeGoodBye    = "goodbye"     // goodbye message
	MessageTypeSpeakReady = "speak_ready" // device is ready to receive proactive broadcast
)

// server message type constants
const (
	ServerMessageTypeHello        = "hello"         // handshake message
	ServerMessageTypeStt          = "stt"           // speech to text
	ServerMessageTypeTts          = "tts"           // text to speech
	ServerMessageTypeIot          = "iot"           // IoT message
	ServerMessageTypeLlm          = "llm"           // large language model
	ServerMessageTypeText         = "text"          // text message
	ServerMessageTypeGoodBye      = "goodbye"       // goodbye message
	ServerMessageTypeSpeakRequest = "speak_request" // proactive broadcast request
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
	MessageStateReady         = "ready"          // device is ready
)

type UdpConfig struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
	Key    string `json:"key"`
	Nonce  string `json:"nonce"`
}

type MqttLifecycleEvent struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	State    string `json:"state"`
	ClientID string `json:"client_id,omitempty"`
	Ts       int64  `json:"ts"`
}

// ServerMessage represents server message
type ServerMessage struct {
	Type        string                   `json:"type"`
	Text        string                   `json:"text,omitempty"`
	SessionID   string                   `json:"session_id,omitempty"`
	Version     int                      `json:"version"`
	State       string                   `json:"state,omitempty"`
	Transport   string                   `json:"transport,omitempty"`
	AudioFormat *types_audio.AudioFormat `json:"audio_params,omitempty"`
	Emotion     string                   `json:"emotion,omitempty"`
	AutoListen  *bool                    `json:"auto_listen,omitempty"`
	Udp         *UdpConfig               `json:"udp,omitempty"`
	PayLoad     json.RawMessage          `json:"payload,omitempty"`
}
