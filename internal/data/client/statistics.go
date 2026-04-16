package client

import "time"

// Statistic structure body already deprecated, please use statistic_plugin at MetricTtsStop when get count info
type Statistic struct {
	TurnStartTs     int64
	AsrFirstTextTs  int64
	AsrFinalTextTs  int64
	LlmStartTs      int64
	LlmFirstTokenTs int64
	LlmEndTs        int64
	TtsStartTs      int64
	TtsFirstFrameTs int64
	TtsStopTs       int64
}

// MarkTurnStart record turn start time
func (state *ClientState) MarkTurnStart() {
	state.Statistic.TurnStartTs = time.Now().UnixMilli()
}

// MarkAsrFirstText record ASR first times return text time
func (state *ClientState) MarkAsrFirstText() {
	if state.Statistic.AsrFirstTextTs == 0 {
		state.Statistic.AsrFirstTextTs = time.Now().UnixMilli()
	}
}

// MarkAsrFinalText record ASR final text time
func (state *ClientState) MarkAsrFinalText() {
	if state.Statistic.AsrFinalTextTs == 0 {
		state.Statistic.AsrFinalTextTs = time.Now().UnixMilli()
	}
}

// MarkLlmStart record LLM start time
func (state *ClientState) MarkLlmStart() {
	state.Statistic.LlmStartTs = time.Now().UnixMilli()
	state.Statistic.LlmFirstTokenTs = 0
	state.Statistic.LlmEndTs = 0
}

// MarkLlmFirstToken record LLM first times return token time
func (state *ClientState) MarkLlmFirstToken() {
	state.Statistic.LlmFirstTokenTs = time.Now().UnixMilli()
}

// MarkLlmEnd record LLM end time
func (state *ClientState) MarkLlmEnd() {
	state.Statistic.LlmEndTs = time.Now().UnixMilli()
}

// MarkTtsStart record TTS start time
func (state *ClientState) MarkTtsStart() {
	state.Statistic.TtsStartTs = time.Now().UnixMilli()
	state.Statistic.TtsFirstFrameTs = 0
	state.Statistic.TtsStopTs = 0
}

// MarkTtsFirstFrame record TTS first frame time
func (state *ClientState) MarkTtsFirstFrame() {
	if state.Statistic.TtsFirstFrameTs == 0 {
		state.Statistic.TtsFirstFrameTs = time.Now().UnixMilli()
	}
}

// MarkTtsStop record TTS end time
func (state *ClientState) MarkTtsStop() {
	state.Statistic.TtsStopTs = time.Now().UnixMilli()
}

// SetStartAsrTs set ASR start time (alias name, for compatibility)
func (state *ClientState) SetStartAsrTs() { state.MarkTurnStart() }

// SetStartLlmTs set LLM start time (alias name, for compatibility)
func (state *ClientState) SetStartLlmTs() { state.MarkLlmStart() }

// SetStartTtsTs set TTS start time (alias name, for compatibility)
func (state *ClientState) SetStartTtsTs() { state.MarkTtsStart() }

// GetAsrDuration get ASR process time consumption (already deprecated, only keep method signature)
func (state *ClientState) GetAsrDuration() int64 {
	return 0
}

// GetAsrLlmTtsDuration get body body time consumption (already deprecated, only keep method signature)
func (state *ClientState) GetAsrLlmTtsDuration() int64 {
	return 0
}

// GetLlmDuration get LLM time consumption (already deprecated, only keep method signature)
func (state *ClientState) GetLlmDuration() int64 {
	return 0
}

// GetTtsDuration get TTS time consumption (already deprecated, only keep method signature)
func (state *ClientState) GetTtsDuration() int64 {
	return 0
}

func (s *Statistic) Reset() {}
