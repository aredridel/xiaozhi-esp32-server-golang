package controllers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeviceActivationController struct {
	DB *gorm.DB
}

// Generate 6-digit random numeric code
func generateCode() string {
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	code := 0
	for i, b := range randomBytes {
		code += int(b) << (8 * i)
	}
	return fmt.Sprintf("%06d", code%1000000)
}

// Generate UUID format challenge code
func generateChallenge() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)

	// Set version (4) and variant bits
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		randomBytes[0:4],
		randomBytes[4:6],
		randomBytes[6:8],
		randomBytes[8:10],
		randomBytes[10:16])
}

// 1. Check if device is activated
// GET /api/internal/device/check-activation?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) CheckDeviceActivation(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "device_id parameter is required",
		})
		return
	}

	var device models.Device
	// Use device_id (corresponds to device_name field) to find device
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"activated": false,
				"message":   "Device does not exist",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"activated": false,
			"error":     "Failed to query device",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": device.Activated,
		"message": func() string {
			if device.Activated {
				return "Device activated"
			}
			return "Device not activated"
		}(),
	})
}

// 2. Get activation info
// GET /api/internal/device/activation-info?device_id=xxx&client_id=xxx
func (dac *DeviceActivationController) GetActivationInfo(c *gin.Context) {
	deviceId := c.Query("device_id")
	//clientId := c.Query("client_id")

	if deviceId == "" /*|| clientId == ""*/ {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id and client_id parameters are required"})
		return
	}

	var device models.Device
	var isNewDevice bool

	// Use device_id (corresponds to device_name field) to find device
	if err := dac.DB.Where("device_name = ?", deviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Device does not exist, create new device record
			device = models.Device{
				DeviceName: deviceId,
				UserID:     0, // Set user_id to 0
				DeviceCode: generateCode(),
				Challenge:  generateChallenge(),
				Activated:  false,
			}

			if err := dac.DB.Create(&device).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create device record"})
				return
			}
			isNewDevice = true
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query device"})
			return
		}
	}

	// If device is already activated, return status directly
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"activated": true,
			"message":   "Device activated",
		})
		return
	}

	// If device is not activated, generate or return activation info
	needUpdate := false

	// If no activation code, generate new activation code
	if device.DeviceCode == "" {
		device.DeviceCode = generateCode()
		needUpdate = true
	}

	// If no challenge code, generate new challenge code
	if device.Challenge == "" {
		device.Challenge = generateChallenge()
		needUpdate = true
	}

	// Ensure user_id is 0 (if not new device and not activated)
	if !isNewDevice && device.UserID != 0 {
		device.UserID = 0
		needUpdate = true
	}

	// Update database
	if needUpdate {
		if err := dac.DB.Save(&device).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update device info"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"activated": false,
		"code":      device.DeviceCode,
		"challenge": device.Challenge,
		"message":   "Please bind and activate device in backend, activation code:" + device.DeviceCode,
	})
}

// Verify HMAC-SHA256
func verifyHMAC(challenge, secretKey, providedHmac string) bool {
	if secretKey == "" {
		return true // If pre_secret_key is empty, pass verification directly
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(challenge))
	expectedHmac := hex.EncodeToString(mac.Sum(nil))

	return expectedHmac == providedHmac
}

// 3. Device activation interface
// POST /api/internal/device/activate
func (dac *DeviceActivationController) ActivateDevice(c *gin.Context) {
	var req struct {
		DeviceId     string `json:"device_id" binding:"required"`
		ClientId     string `json:"client_id" binding:"required"`
		Challenge    string `json:"challenge" binding:"required"`
		Algorithm    string `json:"algorithm" binding:"required"`
		SerialNumber string `json:"serial_number" binding:"required"`
		Hmac         string `json:"hmac" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter error: " + err.Error()})
		return
	}

	var device models.Device
	// Use device_id (corresponds to device_name field) to find device
	if err := dac.DB.Where("device_name = ?", req.DeviceId).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "Device does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to query device",
		})
		return
	}

	// Check if device is already activated
	if device.Activated {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Device activated",
		})
		return
	}

	if device.UserID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Device not bound to user",
		})
		return
	}

	if device.Challenge != req.Challenge {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Challenge code error",
		})
		return
	}

	// Verify HMAC (pass directly if pre_secret_key is empty)
	if !verifyHMAC(req.Challenge, device.PreSecretKey, req.Hmac) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "HMAC verification failed",
		})
		return
	}

	// Activate device
	device.Activated = true
	if err := dac.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to activate device",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device activated successfully",
		"data": gin.H{
			"device_id": device.DeviceName,
			"activated": device.Activated,
		},
	})
}
