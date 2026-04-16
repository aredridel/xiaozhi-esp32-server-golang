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

// DeviceHook Device permission and auto-subscription hook
// Regular users are prohibited from subscribing arbitrarily, only allowed to publish to specified topics, and automatically subscribe to /p2p/device_sub/{mac} upon connection
type DeviceHook struct {
	mqttServer.HookBase
	server *mqttServer.Server
}

func (h *DeviceHook) ID() string {
	return "custom-device-hook"
}

func (h *DeviceHook) Provides(b byte) bool {
	return b == mqttServer.OnDisconnect || b == mqttServer.OnACLCheck || b == mqttServer.OnSessionEstablished || b == mqttServer.OnSubscribe || b == mqttServer.OnPublish
}

// OnACLCheck publish/subscribepermissioncontrol
func (h *DeviceHook) OnACLCheck(cl *mqttServer.Client, topic string, write bool) bool {
	isAdmin := isAdminUser(cl)

	if isAdmin {
		return true // super admin no limit
	}

	if write {
		// Only allow regular users to publish to "device-server"
		if topic == client.MDeviceMockPubTopicPrefix {
			return true
		}
		log.Warnf("Regular users are prohibited from publishing to %s", topic)
		return false
	}

	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Warnf("Regular users are prohibited from subscribing to %s: unable to parse MAC from clientID, clientID=%s", topic, cl.ID)
		return false
	}

	allowedTopic := deviceSubTopic(mac)
	if topic == allowedTopic {
		return true
	}

	log.Warnf("Regular users are prohibited from subscribing to %s: only allowed to subscribe to own topic %s", topic, allowedTopic)
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
		log.Infof("Client %s has been taken over by a new connection with the same ID, skipping unsubscription", cl.ID)
		return
	}
	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Info("Warning: unable to parse MAC address from clientID:", cl.ID)
		return
	}
	topic := deviceSubTopic(mac)

	action := h.server.Topics.Unsubscribe(topic, cl.ID)
	log.Infof("Unsubscribed client %s from topic %s, action: %v", cl.ID, topic, action)

	return
}

// OnSessionEstablished Auto-subscribe after connection is established
func (h *DeviceHook) OnSessionEstablished(cl *mqttServer.Client, pk packets.Packet) {
	isAdmin := isAdminUser(cl)
	mac := parseMacFromClientId(cl.ID)
	if isAdmin {
		return // Super admin has no restrictions
	}
	if mac == "" {
		log.Info("Warning: unable to parse MAC address from clientID:", cl.ID)
		return
	}

	topic := deviceSubTopic(mac)

	// Use server API to subscribe directly instead of injecting packets
	clientID := cl.ID
	exists := h.server.Topics.Subscribe(clientID, packets.Subscription{
		Filter: topic,
		Qos:    0,
	})

	log.Infof("Subscribed client %s to topic %s, exists: %v", clientID, topic, exists)
}

// OnSubscribe Print subscription packet
func (h *DeviceHook) OnSubscribe(cl *mqttServer.Client, pk packets.Packet) packets.Packet {
	log.Info("=== Received Subscribe Packet ===")
	log.Infof("Client ID: %s", cl.ID)
	log.Infof("Packet Type: %v", pk.FixedHeader.Type)
	log.Infof("Packet ID: %d", pk.PacketID)

	if len(pk.Filters) > 0 {
		log.Info("Subscription Info:")
		for i, sub := range pk.Filters {
			log.Infof("  %d. Topic: %s, QoS: %d", i+1, sub.Filter, sub.Qos)
		}
	}

	log.Info("==================")
	return pk
}

// OnPublish Print publish packet
func (h *DeviceHook) OnPublish(cl *mqttServer.Client, pk packets.Packet) (packets.Packet, error) {
	log.Info("=== Received Publish Packet ===")
	log.Infof("Client ID: %s", cl.ID)
	log.Infof("Packet Type: %v", pk.FixedHeader.Type)
	log.Infof("Packet ID: %d", pk.PacketID)
	log.Infof("Topic: %s", pk.TopicName)

	if isAdminUser(cl) {
		return pk, nil
	}

	if len(pk.Payload) > 0 {
		if len(pk.Payload) > 100 {
			// If message is too long, only show first 100 bytes
			log.Infof("Message Content (first 100 bytes): %s...", pk.Payload[:100])
		} else {
			log.Infof("Message Content: %s", pk.Payload)
		}
	} else {
		log.Info("Message Content: <empty>")
	}

	// Find MAC address from cl
	mac := parseMacFromClientId(cl.ID)
	if mac == "" {
		log.Info("Warning: unable to parse MAC address from clientID:", cl.ID)
		return pk, nil
	}
	forwardTopic := fmt.Sprintf("%s%s", client.MDevicePubTopicPrefix, mac)

	pk.TopicName = forwardTopic

	log.Info("==================")
	return pk, nil
}

// Check if super admin
func isAdminUser(cl *mqttServer.Client) bool {
	return string(cl.Properties.Username) == "admin"
}

// Parse clientId to get MAC address
func parseMacFromClientId(clientId string) string {
	parts := strings.Split(clientId, "@@@")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

func deviceSubTopic(mac string) string {
	return fmt.Sprintf("%s%s", client.MDeviceSubTopicPrefix, mac)
}

// Start periodic subscription topic printing task
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
	log.Info("=== Client Subscription Topic List ===")
	clients := h.server.Clients.GetAll()
	if len(clients) == 0 {
		log.Info("No connected clients currently")
		return
	}

	for clientID, _ := range clients {
		log.Infof("Topics subscribed by client %s: ", clientID)

		// Use server.Topics.Subscribers("+") to get all topic subscribers
		// Then filter subscriptions matching current clientID
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
			log.Info("  No subscription topics or unable to retrieve")
		}
	}
	log.Info("=====================")
}
