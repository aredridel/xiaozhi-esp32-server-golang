# Music Playback Feature

This module provides URL-based streaming music playback functionality, supporting downloading audio files from network URLs and real-time decoding into audio frame streams.

## Feature Highlights

- ✅ **Streaming Playback**: Supports real-time download and music playback from URLs
- ✅ **Format Support**: Primarily supports MP3 format, automatically decodes to Opus audio frames
- ✅ **Audio Decoding**: Based on mature audio decoder, efficient and stable
- ✅ **Context Control**: Supports cancellation and timeout control through context
- ✅ **Connection Pool Optimization**: Uses HTTP connection pool to improve network performance
- ✅ **Flexible Configuration**: Configurable frame duration and audio format
- ✅ **Statistics**: Provides playback statistics and status monitoring

## Quick Start

### 1. Basic Usage

```go
package main

import (
    "context"
    "fmt"
    
    "xiaozhi-esp32-server-golang/internal/domain/play_music"
)

func main() {
    // Create music player
    config := play_music.DefaultMusicPlayerConfig()
    player := play_music.NewMusicPlayer(config.ToMap())
    
    // Start music playback
    ctx := context.Background()
    audioChan, err := player.PlayMusicStream(ctx, "https://example.com/music.mp3")
    if err != nil {
        panic(err)
    }
    
    // Process audio frames
    for audioFrame := range audioChan {
        fmt.Printf("Received audio frame: %d bytes\n", len(audioFrame))
        // Here you can send audio frames to playback device or other processing
    }
}
```

### 2. Custom Configuration

```go
// Create custom configuration
config := &play_music.MusicPlayerConfig{
    FrameDuration: 20,   // 20ms frame duration
}

player := play_music.NewMusicPlayer(config.ToMap())

// Or directly pass configuration map
player := play_music.NewMusicPlayer(map[string]interface{}{
    "frame_duration": 20,
    "audio_format":   "mp3",
})
```

### 3. Complete Example with Statistics

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "xiaozhi-esp32-server-golang/internal/domain/play_music"
)

func main() {
    config := play_music.DefaultMusicPlayerConfig()
    player := play_music.NewMusicPlayer(config.ToMap())
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    audioChan, err := player.PlayMusicStream(ctx, "https://example.com/music.mp3")
    if err != nil {
        panic(err)
    }
    
    // Statistics
    stats := &play_music.StreamingStats{
        StartTime: time.Now().UnixMilli(),
    }
    
    frameCount := 0
    for audioFrame := range audioChan {
        frameCount++
        stats.FramesGenerated = int64(frameCount)
        stats.BytesDecoded += int64(len(audioFrame))
        
        if frameCount == 1 {
            stats.FirstFrameTime = time.Now().UnixMilli()
            fmt.Printf("First frame latency: %d ms\n", stats.FirstFrameTime - stats.StartTime)
        }
        
        // Process audio frame...
    }
    
    fmt.Printf("Playback complete, total frames: %d\n", frameCount)
}
```

## API Reference

### MusicPlayer

Main music player struct.

#### Methods

##### `NewMusicPlayer(config map[string]interface{}) *MusicPlayer`

Creates a new music player instance.

**Parameters:**
- `config`: Configuration parameter map

**Configuration Options:**
- `frame_duration` (int): Frame duration (ms), default 20
- `audio_format` (string): Audio format, default "mp3"

##### `PlayMusicStream(ctx context.Context, url string) (chan []byte, error)`

Starts streaming music playback from URL.

**Parameters:**
- `ctx`: Context object, used for cancellation and timeout control
- `url`: URL address of the music file

**Returns:**
- `chan []byte`: Audio frame data channel
- `error`: Error information

##### `GetPlayerInfo() map[string]interface{}`

Gets player configuration information.

##### `Stop() error`

Stops the player and cleans up resources.

### Configuration Types

#### `MusicPlayerConfig`

```go
type MusicPlayerConfig struct {
    FrameDuration int    `json:"frame_duration"` // Frame duration (ms)
    AudioFormat   string `json:"audio_format"`   // Audio format, default "mp3"
}
```

#### `StreamingStats`

Playback statistics struct, used for monitoring playback status.

```go
type StreamingStats struct {
    BytesDownloaded int64         `json:"bytes_downloaded"`
    BytesDecoded    int64         `json:"bytes_decoded"`
    FramesGenerated int64         `json:"frames_generated"`
    StartTime       int64         `json:"start_time"`
    FirstFrameTime  int64         `json:"first_frame_time"`
    Status          PlaybackStatus `json:"status"`
    ErrorCount      int           `json:"error_count"`
}
```

## Testing

Run test example:

```bash
cd test/music_player
go run main.go "https://example.com/music.mp3"
```

## Supported Audio Formats

Currently mainly supports:
- **MP3**: Fully supported, recommended for use
- **WAV**: Partially supported (through general decoder)

## Error Handling

Player provides concise error handling mechanism:

1. **Connection Pool Optimization**: Uses HTTP connection pool to improve network stability
2. **Context Control**: Supports cancellation of operations through context
3. **Graceful Exit**: Gracefully closes channel when encountering errors

## Performance Optimization Suggestions

1. **Set reasonable frame duration**: Default 20ms is suitable for most scenarios
2. **Network optimization**: Use stable network connections, player already optimizes HTTP connection pool
3. **Memory management**: Process audio frame data in a timely manner to avoid channel blocking
4. **Concurrency control**: Avoid playing too many audio streams simultaneously

## Integration Examples

### Integration with WebSocket

```go
func streamToWebSocket(audioChan <-chan []byte, ws *websocket.Conn) {
    for frame := range audioChan {
        err := ws.WriteMessage(websocket.BinaryMessage, frame)
        if err != nil {
            log.Errorf("Failed to send WebSocket message: %v", err)
            return
        }
    }
}
```

### Save to File

```go
func saveToFile(audioChan <-chan []byte, filename string) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    for frame := range audioChan {
        _, err := file.Write(frame)
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Notes

1. **URL Validity**: Ensure audio URL is accessible and returns valid audio file
2. **Memory Usage**: Long-time playback requires attention to memory usage
3. **Network Stability**: Use stable network connections for best playback experience
4. **Context Management**: Cancel unnecessary playback tasks in a timely manner

## Troubleshooting

### Common Issues

**Q: No sound during playback**
A: Check if URL is valid and if audio format is supported

**Q: High playback latency**
A: Check network connection, ensure URL responds quickly

**Q: High memory usage**
A: Check if audio frame processing is timely, avoid channel backlog

## License

MIT License
