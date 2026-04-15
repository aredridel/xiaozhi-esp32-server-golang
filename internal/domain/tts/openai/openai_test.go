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
	// skipactualofnetworkrequesttest，除nonsetenvironmentvariable
	if os.Getenv("RUN_OPENAI_TEST") != "1" {
		t.Skip("skipOpenAI APItest，setenvironmentvariableRUN_OPENAI_TEST=1以启use")
	}

	// fromenvironmentvariablegetAPIkey
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("skipOpenAI APItest，needsetenvironmentvariableOPENAI_API_KEY")
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

	// testtext转voice
	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		frames, err := provider.TextToSpeech(ctx, "Hello, this is a test of OpenAI text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeechfailed: %v", err)
		}

		if len(frames) == 0 {
			t.Error("notreturn任何audio frame")
		}

		t.Logf("successfulgenerate %d 个audio frame", len(frames))
	})

	// teststreamingtext转voice
	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outputChan, err := provider.TextToSpeechStream(ctx, "Hello, this is a test of OpenAI streaming text to speech.", 16000, 1, 60)
		if err != nil {
			t.Fatalf("TextToSpeechStreamfailed: %v", err)
		}

		// receiveallframe
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
				t.Error("receiveaudio frametimeout")
				break receiveLoop
			}
		}

		if len(receivedFrames) == 0 {
			t.Error("notreceiveto任何audio frame")
		}

		t.Logf("successfulreceive %d 个audio frame", len(receivedFrames))
	})

	// testnoat the same timeofvoice
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
					t.Errorf("usevoice %s failed: %v", voice, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("voice %s notreturn任何audio frame", voice)
				}

				t.Logf("voice %s successfulgenerate %d 个audio frame", voice, len(frames))
			})
		}
	})

	// testnoat the same timeofspeed
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
					t.Errorf("usespeed %.1f failed: %v", speed, err)
					return
				}

				if len(frames) == 0 {
					t.Errorf("speed %.1f notreturn任何audio frame", speed)
				}

				t.Logf("speed %.1f successfulgenerate %d 个audio frame", speed, len(frames))
			})
		}
	})
}

// TestOpenAITTSProviderDefaults testdefault values
func TestOpenAITTSProviderDefaults(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider := NewOpenAITTSProvider(config)

	if provider.APIURL != "https://api.openai.com/v1/audio/speech" {
		t.Errorf("期望defaultAPI URLis https://api.openai.com/v1/audio/speech，actualis %s", provider.APIURL)
	}

	if provider.Model != "tts-1" {
		t.Errorf("期望defaultmodelis tts-1，actualis %s", provider.Model)
	}

	if provider.Voice != "alloy" {
		t.Errorf("期望defaultvoiceis alloy，actualis %s", provider.Voice)
	}

	if provider.ResponseFormat != "mp3" {
		t.Errorf("期望defaultrespondformatis mp3，actualis %s", provider.ResponseFormat)
	}

	if provider.Speed != 1.0 {
		t.Errorf("期望defaultspeedis 1.0，actualis %.1f", provider.Speed)
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
		t.Fatalf("generatetest Ogg Opus failed: %v", err)
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
			requestErrCh <- fmt.Errorf("期望 response_format=opus，actualis %s", req.ResponseFormat)
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
		t.Fatalf("TextToSpeechStream returnerror: %v", err)
	}

	frameCount := 0
	for frame := range outputChan {
		if len(frame) == 0 {
			t.Fatal("receiveempty Opus frame")
		}
		frameCount++
	}

	if frameCount == 0 {
		t.Fatal("not receivedto任何 Opus frame")
	}

	select {
	case err := <-requestErrCh:
		t.Fatalf("mock server verifyfailed: %v", err)
	default:
	}
}
