package tts

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"xiaozhi-esp32-server-golang/constants"
	"xiaozhi-esp32-server-golang/internal/domain/tts/cosyvoice"
	"xiaozhi-esp32-server-golang/internal/domain/tts/doubao"
	"xiaozhi-esp32-server-golang/internal/domain/tts/edge"
	"xiaozhi-esp32-server-golang/internal/domain/tts/edge_offline"
	"xiaozhi-esp32-server-golang/internal/domain/tts/minimax"
	"xiaozhi-esp32-server-golang/internal/domain/tts/openai"
	"xiaozhi-esp32-server-golang/internal/domain/tts/qwen"
	"xiaozhi-esp32-server-golang/internal/domain/tts/streaming"
	"xiaozhi-esp32-server-golang/internal/domain/tts/xiaozhi"
	"xiaozhi-esp32-server-golang/internal/domain/tts/xunfei"
	"xiaozhi-esp32-server-golang/internal/domain/tts/xunfei_super_tts"
	"xiaozhi-esp32-server-golang/internal/domain/tts/zhipu"
)

// foundationTTSprovide者interface（no含Contextmethod）
type BaseTTSProvider interface {
	TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error)
	TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error)
}

// DualStreamProvider both TTS input and output are streamingofoptionalinterface：synthesize output while receiving text。Provider ifsupportthenimplementthisinterface。
type DualStreamProvider interface {
	StreamingSynthesize(ctx context.Context, textChan <-chan string, sampleRate int, channels int, frameDuration int) (outputChan chan streaming.SynthesisEvent, err error)
}

// 完bodyTTSprovide者interface（includeContextmethod）
type TTSProvider interface {
	BaseTTSProvider
	// SetVoice dynamicsetvoiceparameter
	// voiceConfig: includevoicerelevantconfigof map，如 {"voice": "xxx"} or {"spk_id": "xxx"}
	SetVoice(voiceConfig map[string]interface{}) error
	// Close closeresource，releasejoinetc
	Close() error
	// IsValid inspectresourcewhethervalid（joinwhether存活etc）
	IsValid() bool
}

// GetTTSProvider geta完bodyofTTSprovide者（supportContext）
// providerName: mayyes config_id/provider orresourcepool key（如 "edge_tts:zh-CN-XiaoxiaoNeural"）
// config: fromdatalibraryconfigs表ofjson_datafieldparseofconfigmap
// priorityuse config inof provider field，elsefrom providerName parse（取 ":" beforepart）
func GetTTSProvider(providerName string, config map[string]interface{}) (TTSProvider, error) {
	effectiveName := providerName
	if configProvider, ok := config["provider"].(string); ok && configProvider != "" {
		effectiveName = configProvider
	}
	// resourcepool key formatis "provider:voiceID"，取before半partasisprovide者type
	if idx := strings.Index(effectiveName, ":"); idx > 0 {
		effectiveName = effectiveName[:idx]
	}
	var baseProvider BaseTTSProvider

	switch effectiveName {
	case constants.TtsTypeDoubao:
		baseProvider = doubao.NewDoubaoTTSProvider(config)
	case constants.TtsTypeDoubaoWS:
		baseProvider = doubao.NewDoubaoWSProvider(config)
	case constants.TtsTypeCosyvoice:
		baseProvider = cosyvoice.NewCosyVoiceTTSProvider(config)
	case constants.TtsTypeEdge:
		baseProvider = edge.NewEdgeTTSProvider(config)
	case constants.TtsTypeEdgeOffline:
		baseProvider = edge_offline.NewEdgeOfflineTTSProvider(config)
	case constants.TtsTypeXiaozhi:
		baseProvider = xiaozhi.NewXiaozhiProvider(config)
	case constants.TtsTypeXunfei:
		baseProvider = xunfei.NewXunfeiTTSProvider(config)
	case constants.TtsTypeXunfeiSuper:
		baseProvider = xunfei_super_tts.NewXunfeiSuperTTSProvider(config)
	case constants.TtsTypeOpenAI:
		baseProvider = openai.NewOpenAITTSProvider(config)
	case constants.TtsTypeZhipu:
		baseProvider = zhipu.NewZhipuTTSProvider(config)
	case constants.TtsTypeMinimax:
		baseProvider = minimax.NewMinimaxTTSProvider(config)
	case constants.TtsTypeAliyunQwen:
		baseProvider = qwen.NewQwenTTSProvider(config)
	case constants.TtsTypeIndexTTSVLLM:
		baseProvider = openai.NewOpenAITTSProvider(buildIndexTTSOpenAIConfig(config))
	default:
		return nil, fmt.Errorf("unsupportedofTTSprovide者: %s", effectiveName)
	}

	if baseProvider == nil {
		return nil, fmt.Errorf("no法createTTSprovide者: %s", effectiveName)
	}

	// useadapterpackage装foundationprovide者，convertis完bodyofTTSProvider
	provider := &ContextTTSAdapter{baseProvider}

	return provider, nil
}

func buildIndexTTSOpenAIConfig(config map[string]interface{}) map[string]interface{} {
	const (
		defaultIndexTTSURL   = "http://127.0.0.1:7860/audio/speech"
		defaultIndexTTSModel = "indextts-vllm"
	)

	normalized := make(map[string]interface{}, len(config)+4)
	for k, v := range config {
		normalized[k] = v
	}

	apiURL, _ := normalized["api_url"].(string)
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = defaultIndexTTSURL
	} else {
		parsed, err := url.Parse(apiURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			trimmed := strings.TrimRight(apiURL, "/")
			if !strings.HasSuffix(strings.ToLower(trimmed), "/audio/speech") {
				trimmed += "/audio/speech"
			}
			apiURL = trimmed
		} else {
			if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
				parsed.Path = "/audio/speech"
				parsed.RawPath = ""
				apiURL = parsed.String()
			}
		}
	}
	normalized["api_url"] = strings.TrimRight(apiURL, "/")

	if model, _ := normalized["model"].(string); strings.TrimSpace(model) == "" {
		normalized["model"] = defaultIndexTTSModel
	}
	if responseFormat, _ := normalized["response_format"].(string); strings.TrimSpace(responseFormat) == "" {
		normalized["response_format"] = "wav"
	}
	if _, exists := normalized["stream"]; !exists {
		normalized["stream"] = false
	}
	if _, exists := normalized["speed"]; !exists {
		normalized["speed"] = float64(1.0)
	}

	return normalized
}

// ContextTTSAdapter yesaadapter，isfoundationTTSprovide者addContextsupport
type ContextTTSAdapter struct {
	Provider BaseTTSProvider
}

// StreamingSynthesize proxytooriginalprovide者ofdual-stream合成interface
func (a *ContextTTSAdapter) StreamingSynthesize(ctx context.Context, textChan <-chan string, sampleRate int, channels int, frameDuration int) (outputChan chan streaming.SynthesisEvent, err error) {
	// inspectunderlying Provider whethersupportdual-stream
	if dsProvider, ok := a.Provider.(DualStreamProvider); ok {
		return dsProvider.StreamingSynthesize(ctx, textChan, sampleRate, channels, frameDuration)
	}
	return nil, fmt.Errorf("underlying Provider unsupporteddual-stream合成")
}

// TextToSpeech proxytooriginalprovide者
func (a *ContextTTSAdapter) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	return a.Provider.TextToSpeech(ctx, text, sampleRate, channels, frameDuration)
}

// TextToSpeechStream proxytooriginalprovide者
func (a *ContextTTSAdapter) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, err error) {
	return a.Provider.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
}

// SetVoice proxytounderlying Provider of SetVoice method
func (a *ContextTTSAdapter) SetVoice(voiceConfig map[string]interface{}) error {
	// ifunderlying Provider implement SetVoice method，directcall
	if setter, ok := a.Provider.(interface {
		SetVoice(map[string]interface{}) error
	}); ok {
		return setter.SetVoice(voiceConfig)
	}
	// elsereturnunsupportedoferror
	return fmt.Errorf("underlying Provider unsupported SetVoice method")
}

// TextToSpeechWithContext useContextversionoftext转voice
func (a *ContextTTSAdapter) TextToSpeechWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	// inspectprovide者whetherdirectsupportContextversion
	if provider, ok := a.Provider.(interface {
		TextToSpeechWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error)
	}); ok {
		// provide者directsupportContextversion
		return provider.TextToSpeechWithContext(ctx, text, sampleRate, channels, frameDuration)
	}

	// elseusestandardversion，andthroughgoroutineandchannelimplementcontextcontrol
	resultChan := make(chan struct {
		frames [][]byte
		err    error
	})

	go func() {
		frames, err := a.Provider.TextToSpeech(ctx, text, sampleRate, channels, frameDuration)
		select {
		case <-ctx.Done():
			// contextalreadycancel，do not sendresult
			return
		case resultChan <- struct {
			frames [][]byte
			err    error
		}{frames, err}:
			// resultalreadysend
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.frames, result.err
	}
}

// TextToSpeechStreamWithContext useContextversionofstreamingtext转voice
func (a *ContextTTSAdapter) TextToSpeechStreamWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (outputChan chan []byte, cancelFunc func(), err error) {
	// inspectprovide者whetherdirectsupportContextversion
	if provider, ok := a.Provider.(interface {
		TextToSpeechStreamWithContext(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, func(), error)
	}); ok {
		// provide者directsupportContextversion
		return provider.TextToSpeechStreamWithContext(ctx, text, sampleRate, channels, frameDuration)
	}

	// elseusestandardversion，butcreate apackage装器来processcontextcancel
	streamChan, err := a.Provider.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, nil, err
	}

	// create anewoutputchannel，used forforwardandprocesscancel
	outputChan = make(chan []byte, 10)

	// create agoroutine来forwarddataandlistencontextcancel
	go func() {
		defer close(outputChan)

		for {
			select {
			case <-ctx.Done():
				// contextalreadycancel，calloriginalcancelfunctionandexit
				cancelFunc()
				return
			case frame, ok := <-streamChan:
				if !ok {
					// originalchannel closed
					return
				}
				// forwarddata
				select {
				case <-ctx.Done():
					// contextalreadycancel
					cancelFunc()
					return
				case outputChan <- frame:
					// successfulforwarddata
				}
			}
		}
	}()

	return outputChan, cancelFunc, nil
}

// Close closeresource
func (a *ContextTTSAdapter) Close() error {
	// ifunderlying Provider implement Close method，directcall
	if closer, ok := a.Provider.(interface {
		Close() error
	}); ok {
		return closer.Close()
	}
	return nil
}

// IsValid inspectresourcewhethervalid
func (a *ContextTTSAdapter) IsValid() bool {
	// ifunderlying Provider implement IsValid method，directcall
	if validator, ok := a.Provider.(interface {
		IsValid() bool
	}); ok {
		return validator.IsValid()
	}
	// elseinspect Provider whetheris nil
	return a.Provider != nil
}
