# Quick Start Bundle Deployment Tutorial

## Download

Visit the [Release page](https://github.com/hackers365/xiaozhi-esp32-server-golang/releases) to download for your platform:

| Platform | Filename |
|-----|-------|
| Windows | `xiaozhi-server-windows-xxx.zip` |
| Linux | `xiaozhi-server-linux-xxx.tar.gz` |
| macOS | `xiaozhi-server-macos-xxx.tar.gz` |

---

## Extraction and Directory Structure

Directory structure after extraction:

```
xiaozhi-aio/
├── xiaozhi_server          # Main program
├── config/                 # Configuration files directory
├── models/                 # Model files directory (if using local ASR/TTS)
└── data/                   # Data directory
```

---

## Start Service

### Windows
Double-click `start.bat`

### Linux
```bash
# ten_vad runtime dependencies
sudo apt install -y libc++1 libc++abi1

chmod +x xiaozhi_server
LD_LIBRARY_PATH="$PWD/ten-vad/lib/Linux/x64:${LD_LIBRARY_PATH:-}" ./xiaozhi_server
```

### macOS
```bash
chmod +x xiaozhi_server
./build/macos/fix_rpath.sh ./xiaozhi_server
./xiaozhi_server
```

If the directory structure is maintained as:

```text
./xiaozhi_server
./ten-vad/lib/macOS/ten_vad.framework
```

Then the macOS package, after executing `fix_rpath.sh`, generally does not require manually setting `DYLD_FRAMEWORK_PATH`.

If you are debugging from an IDE temporary directory, or manually moved the binary causing the relative directory structure to be broken, you can use the fallback method:

```bash
DYLD_FRAMEWORK_PATH="$PWD/ten-vad/lib/macOS" ./xiaozhi_server
```

If you are building the macOS distribution package yourself from the source repository, you need to additionally execute before release:

```bash
./build/macos/fix_rpath.sh ./xiaozhi_server
```

This step will change the `rpath` in the binary from the development machine source path to `@executable_path/ten-vad/lib/macOS`, allowing the release package to run directly when the directory structure is correct.

---

## Next Steps

### 1. Access Web Console

Browser access: **http://<Server IP or Domain>:8080**

<!-- Screenshot position: Login interface -->
> Figure: Web console login interface

### 2. Configure Services

For first-time use, please follow the configuration wizard to complete setup. For details, see:

**[Management Console Guide →](manager_console_guide.md)**

---

## Speaker Identification Service (Optional)

The program has integrated speaker identification service

---

## FAQ

### Q1: Cannot access Web console after startup?

Check firewall settings to ensure port 8080 is accessible.

### Q2: How to restart the service?

Close the program and run it again. Configuration files are saved in the `config/` directory.

### Q3: How to view logs?

Console outputs real-time logs. To save logs, you can redirect:

```bash
./xiaozhi_server > server.log 2>&1
```
