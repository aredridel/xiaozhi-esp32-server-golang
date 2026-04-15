# Eino LLM Provider - Unified Multi-Provider Implementation

## Overview

EinoLLMProvider is a unified LLM provider implementation based on the CloudWeGo Eino framework, supporting multiple large language model providers, including OpenAI and Ollama. This implementation fully uses Eino native types and interfaces, providing a consistent API experience.

## Core Features

### ✅ Multi-Provider Support
- **OpenAI**: Supports GPT-3.5, GPT-4 and other models
- **Ollama**: Supports locally deployed open-source models
- **Unified Interface**: All providers use the same API

### ✅ Eino Native Implementation
- Directly uses `*schema.Message` and `*schema.ToolInfo` types
- Calls `chatModel.Generate()` and `chatModel.Stream()` methods
- Supports `chatModel.BindTools()` for tool binding

### ✅ Complete Feature Support
- Streaming and non-streaming responses
- Tool invocation and function binding
- Context control and cancellation
- Chain configuration calls

### ✅ High Compatibility
- Implements standard `LLMProvider` interface
- Supports seamless migration of existing code
- Provides backward-compatible type conversions

## Architecture Design

```
┌─────────────────────────────────────────────────────────────┐
│                    LLMProvider Interface                    │
│  Response() / ResponseWithFunctions() / ResponseWithContext() │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                   EinoLLMProvider                          │
│  • Unified configuration management                                              │
│  • Multi-provider support                                              │
│  • Chained calls                                                 │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                 Eino ChatModel Interface                   │
│  Generate() / Stream() / BindTools()                       │
└─────────────────────────────────────────────────────────────┘
                               │
                     ┌─────────┴─────────┐
                     ▼                   ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│   OpenAI ChatModel      │  │   Ollama ChatModel      │
│   (eino-ext/openai)     │  │   (eino-ext/ollama)     │
└─────────────────────────┘  └─────────────────────────┘
```

## Quick Start

### 1. Basic Configuration

```go
// OpenAI configuration
openaiConfig := map[string]interface{}{
    "type":       "openai",
    "model_name": "gpt-3.5-turbo",
    "api_key":    "your-openai-api-key",
    "base_url":   "https://api.openai.com/v1",
    "max_tokens": 500,
    "streamable": true,
}

// Ollama configuration
ollamaConfig := map[string]interface{}{
    "type":       "ollama",
    "model_name": "llama2",
    "base_url":   "http://localhost:11434",
    "max_tokens": 500,
    "streamable": true,
}
```

### 2. Create Provider

```go
// Create OpenAI provider
openaiProvider, err := NewEinoLLMProvider(openaiConfig)
if err != nil {
    log.Fatalf("Failed to create OpenAI provider: %v", err)
}

// Create Ollama provider
ollamaProvider, err := NewEinoLLMProvider(ollamaConfig)
if err != nil {
    log.Fatalf("Failed to create Ollama provider: %v", err)
}
```

### 3. Use Eino Native Message Types

```go
messages := []*schema.Message{
    {
        Role:    schema.System,
        Content: "You are a helpful assistant",
    },
    {
        Role:    schema.User,
        Content: "Please introduce the Eino framework",
    },
}
```

### 4. Basic Conversation

```go
// Streaming response
responseChan := provider.Response("session_id", messages)
for content := range responseChan {
    fmt.Print(content)
}
```

### 5. Tool Invocation

```go
tools := []*schema.ToolInfo{
    {
        Name: "get_weather",
        ParamsOneOf: &schema.ParamsOneOf{
            // Tool parameter definitions
        },
    },
}

toolResponseChan := provider.ResponseWithFunctions("session_id", messages, tools)
for response := range toolResponseChan {
    switch resp := response.(type) {
    case map[string]string:
        if resp["type"] == "content" {
            fmt.Print(resp["content"])
        }
    case map[string]interface{}:
        if resp["type"] == "tool_calls" {
            fmt.Printf("Tool invocation: %+v\n", resp["tool_calls"])
        }
    }
}
```

### 6. Chained Calls

```go
enhancedProvider := provider.
    WithMaxTokens(1000).
    WithStreamable(false)

fmt.Printf("Provider type: %s\n", enhancedProvider.GetProviderType())
fmt.Printf("Model info: %+v\n", enhancedProvider.GetModelInfo())
```

## API Documentation

### Core Interfaces

#### `NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error)`
Creates a new Eino LLM provider instance.

**Parameters:**
- `config`: Configuration map, must contain `type` field

**Returns:**
- `*EinoLLMProvider`: Provider instance
- `error`: Error information

#### `Response(sessionID string, dialogue []*schema.Message) chan string`
Generates basic text response.

#### `ResponseWithFunctions(sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan interface{}`
Generates response with tool invocation.

#### `ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message) chan string`
Generates response with context control.

### Configuration Options

| Field | Type | Required | Description |
|------|------|------|------|
| `type` | string | ✅ | Provider type: "openai", "ollama" |
| `model_name` | string | ✅ | Model name |
| `api_key` | string | ⚠️ | API key (Required for OpenAI) |
| `base_url` | string | ❌ | Base URL |
| `max_tokens` | int | ❌ | Maximum tokens (default: 500) |
| `streamable` | bool | ❌ | Whether to support streaming (default: true) |

### Chain Methods

#### `WithMaxTokens(maxTokens int) *EinoLLMProvider`
Sets maximum tokens, returns new provider instance.

#### `WithStreamable(streamable bool) *EinoLLMProvider`
Sets streaming support, returns new provider instance.

#### `GetChatModel() model.ChatModel`
Gets the underlying Eino ChatModel instance.

#### `GetProviderType() string`
Gets provider type.

#### `GetModelInfo() map[string]interface{}`
Gets model information and metadata.

## Advanced Usage

### Directly Use Eino ChatModel

```go
chatModel := provider.GetChatModel()

// Directly call generate
result, err := chatModel.Generate(ctx, messages)
if err != nil {
    log.Printf("Generate failed: %v", err)
    return
}
fmt.Printf("Result: %s\n", result.Content)

// Directly call streaming
streamReader, err := chatModel.Stream(ctx, messages)
if err != nil {
    log.Printf("Streaming call failed: %v", err)
    return
}
defer streamReader.Close()

for {
    message, err := streamReader.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Printf("Receive failed: %v", err)
        break
    }
    fmt.Print(message.Content)
}
```

### Multi-Provider Management

```go
providers := make(map[string]*EinoLLMProvider)

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
        log.Printf("Failed to create %s provider: %v", name, err)
        continue
    }
    providers[name] = provider
}

// Use different providers to process same request
for name, provider := range providers {
    fmt.Printf("=== %s Provider Response ===\n", name)
    responseChan := provider.Response("session", messages)
    for content := range responseChan {
        fmt.Print(content)
    }
    fmt.Println()
}
```

## Testing

Run the full test suite:

```bash
go test ./internal/domain/llm/eino_llm/... -v
```

### Test Coverage

- ✅ Provider creation and configuration
- ✅ Multiple provider type support
- ✅ Basic conversation functionality
- ✅ Tool invocation functionality
- ✅ Chained calls
- ✅ Error handling
- ✅ Performance benchmarks

## Dependencies

- `github.com/cloudwego/eino` v0.3.40+
- `github.com/cloudwego/eino-ext` v0.0.1-alpha+

## Best Practices

### 1. Error Handling
```go
provider, err := NewEinoLLMProvider(config)
if err != nil {
    log.Errorf("Failed to create provider: %v", err)
    return
}
```

### 2. Context Control
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

responseChan := provider.ResponseWithContext(ctx, sessionID, messages)
```

### 3. Resource Management
```go
// For streaming responses, ensure all data is consumed
for content := range responseChan {
    // Process content
}
```

### 4. Configuration Management
```go
// Use environment variables to manage sensitive information
config := map[string]interface{}{
    "type":       "openai",
    "model_name": "gpt-3.5-turbo",
    "api_key":    os.Getenv("OPENAI_API_KEY"),
}
```

## Extension Notes

### Add New Provider

To add new provider support, you need to:

1. Add new implementation in the `createXXXChatModel` function
2. Add new case in the switch statement of `NewEinoLLMProvider`
3. Ensure the new provider implements the `model.ChatModel` interface

### Custom Configuration

Can support provider-specific options by extending the configuration map:

```go
config := map[string]interface{}{
    "type":        "openai",
    "model_name":  "gpt-4",
    "api_key":     "your-key",
    "temperature": 0.7,  // Custom parameter
    "top_p":       0.9,  // Custom parameter
}
```

## Version History

### v3.0.0 (Current Version)
- ✅ Completely rewritten based on Eino framework
- ✅ Supports multiple providers (OpenAI, Ollama)
- ✅ Uses Eino native types
- ✅ Directly calls Eino ChatModel methods
- ✅ Removes adapter layer, improves performance
- ✅ Complete test coverage

### v2.x.x (Already Deprecated)
- Hybrid implementation, uses adapter pattern
- Partial Eino integration

### v1.x.x (Already Deprecated)
- Based on traditional OpenAI implementation
- No Eino integration

## Contributing

Issues and Pull Requests are welcome to improve this implementation.

## License

This project follows the license in the project root directory. 
