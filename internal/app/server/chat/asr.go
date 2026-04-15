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
	session         *ChatSession // used for accessing speakerManager

	// ASR resource as private field management
	asrResource *pool.ResourceWrapper[asr.AsrProvider]
	resourceMu  sync.RWMutex // protected resource access
}

func NewASRManager(clientState *ClientState, serverTransport *ServerTransport, opts ...ASRManagerOption) *ASRManager {
	asr := &ASRManager{
		clientState:     clientState,
		serverTransport: serverTransport,
		session:         nil, // laterafterthrough SetSession set
	}
	for _, opt := range opts {
		opt(asr)
	}
	return asr
}

// ProcessVadAudio start VAD audio process
func (a *ASRManager) ProcessVadAudio(ctx context.Context, onClose func()) {
	state := a.clientState
	go func() {
		hasTriggeredCancel := true // flag bit, record whether already triggered cancel operation (when voiceDuration > 120)
		hasLoggedFirstTextExtendedWait := false
		speakerInterruptTriggered := atomic.Bool{}
		speakerPeekInFlight := atomic.Bool{}
		lastSpeakerPeekDoneAt := atomic.Int64{}
		var speakerPeekAudioMs int64
		var speakerPeekRequestSeq uint64
		const speakerPeekInterval = 200 * time.Millisecond
		const firstSpeakerPeekAudioThresholdMs int64 = 400
		audioFormat := state.InputAudioFormat
		// use a sufficiently large buffer area for decode (assume max frame duration is 120ms)
		maxFrameSize := audioFormat.SampleRate * audioFormat.Channels * 120 / 1000
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, 20) // pass a default value for creating decoder
		if err != nil {
			log.Errorf("get decoder failed: %v", err)
			return
		}

		// get frame size and frame duration from first frame actual data
		var frameSize int
		var frameDurationMs int
		var vadNeedGetCount int // number of frames needed for VAD，will be calculated after first frame

		// VAD resource changed to lazy load + idle release, avoid long-term exclusive resource pool instance.
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

			// check if provider is empty, if empty then log warning
			if vadProviderName == "" {
				log.Warnf("VAD provider is empty, try to get from config")
			} else {
				log.Debugf("get VAD resource: provider=%s", vadProviderName)
			}

			wrapper, err := pool.Acquire[inter.VAD](
				"vad",
				vadProviderName,
				vadProviderConfig,
			)
			if err != nil {
				log.Errorf("failed to get VAD resource: provider=%s, config=%+v, error=%v", vadProviderName, vadProviderConfig, err)
				return false
			}
			vadWrapper = wrapper
			vadProvider = wrapper.GetProvider()
			vadLastUseAt = time.Now()
			return true
		}

		for {
			// use max frame size as buffer area, will get actual frame size after decode
			pcmFrame := make([]float32, maxFrameSize)

			select {
			case <-vadIdleTicker.C:
				if vadWrapper != nil && !vadLastUseAt.IsZero() && time.Since(vadLastUseAt) >= vadIdleReleaseTimeout {
					releaseVad("idle_timeout")
				}
				continue
			case opusFrame, ok := <-state.OpusAudioBuffer:
				//log.Debugf("processAsrAudio receiveaudio data, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio audio channel closed")
					return
				}

				var skipVad bool
				var haveVoice bool
				clientHaveVoice := state.GetClientHaveVoice()
				if state.Asr.AutoEnd || state.ListenMode == "manual" {
					skipVad = true         //skip vad
					clientHaveVoice = true //had voice before
					haveVoice = true       //has voice this time
				}

				if state.GetClientVoiceStop() { //alreadystop speak thennoreceiveaudio data
					//log.Infof("client-side stop speaking, skipaudio data")
					continue
				}

				//log.Debugf("clientVoiceStop: %+v, asrDataSize: %d, listenMode: %s, isSkipVad: %v\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize(), state.ListenMode, skipVad)

				n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
				if err != nil {
					log.Errorf("decodefailed: %v", err)
					continue
				}

				// dynamically calculate frame size and frame duration from actual decoded data
				if frameSize == 0 {
					// first frame: calculate frame info from actual decoded data
					frameSize = n
					samplesPerChannel := n / audioFormat.Channels
					frameDurationMs = samplesPerChannel * 1000 / audioFormat.SampleRate
					audioFormat.FrameDuration = frameDurationMs

					// calculate VAD needed frame count
					vadNeedGetCount = 1
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						// silero_vad needs at least 60ms of audio data
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

				// check if frame size is consistent (should be consistent in normal situation, but use actual value when inconsistent)
				if n != frameSize {
					log.Debugf("frame size inconsistent: expected=%d, actual=%d, use actual value", frameSize, n)
					// recalculate this frame duration
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

					// calculate VAD needed minimum data amount (60ms for silero_vad)
					vadNeedMinSize := frameSize
					if state.DeviceConfig.Vad.Provider == "silero_vad" {
						vadNeedMinSize = vadNeedGetCount * frameSize
					}

					if state.AsrAudioBuffer.GetAsrDataSize() >= vadNeedMinSize {
						// if need to do vad, at least need to get 60ms of audio data
						vadPcmData = state.AsrAudioBuffer.GetAsrData(vadNeedGetCount, frameSize)

						// if voice already detected, then do not do vad detect, directly pass pcmData to asr
						// use VAD resource obtained outside loop for detect
						// reset VAD state
						vadLastUseAt = time.Now()
						if err := vadProvider.Reset(); err != nil {
							log.Errorf("reset vad failed: %v", err)
							continue
						}

						// do VAD detect
						vadLastUseAt = time.Now()
						haveVoice, err = vadProvider.IsVADExt(vadPcmData, audioFormat.SampleRate, frameSize)
						if err != nil {
							log.Errorf("processAsrAudio VAD detect failed: %v", err)
							continue
						}

						// when voice first triggered recognize, for voice data integrity assign vadPcmData to pcmData, all subsequent audio data enters asr
						if haveVoice && !clientHaveVoice {
							// when voice first detected, keep at most 200ms of leading silence data
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
						log.Infof("exceed idle duration: %dms, disconnect", idleDuration)
						// disconnect
						onClose()
						return
					}
				}

				if haveVoice {
					hasLoggedFirstTextExtendedWait = false
					//log.Infof("detected voice, len: %d", len(pcmData))
					state.SetClientHaveVoice(true)
					state.SetClientHaveVoiceLastTime(time.Now().UnixMilli())
					if !state.Asr.AutoEnd {
						state.Vad.ResetIdleDuration()
					}
					// accumulate detected voice duration（simultaneously update duration during process）
					state.Vad.AddVoiceDuration(int64(frameDurationMs))

					continuousVoiceDuration := state.Vad.GetVoiceContinuousDuration()
					if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 1 && continuousVoiceDuration > 360 {
						// only execute in situations not triggered, ensure only execute once
						if !hasTriggeredCancel {
							// in realtime mode, if there are ongoing llm and tts then cancel them
							log.Debugf("realtime mode vad interrupt && voice duration exceeds %d ms, if there are ongoing llm and tts then cancel them", continuousVoiceDuration)
							state.AfterAsrSessionCtx.CancelWithReason("ASRManager.ProcessVadAudio: realtime_mode=1 VAD interrupt")
							if a.session != nil {
								a.session.InterruptAndClearTTSQueue()
							}
							hasTriggeredCancel = true // marked as triggered
						}
					}
				} else {
					state.Vad.ResetVoiceContinuousDuration()

					// when no voice, if no voice before either, then reset accumulated voice duration
					// if had voice before but not this time, keep duration value, let subsequent logic determine if should reset
					if !clientHaveVoice {
						speakerInterruptTriggered.Store(false)
						lastSpeakerPeekDoneAt.Store(0)
						speakerPeekAudioMs = 0
						//keep nearly 10 frames
						/*
							if state.AsrAudioBuffer.GetFrameCount(frameSize) > vadNeedGetCount*3 {
								state.AsrAudioBuffer.RemoveAsrAudioData(1, frameSize)
							}*/
						continue
					}
				}

				if clientHaveVoice || haveVoice {
					// when voice first hit also immediately forward current cache frame, avoid extremely short voice whole segment not sent to ASR.

					// vad recognize successful, send data to asr audio channel
					//log.Infof("vad recognize successful, toasraudio channelinsenddata, len: %d", len(pcmData))
					state.Asr.AddAudioData(pcmData)

					// voiceprint only receives currently judged voice frames, avoid sending leading silence and tail silence to recognize stream.
					if haveVoice &&
						state.IsSpeakerEnabled() && state.HasSpeakerGroups() &&
						a.session != nil && a.session.speakerManager != nil {
						// when voice first detected, start streaming recognize
						if !a.session.speakerManager.IsActive() {
							sampleRate := audioFormat.SampleRate
							agentId := a.session.clientState.AgentID
							if err := a.session.speakerManager.StartStreaming(ctx, sampleRate, agentId); err != nil {
								log.Warnf("start voiceprint recognize stream failed: %v", err)
							} else {
								speakerInterruptTriggered.Store(false)
								lastSpeakerPeekDoneAt.Store(0)
								speakerPeekAudioMs = 0
							}
						}

						// send audio chunk
						if err := a.session.speakerManager.SendAudioChunk(ctx, speakerPcmData); err != nil {
							log.Warnf("sendaudio chunktovoiceprintrecognizeservicefailed: %v", err)
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
												log.Debugf("voiceprint peek failed: device=%s, request_id=%s, err=%v", state.DeviceID, reqID, err)
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
											"realtime pattern voiceprint peek hit, immediately interrupt: device=%s, speaker=%s, confidence=%.4f, threshold=%.4f",
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

				// already has voice, but this time no detected voice, then need to determine if already stopped speaking
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					// judge voice duration of audio, if less than 300ms then reset clientHaveVoice, avoid misjudgment caused by short voice
					voiceDurationInSession := state.Vad.GetVoiceDurationInSession()
					if voiceDurationInSession < 100 {
						log.Debugf("voice duration too short (%dms < 300ms), reset clientHaveVoice", voiceDurationInSession)
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
								"realtime mode not yet received ASR first text, delay close by silence threshold: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
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
								"realtime mode silence timeout and still not received ASR text, continue keep current ASR stream and forward audio: status=%s, idle=%dms, pre_text_timeout=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d",
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

					if state.IsSilence(idleDuration) { // from having voice to silence judgment
						log.Debugf(
							"determine voice end, prepare stop ASR: status=%s, idle=%dms, voice_duration=%dms, voice_duration_in_session=%dms, history_audio_samples=%d, pending_restart=%v",
							state.Status,
							idleDuration,
							state.Vad.GetVoiceDuration(),
							state.Vad.GetVoiceDurationInSession(),
							state.Asr.GetHistoryAudioLen(),
						)
						// reset flag bit before OnVoiceSilence, so it can be triggered again next time
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

// Cleanup cleanup ASR resource (for external call)
func (a *ASRManager) Cleanup() {
	a.releaseResource()
}

// restartAsrRecognition restart ASR recognize
func (a *ASRManager) RestartAsrRecognition(ctx context.Context) error {
	state := a.clientState
	log.Debugf("restart ASR recognize start")
	if a.session != nil {
		a.session.ResetTurnSpeakerInterrupted()
	}

	// cancel current ASR context
	state.Asr.CancelWithReason("ASRManager.RestartAsrRecognition: cancel previous ASR context before restart")

	state.Asr.ResetReceivedText()
	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()
	state.Asr.ClearHistoryAudio() // clear history audio cache

	// check if already have resource, if not then get
	a.resourceMu.Lock()
	var asrProvider asr.AsrProvider
	if a.asrResource == nil {
		// need to get new resource
		a.resourceMu.Unlock()

		asrWrapper, err := pool.Acquire[asr.AsrProvider](
			"asr",
			state.DeviceConfig.Asr.Provider,
			state.DeviceConfig.Asr.Config,
		)
		if err != nil {
			log.Errorf("failed to get ASR resource: %v", err)
			return fmt.Errorf("failed to get ASR resource: %v", err)
		}

		// save resource reference to private field
		a.resourceMu.Lock()
		a.asrResource = asrWrapper
		asrProvider = asrWrapper.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("get new ASR resource")
	} else {
		// reuse existing resource
		asrProvider = a.asrResource.GetProvider()
		a.resourceMu.Unlock()
		log.Debugf("reuse existing ASR resource")
	}

	// recreate ASR context and channel
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)

	// restart streaming recognize
	asrResultChannel, err := asrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		// recognize failed, return resource (because resource may be damaged)
		a.releaseResource()
		log.Errorf("restart ASR streaming recognize failed: %v", err)
		return fmt.Errorf("restart ASR streaming recognize failed: %v", err)
	}

	state.AsrResultChannel = asrResultChannel
	// reset count time, used to calculate overall time consumption of this round of conversation
	state.MarkTurnStart()
	if a.session != nil {
		a.session.TraceTurnStart(state.Asr.Ctx, state.Statistic.TurnStartTs)
	}
	log.Debugf("restart ASR recognize successful")
	return nil
}

// StartAsrRecognitionLoop start ASR recognize result process loop
// onMessageSave: message save callback function
// onError: error process callback function (such as close session)
func (a *ASRManager) StartAsrRecognitionLoop(
	ctx context.Context,
	onMessageSave AsrMessageSaveCallback,
	onError func(error),
) {
	state := a.clientState

	// start a goroutine to process asr result
	go func() {
		// use defer to ensure goroutine exit when releasing ASR resource
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("asr result process goroutine panic: %v, stack: %s", r, string(debug.Stack()))
			}
			// whether normal exit or panic, all release resources
			a.releaseResource()
		}()

		// maximum idle 60s
		var startIdleTime, maxIdleTime int64
		startIdleTime = time.Now().Unix()
		maxIdleTime = 60

		// state not allow restart wait count (avoid infinite loop)
		var invalidStatusWaitCount int64
		maxInvalidStatusWaitCount := int64(10) // at most wait 10 times (about 1 second)

		// empty result short-time protected: avoid ASR service abnormal continuously return empty string cause main process dead loop
		const emptyResultProtectWindow = 3 * time.Second
		const maxEmptyResultInWindow = 3
		emptyResultWindowStart := time.Now()
		emptyResultCount := 0

		// recoverable error short-time protected: avoid upstream continuously return instance invalid infinite reconnection
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

			// count asr time consumption
			log.Debugf("process asr result: %s, time consumption: %d ms", text, state.GetAsrDuration())

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
					err := fmt.Errorf("ASR continuously trigger recoverable error in short time (%d times/%s), stop retry and disconnect", recoverableErrorCount, recoverableErrorProtectWindow)
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
							log.Errorf("ASR recoverable error after restart recognize failed: reason=%s, err=%v", result.RetryReason, restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					}

					log.Warnf("ASR recoverable error occur when current state not allow immediately restart: reason=%s, status=%s, realtime=%v", result.RetryReason, state.Status, state.IsRealTime())
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: recoverable error but restart not allowed yet")
					continue
				case asr_types.RetryReasonDoubaoWaitingNextPacketTimeout:
					log.Warnf("doubao ASR session idle timeout, suspend current stream and wait for next voice to rebuild")
					state.Asr.CancelWithReason("ASRManager.StartAsrRecognitionLoop: doubao waiting next packet timeout")
					continue
				}
			}

			if text != "" {
				// after recognize successful reset empty result count
				emptyResultWindowStart = time.Now()
				emptyResultCount = 0
				recoverableErrorWindowStart = time.Now()
				recoverableErrorCount = 0

				// if in realtime mode, need to stop current llm and tts
				if state.IsRealTime() && viper.GetInt("chat.realtime_mode") == 2 {
					shouldInterrupt := true
					if a.session != nil && a.session.isRealtimeMcpAudioGateActive() {
						shouldInterrupt = false
						log.Debugf("device %s realtime media playback gate active, delay to ASR final gate determination, skip ASR result interrupt", state.DeviceID)
					}
					if shouldInterrupt {
						log.Debugf("OnListenStart in realtime mode, stop current llm and tts")
						state.AfterAsrSessionCtx.CancelWithReason("ASRManager.StartAsrRecognitionLoop: realtime_mode=2 ASR result interrupt")
						if a.session != nil {
							a.session.InterruptAndClearTTSQueue()
						}
					}
				}

				// reset retry counter
				startIdleTime = time.Now().Unix()

				// when get asr result, end voice input (OnVoiceSilence will asynchronously get voiceprint result)
				state.OnVoiceSilence()

				// get cached voiceprint result (with timeout)
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
							log.Errorf("discard ASR result after restart recognize failed: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					}
				}

				// create user message, use hook modified text to enter subsequent use chain
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
				// use MD5 hash to generate fixed 32-character hexadecimal string
				hash := md5.Sum([]byte(rawMessageID))
				messageID := hex.EncodeToString(hash[:])

				// synchronization add to memory (used for LLM context)
				state.AddMessage(userMsg)

				// get audio data (ASR history audio)
				audioData := state.Asr.GetHistoryAudio()
				state.Asr.ClearHistoryAudio()

				// through callback save message
				if onMessageSave != nil {
					onMessageSave(userMsg, messageID, audioData)
				}

				// send to client-side ASR result also use hook modified text
				err = a.serverTransport.SendAsrResult(text)
				if err != nil {
					log.Errorf("send asr message failed: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// add to queue (migrated to ASRManager in process)
				if err := a.addAsrResultToQueue(text, speakerResult); err != nil {
					log.Errorf("start conversation failed: %v", err)
					if onError != nil {
						onError(err)
					}
					return
				}

				// non-realtime mode, ASR recognize complete, return resource
				// in realtime mode, resource will be automatically managed in RestartAsrRecognition (first return old resource then get new resource)
				if !state.IsRealTime() {
					return
				}

				// in realtime mode, restart ASR recognize (RestartAsrRecognition will first return old resource then get new resource)
				if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
					log.Errorf("restart ASR recognize failed: %v", restartErr)
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
					log.Debugf("ASR empty result already classified: reason=%s, status=%s", result.EmptyReason, state.Status)
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
					err := fmt.Errorf("ASR continuously return empty result in short time (%d times/%s), trigger protected and disconnect", emptyResultCount, emptyResultProtectWindow)
					log.Errorf(err.Error())
					if onError != nil {
						onError(err)
					}
					return
				}

				// text is empty situation
				select {
				case <-ctx.Done():
					log.Debugf("asr ctx done")
					return
				default:
				}

				log.Debugf("ready Restart Asr, state.Status: %s", state.Status)
				// in realtime mode, even if state is LLMStart or TTSStart, should also continue listen (allow restart ASR)
				// non-realtime mode, only when Listening or ListenStop state then allow restart ASR
				if isAllowedToRestart() {
					// state allow restart, reset wait count
					invalidStatusWaitCount = 0
					// text is empty, check if need restart ASR
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR recognize result empty, try restart ASR recognize, diff ts: %d", diffTs)
						if restartErr := a.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("restart ASR recognize failed: %v", restartErr)
							if onError != nil {
								onError(restartErr)
							}
							return
						}
						continue
					} else {
						log.Warnf("ASR recognize result empty, already reached maximum idle time: %d", maxIdleTime)
						if onError != nil {
							onError(fmt.Errorf("ASR recognize result empty, already reached maximum idle time: %d", maxIdleTime))
						}
						return
					}
				} else {
					// state not allow restart situation, short wait then continue loop, give state recovery chance
					invalidStatusWaitCount++
					if invalidStatusWaitCount >= maxInvalidStatusWaitCount {
						// wait timeout, exit loop
						log.Debugf("state is %s, realtime: %v, wait %d times after still no change, exit ASR recognize loop", state.Status, state.IsRealTime(), maxInvalidStatusWaitCount)
						return
					}
					// short wait then continue loop, wait state recovery
					log.Debugf("state is %s, realtime: %v, not allow restart, wait state recovery (wait count: %d/%d)", state.Status, state.IsRealTime(), invalidStatusWaitCount, maxInvalidStatusWaitCount)
					time.Sleep(200 * time.Millisecond) // wait 100ms
					continue
				}
			}
		}
	}()
}

// getSpeakerResult get cached voiceprint result (with timeout)
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
		// timeout after read current result (may be nil)
		a.session.speakerResultMu.RLock()
		speakerResult = a.session.pendingSpeakerResult
		a.session.speakerResultMu.RUnlock()
		log.Debugf("get voiceprint recognize result timeout, use current result")
	}
	log.Debugf("get voiceprint recognize result: %+v", speakerResult)
	return speakerResult
}

// addAsrResultToQueue add ASR result to queue (migrated to ASRManager in process)
func (a *ASRManager) addAsrResultToQueue(text string, speakerResult *speaker.IdentifyResult) error {
	if a.session == nil {
		return fmt.Errorf("session is nil")
	}
	return a.session.AddAsrResultToQueue(text, speakerResult)
}
