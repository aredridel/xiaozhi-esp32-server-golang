package client

import "time"

// Statistic structurebodyalready废弃，pleaseuse statistic_plugin at MetricTtsStop whengetcountinfo
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

// MarkTurnStart record轮timesstarttime
func (state *ClientState) MarkTurnStart() {
	state.Statistic.TurnStartTs = time.Now().UnixMilli()
}

// MarkAsrFirstText record ASR firsttimesreturntexttime
func (state *ClientState) MarkAsrFirstText() {
	if state.Statistic.AsrFirstTextTs == 0 {
		state.Statistic.AsrFirstTextTs = time.Now().UnixMilli()
	}
}

// MarkAsrFinalText record ASR finallytexttime
func (state *ClientState) MarkAsrFinalText() {
	if state.Statistic.AsrFinalTextTs == 0 {
		state.Statistic.AsrFinalTextTs = time.Now().UnixMilli()
	}
}

// MarkLlmStart recordLLM starttime
func (state *ClientState) MarkLlmStart() {
	state.Statistic.LlmStartTs = time.Now().UnixMilli()
	state.Statistic.LlmFirstTokenTs = 0
	state.Statistic.LlmEndTs = 0
}

// MarkLlmFirstToken recordLLM firsttimesreturn token time
func (state *ClientState) MarkLlmFirstToken() {
	state.Statistic.LlmFirstTokenTs = time.Now().UnixMilli()
}

// MarkLlmEnd recordLLM endtime
func (state *ClientState) MarkLlmEnd() {
	state.Statistic.LlmEndTs = time.Now().UnixMilli()
}

// MarkTtsStart record TTS starttime
func (state *ClientState) MarkTtsStart() {
	state.Statistic.TtsStartTs = time.Now().UnixMilli()
	state.Statistic.TtsFirstFrameTs = 0
	state.Statistic.TtsStopTs = 0
}

// MarkTtsFirstFrame record TTS firstframetime
func (state *ClientState) MarkTtsFirstFrame() {
	if state.Statistic.TtsFirstFrameTs == 0 {
		state.Statistic.TtsFirstFrameTs = time.Now().UnixMilli()
	}
}

// MarkTtsStop record TTS endtime
func (state *ClientState) MarkTtsStop() {
	state.Statistic.TtsStopTs = time.Now().UnixMilli()
}

// SetStartAsrTs set ASR starttime（别name，is兼容）
func (state *ClientState) SetStartAsrTs() { state.MarkTurnStart() }

// SetStartLlmTs setLLM starttime（别name，is兼容）
func (state *ClientState) SetStartLlmTs() { state.MarkLlmStart() }

// SetStartTtsTs set TTS starttime（别name，is兼容）
func (state *ClientState) SetStartTtsTs() { state.MarkTtsStart() }

// GetAsrDuration get ASR processtime consumption（already废弃，only保留methodsign）
func (state *ClientState) GetAsrDuration() int64 {
	return 0
}

// GetAsrLlmTtsDuration getbodybodytime consumption（already废弃，only保留methodsign）
func (state *ClientState) GetAsrLlmTtsDuration() int64 {
	return 0
}

// GetLlmDuration getLLM time consumption（already废弃，only保留methodsign）
func (state *ClientState) GetLlmDuration() int64 {
	return 0
}

// GetTtsDuration get TTS time consumption（already废弃，only保留methodsign）
func (state *ClientState) GetTtsDuration() int64 {
	return 0
}

func (s *Statistic) Reset() {}
