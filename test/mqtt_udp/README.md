Testing Xiaozhi official server MQTT+UDP protocol response speed
Results:
STT 166ms, LLM ~300ms, first audio frame 642ms

## Usage

### Basic Parameters
- `-ota`: OTA server URL (default: https://api.tenclass.net/xiaozhi/ota/)
- `-device`: Device ID (default: ba:8f:17:de:94:94)
- `-mode`: Listening mode, supports `manual` or `auto`, default: `manual`
- `-tts_provider`: TTS provider, supports `cosyvoice`, `edge`, `edge_offline`, `indextts_vllm`, default: `cosyvoice`

### Listening Mode Details
- **manual mode**: Requires manually sending a listen stop message to stop listening
- **auto mode**: Automatically detects end of speech and stops listening
- The current test program supports MQTT-delivered `speak_request`; the device side will respond with `speak_ready` per protocol (i.e., active broadcast response message)

### Examples
```bash
# Use default manual mode
./main -device "ba:8f:17:de:94:94"

# Use auto mode
./main -device "ba:8f:17:de:94:94" -mode auto

# Use edge_offline for local offline TTS
./main -device "ba:8f:17:de:94:94" -tts_provider edge_offline

# Use indextts_vllm
./main -device "ba:8f:17:de:94:94" -tts_provider indextts_vllm
```
