package memory

import (
	"context"
	"fmt"
	"sync"

	"xiaozhi-esp32-server-golang/internal/domain/config/types"
	log "xiaozhi-esp32-server-golang/logger"
)

// MemoryUserConfigProvider memoryuserconfigprovide者
// implementUserConfigProviderinterface，willconfigstoreatmemoryin
// 注意：restartafterdatawill丢失，适used fortestor临whenstorescenario
type MemoryUserConfigProvider struct {
	mu         sync.RWMutex
	configs    map[string]types.UConfig
	maxEntries int
}

// MemoryConfig memoryconfigstructure
type MemoryConfig struct {
	MaxEntries int `json:"max_entries"` // maximumstore条目count
}

// NewMemoryUserConfigProvider creatememoryuserconfigprovide者
// config: configparametermap，includemax_entriesetc
func NewMemoryUserConfigProvider(config map[string]interface{}) (*MemoryUserConfigProvider, error) {
	// parseconfigparameter
	memoryConfig := &MemoryConfig{
		MaxEntries: 1000, // defaultmaximum1000个config
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

	log.Log().Infof("memoryuserconfigprovide者initializesuccessful，maximum条目count: %d", memoryConfig.MaxEntries)
	return provider, nil
}

// GetUserConfig getuserconfig
func (m *MemoryUserConfigProvider) GetUserConfig(ctx context.Context, userID string) (types.UConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[userID]
	if !exists {
		log.Log().Debugf("user %s configno存at，returnemptyconfig", userID)
		return types.UConfig{}, nil
	}

	return config, nil
}

// SetUserConfig setuserconfig
func (m *MemoryUserConfigProvider) SetUserConfig(ctx context.Context, userID string, config types.UConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// check if超pastmaximum条目count
	if len(m.configs) >= m.maxEntries && !m.configExists(userID) {
		return fmt.Errorf("alreadyreachtomaximumstore条目count %d，no法add新config", m.maxEntries)
	}

	m.configs[userID] = config
	log.Log().Infof("user %s configsetsuccessful (memorystore)", userID)
	return nil
}

// DeleteUserConfig deleteuserconfig
func (m *MemoryUserConfigProvider) DeleteUserConfig(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[userID]; !exists {
		log.Log().Warnf("user %s configno存at，noneeddelete", userID)
		return nil
	}

	delete(m.configs, userID)
	log.Log().Infof("user %s configdeletesuccessful (memorystore)", userID)
	return nil
}

// Close closeprovide者（memoryprovide者noneed特殊cleanup）
func (m *MemoryUserConfigProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// clearallconfig
	m.configs = make(map[string]types.UConfig)
	log.Log().Info("memoryuserconfigprovide者alreadyclose，allconfigalreadyclear")
	return nil
}

// configExists inspectconfigwhether存at（internalmethod，callwhenneed持havelock）
func (m *MemoryUserConfigProvider) configExists(userID string) bool {
	_, exists := m.configs[userID]
	return exists
}

// GetStats getstorecountinfo（额outsideof实usemethod）
func (m *MemoryUserConfigProvider) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_configs": len(m.configs),
		"max_entries":   m.maxEntries,
		"usage_percent": float64(len(m.configs)) / float64(m.maxEntries) * 100,
	}
}

// ListUserIDs 列outalluserID（额outsideof实usemethod）
func (m *MemoryUserConfigProvider) ListUserIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs := make([]string, 0, len(m.configs))
	for userID := range m.configs {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// GetSystemConfig getsystemconfig
func (m *MemoryUserConfigProvider) GetSystemConfig(ctx context.Context) (string, error) {
	// memoryconfigprovide者noprovidesystemconfig
	return "", nil
}

// Init initializeMemoryconfigprovide者
func Init(ctx context.Context) error {
	log.Log().Info("Memory config provider initialized successfully")
	return nil
}

// Close closeMemoryconfigprovide者，cleanupresource
func Close() error {
	log.Log().Info("Memory config provider closed")
	return nil
}

// IsConnected inspectMemoryconfigprovide者whetheralreadyjoin
func IsConnected() bool {
	// memoryconfigprovide者alwaysyes"join"state
	return true
}
