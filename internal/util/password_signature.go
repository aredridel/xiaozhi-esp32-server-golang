package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// GeneratePasswordSignature generates password signature
// Based on clientId + '|' + username and sign key to generate HMAC-SHA256 signature
func GeneratePasswordSignature(data, key string) string {
	// Use HMAC-SHA256 to generate signature
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	signature := h.Sum(nil)

	// Return base64 encoded signature
	return base64.StdEncoding.EncodeToString(signature)
}

// ValidateMqttCredentials validates MQTT credentials
// Implemented according to JavaScript validation logic
func ValidateMqttCredentials(clientId, username, password, signatureKey string) (*MqttCredentialInfo, error) {
	// Validate sign key
	if signatureKey == "" {
		return nil, fmt.Errorf("missing sign key config")
	}

	// Validate clientId
	if clientId == "" {
		return nil, fmt.Errorf("clientId must be a non-empty string")
	}

	// Validate clientId format (must include @@@ separator)
	clientIdParts := strings.Split(clientId, "@@@")
	if len(clientIdParts) != 3 {
		return nil, fmt.Errorf("clientId format error, must include @@@ separator")
	}

	// Validate username
	if username == "" {
		return nil, fmt.Errorf("username must be a non-empty string")
	}

	// Try to decode username (should be base64 encoded JSON)
	var userData map[string]interface{}
	decodedUsername, err := base64.StdEncoding.DecodeString(username)
	if err != nil {
		return nil, fmt.Errorf("username is not valid base64 encoded: %v", err)
	}

	if err := json.Unmarshal(decodedUsername, &userData); err != nil {
		return nil, fmt.Errorf("username is not valid base64 encoded JSON: %v", err)
	}

	// Validate password signature
	signatureData := clientId + "|" + username
	expectedSignature := GeneratePasswordSignature(signatureData, signatureKey)
	if password != expectedSignature {
		return nil, fmt.Errorf("password signature validation failed")
	}

	// Parse info from clientId
	groupId := clientIdParts[0]
	macAddress := strings.ReplaceAll(clientIdParts[1], "_", ":")
	uuid := clientIdParts[2]

	// If validation successful, return parsed useful info
	return &MqttCredentialInfo{
		GroupId:    groupId,
		MacAddress: macAddress,
		UUID:       uuid,
		UserData:   userData,
	}, nil
}

// MqttCredentialInfo MQTT credential info
type MqttCredentialInfo struct {
	GroupId    string                 `json:"groupId"`
	MacAddress string                 `json:"macAddress"`
	UUID       string                 `json:"uuid"`
	UserData   map[string]interface{} `json:"userData"`
}

// GenerateMqttCredentials generates MQTT credentials
// Used for OTA interface to generate MQTT connection info
func GenerateMqttCredentials(deviceId, clientId, ip, signatureKey string) (*MqttCredentials, error) {
	// Process deviceId (replace colon with underscore)
	deviceId = strings.ReplaceAll(deviceId, ":", "_")

	// Build username data (include IP info)
	userName := struct {
		Ip string `json:"ip"`
	}{
		Ip: ip,
	}
	userNameJson, err := json.Marshal(userName)
	if err != nil {
		return nil, fmt.Errorf("username serialization failed: %v", err)
	}
	base64UserName := base64.StdEncoding.EncodeToString(userNameJson)

	// Build clientId, format: GID_test@@@deviceId@@@clientId
	mqttClientId := fmt.Sprintf("GID_test@@@%s@@@%s", deviceId, clientId)

	// Generate password signature
	var pwd string
	if signatureKey != "" {
		// Use sign key to generate password
		signatureData := mqttClientId + "|" + base64UserName
		pwd = GeneratePasswordSignature(signatureData, signatureKey)
	} else {
		// If no sign key configured, use original logic as fallback
		pwd = Sha256Digest([]byte(mqttClientId))
	}

	return &MqttCredentials{
		ClientId: mqttClientId,
		Username: base64UserName,
		Password: pwd,
	}, nil
}

// MqttCredentials MQTT credentials
type MqttCredentials struct {
	ClientId string `json:"client_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}
