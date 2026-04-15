# Device/Agent Dimension MCP Remote Call Documentation

This document introduces the **MCP remote call debugging capability** in the management console, including:

- Agent dimension MCP endpoint generation
- Agent dimension tool list acquisition and remote calling
- Device dimension tool list acquisition and remote calling
- Permission differences between administrators and regular users

Related documents:

- [MCP Architecture Documentation](./mcp.md)
- [MCP Market Feature Documentation](./mcp_market.md)
- [Management Console Guide](./manager_console_guide.md)

---

## 1. Feature Positioning

This feature is mainly used for "debugging and verification":

- Quickly view what MCP tools are currently exposed by the agent/device
- Directly construct parameters and call tools in the console
- Get agent dimension MCP endpoint for external MCP client access testing

Suitable scenarios:

- Verify if a remote MCP service is effective
- Check tool schema / parameter examples
- Troubleshoot differences in MCP behavior between agents and devices

---

## 2. Differences Between Two Call Dimensions

## 2.1 Agent Dimension (Agent)

Features:

- From the "agent configuration" perspective
- Supports getting the agent's MCP endpoint (with token)
- Supports pulling tool list, directly initiating tool calls
- Affected by agent configuration (e.g., `mcp_service_names`)

Common uses:

- Verify available MCP tool set after agent filtering
- Copy endpoint for external debugging client use

## 2.2 Device Dimension (Device)

Features:

- From the "specific device connection" perspective
- Directly requests tool details/calls through device current connection context
- Usually depends on device being online and WebSocket controller being available

Common uses:

- Troubleshoot "inconsistent tool behavior on different devices with same agent"
- Verify device current online session side MCP capability

---

## 3. Page Entry Points (Administrator / Regular User)

### 3.1 Administrator

- `Administrator -> Agent Management` (agent dimension endpoint / tools / call)
- `Administrator -> Device Management` (device dimension tools / call)

### 3.2 Regular User

- `My Agents` (agent dimension tools / call)
- `My Devices` / `Agent Devices` (device dimension tools / call)
- `Agent Edit` (configure `mcp_service_names`, affects agent dimension visible service scope)

---

## 4. Agent Dimension: Complete Debugging Flow

## 4.1 Configure Agent Available MCP Services (Optional but Recommended)

On the agent edit page, you can set `mcp_service_names` (service name list, comma-separated):

- Empty: Use all enabled global MCP services
- Filled: Only use specified service names (must be services that exist and are enabled in the system)

The system will perform the following on this field:

- Deduplication
- Trim spaces
- Legitimacy validation (service name must exist in enabled global service set)

## 4.2 Get Agent MCP Endpoint

The console can get the agent-specific MCP access point URL, format similar to:

```text
ws(s)://<host>/mcp?token=<jwt>
```

Description:

- Endpoint is derived based on default OTA configuration `external.websocket.url` for domain and protocol
- Token contains current user and agent context (used for permission verification/binding)
- Suitable for external MCP client temporary debugging, not recommended for public sharing

## 4.3 Get Tool List

The console will request agent dimension MCP tool details, returned content usually includes:

- `name`
- Tool description
- Parameter schema
- Parameter examples (if provided by device side/server side)

If unable to get (e.g., controller not initialized or client temporarily unreachable), backend will return empty list instead of error, allowing page to continue operation.

## 4.4 Direct Tool Call

Fill in the console:

- `tool_name`
- `arguments` (JSON)

After initiating the call, you can view the complete return body (JSON format) in the result box.

---

## 5. Device Dimension: Complete Debugging Flow

## 5.1 Get Device Tool List

After selecting a device, the console will use device identifier (internally mapped to device name) to request MCP tool details from WebSocket controller.

Common failure situations:

- Device not online
- Device does not belong to current user (user perspective)
- WebSocket controller temporarily unavailable

In these cases, the interface usually returns empty tool list or permission error.

## 5.2 Call Device MCP Tool

Similar to agent dimension, fill in:

- `tool_name`
- `arguments` (JSON)

The difference is that the call body uses `device_id` (actual backend will pass device name) context, so it's closer to the "current device session" real execution environment.

---

## 6. Permissions and Interface Differences (Administrator vs Regular User)

### 6.1 Regular User Interfaces

Agent dimension:

- `GET /user/agents/:id/mcp-endpoint`
- `GET /user/agents/:id/mcp-tools`
- `POST /user/agents/:id/mcp-call`

Device dimension:

- `GET /user/devices/:id/mcp-tools`
- `POST /user/devices/:id/mcp-call`

Agent service filtering auxiliary:

- `GET /user/agents/:id/mcp-services/options`

Regular users can only operate their own agents/devices.

### 6.2 Administrator Interfaces

Agent dimension:

- `GET /admin/agents/:id/mcp-endpoint`
- `GET /admin/agents/:id/mcp-tools`
- `POST /admin/agents/:id/mcp-call`

Device dimension:

- `GET /admin/devices/:id/mcp-tools`
- `POST /admin/devices/:id/mcp-call`

Administrators can debug any agent/device across users (provided the record exists and connection link is normal).

---

## 7. Endpoint Generation Logic (Agent Dimension)

Agent endpoint generation depends on:

1. Default OTA configuration (`type=ota` and `is_default=true`)
2. `external.websocket.url` in OTA configuration
3. Stable token generated based on current user ID + agent ID

Generation result will use:

- Same protocol (`ws` / `wss`)
- Same host (domain/IP + port)
- Fixed path `/mcp`

Therefore, if unable to generate endpoint, please first check OTA external network WebSocket configuration.

---

## 8. FAQ and Troubleshooting

### 8.1 Tool list is empty

Possible causes:

- Device not online (device dimension)
- WebSocket controller not initialized
- Client did not return tool details
- Agent dimension has no available services after `mcp_service_names` filtering

Suggested troubleshooting order:

1. Confirm device online status
2. Check if global MCP service is enabled
3. Check agent `mcp_service_names` configuration
4. Retry getting tools in console

### 8.2 Call reports parameter JSON error

Console parameter area requires valid JSON object, for example:

```json
{
  "query": "hello"
}
```

Common errors:

- Single quotes
- Trailing commas
- Top level is not an object

### 8.3 Failed to get agent endpoint

Usually due to missing OTA default configuration or `external.websocket.url` not configured.

### 8.4 MCP service imported but not visible in agent call

Check:

1. Whether imported service is enabled
2. Global MCP configuration master switch and service enable status
3. Whether agent excluded this service through `mcp_service_names`

---

## 9. Best Practices

- First verify "device dimension" tool availability on administrator side, then verify "agent dimension" tool filtering results
- For production agents, it is recommended to explicitly configure `mcp_service_names` to avoid unrelated tools being exposed to the model
- Treat endpoint as a sensitive debugging entry point, avoid spreading URLs with tokens in public channels
