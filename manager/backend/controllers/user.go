package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	DB                  *gorm.DB
	InternalAuthToken   string
	EndpointAuthToken   string
	WebSocketController interface {
		RequestMcpToolDetailsFromClient(ctx context.Context, agentID string) ([]MCPTool, error)
		RequestDeviceMcpToolDetailsFromClient(ctx context.Context, deviceID string) ([]MCPTool, error)
		CallMcpToolFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error)
		RequestOpenClawStatusFromClient(ctx context.Context, agentID string) (map[string]interface{}, error)
		CallOpenClawChatFromClient(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error)
		CallOpenClawChatStreamFromClient(ctx context.Context, body map[string]interface{}, onResponse func(*WebSocketResponse) error) (map[string]interface{}, error)
		InjectMessageToDevice(ctx context.Context, deviceID, message string, skipLlm bool) error
	}
}

// UserConfigResponse config response visible to regular users (excludes sensitive fields like json_data)
type UserConfigResponse struct {
	ID        uint      `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	ConfigID  string    `json:"config_id"`
	Provider  string    `json:"provider"`
	Enabled   bool      `json:"enabled"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserConfigResponse(cfg *models.Config) *UserConfigResponse {
	if cfg == nil {
		return nil
	}

	return &UserConfigResponse{
		ID:        cfg.ID,
		Type:      cfg.Type,
		Name:      cfg.Name,
		ConfigID:  cfg.ConfigID,
		Provider:  cfg.Provider,
		Enabled:   cfg.Enabled,
		IsDefault: cfg.IsDefault,
		CreatedAt: cfg.CreatedAt,
		UpdatedAt: cfg.UpdatedAt,
	}
}

func toUserConfigResponseList(configs []models.Config) []UserConfigResponse {
	result := make([]UserConfigResponse, 0, len(configs))
	for i := range configs {
		result = append(result, *toUserConfigResponse(&configs[i]))
	}
	return result
}

func normalizeMemoryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return "none"
	case "long":
		return "long"
	default:
		return "short"
	}
}

// Inject message to device
func (uc *UserController) InjectMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
		Message  string `json:"message" binding:"required"`
		SkipLlm  bool   `json:"skip_llm"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	// verify device belongs to current user
	var device models.Device

	if err := uc.DB.Where("device_name = ? AND user_id = ?", req.DeviceID, userID).First(&device).Error; err != nil {
		log.Printf("[InjectMessage] Device query failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device does not exist or does not belong to current user"})
		return
	}

	// Send message injection request to main server via WebSocket
	ctx := context.Background()
	err := uc.WebSocketController.InjectMessageToDevice(ctx, device.DeviceName, req.Message, req.SkipLlm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Message injection failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message injection request sent",
		"data": gin.H{
			"device_id": req.DeviceID,
			"message":   req.Message,
			"skip_llm":  req.SkipLlm,
		},
	})
}

// User creates device directly (no verification code needed)
func (uc *UserController) CreateDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		DeviceName string `json:"device_name" binding:"required,min=2,max=50"`
		AgentID    uint   `json:"agent_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	// verify agent exists and belongs to current user
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent does not exist or does not belong to current user"})
		return
	}

	// Generate 6-digit random device code, ensure no duplicates
	var deviceCode string
	for i := 0; i < 10; i++ { // Maximum 10 attempts
		code := generateRandomCode()

		// Check if code already exists
		var count int64
		if err := uc.DB.Model(&models.Device{}).Where("device_code = ?", code).Count(&count).Error; err == nil && count == 0 {
			deviceCode = code
			break
		}
	}

	// If all 10 attempts duplicate, use timestamp generation
	if deviceCode == "" {
		deviceCode = fmt.Sprintf("%06d", time.Now().Unix()%1000000)
	}

	// Create device
	device := models.Device{
		UserID:     userID.(uint),
		AgentID:    req.AgentID,
		DeviceCode: deviceCode,
		DeviceName: req.DeviceName,
		Activated:  true, // Newly created device defaults to activated
	}

	if err := uc.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create device"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Device created successfully",
		"data": gin.H{
			"device_code": deviceCode,
			"device":      device,
		},
	})
}

// generate 6-digit random numeric code
func generateRandomCode() string {
	// generate 6-digit random number
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	return code
}

// Get all user device overview (read-only)
func (uc *UserController) GetMyDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")

	type DeviceOverview struct {
		ID           uint       `json:"id"`
		DeviceName   string     `json:"device_name"`
		DeviceCode   string     `json:"device_code"`
		AgentID      uint       `json:"agent_id"`
		AgentName    string     `json:"agent_name,omitempty"`
		Activated    bool       `json:"activated"`
		LastActiveAt *time.Time `json:"last_active_at"`
		CreatedAt    time.Time  `json:"created_at"`
	}

	var devices []models.Device
	if err := uc.DB.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device list"})
		return
	}

	// Build device overview information
	var result []DeviceOverview
	for _, device := range devices {
		overview := DeviceOverview{
			ID:           device.ID,
			DeviceName:   device.DeviceName,
			DeviceCode:   device.DeviceCode,
			AgentID:      device.AgentID,
			Activated:    device.Activated,
			LastActiveAt: device.LastActiveAt,
			CreatedAt:    device.CreatedAt,
		}

		// If device is bound to agent, get agent name
		if device.AgentID > 0 {
			var agent models.Agent
			if err := uc.DB.Where("id = ? AND user_id = ?", device.AgentID, userID).First(&agent).Error; err == nil {
				overview.AgentName = agent.Name
			}
		}

		result = append(result, overview)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Agent management
func (uc *UserController) GetAgents(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var agents []models.Agent
	if err := uc.DB.Where("user_id = ?", userID).Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agent list"})
		return
	}

	// Manually load associated config information
	type AgentWithConfigs struct {
		models.Agent
		LLMConfig        *UserConfigResponse `json:"llm_config,omitempty"`
		TTSConfig        *UserConfigResponse `json:"tts_config,omitempty"`
		KnowledgeBaseIDs []uint              `json:"knowledge_base_ids,omitempty"`
	}

	var result []AgentWithConfigs
	for _, agent := range agents {
		agentWithConfig := AgentWithConfigs{Agent: agent}

		// Load LLM config
		if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
			var llmConfig models.Config
			if err := uc.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
				agentWithConfig.LLMConfig = toUserConfigResponse(&llmConfig)
			}
		}

		// Load TTS config
		if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
			var ttsConfig models.Config
			if err := uc.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
				agentWithConfig.TTSConfig = toUserConfigResponse(&ttsConfig)
			}
		}
		if ids, err := uc.listAgentKnowledgeBaseIDs(agent.ID); err == nil {
			agentWithConfig.KnowledgeBaseIDs = ids
		}

		result = append(result, agentWithConfig)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) CreateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Name             string                  `json:"name" binding:"required,min=2,max=50"`
		CustomPrompt     string                  `json:"custom_prompt"`
		LLMConfigID      *string                 `json:"llm_config_id"`
		TTSConfigID      *string                 `json:"tts_config_id"`
		Voice            *string                 `json:"voice"`
		ASRSpeed         string                  `json:"asr_speed"`
		MemoryMode       string                  `json:"memory_mode"`
		SpeakerChatMode  string                  `json:"speaker_chat_mode"`
		MCPServiceNames  string                  `json:"mcp_service_names"`
		OpenClaw         *OpenClawConfigResponse `json:"openclaw"`
		KnowledgeBaseIDs []uint                  `json:"knowledge_base_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error"})
		return
	}

	// Set default values
	if req.ASRSpeed == "" {
		req.ASRSpeed = "normal"
	}
	req.MemoryMode = normalizeMemoryMode(req.MemoryMode)
	req.SpeakerChatMode = normalizeAgentSpeakerChatMode(req.SpeakerChatMode)
	normalizedMCPServiceNames, err := uc.normalizeAndValidateAgentMCPServices(req.MCPServiceNames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := uc.validateKnowledgeBaseOwnership(userID.(uint), req.KnowledgeBaseIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent := models.Agent{
		UserID:          userID.(uint),
		Name:            req.Name,
		CustomPrompt:    req.CustomPrompt,
		LLMConfigID:     req.LLMConfigID,
		TTSConfigID:     req.TTSConfigID,
		Voice:           req.Voice,
		ASRSpeed:        req.ASRSpeed,
		MemoryMode:      req.MemoryMode,
		SpeakerChatMode: req.SpeakerChatMode,
		MCPServiceNames: normalizedMCPServiceNames,
		Status:          "active",
	}
	openClawCfg := mergeOpenClawConfig(
		defaultOpenClawConfig(),
		req.OpenClaw,
	)
	applyOpenClawConfigToAgent(&agent, openClawCfg)

	if err := uc.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent"})
		return
	}
	if err := uc.updateAgentKnowledgeBaseLinks(agent.ID, req.KnowledgeBaseIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent knowledge base association"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"agent": agent, "knowledge_base_ids": uniqueUintSlice(req.KnowledgeBaseIDs)}})
}

func (uc *UserController) GetAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	// Manually load associated config information
	type AgentWithConfigs struct {
		models.Agent
		LLMConfig        *UserConfigResponse `json:"llm_config,omitempty"`
		TTSConfig        *UserConfigResponse `json:"tts_config,omitempty"`
		KnowledgeBaseIDs []uint              `json:"knowledge_base_ids,omitempty"`
	}

	result := AgentWithConfigs{Agent: agent}

	// Load LLM config
	if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
		var llmConfig models.Config
		if err := uc.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
			result.LLMConfig = toUserConfigResponse(&llmConfig)
		}
	}

	// Load TTS config
	if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
		var ttsConfig models.Config
		if err := uc.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
			result.TTSConfig = toUserConfigResponse(&ttsConfig)
		}
	}
	if ids, err := uc.listAgentKnowledgeBaseIDs(agent.ID); err == nil {
		result.KnowledgeBaseIDs = ids
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) UpdateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	var req struct {
		Name             string                  `json:"name" binding:"required,min=2,max=50"`
		CustomPrompt     string                  `json:"custom_prompt"`
		LLMConfigID      *string                 `json:"llm_config_id"`
		TTSConfigID      *string                 `json:"tts_config_id"`
		Voice            *string                 `json:"voice"`
		ASRSpeed         string                  `json:"asr_speed"`
		MemoryMode       *string                 `json:"memory_mode"`
		SpeakerChatMode  *string                 `json:"speaker_chat_mode"`
		MCPServiceNames  string                  `json:"mcp_service_names"`
		OpenClaw         *OpenClawConfigResponse `json:"openclaw"`
		KnowledgeBaseIDs []uint                  `json:"knowledge_base_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error"})
		return
	}

	// Update fields
	agent.Name = req.Name
	agent.CustomPrompt = req.CustomPrompt
	agent.LLMConfigID = req.LLMConfigID
	agent.TTSConfigID = req.TTSConfigID
	agent.Voice = req.Voice

	if req.ASRSpeed != "" {
		agent.ASRSpeed = req.ASRSpeed
	} else {
		agent.ASRSpeed = "normal"
	}
	if req.MemoryMode != nil {
		agent.MemoryMode = normalizeMemoryMode(*req.MemoryMode)
	} else if strings.TrimSpace(agent.MemoryMode) == "" {
		agent.MemoryMode = "short"
	}
	if req.SpeakerChatMode != nil {
		agent.SpeakerChatMode = normalizeAgentSpeakerChatMode(*req.SpeakerChatMode)
	} else if strings.TrimSpace(agent.SpeakerChatMode) == "" {
		agent.SpeakerChatMode = "off"
	}
	normalizedMCPServiceNames, err := uc.normalizeAndValidateAgentMCPServices(req.MCPServiceNames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent.MCPServiceNames = normalizedMCPServiceNames
	openClawCfg := mergeOpenClawConfig(
		buildOpenClawConfigFromAgent(agent),
		req.OpenClaw,
	)
	applyOpenClawConfigToAgent(&agent, openClawCfg)

	if err := uc.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
		return
	}
	if err := uc.validateKnowledgeBaseOwnership(userID.(uint), req.KnowledgeBaseIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := uc.updateAgentKnowledgeBaseLinks(agent.ID, req.KnowledgeBaseIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent knowledge base association"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"agent": agent, "knowledge_base_ids": uniqueUintSlice(req.KnowledgeBaseIDs)}})
}

func (uc *UserController) DeleteAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	if err := uc.DB.Delete(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
		return
	}
	_ = uc.DB.Where("agent_id = ?", agent.ID).Delete(&models.AgentKnowledgeBase{}).Error

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

// Get devices associated with agent
func (uc *UserController) GetAgentDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// First verify agent exists and belongs to current user
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	// Get devices belonging to this agent
	var devices []models.Device
	if err := uc.DB.Where("user_id = ? AND agent_id = ?", userID, agentID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// Add device to agent
func (uc *UserController) AddDeviceToAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	var req struct {
		Code string `json:"code" binding:"required,len=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification code format error"})
		return
	}

	// First verify agent exists and belongs to current user
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	// Verify device verification code (user_id=0 means device not bound to user)
	var device models.Device
	if err := uc.DB.Where("device_code = ? AND user_id = 0", req.Code).First(&device).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification code invalid or device already bound"})
		return
	}

	// Bind device to user and agent
	device.UserID = userID.(uint)

	// Convert agentID string to uint
	agentIDInt, err := strconv.Atoi(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}
	device.AgentID = uint(agentIDInt)

	// Auto-activate device
	device.Activated = true

	if err := uc.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Device binding failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": device})
}

// Remove device from agent
func (uc *UserController) RemoveDeviceFromAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	deviceID := c.Param("device_id")

	// First verify agent exists and belongs to current user
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	// Find device and verify ownership
	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ? AND agent_id = ?", deviceID, userID, agentID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device does not exist or does not belong to this agent"})
		return
	}

	// Remove device from agent (set agent_id to 0, but keep user binding)
	device.AgentID = 0
	if err := uc.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Device removed successfully"})
}

// Get role templates
func (uc *UserController) GetRoleTemplates(c *gin.Context) {
	var roles []models.GlobalRole
	if err := uc.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get role templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func trimSuffixFoldForURL(s string, suffix string) string {
	if len(s) < len(suffix) {
		return s
	}
	start := len(s) - len(suffix)
	if strings.EqualFold(s[start:], suffix) {
		return s[:start]
	}
	return s
}

func normalizeIndexTTSVoiceOptionsBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	baseURL = trimSuffixFoldForURL(baseURL, "/audio/speech")
	baseURL = trimSuffixFoldForURL(baseURL, "/audio/voices")
	return strings.TrimRight(baseURL, "/")
}

func (uc *UserController) fetchIndexTTSVoices(c *gin.Context, configID, overrideURL, overrideAPIKey string) ([]VoiceOption, error) {
	baseURL := "http://127.0.0.1:7860"
	apiKey := ""
	if strings.TrimSpace(configID) != "" {
		var cfg models.Config
		if err := uc.DB.Where("type = ? AND config_id = ?", "tts", configID).First(&cfg).Error; err == nil {
			var cfgMap map[string]any
			if strings.TrimSpace(cfg.JsonData) != "" && json.Unmarshal([]byte(cfg.JsonData), &cfgMap) == nil {
				if v, ok := cfgMap["api_url"].(string); ok && strings.TrimSpace(v) != "" {
					baseURL = strings.TrimSpace(v)
				}
				if v, ok := cfgMap["api_key"].(string); ok {
					apiKey = strings.TrimSpace(v)
				}
			}
		}
	}
	if strings.TrimSpace(overrideURL) != "" {
		baseURL = strings.TrimSpace(overrideURL)
	}
	if strings.TrimSpace(overrideAPIKey) != "" {
		apiKey = strings.TrimSpace(overrideAPIKey)
	}
	baseURL = normalizeIndexTTSVoiceOptionsBaseURL(baseURL)
	if baseURL == "" {
		baseURL = "http://127.0.0.1:7860"
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, baseURL+indexTTSVoicesEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("IndexTTS get voice failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	voiceMap := map[string]any{}
	if err = json.Unmarshal(body, &voiceMap); err != nil {
		return nil, err
	}
	result := make([]VoiceOption, 0, len(voiceMap))
	normalizedConfigPrefix := strings.ToLower(strings.TrimSpace(configID))
	if normalizedConfigPrefix != "" {
		normalizedConfigPrefix += "_"
	}
	for voice := range voiceMap {
		v := strings.TrimSpace(voice)
		if v == "" {
			continue
		}
		// Filter out internal prefix voices generated by current IndexTTS config instance, avoid duplicate display with clone voice.
		if normalizedConfigPrefix != "" && strings.HasPrefix(strings.ToLower(v), normalizedConfigPrefix) {
			continue
		}
		result = append(result, VoiceOption{Value: v, Label: v})
	}
	return result, nil
}

// Get voice options
func (uc *UserController) GetVoiceOptions(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider parameter is required"})
		return
	}
	configID := c.Query("config_id")

	var systemVoices []VoiceOption
	// Special handling: IndexTTS reads available voices from remote service
	if provider == "indextts_vllm" {
		voices, err := uc.fetchIndexTTSVoices(
			c,
			configID,
			c.Query("api_url"),
			c.Query("api_key"),
		)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to get IndexTTS voices: " + err.Error()})
			return
		}
		systemVoices = voices
	} else if provider == "aliyun_qwen" {
		// If config_id not provided, return basic voice list not specific to model (used for admin config page etc.)
		if configID == "" {
			systemVoices = GetVoiceOptionsByProvider("aliyun_qwen")
		} else {
			// Find corresponding TTS config (type=tts)
			var cfg models.Config
			if err := uc.DB.Where("type = ? AND config_id = ?", "tts", configID).First(&cfg).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Corresponding TTS config not found"})
				return
			}

			// Parse json_data to get model
			type qwenConfig struct {
				Model string `json:"model"`
			}
			var qc qwenConfig
			if cfg.JsonData != "" {
				_ = json.Unmarshal([]byte(cfg.JsonData), &qc)
			}
			if qc.Model == "" {
				qc.Model = "qwen3-tts-flash"
			}

			systemVoices = GetAliyunQwenVoicesByModel(qc.Model)
		}
	} else {
		// Other providers: get fixed voice list by provider
		systemVoices = GetVoiceOptionsByProvider(provider)
	}

	result := make([]VoiceOption, 0, len(systemVoices)+8)
	seen := make(map[string]bool, len(systemVoices)+8)

	// Put system voices first
	for _, v := range systemVoices {
		key := strings.TrimSpace(v.Value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, v)
	}

	// Then append user cloned voices (if duplicate with system voices, keep clone label and place at end)
	if userID, ok := c.Get("user_id"); ok && configID != "" {
		var clones []models.VoiceClone
		if err := uc.DB.Where("user_id = ? AND provider = ? AND tts_config_id = ? AND status = ?", userID, provider, configID, "active").Order("created_at DESC").Find(&clones).Error; err == nil {
			for _, clone := range clones {
				opt := BuildVoiceOptionForClone(clone)
				key := strings.TrimSpace(opt.Value)
				if key == "" {
					continue
				}
				if seen[key] {
					for i := range result {
						if strings.TrimSpace(result[i].Value) == key {
							result = append(result[:i], result[i+1:]...)
							break
						}
					}
				}
				seen[key] = true
				result = append(result, opt)
			}
		}
		var sharedClones []models.VoiceClone
		if err := uc.DB.Table("voice_clones").
			Select("voice_clones.*").
			Joins("JOIN users ON users.id = voice_clones.user_id").
			Where("voice_clones.user_id <> ? AND voice_clones.provider = ? AND voice_clones.tts_config_id = ? AND voice_clones.status = ? AND voice_clones.shared_to_all = ? AND users.role = ?",
				userID, provider, configID, "active", true, "admin").
			Order("voice_clones.created_at DESC").
			Scan(&sharedClones).Error; err == nil {
			for _, clone := range sharedClones {
				opt := VoiceOption{
					Value: clone.ProviderVoiceID,
					Label: fmt.Sprintf("[Admin Shared] %s (%s)", clone.Name, clone.ProviderVoiceID),
				}
				key := strings.TrimSpace(opt.Value)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				result = append(result, opt)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Get LLM config list
func (uc *UserController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	// Get all enabled LLM configs from global config, default config first
	if err := uc.DB.Where("type = ? AND enabled = ?", "llm", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LLM config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toUserConfigResponseList(configs)})
}

// Get TTS config list
func (uc *UserController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	// Get all enabled TTS configs from global config, default config first
	if err := uc.DB.Where("type = ? AND enabled = ?", "tts", true).Order("is_default DESC, name ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TTS config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toUserConfigResponseList(configs)})
}

// GetDeviceMcpTools get device dimension MCP tool list (user version)
func (uc *UserController) GetDeviceMcpTools(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device does not exist or does not belong to current user"})
		return
	}

	tools, err := uc.WebSocketController.RequestDeviceMcpToolDetailsFromClient(context.Background(), device.DeviceName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}

// CallAgentMcpTool call agent dimension MCP tool (user version)
func (uc *UserController) CallAgentMcpTool(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist or does not belong to current user"})
		return
	}

	body := map[string]interface{}{
		"agent_id":  agentID,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := uc.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call MCP tool: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (uc *UserController) GetAgentMCPServiceOptions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist"})
		return
	}

	options, err := listEnabledGlobalMCPServiceNames(uc.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get MCP service options: %v", err)})
		return
	}

	normalized := normalizeMCPServiceNamesCSV(agent.MCPServiceNames)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"options":           options,
		"selected":          splitMCPServiceNames(normalized),
		"mcp_service_names": normalized,
	}})
}

// CallDeviceMcpTool call device dimension MCP tool (user version)
func (uc *UserController) CallDeviceMcpTool(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("id")

	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}

	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device does not exist or does not belong to current user"})
		return
	}

	body := map[string]interface{}{
		"device_id": device.DeviceName,
		"tool_name": req.ToolName,
		"arguments": req.Arguments,
	}
	result, err := uc.WebSocketController.CallMcpToolFromClient(context.Background(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call MCP tool: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMCPEndpoint get agent's MCP endpoint URL (user version)
func (uc *UserController) GetAgentMCPEndpoint(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// verify agent exists and belongs to current user
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist or does not belong to current user"})
		return
	}

	// Use common function to generate MCP endpoint
	endpoint, err := GenerateAgentMCPEndpoint(uc.DB, agentID, userID.(uint), uc.EndpointAuthToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return single endpoint string
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"endpoint": endpoint}})
}

// GetAgentOpenClawEndpoint get agent's OpenClaw endpoint URL (user version)
func (uc *UserController) GetAgentOpenClawEndpoint(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist or does not belong to current user"})
		return
	}

	endpoint, err := GenerateAgentOpenClawEndpoint(uc.DB, agentID, userID.(uint), uc.EndpointAuthToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data := gin.H{
		"endpoint":  endpoint,
		"status":    "unknown",
		"connected": false,
	}
	if uc.WebSocketController == nil {
		data["status_message"] = "websocket controller unavailable"
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	statusResult, statusErr := uc.WebSocketController.RequestOpenClawStatusFromClient(context.Background(), agentID)
	if statusErr != nil {
		data["status_message"] = statusErr.Error()
		c.JSON(http.StatusOK, gin.H{"data": data})
		return
	}

	connected, _ := statusResult["connected"].(bool)
	status, _ := statusResult["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		if connected {
			status = "online"
		} else {
			status = "offline"
		}
	}

	data["connected"] = connected
	data["status"] = status
	if msg, ok := statusResult["status_message"].(string); ok && strings.TrimSpace(msg) != "" {
		data["status_message"] = msg
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// CallAgentOpenClawChatTest call agent OpenClaw chat test (user version)
func (uc *UserController) CallAgentOpenClawChatTest(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}
	if uc.WebSocketController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "websocket controller unavailable"})
		return
	}

	var req struct {
		Message   string `json:"message" binding:"required"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request parameter error: " + err.Error()})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message cannot be empty"})
		return
	}

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent does not exist or does not belong to current user"})
		return
	}

	body := map[string]interface{}{
		"agent_id": agentID,
		"message":  req.Message,
	}
	if req.TimeoutMs > 0 {
		body["timeout_ms"] = req.TimeoutMs
	}

	if wantsOpenClawSSE(c) {
		if !prepareOpenClawSSE(c) {
			return
		}
		_ = writeOpenClawSSE(c, "start", map[string]interface{}{
			"agent_id": agentID,
		})

		terminalErrorSent := false
		result, err := uc.WebSocketController.CallOpenClawChatStreamFromClient(
			c.Request.Context(),
			body,
			func(resp *WebSocketResponse) error {
				if resp == nil {
					return nil
				}
				payload := map[string]interface{}{
					"status": resp.Status,
				}
				if resp.Body != nil {
					payload["data"] = resp.Body
				}
				if msg := strings.TrimSpace(resp.Error); msg != "" {
					payload["error"] = msg
				}

				switch resp.Status {
				case http.StatusPartialContent:
					return writeOpenClawSSE(c, "chunk", payload)
				case http.StatusOK:
					return writeOpenClawSSE(c, "result", payload)
				default:
					terminalErrorSent = true
					return writeOpenClawSSE(c, "error", payload)
				}
			},
		)
		if err != nil {
			if !terminalErrorSent {
				_ = writeOpenClawSSE(c, "error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			_ = writeOpenClawSSE(c, "done", map[string]interface{}{
				"ok": false,
			})
			return
		}

		_ = writeOpenClawSSE(c, "done", map[string]interface{}{
			"ok":   true,
			"data": result,
		})
		return
	}

	result, err := uc.WebSocketController.CallOpenClawChatFromClient(context.Background(), body)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(strings.ToLower(msg), "not connected"), strings.Contains(msg, "not connected"):
			c.JSON(http.StatusConflict, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "timeout"), strings.Contains(msg, "timeout"):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": msg})
		case strings.Contains(strings.ToLower(msg), "missing"), strings.Contains(msg, "parameter"):
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		case strings.Contains(msg, "no connected client"):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": msg})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call OpenClaw chat test: " + msg})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetAgentMcpTools get agent's MCP tool list (user version)
func (uc *UserController) GetAgentMcpTools(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// User validation function: verify agent exists and belongs to current user
	userAgentValidator := func(agentID string) error {
		var agent models.Agent
		if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
			return fmt.Errorf("Agent does not exist or does not belong to current user")
		}
		return nil
	}

	// Use common function
	GetAgentMcpToolsCommon(c, agentID, uc.WebSocketController, userAgentValidator)
}

// Get dashboard statistics
func (uc *UserController) GetDashboardStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("role")

	type DashboardStats struct {
		TotalUsers    int64 `json:"totalUsers"`
		TotalDevices  int64 `json:"totalDevices"`
		TotalAgents   int64 `json:"totalAgents"`
		OnlineDevices int64 `json:"onlineDevices"`
	}

	stats := DashboardStats{}

	if userRole == "admin" {
		// Admin views all data
		uc.DB.Model(&models.User{}).Count(&stats.TotalUsers)
		uc.DB.Model(&models.Device{}).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Count(&stats.TotalAgents)
		// Online devices: devices active within last 5 minutes
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("last_active_at > ?", fiveMinutesAgo).Count(&stats.OnlineDevices)
	} else {
		// Regular users only view their own data
		stats.TotalUsers = 0 // Regular users don't see user count
		uc.DB.Model(&models.Device{}).Where("user_id = ?", userID).Count(&stats.TotalDevices)
		uc.DB.Model(&models.Agent{}).Where("user_id = ?", userID).Count(&stats.TotalAgents)
		// Online devices: user's own devices active within last 5 minutes
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
		uc.DB.Model(&models.Device{}).Where("user_id = ? AND last_active_at > ?", userID, fiveMinutesAgo).Count(&stats.OnlineDevices)
	}

	c.JSON(http.StatusOK, stats)
}

func (uc *UserController) updateAgentKnowledgeBaseLinks(agentID uint, knowledgeBaseIDs []uint) error {
	return uc.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&models.AgentKnowledgeBase{}).Error; err != nil {
			return err
		}
		for _, kbID := range uniqueUintSlice(knowledgeBaseIDs) {
			if err := tx.Create(&models.AgentKnowledgeBase{AgentID: agentID, KnowledgeBaseID: kbID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
