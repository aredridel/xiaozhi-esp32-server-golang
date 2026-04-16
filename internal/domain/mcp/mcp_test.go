package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalMCPManager_Singleton(t *testing.T) {
	// test singleton pattern
	manager1 := GetGlobalMCPManager()
	manager2 := GetGlobalMCPManager()

	assert.Equal(t, manager1, manager2, "should return same instance")
}

func TestDeviceMCPManager_Singleton(t *testing.T) {
	t.Skip("GetDeviceMCPManager function not implemented yet")
	// // test singleton pattern
	// manager1 := GetDeviceMCPManager()
	// manager2 := GetDeviceMCPManager()
	//
	// assert.Equal(t, manager1, manager2, "should return same instance")
}

func TestGlobalMCPManager_StartStop(t *testing.T) {
	// set test config
	viper.Set("mcp.global.enabled", false)

	manager := GetGlobalMCPManager()

	// test start (disabled state)
	err := manager.Start()
	assert.NoError(t, err)

	// test stop
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestMCPTool_Info(t *testing.T) {
	tool := &McpTool{
		info: &schema.ToolInfo{
			Name: "test_tool",
			Desc: "test tool",
		},
		serverName: "test_server",
		client:     nil, // test does not need real client
	}

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "test_tool", info.Name)
	assert.Equal(t, "test tool", info.Desc)
}

func TestMCPTool_InvokableRun(t *testing.T) {
	tool := &McpTool{
		info: &schema.ToolInfo{
			Name: "test_tool",
			Desc: "test tool",
		},
		serverName: "test_server",
		client:     nil, // test does not need real client
	}

	// this test will fail because client is nil
	// but can validate method signature and basic logic
	_, err := tool.InvokableRun(context.Background(), `{"query": "test"}`)
	assert.Error(t, err)                                    // expected to have error because client is nil
	assert.Contains(t, err.Error(), "call MCP tool failed") // validate error message contains expected text
}

func TestDeviceMCPManager_GetDeviceTools(t *testing.T) {
	t.Skip("GetDeviceMCPManager function not implemented yet")
	// manager := GetDeviceMCPManager()
	//
	// // test get non-existent device tool
	// tools := manager.GetDeviceTools("non_existent_device")
	// assert.Empty(t, tools)
}

func TestGlobalMCPManager_GetAllTools(t *testing.T) {
	manager := GetGlobalMCPManager()

	// test get all tools (initial state should be empty)
	tools := manager.GetAllTools()
	assert.NotNil(t, tools)
}

func TestGlobalMCPManager_GetToolByName(t *testing.T) {
	manager := GetGlobalMCPManager()

	// test get non-existent tool
	tool, exists := manager.GetToolByName("non_existent_tool")
	assert.False(t, exists)
	assert.Nil(t, tool)
}

func TestMCPServerConfig_Structure(t *testing.T) {
	config := MCPServerConfig{
		Name:    "test_server",
		SSEUrl:  "http://localhost:3001/sse",
		Enabled: true,
	}

	assert.Equal(t, "test_server", config.Name)
	assert.Equal(t, "http://localhost:3001/sse", config.SSEUrl)
	assert.True(t, config.Enabled)
}

func TestReconnectConfig_Structure(t *testing.T) {
	config := ReconnectConfig{
		Interval:    5 * time.Second,
		MaxAttempts: 10,
	}

	assert.Equal(t, 5*time.Second, config.Interval)
	assert.Equal(t, 10, config.MaxAttempts)
}

// TestMCPGoStructures test mcp-go library struct usage
func TestMCPGoStructures(t *testing.T) {
	t.Run("InitializeRequest", func(t *testing.T) {
		initRequest := mcp.InitializeRequest{
			Request: mcp.Request{
				Method: string(mcp.MethodInitialize),
			},
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo: mcp.Implementation{
					Name:    "test-client",
					Version: "1.0.0",
				},
				Capabilities: mcp.ClientCapabilities{
					Experimental: make(map[string]any),
				},
			},
		}

		assert.Equal(t, string(mcp.MethodInitialize), initRequest.Request.Method)
		assert.Equal(t, "test-client", initRequest.Params.ClientInfo.Name)
	})

	t.Run("JSONRPCRequest", func(t *testing.T) {
		request := mcp.JSONRPCRequest{
			JSONRPC: mcp.JSONRPC_VERSION,
			ID:      mcp.NewRequestId(1),
			Request: mcp.Request{
				Method: string(mcp.MethodToolsList),
			},
		}

		assert.Equal(t, mcp.JSONRPC_VERSION, request.JSONRPC)
		assert.Equal(t, string(mcp.MethodToolsList), request.Request.Method)
	})

	t.Run("Tool", func(t *testing.T) {
		tool := mcp.NewTool(
			"test-tool",
			mcp.WithDescription("A test tool"),
		)

		assert.Equal(t, "test-tool", tool.Name)
		assert.Equal(t, "A test tool", tool.Description)
	})
}

// create test tool
func TestMCPTool_InvokableRun_NewTool(t *testing.T) {
	testTool := &McpTool{
		info: &schema.ToolInfo{
			Name: "test_tool",
			Desc: "test tool",
		},
		serverName: "test_server",
		client:     nil, // test does not need real client
	}

	// this test will fail because no real MCP server
	// but can validate method signature and basic logic
	_, err := testTool.InvokableRun(context.Background(), `{"query": "test"}`)
	assert.Error(t, err) // expected to have network error
}

func TestFilterMCPToolsByAllowList(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "alerts"},
		{Name: "forecast"},
		{Name: "history"},
	}

	filtered := filterMCPToolsByAllowList(tools, []string{"forecast", "alerts"})
	require.Len(t, filtered, 2)
	assert.Equal(t, "alerts", filtered[0].Name)
	assert.Equal(t, "forecast", filtered[1].Name)

	unfiltered := filterMCPToolsByAllowList(tools, nil)
	require.Len(t, unfiltered, 3)
}
