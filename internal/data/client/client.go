package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"sync"

	utypes "xiaozhi-esp32-server-golang/internal/domain/config/types"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	"xiaozhi-esp32-server-golang/internal/domain/memory"
	"xiaozhi-esp32-server-golang/internal/domain/speaker"
	"xiaozhi-esp32-server-golang/internal/domain/tts"

	. "xiaozhi-esp32-server-golang/internal/data/audio"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"
)

// Dialogue indicates conversation history
type Dialogue struct {
	mu       sync.RWMutex // protects Messages with read-write lock
	Messages []*schema.Message
}

const (
	ClientStatusInit       = "init"
	ClientStatusListening  = "listening"
	ClientStatusListenStop = "listenStop"
	ClientStatusLLMStart   = "llmStart"
	ClientStatusTTSStart   = "ttsStart"

	ListenPhaseIdle      = "idle"
	ListenPhaseStarting  = "starting"
	ListenPhaseListening = "listening"

	MemoryModeNone  = "none"
	MemoryModeShort = "short"
	MemoryModeLong  = "long"

	SpeakerChatModeOff            = "off"
	SpeakerChatModeIdentifiedOnly = "identified_only"
)

func NormalizeMemoryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case MemoryModeNone:
		return MemoryModeNone
	case MemoryModeLong:
		return MemoryModeLong
	default:
		return MemoryModeShort
	}
}

func NormalizeSpeakerChatMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SpeakerChatModeIdentifiedOnly:
		return SpeakerChatModeIdentifiedOnly
	default:
		return SpeakerChatModeOff
	}
}

type SendAudioData func(audioData []byte) error

// ClientState indicates client-side state
type ClientState struct {
	IsActivated bool
	// conversation history
	Dialogue *Dialogue
	// interrupt state
	Abort bool
	// audio pickup mode
	ListenMode string
	// listen start process state: idle / starting / listening
	ListenPhase string
	// device ID
	DeviceID string
	AgentID  string
	// session ID
	SessionID string

	// device config
	DeviceConfig utypes.UConfig

	Vad
	Asr
	Llm

	// TTS provider
	TTSProvider      tts.TTSProvider        // default TTS provider
	SpeakerTTSConfig map[string]interface{} // voiceprint recognition TTS config (complete config, priority use)
	// memory provider
	MemoryProvider memory.MemoryProvider
	MemoryContext  string // memory context

	// context control
	Ctx    context.Context
	Cancel context.CancelFunc

	SessionCtx         Ctx // conversation context at a time
	AfterAsrSessionCtx Ctx // context after ASR process

	// prompt, system hint words
	SystemPrompt string

	InputAudioFormat  AudioFormat // input audio format
	OutputAudioFormat AudioFormat // output audio format

	// opus received audio data buffer
	OpusAudioBuffer chan []byte

	// pcm received audio data buffer
	AsrAudioBuffer *AsrAudioBuffer

	VoiceStatus

	UdpSendAudioData SendAudioData // send audio data
	Statistic        Statistic     // time consumption count
	MqttLastActiveTs int64         // last active time
	VadLastActiveTs  int64         // vad last active time, exceeds 60s && no tts then disconnect

	Status string // state: listening, llmStart, ttsStart

	IsTtsStart        bool // whether tts started
	IsWelcomeSpeaking bool // whether already played welcome phrase
	IsWelcomePlaying  bool // whether playing welcome phrase

	// voiceprint recognition related
	SpeakerProvider speaker.SpeakerProvider // voiceprint recognition provider (initialized in session)

	// async get voiceprint result callback function (set in session)
	OnVoiceSilenceSpeakerCallback func(ctx context.Context)

	// ASR first time return char callback function (set in session)
	OnAsrFirstTextCallback func(text string, isFinal bool)
}

// IsSpeakerEnabled check if voiceprint recognition is enabled (read from global config)
func (c *ClientState) IsSpeakerEnabled() bool {
	// get enable field from global config (viper)
	enabled := viper.GetBool("voice_identify.enable")
	return enabled
}

// HasSpeakerGroups check if device config has voiceprint groups
func (c *ClientState) HasSpeakerGroups() bool {
	// check if device config has voiceprint group config
	return len(c.DeviceConfig.VoiceIdentify) > 0
}

func (c *ClientState) IsRealTime() bool {
	return c.ListenMode == "realtime"
}

func (c *ClientState) GetMemoryMode() string {
	return NormalizeMemoryMode(c.DeviceConfig.MemoryMode)
}

func (c *ClientState) GetSpeakerChatMode() string {
	return NormalizeSpeakerChatMode(c.DeviceConfig.SpeakerChatMode)
}

func (c *ClientState) RequireMatchedSpeakerForChat() bool {
	return c.HasSpeakerGroups() && c.GetSpeakerChatMode() == SpeakerChatModeIdentifiedOnly
}

func (c *ClientState) HasMatchedConfiguredSpeaker(result *speaker.IdentifyResult) bool {
	if result == nil || !result.Identified {
		return false
	}
	_, ok := c.DeviceConfig.VoiceIdentify[result.SpeakerName]
	return ok
}

func (c *ClientState) GetDeviceIDOrAgentID() string {
	if c.AgentID != "" {
		return c.AgentID
	}
	return c.DeviceID
}

// history message related methods start
func (c *ClientState) AddMessage(msg *schema.Message) {
	if msg == nil {
		log.Warnf("try to add nil message to conversation history")
		return
	}
	c.Dialogue.mu.Lock()
	defer c.Dialogue.mu.Unlock()
	c.Dialogue.Messages = append(c.Dialogue.Messages, msg)
}

func (c *ClientState) GetMessages(count int) []*schema.Message {
	c.Dialogue.mu.RLock()
	defer c.Dialogue.mu.RUnlock()

	// add boundary check, prevent array out of bounds
	if len(c.Dialogue.Messages) == 0 {
		return []*schema.Message{}
	}

	// calculate start index, ensure no out of bounds
	startIndex := len(c.Dialogue.Messages) - count
	if startIndex < 0 {
		startIndex = 0
	}

	return AlignToolMessages(c.Dialogue.Messages[startIndex:])
}

/*
func AlignMessage(messages []*schema.Message) []*schema.Message {
	findMsgTypeUser := false
	// ensure message completeness, traverse to find message after User
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if msg == nil {
			continue
		}
		if !findMsgTypeUser {
			if msg.Role == schema.User {
				return messages[i:]
			}
			continue
		}
	}
	return messages
}
*/
// AlignToolMessages ensures role:tool message's tool_call_id matches role:assistant message's tool_calls id
// if no matching then delete corresponding tool message, also process reverse non-matching scenario
func AlignToolMessages(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	// collect all assistant message's tool_calls id
	validToolCallIDs := make(map[string]bool)
	// collect all tool message's tool_call_id
	usedToolCallIDs := make(map[string]bool)

	// first pass: collect assistant message's tool_calls id and tool message's tool_call_id
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				if toolCall.ID != "" {
					validToolCallIDs[toolCall.ID] = true
				}
			}
		}

		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			usedToolCallIDs[msg.ToolCallID] = true
		}
	}

	// filter message, process dual non-matching situation
	var alignedMessages []*schema.Message
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		// if it's a tool message, check if tool_call_id is valid
		if msg.Role == schema.Tool {
			if msg.ToolCallID != "" && validToolCallIDs[msg.ToolCallID] {
				alignedMessages = append(alignedMessages, msg)
			}
		} else if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			// process assistant message, check if has unused tool_calls
			for _, toolCall := range msg.ToolCalls {
				if toolCall.ID != "" {
					if usedToolCallIDs[toolCall.ID] {
						alignedMessages = append(alignedMessages, msg)
					} else {
						continue
					}
				}
			}
		} else {
			// other type of message keep directly
			alignedMessages = append(alignedMessages, msg)
		}
	}

	return alignedMessages
}

func (c *ClientState) InitMessages(messages []*schema.Message) error {
	c.Dialogue.mu.Lock()
	defer c.Dialogue.mu.Unlock()
	c.Dialogue.Messages = AlignToolMessages(messages)
	return nil
}

// history message related methods end

func (c *ClientState) SetTtsStart(isStart bool) {
	c.IsTtsStart = isStart
}

func (c *ClientState) GetTtsStart() bool {
	return c.IsTtsStart
}

func (c *ClientState) GetMaxIdleDuration() int64 {
	if !viper.IsSet("chat.max_idle_duration") {
		return 30000
	}

	maxIdleDuration := viper.GetInt64("chat.max_idle_duration")
	if maxIdleDuration <= 0 {
		return math.MaxInt64
	}
	return maxIdleDuration
}

func (c *ClientState) GetPreAsrTextSilenceDuration() int64 {
	if viper.IsSet("chat.pre_asr_text_silence_duration") {
		preTextSilenceDuration := viper.GetInt64("chat.pre_asr_text_silence_duration")
		if preTextSilenceDuration <= 0 {
			return math.MaxInt64
		}
		return preTextSilenceDuration
	}

	base := c.VoiceStatus.SilenceThresholdTime
	if base <= 0 {
		base = 400
	}
	preTextSilenceDuration := base * 4
	if preTextSilenceDuration < 1000 {
		preTextSilenceDuration = 1000
	}
	return preTextSilenceDuration
}

func (c *ClientState) UpdateLastActiveTs() {
	c.MqttLastActiveTs = time.Now().Unix()
}

func (c *ClientState) IsActive() bool {
	diff := time.Now().Unix() - c.MqttLastActiveTs
	return c.MqttLastActiveTs > 0 && diff <= ClientActiveTs
}

func (c *ClientState) SetStatus(status string) {
	c.Status = status
}

func (c *ClientState) GetStatus() string {
	return c.Status
}

func (c *ClientState) SetListenPhase(phase string) {
	c.ListenPhase = phase
}

func (c *ClientState) GetListenPhase() string {
	return c.ListenPhase
}

type Ctx struct {
	sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *Ctx) Reset() {
	c.ResetWithReason("Ctx.Reset")
}

func (c *Ctx) ResetWithReason(reason string) {
	c.Lock()
	defer c.Unlock()
	if c.ctx != nil {
		c.cancel()
		c.ctx = nil
		c.cancel = nil
	}
}

func (c *Ctx) Get(parentCtx context.Context) context.Context {
	c.Lock()
	defer c.Unlock()
	if c.ctx == nil || c.ctx.Err() != nil {
		if c.ctx != nil {
			c.cancel()
		}
		c.ctx, c.cancel = context.WithCancel(parentCtx)
	}
	return c.ctx
}

func (c *Ctx) Cancel() {
	c.CancelWithReason("Ctx.Cancel")
}

func (c *Ctx) CancelWithReason(reason string) {
	c.Lock()
	defer c.Unlock()
	if c.ctx != nil {
		c.cancel()
		c.ctx = nil
		c.cancel = nil
	}
}

func (s *ClientState) getLLMProvider() (llm.LLMProvider, error) {
	llmConfig := s.DeviceConfig.Llm
	providerName := llmConfig.Provider
	if providerName == "" {
		providerName = "openai"
	}
	llmProvider, err := llm.GetLLMProvider(providerName, llmConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("create LLM provider failed: %v", err)
	}
	return llmProvider, nil
}

func (s *ClientState) InitLlm() error {
	ctx, cancel := context.WithCancel(s.Ctx)

	llmProvider, err := s.getLLMProvider()
	if err != nil {
		log.Errorf("create LLM provider failed: %v", err)
		return err
	}

	s.Llm = Llm{
		Ctx:         ctx,
		Cancel:      cancel,
		LLMProvider: llmProvider,
	}
	return nil
}

func (s *ClientState) InitAsr() error {
	asrConfig := s.DeviceConfig.Asr

	log.Infof("initialize asr, asrConfig: %+v", asrConfig)

	// initialize asr (no longer directly create AsrProvider, changed to use resource pool)
	ctx, cancel := context.WithCancel(s.Ctx)
	s.Asr = Asr{
		Ctx:             ctx,
		Cancel:          cancel,
		AsrAudioChannel: make(chan []float32, 100),
		AsrEnd:          make(chan bool, 1),
		AsrResult:       bytes.Buffer{},
		AsrType:         asrConfig.Provider,
		ClientState:     s, // set ClientState reference
	}

	// set ASR mode
	if mode, ok := asrConfig.Config["mode"].(string); ok {
		s.Asr.Mode = mode
	}

	if rawAutoEnd, ok := asrConfig.Config["auto_end"]; ok {
		if autoEnd, ok := rawAutoEnd.(bool); ok {
			s.Asr.AutoEnd = autoEnd
		}
	}
	return nil
}

func (c *ClientState) Destroy() {
	c.Asr.StopWithReason("ClientState.Destroy")
	c.Vad.Reset()

	// return ASR resource (if exists)
	// note: this needs to import pool package, but to avoid circular dependency, process at caller
	// or use type assert here, but need to import pool package
	// temporarily process resource return at caller (ChatSession.Close)

	c.VoiceStatus.Reset()
	c.AsrAudioBuffer.ClearAsrAudioData()

	c.SessionCtx.ResetWithReason("ClientState.Destroy: session_ctx")
	c.AfterAsrSessionCtx.ResetWithReason("ClientState.Destroy: after_asr_ctx")

	c.Statistic.Reset()
	c.SetStatus(ClientStatusInit)
	c.SetListenPhase(ListenPhaseIdle)
	c.SetTtsStart(false)
	c.IsWelcomePlaying = false
}

func (state *ClientState) OnManualStop() {
	state.OnVoiceSilence()
}

func (state *ClientState) OnVoiceSilence() {
	log.Debugf("OnVoiceSilence, voiceDuration: %d, voiceDurationInSession: %d", state.Vad.GetVoiceDuration(), state.Vad.GetVoiceDurationInSession())
	state.Asr.ResetReceivedText()
	state.SetClientVoiceStop(true) // set stop speak flag, this time received audio data will not enter vad
	// client-side stop speaking
	state.Asr.StopWithReason("ClientState.OnVoiceSilence") // stop asr and get result, perform llm
	// release vad
	state.Vad.Reset() // release vad instance

	state.SetStatus(ClientStatusListenStop)
	state.SetListenPhase(ListenPhaseIdle)

	// if set async get voiceprint result callback, then call
	if state.OnVoiceSilenceSpeakerCallback != nil {
		state.OnVoiceSilenceSpeakerCallback(state.Ctx)
	}

	state.SetStartAsrTs()
}

type Llm struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	// LLM provider
	LLMProvider llm.LLMProvider
	// asr to text received channel
	LLmRecvChannel chan llm_common.LLMResponseStruct
}

// ClientMessage indicates client-side message
type ClientMessage struct {
	Type        string          `json:"type"`
	DeviceID    string          `json:"device_id,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	Text        string          `json:"text,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	State       string          `json:"state,omitempty"`
	Token       string          `json:"token,omitempty"`
	DeviceMac   string          `json:"device_mac,omitempty"`
	Version     int             `json:"version,omitempty"`
	Transport   string          `json:"transport,omitempty"`
	Features    map[string]bool `json:"features,omitempty"`
	AudioParams *AudioFormat    `json:"audio_params,omitempty"`
	PayLoad     json.RawMessage `json:"payload,omitempty"`
}
