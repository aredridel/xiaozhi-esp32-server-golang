package util

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Resource resourceinterface，allbepoolmanageofresourceareneedimplementthisinterface
type Resource interface {
	// Close closeresource
	Close() error
	// IsValid inspectresourcewhethervalid
	IsValid() bool
}

// ResourceFactory resourcefactoryinterface，used forcreateandvalidateresource
type ResourceFactory interface {
	// Create create newresourceinstance
	Create() (Resource, error)
	// Validate validateresourcewhethervalid（optional，ifreturnfalse，resourcewillbedestroy）
	Validate(resource Resource) bool
	// Reset resetresourcestate（optional，used forresource复usebeforeofcleanup）
	Reset(resource Resource) error
}

// PoolConfig resourcepoolconfig
type PoolConfig struct {
	// MaxSize maximumresourcecount
	MaxSize int
	// MinSize minimumresourcecount（预create）
	MinSize int
	// MaxIdle maximumempty闲resourcecount
	MaxIdle int
	// AcquireTimeout getresourcetimeouttime
	AcquireTimeout time.Duration
	// IdleTimeout resourceempty闲timeouttime
	IdleTimeout time.Duration
	// ValidateOnBorrow getwhenwhethervalidateresource
	ValidateOnBorrow bool
	// ValidateOnReturn 归orwhenwhethervalidateresource
	ValidateOnReturn bool
}

// DefaultConfig returndefaultconfig
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

// pooledResource pool化resourcepackage装器
type pooledResource struct {
	resource   Resource
	createTime time.Time
	lastUsed   time.Time
	inUse      bool
}

// ResourcePool 通useresourcepool
type ResourcePool struct {
	config  *PoolConfig
	factory ResourceFactory

	// availableresourcequeue
	available chan *pooledResource
	// allresourcemap（package括atuseandavailableof）
	resources map[Resource]*pooledResource
	// readwritelock
	mu sync.RWMutex
	// closeflag
	closed bool
	// cancelcontext
	ctx    context.Context
	cancel context.CancelFunc
	// cleanupgoroutinewaitgroup
	cleanupWg sync.WaitGroup
}

// NewResourcePool create newresourcepool
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

	// 预createminimumcountofresource
	if err := pool.preCreateResources(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to pre-create resources: %w", err)
	}

	// startcleanupgoroutine
	pool.startCleanupRoutine()

	return pool, nil
}

// preCreateResources 预createresource
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

// Acquire getresource
func (p *ResourcePool) Acquire() (Resource, error) {
	return p.AcquireWithTimeout(p.config.AcquireTimeout)
}

// AcquireWithTimeout atspecifytimeouttimeinsidegetresource
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
			// validateresourcevalid性
			if p.config.ValidateOnBorrow && pooled.resource != nil {
				if !pooled.resource.IsValid() || !p.factory.Validate(pooled.resource) {
					// resourceinvalid，destroyandtrycreate new
					p.destroyResource(pooled)
					if newResource, err := p.tryCreateResource(); err == nil {
						return newResource, nil
					}
					continue
				}
			}

			// resetresourcestate
			if err := p.factory.Reset(pooled.resource); err != nil {
				p.destroyResource(pooled)
				continue
			}

			// markisusein
			p.mu.Lock()
			pooled.inUse = true
			pooled.lastUsed = time.Now()
			p.mu.Unlock()

			return pooled.resource, nil
		default:
			// noavailableresource，trycreate new
			if resource, err := p.tryCreateResource(); err == nil {
				return resource, nil
			}
			// createfailed，waitresourcerelease
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// tryCreateResource trycreate新resource
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

// Release releaseresource回pool
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

	// validateresourcevalid性
	if p.config.ValidateOnReturn {
		if !resource.IsValid() || !p.factory.Validate(resource) {
			p.destroyResourceUnsafe(pooled)
			return nil
		}
	}

	// check if超pastmaximumempty闲count
	if len(p.available) >= p.config.MaxIdle {
		p.destroyResourceUnsafe(pooled)
		return nil
	}

	// markisavailable
	pooled.inUse = false
	pooled.lastUsed = time.Now()

	// tryplay回availablequeue
	select {
	case p.available <- pooled:
		return nil
	default:
		// queuealreadyfull，destroyresource
		p.destroyResourceUnsafe(pooled)
		return nil
	}
}

// destroyResource destroyresource（带lock）
func (p *ResourcePool) destroyResource(pooled *pooledResource) {
	p.mu.Lock()
	p.destroyResourceUnsafe(pooled)
	p.mu.Unlock()
}

// destroyResourceUnsafe destroyresource（no带lock）
func (p *ResourcePool) destroyResourceUnsafe(pooled *pooledResource) {
	if pooled.resource != nil {
		pooled.resource.Close()
		delete(p.resources, pooled.resource)
	}
}

// Stats getresourcepoolcountinfo
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

// Resize 调bodypoolsize
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

	// ifshortensmallpoolsize，needremove多余ofresource
	if newMaxSize < oldMaxSize {
		excess := len(p.resources) - newMaxSize
		for excess > 0 {
			select {
			case pooled := <-p.available:
				p.destroyResourceUnsafe(pooled)
				excess--
			default:
				// no更多availableresourcecanremove
				break
			}
		}
	}

	return nil
}

// startCleanupRoutine startcleanupgoroutine
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

// cleanupIdleResources cleanupempty闲timeoutofresource
func (p *ResourcePool) cleanupIdleResources() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	var toRemove []*pooledResource

	// inspectavailablequeueinofempty闲resource
	for {
		select {
		case pooled := <-p.available:
			if now.Sub(pooled.lastUsed) > p.config.IdleTimeout {
				toRemove = append(toRemove, pooled)
			} else {
				// play回queue
				p.available <- pooled
				goto cleanup
			}
		default:
			goto cleanup
		}
	}

cleanup:
	// destroytimeoutofresource
	for _, pooled := range toRemove {
		p.destroyResourceUnsafe(pooled)
	}
}

// Close closeresourcepool
func (p *ResourcePool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// cancelcontext
	p.cancel()

	// waitcleanupgoroutineend
	p.cleanupWg.Wait()

	// closeallresource
	p.mu.Lock()
	defer p.mu.Unlock()

	// clearavailablequeue
	close(p.available)
	for pooled := range p.available {
		p.destroyResourceUnsafe(pooled)
	}

	// closeallresource
	for _, pooled := range p.resources {
		p.destroyResourceUnsafe(pooled)
	}

	return nil
}
