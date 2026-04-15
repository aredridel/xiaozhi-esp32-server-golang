package webrtc_vad

import (
	"context"
	"sync"
	"testing"
	"time"

	"xiaozhi-esp32-server-golang/internal/util"
)

func TestWebRTCVADPool(t *testing.T) {
	// createVADconfig
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	// createpoolconfig
	poolConfig := &util.PoolConfig{
		MaxSize:          3,
		MinSize:          1,
		MaxIdle:          2,
		AcquireTimeout:   5 * time.Second,
		IdleTimeout:      1 * time.Minute,
		ValidateOnBorrow: true,
		ValidateOnReturn: true,
	}

	// createVADresourcepool
	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// testgetandreleaseVAD
	vad, err := pool.AcquireVAD()
	if err != nil {
		t.Fatalf("Failed to acquire VAD: %v", err)
	}

	// testVADfunction
	testData := make([]float32, 320) // 20msof16kHzaudio data
	for i := range testData {
		testData[i] = 0.1 // 填充a些testdata
	}

	active, err := vad.IsVAD(testData)
	if err != nil {
		t.Errorf("VAD detection failed: %v", err)
	}

	t.Logf("VAD result: %v", active)

	// releaseVAD
	err = pool.ReleaseVAD(vad)
	if err != nil {
		t.Errorf("Failed to release VAD: %v", err)
	}

	// inspectcountinfo
	stats := pool.Stats()
	t.Logf("Pool stats: %+v", stats)
}

func TestWebRTCVADPoolConcurrency(t *testing.T) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize:        5,
		MinSize:        2,
		MaxIdle:        3,
		AcquireTimeout: 10 * time.Second,
		IdleTimeout:    30 * time.Second,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// concurrenttest
	numWorkers := 10
	numIterations := 5
	var wg sync.WaitGroup

	testData := make([]float32, 320)
	for i := range testData {
		testData[i] = float32(i%100) / 100.0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// getVADinstance
				vad, err := pool.AcquireVAD()
				if err != nil {
					t.Errorf("Worker %d iteration %d: Failed to acquire VAD: %v", workerID, j, err)
					return
				}

				// useVAD
				_, err = vad.IsVAD(testData)
				if err != nil {
					t.Errorf("Worker %d iteration %d: VAD detection failed: %v", workerID, j, err)
				}

				// mocka些processtime
				time.Sleep(10 * time.Millisecond)

				// releaseVAD
				err = pool.ReleaseVAD(vad)
				if err != nil {
					t.Errorf("Worker %d iteration %d: Failed to release VAD: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// inspectfinallycountinfo
	stats := pool.Stats()
	t.Logf("Final pool stats: %+v", stats)
}

func TestWebRTCVADFactory(t *testing.T) {
	config := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	factory := NewWebRTCVADFactory(config)

	// testcreateresource
	resource, err := factory.Create()
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}
	defer resource.Close()

	// validateresourcetype
	vad, ok := resource.(*WebRTCVAD)
	if !ok {
		t.Fatalf("Created resource is not WebRTCVAD type")
	}

	// validateconfig
	if vad.GetSampleRate() != config.SampleRate {
		t.Errorf("Expected sample rate %d, got %d", config.SampleRate, vad.GetSampleRate())
	}

	if vad.GetMode() != config.Mode {
		t.Errorf("Expected mode %d, got %d", config.Mode, vad.GetMode())
	}

	// testvalidatefunction
	if !factory.Validate(resource) {
		t.Error("Factory validation failed for valid resource")
	}

	// testresetfunction
	err = factory.Reset(resource)
	if err != nil {
		t.Errorf("Factory reset failed: %v", err)
	}

	// testresourcevalid性
	if !resource.IsValid() {
		t.Error("Resource should be valid after reset")
	}
}

func TestWebRTCVADPoolTimeout(t *testing.T) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize:        1, // onlyallowaresource
		MinSize:        1,
		MaxIdle:        1,
		AcquireTimeout: 100 * time.Millisecond, // shorttimeouttime
		IdleTimeout:    1 * time.Minute,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// getnthaVADinstance
	vad1, err := pool.AcquireVAD()
	if err != nil {
		t.Fatalf("Failed to acquire first VAD: %v", err)
	}

	// trygetnth二个VADinstance，shouldtimeout
	start := time.Now()
	vad2, err := pool.AcquireVAD()
	elapsed := time.Since(start)

	if err == nil {
		pool.ReleaseVAD(vad2)
		t.Error("Expected timeout error, but got VAD instance")
	}

	if elapsed < 90*time.Millisecond {
		t.Errorf("Expected timeout around 100ms, but got %v", elapsed)
	}

	// releasenthaVAD
	err = pool.ReleaseVAD(vad1)
	if err != nil {
		t.Errorf("Failed to release VAD: %v", err)
	}

	// 现atshould能够getVAD
	vad3, err := pool.AcquireVAD()
	if err != nil {
		t.Errorf("Failed to acquire VAD after release: %v", err)
	}
	pool.ReleaseVAD(vad3)
}

// BenchmarkWebRTCVADPool performancetest
func BenchmarkWebRTCVADPool(b *testing.B) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize: 10,
		MinSize: 2,
		MaxIdle: 5,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		b.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	testData := make([]float32, 320)
	for i := range testData {
		testData[i] = float32(i%100) / 100.0
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			vad, err := pool.AcquireVAD()
			if err != nil {
				b.Errorf("Failed to acquire VAD: %v", err)
				continue
			}

			_, err = vad.IsVAD(testData)
			if err != nil {
				b.Errorf("VAD detection failed: %v", err)
			}

			pool.ReleaseVAD(vad)
		}
	})
}
