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

// LocalToolHandler localtoolprocess functiontype
type LocalToolHandler func(ctx context.Context, argumentsInJSON string) (string, error)

// mcpTool MCPtoolimplement，supportremoteandlocaltool
type McpTool struct {
	info       *schema.ToolInfo
	serverName string
	client     *client.Client

	// localtoolsupport
	isLocal      bool
	localHandler LocalToolHandler
}

// Info gettoolinfo，implementBaseToolinterface
func (t *McpTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *McpTool) InvokeableLocalRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	toolInfo := t.info
	if t.localHandler == nil {
		return "", fmt.Errorf("localtool %s ofprocess functionnot定义", toolInfo.Name)
	}

	log.Infof("executelocaltool: %s, parameter: %s", toolInfo.Name, argumentsInJSON)

	resultStr, err := t.localHandler(ctx, argumentsInJSON)
	if err != nil {
		log.Errorf("localtool %s executefailed: %v", toolInfo.Name, err)
		return "", fmt.Errorf("localtoolexecutefailed: %v", err)
	}
	if len(resultStr) > 2048 {
		log.Infof("localtool %s executesuccessful，resultlength: %d", toolInfo.Name, len(resultStr))
	} else {
		log.Infof("localtool %s executesuccessful，result: %+s", toolInfo.Name, resultStr)
	}

	return resultStr, nil
}

// InvokableRun calltool，implementInvokableToolinterface
func (t *McpTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// ifyeslocaltool，directcalllocalprocess function
	if t.isLocal {
		return t.InvokeableLocalRun(ctx, argumentsInJSON, opts...)
	}

	retContent := ""

	// remoteMCPtoolcalllogical
	// inspectclient-sidewhetheravailable
	if t.client == nil {
		return retContent, fmt.Errorf("callMCPtoolfailed: MCPclient-sidenot initialized")
	}

	// parseparameter
	var arguments map[string]interface{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &arguments); err != nil {
			return retContent, fmt.Errorf("parsetoolparameterfailed: %v", err)
		}
	}

	// preparecallrequest
	callRequest := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      t.info.Name,
			Arguments: arguments,
		},
	}

	// firsttimestrycall
	result, err := t.client.CallTool(ctx, callRequest)
	if err != nil && isSessionClosedError(err) {
		log.Warnf("tool %s callfailed(session closed): %v，tryreconnectafterretry", t.info.Name, err)

		// reconnectandget newclient
		newClient, err := GetGlobalMCPManager().reconnectServer(t.serverName)
		if err != nil {
			return retContent, fmt.Errorf("reconnectserverfailed: %v", err)
		}

		// updatetoolofclientreference
		t.client = newClient

		// retrycall
		result, err = t.client.CallTool(ctx, callRequest)
		if err != nil {
			return retContent, fmt.Errorf("reconnectaftercallstill然failed: %v", err)
		}
	} else if err != nil {
		return retContent, fmt.Errorf("calltoolfailed: %v", err)
	}

	resultStr, err := result.MarshalJSON()
	if err != nil {
		return retContent, fmt.Errorf("toolcallreturninside容convertfailed: %v", err)
	}

	return string(resultStr), nil
}

func (t *McpTool) GetClient() *client.Client {
	return t.client
}

func (t *McpTool) GetServerName() string {
	return t.serverName
}
