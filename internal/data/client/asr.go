package client

import (
	"bytes"
	"context"
	"strings"
	"sync"
	asr_types "xiaozhi-esp32-server-golang/internal/domain/asr/types"
	log "xiaozhi-esp32-server-golang/logger"
)

type Asr struct {
	lock sync.RWMutex
	// ASR context and channel
	Ctx              context.Context
	Cancel           context.CancelFunc
	AsrEnd           chan bool
	AsrAudioChannel  chan []float32                 // streaming audio input channel
	AsrResultChannel chan asr_types.StreamingResult // streaming output asr recognition result fragments
	AsrResult        bytes.Buffer                   // save this time's recognition final text
	Statue           int                            // 0: initialize 1: recognizing 2: recognize end
	AutoEnd          bool                           // auto_end means use asr automatic judge end, no longer use vad module

	// ASR type and mode
	AsrType string // ASR type, such as "funasr", "doubao"
	Mode    string // ASR mode, such as "online", "offline"

	// ClientState reference, used for callback notify
	ClientState *ClientState

	// chat history audio cache: continuously accumulate audio data sent to ASR
	HistoryAudioBuffer []float32

	// whether this round of ASR already received first non-empty text
	ReceivedTextInTurn bool
}

func (a *Asr) Reset() {
	a.AsrResult.Reset()
}

func (a *Asr) CancelWithReason(reason string) {
	a.lock.RLock()
	cancel := a.Cancel
	a.lock.RUnlock()

	if cancel != nil {
		cancel()
	}
}

func (a *Asr) RetireAsrResult(ctx context.Context) (asr_types.StreamingResult, bool, error) {
	defer func() {
		a.Reset()
	}()

	log.Log().Debugf("asr type: %s, mode: %s", a.AsrType, a.Mode)

	// use local variable to track whether already sent first char event
	firstTextSent := false
	var emptyResult asr_types.StreamingResult

	for {
		select {
		case <-ctx.Done():
			log.Debugf("RetireAsrResult: ctx done, exit")
			return emptyResult, false, nil
		default:
			// avoid ctx cancel when probabilistically select channel, cause use already cancel context result
			select {
			case result, ok := <-a.AsrResultChannel:
				log.Debugf("asr result: %s, ok: %+v, isFinal: %+v, emptyReason: %s, error: %+v", result.Text, ok, result.IsFinal, result.EmptyReason, result.Error)
				if result.Error != nil {
					if result.RetryReason != "" {
						log.Warnf("ASR return recoverable error(%s), handled by upper recovery: %v", result.RetryReason, result.Error)
						return result, true, nil
					}
					return emptyResult, false, result.Error
				}

				// detect first time return char (text not empty and not sent before)
				if result.Text != "" && !firstTextSent && a.ClientState != nil && a.ClientState.OnAsrFirstTextCallback != nil {
					firstTextSent = true
					// call callback function to notify first char
					a.ClientState.OnAsrFirstTextCallback(result.Text, result.IsFinal)
				}

				if a.AsrType == "funasr" &&
					strings.EqualFold(a.Mode, "2pass") &&
					strings.EqualFold(result.Mode, "2pass-online") {
					if result.IsFinal {
						log.Debugf("funasr 2pass-online result mislabeled final, continue wait 2pass-offline final result")
					}
					continue
				}

				if result.IsFinal {
					return result, true, nil
				}

				if !ok {
					log.Debugf("asr result channel closed")
					return emptyResult, true, nil
				}
			}
		}
	}
}

func (a *Asr) MarkTextReceived() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.ReceivedTextInTurn = true
}

func (a *Asr) HasReceivedText() bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.ReceivedTextInTurn
}

func (a *Asr) ResetReceivedText() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.ReceivedTextInTurn = false
}

func (a *Asr) StopWithReason(reason string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.AsrAudioChannel != nil {
		close(a.AsrAudioChannel) // close asr input audio channel, notify asr stop, return result
		a.AsrAudioChannel = nil  // since already closed, so need to set empty
	}
}

func (a *Asr) Stop() {
	a.StopWithReason("Asr.Stop")
}

func (a *Asr) AddAudioData(pcmFrameData []float32) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.AsrAudioChannel != nil {
		// use select to implement non-blocking send, avoid channel full deadlock
		select {
		case a.AsrAudioChannel <- pcmFrameData:
			// successful send, sync cache audio data for chat history record
			a.HistoryAudioBuffer = append(a.HistoryAudioBuffer, pcmFrameData...)
		default:
			// channel already full, skip this time data, avoid block caused deadlock
			log.Warnf("AsrAudioChannel already full, skip this time audio data")
		}
	}
	return nil
}

// GetHistoryAudio get history audio cache (return replica, no clear original data)
func (a *Asr) GetHistoryAudio() []float32 {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.HistoryAudioBuffer) == 0 {
		return nil
	}
	// return replica, avoid external modification affecting original data
	result := make([]float32, len(a.HistoryAudioBuffer))
	copy(result, a.HistoryAudioBuffer)
	return result
}

// GetHistoryAudioLen get history audio cache length (sampling point count)
func (a *Asr) GetHistoryAudioLen() int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return len(a.HistoryAudioBuffer)
}

// ClearHistoryAudio clear history audio cache
func (a *Asr) ClearHistoryAudio() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.HistoryAudioBuffer = nil
}

type AsrAudioBuffer struct {
	PcmData          []float32
	AudioBufferMutex sync.RWMutex
}

func (a *AsrAudioBuffer) AddAsrAudioData(pcmFrameData []float32) error {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = append(a.PcmData, pcmFrameData...)
	return nil
}

func (a *AsrAudioBuffer) GetAsrDataSize() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData)
}

// GetFrameCount get frame count (need to pass frame size for calculation)
func (a *AsrAudioBuffer) GetFrameCount(frameSize int) int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	if frameSize == 0 {
		return 0
	}
	return len(a.PcmData) / frameSize
}

func (a *AsrAudioBuffer) GetAndClearAllData() []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmData := make([]float32, len(a.PcmData))
	copy(pcmData, a.PcmData)
	a.PcmData = []float32{}
	return pcmData
}

// GetAsrData sliding window to get data (need to pass frame size for calculation)
func (a *AsrAudioBuffer) GetAsrData(frameCount int, frameSize int) []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	pcmDataLen := len(a.PcmData)
	retSize := frameCount * frameSize
	if pcmDataLen < retSize {
		retSize = pcmDataLen
	}
	pcmData := make([]float32, retSize)
	copy(pcmData, a.PcmData[pcmDataLen-retSize:])
	return pcmData
}

// RemoveAsrAudioData remove specified frame count of audio data (need to pass frame size for calculation)
func (a *AsrAudioBuffer) RemoveAsrAudioData(frameCount int, frameSize int) {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	removeSize := frameCount * frameSize
	if removeSize > len(a.PcmData) {
		removeSize = len(a.PcmData)
	}
	a.PcmData = a.PcmData[removeSize:]
}

func (a *AsrAudioBuffer) ClearAsrAudioData() {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = nil
}
