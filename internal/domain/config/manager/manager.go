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
	defaultManagerOpenClawEnterKeywords = []string{"open龙虾", "enter龙虾"}
	defaultManagerOpenClawExitKeywords  = []string{"close龙虾", "exit龙虾"}
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

// ConfigManager configmanage器
// providehighlayer级ofconfigmanagefunction，package括cache、热update、configvalidateetc
type ConfigManager struct {
	// HTTPclient-side
	client *http.ManagerClient
}

// NewConfigManager create newconfigmanage器
func NewManagerUserConfigProvider(config map[string]interface{}) (*ConfigManager, error) {
	// fromconfigingetafterendpointmanagesystemoffoundationURL
	var baseURL string
	if backendUrl := config["backend_url"]; backendUrl != nil {
		baseURL = backendUrl.(string)
	}
	// ifconfiginno，usedefault values
	if baseURL == "" {
		baseURL = "http://localhost:8080" // default values
	}

	// createManager HTTPclient-side
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

	//log.Log().Debug("configmanage器initializesuccessful", "backend_url", baseURL)
	return manager, nil
}

func (c *ConfigManager) GetUserConfig(ctx context.Context, deviceID string) (types.UConfig, error) {
	// parserespond
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

	// sendHTTPrequest
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method: "GET",
		Path:   "/api/configs",
		QueryParams: map[string]string{
			"device_id": deviceID,
		},
		Response: &response,
	})
	if err != nil {
		log.Log().Error("getuserconfigfailed", "error", err, "device_id", deviceID)
		return types.UConfig{}, err
	}

	// parseJSONconfigdataofauxiliaryfunction
	parseJsonData := func(jsonStr string) map[string]interface{} {
		var data map[string]interface{}
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				log.Log().Warn("parseJSONdatafailed", "error", err, "json", jsonStr)
				return make(map[string]interface{})
			}
		}
		return data
	}

	// fromdeviceconfiggetvoiceprintgroupinfo（onlygetvoiceprintgroupconfig，nogetserviceaddress）
	// VoiceIdentify yesa map，key yesvoiceprintgroupname，value include prompt、description and uuids
	voiceIdentifyData := make(map[string]types.SpeakerGroupInfo)
	if len(response.Data.VoiceIdentify) > 0 {
		// will map formatofvoiceprintgroupinfoconvertisconfigformat
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

	// buildconfigresult
	enterKeywords := response.Data.OpenClaw.EnterKeywords
	if len(enterKeywords) == 0 {
		enterKeywords = cloneOpenClawKeywords(defaultManagerOpenClawEnterKeywords)
	}
	exitKeywords := response.Data.OpenClaw.ExitKeywords
	if len(exitKeywords) == 0 {
		exitKeywords = cloneOpenClawKeywords(defaultManagerOpenClawExitKeywords)
	}

	config := types.UConfig{
		SystemPrompt: response.Data.Prompt, // useagentof自定义hint
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

	log.Log().Infof("successfulgetdeviceconfig: deviceId: %s, config: %+v", deviceID, config)
	return config, nil
}

// get mqtt, mqtt_server, udp, ota, visionconfig
func (c *ConfigManager) GetSystemConfig(ctx context.Context) (string, error) {
	// parserespondJSON
	var apiResponse struct {
		Data map[string]interface{} `json:"data"`
	}

	// sendHTTPrequest
	err := c.client.DoRequest(ctx, http.RequestOptions{
		Method:   "GET",
		Path:     "/api/system/configs",
		Response: &apiResponse,
	})
	if err != nil {
		return "", fmt.Errorf("getsystemconfigfailed: %w", err)
	}

	// process voice_identify config，ensureinclude threshold field
	if voiceIdentifyData, exists := apiResponse.Data["voice_identify"]; exists {
		if voiceIdentifyMap, ok := voiceIdentifyData.(map[string]interface{}); ok {
			// if voice_identify config存atbutno threshold field，adddefault values
			if _, hasThreshold := voiceIdentifyMap["threshold"]; !hasThreshold {
				voiceIdentifyMap["threshold"] = 0.4
				log.Log().Info("voice_identify configMissing threshold field，alreadyadddefault values 0.4")
			} else {
				// validate阈valuerange
				if thresholdVal, ok := voiceIdentifyMap["threshold"].(float64); ok {
					if thresholdVal < 0 || thresholdVal > 1 {
						log.Log().Warnf("voice_identify.threshold value %.4f exceedvalidrange [0.0, 1.0]，usedefault values 0.4", thresholdVal)
						voiceIdentifyMap["threshold"] = 0.4
					}
				}
			}
			// updateconfigdata
			apiResponse.Data["voice_identify"] = voiceIdentifyMap
		}
	}
	//log.Debugf("frominside控gettosystemconfig: %+v", apiResponse.Data)

	// willAPIrespondconvertisconfigJSONcharstring
	configJSON, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return "", fmt.Errorf("serializeconfigfailed: %w", err)
	}

	return string(configJSON), nil
}

// LoadSystemConfigToViper frombackend APIloadsystemconfigandsettoviper
func (c *ConfigManager) LoadSystemConfigToViper(ctx context.Context) error {
	// getsystemconfigJSONcharstring
	configJSON, err := c.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("getsystemconfigfailed: %w", err)
	}

	// useviper.MergeConfigMapwillconfigsettoviper
	// firstfirstwillJSONcharstringparseismap
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("parseconfigJSONfailed: %w", err)
	}

	// settoviper（needimportviperpackage）
	// viper.MergeConfigMap(configMap)

	log.Log().Info("systemconfigalreadysuccessfulloadtoviper", "config_size", len(configJSON))
	return nil
}

// SwitchDeviceRoleByName 按rolename（supportfuzzymatching）switchdevicerole
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
		return "", fmt.Errorf("switch device role failed: notreturnmatchingrole")
	}
	return response.Data.RoleName, nil
}

// RestoreDeviceDefaultRole recoverydevicedefaultrole（cleardevicebindrole）
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

// SearchKnowledge throughmanageafter台unifiedretrieveknowledgelibrary（control台按providerforward）
func (c *ConfigManager) NotifyDeviceEvent(ctx context.Context, eventType string, eventData map[string]interface{}) {
	_, err := SendDeviceRequest(ctx, eventType, eventData)
	if err != nil {
		log.Log().Error("senddeviceeventfailed", "error", err)
	}
}

func (c *ConfigManager) RegisterMessageEventHandler(ctx context.Context, eventType string, handler types.EventHandler) {
	GetDefaultClient().RegisterMessageHandler(ctx, eventType, handler)
}
