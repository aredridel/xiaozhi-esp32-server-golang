package manager

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"xiaozhi-esp32-server-golang/internal/components/http"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// HTTP interface response struct

// CheckActivationResponse check activation state response
type CheckActivationResponse struct {
	Activated bool   `json:"activated"`
	Message   string `json:"message"`
}

// GetActivationInfoResponse get activation info response
type GetActivationInfoResponse struct {
	Activated bool   `json:"activated"`
	Code      string `json:"code,omitempty"` // modified to string type to match backend API
	Challenge string `json:"challenge,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ActivateDeviceRequest device activate request
type ActivateDeviceRequest struct {
	DeviceId     string `json:"device_id"`
	ClientId     string `json:"client_id"`
	Code         string `json:"code"`
	Challenge    string `json:"challenge"`
	Algorithm    string `json:"algorithm"`
	SerialNumber string `json:"serial_number"`
	Hmac         string `json:"hmac"`
}

// ActivateDeviceResponse device activate response
type ActivateDeviceResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// IsDeviceActivated check if device is already activated
func (am *ConfigManager) IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error) {
	// directly call backend management system HTTP interface
	activated, err := am.callCheckActivationAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("check device %s activation state failed: %v", deviceId, err)
		return false, err
	}

	log.Log().Debugf("device %s activation state: %v", deviceId, activated)
	return activated, nil
}

// GetActivationInfo get device activation info
func (am *ConfigManager) GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int) {
	// directly call backend management system HTTP interface
	activated, codeStr, challenge, message, err := am.callGetActivationInfoAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("get device %s activation info failed: %v", deviceId, err)
		return "", "", "", 0
	}

	// if device already activated, return directly
	if activated {
		log.Log().Debugf("device %s already activated", deviceId)
		return "", "", message, 0
	}

	// check if Challenge is empty
	if challenge == "" {
		log.Log().Errorf("device %s Challenge field is empty", deviceId)
		return "", "", "Challenge field is empty, please contact admin", 0
	}

	// device not activated, return activation info
	timeoutMs := 300 // default 5 minutes timeout
	log.Log().Debugf("get device %s activation info: code=%s, challenge=%s", deviceId, codeStr, challenge)
	if codeStr == "" {
		log.Log().Warnf("device %s activation code is empty", deviceId)
	}

	return codeStr, challenge, message, timeoutMs
}

// VerifyChallenge validate challenge code and HMAC
func (am *ConfigManager) VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// validate HMAC (if HMAC provided)
	if activationPayload.HMAC != "" {
		if !am.verifyHMAC(activationPayload.Challenge, activationPayload.HMAC) {
			log.Log().Warnf("device %s HMAC validate failed", deviceId)
			return false, fmt.Errorf("HMAC validate failed")
		}
	}

	// directly call backend management system activate interface
	verified, err := am.callActivateDeviceAPI(ctx, deviceId, clientId, activationPayload)
	if err != nil {
		log.Log().Errorf("device activation failed: %v", err)
		return false, err
	}

	if verified {
		log.Log().Infof("device %s activation validate successful", deviceId)
	}

	return verified, nil
}

// verifyHMAC validate HMAC sign
func (am *ConfigManager) verifyHMAC(challenge, providedHmac string) bool {
	// this can be configured according to actual needs
	// temporarily use empty key, actual application should get from config
	secretKey := ""

	if secretKey == "" {
		// if no config key, directly pass validate
		return true
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// HTTP API call method

// callCheckActivationAPI call check activation state interface
func (am *ConfigManager) callCheckActivationAPI(ctx context.Context, deviceId, clientId string) (bool, error) {
	var response CheckActivationResponse

	// send HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/internal/device/check-activation",
		QueryParams: map[string]string{
			"device_id": deviceId,
			"client_id": clientId,
		},
		Response: &response,
	})
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}

	log.Log().Debugf("check activation state response: %+v", response)
	return response.Activated, nil
}

// callGetActivationInfoAPI call get activation info interface
func (am *ConfigManager) callGetActivationInfoAPI(ctx context.Context, deviceId, clientId string) (bool, string, string, string, error) {
	var response GetActivationInfoResponse

	// send HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/internal/device/activation-info",
		QueryParams: map[string]string{
			"device_id": deviceId,
			"client_id": clientId,
		},
		Response: &response,
	})
	if err != nil {
		return false, "", "", "", fmt.Errorf("request failed: %w", err)
	}

	log.Log().Debugf("get activation info response: %+v", response)

	if response.Activated {
		return true, "", "", response.Message, nil
	}

	return false, response.Code, response.Challenge, response.Message, nil
}

// callActivateDeviceAPI call device activate interface
func (am *ConfigManager) callActivateDeviceAPI(ctx context.Context, deviceId, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// build request body
	request := ActivateDeviceRequest{
		DeviceId:     deviceId,
		ClientId:     clientId,
		Challenge:    activationPayload.Challenge,
		Algorithm:    activationPayload.Algorithm,
		SerialNumber: activationPayload.SerialNumber,
		Hmac:         activationPayload.HMAC,
	}

	var response ActivateDeviceResponse

	// send HTTP request
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     "/api/internal/device/activate",
		Body:     request,
		Response: &response,
	})
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}

	log.Log().Debugf("device activate response: %+v", response)

	if !response.Success {
		return false, nil
	}

	return response.Success, nil
}
