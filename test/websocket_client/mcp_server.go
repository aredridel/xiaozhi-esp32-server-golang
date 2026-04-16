package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type McpInterface interface {
	SendMcpMsg(payload json.RawMessage) error
	RecvMcpMsg(timeOut int) ([]byte, error)
}

type McpTransport struct {
	SendMsgChan chan []byte
	RecvMsgChan chan []byte
}

func (c *McpTransport) SendMcpMsg(payload json.RawMessage) error {
	serverMsg := ServerMessage{
		Type:    MessageTypeMcp,
		PayLoad: payload,
	}
	serverBytes, err := json.Marshal(serverMsg)
	if err != nil {
		return err
	}
	select {
	case c.SendMsgChan <- serverBytes:
		return nil
	case <-time.After(time.Duration(2000) * time.Millisecond):
		return fmt.Errorf("mcp send message timeout")
	}
}

func (c *McpTransport) RecvMcpMsg(timeOut int) ([]byte, error) {
	select {
	case msg := <-c.RecvMsgChan:
		return msg, nil
	case <-time.After(time.Duration(timeOut) * time.Millisecond):
		return nil, fmt.Errorf("mcp receive message timeout")
	}
}

func NewMcpServer(sendMsgChan chan []byte, recvMsgChan chan []byte) {
	/*
		hooks := &server.Hooks{}

		hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
			result.ServerInfo.Name = "taiji-pi-s3"
			result.ServerInfo.Version = "1.0.0"
			fmt.Printf("afterInitialize: %v, %v, %v\n", id, message, result)
		})*/

	s := server.NewMCPServer("taiji-pi-s3", "1.0.0")

	// Add tool
	/*tool := mcp.NewTool("hello_world",
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

	// Add random number generation tool (parameter type is number, converted internally by handler)
	randomNumberTool := mcp.NewTool("random_number",
		mcp.WithDescription("Generate random integer in specified range"),
		mcp.WithNumber("min",
			mcp.Required(),
			mcp.Description("Minimum value"),
		),
		mcp.WithNumber("max",
			mcp.Required(),
			mcp.Description("Maximum value"),
		),
	)



	// Register all tools and their handlers
	s.AddTool(tool, helloHandler)
	s.AddTool(weatherTool, queryWeatherHandler)
	s.AddTool(randomNumberTool, randomNumberHandler)*/

	// Add joke telling tool
	jokeTool := mcp.NewTool("tell_joke",
		mcp.WithDescription("Tell a joke"),
	)
	s.AddTool(jokeTool, jokeHandler)

	// Add vision analysis tool
	visionTool := mcp.NewTool("vision_tool",
		mcp.WithDescription("Take photo and analyze image"),
	)
	s.AddTool(visionTool, visionHandler)

	mcpHandle := &McpTransport{
		SendMsgChan: sendMsgChan,
		RecvMsgChan: recvMsgChan,
	}

	transport, err := NewWebSocketServerTransport(mcpHandle, WithWebSocketServerOptionMcpServer(s))
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

// Random number generation handler
func randomNumberHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Re-implement
	min := request.GetInt("min", 0)
	max := request.GetInt("max", 100)

	if min > max {
		return mcp.NewToolResultError("min cannot be greater than max"), nil
	}
	rnd := min
	if max > min {
		rnd = min + int(time.Now().UnixNano()%int64(max-min+1))
	}
	return mcp.NewToolResultText(fmt.Sprintf("Random number: %d", rnd)), nil
}

// Joke telling handler
func jokeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	joke := "One day, a student went to school. The teacher asked why he was late. The student replied: The homework was too difficult. I was doing it in my dream, and when I woke up, I was already late."
	return mcp.NewToolResultText(joke), nil
}

func visionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	image := "1.jpg"
	question := "What is in the image?"
	url := GetServerVisionURL()
	if url == "" {
		url = "http://192.168.208.214:8989/xiaozhi/api/vision" // Use default when not received from server
	}
	deviceId := "shijingbo"
	responseText, err := requestVllm(image, question, url, deviceId)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(responseText), nil
}
