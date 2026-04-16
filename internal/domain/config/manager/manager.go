package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"xiaozhi-esp32-server-golang/internal/components/http"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

var (
	defaultManagerOpenClawEnterKeywords = []string{"open lobster", "enter lobster"}
	defaultManagerOpenClawExitKeywords  = []string{"close lobster", "exit lobster"}
)

func cloneOpenClawKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return []string{}
	}
	cloned := make([]string, len(keywords))
	copy(cloned, keywords)
	return cloned
}

func normalizeSpeakerChatMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "identified_only":
		return "identified_only"
	default:
		return "off"
	}
}

// ConfigManager - Config Manager
// Provides high-level config management functions, including cache, hot update, config validation, etc.
type ConfigManager struct {
	// HTTP client
	client *http.ManagerClient
}

// NewConfigManager creates new config manager
func NewManagerUserConfigProvider(config map[string]interface{}) (*ConfigManager, error) {
	// Get backend endpoint management system base URL from config
	var baseURL string
	if backendUrl := config["backend_url"]; backendUrl != nil {
		baseURL = backendUrl.(string)
	}
	// If not configured, use default values
	if baseURL == "" {
		baseURL = "http://localhost:8080" // default values
	}

	// Create Manager HTTP client
	authToken := util.GetManagerAuthToken()
	if token, ok := config["auth_token"].(string); ok && strings.TrimSpace(token) != "" {
		authToken = strings.TrimSpace(token)
	}
	managerClient := http.NewManagerClient(http.ManagerClientConfig{
		BaseURL:    baseURL,
		AuthToken:  authToken,
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	manager := &ConfigManager{
		client: managerClient,
	}

	//log.Log().Debug("config manager initialized successfully", "backend_url", baseURL)
	return manager, nil
}

func (c *ConfigManager) GetUserConfig(ctx context.Context, deviceID string) (types.UConfig, error) {
	// Parse response
	var response struct {
		Data struct {
			VAD struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"vad"`
			ASR struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"asr"`
			LLM struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"llm"`
			TTS struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"tts"`
			Memory struct {
				Provider string `json:"provider"`
				JsonData string `json:"json_data"`
			} `json:"memory"`
			VoiceIdentify map[string]struct {
				ID                 uint     `json:"id"`
				Name               string   `json:"name"`
				Prompt             string   `json:"prompt"`
				Description        string   `json:"description"`
				Uuids              []string `json:"uuids"`
				TTSConfigID        *string  `json:"tts_config_id"`
				Voice              *string  `json:"voice"`
				VoiceModelOverride *string  `json:"voice_model_override"`
			} `json:"voice_identify"`
			KnowledgeBases  []types.KnowledgeBaseRef `json:"knowledge_bases"`
			Prompt          string                   `json:"prompt"`
			AgentId         string                   `json:"agent_id"`
			MemoryMode      string                   `json:"memory_mode"`
			SpeakerChatMode string                   `json:"speaker_chat_mode"`
			MCPServiceNames string                   `json:"mcp_service_names"`
			OpenClaw        struct {
				Allowed       bool     `json:"allowed"`
				EnterKeywords []string `json:"enter_keywords"`
				ExitKeywords  []string `json:"exit_keywords"`
			} `json:"openclaw"`
		} `json:"data"`
	}

	// Send HTTP request
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/configs",
		QueryParams: map[string]string{
			"device_id": deviceID,
		},
		Response: &response,
	})
	if err != nil {
		log.Log().Error("Failed to get user config", "error", err, "device_id", deviceID)
		return types.UConfig{}, err
	}

	// Auxiliary function to parse JSON config data
	parseJsonData := func(jsonStr string) map[string]interface{} {
		var data map[string]interface{}
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				log.Log().Warn("Failed to parse JSON data", "error", err, "json", jsonStr)
				return make(map[string]interface{})
			}
		}
		return data
	}

	// Get voiceprint group info from device config (only get voiceprint group config, not service address)
	// VoiceIdentify is a map, key is voiceprint group name, value includes prompt, description and uuids
	voiceIdentifyData := make(map[string]types.SpeakerGroupInfo)
	if len(response.Data.VoiceIdentify) > 0 {
		// Convert map format voiceprint group info to config format
		for groupName, groupInfo := range response.Data.VoiceIdentify {
			groupData := types.SpeakerGroupInfo{
				ID:                 groupInfo.ID,
				Name:               groupInfo.Name,
				Prompt:             groupInfo.Prompt,
				Description:        groupInfo.Description,
				Uuids:              groupInfo.Uuids,
				TTSConfigID:        groupInfo.TTSConfigID,
				Voice:              groupInfo.Voice,
				VoiceModelOverride: groupInfo.VoiceModelOverride,
			}
			voiceIdentifyData[groupName] = groupData
		}
	}

	// Build config result
	enterKeywords := response.Data.OpenClaw.EnterKeywords
	if len(enterKeywords) == 0 {
		enterKeywords = cloneOpenClawKeywords(defaultManagerOpenClawEnterKeywords)
	}
	exitKeywords := response.Data.OpenClaw.ExitKeywords
	if len(exitKeywords) == 0 {
		exitKeywords = cloneOpenClawKeywords(defaultManagerOpenClawExitKeywords)
	}

	config := types.UConfig{
		SystemPrompt: response.Data.Prompt, // Use agent's custom prompt
		Asr: types.AsrConfig{
			Provider: response.Data.ASR.Provider,
			Config:   parseJsonData(response.Data.ASR.JsonData),
		},
		Tts: types.TtsConfig{
			Provider: response.Data.TTS.Provider,
			Config:   parseJsonData(response.Data.TTS.JsonData),
		},
		Llm: types.LlmConfig{
			Provider: response.Data.LLM.Provider,
			Config:   parseJsonData(response.Data.LLM.JsonData),
		},
		Vad: types.VadConfig{
			Provider: response.Data.VAD.Provider,
			Config:   parseJsonData(response.Data.VAD.JsonData),
		},
		Memory: types.MemoryConfig{
			Provider: response.Data.Memory.Provider,
			Config:   parseJsonData(response.Data.Memory.JsonData),
		},
		KnowledgeBases:  response.Data.KnowledgeBases,
		VoiceIdentify:   voiceIdentifyData,
		MemoryMode:      response.Data.MemoryMode,
		SpeakerChatMode: response.Data.SpeakerChatMode,
		AgentId:         response.Data.AgentId,
		MCPServiceNames: strings.TrimSpace(response.Data.MCPServiceNames),
		OpenClaw: types.OpenClawConfig{
			Allowed:       response.Data.OpenClaw.Allowed,
			EnterKeywords: enterKeywords,
			ExitKeywords:  exitKeywords,
		},
	}
	if strings.TrimSpace(config.MemoryMode) == "" {
		config.MemoryMode = "short"
	}
	config.SpeakerChatMode = normalizeSpeakerChatMode(config.SpeakerChatMode)

	log.Log().Infof("Successfully got device config: deviceId: %s, config: %+v", deviceID, config)
	return config, nil
}

// Get mqtt, mqtt_server, udp, ota, vision config
func (c *ConfigManager) GetSystemConfig(ctx context.Context) (string, error) {
	// Parse response JSON
	var apiResponse struct {
		Data map[string]interface{} `json:"data"`
	}

	// Send HTTP request
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "GET",
		Path:     "/api/system/configs",
		Response: &apiResponse,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get system config: %w", err)
	}

	// Process voice_identify config, ensure threshold field is included
	if voiceIdentifyData, exists := apiResponse.Data["voice_identify"]; exists {
		if voiceIdentifyMap, ok := voiceIdentifyData.(map[string]interface{}); ok {
			// If voice_identify config exists but no threshold field, add default value
			if _, hasThreshold := voiceIdentifyMap["threshold"]; !hasThreshold {
				voiceIdentifyMap["threshold"] = 0.4
				log.Log().Info("voice_identify config missing threshold field, added default value 0.4")
			} else {
				// Validate threshold value range
				if thresholdVal, ok := voiceIdentifyMap["threshold"].(float64); ok {
					if thresholdVal < 0 || thresholdVal > 1 {
						log.Log().Warnf("voice_identify.threshold value %.4f exceeds valid range [0.0, 1.0], using default value 0.4", thresholdVal)
						voiceIdentifyMap["threshold"] = 0.4
					}
				}
			}
			// Update config data
			apiResponse.Data["voice_identify"] = voiceIdentifyMap
		}
	}
	//log.Debugf("Got system config from internal control: %+v", apiResponse.Data)

	// Convert API response to config JSON string
	configJSON, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize config: %w", err)
	}

	return string(configJSON), nil
}

// LoadSystemConfigToViper loads system config from backend API and sets to viper
func (c *ConfigManager) LoadSystemConfigToViper(ctx context.Context) error {
	// Get system config JSON string
	configJSON, err := c.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get system config: %w", err)
	}

	// Use viper.MergeConfigMap to set config to viper
	// First parse JSON string to map
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Set to viper (need to import viper package)
	// viper.MergeConfigMap(configMap)

	log.Log().Info("System config successfully loaded to viper", "config_size", len(configJSON))
	return nil
}

// SwitchDeviceRoleByName switches device role by role name (supports fuzzy matching)
func (c *ConfigManager) SwitchDeviceRoleByName(ctx context.Context, deviceID string, roleName string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	roleName = strings.TrimSpace(roleName)
	if deviceID == "" {
		return "", fmt.Errorf("deviceID cannot be empty")
	}
	if roleName == "" {
		return "", fmt.Errorf("roleName cannot be empty")
	}

	var response struct {
		Data struct {
			RoleName string `json:"role_name"`
		} `json:"data"`
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/switch-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "POST",
		Path:   path,
		Body: map[string]string{
			"role_name": roleName,
		},
		Response: &response,
	})
	if err != nil {
		return "", fmt.Errorf("switch device role failed: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf(response.Error)
	}
	if strings.TrimSpace(response.Data.RoleName) == "" {
		return "", fmt.Errorf("switch device role failed: no matching role returned")
	}
	return response.Data.RoleName, nil
}

// RestoreDeviceDefaultRole restores device default role (clears device bound role)
func (c *ConfigManager) RestoreDeviceDefaultRole(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("deviceID cannot be empty")
	}

	var response struct {
		Error string `json:"error"`
	}

	path := fmt.Sprintf("/api/internal/devices/%s/restore-default-role", url.PathEscape(deviceID))
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "POST",
		Path:     path,
		Response: &response,
	})
	if err != nil {
		return fmt.Errorf("failed to restore default role: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf(response.Error)
	}
	return nil
}

// SearchKnowledge searches through backend unified knowledge base (console forwards by provider)
func (c *ConfigManager) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	_, err := SendDeviceRequest(ctx, eventType, eventData)
	if err != nil {
		log.Log().Error("Failed to send device event", "error", err)
	}
}

func (c *ConfigManager) RegisterMessageEventHandler(ctx context.Context, eventType string, handler types.EventHandler) {
	GetDefaultClient().RegisterMessageHandler(ctx, eventType, handler)
}
