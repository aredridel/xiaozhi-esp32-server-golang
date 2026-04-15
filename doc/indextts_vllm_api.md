# IndexTTS vLLM Interface Integration Documentation

This document is used to explain the interface requirements for this project's access to `indextts_vllm`, applicable to:

- Main program TTS inference (`/audio/speech`)
- Administrator interface pulling voices (`/audio/voices`)
- User voice cloning (`/audio/clone`, used for this project's cloning process)

## 1. Quick Compatibility Checklist

Your IndexTTS service needs to meet at least the following three points:

- Provide `POST /audio/speech`, input parameters compatible with OpenAI TTS style: `input`, `voice`, `model`
- Provide `GET /audio/voices`, return enumerable voice list (JSON object)
- If using this project's "voice cloning" capability, provide `POST /audio/clone` (`multipart/form-data`)

Recommended return audio format: `audio/wav` (16-bit PCM).

## 2. Configuration Item Mapping (Administrator -> TTS Configuration -> IndexTTS(vLLM))

| Admin Field | Purpose | Send Location |
| --- | --- | --- |
| `api_url` | IndexTTS service address | As base URL, concatenate endpoints |
| `api_key` | Optional auth | `Authorization: Bearer <api_key>` |
| `model` | Model name | `/audio/speech` request body `model` |
| `voice` | Default voice | `/audio/speech` request body `voice` |
| `frame_duration` | Frame duration (ms) | Local audio frame segmentation parameter |

Description:

- When administrator interface clicks "Voice" dropdown, it will use the latest `api_url` in the current input box to pull `/audio/voices`.
- `api_url` supports filling in base address (e.g., `http://127.0.0.1:7860`), also compatible with filling in to specific path (e.g., `/audio/speech`).

## 3. Interface Requirements

### 3.1 `GET /audio/voices`

Purpose: Administrator configuration page "Voice" dropdown, user-side voice options.

Request headers:

- `Accept: application/json`
- `Authorization: Bearer <api_key>` (optional)

Return example (recommended):

```json
{
  "demo_speaker": ["assets/speaker/demo.wav"],
  "narrator_cn_female": ["assets/speaker/narrator_cn_female.wav"]
}
```

Requirements:

- Return type recommended to be JSON object (key names will be used as voice IDs).
- This project will filter out system voices prefixed with `indextts_vllm`, then append user cloned voices.

### 3.2 `POST /audio/speech`

Purpose: Main program TTS synthesis, post-cloning preview.

Request headers:

- `Content-Type: application/json`
- `Accept: audio/wav,application/octet-stream,*/*`
- `Authorization: Bearer <api_key>` (optional)

Request body example:

```json
{
  "model": "indextts-vllm",
  "input": "Hello, welcome to use IndexTTS.",
  "voice": "demo_speaker"
}
```

Return:

- Success: Binary audio stream (recommended `audio/wav`)
- Failure: HTTP 4xx/5xx, and return readable error message

### 3.3 `POST /audio/clone` (Required for this project's cloning feature)

Purpose: Called when `/user/voice-clones` submits cloning task.

Request type: `multipart/form-data`

Form fields:

- `voice`: Desired generated voice ID
- `audio`: Reference audio file (wav/mp3/m4a, etc.)

Return example:

```json
{
  "voice": "demo_speaker_clone_001",
  "ok": true
}
```

Requirements:

- Recommended to include `voice` field in response; if missing, this project will fall back to using the `voice` field value from the request.

## 4. Compatibility Reference (api_server.py)

Can refer to the following implementation style:

- `POST /audio/speech`: Read `input`, `voice`, `model`
- `GET /audio/voices`: Return available voice dictionary

Reference link:

- https://github.com/hackers365/index-tts-vllm/blob/master/api_server.py

## 5. FAQ Troubleshooting

### 5.1 Administrator clicks voice dropdown and reports error

Priority check:

- Whether `api_url` is reachable (latest input value)
- Whether `/audio/voices` returns JSON object
- Whether `api_key` is needed

### 5.2 Synthesis successful but playback abnormal

Priority check:

- Whether server returns standard WAV (PCM16, correct sample rate)
- Whether there is transcoding or truncation in intermediate link
- Whether response header `Content-Type` is correct

### 5.3 Cloning task failed

Priority check:

- Whether `/audio/clone` accepts `voice + audio` multipart request
- Whether response JSON is parseable, whether it contains available `voice`
