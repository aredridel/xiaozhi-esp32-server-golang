package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	. "xiaozhi-esp32-server-golang/internal/domain/mcp"
)

// ExampleMCPInteractive interactive demonstration of how to use MCP Host
func main() {
	fmt.Println("=== MCP Host Interactive Usage Example ===")

	// 1. Configure MCP
	setupMCPConfig()

	// 2. Start global MCP manager
	globalManager := GetGlobalMCPManager()
	if err := globalManager.Start(); err != nil {
		log.Printf("Failed to start global MCP manager: %v", err)
		return
	}
	defer globalManager.Stop()

	// 3. Show global tools
	showGlobalTools(globalManager)

	// 4. Interactive wait for user input
	reader := bufio.NewReader(os.Stdin)
	missCount := 0
	for {
		fmt.Print("\nPlease enter the tool name to call (or exit to quit, ? to view tool list): ")
		toolName, _ := reader.ReadString('\n')
		toolName = strings.TrimSpace(toolName)
		if toolName == "exit" {
			fmt.Println("Exited interactive mode.")
			break
		}
		if toolName == "?" {
			showGlobalTools(globalManager)
			continue
		}
		tool, exists := globalManager.GetToolByName(toolName)
		if !exists {
			fmt.Printf("Tool not found: %s\n", toolName)
			missCount++
			if missCount >= 3 {
				fmt.Println("Tool not found 3 times in a row, automatically exiting interactive mode.")
				break
			}
			continue
		}
		missCount = 0 // Reset if tool found
		// Get parameter example and print
		info, err := tool.Info(context.Background())
		if err != nil {
			fmt.Printf("Failed to get tool info: %v\n", err)
			continue
		}
		fmt.Println("Parameter example:")
		if info.ParamsOneOf != nil {
			// Try to serialize to JSON for pretty output
			if b, err := json.MarshalIndent(info.ParamsOneOf, "", "  "); err == nil {
				fmt.Println(string(b))
			} else {
				fmt.Printf("%+v\n", info.ParamsOneOf)
			}
		} else {
			fmt.Println("  (No parameters or undefined)")
		}
		fmt.Print("Please enter parameters (JSON format): ")
		argsInJSON, _ := reader.ReadString('\n')
		argsInJSON = strings.TrimSpace(argsInJSON)
		fmt.Println("   Invoking tool...")
		result, err := tool.InvokableRun(context.Background(), argsInJSON)
		if err != nil {
			fmt.Printf("   ❌ Tool invocation failed: %v\n", err)
			continue
		}
		fmt.Printf("   ✓ Tool invocation successful: %s\n", result)
	}
}

// ExampleMCPUsage demonstrates how to use MCP Host
func ExampleMCPUsage(t *testing.T) {
	fmt.Println("=== MCP Host Usage Example ===")

	// 1. Configure MCP
	setupMCPConfig()

	// 2. Start global MCP manager
	globalManager := GetGlobalMCPManager()
	if err := globalManager.Start(); err != nil {
		log.Printf("Failed to start global MCP manager: %v", err)
		return
	}
	defer globalManager.Stop()

	// 3. Get device MCP manager
	deviceManager := GetDeviceMCPManager()

	// 4. Simulate waiting for tool registration
	time.Sleep(30 * time.Second)

	// 5. Show global tools
	showGlobalTools(globalManager)

	// 6. Show device tools
	showDeviceTools(deviceManager, "example_device")

	// 7. Demonstrate tool calling
	demonstrateToolCalling(globalManager)
}

// setupMCPConfig sets MCP configuration
func setupMCPConfig() {
	fmt.Println("1. Setting MCP configuration...")

	// Set global MCP configuration
	viper.Set("mcp.global.enabled", true)
	viper.Set("mcp.global.reconnect_interval", 5)
	viper.Set("mcp.global.max_reconnect_attempts", 3)

	// Set MCP server list
	servers := []map[string]interface{}{
		{
			"name":    "global_mcp",
			"sse_url": "http://192.168.208.214:3001/sse",
			"enabled": true,
		},
	}
	viper.Set("mcp.global.servers", servers)

	// Set device MCP configuration
	viper.Set("mcp.device.enabled", true)
	viper.Set("mcp.device.websocket_path", "/xiaozhi/mcp/")
	viper.Set("mcp.device.max_connections_per_device", 5)

	fmt.Println("   ✓ MCP configuration set")
}

// showGlobalTools shows global tools
func showGlobalTools(manager *GlobalMCPManager) {
	fmt.Println("\n2. Global tool list:")

	tools := manager.GetAllTools()
	if len(tools) == 0 {
		fmt.Println("   No global tools (need to connect to real MCP server)")
		return
	}

	for name, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			fmt.Printf("   ❌ %s: Failed to get info - %v\n", name, err)
			continue
		}
		fmt.Printf("   ✓ %s: %s,%+v\n", info.Name, info.Desc, info.ParamsOneOf)
	}
}

// showDeviceTools shows device tools
func showDeviceTools(manager *DeviceMCPManager, deviceID string) {
	fmt.Printf("\n3. Device %s tool list:\n", deviceID)

	tools := manager.GetDeviceTools(deviceID)
	if len(tools) == 0 {
		fmt.Println("   No device tools (device needs to connect to MCP WebSocket endpoint)")
		return
	}

	for name, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			fmt.Printf("   ❌ %s: Failed to get info - %v\n", name, err)
			continue
		}
		fmt.Printf("   ✓ %s: %s\n", info.Name, info.Desc)
	}
}

// demonstrateToolCalling demonstrates tool calling
func demonstrateToolCalling(manager *GlobalMCPManager) {
	fmt.Println("\n4. Tool invocation demonstration:")

	// Try to get a tool
	tool, exists := manager.GetToolByName("random")
	if !exists {
		fmt.Println("   No available tools for demonstration")
		return
	}

	argsInJSON := `{"min":1,"max":100}`
	fmt.Printf("argsInJSON: %s", argsInJSON)
	// Invoke tool
	fmt.Println("   Invoking tool...")
	result, err := tool.InvokableRun(
		context.Background(),
		argsInJSON,
	)

	if err != nil {
		fmt.Printf("   ❌ Tool invocation failed: %v\n", err)
		return
	}

	fmt.Printf("   ✓ Tool invocation successful: %s\n", result)
}

/*
// ExampleMCPTool demonstrates custom MCP tool
func ExampleMCPTool() {
	fmt.Println("=== Custom MCP Tool Example ===")

	// Create example tool
	tool := &mcpTool{
		name:        "example_tool",
		description: "This is an example tool",
		inputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message to process",
				},
			},
			"required": []string{"message"},
		},
		serverName: "example_server",
		client:     nil, // Real client needs to be provided in actual use
	}

	// Get tool information
	info, err := tool.Info(context.Background())
	if err != nil {
		fmt.Printf("Failed to get tool info: %v\n", err)
		return
	}

	fmt.Printf("Tool name: %s\n", info.Name)
	fmt.Printf("Tool description: %s\n", info.Desc)

	// Note: Tool invocation will fail without real client connection
	fmt.Println("Note: Tool invocation cannot be demonstrated without real MCP client connection")
}*/

// ExampleWebSocketClient demonstrates WebSocket client connection
func ExampleWebSocketClient() {
	fmt.Println("=== WebSocket Client Connection Example ===")

	fmt.Print(`
JavaScript client example:

const ws = new WebSocket('ws://localhost:8989/xiaozhi/mcp/device123');

ws.onopen = function() {
    console.log('MCP connection established');
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    console.log('Received message:', message);
    
    if (message.method === 'initialize') {
        // Respond to initialization
        ws.send(JSON.stringify({
            jsonrpc: "2.0",
            id: message.id,
            result: {
                protocolVersion: "2024-11-05",
                serverInfo: {
                    name: "device-mcp-server",
                    version: "1.0.0"
                }
            }
        }));
    }
};

ws.onerror = function(error) {
    console.error('WebSocket error:', error);
};

ws.onclose = function() {
    console.log('MCP connection closed');
};
`)
}
