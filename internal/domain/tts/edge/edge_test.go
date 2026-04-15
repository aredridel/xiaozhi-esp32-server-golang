package edge

import (
	"context"
	"testing"
	"time"
)

func TestEdgeTTSProvider(t *testing.T) {

	config := map[string]interface{}{
		"voice":           "zh-CN-XiaoxiaoNeural",
		"rate":            "+0%",
		"volume":          "+0%",
		"pitch":           "+0Hz",
		"connect_timeout": 10,
		"receive_timeout": 60,
	}

	provider := NewEdgeTTSProvider(config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("TestTextToSpeech", func(t *testing.T) {
		frames, err := provider.TextToSpeech(ctx, "Hello, Edge TTS test")
		if err != nil {
			t.Fatalf("TextToSpeechfailed: %v", err)
		}
		if len(frames) == 0 {
			t.Error("did not return any audio frames")
		}
	})

	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		outputChan, err := provider.TextToSpeechStream(ctx, "Hello, Edge TTS streaming test")
		if err != nil {
			t.Fatalf("TextToSpeechStreamfailed: %v", err)
		}
		var receivedFrames [][]byte
		timeout := time.After(20 * time.Second)
	ReceiveLoop:
		for {
			select {
			case frame, ok := <-outputChan:
				if !ok {
					break ReceiveLoop
				}
				receivedFrames = append(receivedFrames, frame)
			case <-timeout:
				t.Error("receive audio frame timeout")
				break ReceiveLoop
			}
		}
		if len(receivedFrames) == 0 {
			t.Error("did not receive any audio frames")
		}
	})
}
