# Xiaozhi Service Windows Usage Instructions

Welcome to use Xiaozhi Service Windows AIO package. This document contains startup, configuration and port descriptions.

## Directory Structure

```
xiaozhi_server-windows-amd64-<version>/
├── xiaozhi_server.exe          # Main program
├── onnxruntime.dll             # ONNX Runtime dependency library
├── sherpa-onnx-c-api.dll       # Sherpa-ONNX dependency library
├── sherpa-onnx-cxx-api.dll     # Sherpa-ONNX C++ dependency library
├── ten_vad.dll                 # VAD dependency library
├── start.bat                   # Startup script
├── main_config.yaml            # Main configuration file
├── manager.json                # Management backend configuration
├── asr_server.json             # ASR service configuration
├── models/                     # Model files directory
├── data/                       # Data directory
└── logs/                       # Log directory
```

## Quick Start

Double-click `start.bat` to start the service. After startup, you can view logs in the `logs/` directory.

> Tip: On first startup, the program will automatically download required model files (if models directory is empty).

## Ports and Services

| Port | Configuration Source | Description |
|------|----------|------|
| **8080** | `manager.json` → `server.port` | **Management Backend**: Web console + HTTP API |
| **8989** | `main_config.yaml` → `websocket.port` | **Main Service WebSocket**: Device/Client connection |
| **9000** | `asr_server.json` → `server.port` | **ASR/Speaker ID Service**: Speech recognition internal interface |
| **2883** | Console configuration | **MQTT Service**: Device MQTT connection |
| **8990** | Console configuration | **UDP Service**: Device UDP communication |
| **6060** | Console configuration | **pprof**: Performance analysis (default off) |

## Access Addresses

### Management Backend

- **Local Access**: `http://localhost:8080/`
- **LAN Access**: `http://<Local IP>:8080/`

### Device/Client Connection

- **WebSocket**: `ws://<Server IP>:8989/`
- **MQTT**: `<Server IP>:2883`
- **UDP**: `<Server IP>:8990`

## Modify Configuration

### Ports to Modify in Configuration Files

The following ports take effect after service restart:

| Port | Configuration File | Configuration Item |
|------|----------|--------|
| 8080 | `manager.json` | `server.port` |
| 8989 | `main_config.yaml` | `websocket.port` |
| 9000 | `asr_server.json` | `server.port` |

### Console Configuration

The following ports and all other configurations are changed through the management backend console:

- **Port Configuration**: MQTT (2883), UDP (8990), pprof (6060)
- **Function Configuration**: LLM, TTS, ASR, Speaker Identification, etc.
- Access `http://localhost:8080/` to enter management backend
- Configuration changes take effect in real-time, no service restart needed

## FAQ

### Firewall Prompt

On first run, Windows may pop up a firewall prompt, please allow the program to access the network.

### Port Occupied

If startup fails with port occupied prompt, please:

1. Use `netstat -ano | findstr :port_number` to view occupying process
2. Modify port number in configuration file
3. Or end the process occupying that port

### DLL Missing

If prompted for missing DLL files, please ensure the following files are in the same directory as `xiaozhi_server.exe`:
- `onnxruntime.dll`
- `sherpa-onnx-c-api.dll`
- `sherpa-onnx-cxx-api.dll`
- `ten_vad.dll`

## Stop Service

Press `Ctrl + C` in the startup window or directly close the window to stop the service.
