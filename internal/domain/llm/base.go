package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"xiaozhi-esp32-server-golang/constants"
	"xiaozhi-esp32-server-golang/internal/domain/llm/coze_llm"
	"xiaozhi-esp32-server-golang/internal/domain/llm/dify_llm"
	"xiaozhi-esp32-server-golang/internal/domain/llm/eino_llm"
)

// LLMExtraErrorKey error pass-through convention: ResponseWithContext failed when use this key in Message.Extra
const LLMExtraErrorKey = "error"

// IsLLMErrorMessage determine if it's LLM pass-through error message (contains error in Extra)
func IsLLMErrorMessage(msg *schema.Message) bool {
	if msg == nil || msg.Extra == nil {
		return false
	}
	v, ok := msg.Extra[LLMExtraErrorKey]
	if !ok || v == nil {
		return false
	}
	_, ok = v.(string)
	return ok
}

// LLMErrorMessage parse error message from Message.Extra (if it's error message)
func LLMErrorMessage(msg *schema.Message) string {
	if msg == nil || msg.Extra == nil {
		return ""
	}
	v, ok := msg.Extra[LLMExtraErrorKey].(string)
	if !ok {
		return ""
	}
	return v
}

// LLMProvider large language model provider interface
// all LLM implementations must follow this interface, use Eino native types
type LLMProvider interface {
	// ResponseWithContext response with context control, supports cancel operation
	// ctx: context, can be used to cancel long-running requests
	// sessionID: session identifier
	// dialogue: conversation history, use Eino native message type
	ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan *schema.Message

	ResponseWithVllm(ctx context.Context, file []byte, text string, mimeType string) (string, error)

	// GetModelInfo get model info
	// return model name and other metadata
	GetModelInfo() map[string]interface{}
	// Close release resources, release connection etc
	Close() error
	// IsValid check whether resource is valid
	IsValid() bool
}

// LLMFactory large language model factory interface
// used for creating different types of LLM providers
type LLMFactory interface {
	// CreateProvider create LLM provider according to config
	CreateProvider(config map[string]interface{}) (LLMProvider, error)
}

// GetLLMProvider create LLM provider
// unified use EinoLLMProvider to process all types
func GetLLMProvider(providerName string, config map[string]interface{}) (LLMProvider, error) {
	cfg := cloneConfigMap(config)
	if providerName != "" {
		if _, ok := cfg["provider"]; !ok {
			cfg["provider"] = providerName
		}
	}

	llmType := resolveLLMType(providerName, cfg)
	cfg["type"] = llmType
	providerKey := resolveLLMProviderName(providerName, cfg, llmType)
	if defaultBaseURL := resolveDefaultBaseURL(providerKey); defaultBaseURL != "" {
		cfg["base_url"] = defaultBaseURL
	} else if baseURL, _ := cfg["base_url"].(string); strings.TrimSpace(baseURL) == "" {
		delete(cfg, "base_url")
	}

	switch llmType {
	case constants.LlmTypeOpenai, constants.LlmTypeOllama, constants.LlmTypeEinoLLM, constants.LlmTypeEino:
		// unified use EinoLLMProvider to process all types
		provider, err := eino_llm.NewEinoLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("create EinoLLM provider failed: %v", err)
		}
		return provider, nil
	case constants.LlmTypeDify:
		provider, err := dify_llm.NewDifyLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("create DifyLLM provider failed: %v", err)
		}
		return provider, nil
	case constants.LlmTypeCoze:
		provider, err := coze_llm.NewCozeLLMProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("create CozeLLM provider failed: %v", err)
		}
		return provider, nil
	}
	return nil, fmt.Errorf("unsupported LLM provider: %s", llmType)
}

func resolveLLMProviderName(providerName string, config map[string]interface{}, llmType string) string {
	provider := strings.ToLower(strings.TrimSpace(providerName))
	if provider == "" {
		if rawProvider, ok := config["provider"].(string); ok {
			provider = strings.ToLower(strings.TrimSpace(rawProvider))
		}
	}
	if provider == "openai" {
		switch llmType {
		case constants.LlmTypeOllama:
			return "ollama"
		case constants.LlmTypeDify:
			return "dify"
		case constants.LlmTypeCoze:
			return "coze"
		}
	}
	return provider
}

func resolveDefaultBaseURL(provider string) string {
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com/v1/"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "aliyun":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "doubao":
		return "https://ark.cn-beijing.volces.com/api/v3"
	case "siliconflow":
		return "https://api.siliconflow.cn/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	default:
		return ""
	}
}

func resolveLLMType(providerName string, config map[string]interface{}) string {
	provider := strings.ToLower(strings.TrimSpace(providerName))
	if provider == "" {
		if rawProvider, ok := config["provider"].(string); ok {
			provider = strings.ToLower(strings.TrimSpace(rawProvider))
		}
	}

	llmType, _ := config["type"].(string)
	llmType = strings.ToLower(strings.TrimSpace(llmType))

	if provider == "openai" {
		switch llmType {
		case constants.LlmTypeOllama:
			return constants.LlmTypeOllama
		case constants.LlmTypeDify:
			return constants.LlmTypeDify
		case constants.LlmTypeCoze:
			return constants.LlmTypeCoze
		}
	}

	switch provider {
	case "ollama":
		return constants.LlmTypeOllama
	case "dify":
		return constants.LlmTypeDify
	case "coze":
		return constants.LlmTypeCoze
	case "openai", "azure", "anthropic", "zhipu", "aliyun", "doubao", "siliconflow", "deepseek":
		return constants.LlmTypeOpenai
	}

	switch llmType {
	case constants.LlmTypeOllama:
		return constants.LlmTypeOllama
	case constants.LlmTypeDify:
		return constants.LlmTypeDify
	case constants.LlmTypeCoze:
		return constants.LlmTypeCoze
	case constants.LlmTypeOpenai, constants.LlmTypeEinoLLM, constants.LlmTypeEino:
		return constants.LlmTypeOpenai
	default:
		return constants.LlmTypeOpenai
	}
}

// ConfigLLMconfigstructure
type Config struct {
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func cloneConfigMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
