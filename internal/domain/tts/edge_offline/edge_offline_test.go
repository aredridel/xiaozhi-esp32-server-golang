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
		// upgradeHTTPjoinisWebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgradeWebSocketfailed: %v", err)
			return
		}
		defer conn.Close()

		// readtextmessage
		_, text, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("readtextmessagefailed: %v", err)
			return
		}

		// mockaudio data
		audioData := []byte("mock audio data for: " + string(text))

		// send二systemaudio data
		err = conn.WriteMessage(websocket.BinaryMessage, audioData)
		if err != nil {
			t.Errorf("sendaudio datafailed: %v", err)
			return
		}

		// normalclosejoin
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			t.Errorf("closeWebSocketjoinfailed: %v", err)
			return
		}
	}))
}

func TestEdgeOfflineTTSProvider(t *testing.T) {
	// startmockserver
	server := mockTTSServer(t)
	defer server.Close()

	// createconfig
	config := map[string]interface{}{
		"server_url": "ws" + server.URL[4:], // will http:// replaceis ws://
		"timeout":    float64(5),            // 5secondtimeout
	}

	provider := NewEdgeOfflineTTSProvider(config)

	t.Run("TestTextToSpeech", func(t *testing.T) {
		ctx := context.Background()
		frames, err := provider.TextToSpeech(ctx, "testtext", 16000, 1, 20)
		if err != nil {
			t.Fatalf("TextToSpeechfailed: %v", err)
		}
		if len(frames) == 0 {
			t.Error("notreturn任何audio frame")
		}
	})

	t.Run("TestTextToSpeechStream", func(t *testing.T) {
		ctx := context.Background()
		outputChan, err := provider.TextToSpeechStream(ctx, "testtext", 16000, 1, 20)
		if err != nil {
			t.Fatalf("TextToSpeechStreamfailed: %v", err)
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
				t.Error("receiveaudio frametimeout")
				break ReceiveLoop
			}
		}

		if len(receivedFrames) == 0 {
			t.Error("notreceiveto任何audio frame")
		}
	})

	t.Run("TestInvalidServerURL", func(t *testing.T) {
		provider := NewEdgeOfflineTTSProvider(map[string]interface{}{
			"server_url": "ws://invalid-server:12345",
			"timeout":    float64(1),
		})

		ctx := context.Background()
		_, err := provider.TextToSpeech(ctx, "testtext", 16000, 1, 20)
		if err == nil {
			t.Error("期望joininvalidserverwhenreturnerror")
		}
	})

	t.Run("TestTimeout", func(t *testing.T) {
		// create awilldelayrespondofserver
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgradeWebSocketfailed: %v", err)
				return
			}
			defer conn.Close()

			// delay2second
			time.Sleep(2 * time.Second)
		}))
		defer slowServer.Close()

		provider := NewEdgeOfflineTTSProvider(map[string]interface{}{
			"server_url": "ws" + slowServer.URL[4:],
			"timeout":    float64(1), // 1secondtimeout
		})

		ctx := context.Background()
		_, err := provider.TextToSpeech(ctx, "testtext", 16000, 1, 20)
		if err == nil {
			t.Error("期望timeoutwhenreturnerror")
		}
	})
}
