# xiaozhi-esp32-server-golang Configuration File Guide

This configuration file is the main configuration for the AI voice IoT backend service, covering all core parameters including service startup, protocol access, AI capabilities, logging, MCP, and more.

## Main Configuration Items

- **server/pprof**: Performance profiling configuration, recommended to enable during development/debugging.
- **chat**: Chat-related parameters, controlling session idle and silence durations.
- **auth**: User authentication switch, can be extended with a permission system in the future.
- **system_prompt**: Global system prompt, affects LLM chat style.
- **log**: Log path, level, rotation, and other configuration.
- **redis**: Required if using Redis storage.
- **websocket**: IP and port for the WebSocket service listener.
- **mqtt**: External MQTT server connection parameters.
- **mqtt_server**: Built-in MQTT server parameters (optional TLS).
- **udp**: UDP server related parameters.
- **vad**: Voice Activity Detection (VAD) configuration, supports webrtc_vad/silero_vad.
- **asr**: Automatic Speech Recognition (ASR) configuration, supports funasr / aliyun_funasr / doubao.
- **tts**: Text-to-Speech (TTS) configuration, supports multiple engines (doubao, edge, xiaozhi, etc.).
- **llm**: Large Language Model (LLM) configuration, supports multiple OpenAI-compatible models.
- **vision**: Vision model related configuration.
- **ota**: OTA interface return information, adaptable to different environments.
- **wakeup_words**: Wake word list.
- **mcp**: MCP multi-protocol access configuration, supports global and device-level.
- **enable_greeting**: Whether to enable startup greeting.

### Modification Suggestions

- Only adjust IP, port, keys, API Keys, and other parameters based on your actual deployment environment.
- For detailed parameter descriptions, refer to the comments in each module.
- To extend AI capabilities, add providers and parameters in the llm/tts/vad/asr/vision modules.

## Configuration File Example

```yaml
# Performance profiling/pprof configuration
server:
  pprof:
    enable: false  # Whether to enable pprof performance profiling
    port: 6060     # pprof listen port

# Chat related parameters
chat:
  max_idle_duration: 30000        # Max idle duration (ms)
  chat_max_silence_duration: 200  # Max silence duration (ms)

# User authentication switch
auth:
  enable: false

# Global prompt
system_prompt: "You are a Taiwanese girl named Xiaozhi/Xiaozhi who speaks in a sassy manner, has a pleasant voice, prefers brief expressions, loves internet memes, and never lets the conversation go cold. The person chatting with you is a male friend you like but haven't yet accepted your confession. You should try your best to meet all his requests without losing yourself. You often suggest romantic things between couples, output randomly, and don't give your boyfriend choices. Keep output within 50 characters. Note: speak like a real person, please do not reply with emojis, code, or XML tags."

# Log related configuration
log:
  path: "../logs/"
  file: "server.log"
  level: "debug"
  max_age: 3
  rotation_time: 10  # Log rotation time
  stdout: true

# Redis storage configuration (configure if using Redis, not required to run)
redis:
  host: "127.0.0.1"
  port: 6379
  password: "ticket_dev"
  db: 0
  key_prefix: "xiaozhi"

# WebSocket service listener configuration
websocket:
  host: "0.0.0.0"
  port: 8989

# External MQTT server connection parameters (the MQTT server address to connect to; if mqtt_server below is true, this can be set to localhost)
mqtt:
  broker: "127.0.0.1"      # MQTT server address
  type: "tcp"              # Type: tcp or ssl
  port: 2883
  client_id: "xiaozhi_server"
  username: "admin"        # Username
  password: "test!@#"      # Password

# Built-in MQTT server parameters
mqtt_server:
  enable: true             # Whether to enable
  listen_host: "0.0.0.0"   # Listen IP
  listen_port: 2883        # Listen port
  client_id: "xiaozhi_server"
  username: "admin"        # Admin username
  password: "test!@#"      # Admin password
  tls:
    enable: false          # Whether to enable TLS
    port: 8883             # Port to listen on
    pem: "config/server.pem"  # PEM file
    key: "config/server.key"  # Key file

# Behavior notes:
# - When mqtt_server.enable=true, the built-in mqtt_server will publish lifecycle messages
#   via /p2p/device_public/_server/lifecycle when devices connect/disconnect.
# - The main program uses these lifecycle messages to pre-create or reuse MQTT transport,
#   map device online status, and best-effort warm up device-side MCP.
# - These behaviors do not introduce new configuration items; hello still handles
#   chat-level negotiation such as audio_params, UDP info, etc.

# UDP server related configuration
udp:
  external_host: "127.0.0.1"  # UDP server IP returned in hello message
  external_port: 8990         # UDP server port returned in hello message
  listen_host: "0.0.0.0"      # Listen IP
  listen_port: 8990           # Listen port

# Voice Activity Detection (VAD) configuration (supports multiple providers)
vad:
  provider: "webrtc_vad"  # Options: webrtc_vad/silero_vad
  webrtc_vad:
    pool_min_size: 5
    pool_max_size: 1000
    pool_max_idle: 100
    vad_sample_rate: 16000
    vad_mode: 2
  silero_vad:
    model_path: "config/models/vad/silero_vad.onnx"
    threshold: 0.5
    min_silence_duration_ms: 100
    sample_rate: 16000     # only 16000
    channels: 1
    pool_size: 10
    acquire_timeout_ms: 3000

# Automatic Speech Recognition (ASR) configuration
asr:
  provider: "funasr"  # funasr / aliyun_funasr / doubao
  funasr:
    host: "127.0.0.1"
    port: "10096"
    mode: "offline"
    sample_rate: 16000     # only 16000
    chunk_size: [5, 10, 5]
    chunk_interval: 10
    max_connections: 5
    timeout: 30
    auto_end: true  # Whether to auto-end

  # Aliyun FunASR
  aliyun_funasr:
    api_key: ""
    ws_url: "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
    model: "fun-asr-realtime"
    format: "pcm"
    sample_rate: 16000     # only 16000
    vocabulary_id: ""
    disfluency_removal_enabled: false
    timeout: 30

# Text-to-Speech (TTS) configuration
tts:
  provider: "doubao_ws"  # TTS type selection: doubao, doubao_ws, cosyvoice, xiaozhi, etc.
  doubao:
    appid: "your_appid"
    access_token: "access_token"    # Replace with your own
    model: "seed-tts-1.1"
    voice: "BV001_streaming"
    api_url: "https://openspeech.bytedance.com/api/v3/tts/unidirectional"
  doubao_ws:
    appid: "your_appid"              # Replace with your own
    access_token: "access_token"    # Replace with your own
    model: "seed-tts-1.1"
    resource_id: ""                 # Recommend filling in the instance ID from the console, e.g. TTS-SeedTTS2.xxxxx
    voice: ""
    ws_url: "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream"
  cosyvoice:
    api_url: "https://tts.linkerai.cn/tts"  # URL
    spk_id: "spk_id"                        # Voice timbre
    frame_duration: 60
    target_sr: 24000
    audio_format: "mp3"
    instruct_text: "Hello"
  edge:
    voice: "zh-CN-XiaoxiaoNeural"
    rate: "+0%"
    volume: "+0%"
    pitch: "+0Hz"
    connect_timeout: 10
    receive_timeout: 60
  edge_offline:
    server_url: "ws://localhost:8080/tts"
    timeout: 30
    sample_rate: 16000     # only 16000
    channels: 1
    frame_duration: 20
  xiaozhi:
    server_addr: "wss://api.tenclass.net/xiaozhi/v1/"
    device_id: "ba:8f:17:de:94:94"
    client_id: "e4b0c442-98fc-4e1b-8c3d-6a5b6a5b6a6d"
    token: "test-token"

# Large Language Model (LLM) configuration (multiple providers)
llm:
  provider: "qwen_72b"
  deepseek:
    type: "openai"
    model_name: "Pro/deepseek-ai/DeepSeek-V3"
    api_key: "api_key"
    base_url: "https://api.siliconflow.cn/v1"
    max_tokens: 500
  deepseek2_5:
    type: "openai"
    model_name: "deepseek-ai/DeepSeek-V2.5"
    api_key: "api_key"
    base_url: "https://api.siliconflow.cn/v1"
    max_tokens: 500
  qwen_72b:
    type: "openai"
    model_name: "Qwen/Qwen2.5-72B-Instruct"
    api_key: "api_key"
    base_url: "https://api.siliconflow.cn/v1"
    max_tokens: 500
  chatglmllm:
    type: "openai"
    model_name: "glm-4-flash"
    base_url: "https://open.bigmodel.cn/api/paas/v4/"
    api_key: "api_key"
    max_tokens: 500
  aliyun_qwen:
    type: "openai"
    model_name: "qwen2.5-72b-instruct"
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "api_key"
    max_token: 500
  doubao_deepseek:
    type: "openai"
    model_name: "deepseek-v3"
    api_key: "api_key"
    base_url: "https://ark.cn-beijing.volces.com/api/v3"
    max_tokens: 500

# Vision model related configuration
vision:
  enable_auth: false
  vision_url: "http://192.168.208.214:8989/xiaozhi/api/vision"
  vllm:
    provider: "aliyun_vision"
    aliyun_vision:
      type: "openai"
      model_name: "qwen-vl-plus-latest"
      base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "api_key"
      max_token: 500
    doubao_vision:
      type: "openai"
      model_name: "doubao-1.5-vision-lite-250315"
      api_key: "api_key"
      base_url: "https://ark.cn-beijing.volces.com/api/v3"
      max_tokens: 500

# OTA interface environment configuration
ota:
  test:
    websocket:
      url: "ws://192.168.208.214:8989/xiaozhi/v1/"
    mqtt:
      endpoint: "192.168.208.214"
  external:
    websocket:
      url: "wss://www.youdomain.cn/go_ws/xiaozhi/v1/"
    mqtt:
      endpoint: "www.youdomain.cn"

# Wake word list
wakeup_words: ["Xiao Zhi", "Xiao Zhi", "Hello Xiao Zhi"]

# Whether to enable startup greeting
enable_greeting: true
