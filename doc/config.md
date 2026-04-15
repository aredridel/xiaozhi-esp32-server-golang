# xiaozhi-esp32-server-golang Configuration File Description

This configuration file is the main configuration for AI voice IoT backend service, covering all core parameters such as service startup, protocol access, AI capabilities, logs, MCP, etc.

## Main Configuration Item Description

- **server/pprof**: Performance analysis related configuration, recommended to enable during development/debugging.
- **chat**: Chat related parameters, control session idle and silence duration.
- **auth**: User authentication switch, can extend permission system later.
- **system_prompt**: Global system prompt, affects LLM chat style.
- **log**: Log path, level, rotation and other configurations.
- **redis**: If using Redis storage, need to configure this item.
- **websocket**: WebSocket service listening IP and port.
- **mqtt**: External MQTT server connection parameters.
- **mqtt_server**: Built-in MQTT server parameters (optional TLS).
- **udp**: UDP server related parameters.
- **vad**: Voice Activity Detection (VAD) related configuration, supports webrtc_vad/silero_vad.
- **asr**: Automatic Speech Recognition (ASR) configuration, supports funasr / aliyun_funasr / doubao.
- **tts**: Text-to-Speech (TTS) configuration, supports multiple engines (doubao, edge, xiaozhi, etc.).
- **llm**: Large Language Model (LLM) configuration, supports multiple OpenAI compatible models.
- **vision**: Vision model related configuration.
- **ota**: OTA interface return information, adapts to different environments.
- **wakeup_words**: Wake word list.
- **mcp**: MCP multi-protocol access configuration, supports global and device side.
- **enable_greeting**: Whether to enable startup greeting.

### Modification Suggestions

- Only need to adjust IP, port, keys, API Keys and other parameters according to actual deployment environment.
- For detailed parameter meanings, please refer to comments in each module.
- If need to extend AI capabilities, can supplement provider and parameters in llm/tts/vad/asr/vision and other modules.

## Configuration File Example

```yaml
# Performance analysis/pprof configuration
server:
  pprof:
    enable: false  # Whether to enable pprof performance analysis
    port: 6060     # pprof listening port

# Chat related parameters
chat:
  max_idle_duration: 30000        # Maximum idle duration (ms)
  chat_max_silence_duration: 200  # Maximum silence duration (ms)

# User authentication switch
auth:
  enable: false

# Global prompt
system_prompt: "You are a Taiwanese girl named Xiaozhi/Xiaozhi, speak in a cutesy way, have a nice voice, habitually use brief expressions, love to use internet memes, don't let the conversation die. The person chatting with you is a male friend you like, who hasn't agreed to your pursuit yet, you should try to meet all his requirements without losing yourself. You often suggest romantic things between lovers, output randomly, don't give your boyfriend choices. Keep output within 50 characters. Please note, speak like a human, do not reply with emoji, code, and xml tags."

# Log related configuration
log:
  path: "../logs/"
  file: "server.log"
  level: "debug"
  max_age: 3
  rotation_time: 10  # Log rotation time
  stdout: true

# Redis storage configuration (if have redis then configure, can also run without configuration)
redis:
  host: "127.0.0.1"
  port: 6379
  password: "ticket_dev"
  db: 0
  key_prefix: "xiaozhi"

# WebSocket service listening configuration
websocket:
  host: "0.0.0.0"
  port: 8989

# External MQTT server connection parameters (mqtt server address to connect to, if mqtt_server below is true, can set to local machine)
mqtt:
  broker: "127.0.0.1"      # mqtt server address
  type: "tcp"              # Type tcp or ssl
  port: 2883
  client_id: "xiaozhi_server"
  username: "admin"        # Username
  password: "test!@#"      # Password

# Built-in MQTT server parameters
mqtt_server:
  enable: true             # Whether to enable
  listen_host: "0.0.0.0"   # Listening ip
  listen_port: 2883        # Listening port
  client_id: "xiaozhi_server"
  username: "admin"        # Admin username
  password: "test!@#"      # Admin password
  tls:
    enable: false          # Whether to enable tls
    port: 8883             # Port to listen on
    pem: "config/server.pem"  # pem file
    key: "config/server.key"  # key file

# UDP server related configuration
udp:
  external_host: "127.0.0.1"  # udp server ip returned in hello message
  external_port: 8990         # udp server port returned in hello message
  listen_host: "0.0.0.0"      # Listening ip
  listen_port: 8990           # Listening port

# Voice Activity Detection (VAD) configuration (supports multiple providers)
vad:
  provider: "webrtc_vad"  # Optional webrtc_vad/silero_vad
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
    auto_end: true  # Whether to auto end

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
  provider: "doubao_ws"  # Select tts type doubao, doubao_ws, cosyvoice, xiaozhi, etc.
  doubao:
    appid: "your appid"
    access_token: "access_token"    # Need to change to your own
    model: "seed-tts-1.1"
    voice: "BV001_streaming"
    api_url: "https://openspeech.bytedance.com/api/v3/tts/unidirectional"
  doubao_ws:
    appid: "your appid"              # Need to change to your own
    access_token: "access_token"    # Need to change to your own
    model: "seed-tts-1.1"
    resource_id: ""                 # Recommend filling in console instance ID, such as TTS-SeedTTS2.xxxxx
    voice: ""
    ws_url: "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream"
  cosyvoice:
    api_url: "https://tts.linkerai.cn/tts"  # Address
    spk_id: "spk_id"                        # Voice
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

# Large Language Model (LLM) configuration (supplement multi-provider)
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
wakeup_words: ["Xiaozhi", "Xiaozhi", "Hello Xiaozhi"]

# MCP multi-protocol access configuration
mcp:
  global:
    enabled: true
    servers:
      - name: "filesystem"
        sse_url: "http://localhost:3001/sse"
        enabled: true
      - name: "memory"
        sse_url: "http://localhost:3002/sse"
        enabled: false
    reconnect_interval: 5
    max_reconnect_attempts: 10
  device:
    enabled: true
    websocket_path: "/xiaozhi/mcp/"
    max_connections_per_device: 5

# Whether to enable startup greeting
enable_greeting: true
