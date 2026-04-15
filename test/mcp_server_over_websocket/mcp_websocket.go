package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Define command line parameters
	var endPoint string
	flag.StringVar(&endPoint, "endpoint", "", "WebSocket endpoint URL (required)")
	flag.Parse()

	// Check required parameters
	if endPoint == "" {
		fmt.Println("Error: endpoint parameter is required")
		fmt.Println("Usage: go run . -endpoint <websocket_url>")
		fmt.Println("Example: go run . -endpoint ws://192.168.208.214:8989/mcp?token=xxx")
		os.Exit(1)
	}

	// Commented out hardcoded endpoint
	//endPoint := "wss://api.xiaozhi.me/mcp/?token=eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE0NDQzNSwiYWdlbnRJZCI6MzUxNjQsImVuZHBvaW50SWQiOiJhZ2VudF8zNTE2NCIsInB1cnBvc2UiOiJtY3AtZW5kcG9pbnQiLCJpYXQiOjE3NDk1NDk2MzR9.nPMAHaYyRrxQGqHnzFk-SqLDb61p3YGJqRsQ3TZZEqPxQgef0jg_fTLiZsTNVI34VaNOaOobvKnl55VoIuYx7w"
	//endPoint := "ws://192.168.208.214:8989/mcp?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjEsImFnZW50SWQiOiIxIiwiZW5kcG9pbnRJZCI6ImFnZW50XzEiLCJwdXJwb3NlIjoibWNwLWVuZHBvaW50IiwiZXhwIjoxNzU2OTczNDM0LCJpYXQiOjE3NTY4ODcwMzR9.igLC-IFSgaf9maZljD-Tq3tI8nUmhx4vaOBcIsAHrRs"

	fmt.Printf("Connecting to WebSocket endpoint: %s\n", endPoint)
	s := server.NewMCPServer("mcp_websocket_server", "1.0.0")

	// Add tool
	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the person to greet"),
		),
	)

	// Add weather query tool
	weatherTool := mcp.NewTool("query_weather",
		mcp.WithDescription("Query weather"),
	)

	// Add tool handler
	s.AddTool(tool, helloHandler)
	// Register weather query tool
	s.AddTool(weatherTool, queryWeatherHandler)

	transport, err := NewWebSocketServerTransport(endPoint, WithWebSocketServerOptionMcpServer(s))
	if err != nil {
		log.Fatalf("Failed to create websocket server transport: %v", err)
	}
	transport.Run()
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}

// Weather query handler
func queryWeatherHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Sunny weather, 20 degrees, north wind level 3"), nil
}
