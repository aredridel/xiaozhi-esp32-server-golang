package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
)

func TestOpenAITTS(t *testing.T) {
	// skip actual network request test, unless environment variable is set
	if os.Getenv("RUN_OPENAI_TEST") != "1" {
		t.Skip("skip OpenAI API test, set environment variable RUN_OPENAI_TEST=1 to enable")
	}

	// get API key from environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("skip OpenAI API test, need to set environment variable OPENAI_API_KEY")
	}

	config := map[string]interface{}{
		"api_key":         apiKey,
		"api_url":         "https://api.openai.com/v1/audio/speech",
		"model":           "tts-1",
		"voice":           "alloy",
		"response_format": "mp3",
		"speed":           1.0,
		"frame_duration":  float64(60),
	}

	provider := NewOpenAITTSProvider(config)

	// test text to speech
	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		frames, err := provider.TextToSpeech(ctx, "Hello, this is a test of OpenAI text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeech failed: %v", err)
		}

		if len(frames) == 0 {
			t.Error("did not return any audio frames")
		}

		t.Logf("Successfully generated %d audio frames", len(frames))
	})

	// test streaming text to speech
	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outputChan, err := provider.TextToSpeechStream(ctx, "Hello, this is a test of OpenAI streaming text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeechStream failed: %v", err)
		}

		// receive all frames
		var receivedFrames [][]byte
		timeout := time.After(20 * time.Second)

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

		t.Logf("Successfully received %d audio frames", len(receivedFrames))
	})

	// test different voices
	t.Run("TestDifferentVoices", func(t *testing.T) {
		voices := []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}

		for _, voice := range voices {
			t.Run(voice, func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           voice,
					"response_format": "mp3",
					"speed":           1.0,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing voice: "+voice, 16000, 1, 60)
				if err != nil {
					t.Errorf("use voice %s failed: %v", voice, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("voice %s did not return any audio frames", voice)
				}

				t.Logf("voice %s successfully generated %d audio frames", voice, len(frames))
			})
		}
	})

	// test different speeds
	t.Run("TestDifferentSpeeds", func(t *testing.T) {
		speeds := []float64{0.5, 1.0, 1.5, 2.0}

		for _, speed := range speeds {
			t.Run(string(rune(speed)), func(t *testing.T) {
				config := map[string]interface{}{
					"api_key":         apiKey,
					"model":           "tts-1",
					"voice":           "alloy",
					"response_format": "mp3",
					"speed":           speed,
				}

				provider := NewOpenAITTSProvider(config)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				frames, err := provider.TextToSpeech(ctx, "Testing speed", 16000, 1, 60)
				if err != nil {
					t.Errorf("use speed %.1f failed: %v", speed, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("speed %.1f did not return any audio frames", speed)
				}

				t.Logf("speed %.1f successfully generated %d audio frames", speed, len(frames))
			})
		}
	})
}

// TestOpenAITTSProviderDefaults test default values
func TestOpenAITTSProviderDefaults(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider := NewOpenAITTSProvider(config)

	if provider.APIURL != "https://api.openai.com/v1/audio/speech" {
		t.Errorf("expected default API URL is https://api.openai.com/v1/audio/speech, actual is %s", provider.APIURL)
	}

	if provider.Model != "tts-1" {
		t.Errorf("expected default model is tts-1, actual is %s", provider.Model)
	}

	if provider.Voice != "alloy" {
		t.Errorf("expected default voice is alloy, actual is %s", provider.Voice)
	}

	if provider.ResponseFormat != "mp3" {
		t.Errorf("expected default response format is mp3, actual is %s", provider.ResponseFormat)
	}

	if provider.Speed != 1.0 {
		t.Errorf("expected default speed is 1.0, actual is %.1f", provider.Speed)
	}
}

func TestOpenAITTSProviderSupportsOpusResponse(t *testing.T) {
	sampleRate := 16000
	pcm := make([]int16, sampleRate/2)
	for i := range pcm {
		if i%32 < 16 {
			pcm[i] = 2400
		} else {
			pcm[i] = -2400
		}
	}

	opusBytes, err := util.PCM16ToOggOpus(pcm, sampleRate, 1, 20)
	if err != nil {
		t.Fatalf("generate test Ogg Opus failed: %v", err)
	}

	requestErrCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			requestErrCh <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.ResponseFormat != "opus" {
			requestErrCh <- fmt.Errorf("expected response_format=opus, actual is %s", req.ResponseFormat)
			http.Error(w, "unexpected response_format", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "audio/ogg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(opusBytes)
	}))
	defer server.Close()

	provider := NewOpenAITTSProvider(map[string]interface{}{
		"api_url":         server.URL,
		"model":           "tts-1",
		"voice":           "alloy",
		"response_format": "opus",
		"speed":           1.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outputChan, err := provider.TextToSpeechStream(ctx, "test opus output", sampleRate, 1, 60)
	if err != nil {
		t.Fatalf("TextToSpeechStream return error: %v", err)
	}

	frameCount := 0
	for frame := range outputChan {
		if len(frame) == 0 {
			t.Fatal("received empty Opus frame")
		}
		frameCount++
	}

	if frameCount == 0 {
		t.Fatal("did not receive any Opus frames")
	}

	select {
	case err := <-requestErrCh:
		t.Fatalf("mock server verify failed: %v", err)
	default:
	}
}
