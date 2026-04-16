package mcp

import (
	"strings"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

func parseSelectedMCPServiceNames(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	selected := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		selected[name] = struct{}{}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func isGlobalToolAllowed(toolKey string, selected map[string]struct{}) bool {
	if len(selected) == 0 {
		return true
	}
	for serviceName := range selected {
		if strings.HasPrefix(toolKey, serviceName+"_") {
			return true
		}
	}
	return false
}

func filterGlobalToolsBySelectedServices(globalTools map[string]tool.InvokableTool, selectedNames string) map[string]tool.InvokableTool {
	selected := parseSelectedMCPServiceNames(selectedNames)
	if len(selected) == 0 {
		result := make(map[string]tool.InvokableTool, len(globalTools))
		for name, invokable := range globalTools {
			result[name] = invokable
		}
		return result
	}

	result := make(map[string]tool.InvokableTool)
	for toolKey, invokable := range globalTools {
		if isGlobalToolAllowed(toolKey, selected) {
			result[toolKey] = invokable
		}
	}
	return result
}

func GetToolByName(deviceId string, agentId string, toolName string, selectedMCPServiceNames string) (tool.InvokableTool, bool) {
	// Priority: get from local manager first
	localManager := GetLocalMCPManager()
	tool, ok := localManager.GetToolByName(toolName)
	if ok {
		return tool, ok
	}

	// Otherwise get from global manager
	selected := parseSelectedMCPServiceNames(selectedMCPServiceNames)
	if len(selected) == 0 {
		tool, ok = globalManager.GetToolByName(toolName)
		if ok {
			return tool, ok
		}
	} else {
		globalTools := globalManager.GetAllTools()

		// Compatible with direct input "server_tool" scenario
		if invokable, exists := globalTools[toolName]; exists && isGlobalToolAllowed(toolName, selected) {
			return invokable, true
		}

		for serviceName := range selected {
			candidate := serviceName + "_" + toolName
			if invokable, exists := globalTools[candidate]; exists {
				return invokable, true
			}
		}
	}

	// Finally get from device MCP client pool
	tool, ok = mcpClientPool.GetToolByDeviceId(deviceId, toolName)
	if ok {
		return tool, true
	}
	// Compatible with AgentID reported MCP tool
	if agentId != "" && agentId != deviceId {
		tool, ok = mcpClientPool.GetToolByDeviceId(agentId, toolName)
		if ok {
			return tool, true
		}
	}
	return nil, false
}

func GetDeviceMcpClient(deviceId string) *DeviceMcpSession {
	return mcpClientPool.GetMcpClient(deviceId)
}

func AddDeviceMcpClient(deviceId string, mcpClient *DeviceMcpSession) error {
	mcpClientPool.AddMcpClient(deviceId, mcpClient)
	return nil
}

func RemoveDeviceMcpClient(deviceId string) error {
	mcpClientPool.RemoveMcpClient(deviceId)
	return nil
}

func GetToolsByDeviceId(deviceId string, agentId string, selectedMCPServiceNames string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)

	// Priority: get from local manager first
	localManager := GetLocalMCPManager()
	localTools := localManager.GetAllTools()
	for toolName, tool := range localTools {
		retTools[toolName] = tool
	}
	log.Infof("Got %d tools from local manager", len(localTools))

	// Then get from global manager
	globalTools := globalManager.GetAllTools()
	filteredGlobalTools := filterGlobalToolsBySelectedServices(globalTools, selectedMCPServiceNames)
	for toolName, tool := range filteredGlobalTools {
		// Local tool priority, if already exists same name tool then no override
		if _, exists := retTools[toolName]; !exists {
			retTools[toolName] = tool
		}
	}
	log.Infof("Got %d tools from global manager (after filter)", len(filteredGlobalTools))

	// Finally get from MCP client pool
	deviceTools, err := mcpClientPool.GetAllToolsByDeviceIdAndAgentId(deviceId, agentId)
	if err != nil {
		log.Errorf("Failed to get tools for device %s: %v", deviceId, err)
		return retTools, nil
	}
	for toolName, tool := range deviceTools {
		// Local tool and global tool priority, if already exists same name tool then no override
		if _, exists := retTools[toolName]; !exists {
			retTools[toolName] = tool
		}
	}
	log.Infof("Got %d tools from device %s", len(deviceTools), deviceId)
	log.Infof("Device %s total got %d tools", deviceId, len(retTools))

	return retTools, nil
}

func GetWsEndpointMcpTools(agentId string) (map[string]tool.InvokableTool, error) {
	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

// GetReportedToolsByDeviceID only get device reported MCP tools
func GetReportedToolsByDeviceID(deviceId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if deviceId == "" {
		return retTools, nil
	}

	client := mcpClientPool.GetMcpClient(deviceId)
	if client == nil {
		return retTools, nil
	}

	for toolName, invokable := range client.GetTools() {
		retTools[toolName] = invokable
	}

	return retTools, nil
}

// GetReportedToolsByAgentID only get agent (WebSocket endpoint) reported MCP tools
func GetReportedToolsByAgentID(agentId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	if agentId == "" {
		return retTools, nil
	}

	return mcpClientPool.GetWsEndpointMcpTools(agentId)
}

// GetReportedToolByDeviceIDAndName only find in device reported tools
func GetReportedToolByDeviceIDAndName(deviceId, toolName string) (tool.InvokableTool, bool) {
	reportedTools, err := GetReportedToolsByDeviceID(deviceId)
	if err != nil {
		log.Errorf("Failed to get device reported MCP tool: device=%s err=%v", deviceId, err)
		return nil, false
	}

	invokable, ok := reportedTools[toolName]
	return invokable, ok
}

// GetReportedToolByAgentIDAndName only find in agent reported tools
func GetReportedToolByAgentIDAndName(agentId, toolName string) (tool.InvokableTool, bool) {
	reportedTools, err := GetReportedToolsByAgentID(agentId)
	if err != nil {
		log.Errorf("Failed to get agent reported MCP tool: agent=%s err=%v", agentId, err)
		return nil, false
	}

	invokable, ok := reportedTools[toolName]
	return invokable, ok
}

// GetReportedToolsByDeviceIdAndAgentId compatible method: clearly separate device/agent query, no longer mixed use
func GetReportedToolsByDeviceIdAndAgentId(deviceId string, agentId string) (map[string]tool.InvokableTool, error) {
	if deviceId != "" {
		return GetReportedToolsByDeviceID(deviceId)
	}
	if agentId != "" {
		return GetReportedToolsByAgentID(agentId)
	}
	return make(map[string]tool.InvokableTool), nil
}

// GetReportedToolByName compatible method: separate by dimension, no longer mixed use
func GetReportedToolByName(deviceId string, agentId string, toolName string) (tool.InvokableTool, bool) {
	if deviceId != "" {
		return GetReportedToolByDeviceIDAndName(deviceId, toolName)
	}
	if agentId != "" {
		return GetReportedToolByAgentIDAndName(agentId, toolName)
	}
	return nil, false
}

func GetAudioResourceByTool(tool McpTool, resourceLink mcp_go.ResourceLink) (mcp_go.ReadResourceResult, error) {
	/*client := tool.GetClient()
	resourceRequest := mcp_go.ReadResourceRequest{
		Request: mcp_go.Request{
			Params: mcp_go.ReadResourceParams{
				URI: resourceLink.URL,
			},
		},
	}
	client.ReadResource(context.Background(), resourceRequest)*/
	return mcp_go.ReadResourceResult{}, nil
}
