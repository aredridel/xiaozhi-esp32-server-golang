# Standalone Mock ASR/LLM/TTS Service (No Main Program Changes)

This solution provides a **standalone** mock service process for replacing real ASR/LLM/TTS cloud services during load testing.

## 1. Startup

```bash
go run ./cmd/mock_ai_server \
  -addr :18080 \
  -asr-text "Hello, this is load test mock recognition result" \
  -llm-reply "This is mock llm reply" \
  -tts-mode silence
```

Health check:

```bash
curl http://127.0.0.1:18080/healthz
```

## 2. Exposed Interfaces

- `ws://127.0.0.1:18080/asr/`
  - Compatible with FunASR style ws input (receives audio binary frames)
  - Returns final recognition result after receiving `{"is_speaking": false}`

- `POST http://127.0.0.1:18080/v1/chat/completions`
  - OpenAI Chat Completions compatible interface
  - Supports `stream=false/true`

- `POST http://127.0.0.1:18080/v1/audio/speech`
  - OpenAI TTS compatible interface
  - Returns `audio/wav` (silence or beep)

## 3. Main Program Configuration Suggestions (Configuration Only, No Code Changes)

### ASR (FunASR)

- `host=127.0.0.1`
- `port=18080`
- Protocol path uses `ws://host:port/` as per current implementation. If your configuration layer requires a path, please use `/asr/`.

> If your current ASR adapter strongly depends on `ws://host:port/` root path, you can also forward `/` to `/asr/` at the gateway layer.

### LLM (OpenAI Compatible)

- provider select `eino` (`type=openai`)
- `base_url=http://127.0.0.1:18080/v1`
- `api_key` any non-empty value
- `model_name` any value (e.g., `mock-gpt`)

### TTS (OpenAI Compatible)

- provider select `openai`
- `api_url=http://127.0.0.1:18080/v1/audio/speech`
- `response_format=wav`
- `api_key` any non-empty value

## 4. Adjustable Parameters

```bash
-asr-delay-ms         # ASR final return delay
-llm-first-delay-ms   # LLM first token delay
-llm-chunk-delay-ms   # LLM streaming chunk interval delay
-tts-first-delay-ms   # TTS first packet delay
-tts-mode             # silence|beep
-tts-duration-ms      # Returned audio duration
```

## 5. Load Testing Recommendations

1. First verify single connection locally (ensure device can go through complete chain and receive audio).
2. Then use `ws_multi` for concurrent step testing (e.g., 50/100/200/500).
3. Use different delay combinations to simulate real external dependency fluctuations, observe P95/P99 and error rates.


## 6. ws_multi Optimization Assessment (Evaluation)

Conclusion: **Recommend minor optimization, not mandatory refactoring**. Currently can be directly used for load testing, but to more accurately measure "main service performance" rather than "load test client bottleneck", suggest adding the following capabilities:

1. **Add pure audio playback mode (recommended priority)**
   - Current common practice is to do local TTS first then push audio, which mixes client TTS time into results.
   - Suggest adding `-audio_file`/`-audio_dir`, directly send pre-encoded opus or wav-to-opus frames.

2. **Structured latency statistics output**
   - Add first frame RT, full chain completion RT, error code classification statistics.
   - Suggest outputting JSONL for easy post-processing aggregation of P95/P99.

3. **Connection and send throttling control**
   - Add batch connection establishment (e.g., start N clients per second), avoid instantaneous connection amplification causing client-side jitter.
   - Add packet sending jitter parameters to simulate real device networks.

4. **Configurable failure retry and timeout strategy**
   - Such as `-dial_timeout`, `-read_timeout`, `-retry`, improve long load test stability.

5. **Resource metrics collection (optional)**
   - Record client-side CPU/memory, facilitate distinguishing "server bottleneck" from "load test machine bottleneck".

Under this "standalone mock service" solution, `ws_multi` **can run without changes**, but suggest at least doing items 1 and 2, load test conclusions will be significantly more credible.
