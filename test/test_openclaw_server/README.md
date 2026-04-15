# test_openclaw_server

WebSocket test service for interfacing with the `test/xiaozhi_openclaw/xiaozhi-integration/openclaw-channel` plugin.

## Features

- Provides `/ws/openclaw` WebSocket endpoint (validates `user_id/agent_id/endpoint_id` token).
- Actively sends `handshake_ack` after connection establishment.
- Receives plugin `ping` and replies with `pong`.
- Receives plugin `response`, records response (includes `metadata.device_id`).
- Provides HTTP API to actively send `message` to plugin.

## Startup

```bash
go run ./test/test_openclaw_server -addr :18080 -jwt-secret xiaozhi_admin_secret_key
```

Output detailed WebSocket debug logs:

```bash
go run ./test/test_openclaw_server -addr :18080 -jwt-secret xiaozhi_admin_secret_key -verbose
```

## Generate Token (Test)

```bash
node test/xiaozhi_openclaw/xiaozhi-integration/generate-token.js 1 main agent_main
```

Or explicitly specify the same key as the server (recommended):

```bash
JWT_SECRET=xiaozhi_admin_secret_key \
node test/xiaozhi_openclaw/xiaozhi-integration/generate-token.js 1 main agent_main
```

Configure the output `Token` in the plugin configuration:

- `channels.xiaozhi.url = ws://127.0.0.1:18080/ws/openclaw`
- `channels.xiaozhi.token = <token>`
- `JWT_SECRET` must exactly match `go run ./test/test_openclaw_server -jwt-secret ...`, otherwise `signature is invalid` will be reported

## HTTP API

### 1) 健康检查

```bash
curl -sS http://127.0.0.1:18080/healthz | jq
```

### 1.1) Auth Debugging (Troubleshoot 401)

```bash
curl -sS "http://127.0.0.1:18080/debug/ws-auth?token=<token>" | jq
```

Also supports passing token via request header:

```bash
curl -sS "http://127.0.0.1:18080/debug/ws-auth" \
  -H "Authorization: Bearer <token>" | jq
```

### 2) 查看当前连接

```bash
curl -sS http://127.0.0.1:18080/api/connections | jq
```

### 3) 发送测试消息给插件

```bash
curl -sS -X POST http://127.0.0.1:18080/api/send \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id":"main",
    "device_id":"esp32-001",
    "content":"请回复一条测试消息",
    "session_id":"test-session-1"
  }' | jq
```

In multi-connection mode, you can specify `conn_id` for precise sending:

```bash
curl -sS -X POST http://127.0.0.1:18080/api/send \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id":"main",
    "conn_id":"main-2",
    "device_id":"esp32-001",
    "content":"发给指定连接"
  }' | jq
```

### 4) 查看插件回包

```bash
curl -sS "http://127.0.0.1:18080/api/responses?limit=20" | jq
```

Can filter by agent:

```bash
curl -sS "http://127.0.0.1:18080/api/responses?agent_id=main&limit=20" | jq
```
