package memory

import (
	"context"
	"fmt"
	"sync"

	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// MemoryUserConfigProvider memory user config provider
// implement UserConfigProvider interface, store config in memory
// Note: data will be lost after restart, suitable for test or temporary storage scenario
type MemoryUserConfigProvider struct {
	mu         sync.RWMutex
	configs    map[string]types.UConfig
	maxEntries int
}

// MemoryConfig memoryconfigstructure
type MemoryConfig struct {
	MaxEntries int `json:"max_entries"` // maximum store entry count
}

// NewMemoryUserConfigProvider create memory user config provider
// config: config parameter map, include max_entries etc
func NewMemoryUserConfigProvider(config map[string]interface{}) (*MemoryUserConfigProvider, error) {
	// parse config parameter
	memoryConfig := &MemoryConfig{
		MaxEntries: 1000, // default maximum 1000 configs
	}

	if maxEntries, ok := config["max_entries"].(int); ok && maxEntries > 0 {
		memoryConfig.MaxEntries = maxEntries
	} else if maxEntriesFloat, ok := config["max_entries"].(float64); ok && maxEntriesFloat > 0 {
		memoryConfig.MaxEntries = int(maxEntriesFloat)
	}

	provider := &MemoryUserConfigProvider{
		configs:    make(map[string]types.UConfig),
		maxEntries: memoryConfig.MaxEntries,
	}

	log.Log().Infof("memory user config provider initialize successful, maximum entry count: %d", memoryConfig.MaxEntries)
	return provider, nil
}

// GetUserConfig get user config
func (m *MemoryUserConfigProvider) GetUserConfig(ctx context.Context, userID string) (types.UConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[userID]
	if !exists {
		log.Log().Debugf("user %s config not exist, return empty config", userID)
		return types.UConfig{}, nil
	}

	return config, nil
}

// SetUserConfig set user config
func (m *MemoryUserConfigProvider) SetUserConfig(ctx context.Context, userID string, config types.UConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// check if exceed maximum entry count
	if len(m.configs) >= m.maxEntries && !m.configExists(userID) {
		return fmt.Errorf("already reach to maximum store entry count %d, cannot add new config", m.maxEntries)
	}

	m.configs[userID] = config
	log.Log().Infof("user %s config set successful (memory store)", userID)
	return nil
}

// DeleteUserConfig delete user config
func (m *MemoryUserConfigProvider) DeleteUserConfig(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[userID]; !exists {
		log.Log().Warnf("user %s config not exist, no need delete", userID)
		return nil
	}

	delete(m.configs, userID)
	log.Log().Infof("user %s config delete successful (memory store)", userID)
	return nil
}

// Close close provider (memory provider no need special cleanup)
func (m *MemoryUserConfigProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// clear all config
	m.configs = make(map[string]types.UConfig)
	log.Log().Info("memory user config provider already closed, all config already cleared")
	return nil
}

// configExists inspect config whether exists (internal method, call when need hold lock)
func (m *MemoryUserConfigProvider) configExists(userID string) bool {
	_, exists := m.configs[userID]
	return exists
}

// GetStats get store count info (extra method)
func (m *MemoryUserConfigProvider) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_configs": len(m.configs),
		"max_entries":   m.maxEntries,
		"usage_percent": float64(len(m.configs)) / float64(m.maxEntries) * 100,
	}
}

// ListUserIDs list all user IDs (extra method)
func (m *MemoryUserConfigProvider) ListUserIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs := make([]string, 0, len(m.configs))
	for userID := range m.configs {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// GetSystemConfig get system config
func (m *MemoryUserConfigProvider) GetSystemConfig(ctx context.Context) (string, error) {
	// memory config provider no provide system config
	return "", nil
}

// Init initialize Memory config provider
func Init(ctx context.Context) error {
	log.Log().Info("Memory config provider initialized successfully")
	return nil
}

// Close close Memory config provider, cleanup resource
func Close() error {
	log.Log().Info("Memory config provider closed")
	return nil
}

// IsConnected inspect Memory config provider whether already connected
func IsConnected() bool {
	// memory config provider always is "connected" state
	return true
}
