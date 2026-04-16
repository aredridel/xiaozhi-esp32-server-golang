package mcp

import (
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"
)

// MCPManager unifiedofMCP manager，responsible forcoordinateallchild manager
type MCPManager struct {
	localManager  *LocalMCPManager
	globalManager *GlobalMCPManager
	// deviceManager will be managed heredevice managerpool

	mu      sync.RWMutex
	started bool
}

var (
	mcpManager *MCPManager
	mcpOnce    sync.Once
)

// GetMCPManager getunifiedMCP managersingleton
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

// Start startallMCP manager
func (m *MCPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		log.Warn("MCP manageralreadystart")
		return nil
	}

	log.Info("=== startMCP managercluster ===")

	// 1. firstfirststartlocal manager
	log.Info("startlocal MCP manager...")
	if err := m.localManager.Start(); err != nil {
		log.Errorf("startlocal MCP managerfailed: %v", err)
		return fmt.Errorf("startlocal MCP managerfailed: %v", err)
	}

	// 2. 然afterstartglobal manager
	log.Info("startglobal MCP manager...")
	if err := m.globalManager.Start(); err != nil {
		log.Errorf("startglobal MCP managerfailed: %v", err)
		return fmt.Errorf("startglobal MCP managerfailed: %v", err)
	}

	// 3. device managerthroughjoinwhendynamiccreate，这innoneedstart
	log.Info("device MCP managerwilldynamically created according to connection")

	m.started = true
	log.Info("=== MCP managerclusterstartcomplete ===")

	// outputstartstatecount
	m.printStartupStats()

	return nil
}

// Stop stopallMCP manager
func (m *MCPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		log.Info("MCP managernotstart，noneedstop")
		return nil
	}

	log.Info("=== stopMCP managercluster ===")

	// stop managers in reverse order
	// 1. stopglobal manager
	log.Info("stopglobal MCP manager...")
	if err := m.globalManager.Stop(); err != nil {
		log.Errorf("stopglobal MCP managerfailed: %v", err)
	}

	// 2. stoplocal manager
	log.Info("stoplocal MCP manager...")
	if err := m.localManager.Stop(); err != nil {
		log.Errorf("stoplocal MCP managerfailed: %v", err)
	}

	// 3. device managerthroughjoindisconnectautomaticcleanup
	log.Info("deviceMCPjoinwillautomaticcleanup")

	m.started = false
	log.Info("=== MCP managerclusteralreadystop ===")
	return nil
}

// IsStarted check if manager already started
func (m *MCPManager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetLocalManager getlocal manager
func (m *MCPManager) GetLocalManager() *LocalMCPManager {
	return m.localManager
}

// GetGlobalManager getglobal manager
func (m *MCPManager) GetGlobalManager() *GlobalMCPManager {
	return m.globalManager
}

// printStartupStats outputstartstatecount
func (m *MCPManager) printStartupStats() {
	localToolCount := m.localManager.GetToolCount()
	globalToolCount := len(m.globalManager.GetAllTools())

	log.Infof("MCP managerstartcount:")
	log.Infof("  - localtoolcount: %d", localToolCount)
	log.Infof("  - globaltoolcount: %d", globalToolCount)
	log.Infof("  - device manager: dynamicmanage")
	log.Infof("  - totaltoolcount: %d", localToolCount+globalToolCount)
}

// GetAllManagersStatus get all managersofstateinfo
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

// RestartManager restart specified manager
func (m *MCPManager) RestartManager(managerType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("MCP managerclusternotstart")
	}

	switch managerType {
	case "local":
		log.Info("restartlocal MCP manager...")
		if err := m.localManager.Stop(); err != nil {
			log.Errorf("stoplocal managerfailed: %v", err)
		}
		if err := m.localManager.Start(); err != nil {
			return fmt.Errorf("restartlocal managerfailed: %v", err)
		}
		log.Info("local MCP managerrestartcomplete")

	case "global":
		log.Info("restartglobal MCP manager...")
		if err := m.globalManager.Stop(); err != nil {
			log.Errorf("stopglobal managerfailed: %v", err)
		}
		if err := m.globalManager.Start(); err != nil {
			return fmt.Errorf("restartglobal managerfailed: %v", err)
		}
		log.Info("global MCP managerrestartcomplete")

	default:
		return fmt.Errorf("unsupported manager type: %s", managerType)
	}

	return nil
}

// istoafter兼容，provide便捷function

// StartMCPManagers startallMCP manager（便捷function）
func StartMCPManagers() error {
	return GetMCPManager().Start()
}

// StopMCPManagers stopallMCP manager（便捷function）
func StopMCPManagers() error {
	return GetMCPManager().Stop()
}

// GetMCPManagerStatus getMCP managerstate（便捷function）
func GetMCPManagerStatus() map[string]interface{} {
	return GetMCPManager().GetAllManagersStatus()
}
