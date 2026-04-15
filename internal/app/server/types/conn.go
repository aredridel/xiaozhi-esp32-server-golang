package types

import "context"

// IConn yesprotocolirrelevantofjoininterface，by websocket/mqtt_udp etcprotocoladapterimplement
// 你canaccording toactualneedextendmethod

const (
	TransportTypeWebsocket = "websocket"
	TransportTypeMqttUdp   = "udp"
)

type IConn interface {
	// sendcommand/信令data
	SendCmd(msg []byte) error
	// receivecommand/信令data
	RecvCmd(ctx context.Context, timeout int) ([]byte, error)
	// sendvoicedata
	SendAudio(audio []byte) error
	// receivevoicedata
	RecvAudio(ctx context.Context, timeout int) ([]byte, error)

	GetDeviceID() string

	Close() error
	OnClose(func(deviceId string))

	CloseAudioChannel() error

	GetTransportType() string

	//getprivatedata
	GetData(key string) (interface{}, error)
}

type OnNewConnection func(conn IConn)
