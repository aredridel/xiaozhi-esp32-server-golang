package mcp

import (
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"
)

// MCPManager unifiedofMCPmanage器，negative责coordinateallchildmanage器
type MCPManager struct {
	localManager  *LocalMCPManager
	globalManager *GlobalMCPManager
	// deviceManager will来canat这inmanagedevicemanage器pool

	mu      sync.RWMutex
	started bool
}

var (
	mcpManager *MCPManager
	mcpOnce    sync.Once
)

// GetMCPManager getunifiedMCPmanage器singleton
func GetMCPManager() *MCPManager {
	mcpOnce.Do(func() {
		mcpManager = &MCPManager{
			localManager:  GetLocalMCPManager(),
			globalManager: GetGlobalMCPManager(),
			started:       false,
		}
	})
	return mcpManager
}

// Start startallMCPmanage器
func (m *MCPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		log.Warn("MCPmanage器alreadystart")
		return nil
	}

	log.Info("=== startMCPmanage器cluster ===")

	// 1. firstfirststartlocalmanage器
	log.Info("startlocalMCPmanage器...")
	if err := m.localManager.Start(); err != nil {
		log.Errorf("startlocalMCPmanage器failed: %v", err)
		return fmt.Errorf("startlocalMCPmanage器failed: %v", err)
	}

	// 2. 然afterstartglobalmanage器
	log.Info("startglobalMCPmanage器...")
	if err := m.globalManager.Start(); err != nil {
		log.Errorf("startglobalMCPmanage器failed: %v", err)
		return fmt.Errorf("startglobalMCPmanage器failed: %v", err)
	}

	// 3. devicemanage器throughjoinwhendynamiccreate，这innoneedstart
	log.Info("deviceMCPmanage器willaccording tojoindynamiccreate")

	m.started = true
	log.Info("=== MCPmanage器clusterstartcomplete ===")

	// outputstartstatecount
	m.printStartupStats()

	return nil
}

// Stop stopallMCPmanage器
func (m *MCPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		log.Info("MCPmanage器notstart，noneedstop")
		return nil
	}

	log.Info("=== stopMCPmanage器cluster ===")

	// 按相反sequentialstopmanage器
	// 1. stopglobalmanage器
	log.Info("stopglobalMCPmanage器...")
	if err := m.globalManager.Stop(); err != nil {
		log.Errorf("stopglobalMCPmanage器failed: %v", err)
	}

	// 2. stoplocalmanage器
	log.Info("stoplocalMCPmanage器...")
	if err := m.localManager.Stop(); err != nil {
		log.Errorf("stoplocalMCPmanage器failed: %v", err)
	}

	// 3. devicemanage器throughjoindisconnectautomaticcleanup
	log.Info("deviceMCPjoinwillautomaticcleanup")

	m.started = false
	log.Info("=== MCPmanage器clusteralreadystop ===")
	return nil
}

// IsStarted inspectmanage器whetheralreadystart
func (m *MCPManager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetLocalManager getlocalmanage器
func (m *MCPManager) GetLocalManager() *LocalMCPManager {
	return m.localManager
}

// GetGlobalManager getglobalmanage器
func (m *MCPManager) GetGlobalManager() *GlobalMCPManager {
	return m.globalManager
}

// printStartupStats outputstartstatecount
func (m *MCPManager) printStartupStats() {
	localToolCount := m.localManager.GetToolCount()
	globalToolCount := len(m.globalManager.GetAllTools())

	log.Infof("MCPmanage器startcount:")
	log.Infof("  - localtoolcount: %d", localToolCount)
	log.Infof("  - globaltoolcount: %d", globalToolCount)
	log.Infof("  - devicemanage器: dynamicmanage")
	log.Infof("  - totaltoolcount: %d", localToolCount+globalToolCount)
}

// GetAllManagersStatus getallmanage器ofstateinfo
func (m *MCPManager) GetAllManagersStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"mcp_manager": map[string]interface{}{
			"started": m.started,
		},
		"local_manager": map[string]interface{}{
			"tool_count": m.localManager.GetToolCount(),
			"tool_names": m.localManager.GetToolNames(),
		},
		"global_manager": map[string]interface{}{
			"tool_count": len(m.globalManager.GetAllTools()),
		},
		"device_manager": map[string]interface{}{
			"active_devices": mcpClientPool.device2McpClient.Count(),
		},
	}

	return status
}

// RestartManager restartspecifyofmanage器
func (m *MCPManager) RestartManager(managerType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("MCPmanage器clusternotstart")
	}

	switch managerType {
	case "local":
		log.Info("restartlocalMCPmanage器...")
		if err := m.localManager.Stop(); err != nil {
			log.Errorf("stoplocalmanage器failed: %v", err)
		}
		if err := m.localManager.Start(); err != nil {
			return fmt.Errorf("restartlocalmanage器failed: %v", err)
		}
		log.Info("localMCPmanage器restartcomplete")

	case "global":
		log.Info("restartglobalMCPmanage器...")
		if err := m.globalManager.Stop(); err != nil {
			log.Errorf("stopglobalmanage器failed: %v", err)
		}
		if err := m.globalManager.Start(); err != nil {
			return fmt.Errorf("restartglobalmanage器failed: %v", err)
		}
		log.Info("globalMCPmanage器restartcomplete")

	default:
		return fmt.Errorf("unsupportedofmanage器type: %s", managerType)
	}

	return nil
}

// istoafter兼容，provide便捷function

// StartMCPManagers startallMCPmanage器（便捷function）
func StartMCPManagers() error {
	return GetMCPManager().Start()
}

// StopMCPManagers stopallMCPmanage器（便捷function）
func StopMCPManagers() error {
	return GetMCPManager().Stop()
}

// GetMCPManagerStatus getMCPmanage器state（便捷function）
func GetMCPManagerStatus() map[string]interface{} {
	return GetMCPManager().GetAllManagersStatus()
}
