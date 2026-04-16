package xiaozhi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

var deviceIdList = []string{
	"ba:8f:17:de:94:94",
	"f2:85:44:27:7b:51",
	"4f:57:fb:d4:69:fa",
	"b3:1e:1c:80:cc:78",
	"32:a5:cc:b7:c0:e4",
	"2b:60:6a:5a:72:10",
	"ca:a6:8b:20:f1:6f",
	"26:1a:d7:27:9f:f8",
	"03:02:26:58:2b:06",
	"5f:f3:85:8b:5d:da",
}

// record recent error deviceId and its block expiration time
var (
	deviceIdBlocklist     = make(map[string]time.Time)
	deviceIdBlocklistLock sync.Mutex
	// deviceID block time (how long after error to not use)
	deviceIdBlockDuration = 5 * time.Second
)

// XiaozhiProvider Xiaozhi TTS WebSocket Provider
// supports streaming text to speech
type XiaozhiProvider struct {
	ServerAddr  string
	DeviceID    string
	AudioFormat map[string]interface{}
	Header      http.Header
}

// periodically cleanup expired deviceId block list
func init() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// cleanup expired deviceId block list
			deviceIdBlocklistLock.Lock()
			now := time.Now()
			for id, expireTime := range deviceIdBlocklist {
				if now.After(expireTime) {
					delete(deviceIdBlocklist, id)
					log.Debugf("deviceID block expired, re-enable: %s", id)
				}
			}
			deviceIdBlocklistLock.Unlock()
		}
	}()
}

// add deviceId to block list
func blockDeviceId(deviceId string) {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	deviceIdBlocklist[deviceId] = time.Now().Add(deviceIdBlockDuration)
	log.Warnf("deviceID %s added to block list, will re-enable after %v", deviceId, deviceIdBlockDuration)
}

// check if deviceId is in block list
func isDeviceIdBlocked(deviceId string) bool {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	expireTime, exists := deviceIdBlocklist[deviceId]
	if !exists {
		return false
	}

	// if expiration time has passed, remove from block list
	if time.Now().After(expireTime) {
		delete(deviceIdBlocklist, deviceId)
		log.Debugf("deviceID block expired, re-enable: %s", deviceId)
		return false
	}

	return true
}

// NewXiaozhiProvider create new Xiaozhi TTS Provider
func NewXiaozhiProvider(config map[string]interface{}) *XiaozhiProvider {
	serverAddr, _ := config["server_addr"].(string)
	deviceID, _ := config["device_id"].(string)
	clientID, _ := config["client_id"].(string)
	token, _ := config["token"].(string)
	format := map[string]interface{}{
		"sample_rate":    16000,
		"channels":       1,
		"frame_duration": 20,
		"format":         "opus",
	}

	header := http.Header{}
	header.Set("Device-Id", deviceID)
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Bearer "+token)
	header.Set("Protocol-Version", "1")
	header.Set("Client-Id", clientID)

	return &XiaozhiProvider{
		ServerAddr:  serverAddr,
		DeviceID:    deviceID,
		AudioFormat: format,
		Header:      header,
	}
}

// selectDeviceId select an available deviceID
func (p *XiaozhiProvider) selectDeviceId() string {
	// find non-blocked deviceId from deviceIdList
	for _, deviceId := range deviceIdList {
		if !isDeviceIdBlocked(deviceId) {
			log.Debugf("select non-blocked deviceID: %s", deviceId)
			return deviceId
		}
	}

	// if all deviceIds are blocked, poll from all deviceIds
	if len(deviceIdList) > 0 {
		// use simple polling strategy (based on time)
		selectedIndex := int(time.Now().Unix()) % len(deviceIdList)
		selectedDeviceId := deviceIdList[selectedIndex]
		log.Warnf("all deviceIds are blocked, polling select deviceID: %s (index: %d)", selectedDeviceId, selectedIndex)
		return selectedDeviceId
	}

	// if deviceIdList is empty, use passed deviceId
	if p.DeviceID != "" {
		log.Warnf("deviceIdList is empty, use current deviceID: %s", p.DeviceID)
		return p.DeviceID
	}

	// if none, return first deviceID (if exists)
	if len(deviceIdList) > 0 {
		return deviceIdList[0]
	}

	return ""
}

// createWSConnection create new WebSocket connection
func (p *XiaozhiProvider) createWSConnection(ctx context.Context) (*websocket.Conn, string, error) {
	// select an available deviceID
	selectedDeviceId := p.selectDeviceId()
	if selectedDeviceId == "" {
		return nil, "", fmt.Errorf("unable to select deviceID")
	}

	// update current p.DeviceID and Header
	p.DeviceID = selectedDeviceId
	p.Header.Set("Device-Id", selectedDeviceId)

	// create new connection
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.ServerAddr, p.Header)
	if err != nil {
		log.Errorf("create WebSocket connection failed: %v, deviceID: %s", err, selectedDeviceId)
		blockDeviceId(selectedDeviceId) // add failed deviceId to block list
		return nil, "", err
	}

	// set keep connection
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// send hello message when creating new connection
	helloMsg := map[string]interface{}{
		"type":         "hello",
		"device_id":    selectedDeviceId,
		"transport":    "websocket",
		"version":      1,
		"audio_params": p.AudioFormat,
	}
	log.Debugf("create new connection and send hello message, deviceID: %s", selectedDeviceId)
	if err := conn.WriteJSON(helloMsg); err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("send hello message failed: %v", err)
	}

	return conn, selectedDeviceId, nil
}

type RecvMsg struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Text    string `json:"text"`
	Version int    `json:"version"`
}

// sendStopMessage send stop message and close connection
func sendStopMessage(conn *websocket.Conn, deviceId string) {
	stopMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "stop",
	}
	if err := conn.WriteJSON(stopMsg); err != nil {
		log.Warnf("send stop message failed: %v, deviceID: %s", err, deviceId)
	} else {
		log.Debugf("send stop message successful, deviceID: %s", deviceId)
	}
}

// handleTTSConnection encapsulation get connection, send message and receive message logic
func (p *XiaozhiProvider) handleTTSConnection(ctx context.Context, text string, outputChan chan []byte) error {
	// create new connection
	conn, deviceId, err := p.createWSConnection(ctx)
	if err != nil {
		return fmt.Errorf("create Xiaozhi TTS connection failed: %v", err)
	}
	defer func() {
		// send stop message and close connection
		sendStopMessage(conn, deviceId)
		conn.Close()
	}()

	// send listen detect message
	sendText := fmt.Sprintf("`%s`", text)
	listenMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "detect",
		"text":      sendText,
	}
	log.Debugf("send xiaozhi server-side message: %v", listenMsg)

	if err := conn.WriteJSON(listenMsg); err != nil {
		log.Errorf("send listen message failed: %v, deviceID: %s", err, deviceId)
		blockDeviceId(deviceId) // add error deviceId to block list
		return fmt.Errorf("send message failed: %v", err)
	}

	// read and process message
	startTs := time.Now().UnixMilli()
	var firstFrameTs bool
	i := 0
	receivedFrames := false

	for {
		select {
		case <-ctx.Done():
			log.Debugf("xiaozhi server-side message ctx.Done(), deviceID: %s", deviceId)
			return nil
		default:
		}
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			// connection error
			log.Errorf("read message error: %v, deviceID: %s", err, deviceId)

			// if no audio frame received, indicates connection may have problem, add deviceId to block list
			if !receivedFrames {
				blockDeviceId(deviceId)
			}

			return fmt.Errorf("read message error: %v", err)
		}
		if msgType == websocket.TextMessage {
			log.Debugf("receive xiaozhi server-side message: %s", string(msg))
			var recvMsg RecvMsg
			err := json.Unmarshal(msg, &recvMsg)
			if err != nil {
				continue
			}
			if recvMsg.Type == "tts" {
				if recvMsg.State == "stop" {
					log.Debugf("xiaozhi server-side message tts stop message")
					return nil
				}
			}
		} else if msgType == websocket.BinaryMessage {
			receivedFrames = true
			if !firstFrameTs {
				firstFrameTs = true
				log.Debugf("tts time consumption count: xiaozhi service tts first audio frame time: %d", time.Now().UnixMilli()-startTs)
			}
			outputChan <- msg
			if i%20 == 0 {
				log.Debugf("xiaozhi server-side audio message, already received %d audio frames", i)
			}
			i++
		}
	}
}

// TextToSpeechStream implement streaming TTS, return opus audio frame chan
func (p *XiaozhiProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 1000)

	// try process TTS connection, support retry
	go func() {
		defer close(outputChan)

		retryCount := 0
		maxRetries := 2
		var lastError error

		// at most try maxRetries times
		for retryCount <= maxRetries {
			if retryCount > 0 {
				log.Infof("try re-get connection, attempt %d/%d retry", retryCount, maxRetries)

				// before retry check if context already cancelled
				select {
				case <-ctx.Done():
					log.Debugf("context already cancelled, stop retry")
					return
				default:
					// continue retry
				}
			}

			// process TTS connection
			err := p.handleTTSConnection(ctx, text, outputChan)

			if err == nil {
				// connection process successful, no need to retry
				return
			}

			lastError = err
			log.Errorf("TTS connection process failed: %v (retry: %d/%d)", err, retryCount, maxRetries)

			retryCount++
		}

		if retryCount > maxRetries {
			log.Warnf("reached maximum retry count %d, abort retry, last error: %v", maxRetries, lastError)
		}
	}()

	return outputChan, nil
}

// GetVoiceInfo getTTSconfiginfo
func (p *XiaozhiProvider) GetVoiceInfo() map[string]interface{} {
	return map[string]interface{}{
		"type":         "xiaozhi_ws",
		"server_addr":  p.ServerAddr,
		"device_id":    p.DeviceID,
		"audio_format": p.AudioFormat,
	}
}

// SetVoice set voice parameter (Xiaozhi Provider does not support dynamic voice setting)
func (p *XiaozhiProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return fmt.Errorf("Xiaozhi TTS Provider does not support dynamic voice setting")
}

// Close close resource (stateless Provider, no need to close)
func (p *XiaozhiProvider) Close() error {
	return nil
}

// IsValid check if resource is valid
func (p *XiaozhiProvider) IsValid() bool {
	return p != nil
}

// TextToSpeech implement BaseTTSProvider interface, directly aggregate streaming frames
func (p *XiaozhiProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	ch, err := p.TextToSpeechStream(ctx, text, sampleRate, channels, frameDuration)
	if err != nil {
		return nil, err
	}
	var frames [][]byte
	for frame := range ch {
		frames = append(frames, frame)
	}
	return frames, nil
}
