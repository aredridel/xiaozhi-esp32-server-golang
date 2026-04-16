package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"xiaozhi-esp32-server-golang/internal/domain/asr/doubao/request"
	"xiaozhi-esp32-server-golang/internal/domain/asr/doubao/response"
	"xiaozhi-esp32-server-golang/internal/util"

	log "xiaozhi-esp32-server-golang/logger"
)

type AsrWsClient struct {
	seq            int
	url            string
	connect        *websocket.Conn
	appId          string
	accessKey      string
	resourceID     string
	connectID      string
	debugID        string
	requestOptions request.FullClientRequestOptions
	mu             sync.RWMutex // Protects connect from concurrent access

	// delayconnectionrelevantfield
	connectOnce  sync.Once     // ensureconnectiononlyestablishatimes
	connectReady chan struct{} // notifyreceive goroutine connectionalreadyestablish
	connectErr   error         // connectionestablishwhenerror
	connectErrMu sync.Mutex    // protected connectErr
}

func NewAsrWsClient(url string, appKey, accessKey, resourceID, connectID, debugID string, requestOptions request.FullClientRequestOptions) *AsrWsClient {
	return &AsrWsClient{
		seq:            1,
		url:            url,
		appId:          appKey,
		accessKey:      accessKey,
		resourceID:     resourceID,
		connectID:      connectID,
		debugID:        debugID,
		requestOptions: requestOptions,
		connectReady:   make(chan struct{}),
	}
}

func (c *AsrWsClient) logPrefix() string {
	if c.debugID == "" {
		return "[doubao-asr:unknown]"
	}
	return fmt.Sprintf("[doubao-asr:%s]", c.debugID)
}

func previewText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 32
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func firstNonEmptyUtteranceText(payload *response.AsrResponsePayload) string {
	if payload == nil {
		return ""
	}
	for _, utterance := range payload.Result.Utterances {
		if utterance.Text != "" {
			return utterance.Text
		}
	}
	return ""
}

func (c *AsrWsClient) CreateConnection(ctx context.Context) error {
	header := request.NewAuthHeader(c.appId, c.accessKey, c.resourceID, c.connectID)
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
	if err != nil {
		if resp != nil {
			var body string
			if resp.Body != nil {
				bodyBytes, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr == nil {
					body = string(bodyBytes)
				}
			}
			return fmt.Errorf("dial websocket err: %w, status=%d, body=%s", err, resp.StatusCode, body)
		}
		return fmt.Errorf("dial websocket err: %w", err)
	}
	logID := ""
	if resp != nil {
		logID = resp.Header.Get("X-Tt-Logid")
		if logID == "" {
			logID = resp.Header.Get("x-tt-logid")
		}
	}
	log.Debugf("%s websocket connectionestablishsuccessful: connect_id=%s, logid=%s", c.logPrefix(), c.connectID, logID)
	c.mu.Lock()
	c.connect = conn
	c.mu.Unlock()
	return nil
}

func (c *AsrWsClient) SendFullClientRequest() error {
	c.mu.RLock()
	conn := c.connect
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	fullClientRequest := request.NewFullClientRequest(c.requestOptions)
	c.seq++
	err := conn.WriteMessage(websocket.BinaryMessage, fullClientRequest)
	if err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	_ = resp
	//respStruct := response.ParseResponse(resp)
	//log.Println(respStruct)
	return nil
}

// ensureConnection ensureconnectionalreadyestablish（delayconnection，withretrymechanism）
func (c *AsrWsClient) ensureConnection(ctx context.Context) error {
	var err error
	c.connectOnce.Do(func() {
		log.Debugf("%s delayestablishconnection：receiventhaudiopackage，startestablishconnection", c.logPrefix())

		// retryconfig
		const (
			maxRetries = 3                      // maximumretrytimescount（totaltotaltry4times：initial1times + retry3times）
			retryDelay = 500 * time.Millisecond // retrydelay
		)

		for attempt := 1; attempt <= maxRetries+1; attempt++ {
			// tryestablishconnection
			err = c.CreateConnection(ctx)
			if err != nil {
				if attempt <= maxRetries {
					log.Warnf("%s delayestablishconnectionfailed(nth%dtimes): %v, %vafterretry", c.logPrefix(), attempt, err, retryDelay)
					select {
					case <-ctx.Done():
						err = fmt.Errorf("connectionestablishbecancel: %w", ctx.Err())
						c.connectErrMu.Lock()
						c.connectErr = err
						c.connectErrMu.Unlock()
						return
					case <-time.After(retryDelay):
						// fixeddelayafterretry
					}
					continue
				} else {
					// finalatimesretryfailed
					log.Errorf("%s delayestablishconnectionfailed(nth%dtimes, alreadyreachmaximumretrytimescount): %v", c.logPrefix(), attempt, err)
					c.connectErrMu.Lock()
					c.connectErr = err
					c.connectErrMu.Unlock()
					return
				}
			}

			// connectionestablishsuccessful，sendinitializerequest
			err = c.SendFullClientRequest()
			if err != nil {
				// sendinitializerequestfailed，closeconnectionandretry
				log.Warnf("%s sendinitializerequestfailed(nth%dtimes): %v", c.logPrefix(), attempt, err)
				c.Close()

				if attempt <= maxRetries {
					log.Warnf("%s %vafterretryestablishconnection", c.logPrefix(), retryDelay)
					select {
					case <-ctx.Done():
						err = fmt.Errorf("connectionestablishbecancel: %w", ctx.Err())
						c.connectErrMu.Lock()
						c.connectErr = err
						c.connectErrMu.Unlock()
						return
					case <-time.After(retryDelay):
						// fixeddelayafterretry
					}
					continue
				} else {
					// finalatimesretryfailed
					log.Errorf("%s sendinitializerequestfailed(nth%dtimes, alreadyreachmaximumretrytimescount): %v", c.logPrefix(), attempt, err)
					c.connectErrMu.Lock()
					c.connectErr = err
					c.connectErrMu.Unlock()
					return
				}
			}

			// connectionandinitializearesuccessful
			if attempt > 1 {
				log.Infof("%s delayestablishconnectionsuccessful(nth%dtimestry)", c.logPrefix(), attempt)
			} else {
				log.Debugf("%s delayestablishconnectionsuccessful", c.logPrefix())
			}
			// notifyreceive goroutine connectionalreadyestablish
			close(c.connectReady)
			return
		}
	})
	return err
}

func (c *AsrWsClient) SendMessages(ctx context.Context, audioStream <-chan []float32, stopChan <-chan struct{}) error {
	messageChan := make(chan []byte)
	packetCount := 0
	totalSamples := 0
	exitReason := "unknown"
	defer func() {
		log.Debugf(
			"%s SendMessages exit: reason=%s, packets=%d, total_samples=%d, next_seq=%d",
			c.logPrefix(),
			exitReason,
			packetCount,
			totalSamples,
			c.seq,
		)
	}()
	go func() {
		for message := range messageChan {
			c.mu.RLock()
			conn := c.connect
			c.mu.RUnlock()

			if conn == nil {
				log.Debugf("%s websocket connection is nil, stopping message writer", c.logPrefix())
				return
			}

			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Debugf("%s write message err: %s", c.logPrefix(), err)
				return
			}
		}
	}()

	defer close(messageChan)
	firstPacket := true
	for {
		select {
		case <-ctx.Done():
			exitReason = "context_done"
			return fmt.Errorf("send messages context done")
		case <-stopChan:
			exitReason = "stop_chan"
			return fmt.Errorf("send messages stop chan")
		case audioData, ok := <-audioStream:
			if !ok {
				exitReason = "audio_stream_closed"
				log.Debugf("%s sendMessages audioStream closed", c.logPrefix())
				// ifconnectionnotestablish（silencesituation），directreturn
				c.mu.RLock()
				conn := c.connect
				c.mu.RUnlock()
				if conn == nil {
					log.Debugf("%s audioStream closeandconnectionnotestablish, directreturn（silencesituation）", c.logPrefix())
					return nil
				}
				// connectionalreadyestablish，sendendmessage
				endMessage := request.NewAudioOnlyRequest(-c.seq, []byte{})
				messageChan <- endMessage
				log.Debugf("%s sendendaudiopackage: seq=%d", c.logPrefix(), -c.seq)
				return nil
			}

			// receiventhaudiopackagewhen，establishconnection
			if firstPacket {
				firstPacket = false
				err := c.ensureConnection(ctx)
				if err != nil {
					exitReason = "ensure_connection_failed"
					log.Errorf("%s establishconnectionfailed: %v", c.logPrefix(), err)
					return fmt.Errorf("ensure connection err: %w", err)
				}
			}

			packetCount++
			totalSamples += len(audioData)
			if packetCount <= 3 || packetCount%25 == 0 {
				log.Debugf(
					"%s sendaudiopackage: idx=%d, seq=%d, samples=%d, total_samples=%d",
					c.logPrefix(),
					packetCount,
					c.seq,
					len(audioData),
					totalSamples,
				)
			}

			byteData := make([]byte, len(audioData)*2)
			util.Float32ToPCMBytes(audioData, byteData)
			message := request.NewAudioOnlyRequest(c.seq, byteData)
			messageChan <- message
			c.seq++
		}
	}
}

func (c *AsrWsClient) recvMessages(ctx context.Context, resChan chan<- *response.AsrResponse, stopChan chan<- struct{}) {
	recvCount := 0
	for {
		c.mu.RLock()
		conn := c.connect
		c.mu.RUnlock()

		if conn == nil {
			log.Debugf("%s websocket connection is nil, stopping message receiver", c.logPrefix())
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Warnf("%s read doubao response failed: recv_count=%d, err=%v", c.logPrefix(), recvCount, err)
			return
		}
		resp := response.ParseResponse(message)
		recvCount++

		textLen := 0
		textSnippet := ""
		utteranceCount := 0
		firstUtterance := ""
		audioDuration := 0
		if resp.PayloadMsg != nil {
			textLen = len([]rune(resp.PayloadMsg.Result.Text))
			textSnippet = previewText(resp.PayloadMsg.Result.Text, 24)
			utteranceCount = len(resp.PayloadMsg.Result.Utterances)
			firstUtterance = previewText(firstNonEmptyUtteranceText(resp.PayloadMsg), 24)
			audioDuration = resp.PayloadMsg.AudioInfo.Duration
		}
		log.Debugf(
			"%s receiverespondpackage: idx=%d, payload_seq=%d, event=%d, last=%v, code=%d, text_len=%d, text=%q, utterances=%d, first_utterance=%q, audio_duration=%d",
			c.logPrefix(),
			recvCount,
			resp.PayloadSequence,
			resp.Event,
			resp.IsLastPackage,
			resp.Code,
			textLen,
			textSnippet,
			utteranceCount,
			firstUtterance,
			audioDuration,
		)
		select {
		case <-ctx.Done():
			return
		case resChan <- resp:
		}
		if resp.IsLastPackage {
			log.Debugf("%s receivefinalaresponsepackage，stopreceive: recv_count=%d", c.logPrefix(), recvCount)
			return
		}
		if resp.Code != 0 {
			log.Warnf("%s responsepackagereturnerrorcode, notifysend goroutinestop: recv_count=%d, code=%d", c.logPrefix(), recvCount, resp.Code)
			close(stopChan)
			return
		}
	}
}

func (c *AsrWsClient) StartAudioStream(ctx context.Context, audioStream <-chan []float32, resChan chan<- *response.AsrResponse) error {
	stopChan := make(chan struct{})
	sendDoneChan := make(chan error, 1) // sendcompletenotify（nilindicatenormalcomplete，errorindicateerror）
	log.Debugf("%s StartAudioStream begin", c.logPrefix())

	// startsend goroutine
	go func() {
		err := c.SendMessages(ctx, audioStream, stopChan)
		// whethersuccessfuloryesfailed，aresendnotify
		sendDoneChan <- err
	}()

	// waitconnectestablishorsendcomplete
	select {
	case <-ctx.Done():
		log.Debugf("%s StartAudioStream context done before connect", c.logPrefix())
		return fmt.Errorf("start audio stream context done")
	case <-c.connectReady:
		// connectionalreadyestablish，startreceive goroutine
		log.Debugf("%s connectionalreadyestablish, startreceive goroutine", c.logPrefix())
		c.recvMessages(ctx, resChan, stopChan)
		return nil
	case err := <-sendDoneChan:
		// sendcomplete（maybenormalcompleteorerror）
		if err != nil {
			// sendprocessinerror
			log.Errorf("%s sendaudio streamfailed: %v", c.logPrefix(), err)
			return err
		}
		// check ifsilencesituation（connectionnotestablish）
		c.mu.RLock()
		conn := c.connect
		c.mu.RUnlock()
		if conn == nil {
			// silencesituation: audioStream closebutconnectionnotestablish
			log.Debugf("%s silencesituation: connectionnotestablish, sendemptyresult", c.logPrefix())
			payload := &response.AsrResponsePayload{}
			payload.Result.Text = ""
			resChan <- &response.AsrResponse{
				Code:          0,
				IsLastPackage: true,
				PayloadMsg:    payload,
			}
			return nil
		}
		// connectionalreadyestablish, startreceive goroutine(processremainingofresponse)
		log.Debugf("%s SendMessages alreadyend, startreceiveremainingresponse", c.logPrefix())
		c.recvMessages(ctx, resChan, stopChan)
		return nil
	}
}

func (c *AsrWsClient) Excute(ctx context.Context, audioStream chan []float32, resChan chan<- *response.AsrResponse) error {
	c.seq = 1
	if c.url == "" {
		return errors.New("url is empty")
	}
	err := c.CreateConnection(ctx)
	if err != nil {
		return fmt.Errorf("create connection err: %w", err)
	}
	err = c.SendFullClientRequest()
	if err != nil {
		return fmt.Errorf("send full request err: %w", err)
	}

	err = c.StartAudioStream(ctx, audioStream, resChan)
	if err != nil {
		return fmt.Errorf("start audio stream err: %w", err)
	}
	return nil
}

func (c *AsrWsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connect != nil {
		err := c.connect.Close()
		c.connect = nil
		return err
	}
	return nil
}
