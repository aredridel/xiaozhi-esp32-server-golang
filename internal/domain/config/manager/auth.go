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

// HTTPinterfacerespondstructurebody

// CheckActivationResponse inspectactivatestaterespond
type CheckActivationResponse struct {
	Activated bool   `json:"activated"`
	Message   string `json:"message"`
}

// GetActivationInfoResponse getactivateinforespond
type GetActivationInfoResponse struct {
	Activated bool   `json:"activated"`
	Code      string `json:"code,omitempty"` // modifyisstringtype以matchingafterendpointAPI
	Challenge string `json:"challenge,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ActivateDeviceRequest deviceactivaterequest
type ActivateDeviceRequest struct {
	DeviceId     string `json:"device_id"`
	ClientId     string `json:"client_id"`
	Code         string `json:"code"`
	Challenge    string `json:"challenge"`
	Algorithm    string `json:"algorithm"`
	SerialNumber string `json:"serial_number"`
	Hmac         string `json:"hmac"`
}

// ActivateDeviceResponse deviceactivaterespond
type ActivateDeviceResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// IsDeviceActivated inspectdevicewhetheralreadyactivate
func (am *ConfigManager) IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error) {
	// directcallafterendpointmanagesystemofHTTPinterface
	activated, err := am.callCheckActivationAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("inspectdevice %s activatestatefailed: %v", deviceId, err)
		return false, err
	}

	log.Log().Debugf("device %s activatestate: %v", deviceId, activated)
	return activated, nil
}

// GetActivationInfo getdeviceactivateinfo
func (am *ConfigManager) GetActivationInfo(ctx context.Context, deviceId string, clientId string) (string, string, string, int) {
	// directcallafterendpointmanagesystemofHTTPinterface
	activated, codeStr, challenge, message, err := am.callGetActivationInfoAPI(ctx, deviceId, clientId)
	if err != nil {
		log.Log().Errorf("getdevice %s activateinfofailed: %v", deviceId, err)
		return "", "", "", 0
	}

	// ifdevicealreadyactivate，directreturn
	if activated {
		log.Log().Debugf("device %s alreadyactivate", deviceId)
		return "", "", message, 0
	}

	// inspectChallengewhetherisempty
	if challenge == "" {
		log.Log().Errorf("device %s ofChallengefieldisempty", deviceId)
		return "", "", "Challengefieldisempty，please联系manage员", 0
	}

	// devicenotactivate，returnactivateinfo
	timeoutMs := 300 // default5minute钟timeout
	log.Log().Debugf("getdevice %s activateinfo: code=%s, challenge=%s", deviceId, codeStr, challenge)
	if codeStr == "" {
		log.Log().Warnf("device %s activate码isempty", deviceId)
	}

	return codeStr, challenge, message, timeoutMs
}

// VerifyChallenge validate挑战码andHMAC
func (am *ConfigManager) VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// validateHMAC（ifprovideHMAC）
	if activationPayload.HMAC != "" {
		if !am.verifyHMAC(activationPayload.Challenge, activationPayload.HMAC) {
			log.Log().Warnf("device %s HMACvalidatefailed", deviceId)
			return false, fmt.Errorf("HMACvalidatefailed")
		}
	}

	// directcallafterendpointmanagesystemofactivateinterface
	verified, err := am.callActivateDeviceAPI(ctx, deviceId, clientId, activationPayload)
	if err != nil {
		log.Log().Errorf("deviceactivatefailed: %v", err)
		return false, err
	}

	if verified {
		log.Log().Infof("device %s activatevalidatesuccessful", deviceId)
	}

	return verified, nil
}

// verifyHMAC validateHMACsign
func (am *ConfigManager) verifyHMAC(challenge, providedHmac string) bool {
	// 这incanaccording toactualneed求configkey
	// 暂whenuseemptykey，actualapplicationinshouldfromconfiginget
	secretKey := ""

	if secretKey == "" {
		// ifnoconfigkey，directthroughvalidate
		return true
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// HTTP API callmethod

// callCheckActivationAPI callinspectactivatestateinterface
func (am *ConfigManager) callCheckActivationAPI(ctx context.Context, deviceId, clientId string) (bool, error) {
	var response CheckActivationResponse

	// sendHTTPrequest
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
		return false, fmt.Errorf("requestfailed: %w", err)
	}

	log.Log().Debugf("inspectactivatestaterespond: %+v", response)
	return response.Activated, nil
}

// callGetActivationInfoAPI callgetactivateinfointerface
func (am *ConfigManager) callGetActivationInfoAPI(ctx context.Context, deviceId, clientId string) (bool, string, string, string, error) {
	var response GetActivationInfoResponse

	// sendHTTPrequest
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
		return false, "", "", "", fmt.Errorf("requestfailed: %w", err)
	}

	log.Log().Debugf("getactivateinforespond: %+v", response)

	if response.Activated {
		return true, "", "", response.Message, nil
	}

	return false, response.Code, response.Challenge, response.Message, nil
}

// callActivateDeviceAPI calldeviceactivateinterface
func (am *ConfigManager) callActivateDeviceAPI(ctx context.Context, deviceId, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	// buildrequestbody
	request := ActivateDeviceRequest{
		DeviceId:     deviceId,
		ClientId:     clientId,
		Challenge:    activationPayload.Challenge,
		Algorithm:    activationPayload.Algorithm,
		SerialNumber: activationPayload.SerialNumber,
		Hmac:         activationPayload.HMAC,
	}

	var response ActivateDeviceResponse

	// sendHTTPrequest
	err := am.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     "/api/internal/device/activate",
		Body:     request,
		Response: &response,
	})
	if err != nil {
		return false, fmt.Errorf("requestfailed: %w", err)
	}

	log.Log().Debugf("deviceactivaterespond: %+v", response)

	if !response.Success {
		return false, nil
	}

	return response.Success, nil
}
