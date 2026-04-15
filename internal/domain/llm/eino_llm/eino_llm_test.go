package eino_llm

import (
	"fmt"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEinoLLMProvider(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		expectErr bool
	}{
		{
			name: "valid openai config",
			config: map[string]interface{}{
				"type":       "openai",
				"model_name": "gpt-3.5-turbo",
				"api_key":    "test-key",
				"base_url":   "https://api.openai.com/v1",
				"max_tokens": 500,
			},
			expectErr: false,
		},
		{
			name: "valid ollama config",
			config: map[string]interface{}{
				"type":       "ollama",
				"model_name": "llama2",
				"base_url":   "http://localhost:11434",
				"max_tokens": 500,
			},
			expectErr: false,
		},
		{
			name: "missing type",
			config: map[string]interface{}{
				"model_name": "gpt-3.5-turbo",
				"api_key":    "test-key",
			},
			expectErr: true,
		},
		{
			name: "missing model_name",
			config: map[string]interface{}{
				"type":    "openai",
				"api_key": "test-key",
			},
			expectErr: true,
		},
		{
			name: "with streamable config",
			config: map[string]interface{}{
				"type":       "openai",
				"model_name": "gpt-4",
				"api_key":    "test-key",
				"streamable": false,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewEinoLLMProvider(tt.config)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.config["model_name"], provider.modelName)
				assert.NotNil(t, provider.chatModel)
				if tt.config["type"] != nil {
					assert.Equal(t, tt.config["type"], provider.providerType)
				}
			}
		})
	}
}

func TestEinoLLMProvider_GetModelInfo(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"max_tokens": 1000,
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	info := provider.GetModelInfo()

	assert.Equal(t, "eino", info["framework"])
	assert.Equal(t, "eino", info["type"])
	assert.Equal(t, "openai", info["provider_type"])
	assert.Equal(t, "3.0.0", info["adapter_version"])
	assert.Equal(t, true, info["streamable"])
	assert.Contains(t, info, "model_name")
}

func TestEinoLLMProvider_WithMaxTokens(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"max_tokens": 500,
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	// test chain call
	newProvider := provider.WithMaxTokens(1000)

	assert.NotEqual(t, provider, newProvider)    // should not be the same instance
	assert.Equal(t, 500, provider.maxTokens)     // original instance unchanged
	assert.Equal(t, 1000, newProvider.maxTokens) // new instance already updated
}

func TestEinoLLMProvider_WithStreamable(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"streamable": true,
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	// test chain call
	newProvider := provider.WithStreamable(false)

	assert.NotEqual(t, provider, newProvider)      // should not be the same instance
	assert.Equal(t, true, provider.streamable)     // original instance unchanged
	assert.Equal(t, false, newProvider.streamable) // new instance already updated
}

func TestEinoLLMProvider_GetChatModel(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	chatModel := provider.GetChatModel()
	assert.NotNil(t, chatModel)
	assert.Equal(t, provider.chatModel, chatModel)
}

func TestEinoLLMProvider_GetProviderType(t *testing.T) {
	config := map[string]interface{}{
		"type":       "ollama",
		"model_name": "llama2",
		"base_url":   "http://localhost:11434",
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	providerType := provider.GetProviderType()
	assert.Equal(t, "ollama", providerType)
}

func TestEinoLLMProvider_ResponseWithEinoMessages(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai", // useopenaitype
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"streamable": false, // use non-streaming for test
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	// use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are an assistant",
		},
		{
			Role:    schema.User,
			Content: "Hello",
		},
	}

	// test Response method - note: this will try real API call
	// without real API key, this will fail, but we mainly test structure
	responseChan := provider.Response("test_session", messages)
	var responses []string
	for content := range responseChan {
		responses = append(responses, content)
		break // only get first response to avoid long wait
	}

	// for real API call, we mainly validate no panic
	// assert.Len(t, responses, 1)
}

func TestEinoLLMProvider_ResponseWithFunctionsEinoTypes(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai", // useopenaitype
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"streamable": false,
	}

	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)

	// use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "How is the weather in Beijing today?",
		},
	}

	// use Eino native tool type
	tools := []*schema.ToolInfo{
		{
			Name:        "get_weather",
			ParamsOneOf: &schema.ParamsOneOf{
				// simplified tool parameter definition
			},
		},
	}

	// test ResponseWithFunctions method - only validate structure
	responseChan := provider.ResponseWithFunctions("test_session", messages, tools)
	go func() {
		for range responseChan {
			// consume response but don't validate content
		}
	}()
}

func TestEinoConfig_Structure(t *testing.T) {
	// testconfigstructurebody
	config := EinoConfig{
		Type:       "openai",
		ModelName:  "gpt-4",
		APIKey:     "test-key",
		BaseURL:    "https://api.openai.com/v1",
		MaxTokens:  1000,
		Streamable: true,
		Parameters: map[string]interface{}{
			"temperature": 0.7,
		},
	}

	assert.Equal(t, "openai", config.Type)
	assert.Equal(t, "gpt-4", config.ModelName)
	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, true, config.Streamable)
	assert.Contains(t, config.Parameters, "temperature")
}

// BenchmarkEinoLLMProvider_Response performance benchmark test
func BenchmarkEinoLLMProvider_Response(b *testing.B) {
	config := map[string]interface{}{
		"type":       "openai", // useopenaitype
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
	}

	provider, _ := NewEinoLLMProvider(config)
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "This is performance test content",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		responseChan := provider.Response("bench_session", messages)
		// consume response to complete call
		go func() {
			for range responseChan {
				// consume response
			}
		}()
	}
}

// BenchmarkEinoLLMProvider_WithMaxTokens chain call performance test
func BenchmarkEinoLLMProvider_WithMaxTokens(b *testing.B) {
	config := map[string]interface{}{
		"type":       "openai", // useopenaitype
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
	}

	provider, _ := NewEinoLLMProvider(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.WithMaxTokens(1000 + i)
	}
}

// TestExampleConfig test example config
func TestExampleConfig(t *testing.T) {
	assert.Equal(t, "eino_llm", ExampleConfig["type"])
	assert.Equal(t, "gpt-3.5-turbo", ExampleConfig["model_name"])
	assert.Equal(t, 500, ExampleConfig["max_tokens"])
	assert.Equal(t, true, ExampleConfig["streamable"])
}

// TestEinoLLMProvider_FullWorkflow complete workflow test (only structure validation, no real API involved)
func TestEinoLLMProvider_FullWorkflow(t *testing.T) {
	config := map[string]interface{}{
		"type":       "openai", // useopenaitype
		"model_name": "gpt-3.5-turbo",
		"api_key":    "test-key",
		"max_tokens": 500,
		"streamable": true,
	}

	// 1. create provider
	provider, err := NewEinoLLMProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// 2. test config chain call
	enhancedProvider := provider.WithMaxTokens(1000).WithStreamable(false)
	assert.Equal(t, 1000, enhancedProvider.maxTokens)
	assert.Equal(t, false, enhancedProvider.streamable)

	// 3. testmodelinfoget
	info := enhancedProvider.GetModelInfo()
	assert.Equal(t, "eino", info["framework"])
	assert.Equal(t, "eino", info["type"])
	assert.Equal(t, "openai", info["provider_type"])

	// 4. testunderlyingChatModelaccess
	chatModel := enhancedProvider.GetChatModel()
	assert.NotNil(t, chatModel)

	// 5. test provider type
	providerType := enhancedProvider.GetProviderType()
	assert.Equal(t, "openai", providerType)

	// 6. teststructurevalidate（nocallrealAPI）
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "testmessage",
		},
	}

	// only validate function call won't panic, don't validate response content
	responseChan := provider.Response("full_workflow_test", messages)
	go func() {
		for range responseChan {
			// consume response but don't validate content
		}
	}()
}

// TestMultipleProviderTypes test multiple provider types
func TestMultipleProviderTypes(t *testing.T) {
	testCases := []struct {
		name         string
		providerType string
		modelName    string
	}{
		{
			name:         "OpenAI Provider",
			providerType: "openai",
			modelName:    "gpt-3.5-turbo",
		},
		{
			name:         "Ollama Provider",
			providerType: "ollama",
			modelName:    "llama2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"type":       tc.providerType,
				"model_name": tc.modelName,
				"api_key":    "test-key",
			}

			if tc.providerType == "ollama" {
				config["base_url"] = "http://localhost:11434"
			}

			provider, err := NewEinoLLMProvider(config)
			require.NoError(t, err)
			assert.NotNil(t, provider)
			assert.Equal(t, tc.providerType, provider.GetProviderType())
			assert.Equal(t, tc.modelName, provider.modelName)

			// test basic structure
			messages := []*schema.Message{
				{
					Role:    schema.User,
					Content: fmt.Sprintf("test %s provider", tc.providerType),
				},
			}

			// onlyvalidatefunctioncallnowillpanic
			responseChan := provider.Response("multi_provider_test", messages)
			go func() {
				for range responseChan {
					// consumerespond
				}
			}()
		})
	}
}
