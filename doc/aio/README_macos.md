# Xiaozhi Service macOS Usage Instructions

Welcome to use Xiaozhi Service macOS AIO package. This document contains dependency installation, startup and configuration instructions.

## Directory Structure

```
xiaozhi_server-macos-<arch>-<version>/
├── xiaozhi_server              # Main program
├── ten-vad/
│   └── lib/macOS/
│       ├── ten_vad.framework/  # VAD framework
│       ├── libonnxruntime.*.dylib
│       └── libsherpa-onnx-*.dylib
├── main_config.yaml            # Main configuration file
├── manager.json                # Management backend configuration
├── asr_server.json             # ASR service configuration
├── models/                     # Model files directory
├── data/                       # Data directory
└── logs/                       # Log directory
```

> **Note**: macOS version is divided into **amd64** (Intel) and **arm64** (Apple Silicon), please download the version matching your Mac.

## Runtime Dependencies

### System Requirements

- **macOS Version**: macOS 11 (Big Sur) or higher
- **Architecture**: Intel (x86_64) or Apple Silicon (arm64)

### Install Dependencies

Use Homebrew to install necessary dependencies:

```bash
# Install Homebrew (if not already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install pkg-config
```

## Quick Start

```bash
# Add execution permission
chmod +x xiaozhi_server

# If this is a release package you built yourself, first fix rpath
./build/macos/fix_rpath.sh ./xiaozhi_server

# Start service
./xiaozhi_server
```

Description:

- Official release packages generally do not need to execute `fix_rpath.sh` again if packaging is complete
- Only need to add this step when building macOS distribution package yourself from source repository
- This step will change the development machine absolute path `rpath` in the binary to `@executable_path/ten-vad/lib/macOS`

### First Run Security Prompt

On first run, macOS may pop up a security prompt because the program is not Apple certified. Please:

1. Open "System Settings" → "Privacy & Security"
2. Find the prompt about `xiaozhi_server`
3. Click "Open Anyway" or "Allow"

Or use the following command to remove quarantine:

```bash
xattr -cr xiaozhi_server
```

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

## Background Run

### Use nohup

```bash
nohup ./xiaozhi_server > logs/output.log 2>&1 &
```

### Create launchd Service (Recommended)

Create `~/Library/LaunchAgents/com.xiaozhi.server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.xiaozhi.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/xiaozhi_server</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/xiaozhi_server-macos-<arch>-<version></string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/path/to/logs/output.log</string>
    <key>StandardErrorPath</key>
    <string>/path/to/logs/error.log</string>
</dict>
</plist>
```

Load service:

```bash
# Load service
launchctl load ~/Library/LaunchAgents/com.xiaozhi.server.plist

# Start service
launchctl start com.xiaozhi.server

# View status
launchctl list | grep xiaozhi

# Stop service
launchctl stop com.xiaozhi.server

# Unload service
launchctl unload ~/Library/LaunchAgents/com.xiaozhi.server.plist
```

## Firewall Configuration

If firewall is enabled, need to allow `xiaozhi_server` to accept incoming connections:

1. Open "System Settings" → "Network" → "Firewall"
2. Click "Options"
3. Find `xiaozhi_server`, set to "Allow incoming connections"

Or use commands in terminal:

```bash
# Add firewall exception (requires sudo)
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /path/to/xiaozhi_server
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblock /path/to/xiaozhi_server
```

## FAQ

### Security Prompt "Damaged"

If prompted that the application is damaged, run the following command:

```bash
xattr -cr xiaozhi_server
```

### Dynamic Library Loading Failure

If `dylib` loading failure occurs, check:

```bash
# View dependencies
otool -L xiaozhi_server

# View rpath
otool -l xiaozhi_server | grep -A2 LC_RPATH

# Ensure dynamic libraries are in correct location
ls -la ten-vad/lib/macOS/
```

If `LC_RPATH` is still the development machine source code absolute path, instead of `@executable_path/ten-vad/lib/macOS`, please execute:

```bash
./build/macos/fix_rpath.sh ./xiaozhi_server
```

If you are debugging from an IDE temporary directory, or manually moved the binary causing directory structure inconsistency, you can temporarily use:

```bash
DYLD_FRAMEWORK_PATH="$PWD/ten-vad/lib/macOS" ./xiaozhi_server
```

### Port Occupied

```bash
# View port occupancy
lsof -i :port_number

# End occupying process or modify port in configuration file
```

### Apple Silicon (M1/M2/M3) Running Intel Version

Running Intel version on Apple Silicon Mac requires Rosetta 2:

```bash
# Install Rosetta 2
softwareupdate --install-rosetta
```

But it is recommended to download the corresponding arm64 version for best performance.
