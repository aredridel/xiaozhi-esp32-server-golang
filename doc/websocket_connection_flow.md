# WebSocket Connection Flow Documentation

## Overview

This document describes the WebSocket connection and communication flow between `internal/domain/config/manager/websocket_client.go` and `websocket.go`.

## Architecture Design

### Role Definitions

1. **`internal/domain/config/manager/websocket_client.go`** - Main Server WebSocket Client
   - Acts as a client connecting to Manager Backend
   - Can send requests and receive responses
   - Supports bidirectional communication

2. **`websocket.go`** - Manager Backend WebSocket Server
   - Acts as a server receiving WebSocket connections from the main server
   - Handles requests sent by the main server
   - **Only keeps the last valid connection** (new connections will disconnect old ones)
   - Supports active message pushing

### Connection Flow

```
Main Server (internal/domain/config/manager/websocket_client.go)  →  Manager Backend (websocket.go)
       Client                          Server (Single Connection)
```

## Detailed Flow

### 1. Establishing Connection

#### Manager Backend Starts WebSocket Server
```go
// In websocket.go
controller := NewWebSocketController(db)
// Register in router
router.GET("/ws", controller.HandleWebSocket)
```

#### Main Server Connects to Manager Backend
```go
// In internal/domain/config/manager/websocket_client.go
client := manager.NewWebSocketClient()
err := client.Connect(ctx)
```

Connection URL format:
- If configured as `http://localhost:8080`
- Actually connects to `ws://localhost:8080/ws`

**Important**: If there is a new connection request, Manager Backend will automatically disconnect the existing connection, keeping only the latest connection.

### 2. Request Tool List Flow

#### Main Server Requests MCP Tool List
```go
// In internal/domain/config/manager/websocket_client.go
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend Processes Request
```go
// In websocket.go
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)
    
    // Get tool list logic
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }
    
    client.sendResponse(request.ID, 200, response, "")
}
```

### 3. Bidirectional Communication Support

### Client → Server (Original Function)
#### Main Server Requests MCP Tool List
```go
// In internal/domain/config/manager/websocket_client.go
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "some_agent_id",
})
```

#### Manager Backend Processes Request
```go
// In websocket.go
func (client *WebSocketClient) handleMcpToolListRequest(request *WebSocketRequest) {
    agentID := request.Body["agent_id"].(string)
    
    // Get tool list logic
    response := map[string]interface{}{
        "agent_id": agentID,
        "tools":    []string{"tool1", "tool2", "tool3"},
        "count":    3,
    }
    
    client.sendResponse(request.ID, 200, response, "")
}
```

### Server → Client (New Function)
#### Manager Backend Actively Requests Client
```go
// In websocket.go
func (ctrl *WebSocketController) RequestMcpToolsFromClient(ctx context.Context, agentID string) (*WebSocketResponse, error) {
    body := map[string]interface{}{
        "agent_id": agentID,
    }
    return ctrl.SendRequestToClient(ctx, "GET", "/api/mcp/tools", body)
}

// Request client server info
func (ctrl *WebSocketController) RequestServerInfoFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/info", nil)
}

// Request client ping
func (ctrl *WebSocketController) RequestPingFromClient(ctx context.Context) (*WebSocketResponse, error) {
    return ctrl.SendRequestToClient(ctx, "GET", "/api/server/ping", nil)
}
```

#### Client Processes Server Request
```go
// In internal/domain/config/manager/websocket_client.go
client.SetRequestHandler(func(request *WebSocketRequest) {
    // Process received request
    switch request.Path {
    case "/api/mcp/tools":
        // Process MCP tool list request
        c.handleMcpToolListRequest(request)
    case "/api/server/info":
        // Process server info request
        c.handleServerInfoRequest(request)
    case "/api/server/ping":
        // Process ping request
        c.handlePingRequest(request)
    }
})
```

### Complete Bidirectional Communication Example
```go
// 1. Client connects to server
client := manager.NewWebSocketClient()
err := client.Connect(ctx)

// 2. Client sets request handler
client.SetRequestHandler(func(request *WebSocketRequest) {
    // Process requests from server
    // And send response
})

// 3. Client actively requests server
response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
    "agent_id": "agent_123",
})

// 4. Server actively requests client
serverResponse, err := websocketController.RequestMcpToolsFromClient(ctx, "agent_456")

// 5. Bidirectional communication complete
```

## Message Format

### Request Message (WebSocketRequest)
```json
{
    "id": "uuid-string",
    "method": "GET",
    "path": "/api/mcp/tools",
    "body": {
        "agent_id": "agent_123"
    }
}
```

### Response Message (WebSocketResponse)
```json
{
    "id": "uuid-string",
    "status": 200,
    "body": {
        "agent_id": "agent_123",
        "tools": ["tool1", "tool2", "tool3"],
        "count": 3
    },
    "error": ""
}
```

### Ping/Pong Messages
```json
// Ping
{"ping": 1640995200}

// Pong
{"pong": 1640995200}
```

## Connection Management

### Single Connection Strategy
- **Only keeps the last valid connection**
- New connections automatically disconnect existing connections
- Simplifies connection management logic
- Suitable for one-to-one communication scenarios

### Connection Status Monitoring
```go
// Check if there is a connected client
func (ctrl *WebSocketController) HasConnectedClient() bool

// Get current connected client
func (ctrl *WebSocketController) GetCurrentClient() *WebSocketClient
```

### Connection Switching Logic
```go
// In HandleWebSocket
if ctrl.currentClient != nil && ctrl.currentClient.isConnected {
    log.Printf("Disconnecting existing connection: %s", ctrl.currentClient.ID)
    ctrl.currentClient.conn.Close()
    ctrl.currentClient.isConnected = false
}

// Set new connection as current client
ctrl.currentClient = client
```

## Error Handling

### Connection Errors
- Automatic heartbeat detection
- Automatic disconnection on timeout
- Automatic cleanup on connection anomalies
- New connections automatically replace old ones

### Message Errors
- Message format validation
- Error response return
- Logging

## Configuration Requirements

### Main Server Configuration
```yaml
manager:
  backend_url: "http://localhost:8080"
```

### Manager Backend Configuration
```go
// Register WebSocket endpoint in router
router.GET("/ws", websocketController.HandleWebSocket)
```

## Testing Recommendations

1. **Connection Testing**
   - Verify WebSocket connection establishment
   - Test new connection disconnecting old connection
   - Test connection disconnection and reconnection

2. **Function Testing**
   - Test MCP tool list request
   - Verify bidirectional communication
   - Test message pushing

3. **Error Testing**
   - Network disconnection and reconnection
   - Invalid message handling
   - Timeout handling
   - Heartbeat timeout
   - Connection switching

## Notes

1. **Single Connection Limit**
   - Only one active connection allowed at a time
   - New connections will forcefully disconnect old ones
   - Suitable for master-slave architecture, not for multi-client scenarios

2. **Concurrency Safety**
   - Use read-write locks to protect current client reference
   - Safe client switching
   - Thread-safe message sending

3. **Resource Management**
   - Timely cleanup of disconnected connections
   - Properly close WebSocket connections
   - Avoid memory leaks

4. **Heartbeat Mechanism**
   - Send ping every 30 seconds
   - Automatically disconnect if no response within 60 seconds
   - Support ping/pong messages

5. **Logging**
   - Log connection status changes
   - Log connection switching
   - Log request and response information
   - Log errors and exceptions

## Complete Usage Example

### Bidirectional Communication Test Code
```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "xiaozhi-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()
    
    // 1. Create client and connect
    client := manager.NewWebSocketClient()
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer client.Disconnect()
    
    // 2. Set request handler (process requests from server)
    client.SetRequestHandler(func(request *manager.WebSocketRequest) {
        log.Printf("Received server request: %s %s", request.Method, request.Path)
        
        switch request.Path {
        case "/api/mcp/tools":
            // Process MCP tool list request
            agentID := ""
            if request.Body != nil {
                if id, ok := request.Body["agent_id"].(string); ok {
                    agentID = id
                }
            }
            
            response := map[string]interface{}{
                "agent_id": agentID,
                "tools":    []string{"client_tool_1", "client_tool_2"},
                "count":    2,
            }
            
            client.SendResponse(request.ID, 200, response, "")
            
        case "/api/server/info":
            response := map[string]interface{}{
                "server_name": "xiaozhi-client",
                "version":     "1.0.0",
                "uptime":      time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")
            
        case "/api/server/ping":
            response := map[string]interface{}{
                "message": "pong from client",
                "time":    time.Now().Format(time.RFC3339),
            }
            client.SendResponse(request.ID, 200, response, "")
        }
    })
    
    // 3. Client actively requests server
    fmt.Println("=== Client requests server ===")
    response, err := client.SendRequest(ctx, "GET", "/api/mcp/tools", map[string]interface{}{
        "agent_id": "client_agent_123",
    })
    if err != nil {
        log.Printf("Client request failed: %v", err)
    } else {
        fmt.Printf("Server response: %+v\n", response)
    }
    
    // 4. Wait for a while to give server a chance to send requests
    fmt.Println("Waiting for server requests...")
    time.Sleep(5 * time.Second)
    
    fmt.Println("Bidirectional communication test completed!")
}
```

### Server-side Test Code
```go
// In Manager Backend
func testBidirectionalCommunication() {
    ctx := context.Background()
    
    // 1. Check client connection status
    status := websocketController.GetClientConnectionStatus()
    fmt.Printf("Client status: %+v\n", status)
    
    // 2. Server actively requests client
    fmt.Println("=== Server requests client ===")
    
    // Request MCP tool list
    response, err := websocketController.RequestMcpToolsFromClient(ctx, "server_agent_456")
    if err != nil {
        log.Printf("Request MCP tool list failed: %v", err)
    } else {
        fmt.Printf("Client MCP tool response: %+v\n", response)
    }
    
    // Request server info
    infoResponse, err := websocketController.RequestServerInfoFromClient(ctx)
    if err != nil {
        log.Printf("Request server info failed: %v", err)
    } else {
        fmt.Printf("Client server info: %+v\n", infoResponse)
    }
    
    // Request ping
    pingResponse, err := websocketController.RequestPingFromClient(ctx)
    if err != nil {
        log.Printf("Request ping failed: %v", err)
    } else {
        fmt.Printf("Client ping response: %+v\n", pingResponse)
    }
}
```

## Notes

1. **Bidirectional Communication Requirements**
   - Client must set request handler
   - Both server and client must implement corresponding request processing methods
   - Request ID must match to ensure correct response routing

2. **Error Handling**
   - Bidirectional communication will fail when network disconnects
   - Timeout handling is important
   - Connection status checking is essential

3. **Performance Considerations**
   - Avoid frequent bidirectional requests
   - Set reasonable timeout times
   - Monitor connection status
