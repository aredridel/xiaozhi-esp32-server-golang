# Management Console Guide

## Access Management Console

- Address: http://<Server IP or Domain>:8080

---

## 1. Configuration Wizard

Automatically enters configuration wizard after first login, total of 5 steps.

### Step 1: OTA Configuration

Configure OTA server information, used to configure websocket and mqtt addresses issued to Xiaozhi hardware.

<!-- Screenshot position: OTA configuration interface -->
> Figure: OTA configuration wizard interface

| Configuration Item | Description |
|-------|------|
| MQTT Broker | MQTT server address |
| MQTT Port | MQTT port (default 1883) |
| UDP Port | UDP port |
| ... | ... |

**Test Connectivity**: Click "Test Current Configuration" to verify MQTT/UDP connection.

---

### Step 2: VAD Configuration

Select voice activity detection engine:

<!-- Screenshot position: VAD configuration interface -->
> Figure: VAD configuration wizard interface

| Engine | Description | Recommended Scenario |
|-----|------|---------|
| Silero VAD | High precision | Production environment |
| WebRTC VAD | Lightweight | Resource constrained |
| ten_vad | Local C++ version | High performance requirements |

---

### Step 3: ASR Configuration

Select speech recognition engine:

<!-- Screenshot position: ASR configuration interface -->
> Figure: ASR configuration wizard interface

| Engine | Description |
|-----|------|
| FunASR | Local recognition, requires model download |
| Doubao ASR | Cloud API |

---

### Step 4: LLM Configuration

Select large language model:

<!-- Screenshot position: LLM configuration interface -->
> Figure: LLM configuration wizard interface

| Engine | Description |
|-----|------|
| OpenAI compatible | Supports various APIs |
| Ollama | Local deployment |
| Doubao | ByteDance Doubao |

---

### Step 5: TTS Configuration

Select text-to-speech engine:

<!-- Screenshot position: TTS configuration interface -->
> Figure: TTS configuration wizard interface

| Engine | Description |
|-----|------|
| Doubao TTS | Cloud API |
| EdgeTTS | Microsoft free TTS |
| CosyVoice | Local high quality |

---

## 2. Configuration Testing

### Test Single Configuration

On each configuration page, click the "Test" button on the right side of the configuration item:

<!-- Screenshot position: Single configuration test button -->
> Figure: Configuration test button

Test result description:

| Field | Description |
|-----|------|
| Status | Success/Failure |
| First Packet Latency | Millisecond-level response time |
| Message | Error details (if failed) |

<!-- Screenshot position: Test result popup -->
> Figure: Configuration test result popup

### Batch Testing

On the configuration management page, click "Test All" to batch test all configurations:

<!-- Screenshot position: Batch test interface -->
> Figure: Batch test interface

### Supported Test Types

| Test Type | Description |
|---------|------|
| VAD | Voice activity detection connectivity and response time |
| ASR | Speech recognition connectivity and first packet latency |
| LLM | Large model inference connectivity and first packet latency |
| TTS | Speech synthesis connectivity and first packet latency |
| OTA | MQTT/UDP connectivity test |

---

## 3. Latency Monitoring

View first packet latency statistics for each system module:

<!-- Screenshot position: Latency monitoring interface -->
> Figure: Latency monitoring interface

### Latency Optimization Suggestions

| Module | Optimization Direction |
|-----|---------|
| ASR | Use local model or nearby API node |
| LLM | Select smaller model or use streaming output |
| TTS | Use edge TTS or local model |

---

## 4. Configuration Management

### Edit Configuration

Go to "Configuration Management" → Corresponding module → Edit configuration item

<!-- Screenshot position: Configuration management interface -->
> Figure: Configuration management interface

### Enable/Disable Configuration

Control whether configuration takes effect through toggle switch.

### Set Default Configuration

Each module can set one default configuration, used when device does not specify.

---

## FAQ

### Q1: Configuration test failed?

1. Check network connection
2. Verify if API key is correct
3. View main program console logs

### Q2: How to restore default configuration?

Delete configuration files in `config/` directory, restart service.

### Q3: Need to restart after configuration modification?

Most configuration modifications take effect in real-time, some module configurations may require restarting device connection.
