package cosyvoice

import (
	"os"
	"testing"
	"time"
)

func TestCosyVoiceTTS(t *testing.T) {
	// skip actual network request test unless environment variable is set
	if os.Getenv("RUN_COSYVOICE_TEST") != "1" {
		t.Skip("skip CosyVoice API test, set environment variable RUN_COSYVOICE_TEST=1 to enable")
	}

	config := map[string]interface{}{
		"api_url":        "https://cosyvoice.com/tts",
		"spk_id":         "OUeAo1mhq6IBExi",
		"frame_duration": float64(60),
		"target_sr":      float64(16000),
		"audio_format":   "mp3",
		"instruct_text":  "Hello",
	}

	provider := NewCosyVoiceTTSProvider(config)

	// test text to speech
	t.Run("TestTextToSpeech", func(t *testing.T) {
		frames, err := provider.TextToSpeech("Can you speak Sichuan dialect?")
		if err != nil {
			t.Fatalf("TextToSpeech failed: %v", err)
		}

		if len(frames) == 0 {
			t.Error("did not return any audio frames")
		}
	})

	// test streaming text to speech
	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		outputChan, cancel, err := provider.TextToSpeechStream("Can you speak Sichuan dialect?")
		if err != nil {
			t.Fatalf("TextToSpeechStream failed: %v", err)
		}

		defer cancel()

		// receive all frames
		var receivedFrames [][]byte
		timeout := time.After(10 * time.Second)

	receiveLoop:
		for {
			select {
			case frame, ok := <-outputChan:
				if !ok {
					break receiveLoop
				}
				receivedFrames = append(receivedFrames, frame)
			case <-timeout:
				t.Error("receive audio frame timeout")
				break receiveLoop
			}
		}

		if len(receivedFrames) == 0 {
			t.Error("did not receive any audio frames")
		}
	})
}
