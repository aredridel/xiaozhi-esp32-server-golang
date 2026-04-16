package util

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Resource resource interface, all resources managed by pool need to implement this interface
type Resource interface {
	// Close closeresource
	Close() error
	// IsValid inspectresourcewhethervalid
	IsValid() bool
}

// ResourceFactory resource factory interface, used for creating and validating resources
type ResourceFactory interface {
	// Create create new resource instance
	Create() (Resource, error)
	// Validate validate resource whether valid (optional, if return false, resource will be destroyed)
	Validate(resource Resource) bool
	// Reset reset resource state (optional, used for cleanup before resource reuse)
	Reset(resource Resource) error
}

// PoolConfig resource pool config
type PoolConfig struct {
	// MaxSize maximum resource count
	MaxSize int
	// MinSize minimum resource count (pre-create)
	MinSize int
	// MaxIdle maximum idle resource count
	MaxIdle int
	// AcquireTimeout get resource timeout time
	AcquireTimeout time.Duration
	// IdleTimeout resource idle timeout time
	IdleTimeout time.Duration
	// ValidateOnBorrow get when whether validate resource
	ValidateOnBorrow bool
	// ValidateOnReturn return when whether validate resource
	ValidateOnReturn bool
}

// DefaultConfig return default config
func DefaultConfig() *PoolConfig {
	return &PoolConfig{
		MaxSize:          1000,
		MinSize:          1,
		MaxIdle:          5,
		AcquireTimeout:   30 * time.Second,
		IdleTimeout:      5 * time.Minute,
		ValidateOnBorrow: true,
		ValidateOnReturn: false,
	}
}

// pooledResource pooled resource wrapper
type pooledResource struct {
	resource   Resource
	createTime time.Time
	lastUsed   time.Time
	inUse      bool
}

// ResourcePool general resource pool
type ResourcePool struct {
	config  *PoolConfig
	factory ResourceFactory

	// available resource queue
	available chan *pooledResource
	// all resource map (including in use and available)
	resources map[Resource]*pooledResource
	// read write lock
	mu sync.RWMutex
	// close flag
	closed bool
	// cancel context
	ctx    context.Context
	cancel context.CancelFunc
	// cleanup goroutine waitgroup
	cleanupWg sync.WaitGroup
}

// NewResourcePool create new resource pool
func NewResourcePool(config *PoolConfig, factory ResourceFactory) (*ResourcePool, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if factory == nil {
		return nil, errors.New("factory cannot be nil")
	}
	if config.MaxSize <= 0 {
		return nil, errors.New("max size must be positive")
	}
	if config.MinSize < 0 {
		return nil, errors.New("min size cannot be negative")
	}
	if config.MinSize > config.MaxSize {
		return nil, errors.New("min size cannot be greater than max size")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ResourcePool{
		config:    config,
		factory:   factory,
		available: make(chan *pooledResource, config.MaxSize),
		resources: make(map[Resource]*pooledResource),
		ctx:       ctx,
		cancel:    cancel,
	}

	// pre-create minimum count of resources
	if err := pool.preCreateResources(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to pre-create resources: %w", err)
	}

	// start cleanup goroutine
	pool.startCleanupRoutine()

	return pool, nil
}

// preCreateResources pre-create resources
func (p *ResourcePool) preCreateResources() error {
	for i := 0; i < p.config.MinSize; i++ {
		resource, err := p.factory.Create()
		if err != nil {
			return fmt.Errorf("failed to create resource %d: %w", i, err)
		}

		pooled := &pooledResource{
			resource:   resource,
			createTime: time.Now(),
			lastUsed:   time.Now(),
			inUse:      false,
		}

		p.resources[resource] = pooled
		p.available <- pooled
	}
	return nil
}

// Acquire get resource
func (p *ResourcePool) Acquire() (Resource, error) {
	return p.AcquireWithTimeout(p.config.AcquireTimeout)
}

// AcquireWithTimeout get resource within specified timeout time
func (p *ResourcePool) AcquireWithTimeout(timeout time.Duration) (Resource, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, errors.New("pool is closed")
	}
	p.mu.RUnlock()

	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire timeout after %v", timeout)
		case pooled := <-p.available:
			// validate resource validity
			if p.config.ValidateOnBorrow && pooled.resource != nil {
				if !pooled.resource.IsValid() || !p.factory.Validate(pooled.resource) {
					// resource invalid, destroy and try create new
					p.destroyResource(pooled)
					if newResource, err := p.tryCreateResource(); err == nil {
						return newResource, nil
					}
					continue
				}
			}

			// reset resource state
			if err := p.factory.Reset(pooled.resource); err != nil {
				p.destroyResource(pooled)
				continue
			}

			// mark is in use
			p.mu.Lock()
			pooled.inUse = true
			pooled.lastUsed = time.Now()
			p.mu.Unlock()

			return pooled.resource, nil
		default:
			// no available resource, try create new
			if resource, err := p.tryCreateResource(); err == nil {
				return resource, nil
			}
			// create failed, wait resource release
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// tryCreateResource try create new resource
func (p *ResourcePool) tryCreateResource() (Resource, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.resources) >= p.config.MaxSize {
		return nil, errors.New("pool is full")
	}

	resource, err := p.factory.Create()
	if err != nil {
		return nil, err
	}

	pooled := &pooledResource{
		resource:   resource,
		createTime: time.Now(),
		lastUsed:   time.Now(),
		inUse:      true,
	}

	p.resources[resource] = pooled
	return resource, nil
}

// Release release resource back to pool
func (p *ResourcePool) Release(resource Resource) error {
	if resource == nil {
		return errors.New("resource cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	pooled, exists := p.resources[resource]
	if !exists {
		return errors.New("resource not managed by this pool")
	}

	if !pooled.inUse {
		return errors.New("resource is not in use")
	}

	// validate resource validity
	if p.config.ValidateOnReturn {
		if !resource.IsValid() || !p.factory.Validate(resource) {
			p.destroyResourceUnsafe(pooled)
			return nil
		}
	}

	// check if exceed maximum idle count
	if len(p.available) >= p.config.MaxIdle {
		p.destroyResourceUnsafe(pooled)
		return nil
	}

	// mark is available
	pooled.inUse = false
	pooled.lastUsed = time.Now()

	// try put back to available queue
	select {
	case p.available <- pooled:
		return nil
	default:
		// queue already full, destroy resource
		p.destroyResourceUnsafe(pooled)
		return nil
	}
}

// destroyResource destroy resource (with lock)
func (p *ResourcePool) destroyResource(pooled *pooledResource) {
	p.mu.Lock()
	p.destroyResourceUnsafe(pooled)
	p.mu.Unlock()
}

// destroyResourceUnsafe destroy resource (without lock)
func (p *ResourcePool) destroyResourceUnsafe(pooled *pooledResource) {
	if pooled.resource != nil {
		pooled.resource.Close()
		delete(p.resources, pooled.resource)
	}
}

// Stats get resource pool count info
func (p *ResourcePool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	inUseCount := 0
	for _, pooled := range p.resources {
		if pooled.inUse {
			inUseCount++
		}
	}

	return map[string]interface{}{
		"total_resources":     len(p.resources),
		"available_resources": len(p.available),
		"in_use_resources":    inUseCount,
		"max_size":            p.config.MaxSize,
		"min_size":            p.config.MinSize,
		"max_idle":            p.config.MaxIdle,
		"is_closed":           p.closed,
	}
}

// Resize adjust pool size
func (p *ResourcePool) Resize(newMaxSize int) error {
	if newMaxSize <= 0 {
		return errors.New("new max size must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	oldMaxSize := p.config.MaxSize
	p.config.MaxSize = newMaxSize

	// if shorten pool size, need remove excess resources
	if newMaxSize < oldMaxSize {
		excess := len(p.resources) - newMaxSize
		for excess > 0 {
			select {
			case pooled := <-p.available:
				p.destroyResourceUnsafe(pooled)
				excess--
			default:
				// no more available resource can remove
				break
			}
		}
	}

	return nil
}

// startCleanupRoutine start cleanup goroutine
func (p *ResourcePool) startCleanupRoutine() {
	if p.config.IdleTimeout <= 0 {
		return
	}

	p.cleanupWg.Add(1)
	go func() {
		defer p.cleanupWg.Done()
		ticker := time.NewTicker(p.config.IdleTimeout / 2)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.cleanupIdleResources()
			}
		}
	}()
}

// cleanupIdleResources cleanup idle timeout resources
func (p *ResourcePool) cleanupIdleResources() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	var toRemove []*pooledResource

	// inspect available queue of idle resources
	for {
		select {
		case pooled := <-p.available:
			if now.Sub(pooled.lastUsed) > p.config.IdleTimeout {
				toRemove = append(toRemove, pooled)
			} else {
				// put back to queue
				p.available <- pooled
				goto cleanup
			}
		default:
			goto cleanup
		}
	}

cleanup:
	// destroy timeout resources
	for _, pooled := range toRemove {
		p.destroyResourceUnsafe(pooled)
	}
}

// Close close resource pool
func (p *ResourcePool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// cancel context
	p.cancel()

	// wait cleanup goroutine end
	p.cleanupWg.Wait()

	// close all resources
	p.mu.Lock()
	defer p.mu.Unlock()

	// clear available queue
	close(p.available)
	for pooled := range p.available {
		p.destroyResourceUnsafe(pooled)
	}

	// close all resources
	for _, pooled := range p.resources {
		p.destroyResourceUnsafe(pooled)
	}

	return nil
}
