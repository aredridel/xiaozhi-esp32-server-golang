Test Xiaozhi official server MQTT+UDP protocol response speed
Results:
stt 166ms, llm ~300ms, first audio frame 642ms

## Usage

### Basic Parameters
- `-ota`: OTA server address (default: https://api.tenclass.net/xiaozhi/ota/)
- `-device`: Device ID (default: ba:8f:17:de:94:94)
- `-mode`: Audio pickup mode, supports `manual` or `auto`, default: `manual`

### Audio Pickup Mode Description
- **manual mode**: Need to manually send listen stop message to stop audio pickup
- **auto mode**: Automatically detect voice end and stop audio pickup

### Usage Example
```bash
# Use default manual mode
./main -device "ba:8f:17:de:94:94"

# Use auto mode
./main -device "ba:8f:17:de:94:94" -mode auto
```