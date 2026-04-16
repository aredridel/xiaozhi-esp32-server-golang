package mcp

import (
	"fmt"
	"sync"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"

	mcp_protocol "github.com/ThinkInAIXYZ/go-mcp/protocol"
)

// LocalMCPManager local MCP tool manager
type LocalMCPManager struct {
	tools map[string]*McpTool // tool name -> tool definition
	mu    sync.RWMutex        // read-write lock protected concurrent access
}

var (
	localManager *LocalMCPManager
	localOnce    sync.Once
)

// GetLocalMCPManager get local MCP manager singleton
func GetLocalMCPManager() *LocalMCPManager {
	localOnce.Do(func() {
		localManager = &LocalMCPManager{
			tools: make(map[string]*McpTool),
		}
		// initialize default local tools
		localManager.initDefaultTools()
	})
	return localManager
}

// initDefaultTools initialize default local tools
func (l *LocalMCPManager) initDefaultTools() {

	log.Info("local MCP manager default tools initialize complete")
}

// RegisterTool register local tool
func (l *LocalMCPManager) RegisterTool(tool *McpTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be empty")
	}

	if tool.info.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if !tool.isLocal || tool.localHandler == nil {
		return fmt.Errorf("tool process function cannot be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// check if tool already exists
	if _, exists := l.tools[tool.info.Name]; exists {
		log.Warnf("local tool %s already exists, will be overwritten", tool.info.Name)
	}

	l.tools[tool.info.Name] = tool
	log.Infof("successfully register local tool: %s - %s", tool.info.Name, tool.info.Desc)
	return nil
}

func (l *LocalMCPManager) convertStructToOpenaipi3Schema(inputParams any) (*openapi3.Schema, error) {
	// use github.com/ThinkInAIXYZ/go-mcp through struct generate tool, then convert to openapi3.Schema
	toolInstance, err := mcp_protocol.NewTool("get_system_info", "get system basic info", inputParams)
	if err != nil {
		return nil, err
	}

	marshaledInputSchema, err := sonic.Marshal(toolInstance.InputSchema)
	if err != nil {
		return nil, err
	}

	inputSchema := &openapi3.Schema{}
	err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
	if err != nil {
		return nil, err
	}
	return inputSchema, nil
}

// RegisterToolFunc register tool function (simplified version)
func (l *LocalMCPManager) RegisterToolFunc(name, description string, inputParams any, handler LocalToolHandler) error {
	inputSchema, err := l.convertStructToOpenaipi3Schema(inputParams)
	if err != nil {
		log.Errorf("Failed to convert struct to openapi3 schema: %v", err)
		return err
	}
	tool := &McpTool{
		info: &schema.ToolInfo{
			Name:        name,
			Desc:        description,
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(inputSchema),
		},
		isLocal:      true,
		localHandler: handler,
	}
	return l.RegisterTool(tool)
}

// UnregisterTool unregister tool
func (l *LocalMCPManager) UnregisterTool(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.tools[name]; !exists {
		return fmt.Errorf("tool %s not exists", name)
	}

	delete(l.tools, name)
	log.Infof("successfully unregister local tool: %s", name)
	return nil
}

// GetAllTools get all local tools, return Eino tool interface format
func (l *LocalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpTool := range l.tools {
		result[name] = mcpTool
	}
	return result
}

// GetToolByName get tool by name
func (l *LocalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	mcpTool, exists := l.tools[name]
	if !exists {
		return nil, false
	}

	return mcpTool, true
}

// GetToolNames get all tool name list
func (l *LocalMCPManager) GetToolNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.tools))
	for name := range l.tools {
		names = append(names, name)
	}
	return names
}

// GetToolCount get tool count
func (l *LocalMCPManager) GetToolCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tools)
}

// Start start local manager (reserved interface)
func (l *LocalMCPManager) Start() error {
	log.Info("local MCP manager already started")
	return nil
}

// Stop stop local manager (reserved interface)
func (l *LocalMCPManager) Stop() error {
	// note: we don't clear tools, because local manager's tools should keep available throughout the entire application lifecycle
	// if need to clear tools, should explicitly call UnregisterTool method
	log.Info("local MCP manager already stopped")
	return nil
}
