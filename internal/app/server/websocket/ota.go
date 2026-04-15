package websocket

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"xiaozhi-esp32-server-golang/internal/data/client"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"
	ctypes "xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

type ActivationRequest struct {
	Payload ctypes.ActivationPayload `json:"Payload"`
}

func (s *WebSocketServer) handleOta(w http.ResponseWriter, r *http.Request) {
	// Get client IP
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	// Get Device-Id and Client-Id from header
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")

	if deviceId == "" || clientId == "" {
		log.Errorf("Missing Device-Id or Client-Id")
		http.Error(w, "Missing Device-Id or Client-Id", http.StatusBadRequest)
		return
	}

	//deviceId = strings.ReplaceAll(deviceId, ":", "_")

	// Select different configuration based on IP
	clientIp := r.Header.Get("X-Real-IP")
	if clientIp == "" {
		clientIp = r.Header.Get("X-Forwarded-For")
	}
	if clientIp == "" {
		clientIp = r.RemoteAddr
	}

	var activationInfo *ActivationInfo
	authEnable := viper.GetBool("auth.enable")
	log.Debugf("authEnable: %v", authEnable)
	if authEnable {
		configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
		// Check if this deviceId is authenticated
		isActivited, err := configProvider.IsDeviceActivated(r.Context(), deviceId, clientId)
		if err != nil {
			log.Errorf("Failed to check device authentication: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !isActivited {
			code, challenge, msg, timeoutMs := configProvider.GetActivationInfo(r.Context(), deviceId, clientId)
			activationInfo = &ActivationInfo{
				Code:      code,
				Message:   msg,
				Challenge: challenge,
				TimeoutMs: timeoutMs,
			}
			log.Infof("Activation info: &{Code:%s Message:%s Challenge:%s TimeoutMs:%d}", code, msg, challenge, timeoutMs)
		}
	}

	otaConfigPrefix := "ota.external."
	// If IP starts with 192.168, select test configuration
	if strings.HasPrefix(clientIp, "192.168") || strings.HasPrefix(clientIp, "10.") || strings.HasPrefix(clientIp, "127.0.0.1") {
		otaConfigPrefix = "ota.test."
	} else {
		otaConfigPrefix = "ota.external."
	}

	mqttInfo := getMqttInfo(deviceId, clientId, otaConfigPrefix, ip)
	// Password
	respData := &OtaResponse{
		Websocket: WebsocketInfo{
			Url:   viper.GetString(otaConfigPrefix + "websocket.url"),
			Token: viper.GetString(otaConfigPrefix + "websocket.token"),
		},
		Mqtt: mqttInfo,
		ServerTime: ServerTimeInfo{
			Timestamp:      time.Now().UnixMilli(),
			TimezoneOffset: 480,
		},
		Activation: activationInfo,
		Firmware: FirmwareInfo{
			Version: "0.9.9",
			Url:     "",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(respData); err != nil {
		log.Errorf("OTA response serialization failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	return
}

func getMqttInfo(deviceId, clientId, otaConfigPrefix, ip string) *MqttInfo {
	if !viper.GetBool(otaConfigPrefix + "mqtt.enable") {
		return nil
	}

	// Generate MQTT credentials
	signatureKey := viper.GetString("ota.signature_key")
	credentials, err := util.GenerateMqttCredentials(deviceId, clientId, ip, signatureKey)
	if err != nil {
		log.Errorf("Failed to generate MQTT credentials: %v", err)
		return nil
	}

	return &MqttInfo{
		Endpoint:       viper.GetString(otaConfigPrefix + "mqtt.endpoint"),
		ClientId:       credentials.ClientId,
		Username:       credentials.Username,
		Password:       credentials.Password,
		PublishTopic:   client.DeviceMockPubTopicPrefix,
		SubscribeTopic: client.DeviceMockSubTopicPrefix,
	}
}

// handleOtaActivate device activation interface
func (s *WebSocketServer) handleOtaActivate(w http.ResponseWriter, r *http.Request) {
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")
	if deviceId == "" || clientId == "" {
		log.Errorf("Missing Device-Id or Client-Id")
		http.Error(w, "Missing Device-Id or Client-Id", http.StatusBadRequest)
		return
	}
	var req ActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("Activation request parsing failed: %v", err)
		http.Error(w, "Request body parsing failed", http.StatusBadRequest)
		return
	}
	// Verify algorithm
	if req.Payload.Algorithm != "hmac-sha256" {
		http.Error(w, "Unsupported algorithm", http.StatusBadRequest)
		return
	}

	// Call config Provider for binding verification
	configProvider, err := user_config.GetProvider(viper.GetString("config_provider.type"))
	if err != nil {
		log.Errorf("Failed to get config Provider: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	ok, err := configProvider.VerifyChallenge(r.Context(), deviceId, clientId, req.Payload)
	if err != nil {
		log.Errorf("Device activation verification failed: %v", err)
		http.Error(w, "Device activation verification failed", http.StatusInternalServerError)
		return
	}
	if !ok {
		log.Warnf("Device activation verification failed: deviceId=%s, clientId=%s", deviceId, clientId)
		http.Error(w, "Device activation verification failed", http.StatusAccepted)
		return
	}
	// Activation successful, return 200
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Activation successful"))
}
