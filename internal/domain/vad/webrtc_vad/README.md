# WebRTC VAD Resource Pool Implementation

This implementation provides WebRTC VAD (Voice Activity Detection) resource pool management functionality, used for improving performance and resource utilization in concurrent scenarios.

## Main Components

### 1. WebRTCVAD
Basic VAD implementation, now implements `Resource` interface:
- `IsValid()`: check resource whether valid
- `Close()`: close and release resource
- Thread-safe operations

### 2. WebRTCVADFactory
Resource factory, implements `ResourceFactory` interface:
- `Create()`: create new VAD instance
- `Validate()`: validate resource validity
- `Reset()`: reset resource state

### 3. WebRTCVADPool
VAD resource pool manager:
- `AcquireVAD()`: get VAD instance
- `ReleaseVAD()`: release VAD instance
- `Stats()`: get statistics
- `Close()`: close resource pool

### 4. VADManager
High-level encapsulation, provides convenient usage interface:
- `ProcessAudio()`: process single audio data
- `ProcessAudioBatch()`: batch process audio data
- `WithVAD()`: use callback function to process VAD

## Usage

### Basic Usage

```go
// Create VAD config
config := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2, // medium sensitivity
}

// Create VAD manager
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Process audio data
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

### Advanced Usage - Direct Use Resource Pool

```go
// Create resource pool
vadConfig := WebRTCVADConfig{
    SampleRate: 16000,
    Mode:       2,
}

poolConfig := &util.PoolConfig{
    MaxSize:          5,               // maximum instance count
    MinSize:          1,               // pre-create instance count
    MaxIdle:          3,               // maximum idle instance count
    AcquireTimeout:   5 * time.Second, // acquire timeout
    IdleTimeout:      2 * time.Minute, // idle timeout
    ValidateOnBorrow: true,            // validate on acquire
}

pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// Get VAD instance
vad, err := pool.AcquireVAD()
if err != nil {
    log.Fatal(err)
}

// Use VAD
isActive, err := vad.IsVAD(audioData)

// Release VAD instance
pool.ReleaseVAD(vad)
```

### Concurrent Usage

```go
manager, err := NewVADManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Multiple goroutines concurrent processing
for i := 0; i < numWorkers; i++ {
    go func(workerID int) {
        audioData := generateAudioData() // generate audio data
        
        active, err := manager.ProcessAudio(audioData)
        if err != nil {
            log.Printf("Worker %d failed: %v", workerID, err)
            return
        }
        
        fmt.Printf("Worker %d: active = %v\n", workerID, active)
    }(i)
}
```

## Configuration Parameters

### WebRTCVADConfig
- `SampleRate`: sampling rate (8000, 16000, 32000, 48000)
- `Mode`: VAD sensitivity mode (0: least sensitive, 3: most sensitive)

### PoolConfig
- `MaxSize`: maximum resource count
- `MinSize`: minimum resource count (pre-create)
- `MaxIdle`: maximum idle resource count
- `AcquireTimeout`: get resource timeout time
- `IdleTimeout`: resource idle timeout time
- `ValidateOnBorrow`: get when whether validate resource
- `ValidateOnReturn`: return when whether validate resource

## Advantages

1. **Resource Reuse**: Avoid frequent creation and destruction of VAD instances
2. **Concurrent Safety**: Support multiple goroutines concurrent usage
3. **Automatic Management**: Automatically clean up idle timeout resources
4. **Performance Monitoring**: Provide detailed statistics
5. **Flexible Configuration**: Support custom pool size and timeout parameters

## Performance Statistics

Use `GetStats()` method to get resource pool statistics:

```go
stats := manager.GetStats()
fmt.Printf("Pool stats: %+v\n", stats)
// output example:
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

## Error Processing

Main error types:
- Acquire timeout: `acquire timeout after 5s`
- Resource pool already closed: `pool is closed`
- Invalid resource type: `invalid resource type`
- VAD initialization failed: `failed to initialize WebRTC VAD`

## Best Practices

1. **Reasonably Set Pool Size**: Set `MaxSize` according to concurrent requirements
2. **Release Resources in Time**: Use `defer` to ensure resources are released
3. **Monitor Statistics**: Regularly check pool usage
4. **Graceful Shutdown**: Call `Close()` method when program exits
5. **Error Processing**: Handle acquire timeout and other abnormal situations

## Example Code

Check `example_usage.go` file for complete examples:
- Basic usage example
- Batch processing example
- Callback function usage example
- Concurrent usage example

## Testing

Run tests:
```bash
go test -v ./internal/domain/vad/webrtc_vad/
```

Run performance tests:
```bash
go test -bench=. ./internal/domain/vad/webrtc_vad/
``` 