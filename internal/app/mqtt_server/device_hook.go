package mqtt_server

import (
	"fmt"
	"strings"
	"time"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	client "xiaozhi-esp32-server-golang/internal/data/msg"
	log "xiaozhi-esp32-server-golang/logger"
)

// DeviceHook device permission and auto-subscription hook
// Normal users are forbidden from subscribing freely, only allowed to publish to specified topics, auto-subscribe to /p2p/device_sub/{mac} on connect
type DeviceHook struct {
	mqttServer.HookBase
	server           *mqttServer.Server
	publishLifecycle func(event client.MqttLifecycleEvent) error
}

func (h *DeviceHook) ID() string {
	return "custom-device-hook"
}

func (h *DeviceHook) Provides(b byte) bool {
	return b == mqttServer.OnDisconnect || b == mqttServer.OnACLCheck || b == mqttServer.OnSessionEstablished || b == mqttServer.OnSubscribe || b == mqttServer.OnPublish
}

// OnACLCheck publish/subscribe access control
func (h *DeviceHook) OnACLCheck(cl *mqttServer.Client, topic string, write bool) bool {
	isAdmin := isAdminUser(cl)

	if isAdmin {
		return true
	}

	if write {
		if topic == client.MDeviceMockPubTopicPrefix {
			return true
		}
		log.Warnf("forbidden: normal user publish to %s", topic)
		return false
	}

	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Warnf("forbidden: normal user subscribe to %s: unable to parse MAC from client ID, clientID=%s", topic, cl.ID)
		return false
	}

	allowedTopic := deviceSubTopic(mac)
	if topic == allowedTopic {
		return true
	}

	log.Warnf("forbidden: normal user subscribe to %s: only allowed to subscribe to own topic %s", topic, allowedTopic)
	return false
}

func (h *DeviceHook) OnConnect(cl *mqttServer.Client, pk packets.Packet) error {
	isAdmin := isAdminUser(cl)
	if isAdmin {
		return nil
	}
	pk.Connect.Clean = true
	return nil
}

func (h *DeviceHook) OnDisconnect(cl *mqttServer.Client, err error, ok bool) {
	isAdmin := isAdminUser(cl)
	if isAdmin {
		return
	}
	if cl.IsTakenOver() {
		log.Infof("client %s taken over by new connection with same ID, skip unsubscribe", cl.ID)
		return
	}
	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Info("warning: unable to parse MAC address from client ID:", cl.ID)
		return
	}
	h.publishLifecycleEvent(cl.ID, client.MqttLifecycleStateOffline)
	topic := deviceSubTopic(mac)

	action := h.server.Topics.Unsubscribe(topic, cl.ID)
	log.Infof("unsubscribe client %s from topic %s, action: %v", cl.ID, topic, action)

	return
}

// OnSessionEstablished auto-subscribe after connection established
func (h *DeviceHook) OnSessionEstablished(cl *mqttServer.Client, pk packets.Packet) {
	isAdmin := isAdminUser(cl)
	mac := parseMacFromClientId(cl.ID)
	if isAdmin {
		return
	}
	if mac == "" {
		log.Info("warning: unable to parse MAC address from client ID:", cl.ID)
		return
	}
	h.publishLifecycleEvent(cl.ID, client.MqttLifecycleStateOnline)

	topic := deviceSubTopic(mac)

	// Use the server API to subscribe directly, rather than injecting packets
	clientID := cl.ID
	exists := h.server.Topics.Subscribe(clientID, packets.Subscription{
		Filter: topic,
		Qos:    0,
	})

	log.Infof("subscribe client %s to topic %s, exists: %v", clientID, topic, exists)
}

// OnSubscribe print subscribe packet
func (h *DeviceHook) OnSubscribe(cl *mqttServer.Client, pk packets.Packet) packets.Packet {
	log.Info("=== received subscribe packet ===")
	log.Infof("client ID: %s", cl.ID)
	log.Infof("packet type: %v", pk.FixedHeader.Type)
	log.Infof("packet ID: %d", pk.PacketID)

	if len(pk.Filters) > 0 {
		log.Info("subscription info:")
		for i, sub := range pk.Filters {
			log.Infof("  %d. topic: %s, QoS: %d", i+1, sub.Filter, sub.Qos)
		}
	}

	log.Info("==================")
	return pk
}

// OnPublish print publish packet
func (h *DeviceHook) OnPublish(cl *mqttServer.Client, pk packets.Packet) (packets.Packet, error) {
	if cl == nil {
		return pk, nil
	}

	log.Info("=== received publish packet ===")
	log.Infof("client ID: %s", cl.ID)
	log.Infof("packet type: %v", pk.FixedHeader.Type)
	log.Infof("packet ID: %d", pk.PacketID)
	log.Infof("topic: %s", pk.TopicName)

	if isAdminUser(cl) {
		return pk, nil
	}

	if len(pk.Payload) > 0 {
		if len(pk.Payload) > 100 {
			log.Infof("message content (first 100 bytes): %s...", pk.Payload[:100])
		} else {
			log.Infof("message content: %s", pk.Payload)
		}
	} else {
		log.Info("message content: <empty>")
	}

	// Find MAC address from client
	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Info("warning: unable to parse MAC address from client ID:", cl.ID)
		return pk, nil
	}
	forwardTopic := fmt.Sprintf("%s%s", client.MDevicePubTopicPrefix, mac)

	pk.TopicName = forwardTopic

	log.Info("==================")
	return pk, nil
}

// Check if super admin
func isAdminUser(cl *mqttServer.Client) bool {
	if cl == nil {
		return false
	}
	return string(cl.Properties.Username) == "admin"
}

// Parse clientId to extract MAC address
func parseMacFromClientId(clientId string) string {
	parts := strings.Split(clientId, "@@@")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

func deviceIDFromClientId(clientID string) string {
	mac := parseMacFromClientId(clientID)
	if mac == "" {
		return ""
	}
	return strings.ReplaceAll(mac, "_", ":")
}

func (h *DeviceHook) publishLifecycleEvent(clientID string, state string) {
	if h == nil || h.publishLifecycle == nil {
		return
	}
	deviceID := deviceIDFromClientId(clientID)
	if deviceID == "" {
		return
	}
	event := client.MqttLifecycleEvent{
		Type:     client.MqttLifecycleType,
		DeviceID: deviceID,
		State:    state,
		ClientID: clientID,
		Ts:       time.Now().UnixMilli(),
	}
	if err := h.publishLifecycle(event); err != nil {
		log.Warnf("failed to publish MQTT lifecycle event: device=%s state=%s err=%v", deviceID, state, err)
	}
}

func deviceSubTopic(mac string) string {
	return fmt.Sprintf("%s%s", client.MDeviceSubTopicPrefix, mac)
}

// Start periodic subscription printer task
func (h *DeviceHook) StartPeriodicSubscriptionPrinter(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			h.PrintAllClientSubscriptions()
		}
	}()
}

// Print all client subscription topics
func (h *DeviceHook) PrintAllClientSubscriptions() {
	log.Info("=== client subscription topic list ===")
	clients := h.server.Clients.GetAll()
	if len(clients) == 0 {
		log.Info("no connected clients")
		return
	}

	for clientID, _ := range clients {
		log.Infof("client %s subscribed topics: ", clientID)

		// Use server.Topics.Subscribers("+") to get subscribers of all topics
		// then filter subscriptions matching current clientID
		allSubs := h.server.Topics.Subscribers("+")
		foundTopics := false

		// Check client subscriptions
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// Check more possible topic subscriptions
		allSubs = h.server.Topics.Subscribers("#")
		if subs, ok := allSubs.Subscriptions[clientID]; ok {
			log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
			foundTopics = true
		}

		// Check specific topics
		mac := parseMacFromClientId(clientID)
		if mac != "" {
			topic := deviceSubTopic(mac)
			topicSubs := h.server.Topics.Subscribers(topic)
			if subs, ok := topicSubs.Subscriptions[clientID]; ok {
				log.Infof("  - %s (QoS: %d)", subs.Filter, subs.Qos)
				foundTopics = true
			}
		}

		if !foundTopics {
			log.Info("  no subscribed topics or unable to retrieve")
		}
	}
	log.Info("=====================")
}
