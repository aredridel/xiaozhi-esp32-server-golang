# VAD/ASR/LLM/TTS Full Chain Load Test Mock Solution (Pending Confirmation)

> Goal: Without calling real ASR/LLM/TTS paid services, retain existing WebSocket full chain behavior, support high concurrency load testing, controllable latency injection, and observability statistics.

## 1. Design Goals

1. **Complete Chain**: Retain the "device audio input -> VAD -> ASR -> LLM -> TTS -> audio delivery" main flow.
2. **Zero External Cost**: ASR/LLM/TTS all return local mock data, do not access third-party cloud services.
3. **Low Intrusion**: Based on existing provider factory mechanism, extend `mock` provider, try not to change business main flow.
4. **Testable and Reproducible**: Support fixed return, template return, error injection by probability, latency injection by configuration.
5. **Comparable to Real Services**: Through configuration switching, can restore real provider at any time for performance comparison.

## 2. Overall Solution

Adopt **Provider-level Mock + Load Test Client Reuse** solution:

- Add three providers:
  - `asr/mock`
  - `llm/mock`
  - `tts/mock`
- Add corresponding configuration items in backend configuration (`type=asr|llm|tts`, `provider=mock`).
- Bind mock configuration through role/agent to achieve full chain mock within session.
- Load test side continues to use existing websocket load test tool (`ws_multi`) to concurrently push audio.

This ensures:
- WebSocket protocol, session state machine, message orchestration logic all go through real code paths.
- Only replace calls to external cloud services, lowest cost, minimum risk.

## 3. Mock Behavior Design

### 3.1 ASR Mock

Input: Audio frame stream (keep existing interface).
Output: Recognition text (fixed/round-robin/by rule).

Recommended configuration:

- `mode`: `fixed` | `sequence` | `echo_hint`
- `fixed_text`: Fixed return, e.g., "Hello, this is load test text"
- `sequence_texts`: Text array, round-robin by request
- `first_token_delay_ms`: First packet latency simulation
- `final_delay_ms`: End packet latency simulation
- `error_rate`: 0~1 probability injection recognition failure

### 3.2 LLM Mock

Input: ASR text + context messages.
Output: Reply text (can carry context length information).

Recommended configuration:

- `mode`: `fixed` | `template` | `echo`
- `fixed_answer`: Fixed reply
- `template`: Template, e.g., `"Received: {{input}}"`
- `first_token_delay_ms`: First token latency
- `stream_chunk_chars`: Streaming chunk character count
- `total_delay_ms`: Completion total time simulation
- `error_rate`: Probability failure

### 3.3 TTS Mock

Input: LLM text.
Output: Playable Opus/PCM frames (recommend prioritizing Opus, compatible with current chain).

Recommended configuration:

- `audio_source`: `builtin_silence` | `builtin_beep` | `file`
- `file_path`: Preset audio path (local wav/opus)
- `frame_duration_ms`: Frame length (e.g., 20ms)
- `first_frame_delay_ms`: First frame latency
- `inter_frame_delay_ms`: Inter-frame latency
- `error_rate`: Probability failure

> To reduce complexity, first version recommendation: first return "silence frame + fixed latency", then add "beep/file playback" later.

## 4. Load Test Scenario Matrix

### Scenario A: Pure Success Chain (Baseline)
- ASR fixed text
- LLM fixed short reply
- TTS silence frame
- Goal: Test maximum stable concurrency, average RT, P95/P99

### Scenario B: High Latency Chain
- ASR/LLM/TTS inject 100~500ms latency respectively
- Goal: Test timeout threshold, queuing accumulation

### Scenario C: Error Injection Chain
- error_rate set to 1%/5%/10%
- Goal: Test error recovery, connection stability, retry strategy

### Scenario D: Long Text Chain
- LLM output ultra-long text (e.g., 500~1500 characters)
- Goal: Test TTS framing, send backpressure and memory stability

## 5. Metrics and Acceptance Criteria (Recommended)

Core metrics:
- Session success rate (successfully returned voice)
- End-to-end first frame latency (listen stop -> first audio packet)
- End-to-end completion latency (listen stop -> tts finish)
- Active sessions per second / peak concurrency
- Error rate (by ASR/LLM/TTS stage)
- Service resources: CPU, memory, Goroutine, GC count

Recommended acceptance (can be adjusted later):
- Success rate >= 99%
- P95 first frame latency < 1.5s at target concurrency
- No obvious memory leak for 30min (RSS change controllable)

## 6. Implementation Steps (Two Phases)

### Phase 1 (Minimum Viable, 1~2 days)
1. Add ASR/LLM/TTS three mock provider registrations.
2. Each provider supports fixed return + fixed latency + error rate.
3. Backend adds three mock configurations and can be set as default.
4. Run through `ws_multi` and output baseline load test results.

### Phase 2 (Enhancement, 1~2 days)
1. Add template reply, sequence reply, file audio playback.
2. Add more granular metric logs (stage-by-stage time consumption).
3. Add load test scripts (batch scenario execution + summary report).

## 7. Risks and Mitigation

1. **Audio format mismatch**: Mock tts output format needs to be consistent with current downstream decoding.
   - Mitigation: First version uses existing common encoding path and adds format validation logs.
2. **Excessive logs under high concurrency**: Detailed logs under high concurrency will affect performance.
   - Mitigation: Downgrade log level in load test mode, aggregate output key metrics.
3. **Accidental switch to real service**: Still calling external interface.
   - Mitigation: Disable network in load test environment or add provider whitelist verification (non-mock refuses to start).

## 8. Implementation Content After Your Confirmation

After confirmation, I will directly modify code according to the following list:

1. Add `internal/domain/asr/mock`, `internal/domain/llm/mock`, `internal/domain/tts/mock`.
2. Mount `mock` provider at provider factory / pool registration point.
3. Supplement default configuration examples (can directly select mock in management backend).
4. Add minimum unit tests (at least provider behavior tests).
5. Provide a load test execution command list (concurrency step + metric collection).

---

## Options for Your Confirmation

Please confirm the following 4 points, then I will start formal modification:

1. **Mock Granularity**: Agree to mock by provider level (recommended)?
2. **TTS Output**: Accept "silence frame" as mock audio for first version (fastest)?
3. **Load Test Target Concurrency**: What concurrency to target first (e.g., 100/300/500)?
4. **Acceptance Threshold**: Execute according to default acceptance criteria in this document?
