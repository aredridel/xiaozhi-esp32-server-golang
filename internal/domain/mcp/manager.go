package mcp

import (
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"
)

// MCPManager unified MCP manager, responsible for coordinating all child managers
type MCPManager struct {
	localManager  *LocalMCPManager
	globalManager *GlobalMCPManager
	// deviceManager will be managed here device manager pool

	mu      sync.RWMutex
	started bool
}

var (
	mcpManager *MCPManager
	mcpOnce    sync.Once
)

// GetMCPManager get unified MCP manager singleton
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

// Start start all MCP manager
func (m *MCPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		log.Warn("MCP manager already started")
		return nil
	}

	log.Info("=== start MCP manager cluster ===")

	// 1. first start local manager
	log.Info("start local MCP manager...")
	if err := m.localManager.Start(); err != nil {
		log.Errorf("start local MCP manager failed: %v", err)
		return fmt.Errorf("start local MCP manager failed: %v", err)
	}

	// 2. then start global manager
	log.Info("start global MCP manager...")
	if err := m.globalManager.Start(); err != nil {
		log.Errorf("start global MCP manager failed: %v", err)
		return fmt.Errorf("start global MCP manager failed: %v", err)
	}

	// 3. device manager dynamically created through connection, no need to start here
	log.Info("device MCP manager will be dynamically created according to connection")

	m.started = true
	log.Info("=== MCP manager cluster start complete ===")

	// output start state count
	m.printStartupStats()

	return nil
}

// Stop stop all MCP manager
func (m *MCPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		log.Info("MCP manager not started, no need to stop")
		return nil
	}

	log.Info("=== stop MCP manager cluster ===")

	// stop managers in reverse order
	// 1. stop global manager
	log.Info("stop global MCP manager...")
	if err := m.globalManager.Stop(); err != nil {
		log.Errorf("stop global MCP manager failed: %v", err)
	}

	// 2. stop local manager
	log.Info("stop local MCP manager...")
	if err := m.localManager.Stop(); err != nil {
		log.Errorf("stop local MCP manager failed: %v", err)
	}

	// 3. device manager automatically cleanup through connection disconnect
	log.Info("device MCP connection will automatically cleanup")

	m.started = false
	log.Info("=== MCP manager cluster already stopped ===")
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

// printStartupStats output start state count
func (m *MCPManager) printStartupStats() {
	localToolCount := m.localManager.GetToolCount()
	globalToolCount := len(m.globalManager.GetAllTools())

	log.Infof("MCP manager start count:")
	log.Infof("  - local tool count: %d", localToolCount)
	log.Infof("  - global tool count: %d", globalToolCount)
	log.Infof("  - device manager: dynamic manage")
	log.Infof("  - total tool count: %d", localToolCount+globalToolCount)
}

// GetAllManagersStatus get all managers state info
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
		return fmt.Errorf("MCP manager cluster not started")
	}

	switch managerType {
	case "local":
		log.Info("restart local MCP manager...")
		if err := m.localManager.Stop(); err != nil {
			log.Errorf("stop local manager failed: %v", err)
		}
		if err := m.localManager.Start(); err != nil {
			return fmt.Errorf("restart local manager failed: %v", err)
		}
		log.Info("local MCP manager restart complete")

	case "global":
		log.Info("restart global MCP manager...")
		if err := m.globalManager.Stop(); err != nil {
			log.Errorf("stop global manager failed: %v", err)
		}
		if err := m.globalManager.Start(); err != nil {
			return fmt.Errorf("restart global manager failed: %v", err)
		}
		log.Info("global MCP manager restart complete")

	default:
		return fmt.Errorf("unsupported manager type: %s", managerType)
	}

	return nil
}

// for backward compatibility, provide convenient functions

// StartMCPManagers start all MCP manager (convenient function)
func StartMCPManagers() error {
	return GetMCPManager().Start()
}

// StopMCPManagers stop all MCP manager (convenient function)
func StopMCPManagers() error {
	return GetMCPManager().Stop()
}

// GetMCPManagerStatus get MCP manager state (convenient function)
func GetMCPManagerStatus() map[string]interface{} {
	return GetMCPManager().GetAllManagersStatus()
}
