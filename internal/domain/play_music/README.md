# play music功能

这个模块providefromURLstreamingplay musicof功能，支持from网络URLgetaudio文件并实when解码isaudio frame流。

## 功能特性

- ✅ **streaming播放**: 支持fromURL实when下载和play music
- ✅ **格式支持**: 主要支持MP3格式，自动解码isOpusaudio frame
- ✅ **audio解码**: 基于成熟ofaudio解码器，高效稳定
- ✅ **上下文控制**: 支持throughcontext取消和超when控制
- ✅ **连接池优化**: useHTTP连接池，提高网络性能
- ✅ **config灵活**: 可configframewhen长和audio格式
- ✅ **统计信息**: provide播放统计和状态监控

## 快速start

### 1. 基础use

```go
package main

import (
    "context"
    "fmt"
    
    "xiaozhi-esp32-server-golang/internal/domain/play_music"
)

func main() {
    // 创建音乐播放器
    config := play_music.DefaultMusicPlayerConfig()
    player := play_music.NewMusicPlayer(config.ToMap())
    
    // startplay music
    ctx := context.Background()
    audioChan, err := player.PlayMusicStream(ctx, "https://example.com/music.mp3")
    if err != nil {
        panic(err)
    }
    
    // processaudio frame
    for audioFrame := range audioChan {
        fmt.Printf("收toaudio frame: %d 字节\n", len(audioFrame))
        // 这里canwillaudio framesendto播放device或其他process
    }
}
```

### 2. 自定义config

```go
// 创建自定义config
config := &play_music.MusicPlayerConfig{
    FrameDuration: 20,   // 20msframewhen长
}

player := play_music.NewMusicPlayer(config.ToMap())

// or直接传入config映射
player := play_music.NewMusicPlayer(map[string]interface{}{
    "frame_duration": 20,
    "audio_format":   "mp3",
})
```

### 3. 带统计信息of完整示例

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
    
    // 统计信息
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
            fmt.Printf("首frame延迟: %d ms\n", stats.FirstFrameTime - stats.StartTime)
        }
        
        // processaudio frame...
    }
    
    fmt.Printf("播放complete，总frame数: %d\n", frameCount)
}
```

## API 参考

### MusicPlayer

主要of音乐播放器结构体。

#### 方法

##### `NewMusicPlayer(config map[string]interface{}) *MusicPlayer`

创建new音乐播放器实例。

**参数:**
- `config`: config参数映射

**config选项:**
- `frame_duration` (int): framewhen长(ms)，default20
- `audio_format` (string): audio格式，default"mp3"

##### `PlayMusicStream(ctx context.Context, url string) (chan []byte, error)`

fromURLstartstreamingplay music。

**参数:**
- `ctx`: 上下文to象，used for取消和超when控制
- `url`: 音乐文件ofURL地址

**return:**
- `chan []byte`: audio framedata通道
- `error`: error信息

##### `GetPlayerInfo() map[string]interface{}`

get播放器config信息。

##### `Stop() error`

stop播放器并清理resource。

### config类型

#### `MusicPlayerConfig`

```go
type MusicPlayerConfig struct {
    FrameDuration int    `json:"frame_duration"` // framewhen长(ms)
    AudioFormat   string `json:"audio_format"`   // audio格式，default"mp3"
}
```

#### `StreamingStats`

播放统计信息结构体，used for监控播放状态。

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

## 测试

运行测试示例：

```bash
cd test/music_player
go run main.go "https://example.com/music.mp3"
```

## 支持ofaudio格式

目before主要支持：
- **MP3**: 完全支持，推荐use
- **WAV**: 部分支持（through通用解码器）

## errorprocess

播放器provide简洁oferrorprocess机制：

1. **连接池优化**: useHTTP连接池提高网络稳定性
2. **上下文控制**: 支持throughcontext取消操作
3. **优雅exit**: 遇toerrorwhen优雅关闭通道

## 性能优化建议

1. **合理设置framewhen长**: default20ms适合大多数场景
2. **网络优化**: use稳定of网络连接，播放器already优化HTTP连接池
3. **内存管理**: andwhenprocessaudio framedata，避免通道阻塞
4. **并发控制**: 避免同when播放过多audio stream

## 集成示例

### andWebSocket集成

```go
func streamToWebSocket(audioChan <-chan []byte, ws *websocket.Conn) {
    for frame := range audioChan {
        err := ws.WriteMessage(websocket.BinaryMessage, frame)
        if err != nil {
            log.Errorf("sendWebSocketmessagefailed: %v", err)
            return
        }
    }
}
```

### 保存to文件

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

## 注意事项

1. **URL有效性**: 确保audioURL可访问且return有效audio文件
2. **内存use**: 长time播放need注意内存use情况
3. **网络稳定性**: use稳定of网络连接以获得最佳播放体验
4. **上下文管理**: andwhen取消不needof播放任务

## 故障排除

### 常见问题

**Q: 播放no声音**
A: checkURLwhether有效，audio格式whether支持

**Q: 播放延迟很高**
A: check网络连接，确保URL响应速度较快

**Q: 内存use过高**
A: checkaudio frameprocesswhetherandwhen，避免通道积压

## License

MIT License 