package asr

import (
	"context"
	"fmt"

	"xiaozhi-esp32-server-golang/constants"
	"xiaozhi-esp32-server-golang/internal/domain/asr/doubao"
	"xiaozhi-esp32-server-golang/internal/domain/asr/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// Asr voice recognition interface
type AsrProvider interface {
	// Process one-time process whole segment audio, return complete recognition result
	Process(pcmData []float32) (string, error)

	// StreamingRecognize streaming recognition interface
	// input audio data through audioStream channel, recognition result obtained through returned channel
	// when audioStream is closed, indicates input end, final result will be sent through returned channel, then close this channel
	// can control recognition process cancellation and timeout through ctx
	StreamingRecognize(ctx context.Context, audioStream <-chan []float32) (chan types.StreamingResult, error)
	// Close release resources, release connection etc
	Close() error
	// IsValid check whether resource is valid
	IsValid() bool
}

// NewAsrProvider creates a new ASR instance
// asrType: ASR engine type, currently supports "funasr"
// config: ASR engine config, is map[string]interface{} type
func NewAsrProvider(asrType string, config map[string]interface{}) (AsrProvider, error) {
	// priority use provider in config, else use provider in parameter
	if configProvider, ok := config["provider"].(string); ok && configProvider != "" {
		asrType = configProvider
	}
	switch asrType {
	case constants.AsrTypeFunAsr:
		return NewFunasrAdapter(config)
	case constants.AsrTypeAliyunFunASR:
		return NewAliyunFunASRAdapter(config)
	case constants.AsrTypeDoubao:
		log.Info("using doubao ASR provider")
		provider, err := doubao.NewDoubaoV2Adapter(config)
		if err != nil {
			log.Errorf("doubao ASR adapter create failed: %v", err)
		} else {
			log.Info("doubao ASR adapter create successful")
		}
		return provider, err
	case constants.AsrTypeAliyunQwen3:
		log.Info("using aliyun Qwen3 ASR provider")
		provider, err := NewAliyunQwen3Adapter(config)
		if err != nil {
			log.Errorf("aliyun Qwen3 ASR adapter create failed: %v", err)
		} else {
			log.Info("aliyun Qwen3 ASR adapter create successful")
		}
		return provider, err
	case constants.AsrTypeXunfei:
		log.Info("using xunfei ASR provider")
		provider, err := NewXunfeiAdapter(config)
		if err != nil {
			log.Errorf("xunfei ASR adapter create failed: %v", err)
		} else {
			log.Info("xunfei ASR adapter create successful")
		}
		return provider, err
	default:
		return nil, fmt.Errorf("unsupported ASR engine type: %s, currently only supports 'funasr', 'aliyun_funasr', 'doubao', 'aliyun_qwen3', 'xunfei'", asrType)
	}
}
