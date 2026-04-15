# 🚀 xiaozhi-esp32-server-golang

> **Xiaozhi AI Backend for ESP32 Devices**

---

## Project Overview

xiaozhi-esp32-server-golang is a high-performance, fully streaming AI backend service designed for IoT and intelligent voice scenarios. Built with Go, it integrates core capabilities including ASR (Automatic Speech Recognition), LLM (Large Language Model), and TTS (Text-to-Speech), supporting large-scale concurrency and multi-protocol access to enable AI voice interaction for smart terminals and edge devices.

---

## ✨ Key Features

- ⚡ **End-to-End Streaming AI Voice Pipeline**: Full streaming processing from ASR → LLM → TTS for low-latency real-time interaction
- 🎙️ **Voiceprint Recognition & Dynamic TTS Switching**: Automatically switch TTS voices based on speaker identity for personalized voice experience
- 🔌 **Transport Interface Abstraction**: Unified abstraction for WebSocket / MQTT UDP, flexible injection into main logic for easy protocol extension
- 📬 **Message Queue Processing**: Asynchronous message queue processing for LLM and TTS, supporting flexible business logic injection
- 🌐 **Multi-Protocol High-Concurrency Access**: Supports large-scale device concurrent access and message push
- ♻️ **Efficient Resource Pool & Connection Reuse**: External resource connection pool mechanism to reduce response time and improve system throughput
- 🤖 **Multi-Engine AI Capability Integration**: Based on the Eino framework, supports FunASR, OpenAI-compatible, Ollama, Doubao, EdgeTTS, CosyVoice, and more
- 🧩 **Modular Extensible Architecture**: VAD/ASR/LLM/TTS/MCP/Vision and other core modules are independently pluggable
- 🎵 **MCP Audio Server**: Paginated audio resource retrieval and streaming processing, music playback and volume control
- 🦞 **OpenClaw Agent Integration**: Generate dedicated OpenClaw Endpoints per agent, supporting connection status viewing, session testing, and enter/exit keyword routing (default: "open lobster/enter lobster" and "close lobster/exit lobster")
- 🖥️ **Full-Featured Web Management Console**: Visual configuration wizard, VAD/ASR/LLM/TTS availability and latency testing, device management and message injection, real-time latency monitoring and OTA verification
- 🧠 **Advanced Business Features**: MCP market aggregation and import, voice cloning, knowledge base (Dify/RAGFlow/WeKnora), device/agent-level MCP remote call debugging
- 📦 **Easy One-Click Deployment**: Pre-compiled aio packages ready to use out-of-the-box (main program + console + voiceprint service), Docker one-click deployment, supports Linux/Windows/macOS local compilation
- 🔐 **Security & Permission System** (planned): Reserved user authentication and permission management interfaces

---

[deepwiki Architecture Analysis](https://deepwiki.com/hackers365/xiaozhi-esp32-server-golang)

## 🚀 Quick Start

### Method 1: One-Click Bundle (Recommended)

Download the compressed package for your platform, extract and run:

- **Release Page**: <https://github.com/hackers365/xiaozhi-esp32-server-golang/releases>
- **Tutorial**: [doc/quickstart_bundle_tutorial.md](doc/quickstart_bundle_tutorial.md)

After starting, visit **http://<server-ip-or-domain>:8080** to enter the Web Console for configuration.

### Method 2: Docker Deployment

- [Docker Compose (with console)](doc/docker_compose.md)
- [Docker (without console)](doc/docker.md)

### Method 3: Local Compilation

For development environments or custom compilation scenarios.

**Install Dependencies** (Ubuntu example)

```bash
# Go 1.20+
# Opus codec
sudo apt-get install -y pkg-config libopus0 libopusfile-dev

# ONNX Runtime (1.21.0)
wget https://github.com/microsoft/onnxruntime/releases/download/v1.21.0/onnxruntime-linux-x64-1.21.0.tgz
tar -xzf onnxruntime-linux-x64-1.21.0.tgz
sudo cp -r onnxruntime-linux-x64-1.21.0/include/* /usr/local/include/onnxruntime/
sudo cp -r onnxruntime-linux-x64-1.21.0/lib/* /usr/local/lib/
sudo ldconfig

# ten_vad runtime dependencies
sudo apt install -y libc++1 libc++abi1
```

> 📖 For complete dependency instructions and Windows/macOS configuration, see [config.md](doc/config.md)

For separate compilation of main program, console frontend/backend, and voiceprint service, and AIO packaging process, see [doc/compile_deploy.md](doc/compile_deploy.md)

Refer to [FunASR Official Documentation](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md) for deployment.

**Compile and Start**

```bash
# Compile
go build -o xiaozhi_server ./cmd/server/

# Start (see config/config.yaml for configuration)
./xiaozhi_server -c config/config.yaml
```

---

## 📚 Documentation

### Deployment
- [One-Click Bundle Tutorial](doc/quickstart_bundle_tutorial.md)
- [Docker Compose Deployment](doc/docker_compose.md)
- [Docker Deployment](doc/docker.md)
- [Compilation and Deployment Guide](doc/compile_deploy.md)
- [Configuration Details](doc/config.md)

### User Guide
- [Management Console Guide](doc/manager_console_guide.md)
- [WebSocket Service and OTA Configuration](doc/websocket_server.md)
- [MQTT + UDP Configuration](doc/mqtt_udp.md)
- [MQTT UDP Protocol](doc/mqtt_udp_protocol.md)

### Feature Modules
- [Vision Capabilities](doc/vision.md)
- [Voiceprint Recognition](doc/speaker_identification.md)
- [MCP Architecture](doc/mcp.md)
- [MCP Audio Resources](doc/mcp_resource.md)
- [MCP Market (Market Discovery/Import/Hot Update)](doc/mcp_market.md)
- [OpenClaw Agent Integration (Endpoint/Keyword Routing/Session Testing)](doc/openclaw_integration.md)
- [Voice Cloning (User Operations and Admin Quota)](doc/voice_clone.md)
- [Knowledge Base (Provider Configuration/Sync/Recall Testing/RAG)](doc/knowledge_base.md)
- [Device/Agent-Level MCP Remote Call (Endpoint/Tools/Call)](doc/mcp_remote_call_agent_device.md)

### Device Access
- [ESP32 Access Guide](doc/esp32_xiaozhi_backend_guide.md)
- [OTA MQTT Authorization](doc/ota_mqtt_auth.md)

---

## 🧩 Module Architecture

| Module | Function | Tech Stack |
|------|----------|--------|
| VAD | Voice Activity Detection | Silero VAD / WebRTC VAD / ten_vad |
| ASR | Speech Recognition | FunASR / Doubao ASR |
| LLM | Large Model Inference | Eino framework compatible, OpenAI, Ollama, etc. |
| TTS | Text-to-Speech | Doubao / EdgeTTS / CosyVoice |
| MCP | Multi-protocol access, MCP market discovery and import, device/agent-level remote call debugging | MCP Server / Endpoint / MCP Market / SSE / StreamableHTTP / WebSocket Controller / MCP Tool Call |
| OpenClaw | Agent-level endpoints, enter/exit keyword mode switching, session message forwarding and testing | OpenClaw WebSocket / Agent Endpoint / Chat Router |
| Vision | Vision Processing | Doubao / Alibaba Cloud Vision |
| Voiceprint Recognition | Speaker Recognition | sherpa-onnx + Vector Database |
| Voice Cloning | User-side voice cloning creation and preview | Minimax / CosyVoice / Qwen |
| Knowledge Base (RAG) | Document sync, recall testing and dialogue retrieval | Dify / RAGFlow / WeKnora |

---

## 📈 Performance & Testing

- [Latency Test Report](doc/delay_test.md)
- Management console provides VAD/ASR/LLM/TTS availability and latency testing entry points

---

## 🛠️ Roadmap

- Long connection establishment with devices
- Proactive AI

---

## 🤝 Contributing

Welcome to submit Issues, PRs, or suggestions!

---

## 📄 License

MIT License

---

## 📬 Contact

**Discussion Group** (QR code expires, please contact author)

![Group QR Code](https://github.com/user-attachments/assets/c1c1c4ab-2567-4a6b-92a2-c8fcde7a5dcb)

**WeChat**: hackers365

![WeChat](https://github.com/user-attachments/assets/6b8d3d11-7bf5-4fa4-a73e-5109019dab85)

---

> © 2024 xiaozhi-esp32-server-golang
