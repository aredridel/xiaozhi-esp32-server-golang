package chat

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/asr"
	asr_types "xiaozhi-esp32-server-golang/internal/domain/asr/types"
	"xiaozhi-esp32-server-golang/internal/domain/audio"
	chathooks "xiaozhi-esp32-server-golang/internal/domain/chat/hooks"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/domain/vad/inter"
	"xiaozhi-esp32-server-golang/internal/pool"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

type ASRManagerOption func(*ASRManager)

// AsrMessageSaveCallback message save callback function type
type AsrMessageSaveCallback func(userMsg *schema.Message, messageID string, audioData []float32)

type ASRManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	session         *ChatSession // used to access speakerManager

	// ASR resources managed as private fields
	asrResource *pool.ResourceWrapper[asr.AsrProvider]
	resourceMu  sync.RWMutex // protect resource access
}

func NewASRManager(clientState *ClientState, serverTransport *ServerTransport, opts ...ASRManagerOption) *ASRManager {
	asr := &ASRManager{
		clientState:     clientState,
		serverTransport: serverTransport,
		session:         nil, // set later via SetSession
	}
	for _, opt := range opts {
		opt(asr)
	}
	return asr
}

// ProcessVadAudio start VAD audio processing
func (a *ASRManager) ProcessVadAudio(ctx context.Context, onClose func()) {
	state := a.clientState
	go func() {
		hasTriggeredCancel := true // flag indicating whether cancel has been triggered (when voiceDuration > 120)
		hasLoggedFirstTextExtendedWait := false
		speakerInterruptTriggered := atomic.Bool{}
		speakerPeekInFlight := atomic.Bool{}
		lastSpeakerPeekDoneAt := atomic.Int64{}
		var speakerPeekAudioMs int64
		var speakerPeekRequestSeq uint64
		const speakerPeekInterval = 200 * time.Millisecond
		const firstSpeakerPeekAudioThresholdMs int64 = 400
		audioFormat := state.InputAudioFormat
		// use a sufficiently large buffer for decoding (assuming max frame duration of 120ms)
		maxFrameSize := audioFormat.SampleRate * audioFormat.Channels * 120 / 1000
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, 20) // pass a default value to create decoder
		if err != nil {
			log.Errorf("failed to get decoder: %v", err)
			return
		}

		// get frame size and duration from first actual frame
		var frameSize int
		var frameDurationMs int
		var vadNeedGetCount int // number of frames needed by VAD, calculated after first frame

		// VAD resources use lazy loading + idle release, avoiding long-term occupation of pool instances.
		var vadWrapper *pool.ResourceWrapper[inter.VAD]
		var vadProvider inter.VAD
		var vadLastUseAt time.Time
		const vadIdleReleaseTimeout = 2 * time.Second
		vadIdleTicker := time.NewTicker(time.Second)
		defer vadIdleTicker.Stop()
		needVad := !(state.Asr.AutoEnd || state.ListenMode == "manual")
		vadProviderName := state.DeviceConfig.Vad.Provider
		vadProviderConfig := state.DeviceConfig.Vad.Config
		releaseVad := func(reason string) {
			if vadWrapper == nil {
				return
			}
			pool.Release(vadWrapper)
			vadWrapper = nil
			vadProvider = nil
			vadLastUseAt = time.Time{}
			log.Debugf("release VAD resource: device=%s, reason=%s", state.DeviceID, reason)
		}
		defer releaseVad("process_exit")
		ensureVad := func() bool {
			if !needVad {
				return false
			}
			if vadProvider != nil {
				return true
			}

			// check if provider is empty, log warning if so
			if vadProviderName == "" {
				log.Warnf("VAD provider is empty, trying to get from config")
			} else {
				log.Debugf("acquiring VAD resource: provider=%s", vadProviderName)
			}

			wrapper, err := pool.Acquire[inter.VAD](
				"vad",
				vadProviderName,
				vadProviderConfig,
			)
			if err != nil {
				log.Errorf("acquire VAD resource failed: provider=%s, config=%+v, error=%v", vadProviderName, vadProviderConfig, err)
				return false
			}
			vadWrapper = wrapper
			vadProvider = wrapper.GetProvider()
			vadLastUseAt = time.Now()
			return true
		}

		for {
			// use max frame size as buffer, actual frame size obtained after decoding
			pcmFrame := make([]float32, maxFrameSize)

			select {
			case <-vadIdleTicker.C:
				if vadWrapper != nil && !vadLastUseAt.IsZero() && time.Since(vadLastUseAt) >= vadIdleReleaseTimeout {
					releaseVad("idle_timeout")
				}
				continue
			case opusFrame, ok := <-state.OpusAudioBuffer:
				//log.Debugf("processAsrAudio received audio data, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio audio channel closed")
					return
				}

				var skipVad bool
				var haveVoice bool
				clientHaveVoice := state.GetClientHaveVoice()
				if state.Asr.AutoEnd || state.ListenMode == "manual" {
					skipVad = true         // skip VAD
					clientHaveVoice = true // had voice before
					haveVoice = true       // has voice this time
				}

				if state.GetClientVoiceStop() { // stopped speaking, do not receive audio data
					//log.Infof("client stopped speaking, skipping audio data")
					continue
				}

				//log.Debugf("clientVoiceStop: %+v, asrDataSize: %d, listenMode: %s, isSkipVad: %v\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize(), state.ListenMode, skipVad)

				n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
				if err != nil {
					log.Errorf("decode failed: %v", err)
					continue
				}

				// dynamically calculate frame size and duration from decoded data
				if frameSize == 0 {
					// first frame: calculate frame info from actual decoded data
					frameSize = n
					samplesPerChannel := n / audioFormat.Channels
					frameDurationMs = samplesPerChannel * 1000 / audioFormat.SampleRate
					audioFormat.FrameDuration = frameDurationMs

					// calculate number of frames needed by VAD
					vadNeedGetCount = 1
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						// silero_vad requires at least 60ms of audio data
						vadNeedGetCount = 60 / frameDurationMs
						if vadNeedGetCount < 1 {
							vadNeedGetCount = 1
						}
					}
					log.Debugf("calculated frame info from actual audio data: frameSize=%d, frameDurationMs=%d, vadNeedGetCount=%d", frameSize, frameDurationMs, vadNeedGetCount)
				}

				var vadPcmData []float32
				pcmData := pcmFrame[:n]
				speakerPcmData := pcmFrame[:n]

				if n != frameSize {
					log.Debugf("frame size mismatch: expected=%d, actual=%d, using actual value", frameSize, n)
					samplesPerChannel := n / audioFormat.Channels
					currentFrameDurationMs := samplesPerChannel * 1000 / audioFormat.SampleRate
					frameSize = n
					frameDurationMs = currentFrameDurationMs
					audioFormat.FrameDuration = frameDurationMs
				}

				if !skipVad && needVad {
					if !ensureVad() {
						continue
					}
					//decode opus to pcm
					state.AsrAudioBuffer.AddAsrAudioData(pcmData)

					// calculate minimum data needed for VAD (60ms for silero_vad)
					vadNeedMinSize := frameSize
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						vadNeedMinSize = vadNeedGetCount * frameSize
					}

					if state.AsrAudioBuffer.GetAsrDataSize() >= vadNeedMinSize {
						//if VAD needed, must get at least 60ms of audio data
						vadPcmData = state.AsrAudioBuffer.GetAsrData(vadNeedGetCount, frameSize)

						//if voice already detected, skip VAD detection, pass pcmData directly to ASR
						// use VAD resource acquired outside the loop for detection
						// reset VAD state
						vadLastUseAt = time.Now()
						if err := vadProvider.Reset(); err != nil {
							log.Errorf("reset VAD failed: %v", err)
							continue
						}

						// perform VAD detection
						vadLastUseAt = time.Now()
						haveVoice, err = vadProvider.IsVADExt(vadPcmData, audioFormat.SampleRate, frameSize)
						if err != nil {
							log.Errorf("processAsrAudio VAD detection failed: %v", err)
							continue
						}

						//on first voice detection trigger, assign vadPcmData to pcmData for audio data completeness, subsequent audio data all goes to ASR
						if haveVoice && !clientHaveVoice {
							//on first voice detection, keep at most 200ms of leading silence data
							allData := state.AsrAudioBuffer.GetAndClearAllData()
							pcmData = allData
						}
					}
					//log.Debugf("isVad, pcmData len: %d, vadPcmData len: %d, haveVoice: %v", len(pcmData), len(vadPcmData), haveVoice)
				}

				if !haveVoice || state.Asr.AutoEnd {
					state.Vad.AddIdleDuration(int64(frameDurationMs))
					idleDuration := state.Vad.GetIdleDuration()
					log.Infof("idle time: %dms", idleDuration)
					if idleDuration > state.GetMaxIdleDuration() {
						log.Infof("exceeded idle duration: %dms, disconnecting", idleDuration)
						//disconnect
						onClose()
						return
					}
				}

				if haveVoice {
					hasLoggedFirstTextExtendedWait = false
					//log.Infof("voice detected, len: %d", len(pcmData))
					state.SetClientHaveVoice(true)
					state.SetClientHaveVoiceLastTime(time.Now().UnixMilli())
					if !state.Asr.AutoEnd {
						state.Vad.ResetIdleDuration()
					}
					// accumulate detected voice duration (also update duration during process)
					state.Vad.AddVoiceDuration(int64(frameDurationMs))

					continuousVoiceDuration := state.Vad.GetVoiceContinuousDuration()
					if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 1 && continuousVoiceDuration > 360 {
						// only execute if not triggered before, ensure executed only once
						if !hasTriggeredCancel {
							//in realtime mode, cancel any ongoing LLM and TTS
							log.Debugf("realtime mode VAD interrupt && voice duration exceeds %d ms, canceling ongoing LLM and TTS", continuousVoiceDuration)
							state.AfterAsrSessionCtx.CancelWithReason("ASRManager.ProcessVadAudio: realtime_mode=1 VAD interrupt")
							if a.session != nil {
								a.session.InterruptAndClearTTSQueue()
							}
							hasTriggeredCancel = true // mark as triggered
						}
					}
				} else {
					state.Vad.ResetVoiceContinuousDuration()

					// when no voice and no previous voice, reset accumulated voice duration
					// if had voice before but not this time, keep duration for subsequent logic to decide whether to reset
					if !clientHaveVoice {
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						//keep recent 10 frames
						/*
							if state.AsrAudioBuffer.GetFrameCount(frameSize) > vadNeedGetCount*3 {
								state.AsrAudioBuffer.RemoveAsrAudioData(1, frameSize)
							}*/
						continue
					}
				}

				if clientHaveVoice || haveVoice {
					// on first voice hit, immediately forward current buffered frame to avoid extremely short audio not being sent to ASR.

					//VAD recognition successful, send data to ASR audio channel
					//log.Infof("VAD recognition successful, sending data to ASR audio channel, len: %d", len(pcmData))
					state.Asr.AddAudioData(pcmData)

					// speaker recognition only receives frames determined as voiced, avoid sending leading silence and trailing silence to recognition stream.
					if haveVoice &&
						state.IsSpeakerEnabled() && state.HasSpeakerGroups() &&
						a.session != nil && a.session.speakerManager != nil {
						// start streaming recognition when voice first detected
						if !a.session.speakerManager.IsActive() {
							sampleRate := audioFormat.SampleRate
							agentId := a.session.clientState.AgentID
							if err := a.session.speakerManager.StartStreaming(ctx, sampleRate, agentId); err != nil {
								log.Warnf("start streaming speaker recognition failed: %v", err)
							} else {
								speakerInterruptTriggered.Store(false)
								lastSpeakerPeekDoneAt.Store(0)
								speakerPeekAudioMs = 0
							}
						}

						// send audio chunk
						if err := a.session.speakerManager.SendAudioChunk(ctx, speakerPcmData); err != nil {
							log.Warnf("send audio chunk to speaker recognition service failed: %v", err)
						} else if a.session.speakerManager.IsActive() {
							if audioFormat.Channels > 0 && audioFormat.SampleRate > 0 {
								speakerPeekAudioMs += int64(len(speakerPcmData)/audioFormat.Channels) * 1000 / int64(audioFormat.SampleRate)
							}

							if state.IsRealTime() &&
								viper.GetInt("chat.realtime_mode") == 3 &&
								!speakerInterruptTriggered.Load() &&
								speakerPeekAudioMs >= firstSpeakerPeekAudioThresholdMs {
								now := time.Now()
								lastDoneAt := lastSpeakerPeekDoneAt.Load()
								if (lastDoneAt <= 0 || now.Sub(time.Unix(0, lastDoneAt)) >= speakerPeekInterval) &&
									speakerPeekInFlight.CompareAndSwap(false, true) {
									reqSeq := atomic.AddUint64(&speakerPeekRequestSeq, 1)
									requestID := fmt.Sprintf("peek_%d_%d", now.UnixMilli(), reqSeq)

									go func(reqID string) {
										defer func() {
											lastSpeakerPeekDoneAt.Store(time.Now().UnixNano())
											speakerPeekInFlight.Store(false)
										}()

										if a.session == nil || a.session.speakerManager == nil || !a.session.speakerManager.IsActive() {
											return
										}

										peekCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
										defer cancel()

										peekResult, throttled, err := a.session.speakerManager.PeekAndIdentify(peekCtx, reqID)
										if err != nil {
											if ctx.Err() == nil {
												log.Debugf("speaker peek failed: device=%s, request_id=%s, err=%v", state.DeviceID, reqID, err)
											}
											return
										}
										if throttled {
											return
										}
										if peekResult == nil || !peekResult.Identified {
											return
										}
										if !speakerInterruptTriggered.CompareAndSwap(false, true) {
											return
										}

										log.Infof(
											"realtime mode speaker peek hit, immediate interrupt: device=%s, speaker=%s, confidence=%.4f, threshold=%.4f",
											state.DeviceID,
											peekResult.SpeakerName,
											peekResult.Confidence,
											peekResult.Threshold,
										)
										a.session.MarkTurnSpeakerInterrupted()
										state.AfterAsrSessionCtx.CancelWithReason("ASRManager.ProcessVadAudio: realtime_mode=3 speaker peek interrupt")
										a.session.InterruptAndClearTTSQueue()
									}(requestID)
								}
							}
						}
					}
				}

				//had voice before but not detected this time, need to determine if speaking has stopped
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					// check voice duration with audio, reset clientHaveVoice if less than 300ms, avoid misjudgment from short voice
					voiceDurationInSession := state.Vad.GetVoiceDurationInSession()
					if voiceDurationInSession < 100 {
						log.Debugf("voice duration too short (%dms < 300ms), resetting clientHaveVoice", voiceDurationInSession)
						state.SetClientHaveVoice(false)
						state.Vad.ResetVoiceDuration()
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						continue
					}

					idleDuration := state.Vad.GetIdleDuration()
					if state.IsRealTime() && !state.Asr.HasReceivedText() {
						preTextSilenceDuration := state.GetPreAsrTextSilenceDuration()
						if idleDuration <= preTextSilenceDuration {
							log.Debugf(
								"realtime mode pre-ASR-text silence timeout, delaying close: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
								state.Status,
								idleDuration,
								preTextSilenceDuration,
								state.Vad.GetVoiceDuration(),
								voiceDurationInSession,
								state.Asr.GetHistoryAudioLen(),
							)
							continue
						}

						if !hasLoggedFirstTextExtendedWait {
							log.Debugf(
								"realtime mode silence timeout without ASR text, keeping current ASR stream and forwarding audio: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
								state.Status,
								idleDuration,
								preTextSilenceDuration,
								state.Vad.GetVoiceDuration(),
								voiceDurationInSession,
								state.Asr.GetHistoryAudioLen(),
							)
							hasLoggedFirstTextExtendedWait = true
						}
						continue
					}

					if state.IsSilence(idleDuration) { // judgment from voice to silence
						log.Debugf(
							"voice end detected, preparing to stop ASR: status=%s, idle=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d, pending_restart=%v",
							state.Status,
							idleDuration,
							state.Vad.GetVoiceDuration(),
							state.Vad.GetVoiceDurationInSession(),
							state.Asr.GetHistoryAudioLen(),
						)
						// reset flag before OnVoiceSilence so it can be triggered again next time
						hasTriggeredCancel = false
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						state.OnVoiceSilence()
						state.VoiceStatus.Reset()
						continue
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// releaseResource release ASR resource (internal method)
func (a *ASRManager) releaseResource() {
	a.resourceMu.Lock()
	defer a.resourceMu.Unlock()
	if a.asrResource != nil {
		pool.Release(a.asrResource)
		a.asrResource = nil
		log.Debugf("ASR resource returned")
	}
}

// Cleanup cleanup ASR resource (for external use)
func (a *ASRManager) Cleanup() {
	a.releaseResource()
}

// restartAsrRecognition restart ASR recognition
func (a *ASRManager) RestartAsrRecognition(ctx context.Context) error {
	state := a.clientState
	log.Debugf("restarting ASR recognition")
	if a.session != nil {
		a.session.ResetTurnSpeakerInterrupted()
	}

	// cancel current ASR context
	state.Asr.CancelWithReason("ASRManager.RestartAsrRecognition: cancel previous ASR context before restart")

	state.Asr.ResetReceivedText()
	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()
	state.Asr.ClearHistoryAudio() // clear history audio cache

	// check if resource already exists, acquire if not
	a.resourceMu.Lock()
	var asrProvider asr.AsrProvider
	if a.asrResource == nil {
		// need to acquire new resource
		a.resourceMu.Unlock()

		asrWrapper, err := pool.Acquire[asr.AsrProvider](
			"asr",
			state.DeviceConfig.Asr.Provider,
			state.DeviceConfig.Asr.Config,
		)
		if err != nil {
			log.Errorf("acquire ASR resource failed: %v", err)
			return fmt.Errorf("acquire ASR resource failed: %v", err)
		}

		// save resource reference to private field
		a.resourceMu.Lock()
		a.asrResource = asrWrapper
		asrProvider = asrWrapper.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("acquired new ASR resource")
	} else {
		// reuse existing resource
		asrProvider = a.asrResource.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("reusing existing ASR resource")
	}

	// recreate ASR context and channels
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)

	// restart streaming recognition
	asrResultChannel, err := asrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		// recognition failed, return resource (because resource may be corrupted)
		a.releaseResource()
		log.Errorf("restart ASR streaming recognition failed: %v", err)
		return fmt.Errorf("restart ASR streaming recognition failed: %v", err)
	}

	state.AsrResultChannel = asrResultChannel
	// reset statistics time, used to calculate total time of this conversation round
	state.MarkTurnStart()
	if a.session != nil {
		a.session.TraceTurnStart(state.Asr.Ctx, state.Statistic.TurnStartTs)
	}
	log.Debugf("ASR recognition restarted successfully")
	return nil
}

// StartAsrRecognitionLoop start ASR recognition result processing loop
// onMessageSave: message save callback function
// onError: error callback function (e.g. close session)
func (a *ASRManager) StartAsrRecognitionLoop(
	ctx context.Context,
	onMessageSave AsrMessageSaveCallback,
	onError func(error),
) {
	state := a.clientState

	// start a goroutine to process ASR results
	go func() {
		// use defer to ensure ASR resources are released when goroutine exits
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("asr result processing goroutine panic: %v, stack: %s", r, string(debug.Stack()))
			}
			// release resources regardless of normal exit or panic
			a.releaseResource()
		}()

		//max idle 60s
		var startIdleTime, maxIdleTime int64
		startIdleTime = time.Now().Unix()
		maxIdleTime = 60

		// wait count when status does not allow restart (avoid infinite loop)
		var invalidStatusWaitCount int64
		maxInvalidStatusWaitCount := int64(10) // max 10 waits (approximately 1 second)

		// empty result short-term protection: avoid ASR service abnormally returning empty strings continuously, causing main loop to spin endlessly
		const emptyResultProtectWindow = 3 * time.Second
		const maxEmptyResultInWindow = 3
		emptyResultWindowStart := time.Now()
		emptyResultCount := 0

		// recoverable error short-term protection: avoid infinite reconnect when upstream keeps returning instance invalid
		const recoverableErrorProtectWindow = 10 * time.Second
		const maxRecoverableErrorInWindow = 3
		recoverableErrorWindowStart := time.Now()
		recoverableErrorCount := 0

		isAllowedToRestart := func() bool {
			allowed := state.Status == ClientStatusListening || state.Status == ClientStatusListenStop
			if state.IsRealTime() {
				allowed = state.Status != ClientStatusInit
			}
			return allowed
		}

		for {
			select {
			case <-ctx.Done():
				log.Debugf("asr ctx done")
				return
			default:
			}

			result, isRetry, err := state.RetireAsrResult(ctx)
			if err != nil {
				log.Errorf("process asr result failed: %v", err)
				if onError != nil {
					onError(err)
				}
				return
			}
			if !isRetry {
				log.Debugf("asrResult is not retry, return")
				return
			}
			text := result.Text

			//measure ASR duration
			log.Debugf("processing asr result: %s, duration: %d ms", text, state.GetAsrDuration())

			if result.RetryReason != "" {
				now := time.Now()
				if now.Sub(recoverableErrorWindowStart) > recoverableErrorProtectWindow {
					recoverableErrorWindowStart = now
					recoverableErrorCount = 0
				}
				recoverableErrorCount++
				log.Warnf(
					"ASR recoverable error: reason=%s, count=%d/%d, status=%s",
					result.RetryReason,
					recoverableErrorCount,
					maxRecoverableErrorInWindow,
					state.Status,
				)

				if recoverableErrorCount >= maxRecoverableErrorInWindow {
					err := fmt.Errorf("ASR consecutive recoverable errors within short time (%d times/%s), stopping retry and disconnecting", recoverableErrorCount, recoverableErrorProtectWindow)
					log.Errorf(err.Error())
					if onError != nil {
						onError(err)
					}
					return
				}

				switch result.RetryReason {
				case asr_types.RetryReasonDoubaoResponseCode45000081, asr_types.RetryReasonXunfeiServiceInstanceInvalid, asr_types.RetryReasonAliyunQwen3ConnectionClosed:
					a.releaseResource()
					if isAllowedToRestart() {
						invalidStatusWaitCount = 0
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("ASR restart after recoverable error failed: reason=%s, err=%v", result.RetryReason, restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					}

					log.Warnf("ASR recoverable error but current state does not allow immediate restart: reason=%s, status=%s, realtime=%v", result.RetryReason, state.Status, state.IsRealTime())
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: recoverable error but restart not allowed yet")
					continue
				case asr_types.RetryReasonDoubaoWaitingNextPacketTimeout:
					log.Warnf("doubao ASR session idle timeout, suspending current stream and waiting for next voice to rebuild")
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: doubao waiting next packet timeout")
					continue
				}
			}

			if text != "" {
				// reset empty result count after successful recognition
				emptyResultWindowStart = time.Now()
				emptyResultCount = 0
				recoverableErrorWindowStart = time.Now()
				recoverableErrorCount = 0

				//in realtime mode, need to stop current LLM and TTS
				if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 2 {
					shouldInterrupt := true
					if a.session != nil && a.session.isRealtimeMcpAudioGateActive() {
						shouldInterrupt = false
						log.Debugf("device %s realtime media playback gate active, deferring to ASR final gate decision, skipping ASR result interrupt", state.DeviceID)
					}
					if shouldInterrupt {
						log.Debugf("OnListenStart realtime mode, stopping current LLM and TTS")
						state.AfterAsrSessionCtx.CancelWithReason("ASRManager.StartAsrRecognitionLoop: realtime_mode=2 ASR result interrupt")
						if a.session != nil {
							a.session.InterruptAndClearTTSQueue()
						}
					}
				}

				// reset retry counter
				startIdleTime = time.Now().Unix()

				//when ASR result obtained, end voice input (OnVoiceSilence will asynchronously get speaker result)
				state.OnVoiceSilence()

				// get cached speaker result (with timeout)
				speakerResult := a.getSpeakerResult()
				speakerInterrupted := false
				if a.session != nil {
					speakerInterrupted = a.session.ConsumeTurnSpeakerInterrupted()
				}
				state.MarkAsrFinalText()
				if a.session != nil {
					a.session.TraceAsrFinalText(ctx, time.Now().UnixMilli())
				}

				if a.session != nil {
					payload, stop, hookErr := a.session.hookHub.EmitASROutput(a.session.hookContext(ctx), chathooks.ASROutputData{Text: text, SpeakerResult: speakerResult})
					if hookErr != nil {
						log.Warnf("ASR_OUTPUT hook execution failed: %v", hookErr)
					}
					text = payload.Text
					speakerResult = payload.SpeakerResult
					if stop {
						log.Infof("ASR_OUTPUT hook requested to stop current flow")
						state.Asr.ClearHistoryAudio()
						continue
					}
				}

				if a.session != nil {
					allowChat, denyReason := a.session.ShouldAllowSpeakerChat(speakerResult, speakerInterrupted)
					if !allowChat {
						log.Infof(
							"discard ASR result and skip STT/LLM: device=%s, reason=%s, speaker_interrupted=%v, speaker_result=%+v, text=%q",
							state.DeviceID,
							denyReason,
							speakerInterrupted,
							speakerResult,
							text,
						)
						state.Asr.ClearHistoryAudio()

						if !state.IsRealTime() {
							return
						}
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("restart recognition after discarding ASR result failed: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					}
				}

				// create user message, use hook-rewritten text for subsequent side effects
				userMsg := &schema.Message{
					Role:    schema.User,
					Content: text,
				}

				// generate MessageID (use MD5 hash to shorten length, avoid exceeding database varchar(64) limit)
				// original format: {SessionID}-{Role}-{Timestamp}
				rawMessageID := fmt.Sprintf("%s-%s-%d",
					state.SessionID,
					userMsg.Role,
					time.Now().UnixMilli())
				// use MD5 hash to generate fixed 32-char hex string
				hash := md5.Sum([]byte(rawMessageID))
				messageID := hex.EncodeToString(hash[:])

				// add to memory synchronously (for LLM context)
				state.AddMessage(userMsg)

				// get audio data (ASR history audio)
				audioData := state.Asr.GetHistoryAudio()
				state.Asr.ClearHistoryAudio()

				// save message via callback
				if onMessageSave != nil {
					onMessageSave(userMsg, messageID, audioData)
				}

				// ASR result sent to client also uses hook-rewritten text
				err = a.serverTransport.SendAsrResult(text)
				if err != nil {
					log.Errorf("send asr message failed: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				if a.session != nil {
					handledByRealtimeGate, gateErr := a.session.tryHandleRealtimeMcpAudioASR(ctx, text)
					if gateErr != nil {
						log.Warnf("realtime media playback quick control failed: device=%s text=%q err=%v", state.DeviceID, text, gateErr)
					}
					if handledByRealtimeGate {
						if !state.IsRealTime() {
							return
						}
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("restart ASR recognition after realtime media control failed: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					}
				}

				// add to queue (migrated to ASRManager)
				if err := a.addAsrResultToQueue(text, speakerResult); err != nil {
					log.Errorf("start conversation failed: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// in non-realtime mode, ASR recognition complete, return resource
				// in realtime mode, resources are auto-managed in RestartAsrRecognition (return old resource first, then acquire new)
				if !state.IsRealTime() {
					return
				}

				// in realtime mode, restart ASR recognition (RestartAsrRecognition returns old resource first, then acquires new)
				if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
					log.Errorf("failed to restart ASR recognition: %v", restartErr)
					if onError != nil {
						onError(restartErr)
					}
					return
				}
				// in realtime mode, continue loop to process next ASR result
				continue
			} else {
				log.Debugf(
					"ASR empty result details: status=%s, emptyReason=%s, client_voice_stop=%v, history_audio_samples=%d, voice_duration=%dms, voice_duration_in_session=%dms, idle_duration=%dms, realtime=%v",
					state.Status,
					result.EmptyReason,
					state.GetClientVoiceStop(),
					state.Asr.GetHistoryAudioLen(),
					state.Vad.GetVoiceDuration(),
					state.Vad.GetVoiceDurationInSession(),
					state.Vad.GetIdleDuration(),
					state.IsRealTime(),
				)
				if result.EmptyReason != "" {
					log.Debugf("ASR empty result classified: reason=%s, status=%s", result.EmptyReason, state.Status)
					emptyResultWindowStart = time.Now()
					emptyResultCount = 0

					if result.EmptyReason == asr_types.EmptyReasonNoServerResponse ||
						result.EmptyReason == asr_types.EmptyReasonProviderEmptyFinal {
						state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: empty final result from provider")
						continue
					}
				}

				now := time.Now()
				if now.Sub(emptyResultWindowStart) > emptyResultProtectWindow {
					emptyResultWindowStart = now
					emptyResultCount = 0
				}
				emptyResultCount++
				if emptyResultCount >= maxEmptyResultInWindow {
					err := fmt.Errorf("ASR consecutive empty results within short time (%d times/%s), triggering protection and disconnecting", emptyResultCount, emptyResultProtectWindow)
					log.Errorf(err.Error())
					if onError != nil {
						onError(err)
					}
					return
				}

				// when text is empty
				select {
				case <-ctx.Done():
					log.Debugf("asr ctx done")
					return
				default:
				}

				log.Debugf("ready Restart Asr, state.Status: %s", state.Status)
				// in realtime mode, even if status is LLMStart or TTSStart, should continue listening (allow ASR restart)
				// in non-realtime mode, only Listening or ListenStop status allows ASR restart
				if isAllowedToRestart() {
					// status allows restart, reset wait count
					invalidStatusWaitCount = 0
					// text is empty, check if ASR restart is needed
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR recognition result is empty, attempting to restart ASR recognition, diff ts: %d", diffTs)
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("restart ASR recognition failed: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					} else {
						log.Warnf("ASR recognition result is empty, max idle time reached: %d", maxIdleTime)
						if onError != nil {
							onError(fmt.Errorf("ASR recognition result is empty, max idle time reached: %d", maxIdleTime))
						}
						return
					}
				} else {
					// when status does not allow restart, continue loop after brief wait, give status a chance to recover
					invalidStatusWaitCount++
					if invalidStatusWaitCount >= maxInvalidStatusWaitCount {
						// wait timeout, exit loop
						log.Debugf("status is %s, realtime: %v, still no change after waiting %d times, exiting ASR recognition loop", state.Status, state.IsRealTime(), maxInvalidStatusWaitCount)
						return
					}
					// continue loop after brief wait, waiting for status to recover
					log.Debugf("status is %s, realtime: %v, restart not allowed, waiting for status recovery (wait count: %d/%d)", state.Status, state.IsRealTime(), invalidStatusWaitCount, maxInvalidStatusWaitCount)
					time.Sleep(200 * time.Millisecond) // wait 200ms
					continue
				}
			}
		}
	}()
}

// getSpeakerResult get cached speaker result (with timeout)
func (a *ASRManager) getSpeakerResult() *speaker.IdentifyResult {
	if a.session == nil || a.session.speakerManager == nil {
		return nil
	}

	log.Debugf("speakerManager: %+v, IsActive: %+v", a.session.speakerManager, a.session.speakerManager.IsActive())

	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()

	var speakerResult *speaker.IdentifyResult
	select {
	case <-a.session.speakerResultReady:
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
	case <-timeout.C:
		// read current result on timeout (may be nil)
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
		log.Debugf("get speaker recognition result timed out, using current result")
	}
	log.Debugf("got speaker recognition result: %+v", speakerResult)
	return speakerResult
}

// addAsrResultToQueue add ASR result to queue (migrated to ASRManager)
func (a *ASRManager) addAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	if a.session == nil {
		return fmt.Errorf("session is nil")
	}
	return a.session.AddAsrResultToQueue(text, speakerResult)
}
