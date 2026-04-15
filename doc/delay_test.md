#### Latency Test Results

Can achieve reply within 1-1.3s, should be faster with smaller models

asr: funasr
llm: Alibaba Cloud API qwen2.5-72b-instruct
tts: cosyvoice 

```
time="2025-05-22 19:33:09.940" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1394 ms" caller="client.go:428"
time="2025-05-22 19:33:33.458" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1237 ms" caller="client.go:428"
time="2025-05-22 19:33:52.596" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1190 ms" caller="client.go:428"
time="2025-05-22 19:34:12.272" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1361 ms" caller="client.go:428"
time="2025-05-22 19:34:31.598" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1347 ms" caller="client.go:428"
time="2025-05-22 19:35:00.281" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1194 ms" caller="client.go:428"
time="2025-05-22 19:35:24.418" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 975 ms" caller="client.go:428"
time="2025-05-22 19:35:49.868" level=debug msg="From receiving audio end asr->llm->tts first frame Overall time: 1150 ms" caller="client.go:428"
```

---

## Management Backend Testing

One-click startup package and Docker deployment both have built-in Web management backend, providing visual testing interface.

Supported test types:

| Test Type | Description |
|---------|------|
| VAD | Voice activity detection connectivity and response time |
| ASR | Speech recognition connectivity and first packet latency |
| LLM | Large model inference connectivity and first packet latency |
| TTS | Speech synthesis connectivity and first packet latency |
| OTA | MQTT/UDP connectivity test |

Detailed usage please refer to: **[Management Console Guide →](manager_console_guide.md)**
