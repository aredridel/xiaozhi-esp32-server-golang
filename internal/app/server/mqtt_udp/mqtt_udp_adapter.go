package mqtt_udp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"xiaozhi-esp32-server-golang/internal/app/server/types"
	"xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/logger"
	log "xiaozhi-esp32-server-golang/logger"
)

type MqttConfig struct {
	Broker   string
	Type     string
	Port     int
	ClientID string
	Username string
	Password string
}

// MqttUdpAdapter MQTT-UDP adapter structure
type MqttUdpAdapter struct {
	client          mqtt.Client
	udpServer       *UdpServer
	mqttConfig      *MqttConfig
	deviceId2Conn   *sync.Map
	msgChan         chan mqtt.Message
	onNewConnection types.OnNewConnection
	stopCtx         context.Context
	stopCancel      context.CancelFunc
	sync.RWMutex
}

// MqttUdpAdapterOption for optional parameters
type MqttUdpAdapterOption func(*MqttUdpAdapter)

// WithUdpServer set udpServer
func WithUdpServer(udpServer *UdpServer) MqttUdpAdapterOption {
	return func(s *MqttUdpAdapter) {
		s.udpServer = udpServer
	}
}

func WithOnNewConnection(onNewConnection types.OnNewConnection) MqttUdpAdapterOption {
	return func(s *MqttUdpAdapter) {
		s.onNewConnection = onNewConnection
	}
}

// NewMqttUdpAdapter Create new MQTT-UDP adapter, config is required, other parameters use Option
func NewMqttUdpAdapter(config *MqttConfig, opts ...MqttUdpAdapterOption) *MqttUdpAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	s := &MqttUdpAdapter{
		mqttConfig:    config,
		deviceId2Conn: &sync.Map{},
		msgChan:       make(chan mqtt.Message, 10000),
		stopCtx:       ctx,
		stopCancel:    cancel,
	}
	for _, opt := range opts {
		opt(s)
	}

	go s.processMessage()
	return s
}

func (s *MqttUdpAdapter) getClient() mqtt.Client {
	s.RLock()
	client := s.client
	s.RUnlock()
	return client
}

func (s *MqttUdpAdapter) setClient(client mqtt.Client) {
	s.Lock()
	s.client = client
	s.Unlock()
	s.updateSessionsClient(client)
}

func (s *MqttUdpAdapter) getUdpServer() *UdpServer {
	s.RLock()
	udpServer := s.udpServer
	s.RUnlock()
	return udpServer
}

func (s *MqttUdpAdapter) setUdpServer(udpServer *UdpServer) {
	s.Lock()
	s.udpServer = udpServer
	s.Unlock()
}

func (s *MqttUdpAdapter) updateSessionsClient(client mqtt.Client) {
	s.deviceId2Conn.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*MqttUdpConn); ok {
			conn.SetMqttClient(client)
		}
		return true
	})
}

func (s *MqttUdpAdapter) clearDeviceSessions() {
	s.deviceId2Conn.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*MqttUdpConn); ok {
			conn.Destroy()
		}
		s.deviceId2Conn.Delete(key)
		return true
	})
}

// Start Start MQTT client (non-blocking): connect and retry in background goroutine, does not block program execution
func (s *MqttUdpAdapter) Start() error {
	Infof("MqttUdpAdapter starting, connecting to MQTT server in background Broker=%s:%d ClientID=%s", s.mqttConfig.Broker, s.mqttConfig.Port, s.mqttConfig.ClientID)
	go s.connectAndRetry()
	return nil
}

// connectAndRetry Connect to MQTT in background loop, retry at intervals on failure, decoupled from mqtt_server to not block main flow
func (s *MqttUdpAdapter) connectAndRetry() {
	const retryInterval = 5 * time.Second

	s.RLock()
	cfg := s.mqttConfig
	s.RUnlock()
	if cfg == nil {
		return
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("%s://%s:%d", cfg.Type, cfg.Broker, cfg.Port))
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		Errorf("MQTT connection lost: %v", err)
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		Info("MQTT connected")
		topic := ServerSubTopicPrefix
		if token := client.Subscribe(topic, 0, s.handleMessage); token.Wait() && token.Error() != nil {
			Errorf("Failed to subscribe to topic: %v", token.Error())
		}
	})

	var retryCount int
	for {
		select {
		case <-s.stopCtx.Done():
			return
		default:
		}
		client := mqtt.NewClient(opts)
		s.setClient(client)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			retryCount++
			Errorf("Failed to connect to MQTT server (attempt %d): %v, retrying in %d seconds", retryCount, token.Error(), int(retryInterval.Seconds()))
			select {
			case <-s.stopCtx.Done():
				return
			case <-time.After(retryInterval):
				continue
			}
		}
		break
	}

	_ = s.checkClientActive()
}

func (s *MqttUdpAdapter) checkClientActive() error {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCtx.Done():
				return
			case <-ticker.C:
				s.deviceId2Conn.Range(func(key, value interface{}) bool {
					conn := value.(*MqttUdpConn)
					if !conn.IsActive() {
						conn.Destroy()
					}
					return true
				})
			}
		}
	}()
	return nil
}

func (s *MqttUdpAdapter) SetDeviceSession(deviceId string, conn *MqttUdpConn) {
	Debugf("SetDeviceSession, deviceId: %s", deviceId)
	s.deviceId2Conn.Store(deviceId, conn)
}

func (s *MqttUdpAdapter) getDeviceSession(deviceId string) *MqttUdpConn {
	Debugf("getDeviceSession, deviceId: %s", deviceId)
	if conn, ok := s.deviceId2Conn.Load(deviceId); ok {
		return conn.(*MqttUdpConn)
	}
	return nil
}

// handleMessage will message put into queue
func (s *MqttUdpAdapter) handleMessage(client mqtt.Client, msg mqtt.Message) {
	select {
	case s.msgChan <- msg:
		return
	default:
		Debugf("handleMessage msg chan is full, topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))
	}
}

// Disconnect, timeout or goodbye initiated disconnect
func (s *MqttUdpAdapter) handleDisconnect(deviceId string) {
	Debugf("handleDisconnect, deviceId: %s", deviceId)

	conn := s.getDeviceSession(deviceId)
	if conn == nil {
		Debugf("handleDisconnect, deviceId: %s not found", deviceId)
		return
	}
	conn.ReleaseUdpSession()
	s.deviceId2Conn.Delete(deviceId)
}

// Stop Stop adapter: cancel context, disconnect MQTT, close UDP, cleanup sessions (call before hot reload)
func (s *MqttUdpAdapter) Stop() {
	Debugf("enter MqttUdpAdapter Stop ")
	defer Debugf("exit MqttUdpAdapter Stop ")
	s.stopCancel()
	client := s.getClient()
	if client != nil && client.IsConnected() {
		Debugf("MqttUdpAdapter Stop, disconnect mqtt client")
		client.Disconnect(250)
	}
	udpServer := s.getUdpServer()
	Debugf("MqttUdpAdapter Stop, udpServer: %v", udpServer)
	if udpServer != nil {
		Debugf("MqttUdpAdapter Stop, close udpServer")
		_ = udpServer.Close()
	}
	s.clearDeviceSessions()
}

// ReloadMqttClient Only reconnect MQTT (keep UDP server instance)
func (s *MqttUdpAdapter) ReloadMqttClient(newConfig *MqttConfig) {
	if newConfig == nil {
		return
	}
	s.Lock()
	s.mqttConfig = newConfig
	oldClient := s.client
	s.Unlock()
	if oldClient != nil && oldClient.IsConnected() {
		oldClient.Disconnect(250)
	}
	s.clearDeviceSessions()
	go s.connectAndRetry()
}

// ReloadUdpServer Only restart UDP (keep MQTT connection)
func (s *MqttUdpAdapter) ReloadUdpServer(newUdpServer *UdpServer) {
	if newUdpServer == nil {
		return
	}
	oldUdp := s.getUdpServer()
	s.clearDeviceSessions()
	s.setUdpServer(newUdpServer)
	if oldUdp != nil {
		_ = oldUdp.Close()
	}
}

// Process messages
func (s *MqttUdpAdapter) processMessage() {
	for {
		select {
		case <-s.stopCtx.Done():
			return
		case msg := <-s.msgChan:
			Debugf("mqtt handleMessage, topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))
			var clientMsg ClientMessage
			if err := json.Unmarshal(msg.Payload(), &clientMsg); err != nil {
				Errorf("Failed to parse JSON: %v", err)
				continue
			}
			topicMacAddr, deviceId := s.getDeviceIdByTopic(msg.Topic())
			if deviceId == "" {
				Errorf("Failed to parse mac_addr: %v", msg.Topic())
				continue
			}

			deviceSession := s.getDeviceSession(deviceId)
			if deviceSession == nil {
				// Get session info from UDP server
				udpServer, udpSession, err := s.createUdpSession(deviceId)
				if err != nil {
					Errorf("Failed to create udpSession, deviceId: %s, err: %v", deviceId, err)
					continue
				}

				publicTopic := fmt.Sprintf("%s%s", client.ServerPubTopicPrefix, topicMacAddr)

				mqttClient := s.getClient()
				if mqttClient == nil {
					Errorf("mqtt client is nil, deviceId: %s", deviceId)
					continue
				}
				deviceSession = NewMqttUdpConn(deviceId, publicTopic, mqttClient, udpServer, udpSession)
				s.bindUdpSessionData(deviceSession, udpSession)

				// Save to deviceId2UdpSession
				s.SetDeviceSession(deviceId, deviceSession)

				deviceSession.OnClose(s.handleDisconnect)

				s.onNewConnection(deviceSession)
			} else if clientMsg.Type == "hello" {
				newUdpSession, err := s.rotateDeviceUdpSession(deviceSession, deviceId)
				if err != nil {
					Errorf("hello rebuild udpSession failed, deviceId: %s, err: %v", deviceId, err)
					continue
				}
				Debugf("hello rebuild udpSession successful, deviceId: %s, connID: %s", deviceId, newUdpSession.ConnId)
			}

			err := deviceSession.PushMsgToRecvCmd(msg.Payload())
			if err != nil {
				Errorf("InternalRecvCmd failed: %v", err)
				continue
			}
		}
	}
}

func (s *MqttUdpAdapter) createUdpSession(deviceId string) (*UdpServer, *UdpSession, error) {
	udpServer := s.getUdpServer()
	if udpServer == nil {
		return nil, nil, fmt.Errorf("udpServer is nil")
	}
	udpSession := udpServer.CreateSession(deviceId, "")
	if udpSession == nil {
		return nil, nil, fmt.Errorf("udpSession is nil")
	}
	return udpServer, udpSession, nil
}

func (s *MqttUdpAdapter) bindUdpSessionData(deviceSession *MqttUdpConn, udpSession *UdpSession) {
	if deviceSession == nil || udpSession == nil {
		return
	}
	deviceSession.SetUdpSession(udpSession)
	strAesKey, strFullNonce := udpSession.GetAesKeyAndNonce()
	deviceSession.SetData("aes_key", strAesKey)
	deviceSession.SetData("full_nonce", strFullNonce)
}

func (s *MqttUdpAdapter) rotateDeviceUdpSession(deviceSession *MqttUdpConn, deviceId string) (*UdpSession, error) {
	if deviceSession == nil {
		return nil, fmt.Errorf("deviceSession is nil")
	}
	udpServer, udpSession, err := s.createUdpSession(deviceId)
	if err != nil {
		return nil, err
	}
	oldSession := deviceSession.GetUdpSession()
	s.bindUdpSessionData(deviceSession, udpSession)
	if oldSession != nil {
		udpServer.CloseSessionByRef(oldSession)
	}
	return udpSession, nil
}

func (s *MqttUdpAdapter) getDeviceIdByTopic(topic string) (string, string) {
	var topicMacAddr, deviceId string
	// Parse mac_addr from topic (/p2p/device_public/mac_addr)
	strList := strings.Split(topic, "/")
	if len(strList) == 4 {
		topicMacAddr = strList[3]

		// check if is new format: "GID_test@@@ba_8f_17_de_94_94@@@e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d"
		if strings.Contains(topicMacAddr, "@@@") {
			parts := strings.Split(topicMacAddr, "@@@")
			if len(parts) >= 2 {
				// extract middle part as MAC address
				macAddr := parts[1]
				deviceId = strings.ReplaceAll(macAddr, "_", ":")
			}
		} else {
			deviceId = strings.ReplaceAll(topicMacAddr, "_", ":")
		}
	}

	log.Log().Debugf("topicMacAddr: %s, deviceId: %s", topicMacAddr, deviceId)
	return topicMacAddr, deviceId
}
