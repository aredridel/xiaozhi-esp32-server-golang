package types

import "context"

// IConn is protocol-agnostic connection interface, implemented by websocket/mqtt_udp etc protocol adapters
// you can extend methods according to actual needs

const (
	TransportTypeWebsocket = "websocket"
	TransportTypeMqttUdp   = "udp"
)

type IConn interface {
	// send command/signal data
	SendCmd(msg []byte) error
	// receive command/signal data
	RecvCmd(ctx context.Context, timeout int) ([]byte, error)
	// send voice data
	SendAudio(audio []byte) error
	// receive voice data
	RecvAudio(ctx context.Context, timeout int) ([]byte, error)

	GetDeviceID() string

	Close() error
	OnClose(func(deviceId string))

	CloseAudioChannel() error

	GetTransportType() string

	// get private data
	GetData(key string) (interface{}, error)
}

type OnNewConnection func(conn IConn)
