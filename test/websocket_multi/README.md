# websocket_multi Load Test Client Documentation

## New Optimization Parameters

- `-audio_wav`: Use local wav files (comma-separated) as audio input, avoiding cloud TTS for test audio generation.
- `-metrics_jsonl`: Output structured latency metrics (JSONL), including `first_frame`, `tts_stop` events.
- `-ramp_ms`: Client startup interval in milliseconds, reducing instantaneous connection jitter.

## Example

```bash
go run ./test/websocket_multi/xiaozhi_ws_client_multi.go \
  -server ws://127.0.0.1:8989/xiaozhi/v1/ \
  -count 100 \
  -audio_wav ./test/websocket_client/test.wav \
  -ramp_ms 20 \
  -metrics_jsonl ./ws_metrics.jsonl
```

If `-audio_wav` is not provided, original behavior will be used (generate audio via cosyvoice).

---

## Concurrent Load Test Template (Recommended)

> First ensure you have started:
> 1) Main service;
> 2) Standalone mock service (e.g., `go run ./cmd/mock_ai_server -addr :18080`);
> 3) Main service configuration points to mock ASR/LLM/TTS.

### 1) 50 Concurrent

```bash
go run ./test/websocket_multi/xiaozhi_ws_client_multi.go \
  -server ws://127.0.0.1:8989/xiaozhi/v1/ \
  -count 50 \
  -audio_wav ./test/websocket_client/test.wav \
  -ramp_ms 30 \
  -metrics_jsonl ./metrics_50.jsonl
```

### 2) 100 Concurrent

```bash
go run ./test/websocket_multi/xiaozhi_ws_client_multi.go \
  -server ws://127.0.0.1:8989/xiaozhi/v1/ \
  -count 100 \
  -audio_wav ./test/websocket_client/test.wav \
  -ramp_ms 20 \
  -metrics_jsonl ./metrics_100.jsonl
```

### 3) 300 Concurrent

```bash
go run ./test/websocket_multi/xiaozhi_ws_client_multi.go \
  -server ws://127.0.0.1:8989/xiaozhi/v1/ \
  -count 300 \
  -audio_wav ./test/websocket_client/test.wav \
  -ramp_ms 10 \
  -metrics_jsonl ./metrics_300.jsonl
```

### 4) 500 Concurrent

```bash
go run ./test/websocket_multi/xiaozhi_ws_client_multi.go \
  -server ws://127.0.0.1:8989/xiaozhi/v1/ \
  -count 500 \
  -audio_wav ./test/websocket_client/test.wav \
  -ramp_ms 5 \
  -metrics_jsonl ./metrics_500.jsonl
```

---

## Metrics Summary Script

New: `test/websocket_multi/summarize_metrics.py`

```bash
python3 ./test/websocket_multi/summarize_metrics.py ./metrics_100.jsonl
```

Output includes:
- `first_frame`: avg/p50/p95/p99/max
- `tts_stop`: avg/p50/p95/p99/max
- `approx_success_rate(first_frame/tts_stop)`

