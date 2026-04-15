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

// record最近out错ofdeviceIdand其禁useto期time
var (
	deviceIdBlocklist     = make(map[string]time.Time)
	deviceIdBlocklistLock sync.Mutex
	// deviceID禁usetime（out错after多久insidenouse）
	deviceIdBlockDuration = 5 * time.Second
)

// XiaozhiProvider small智TTS WebSocket Provider
// supportstreamingtext转voice
type XiaozhiProvider struct {
	ServerAddr  string
	DeviceID    string
	AudioFormat map[string]interface{}
	Header      http.Header
}

// 定期cleanupexpireofdeviceId禁uselist
func init() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// cleanupexpireofdeviceId禁uselist
			deviceIdBlocklistLock.Lock()
			now := time.Now()
			for id, expireTime := range deviceIdBlocklist {
				if now.After(expireTime) {
					delete(deviceIdBlocklist, id)
					log.Debugf("deviceID禁usealreadyexpire，re启use: %s", id)
				}
			}
			deviceIdBlocklistLock.Unlock()
		}
	}()
}

// willdeviceIdaddto禁uselist
func blockDeviceId(deviceId string) {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	deviceIdBlocklist[deviceId] = time.Now().Add(deviceIdBlockDuration)
	log.Warnf("deviceID %s alreadyaddto禁uselist，willat %v afterre启use", deviceId, deviceIdBlockDuration)
}

// inspectdeviceIdwhetherat禁uselistin
func isDeviceIdBlocked(deviceId string) bool {
	deviceIdBlocklistLock.Lock()
	defer deviceIdBlocklistLock.Unlock()

	expireTime, exists := deviceIdBlocklist[deviceId]
	if !exists {
		return false
	}

	// ifexpiretimealreadypast，thenfrom禁uselistinremove
	if time.Now().After(expireTime) {
		delete(deviceIdBlocklist, deviceId)
		log.Debugf("deviceID禁usealreadyexpire，re启use: %s", deviceId)
		return false
	}

	return true
}

// NewXiaozhiProvider create newsmall智TTS Provider
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

// selectDeviceId selectaavailableofdeviceID
func (p *XiaozhiProvider) selectDeviceId() string {
	// fromdeviceIdListin找outnotbe禁useofdeviceId
	for _, deviceId := range deviceIdList {
		if !isDeviceIdBlocked(deviceId) {
			log.Debugf("selectnotbe禁useofdeviceID: %s", deviceId)
			return deviceId
		}
	}

	// ifalldeviceIdarebe禁use，thenfromalldeviceIdinpollingselect
	if len(deviceIdList) > 0 {
		// use简单ofpollingstrategy（基于time）
		selectedIndex := int(time.Now().Unix()) % len(deviceIdList)
		selectedDeviceId := deviceIdList[selectedIndex]
		log.Warnf("alldeviceId均be禁use，pollingselectdeviceID: %s (index: %d)", selectedDeviceId, selectedIndex)
		return selectedDeviceId
	}

	// ifdeviceIdListisempty，use传入ofdeviceId
	if p.DeviceID != "" {
		log.Warnf("deviceIdListisempty，usecurrentdeviceID: %s", p.DeviceID)
		return p.DeviceID
	}

	// ifareno，returnnthadeviceID（if存at）
	if len(deviceIdList) > 0 {
		return deviceIdList[0]
	}

	return ""
}

// createWSConnection create newWebSocketjoin
func (p *XiaozhiProvider) createWSConnection(ctx context.Context) (*websocket.Conn, string, error) {
	// selectaavailableofdeviceID
	selectedDeviceId := p.selectDeviceId()
	if selectedDeviceId == "" {
		return nil, "", fmt.Errorf("no法selectdeviceID")
	}

	// updatecurrentp.DeviceIDandHeader
	p.DeviceID = selectedDeviceId
	p.Header.Set("Device-Id", selectedDeviceId)

	// create新join
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.ServerAddr, p.Header)
	if err != nil {
		log.Errorf("createWebSocketjoinfailed: %v, deviceID: %s", err, selectedDeviceId)
		blockDeviceId(selectedDeviceId) // willfailedofdeviceIdadd to禁uselist
		return nil, "", err
	}

	// setkeepjoin
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// 新建joinwhensendhellomessage
	helloMsg := map[string]interface{}{
		"type":         "hello",
		"device_id":    selectedDeviceId,
		"transport":    "websocket",
		"version":      1,
		"audio_params": p.AudioFormat,
	}
	log.Debugf("create新joinandsendhellomessage，deviceID: %s", selectedDeviceId)
	if err := conn.WriteJSON(helloMsg); err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("sendhellomessagefailed: %v", err)
	}

	return conn, selectedDeviceId, nil
}

type RecvMsg struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Text    string `json:"text"`
	Version int    `json:"version"`
}

// sendStopMessage sendstopmessageandclosejoin
func sendStopMessage(conn *websocket.Conn, deviceId string) {
	stopMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "stop",
	}
	if err := conn.WriteJSON(stopMsg); err != nil {
		log.Warnf("sendstopmessagefailed: %v, deviceID: %s", err, deviceId)
	} else {
		log.Debugf("sendstopmessagesuccessful，deviceID: %s", deviceId)
	}
}

// handleTTSConnection encapsulationgetjoin、sendmessageandreceivemessageoflogical
func (p *XiaozhiProvider) handleTTSConnection(ctx context.Context, text string, outputChan chan []byte) error {
	// create新join
	conn, deviceId, err := p.createWSConnection(ctx)
	if err != nil {
		return fmt.Errorf("createsmall智TTSjoinfailed: %v", err)
	}
	defer func() {
		// sendstopmessageandclosejoin
		sendStopMessage(conn, deviceId)
		conn.Close()
	}()

	// sendlisten detectmessage
	sendText := fmt.Sprintf("`%s`", text)
	listenMsg := map[string]interface{}{
		"type":      "listen",
		"device_id": deviceId,
		"state":     "detect",
		"text":      sendText,
	}
	log.Debugf("sendxiaozhiserver-sidemessage: %v", listenMsg)

	if err := conn.WriteJSON(listenMsg); err != nil {
		log.Errorf("sendlistenmessagefailed: %v，deviceID: %s", err, deviceId)
		blockDeviceId(deviceId) // willout错ofdeviceIdadd to禁uselist
		return fmt.Errorf("sendmessagefailed: %v", err)
	}

	// readandprocessmessage
	startTs := time.Now().UnixMilli()
	var firstFrameTs bool
	i := 0
	receivedFrames := false

	for {
		select {
		case <-ctx.Done():
			log.Debugf("xiaozhiserver-sidemessagectx.Done(), deviceID: %s", deviceId)
			return nil
		default:
		}
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			// joinout错
			log.Errorf("readcancel息error: %v，deviceID: %s", err, deviceId)

			// ifornoreceive任何audio frame，instructionjoinmayhave问题，willdeviceIdadd to禁uselist
			if !receivedFrames {
				blockDeviceId(deviceId)
			}

			return fmt.Errorf("readcancel息error: %v", err)
		}
		if msgType == websocket.TextMessage {
			log.Debugf("receivexiaozhiserver-sidemessage: %s", string(msg))
			var recvMsg RecvMsg
			err := json.Unmarshal(msg, &recvMsg)
			if err != nil {
				continue
			}
			if recvMsg.Type == "tts" {
				if recvMsg.State == "stop" {
					log.Debugf("xiaozhiserver-sidemessagetts stopmessage")
					return nil
				}
			}
		} else if msgType == websocket.BinaryMessage {
			receivedFrames = true
			if !firstFrameTs {
				firstFrameTs = true
				log.Debugf("ttstime consumptioncount: xiaozhiservicetts nthaaudio frametime: %d", time.Now().UnixMilli()-startTs)
			}
			outputChan <- msg
			if i%20 == 0 {
				log.Debugf("xiaozhiserver-sideaudiomessage, alreadyreceive%d个audio frame", i)
			}
			i++
		}
	}
}

// TextToSpeechStream implementstreaming TTS，returnopusaudio framechan
func (p *XiaozhiProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 1000)

	// tryprocessTTSjoin，supportretry
	go func() {
		defer close(outputChan)

		retryCount := 0
		maxRetries := 2
		var lastError error

		// at mosttrymaxRetriestimes
		for retryCount <= maxRetries {
			if retryCount > 0 {
				log.Infof("tryregetjoin，nth %d/%d timesretry", retryCount, maxRetries)

				// atretrybeforeinspectcontextwhetheralreadycancel
				select {
				case <-ctx.Done():
					log.Debugf("contextalreadycancel，stopretry")
					return
				default:
					// continueretry
				}
			}

			// processTTSjoin
			err := p.handleTTSConnection(ctx, text, outputChan)

			if err == nil {
				// joinprocesssuccessful，noneedretry
				return
			}

			lastError = err
			log.Errorf("TTSjoinprocessfailed: %v (retry: %d/%d)", err, retryCount, maxRetries)

			retryCount++
		}

		if retryCount > maxRetries {
			log.Warnf("reachtomaximumretrytimescount %d，abortretry，最aftererror: %v", maxRetries, lastError)
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

// SetVoice setvoiceparameter（Xiaozhi Provider unsupporteddynamicsetvoice）
func (p *XiaozhiProvider) SetVoice(voiceConfig map[string]interface{}) error {
	return fmt.Errorf("Xiaozhi TTS Provider unsupporteddynamicsetvoice")
}

// Close closeresource（nostate Provider，noneedclose）
func (p *XiaozhiProvider) Close() error {
	return nil
}

// IsValid inspectresourcewhethervalid
func (p *XiaozhiProvider) IsValid() bool {
	return p != nil
}

// TextToSpeech implement BaseTTSProvider interface，directaggregatestreamingframe
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
