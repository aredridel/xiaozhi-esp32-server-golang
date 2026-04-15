package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/tts"
	"xiaozhi-esp32-server-golang/internal/util"

	"github.com/gorilla/websocket"
)

var detectStartTs int64
var waitInput = make(chan struct{}, 1)
var status = "idle"

// Message type constants
const (
	MessageTypeHello  = "hello"
	MessageTypeListen = "listen"
	MessageTypeAbort  = "abort"
	MessageTypeIot    = "iot"
	MessageTypeMcp    = "mcp"
)

// Message state constants
const (
	MessageStateStart   = "start"
	MessageStateStop    = "stop"
	MessageStateDetect  = "detect"
	MessageStateSuccess = "success"
	MessageStateError   = "error"
	MessageStateAbort   = "abort"
)

// ClientMessage represents client message
type ClientMessage struct {
	Type        string          `json:"type"`
	DeviceID    string          `json:"device_id"`
	Text        string          `json:"text,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	State       string          `json:"state,omitempty"`
	Token       string          `json:"token,omitempty"`
	DeviceMac   string          `json:"device_mac,omitempty"`
	Version     int             `json:"version,omitempty"`
	Transport   string          `json:"transport,omitempty"`
	AudioParams *AudioFormat    `json:"audio_params,omitempty"`
	Features    map[string]bool `json:"features,omitempty"`
	PayLoad     json.RawMessage `json:"payload,omitempty"`
}

// ServerMessage represents server message
type ServerMessage struct {
	Type        string          `json:"type"`
	Text        string          `json:"text,omitempty"`
	State       string          `json:"state,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	Transport   string          `json:"transport,omitempty"`
	AudioFormat *AudioFormat    `json:"audio_format,omitempty"`
	PayLoad     json.RawMessage `json:"payload,omitempty"`
}

// AudioFormat represents audio format
type AudioFormat struct {
	SampleRate    int    `json:"sample_rate"`
	Channels      int    `json:"channels"`
	FrameDuration int    `json:"frame_duration"`
	Format        string `json:"format"`
}

// Opus encoding constants
var (
	// Sample rate for Opus encoding
	SampleRate = 16000
	// Number of audio channels
	Channels = 1
	// Frame duration in milliseconds
	FrameDurationMs = 20
	// PCM buffer size = sample_rate * channels * frame_duration(seconds)
	PCMBufferSize = SampleRate * Channels * FrameDurationMs / 1000

	mode = "auto"

	addMcp = false
)

var speectText = "Hello test"
var clientId = "e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d"
var token = "test-token"
var ttsProviderName = "edge_offline"

// serverVisionURL vision interface address parsed from server MCP initialize message
var (
	serverVisionURL   string
	serverVisionURLMu sync.RWMutex
)

// parseAndSaveVisionURL parses params.capabilities.vision.url from MCP initialize payload and saves it
func parseAndSaveVisionURL(payload json.RawMessage) {
	var mcpMsg struct {
		Method string `json:"method"`
		Params struct {
			Capabilities struct {
				Vision struct {
					URL string `json:"url"`
				} `json:"vision"`
			} `json:"capabilities"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &mcpMsg); err != nil {
		return
	}
	if mcpMsg.Method == "initialize" && mcpMsg.Params.Capabilities.Vision.URL != "" {
		serverVisionURLMu.Lock()
		serverVisionURL = mcpMsg.Params.Capabilities.Vision.URL
		serverVisionURLMu.Unlock()
		fmt.Printf("Saved server vision_url: %s\n", serverVisionURL)
	}
}

// GetServerVisionURL returns the vision interface address from server, returns empty string if not sent
func GetServerVisionURL() string {
	serverVisionURLMu.RLock()
	defer serverVisionURLMu.RUnlock()
	return serverVisionURL
}

func main() {
	// Parse command line arguments
	serverAddr := flag.String("server", "ws://localhost:8989/xiaozhi/v1/", "Server address")
	deviceID := flag.String("device", "test-device-001", "Device ID")
	audioFile := flag.String("audio", "", "Audio file path")
	text := flag.String("text", "Hello test", "Text")
	modeFlag := flag.String("mode", "auto", "Mode")
	ttsProviderFlag := flag.String("tts_provider", "edge_offline", "TTS provider (edge_offline|edge|cosyvoice)")
	sampleRate := flag.Int("sample_rate", 16000, "sampleRate")
	frameDurationsMs := flag.Int("frame_ms", 20, "frame duration ms")
	addMcpFlag := flag.Bool("mcp", false, "Enable MCP")

	flag.Parse()

	fmt.Printf("Running Xiaozhi client\nServer: %s\nDevice ID: %s\nAudio file: %s\n",
		*serverAddr, *deviceID, *audioFile)

	speectText = *text
	SampleRate = *sampleRate
	FrameDurationMs = *frameDurationsMs
	mode = *modeFlag
	ttsProviderName = strings.TrimSpace(*ttsProviderFlag)
	addMcp = *addMcpFlag

	// Run client
	if err := runClient(*serverAddr, *deviceID, *audioFile); err != nil {
		log.Fatalf("Client run failed: %v", err)
	}
}

var OpusData [][]byte
var firstRecvFrame bool

// runClient runs Xiaozhi client
func runClient(serverAddr, deviceID, audioFile string) error {
	OpusData = [][]byte{}
	// Build WebSocket URL
	wsURL := serverAddr
	fmt.Printf("Connecting to server: %s\n", wsURL)

	// Set HTTP headers
	header := http.Header{}
	header.Set("Device-Id", deviceID)
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Bearer "+token)
	header.Set("Protocol-Version", "1")
	header.Set("Client-Id", clientId)

	// Connect to WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("Connection failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to server")

	mcpSendMsgChan := make(chan []byte, 10)
	mcpRecvMsgChan := make(chan []byte, 10)

	go func() {
		for msg := range mcpSendMsgChan {
			fmt.Printf("Sending MCP message: %s\n", string(msg))
			/*respMsg := ClientMessage{
				Type:     MessageTypeMcp,
				DeviceID: deviceID,
				PayLoad:  msg,
			}
			jsonByte, _ := json.Marshal(respMsg)*/
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	// Set up message handling
	done := make(chan struct{})

	var startTs int64
	_ = startTs
	// Start a goroutine to handle messages received from server
	go func() {
		var iLock sync.Mutex
		defer close(done)
		//var recvInterval int64
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Failed to read message: %v\n", err)
				return
			}

			if messageType == websocket.TextMessage {
				fmt.Printf("Received server message: %+v\n", string(message))
				var serverMsg ServerMessage
				if err := json.Unmarshal(message, &serverMsg); err != nil {
					fmt.Printf("Failed to parse message: %v\n", err)
					continue
				}

				if serverMsg.Type == "mcp" {
					// Parse server vision_url from MCP initialize message
					if len(serverMsg.PayLoad) > 0 {
						parseAndSaveVisionURL(serverMsg.PayLoad)
					}
					select {
					case mcpRecvMsgChan <- serverMsg.PayLoad:
					default:
						fmt.Printf("MCP message queue full, dropping message: %s\n", string(serverMsg.PayLoad))
					}
				}

				if serverMsg.Type == "tts" && serverMsg.State == "stop" {
					//OpusToWav(OpusData, 24000, 1, "ws_output_24000.wav")
					select {
					case waitInput <- struct{}{}:
					default:
					}
				}
			} else if messageType == websocket.BinaryMessage {
				if !firstRecvFrame {
					iLock.Lock()
					if !firstRecvFrame {
						firstRecvFrame = true
						fmt.Printf("First frame arrival time: %d ms\n", time.Now().UnixMilli()-detectStartTs)
					}
					iLock.Unlock()
					//os.WriteFile("ws_output_first_frame.wav", message, 0644)
				}
				OpusData = append(OpusData, message)
				//fmt.Printf("Received audio data: %d bytes, interval: %d ms\n", len(message), time.Now().UnixMilli()-recvInterval)
				//recvInterval = time.Now().UnixMilli()
			}
		}
	}()

	go func() {
		NewMcpServer(mcpSendMsgChan, mcpRecvMsgChan)
	}()

	// Send hello message
	helloMsg := ClientMessage{
		Type:      MessageTypeHello,
		DeviceID:  deviceID,
		Transport: "websocket",
		Version:   1,
		Features:  map[string]bool{},
		AudioParams: &AudioFormat{
			SampleRate:    SampleRate,
			Channels:      Channels,
			FrameDuration: FrameDurationMs,
			Format:        "opus",
		},
	}

	if addMcp {
		helloMsg.Features["mcp"] = true
	}

	if err := sendJSONMessage(conn, helloMsg); err != nil {
		return fmt.Errorf("Failed to send hello message: %v", err)
	}

	// Wait for server response
	time.Sleep(1 * time.Second)

	// If audio file is specified, send it
	if audioFile != "" {
		// Send listen start auto signal
		if err := sendListenStart(conn, deviceID, "auto"); err != nil {
			return fmt.Errorf("Failed to send listen start message: %v", err)
		}
		fmt.Println("Sent listen start auto signal")

		// Wait a short time to ensure server is ready to receive audio
		time.Sleep(100 * time.Millisecond)

		fmt.Println("Starting to send audio data...")
		// Read and send audio file (using Opus encoding)
		if err := sendWavFileWithOpusEncoding(conn, audioFile); err != nil {
			return fmt.Errorf("Failed to send audio data: %v\n", err)
		}
		fmt.Println("Audio data sent, waiting for server response...")
		// Wait 10 seconds then exit
		time.Sleep(10 * time.Second)
		return nil
	}

	// If no audio file specified, use TTS mode
	waitInput <- struct{}{}
	if err := sendTextToSpeech(conn, deviceID); err != nil {
		return fmt.Errorf("Failed to send text to speech: %v", err)
	}
	/*
		for i := 0; i < 1; i++ {
			detectStartTs = time.Now().UnixMilli()
			if err := sendListenDetect(conn, deviceID, speectText); err != nil {
				return fmt.Errorf("Failed to send text: %v", err)
			}
			time.Sleep(5000 * time.Millisecond)
		}

		saveOpusData()
		OpusToWav(OpusData, 24000, 1, "ws_output_24000.wav")
	*/

	time.Sleep(100 * time.Millisecond)
	// Send listen stop message
	/*listenStopMsg := ClientMessage{
		Type:     MessageTypeListen,
		DeviceID: deviceID,
		State:    MessageStateStop,
	}

	if err := sendJSONMessage(conn, listenStopMsg); err != nil {
		return fmt.Errorf("Failed to send listen stop message: %v", err)
	}*/

	fmt.Println("Stop message sent, waiting for server response...")

	// Wait for a period to receive server response
	time.Sleep(30 * time.Second)

	return nil
}

func saveOpusData() error {
	f, err := os.Create("opus_ws.data")
	if err != nil {
		return err
	}
	defer f.Close()

	for _, data := range OpusData {
		f.Write(data)
	}

	f.Close()

	return nil
}

func sendListenStart(conn *websocket.Conn, deviceID string, mode string) error {
	// Send listen start message
	listenStartMsg := ClientMessage{
		Type:     MessageTypeListen,
		DeviceID: deviceID,
		State:    MessageStateStart,
		Mode:     mode,
	}

	if err := sendJSONMessage(conn, listenStartMsg); err != nil {
		return fmt.Errorf("Failed to send listen start message: %v", err)
	}
	return nil
}

func sendListenStop(conn *websocket.Conn, deviceID string) error {
	// Send listen start message
	listenStartMsg := ClientMessage{
		Type:     MessageTypeListen,
		DeviceID: deviceID,
		State:    MessageStateStop,
		Mode:     "manual",
	}

	if err := sendJSONMessage(conn, listenStartMsg); err != nil {
		return fmt.Errorf("Failed to send listen stop message: %v", err)
	}

	return nil
}

func sendAbort(conn *websocket.Conn, deviceID string) error {
	// Send listen start message
	listenStartMsg := ClientMessage{
		Type:     MessageTypeAbort,
		DeviceID: deviceID,
	}

	if err := sendJSONMessage(conn, listenStartMsg); err != nil {
		return fmt.Errorf("Failed to send listen start message: %v", err)
	}
	return nil
}

func sendListenDetect(conn *websocket.Conn, deviceID string, text string) error {
	// Send listen start message
	listenStartMsg := ClientMessage{
		Type:     MessageTypeListen,
		DeviceID: deviceID,
		State:    MessageStateDetect,
		Text:     text,
	}

	if err := sendJSONMessage(conn, listenStartMsg); err != nil {
		return fmt.Errorf("Failed to send listen detect message: %v", err)
	}
	return nil
}

// Send JSON message
func sendJSONMessage(conn *websocket.Conn, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	fmt.Printf("Sending message: %s\n", string(data))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// Read WAV file and send with Opus encoding
func sendWavFileWithOpusEncoding(conn *websocket.Conn, filePath string) error {
	// Open WAV file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Failed to open WAV file: %v", err)
	}
	defer file.Close()

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("Failed to read file content: %v", err)
	}
	fmt.Printf("File content length: %d\n", len(fileContent))
	file.Close()

	opusFrames, err := util.WavToOpus(fileContent, SampleRate, Channels, 0)
	if err != nil {
		return fmt.Errorf("Failed to convert WAV file: %v", err)
	}

	fmt.Printf("Converted Opus frame count: %d\n", len(opusFrames))

	for i, frame := range opusFrames {
		fmt.Printf("Opus frame %d length: %d\n", i, len(frame))
		// Send Opus frame
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			return fmt.Errorf("Failed to send Opus frame: %v", err)
		}
		// Control send rate, simulate real-time audio stream
		time.Sleep(time.Duration(FrameDurationMs) * time.Millisecond)
	}

	// Send 200ms silence audio data
	silenceDurationMs := 1000
	silenceFrameCount := silenceDurationMs / FrameDurationMs
	fmt.Printf("Starting to send %dms silence audio data, total %d frames\n", silenceDurationMs, silenceFrameCount)

	// Generate silence Opus data
	emptyOpusData := genEmptyOpusData(SampleRate, Channels, FrameDurationMs, 1)
	if emptyOpusData == nil {
		return fmt.Errorf("Failed to generate silence Opus data")
	}

	// Loop to send silence frames
	for i := 0; i < silenceFrameCount; i++ {
		if err := conn.WriteMessage(websocket.BinaryMessage, emptyOpusData); err != nil {
			return fmt.Errorf("Failed to send silence Opus frame: %v", err)
		}
		// Control send rate, simulate real-time audio stream
		time.Sleep(time.Duration(FrameDurationMs) * time.Millisecond)
	}
	fmt.Printf("Silence audio data sending completed\n")

	return nil
}

// Read and send audio file (raw method, without Opus encoding)
func sendAudioFile(conn *websocket.Conn, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Failed to open audio file: %v", err)
	}
	defer file.Close()

	// Directly read file content and send in chunks
	// Read and send a fixed-size chunk each time
	const chunkSize = 4096
	buffer := make([]byte, chunkSize)

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read audio data: %v", err)
		}

		if n > 0 {
			// Send binary audio data
			if err := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
				return fmt.Errorf("Failed to send audio data: %v", err)
			}

			// Control send rate, simulate real-time audio stream
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

func genEmptyOpusData(sampleRate int, channels int, frameDurationMs int, count int) []byte {
	audioProcesser, err := audio.GetAudioProcesser(sampleRate, channels, frameDurationMs)
	if err != nil {
		return nil
	}

	frameSize := sampleRate * channels * frameDurationMs / 1000

	pcmFrame := make([]int16, frameSize)
	opusFrame := make([]byte, 1000)

	n, err := audioProcesser.Encoder(pcmFrame, opusFrame)
	if err != nil {
		return nil
	}

	tmp := make([]byte, n)
	copy(tmp, opusFrame)
	return tmp
}

// Call TTS service to generate speech, encode to opus and send to server
func sendTextToSpeech(conn *websocket.Conn, deviceID string) error {
	cosyVoiceConfig := map[string]interface{}{
		"api_url":        "https://tts.linkerai.cn/tts",
		"spk_id":         "OUeAo1mhq6IBExi",
		"frame_duration": FrameDurationMs,
		"target_sr":      SampleRate,
		"audio_format":   "mp3",
		"instruct_text":  "Hello",
	}
	edgeConfig := map[string]interface{}{
		"voice":           "zh-CN-XiaoxiaoNeural",
		"rate":            "+0%",
		"volume":          "+0%",
		"pitch":           "+0Hz",
		"connect_timeout": 10,
		"receive_timeout": 60,
	}
	edgeOfflineConfig := map[string]interface{}{
		"server_url":        "ws://192.168.208.214:8081/tts",
		"timeout":           30.0,
		"handshake_timeout": 10.0,
	}
	providerName := strings.TrimSpace(ttsProviderName)
	var providerConfig map[string]interface{}
	switch providerName {
	case "edge_offline":
		providerConfig = edgeOfflineConfig
	case "edge":
		providerConfig = edgeConfig
	case "cosyvoice":
		providerConfig = cosyVoiceConfig
	default:
		return fmt.Errorf("Unsupported tts provider: %s, options: edge_offline|edge|cosyvoice", providerName)
	}
	fmt.Printf("Using TTS provider: %s\n", providerName)
	ttsProvider, err := tts.GetTTSProvider(providerName, providerConfig)
	if err != nil {
		return fmt.Errorf("Failed to get tts service(provider=%s): %v", providerName, err)
	}

	/*
		audioData, err := ttsProvider.TextToSpeech(context.Background(), "What is your name?")
		if err != nil {
			fmt.Printf("Failed to generate speech: %v\n", err)
			return fmt.Errorf("Failed to generate speech: %v", err)
		}
	*/

	emptyOpusData := genEmptyOpusData(SampleRate, 1, FrameDurationMs, 1000)

	genAndSendAudio := func(msg string, count int) error {
		audioChan, err := ttsProvider.TextToSpeechStream(context.Background(), msg, SampleRate, 1, FrameDurationMs)
		if err != nil {
			fmt.Printf("Failed to generate speech: %v\n", err)
			return fmt.Errorf("Failed to generate speech: %v", err)
		}

		for audioData := range audioChan {
			//fmt.Printf("Sending voice data length: %d\n", len(audioData))
			conn.WriteMessage(websocket.BinaryMessage, audioData)
			time.Sleep(time.Duration(FrameDurationMs) * time.Millisecond)
		}

		detectStartTs = time.Now().UnixMilli()

		for i := 0; i <= count; i++ {
			conn.WriteMessage(websocket.BinaryMessage, emptyOpusData)
			time.Sleep(time.Duration(FrameDurationMs) * time.Millisecond)
		}

		firstRecvFrame = false
		return nil
	}

	// Send detect message
	sendListenDetect(conn, deviceID, "Hello Xiaozhi")

	// New: Wait for user input text
	reader := bufio.NewReader(os.Stdin)

	var stopEmptyOpusChan = make(chan struct{})
	var resumeChan = make(chan struct{})
	go func() {
		if mode == "realtime" {
			// Continuously send emptyOpusData until stop signal is received
			for {
				select {
				case <-stopEmptyOpusChan:
					resumeChan <- struct{}{}
					<-resumeChan
				default:
					conn.WriteMessage(websocket.BinaryMessage, emptyOpusData)
					time.Sleep(time.Duration(FrameDurationMs) * time.Millisecond)
				}
			}
		}
	}()

	for {

		fmt.Print("Please enter text to synthesize (press Enter to send, empty Enter to exit): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Failed to read input: %v\n", err)
			continue
		}
		input = strings.TrimSpace(input)
		if input == "" {
			// Send abort
			sendAbort(conn, deviceID)
			select {
			case waitInput <- struct{}{}:
				status = "idle"
			default:
			}
			continue
		}
		f := func() {
			if mode == "realtime" {
				stopEmptyOpusChan <- struct{}{}
				<-resumeChan
			}
			sendListenStart(conn, deviceID, mode)
			genAndSendAudio(input, 100)
			if mode == "manual" {
				sendListenStop(conn, deviceID)
			}
			if mode == "realtime" {
				resumeChan <- struct{}{}
			}
		}
		if mode == "realtime" {
			go f()
			continue
		}
		select {
		case <-waitInput:
			go f()
		}
	}

	return nil
}
