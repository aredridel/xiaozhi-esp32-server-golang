package edge_offline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// mock TTS WebSocket server
func mockTTSServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// upgrade HTTP join is WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade WebSocket failed: %v", err)
			return
		}
		defer conn.Close()

		// read text message
		_, text, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read text message failed: %v", err)
			return
		}

		// mock audio data
		audioData := []byte("mock audio data for: " + string(text))

		// send binary audio data
		err = conn.WriteMessage(websocket.BinaryMessage, audioData)
		if err != nil {
			t.Errorf("send audio data failed: %v", err)
			return
		}

		// normal close join
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			t.Errorf("close WebSocket join failed: %v", err)
			return
		}
	}))
}

func TestEdgeOfflineTTSProvider(t *testing.T) {
	// start mock server
	server := mockTTSServer(t)
	defer server.Close()

	// create config
	config := map[string]interface{}{
		"server_url": "ws" + server.URL[4:], // will http:// replace is ws://
		"timeout":    float64(5),            // 5 second timeout
	}

	provider := NewEdgeOfflineTTSProvider(config)

	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx := context.Background()
		frames, err := provider.TextToSpeech(ctx, "test text", 16000, 1, 20)
		if err != nil {
			t.Fatalf("TextToSpeech failed: %v", err)
		}
		if len(frames) == 0 {
			t.Error("not return any audio frame")
		}
	})

	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx := context.Background()
		outputChan, err := provider.TextToSpeechStream(ctx, "test text", 16000, 1, 20)
		if err != nil {
			t.Fatalf("TextToSpeechStream failed: %v", err)
		}

		var receivedFrames [][]byte
		timeout := time.After(5 * time.Second)

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
			t.Error("not receive any audio frame")
		}
	})

	t.Run("TestInvalidServerURL", func(t *testing.T) {
		provider := NewEdgeOfflineTTSProvider(map[string]interface{}{
			"server_url": "ws://invalid-server:12345",
			"timeout":    float64(1),
		})

		ctx := context.Background()
		_, err := provider.TextToSpeech(ctx, "test text", 16000, 1, 20)
		if err == nil {
			t.Error("expect join invalid server when return error")
		}
	})

	t.Run("TestTimeout", func(t *testing.T) {
		// create a will delay respond of server
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgrade WebSocket failed: %v", err)
				return
			}
			defer conn.Close()

			// delay 2 second
			time.Sleep(2 * time.Second)
		}))
		defer slowServer.Close()

		provider := NewEdgeOfflineTTSProvider(map[string]interface{}{
			"server_url": "ws" + slowServer.URL[4:],
			"timeout":    float64(1), // 1 second timeout
		})

		ctx := context.Background()
		_, err := provider.TextToSpeech(ctx, "test text", 16000, 1, 20)
		if err == nil {
			t.Error("expect timeout when return error")
		}
	})
}
