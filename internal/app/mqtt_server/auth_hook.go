package mqtt_server

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/spf13/viper"
)

// AuthHook implements custom authentication logic
// Supports regular users and super administrators
// Regular user: username is base64 encoded {"ip":"1.202.193.194"}, password is HMAC-SHA256 signature
// Super admin: username admin, password shijingbo!@#
type AuthHook struct {
	mqttServer.HookBase
}

func (h *AuthHook) ID() string {
	return "custom-auth-hook"
}

func (h *AuthHook) Provides(b byte) bool {
	return b == mqttServer.OnConnectAuthenticate
}

func (h *AuthHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
	// Check if authentication is enabled
	enableAuth := viper.GetBool("mqtt_server.enable_auth")
	if !enableAuth {
		//log.Infof("MQTT authentication disabled, allowing all connections")
		return true
	}

	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)
	clientId := string(pk.Connect.ClientIdentifier)

	// Super admin verification
	adminUsername := viper.GetString("mqtt_server.username")
	adminPassword := viper.GetString("mqtt_server.password")
	if username == adminUsername && password == adminPassword {
		log.Infof("Super admin login successful: %s", username)
		return true
	}

	// Regular user verification - use new signature validation logic
	signatureKey := viper.GetString("mqtt_server.signature_key")
	if signatureKey != "" {
		credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
		//log.Infof("MQTT user validation start: clientId=%s, username=%s, password=%s, signatureKey=%s",
		//	clientId, username, password, signatureKey)
		//log.Infof("MQTT user validation start: credentialInfo=%+v", credentialInfo)

		if err != nil {
			log.Warnf("MQTT credential validation failed: %v", err)
			return false
		}

		log.Infof("MQTT user validation successful: groupId=%s, macAddress=%s, uuid=%s",
			credentialInfo.GroupId, credentialInfo.MacAddress, credentialInfo.UUID)
		return true
	}

	// If no signature key configured, fall back to original AES validation logic
	log.Warnf("Missing OTA signature key config, using AES validation method")
	return h.validateWithAes(username, password)
}

// validateWithAes uses AES method to validate password (for backward compatibility)
func (h *AuthHook) validateWithAes(username, password string) bool {
	// Regular user verification
	decoded, err := base64.StdEncoding.DecodeString(username)
	if err != nil {
		return false
	}
	var userInfo map[string]string
	if err := json.Unmarshal(decoded, &userInfo); err != nil {
		return false
	}
	if _, ok := userInfo["ip"]; !ok {
		return false
	}
	// Verify if password is AES encrypted username
	if !checkAesPassword(username, password) {
		return false
	}
	return true
}

// checkAesPassword verifies if password is AES-ECB encrypted base64(username)
func checkAesPassword(username, password string) bool {
	key := []byte("xiaozhi_aes_key_1") // 16-byte key, actual config recommended
	ciphertext, err := aesEncryptECB([]byte(username), key)
	if err != nil {
		return false
	}
	cipherBase64 := base64.StdEncoding.EncodeToString(ciphertext)
	return cipherBase64 == password
}

// aesEncryptECB implements AES-ECB encryption
func aesEncryptECB(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	// PKCS7 padding
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	src = append(src, padtext...)
	encrypted := make([]byte, len(src))
	for bs, be := 0, blockSize; bs < len(src); bs, be = bs+blockSize, be+blockSize {
		block.Encrypt(encrypted[bs:be], src[bs:be])
	}
	return encrypted, nil
}
