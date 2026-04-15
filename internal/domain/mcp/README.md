# MCP Host Implementation

MCP (Model Context Protocol) Host implementation based on the [Eino Framework](https://github.com/cloudwego/eino), supporting global and device-level tool management.

## Features

### 🌐 Global MCP Tool Management
- Connect to multiple MCP servers via SSE
- Automatic tool discovery and registration
- Connection status monitoring and auto-reconnect
- Tool invocation proxy

### 📱 Device-Level MCP Management  
- Independent MCP connections for each device
- WebSocket protocol support
- Device-specific tool registration
- Connection limit and cleanup

### 🔧 Eino Framework Integration
- Implements `tool.InvokableTool` interface
- Supports Eino native tool invocation
- Full type safety
- Streaming processing support

## Architecture Design

```
┌─────────────────────────────────────────────────────────────┐
│                    WebSocket Server                        │
│  /xiaozhi/mcp/{deviceId} - Device MCP Connection                      │
│  /xiaozhi/api/mcp/tools/{deviceId} - Tool List API            │
└─────────────────────────────────────────────────────────────┘
                               │
                     ┌─────────┴─────────┐
                     ▼                   ▼
┌─────────────────────────┐  ┌─────────────────────────┐
│   GlobalMCPManager      │  │   DeviceMCPManager      │
│   • SSE Connection Management        │  │   • WebSocket Connection Management   │
│   • Global Tool Registration        │  │   • Device Tool Registration         │
│   • Auto Reconnect           │  │   • Connection Cleanup            │
└─────────────────────────┘  └─────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                    Eino Tool Interface                     │
│  tool.InvokableTool - Unified Tool Invocation Interface                      │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### config.json Configuration

```json
{
  "mcp": {
    "global": {
      "enabled": true,
      "servers": [
        {
          "name": "filesystem",
          "sse_url": "http://localhost:3001/sse",
          "enabled": true
        },
        {
          "name": "memory", 
          "sse_url": "http://localhost:3002/sse",
          "enabled": false
        }
      ],
      "reconnect_interval": 5,
      "max_reconnect_attempts": 10
    },
    "device": {
      "enabled": true,
      "websocket_path": "/xiaozhi/mcp/",
      "max_connections_per_device": 5
    }
  }
}
```

### Configuration Parameter Description

| Parameter | Type | Description |
|------|------|------|
| `mcp.global.enabled` | bool | Whether to enable global MCP manager |
| `mcp.global.servers` | array | MCP server list |
| `mcp.global.reconnect_interval` | int | Reconnect interval (seconds) |
| `mcp.global.max_reconnect_attempts` | int | Maximum reconnect attempts |
| `mcp.device.enabled` | bool | Whether to enable device MCP manager |
| `mcp.device.websocket_path` | string | WebSocket path prefix |
| `mcp.device.max_connections_per_device` | int | Maximum connections per device |

## API Interface

### WebSocket Endpoint

#### Device MCP Connection
```
ws://localhost:8989/xiaozhi/mcp/{deviceId}
```

**Connection Flow:**
1. Client connects to WebSocket endpoint
2. Server sends initialization message
3. Client responds with tool list
4. Establish bidirectional communication

**Message Format:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "id": 1,
  "params": {}
}
```

### REST API

#### Get Device Tool List
```http
GET /xiaozhi/api/mcp/tools/{deviceId}
```

**Response Example:**
```json
{
  "deviceId": "device123",
  "tools": {
    "filesystem_read_file": {
      "name": "read_file",
      "description": "Read file content",
      "type": "global"
    },
    "device_sensor_data": {
      "name": "sensor_data", 
      "description": "Get sensor data",
      "type": "device"
    }
  },
  "globalCount": 5,
  "deviceCount": 3,
  "totalCount": 8,
  "timestamp": 1704067200
}
```

## Usage Examples

### 1. Start Server

```go
package main

import (
    "xiaozhi-esp32-server-golang/internal/app/server/websocket"
)

func main() {
    server := websocket.NewWebSocketServer(8989)
    server.Start()
}
```

### 2. Connect MCP Server

MCP servers need to provide SSE endpoints, supporting the following events:

- `tools` - Tool list update
- `status` - Connection status update

### 3. Device Connection Example

```javascript
// Device-side WebSocket connection
const ws = new WebSocket('ws://localhost:8989/xiaozhi/mcp/device123');

ws.onopen = function() {
    console.log('MCP connection already established');
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
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
```

### 4. Tool Invocation Example

```go
// Get global tools
globalManager := mcp.GetGlobalMCPManager()
tools := globalManager.GetAllTools()

// Invoke tool
for name, tool := range tools {
    result, err := tool.InvokableRun(
        context.Background(),
        `{"path": "/tmp/test.txt"}`,
    )
    if err != nil {
        log.Errorf("Tool invocation failed: %v", err)
        continue
    }
    log.Infof("Tool %s result: %s", name, result)
}
```

## Development Guide

### Implement Custom MCP Tool

```go
type customTool struct {
    name        string
    description string
}

func (t *customTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: t.name,
        Desc: t.description,
        ParamsOneOf: &schema.ParamsOneOf{
            // Parameter definitions
        },
    }, nil
}

func (t *customTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // Tool implementation logic
    return "result", nil
}
```

### Extend MCP Protocol

1. Add new fields to the `MCPMessage` struct
2. Add new message processing in the `handleMessage` method
3. Implement corresponding processing functions

## Monitoring and Debugging

### Log Levels

- `INFO` - Key events such as connection establishment, tool registration, etc.
- `ERROR` - Connection failures, tool invocation errors, etc.
- `DEBUG` - Detailed protocol interaction information

### Health Check

```bash
# Check global tools
curl http://localhost:8989/xiaozhi/api/mcp/tools/health_check

# Check specific device tools  
curl http://localhost:8989/xiaozhi/api/mcp/tools/device123
```

## Troubleshooting

### Common Issues

1. **SSE Connection Failed**
   - Check if MCP server is running
   - Verify SSE URL configuration
   - Check network connection

2. **WebSocket Connection Disconnected**
   - Check heartbeat mechanism
   - Verify device ID format
   - Check connection limit

3. **Tool Invocation Failed**
   - Verify tool parameter format
   - Check if tool is already registered
   - Check error logs

### Performance Optimization

- Adjust reconnect interval and count
- Set appropriate connection limits
- Enable connection pool reuse
- Regularly clean up expired connections

## References

- [Eino Framework Documentation](https://www.cloudwego.io/docs/eino/)
- [MCP Protocol Specification](https://github.com/mark3labs/mcp-go)
- [SSE Specification](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [WebSocket Protocol](https://tools.ietf.org/html/rfc6455) 
