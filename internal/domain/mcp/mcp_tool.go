package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// LocalToolHandler local tool process function type
type LocalToolHandler func(ctx context.Context, argumentsInJSON string) (string, error)

// mcpTool MCP tool implementation, support remote and local tools
type McpTool struct {
	info       *schema.ToolInfo
	serverName string
	client     *client.Client

	// local tool support
	isLocal      bool
	localHandler LocalToolHandler
}

// Info get tool info, implement BaseTool interface
func (t *McpTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *McpTool) InvokeableLocalRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	toolInfo := t.info
	if t.localHandler == nil {
		return "", fmt.Errorf("local tool %s process function not defined", toolInfo.Name)
	}

	log.Infof("execute local tool: %s, parameter: %s", toolInfo.Name, argumentsInJSON)

	resultStr, err := t.localHandler(ctx, argumentsInJSON)
	if err != nil {
		log.Errorf("local tool %s execute failed: %v", toolInfo.Name, err)
		return "", fmt.Errorf("local tool execute failed: %v", err)
	}
	if len(resultStr) > 2048 {
		log.Infof("local tool %s execute successful, result length: %d", toolInfo.Name, len(resultStr))
	} else {
		log.Infof("local tool %s execute successful, result: %+s", toolInfo.Name, resultStr)
	}

	return resultStr, nil
}

// InvokableRun call tool, implement InvokableTool interface
func (t *McpTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// if is local tool, directly call local process function
	if t.isLocal {
		return t.InvokeableLocalRun(ctx, argumentsInJSON, opts...)
	}

	retContent := ""

	// remote MCP tool call logic
	// check if client is available
	if t.client == nil {
		return retContent, fmt.Errorf("call MCP tool failed: MCP client not initialized")
	}

	// parse parameter
	var arguments map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &arguments); err != nil {
			return retContent, fmt.Errorf("parse tool parameter failed: %v", err)
		}
	}

	// prepare call request
	callRequest := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      t.info.Name,
			Arguments: arguments,
		},
	}

	// first time try call
	result, err := t.client.CallTool(ctx, callRequest)
	if err != nil && isSessionClosedError(err) {
		log.Warnf("tool %s call failed(session closed): %v, try reconnect after retry", t.info.Name, err)

		// reconnect and get new client
		newClient, err := GetGlobalMCPManager().reconnectServer(t.serverName)
		if err != nil {
			return retContent, fmt.Errorf("reconnect server failed: %v", err)
		}

		// update tool's client reference
		t.client = newClient

		// retry call
		result, err = t.client.CallTool(ctx, callRequest)
		if err != nil {
			return retContent, fmt.Errorf("reconnect after call still failed: %v", err)
		}
	} else if err != nil {
		return retContent, fmt.Errorf("call tool failed: %v", err)
	}

	resultStr, err := result.MarshalJSON()
	if err != nil {
		return retContent, fmt.Errorf("tool call return content convert failed: %v", err)
	}

	return string(resultStr), nil
}

func (t *McpTool) GetClient() *client.Client {
	return t.client
}

func (t *McpTool) GetServerName() string {
	return t.serverName
}
