# Running Environment

#### 1. Deploy funasr

See [FunASR Docker deployment documentation](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md)

#### 2. Clone Code
git clone 'https://github.com/hackers365/xiaozhi-esp32-server-golang'

#### 3. Configure config/config.yaml, see [config configuration description](config.md) for details

Main modification items are as follows:
```yaml
# 1. ASR speech recognition
asr:
  provider: "funasr"
  funasr:
    host: "127.0.0.1"      # IP of deployed FunASR WebSocket service
    port: "10096"          # Port of deployed FunASR WebSocket
    mode: "offline"        # Mode, use offline
    # ...

# 2. TTS
tts:
  provider: "xiaozhi"      # Type of TTS to use, recommend doubao_ws, or choose free edge
  doubao_ws:
    appid: "6886011847"                         # Your appid
    access_token: "access_token"                # Your access token
    cluster: "volcano_tts"
    voice: "zh_female_wanwanxiaohe_moon_bigtts" # Voice, default is Wanwan Xiaohe
    ws_host: "openspeech.bytedance.com"
    use_stream: true
  edge:
    voice: "zh-CN-XiaoxiaoNeural"
    rate: "+0%"
    volume: "+0%"
    pitch: "+0Hz"
    connect_timeout: 10
    receive_timeout: 60
  # ....

# 3. LLM large model
llm:
  provider: "deepseek"                        # Provider, corresponds to key below
  deepseek:
    type: "openai"                            # Type of server interface compatibility
    model_name: "Pro/deepseek-ai/DeepSeek-V3" # Model name
    api_key: "api_key"                        # API key
    base_url: "https://api.siliconflow.cn/v1" # Service interface, default SiliconFlow
    max_tokens: 500
  # ...

```

#### 4. Start docker
Start Docker in project root directory and mount config directory and ports (http/websocket:8989, other ports map as needed)

```
docker run -itd --name xiaozhi_server -v $(pwd)/config:/workspace/config -p 8989:8989 hackers365/xiaozhi_server:latest

If domestic connection is not available, use the following source

docker run -itd --name xiaozhi_server -v $(pwd)/config:/workspace/config -p 8989:8989 docker.jsdelivr.fyi/hackers365/xiaozhi_server:latest
```

**ten_vad support description:**
- Docker image has automatically included ten_vad library files, no additional mount needed
- If using ten_vad as VAD provider, just set vad.provider: "ten_vad" in configuration file

Should be able to connect now
ws://machine_ip:8989/xiaozhi/v1/ 

for chatting


# Development Environment
```
docker run -itd --name xiaozhi_server_golang -v $(pwd):/workspace/ -p 8989:8989 hackers365/xiaozhi_golang:0.1
If domestic connection is not available, use the following source
docker run -itd --name xiaozhi_server_golang -v $(pwd):/workspace/ -p 8989:8989 docker.jsdelivr.fyi/hackers365/xiaozhi_golang:0.1

go build -o xiaozhi_server cmd/server/*.go
```

**Development environment ten_vad description:**
- Development environment image has included ten_vad compilation and runtime dependencies
- If you need to use ten_vad in development environment, ensure lib/ten-vad directory exists in project root directory
- Will automatically use ten_vad header files and library files during compilation
