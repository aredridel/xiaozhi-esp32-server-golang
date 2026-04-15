# WebRTC VAD resource池实现

这个实现is WebRTC VAD (Voice Activity Detection) provideresource池管理功能，used for提高并发场景下of性能和resource利用率。

## 主要组件

### 1. WebRTCVAD
基础of VAD 实现，现at实现 `Resource` 接口：
- `IsValid()`: checkresourcewhether有效
- `Close()`: 关闭并释放resource
- 线程安全of操作

### 2. WebRTCVADFactory
resource工厂，实现 `ResourceFactory` 接口：
- `Create()`: 创建new VAD 实例
- `Validate()`: 验证resource有效性
- `Reset()`: resetresource状态

### 3. WebRTCVADPool
VAD resource池管理器：
- `AcquireVAD()`: get VAD 实例
- `ReleaseVAD()`: 释放 VAD 实例
- `Stats()`: get统计信息
- `Close()`: 关闭resource池

### 4. VADManager
高级封装，provide便捷ofuse接口：
- `ProcessAudio()`: process单个audiodata
- `ProcessAudioBatch()`: 批量processaudiodata
- `WithVAD()`: use回调函数process VAD

## use方法

### 基本use

```go
// 创建 VAD config
config := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2, // inetc敏感度
}

// 创建 VAD 管理器
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// processaudiodata
audioData := make([]float32, 320) // 16kHz, 20ms
isActive, err := manager.ProcessAudio(audioData)
if err != nil {
    log.Printf("VAD processing failed: %v", err)
    return
}

if isActive {
    fmt.Println("Voice activity detected!")
}
```

### 高级use - 直接useresource池

```go
// 创建resource池
vadConfig := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2,
}

poolConfig := &util.PoolConfig{
    MaxSize:          5,               // 最大实例数
    MinSize:          1,               // 预创建实例数
    MaxIdle:          3,               // 最大空闲实例数
    AcquireTimeout:   5 * time.Second, // get超when
    IdleTimeout:      2 * time.Minute, // 空闲超when
    ValidateOnBorrow: true,            // getwhen验证
}

pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// get VAD 实例
vad, err := pool.AcquireVAD()
if err != nil {
    log.Fatal(err)
}

// use VAD
isActive, err := vad.IsVAD(audioData)

// 释放 VAD 实例
pool.ReleaseVAD(vad)
```

### 并发use

```go
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// 多个 goroutine 并发process
for i := 0; i < numWorkers; i++ {
    go func(workerID int) {
        audioData := generateAudioData() // generateaudiodata
        
        active, err := manager.ProcessAudio(audioData)
        if err != nil {
            log.Printf("Worker %d failed: %v", workerID, err)
            return
        }
        
        fmt.Printf("Worker %d: active = %v\n", workerID, active)
    }(i)
}
```

## config参数

### WebRTCVADConfig
- `SampleRate`: 采样率 (8000, 16000, 32000, 48000)
- `Mode`: VAD 敏感度模式 (0: 最不敏感, 3: 最敏感)

### PoolConfig
- `MaxSize`: 最大resourcecount
- `MinSize`: 最小resourcecount（预创建）
- `MaxIdle`: 最大空闲resourcecount
- `AcquireTimeout`: getresource超whentime
- `IdleTimeout`: resource空闲超whentime
- `ValidateOnBorrow`: getwhenwhether验证resource
- `ValidateOnReturn`: 归还whenwhether验证resource

## 优势

1. **resource复用**: 避免频繁创建和销毁 VAD 实例
2. **并发安全**: 支持多个 goroutine 并发use
3. **自动管理**: 自动清理空闲超whenofresource
4. **性能监控**: provide详细of统计信息
5. **config灵活**: 支持自定义池size和超when参数

## 性能统计

use `GetStats()` 方法getresource池统计信息：

```go
stats := manager.GetStats()
fmt.Printf("Pool stats: %+v\n", stats)
// output示例:
// {
//   "total_resources": 3,
//   "available_resources": 2,
//   "in_use_resources": 1,
//   "max_size": 5,
//   "min_size": 1,
//   "max_idle": 3,
//   "is_closed": false
// }
```

## errorprocess

主要oferror类型：
- get超when：`acquire timeout after 5s`
- resource池already关闭：`pool is closed`
- 无效resource类型：`invalid resource type`
- VAD 初始化failed：`failed to initialize WebRTC VAD`

## 最佳实践

1. **合理设置池size**: according to并发需求设置 `MaxSize`
2. **andwhen释放resource**: use `defer` 确保resourcebe释放
3. **监控统计信息**: 定期check池ofuse情况
4. **优雅关闭**: 程序exitwhen调用 `Close()` 方法
5. **errorprocess**: processget超whenetc异常情况

## 示例代码

查看 `example_usage.go` 文件inof完整示例：
- 基本use示例
- 批量process示例
- 回调函数use示例
- 并发use示例

## 测试

运行测试：
```bash
go test -v ./internal/domain/vad/webrtc_vad/
```

运行性能测试：
```bash
go test -bench=. ./internal/domain/vad/webrtc_vad/
``` 