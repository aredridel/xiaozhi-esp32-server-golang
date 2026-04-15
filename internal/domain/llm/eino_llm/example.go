package eino_llm

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"

	log "xiaozhi-esp32-server-golang/logger"
)

// ExampleConfig example config
var ExampleConfig = map[string]interface{}{
	"type":       "eino_llm",
	"model_name": "gpt-3.5-turbo",
	"api_key":    "your-api-key-here",
	"base_url":   "https://api.openai.com/v1",
	"max_tokens": 500,
	"streamable": true,
}

// ExampleUsage demonstrates how to use EinoLLMProvider
func ExampleUsage() {
	// 1. OpenAI config example
	openaiConfig := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "your-openai-api-key",
		"base_url":   "https://api.openai.com/v1",
		"max_tokens": 500,
		"streamable": true,
	}

	// 2. Ollama config example
	ollamaConfig := map[string]interface{}{
		"type":       "ollama",
		"model_name": "llama2",
		"base_url":   "http://localhost:11434",
		"max_tokens": 500,
		"streamable": true,
	}

	// 3. create provider
	openaiProvider, err := NewEinoLLMProvider(openaiConfig)
	if err != nil {
		log.Errorf("create OpenAI provider failed: %v", err)
		return
	}

	ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
	if err != nil {
		log.Errorf("create Ollama provider failed: %v", err)
		return
	}

	// 4. use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a useful assistant",
		},
		{
			Role:    schema.User,
			Content: "Please introduce the Eino framework",
		},
	}

	// 5. basic conversation
	fmt.Println("=== OpenAI Basic Conversation ===")
	responseChan := openaiProvider.ResponseWithContext(context.Background(), "example_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	fmt.Println("=== Ollama Basic Conversation ===")
	responseChan = ollamaProvider.ResponseWithContext(context.Background(), "example_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 6. tool call example
	tools := []*schema.ToolInfo{
		{
			Name:        "get_weather",
			ParamsOneOf: &schema.ParamsOneOf{
				// tool parameter definition
			},
		},
	}

	fmt.Println("=== Conversation with tool call ===")
	toolResponseChan := openaiProvider.ResponseWithContext(context.Background(), "example_session", messages, tools)
	for resp := range toolResponseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 7. chain call example
	fmt.Println("=== Chain call example ===")
	enhancedProvider := openaiProvider.
		WithMaxTokens(1000).
		WithStreamable(false)

	fmt.Printf("provider type: %s\n", enhancedProvider.GetProviderType())
	fmt.Printf("model info: %+v\n", enhancedProvider.GetModelInfo())
}

// ExampleAdvancedUsage advanced usage example
func ExampleAdvancedUsage() {
	config := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-4",
		"api_key":    "your-api-key",
		"max_tokens": 1000,
		"streamable": true,
	}

	provider, err := NewEinoLLMProvider(config)
	if err != nil {
		log.Errorf("create provider failed: %v", err)
		return
	}

	// use context control
	ctx := context.Background()
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Please write a long article about AI",
		},
	}

	fmt.Println("=== Conversation with context control ===")
	responseChan := provider.ResponseWithContext(ctx, "advanced_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// direct use Eino ChatModel
	chatModel := provider.GetChatModel()
	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Errorf("direct call ChatModel failed: %v", err)
		return
	}

	fmt.Printf("direct call result: %s\n", result.Content)
}

// ExampleMultiProvider multiple provider example
func ExampleMultiProvider() {
	providers := make(map[string]*EinoLLMProvider)

	// create multiple providers
	configs := map[string]map[string]interface{}{
		"openai": {
			"type":       "openai",
			"model_name": "gpt-3.5-turbo",
			"api_key":    "your-openai-key",
		},
		"ollama": {
			"type":       "ollama",
			"model_name": "llama2",
			"base_url":   "http://localhost:11434",
		},
	}

	for name, config := range configs {
		provider, err := NewEinoLLMProvider(config)
		if err != nil {
			log.Errorf("create %s provider failed: %v", name, err)
			continue
		}
		providers[name] = provider
	}

	// use different providers to process different requests
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello, please introduce yourself",
		},
	}

	for name, provider := range providers {
		fmt.Printf("=== %s provider response ===\n", name)
		responseChan := provider.ResponseWithContext(context.Background(), "multi_session", messages, nil)
		for resp := range responseChan {
			if resp.Content != "" {
				fmt.Print(resp.Content)
			}
			if len(resp.ToolCalls) > 0 {
				fmt.Printf("tool call: %+v\n", resp.ToolCalls)
			}
		}
		fmt.Println()
	}
}

// ExampleWithTools tool call example
func ExampleWithTools() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("create provider failed: %v", err)
		return
	}

	// use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "How is the weather in Beijing today? Please help me check.",
		},
	}

	// use Eino native tool type
	tools := []*schema.ToolInfo{
		{
			Name:        "get_weather",
			ParamsOneOf: &schema.ParamsOneOf{
				// simplified tool parameter definition
				// in actual use, need to properly define parameter structure
			},
		},
	}

	fmt.Println("=== Tool call example ===")

	// use Eino native tool call interface
	fmt.Println("--- Eino native tool call ---")
	responseChan := provider.ResponseWithContext(context.Background(), "tool_session", messages, tools)
	for resp := range responseChan {
		fmt.Printf("response: %+v\n", resp)
	}
}

// MultiProviderExample multiple provider example
func MultiProviderExample() {
	// OpenAI provider example
	fmt.Println("=== OpenAI provider example ===")
	openaiConfig := map[string]interface{}{
		"type":       "openai",
		"model_name": "gpt-3.5-turbo",
		"api_key":    "your-openai-api-key",
		"base_url":   "https://api.openai.com/v1",
		"max_tokens": 500,
	}

	openaiProvider, err := NewEinoLLMProvider(openaiConfig)
	if err != nil {
		log.Errorf("create OpenAI provider failed: %v", err)
		return
	}

	fmt.Printf("provider type: %s\n", openaiProvider.GetProviderType())

	// Ollama provider example
	fmt.Println("\n=== Ollama provider example ===")
	ollamaConfig := map[string]interface{}{
		"type":       "ollama",
		"model_name": "llama2",
		"base_url":   "http://localhost:11434",
		"max_tokens": 500,
	}

	ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
	if err != nil {
		log.Errorf("create Ollama provider failed: %v", err)
		return
	}

	fmt.Printf("provider type: %s\n", ollamaProvider.GetProviderType())

	// use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Please introduce yourself.",
		},
	}

	// test two providers separately
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n--- OpenAI response ---")
	openaiResponse := openaiProvider.ResponseWithContext(ctx, "openai_session", messages, nil)
	for resp := range openaiResponse {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}

	fmt.Println("\n--- Ollama response ---")
	ollamaResponse := ollamaProvider.ResponseWithContext(ctx, "ollama_session", messages, nil)
	for resp := range ollamaResponse {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()
}

// EinoFrameworkAdvantages Eino framework advantages description
func EinoFrameworkAdvantages() string {
	return `
Eino framework main advantages:

1. **Component-based design**
   - Rich component abstractions (ChatModel, Tool, ChatTemplate, Retriever, etc)
   - Each component has unified input/output interface
   - Support component nesting and complex business logic encapsulation

2. **Strong orchestration capability**
   - Graph-based data flow orchestration
   - Automatic type checking, stream processing, concurrent management
   - Support branch execution, state management, field mapping

3. **Complete stream processing**
   - Automatic concatenation of streaming data blocks
   - Automatic boxing of non-stream data as stream
   - Automatic merging of multiple streams
   - Automatic replication of stream to multiple downstream nodes

4. **High extensibility**
   - Support custom callback processors
   - Five cut-face support (OnStart, OnEnd, OnError, etc)
   - Injectable log, trace, monitor and other cross-cutting concerns

5. **Production ready**
   - Complete error processing mechanism
   - Support timeout and cancel operations
   - Connection pool and performance optimization
   - Detailed log and monitoring

This implementation features:

**Multi-provider support**:
- Unified Eino interface support for OpenAI and Ollama
- Flexible provider switching through type config
- Each provider uses the same Eino ChatModel interface

**Eino native implementation**:
- Direct use of *schema.Message type for conversation
- Direct use of *schema.ToolInfo type for tool calls
- Completely built based on Eino framework, no type conversion needed

**Enhanced functions**:
- Chain call support (WithMaxTokens, WithStreamable)
- Unified error processing and log recording
- Support streaming and non-streaming call patterns
- Fully compatible with original LLMProvider interface

**Best practices**:
- Support context cancel and timeout control
- Structured log and monitoring integration
- Type-safe config management
- Resource automatic management and cleanup

This implementation truly leverages the core capabilities of the Eino framework while supporting multiple LLM providers.
`
}

// BasicUsageExample basic usage example
func BasicUsageExample() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("create provider failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// demo chain config
	enhancedProvider := provider.
		WithMaxTokens(2000).
		WithStreamable(true)

	// get underlying Eino ChatModel
	chatModel := enhancedProvider.GetChatModel()
	fmt.Printf("underlying ChatModel: %+v\n", chatModel)

	// get provider type
	providerType := enhancedProvider.GetProviderType()
	fmt.Printf("provider type: %s\n", providerType)

	// get enhanced model info
	modelInfo := enhancedProvider.GetModelInfo()
	fmt.Printf("enhanced model info: %+v\n", modelInfo)

	// complex conversation example - use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a professional software architect, proficient in Go language and AI application development.",
		},
		{
			Role:    schema.User,
			Content: "Please design a chatbot system architecture based on Eino framework.",
		},
	}

	// use enhanced config to perform call
	responseChan := enhancedProvider.ResponseWithContext(ctx, "basic_example", messages, nil)
	fmt.Printf("architecture design response:\n")
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()
}

// EinoNativeExample Eino native API example
func EinoNativeExample() {
	provider, err := NewEinoLLMProvider(ExampleConfig)
	if err != nil {
		log.Errorf("create provider failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// use Eino native message type
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a useful AI assistant.",
		},
		{
			Role:    schema.User,
			Content: "Please briefly introduce the Eino framework.",
		},
	}

	fmt.Println("=== Eino native API example ===")

	// 1. use Eino Response
	fmt.Println("--- Eino Response ---")
	responseChan := provider.ResponseWithContext(ctx, "eino_session", messages, nil)
	for resp := range responseChan {
		if resp.Content != "" {
			fmt.Print(resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
	fmt.Println()

	// 2. use Eino ResponseWithTools
	fmt.Println("\n--- Eino ResponseWithTools ---")
	tools := []*schema.ToolInfo{
		{
			Name:        "search_docs",
			ParamsOneOf: &schema.ParamsOneOf{
				// tool parameter definition
			},
		},
	}

	toolResponseChan := provider.ResponseWithContext(ctx, "eino_tools_session", messages, tools)
	for resp := range toolResponseChan {
		if resp.Content != "" {
			fmt.Printf("content: %s\n", resp.Content)
		}
		if len(resp.ToolCalls) > 0 {
			fmt.Printf("tool call: %+v\n", resp.ToolCalls)
		}
	}
}

func main() {
	BasicUsageExample()
}
