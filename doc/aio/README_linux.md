# Xiaozhi Service Linux Usage Instructions

Welcome to use Xiaozhi Service Linux AIO package. This document contains dependency installation, startup and configuration instructions.

## Directory Structure

```
xiaozhi_server-linux-amd64-<version>/
├── xiaozhi_server              # Main program
├── ten-vad/
│   └── lib/Linux/x64/
│       ├── libten_vad.so       # VAD dependency library
│       ├── libsherpa-onnx-c-api.so
│       ├── libsherpa-onnx-cxx-api.so
│       └── libonnxruntime.so   # ONNX Runtime dependency library
├── main_config.yaml            # Main configuration file
├── manager.json                # Management backend configuration
├── asr_server.json             # ASR service configuration
├── models/                     # Model files directory
├── data/                       # Data directory
└── logs/                       # Log directory
```

## Runtime Dependencies

### System Requirements

| System | Minimum Version | Test Status |
|------|----------|----------|
| Ubuntu | 18.04 LTS | Tested |
| Debian | 10 (Buster) | Expected compatible, not tested |
| CentOS / RHEL | 8 | Expected compatible, not tested |

**Runtime Requirements**:
- **Architecture**: x86_64 (amd64)

### Install Dependencies

#### Debian / Ubuntu

```bash
sudo apt update
sudo apt install -y libc++1 libc++abi1
```

#### CentOS / RHEL / Fedora

```bash
sudo dnf install -y libcxx libcxxabi
# Or
sudo yum install -y libcxx libcxxabi
```

#### Other Distributions

Please install corresponding packages for the following libraries:
- `libc++.so.1` — LLVM C++ Standard Library
- `libc++abi.so.1` — LLVM C++ ABI

## Quick Start

```bash
# Add execution permission
chmod +x xiaozhi_server

# Start service
./xiaozhi_server
```

### Background Run

Use nohup:

```bash
nohup ./xiaozhi_server > logs/output.log 2>&1 &
```

Or use systemd (recommended for production environment), see below.

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
- **LAN Access**: `http://<Server IP>:8080/`

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

## Production Environment Deployment (systemd)

Create service file `/etc/systemd/system/xiaozhi.service`:

```ini
[Unit]
Description=Xiaozhi Server
After=network.target

[Service]
Type=simple
User=YOUR_USER
WorkingDirectory=/path/to/xiaozhi_server-linux-amd64
ExecStart=/path/to/xiaozhi_server-linux-amd64/xiaozhi_server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

Start service:

```bash
# Reload configuration
sudo systemctl daemon-reload

# Enable auto-start on boot
sudo systemctl enable xiaozhi

# Start service
sudo systemctl start xiaozhi

# View status
sudo systemctl status xiaozhi

# View logs
sudo journalctl -u xiaozhi -f
```

## Firewall Configuration

If server has firewall enabled, need to open corresponding ports:

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 8080/tcp  # Management backend
sudo ufw allow 8989/tcp  # WebSocket
sudo ufw allow 2883/tcp  # MQTT
sudo ufw allow 8990/udp  # UDP

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=8989/tcp
sudo firewall-cmd --permanent --add-port=2883/tcp
sudo firewall-cmd --permanent --add-port=8990/udp
sudo firewall-cmd --reload
```

## FAQ

### Prompt for Missing Shared Library

Use `ldd` command to check missing libraries:

```bash
ldd xiaozhi_server
ldd ten-vad/lib/Linux/x64/libten_vad.so
```

Install corresponding system packages according to output.

### glibc Version Too Low

If `version 'GLIBC_2.xx' not found` appears, it means system glibc version is too old. Suggest:
- Upgrade system to newer version
- Or run using Docker container

### Port Occupied

```bash
# View port occupancy
sudo lsof -i :port_number
# Or
sudo netstat -tulpn | grep port_number

# Modify port number in configuration file or end occupying process
```
