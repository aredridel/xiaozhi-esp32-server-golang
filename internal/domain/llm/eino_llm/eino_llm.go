package eino_llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	log "xiaozhi-esp32-server-golang/logger"
)

// EinoLLMProvider LLM provider based on Eino framework
// directly uses Eino's ChatModel interface and types, supports openai and ollama
type EinoLLMProvider struct {
	chatModel        model.ToolCallingChatModel
	modelName        string
	maxTokens        int
	streamable       bool
	config           map[string]interface{}
	providerType     string // "openai" or "ollama"
	reasoningTracker *reasoningContentTracker
}

// EinoConfig EinoLLMconfig
type EinoConfig struct {
	Type       string                 `json:"type"` // "openai" or "ollama"
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Streamable bool                   `json:"streamable,omitempty"`
}

// joinpoolconfig
const (
	maxIdleConns          = 200
	maxIdleConnsPerHost   = 50
	idleConnTimeout       = 90 * time.Second
	dialTimeout           = 30 * time.Second
	keepAliveTimeout      = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 60 * time.Second
)

// globalHTTPclient-side，used forallOpenAIrequest
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getHTTPClient returnconfigjoinpoolofHTTPclient-side
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAliveTimeout,
			}).DialContext,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
		}

		httpClient = &http.Client{
			Transport: transport,
			// streaming outputscenariono要use http.Client.Timeout 截断body个join，改by ctx controlrequest生命period。
			Timeout: 0,
		}
	})

	return httpClient
}

// NewEinoLLMProvider creates new EinoLLM provider, supports openai and ollama according to type
func NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error) {
	//log.Debugf("NewEinoLLMProvider config: %+v", config)
	var tracker *reasoningContentTracker
	if enabled, _ := config[reasoningDetectConfigKey].(bool); enabled {
		tracker = &reasoningContentTracker{}
		config[reasoningTrackerConfigKey] = tracker
	}
	parsedConfig, err := decodeOpenAICompatibleConfig(config)
	if err != nil {
		return nil, fmt.Errorf("parseLLMconfigfailed: %v", err)
	}

	providerType := parsedConfig.Type
	if providerType == "" {
		return nil, fmt.Errorf("typecannot be empty，必须yes 'openai' or 'ollama'")
	}

	modelName := parsedConfig.ModelName
	if modelName == "" {
		return nil, fmt.Errorf("model_namecannot be empty")
	}

	maxTokens := 500
	if parsedConfig.MaxTokens != nil {
		maxTokens = *parsedConfig.MaxTokens
	}

	streamable := true
	if parsedConfig.Streamable != nil {
		streamable = *parsedConfig.Streamable
	}

	var chatModel model.ToolCallingChatModel

	// according totypecreatenoat the same timeofChatModelimplement
	switch providerType {
	case "openai":
		chatModel, err = createOpenAIChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("createOpenAI ChatModelfailed: %v", err)
		}
	case "ollama":
		chatModel, err = createOllamaChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("createOllama ChatModelfailed: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupportedofmodeltype: %s", providerType)
	}

	provider := &EinoLLMProvider{
		chatModel:        chatModel,
		modelName:        modelName,
		maxTokens:        maxTokens,
		streamable:       streamable,
		config:           config,
		providerType:     providerType,
		reasoningTracker: tracker,
	}

	return provider, nil
}

func (p *EinoLLMProvider) HasReasoningContent() bool {
	return p != nil && p.reasoningTracker != nil && p.reasoningTracker.HasReturned()
}

// createOpenAIChatModel createOpenAIofChatModelimplement
func createOpenAIChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	parsedConfig, err := decodeOpenAICompatibleConfig(config)
	if err != nil {
		return nil, fmt.Errorf("parseOpenAI兼容configfailed: %v", err)
	}

	modelName := parsedConfig.ModelName
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
	}

	apiKey := parsedConfig.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	httpClient := buildThinkingHTTPClient(config, getHTTPClient())
	useMaxCompletionTokens := shouldUseMaxCompletionTokens(parsedConfig.Provider, modelName)

	// createOpenAI ChatModelconfig
	openaiConfig := &openai.ChatModelConfig{
		Model:      modelName,
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}

	if parsedConfig.BaseURL != "" {
		openaiConfig.BaseURL = parsedConfig.BaseURL
	}
	if parsedConfig.APIVersion != "" {
		openaiConfig.APIVersion = parsedConfig.APIVersion
	}
	if !useMaxCompletionTokens && parsedConfig.MaxTokens != nil && *parsedConfig.MaxTokens > 0 {
		openaiConfig.MaxTokens = parsedConfig.MaxTokens
	}
	if parsedConfig.Temperature != nil {
		openaiConfig.Temperature = parsedConfig.Temperature
	}
	if parsedConfig.TopP != nil {
		openaiConfig.TopP = parsedConfig.TopP
	}

	log.Debugf("openaiConfig: %+v", openaiConfig)

	// useeino-ext官方OpenAIimplement
	chatModel, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("createOpenAI ChatModelfailed: %v", err)
	}

	log.Infof("successfulcreateOpenAI ChatModel，model: %s", modelName)
	return chatModel, nil
}

// createOllamaChatModel createOllamaofChatModelimplement
func createOllamaChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	baseURL, _ := config["base_url"].(string)

	if modelName == "" || baseURL == "" {
		log.Warnf("model_nameandbase_urlcannot be empty，usedefaultmodel: %s", modelName)
		return nil, fmt.Errorf("model_nameandbase_urlcannot be empty")
	}

	// createOllama ChatModelconfig
	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
	}

	// useeino-ext官方Ollamaimplement
	chatModel, err := ollama.NewChatModel(ctx, ollamaConfig)
	if err != nil {
		return nil, fmt.Errorf("createOllama ChatModelfailed: %v", err)
	}

	log.Infof("successfulcreateOllama ChatModel，model: %s", modelName)
	return chatModel, nil
}

// GetModelInfo getmodelinfo
func (p *EinoLLMProvider) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model_name":      p.modelName,
		"max_tokens":      p.maxTokens,
		"streamable":      p.streamable,
		"type":            "eino",
		"provider_type":   p.providerType,
		"framework":       "eino",
		"adapter_version": "3.0.0",
		"base_url":        p.config["base_url"],
	}
}

// ResponseWithFunctions 带functioncallofrespond，useEino原生tooltype，directcallEinoResponseWithTools
func (p *EinoLLMProvider) ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan *schema.Message {

	log.Infof("[Eino-LLM] startprocess带toolofrequest - SessionID: %s, Type: %s", sessionID, p.providerType)

	logMessages(dialogue)
	// directcallEinoResponseWithToolsgetEino原生respond
	einoResponseChan := p.EinoResponseWithTools(ctx, sessionID, dialogue, functions)

	log.Infof("[Eino-LLM] toolcallrequestprocesscomplete - SessionID: %s", sessionID)

	return einoResponseChan
}

func logMessages(messages []*schema.Message) {
	for _, msg := range messages {
		if msg == nil {
			log.Debugf("history llm msg: <nil>")
			continue
		}
		log.Debugf("history llm msg: %s\n", msg.String())
	}
}

// llmExtraErrorKey and domain/llm.LLMExtraErrorKey keepconsistent，failedwhen透传erroruse（avoidloop依赖）
const llmExtraErrorKey = "error"

// sendLLMError to channel send带 Extra.error oferrormessage
func sendLLMError(ch chan *schema.Message, err error) {
	ch <- &schema.Message{
		Role:  schema.System,
		Extra: map[string]any{llmExtraErrorKey: err.Error()},
	}
}

// EinoResponseWithTools directuseEinotypeof带toolrespond
func (p *EinoLLMProvider) EinoResponseWithTools(ctx context.Context, sessionID string, messages []*schema.Message, tools []*schema.ToolInfo) chan *schema.Message {
	responseChan := make(chan *schema.Message, 200)

	var err error
	go func() {
		defer close(responseChan)
		if p.reasoningTracker != nil {
			p.reasoningTracker.Reset()
		}

		log.Infof("[Eino-LLM] startprocessEinotoolrequest - SessionID: %s, tools: %+v", sessionID, tools)

		// ifhavetool，needbindtooltoChatModel
		if len(tools) > 0 {
			p.chatModel, err = p.chatModel.WithTools(tools)
			if err != nil {
				log.Errorf("bindtoolfailed: %v", err)
				sendLLMError(responseChan, err)
				return
			}
		}

		if p.streamable {
			log.Debugf("EinoLLMProvider.EinoResponseWithTools() streamable: %t", p.streamable)
			// directuseEinoofStreammethod
			streamReader, err := p.chatModel.Stream(ctx, messages, p.buildModelCallOptions()...)
			if err != nil {
				log.Errorf("Einotoolstreamingcallfailed: %v", err)
				// to于mockimplement，ifStreamfailed，回退toGenerate
				message, genErr := p.chatModel.Generate(ctx, messages, p.buildModelCallOptions()...)
				if genErr != nil {
					log.Errorf("Einotoolgeneraterespondfailed: %v", genErr)
					sendLLMError(responseChan, genErr)
					return
				}
				if message != nil {
					responseChan <- message
				}
				return
			}

			if streamReader != nil {
				defer streamReader.Close()

				var currentToolCall *schema.ToolCall
				var toolCallBuffer string
				var isToolCallComplete bool
				var streamChunkCount int

				// processstreamingrespond
				for {
					message, err := streamReader.Recv()
					//log.Debugf("streamReader.Recv() message: %+v", message)
					if err == io.EOF {
						if streamChunkCount == 0 {
							sendLLMError(responseChan, errors.New("streamingrespondisempty"))
							break
						}
						// ifhavenotcompleteoftoolcall，send最afteratimes
						if currentToolCall != nil {
							completeMessage := &schema.Message{
								Role:      schema.Assistant,
								ToolCalls: []schema.ToolCall{*currentToolCall},
							}
							responseChan <- completeMessage
						}
						break
					}
					if err != nil {
						if ctxErr := ctx.Err(); ctxErr != nil {
							if errors.Is(ctxErr, context.Canceled) {
								log.Debugf("streamingrespondalreadycancel: %v", ctxErr)
							} else {
								log.Warnf("streamingrespondalreadyend: %v", ctxErr)
							}
							break
						}
						log.Errorf("receivestreamingrespondfailed: %v", err)
						sendLLMError(responseChan, err)
						break
					}

					if message != nil {
						streamChunkCount++
						// check ifyestoolcallofstart
						if len(message.ToolCalls) > 0 {
							toolCall := message.ToolCalls[0]

							if toolCall.Function.Name != "" {
								// 新toolcallstart
								currentToolCall = &toolCall
								toolCallBuffer = toolCall.Function.Arguments
								isToolCallComplete = false
							} else if currentToolCall != nil {
								// accumulatetoolcallparameter
								toolCallBuffer += toolCall.Function.Arguments
								currentToolCall.Function.Arguments = toolCallBuffer

								// inspectparameterwhetheryes完bodyof JSON
								if isValidJSON(toolCallBuffer) {
									isToolCallComplete = true
								}
							}

							// iftoolcall完body，sendmessage
							if isToolCallComplete {
								completeMessage := &schema.Message{
									Role:      schema.Assistant,
									ToolCalls: []schema.ToolCall{*currentToolCall},
								}
								responseChan <- completeMessage

								// resetstate
								currentToolCall = nil
								toolCallBuffer = ""
								isToolCallComplete = false
							}
						} else if message.Content != "" {
							// sendnontoolcallof普通message
							message.ToolCalls = nil
							responseChan <- message
						}
					}
				}
			} else {
				sendLLMError(responseChan, errors.New("streamingrespondisempty"))
			}
		} else {
			// directuseEinoofGeneratemethod
			message, err := p.chatModel.Generate(ctx, messages, p.buildModelCallOptions()...)
			if err != nil {
				log.Errorf("Einotoolgeneraterespondfailed: %v", err)
				sendLLMError(responseChan, err)
				return
			}

			if message != nil {
				responseChan <- message
			}
		}

		log.Infof("[Eino-LLM] Einotoolrequestprocesscomplete - SessionID: %s", sessionID)
	}()

	return responseChan
}

func (p *EinoLLMProvider) buildModelCallOptions() []model.Option {
	if p == nil || p.maxTokens <= 0 {
		return nil
	}

	provider := ""
	if p.config != nil {
		if rawProvider, ok := p.config["provider"].(string); ok {
			provider = rawProvider
		}
	}

	if shouldUseMaxCompletionTokens(provider, p.modelName) {
		return nil
	}

	return []model.Option{model.WithMaxTokens(p.maxTokens)}
}

// isValidJSON inspectcharstringwhetheryesvalidofJSON
func isValidJSON(str string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// GetChatModel getunderlyingofEino ChatModel
func (p *EinoLLMProvider) GetChatModel() model.ToolCallingChatModel {
	return p.chatModel
}

// GetProviderType get provider type
func (p *EinoLLMProvider) GetProviderType() string {
	return p.providerType
}

// WithMaxTokens setmaximumtokencount
func (p *EinoLLMProvider) WithMaxTokens(maxTokens int) *EinoLLMProvider {
	newProvider := *p
	newProvider.maxTokens = maxTokens
	return &newProvider
}

// WithStreamable setwhethersupportstreaming
func (p *EinoLLMProvider) WithStreamable(streamable bool) *EinoLLMProvider {
	newProvider := *p
	newProvider.streamable = streamable
	return &newProvider
}

// Close closeresource（nostate Provider，noneedclose）
func (p *EinoLLMProvider) Close() error {
	return nil
}

// IsValid inspectresourcewhethervalid
func (p *EinoLLMProvider) IsValid() bool {
	return p != nil && p.chatModel != nil
}
