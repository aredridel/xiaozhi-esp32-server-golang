# TEN-VAD Test Program

This is a test program for testing and validating TEN-VAD (Voice Activity Detection) functionality.

## Features

- Read WAV audio files
- Convert WAV to PCM format (float32)
- Perform voice activity detection on each frame using TEN-VAD
- Output detailed detection results and statistics

## Usage

### Basic Usage

```bash
# Use default parameters (hop_size=512, threshold=0.3)
go run main.go audio_utils.go <wav_file_path>

# Example
go run main.go audio_utils.go test.wav
```

### Custom Parameters

```bash
# Specify hop_size and threshold
go run main.go audio_utils.go <wav_file_path> <hop_size> <threshold>

# Example: use hop_size=256, threshold=0.5
go run main.go audio_utils.go test.wav 256 0.5
```

### Compile and Run

```bash
# Compile
go build -o ten_vad_test main.go audio_utils.go

# Run
./ten_vad_test test.wav
./ten_vad_test test.wav 512 0.3
```

## Parameter Description

- **wav_file_path** (required): Path to the WAV audio file to test
- **hop_size** (optional, default 512): Frame shift size, frame size used by TEN-VAD when processing audio
- **threshold** (optional, default 0.3): VAD detection threshold, range 0.0-1.0, higher values are stricter

## Output Information

The program outputs the following information:

1. **File Information**: WAV file size, format information
2. **Conversion Information**: Number of PCM data frames
3. **Detection Process**: Detection results for each frame (whether voice is present)
4. **Statistics**:
   - Total frames
   - Voice frames
   - Voice activity ratio (%)
   - Final conclusion

## Example Output

```
Successfully read WAV file: test.wav (123456 bytes)
WAV format: {SampleRate:16000 NumChannels:1 BitDepth:16}
Starting conversion...
Successfully converted to PCM data, 100 frames total
TEN-VAD created successfully (hop_size=512, threshold=0.30), starting test...
Starting voice activity detection...
Frame 1: No voice activity
Frame 2: Voice activity detected
...
Frame 100: No voice activity

=== TEN-VAD Detection Results Statistics ===
Total frames: 100
Voice frames: 45
Voice activity ratio: 45.00%
Conclusion: Voice activity detected
```

## WAV File Format Requirements

### Recommended Format (Best Compatibility)
- **Sample Rate**: 16000 Hz (recommended) or 8000/32000/48000 Hz
- **Channels**: Mono (1 channel)
- **Bit Depth**: 16 bit
- **Encoding**: PCM (uncompressed)
- **File Format**: Standard WAV file (.wav)

### Supported Formats
The program uses the `go-audio/wav` library and supports most standard WAV formats:
- Different sample rates (the program will auto-process, but 16000Hz is recommended)
- Mono or multi-channel (multi-channel will be auto-processed)
- Different bit depths (8/16/24/32 bit)

### Format Conversion Examples

If your audio file is not in the recommended format, you can use the following tools to convert:

**Using ffmpeg:**
```bash
# Convert to 16000Hz, mono, 16bit PCM WAV
ffmpeg -i input.wav -ar 16000 -ac 1 -sample_fmt s16 output.wav

# Convert from MP3
ffmpeg -i input.mp3 -ar 16000 -ac 1 -sample_fmt s16 output.wav

# Convert from other formats
ffmpeg -i input.m4a -ar 16000 -ac 1 -sample_fmt s16 output.wav
```

**Using sox:**
```bash
sox input.wav -r 16000 -c 1 -b 16 output.wav
```

## Notes

1. **Library Requirements**: Ensure the `lib/ten-vad` directory contains the correct dynamic library files:
   - Windows: `lib/ten-vad/lib/Windows/x64/ten_vad.dll` and `ten_vad.lib`
   - Linux: `lib/ten-vad/lib/Linux/x64/libten_vad.so`

2. **Audio Format Requirements**:
   - Sample rate: 16000Hz recommended (the program will auto-convert, but standard format is recommended)
   - Channels: Mono or multi-channel both work (the program will auto-process)
   - Format: Standard WAV file (PCM encoded)

3. **CGO Compilation**: Requires C compiler and correct CGO environment

4. **File Size**: No strict limit, but test files should not be too long (a few seconds to a few minutes)

## Troubleshooting

If you encounter "VAD initialization failed" error:

1. Check if the `lib/ten-vad` directory exists and contains the correct library files
2. Check if the CGO environment is properly configured
3. On Windows, ensure the DLL file is in the system path or program directory
4. On Linux, ensure the `.so` file is in the library path
