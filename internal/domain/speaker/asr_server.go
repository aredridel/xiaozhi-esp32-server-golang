package speaker

import (
	"context"
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"
)

// AsrServerProvider asr_server voiceprintrecognizeprovide者
type AsrServerProvider struct {
	streamingClient *StreamingClient
	threshold       float32 // voiceprintrecognize阈value
	isActive        bool
	mutex           sync.Mutex
}

// NewAsrServerProvider create asr_server voiceprintrecognizeprovide者
func NewAsrServerProvider(config map[string]interface{}) (*AsrServerProvider, error) {
	baseURL, ok := config["base_url"].(string)
	if !ok || baseURL == "" {
		return nil, fmt.Errorf("configinMissing service.base_url field")
	}

	// read阈valueconfig，default valuesis 0.4
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
		// validate阈valuerange
		if threshold < 0 || threshold > 1 {
			log.Warnf("阈value %.4f exceedvalidrange [0.0, 1.0]，usedefault values 0.4", threshold)
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

// StartStreaming startstreaming recognize
func (p *AsrServerProvider) StartStreaming(ctx context.Context, sampleRate int, agentId string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isActive {
		return nil // alreadyactivate，directreturn
	}

	err := p.streamingClient.Connect(sampleRate, agentId, p.threshold)
	if err != nil {
		log.Warnf("start voiceprint recognize stream failed: %v", err)
		return err
	}

	p.isActive = true
	log.Debugf("voiceprintrecognizestreamalreadystart，sampling率: %d Hz, agent_id: %s, 阈value: %.4f", sampleRate, agentId, p.threshold)
	return nil
}

// SendAudioChunk sendaudio chunk
func (p *AsrServerProvider) SendAudioChunk(ctx context.Context, pcmData []float32) error {
	p.mutex.Lock()
	isActive := p.isActive
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	if !isActive {
		return nil // notactivate，silenceignore
	}

	err := streamingClient.SendAudioChunk(pcmData)
	if err != nil {
		log.Warnf("sendaudio chunktovoiceprintrecognizeservicefailed: %v", err)
		// sendfailedwhen，markisnonactivatestate
		p.mutex.Lock()
		p.isActive = false
		p.mutex.Unlock()
		return err
	}

	return nil
}

// FinishAndIdentify completerecognizeandgetresult
func (p *AsrServerProvider) FinishAndIdentify(ctx context.Context) (*IdentifyResult, error) {
	p.mutex.Lock()
	if !p.isActive {
		p.mutex.Unlock()
		return nil, nil // notactivate，return nil
	}
	p.isActive = false
	streamingClient := p.streamingClient
	p.mutex.Unlock()

	result, err := streamingClient.FinishAndIdentify(ctx)

	if err != nil {
		log.Warnf("getvoiceprintrecognizeresultfailed: %v", err)
		return nil, err
	}

	return result, nil
}

// PeekAndIdentify getmiddlerecognizeresult（noendcurrent轮times）
// return: recognizeresult, whetherbeserver-sidedebounce, error
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
		log.Warnf("getvoiceprintmiddlerecognizeresultfailed: %v", err)
		return nil, throttled, err
	}

	return result, throttled, nil
}

// Close closevoiceprintprovide者
func (p *AsrServerProvider) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.isActive = false
	if p.streamingClient != nil {
		return p.streamingClient.Close()
	}
	return nil
}

// IsActive check if处于activatestate
func (p *AsrServerProvider) IsActive() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.isActive
}
