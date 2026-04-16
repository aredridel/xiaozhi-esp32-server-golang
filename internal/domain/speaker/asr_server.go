package speaker

import (
	"context"
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"
)

// AsrServerProvider asr_server voiceprint recognition provider
type AsrServerProvider struct {
	streamingClient *StreamingClient
	threshold       float32 // voiceprint recognition threshold
	isActive        bool
	mutex           sync.Mutex
}

// NewAsrServerProvider create asr_server voiceprint recognition provider
func NewAsrServerProvider(config map[string]interface{}) (*AsrServerProvider, error) {
	baseURL, ok := config["base_url"].(string)
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("config missing service.base_url field")
	}

	// read threshold config, default value is 0.4
	threshold := float32(0.4)
	if thresholdVal, ok := config["threshold"]; ok {
		switch v := thresholdVal.(type) {
		case float64:
			threshold = float32(v)
		case float32:
			threshold = v
		case int:
			threshold = float32(v)
		case int64:
			threshold = float32(v)
		}
		// validate threshold range
		if threshold < 0 || threshold > 1 {
			log.Warnf("threshold %.4f exceeds valid range [0.0, 1.0], using default value 0.4", threshold)
			threshold = 0.4
		}
	}

	streamingClient := NewStreamingClient(baseURL)
	return &AsrServerProvider{
		streamingClient: streamingClient,
		threshold:       threshold,
		isActive:        false,
	}, nil
}

// StartStreaming start streaming recognize
func (p *AsrServerProvider) StartStreaming(ctx context.Context, sampleRate int, agentId string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isActive {
		return nil // already activated, return directly
	}

	err := p.streamingClient.Connect(sampleRate, agentId, p.threshold)
	if err != nil {
		log.Warnf("start voiceprint recognize stream failed: %v", err)
		return err
	}

	p.isActive = true
	log.Debugf("voiceprint recognition stream already started, sample rate: %d Hz, agent_id: %s, threshold: %.4f", sampleRate, agentId, p.threshold)
	return nil
}

// SendAudioChunk send audio chunk
func (p *AsrServerProvider) SendAudioChunk(ctx context.Context, pcmData []float32) error {
	p.mutex.Lock()
	isActive := p.isActive
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	if !isActive {
		return nil // not activated, silently ignore
	}

	err := streamingClient.SendAudioChunk(pcmData)
	if err != nil {
		log.Warnf("send audio chunk to voiceprint recognition service failed: %v", err)
		// when send failed, mark as non-active state
		p.mutex.Lock()
		p.isActive = false
		p.mutex.Unlock()
		return err
	}

	return nil
}

// FinishAndIdentify complete recognize and get result
func (p *AsrServerProvider) FinishAndIdentify(ctx context.Context) (*IdentifyResult, error) {
	p.mutex.Lock()
	if !p.isActive {
		p.mutex.Unlock()
		return nil, nil // not activated, return nil
	}
	p.isActive = false
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	result, err := streamingClient.FinishAndIdentify(ctx)

	if err != nil {
		log.Warnf("get voiceprint recognition result failed: %v", err)
		return nil, err
	}

	return result, nil
}

// PeekAndIdentify get middle recognize result (not ending current round)
// return: recognize result, whether server-side debounce, error
func (p *AsrServerProvider) PeekAndIdentify(ctx context.Context, requestID string) (*IdentifyResult, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	p.mutex.Lock()
	isActive := p.isActive
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	if !isActive {
		return nil, false, nil
	}

	result, throttled, err := streamingClient.PeekAndIdentify(ctx, requestID)
	if err != nil {
		if !streamingClient.IsConnected() {
			p.mutex.Lock()
			p.isActive = false
			p.mutex.Unlock()
		}
		log.Warnf("get voiceprint middle recognition result failed: %v", err)
		return nil, throttled, err
	}

	return result, throttled, nil
}

// Close close voiceprint provider
func (p *AsrServerProvider) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.isActive = false
	if p.streamingClient != nil {
		return p.streamingClient.Close()
	}
	return nil
}

// IsActive check if in active state
func (p *AsrServerProvider) IsActive() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.isActive
}
