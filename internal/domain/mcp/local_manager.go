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

// LocalMCPManager localMCPtoolmanage器
type LocalMCPManager struct {
	tools map[string]*McpTool // toolname -> tool定义
	mu    sync.RWMutex        // readwritelockprotectedconcurrentaccess
}

var (
	localManager *LocalMCPManager
	localOnce    sync.Once
)

// GetLocalMCPManager getlocalMCPmanage器singleton
func GetLocalMCPManager() *LocalMCPManager {
	localOnce.Do(func() {
		localManager = &LocalMCPManager{
			tools: make(map[string]*McpTool),
		}
		// initializedefaultoflocaltool
		localManager.initDefaultTools()
	})
	return localManager
}

// initDefaultTools initializedefaultoflocaltool
func (l *LocalMCPManager) initDefaultTools() {

	log.Info("localMCPmanage器defaulttoolinitializecomplete")
}

// RegisterTool registerlocaltool
func (l *LocalMCPManager) RegisterTool(tool *McpTool) error {
	if tool == nil {
		return fmt.Errorf("toolcannot be empty")
	}

	if tool.info.Name == "" {
		return fmt.Errorf("toolnamecannot be empty")
	}

	if !tool.isLocal || tool.localHandler == nil {
		return fmt.Errorf("toolprocess functioncannot be empty")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// inspecttoolwhetheralready存at
	if _, exists := l.tools[tool.info.Name]; exists {
		log.Warnf("localtool %s already存at，willbe覆盖", tool.info.Name)
	}

	l.tools[tool.info.Name] = tool
	log.Infof("successfulregisterlocaltool: %s - %s", tool.info.Name, tool.info.Desc)
	return nil
}

func (l *LocalMCPManager) convertStructToOpenaipi3Schema(inputParams any) (*openapi3.Schema, error) {
	//usegithub.com/ThinkInAIXYZ/go-mcp throughstructgenerate tool, 然afterconvert成openapi3.Schema
	toolInstance, err := mcp_protocol.NewTool("get_system_info", "getsystem基本info", inputParams)
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

// RegisterToolFunc registertoolfunction（简化version）
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

// UnregisterTool unregistertool
func (l *LocalMCPManager) UnregisterTool(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.tools[name]; !exists {
		return fmt.Errorf("tool %s no存at", name)
	}

	delete(l.tools, name)
	log.Infof("successfulunregisterlocaltool: %s", name)
	return nil
}

// GetAllTools getalllocaltool，returnEinotoolinterfaceformat
func (l *LocalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, mcpTool := range l.tools {
		result[name] = mcpTool
	}
	return result
}

// GetToolByName according tonamegettool
func (l *LocalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	mcpTool, exists := l.tools[name]
	if !exists {
		return nil, false
	}

	return mcpTool, true
}

// GetToolNames getalltoolnamelist
func (l *LocalMCPManager) GetToolNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.tools))
	for name := range l.tools {
		names = append(names, name)
	}
	return names
}

// GetToolCount gettoolcount
func (l *LocalMCPManager) GetToolCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tools)
}

// Start startlocalmanage器（预留interface）
func (l *LocalMCPManager) Start() error {
	log.Info("localMCPmanage器alreadystart")
	return nil
}

// Stop stoplocalmanage器（预留interface）
func (l *LocalMCPManager) Stop() error {
	// 注意：我们nocleartool，becauseislocalmanage器oftoolshouldatbody个applicationprogram生命periodinsidekeepavailable
	// ifneedcleartool，should显式callUnregisterToolmethod
	log.Info("localMCPmanage器alreadystop")
	return nil
}
