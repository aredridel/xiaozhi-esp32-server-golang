# HTTP 组件

统一of HTTP 客户端组件，used for管理所有to Manager after端of HTTP 调用。

## 目录结构

```
internal/components/http/
├── client.go          # 通用 HTTP 客户端（支持重试、认证etc）
├── manager_client.go  # Manager after端专用客户端
├── types.go           # 类型定义
└── README.md          # 本文档
```

## 设计说明

### Client（通用 HTTP 客户端）

provide基础of HTTP 请求功能：
- 支持重试机制（use exponential backoff）
- 支持认证 Token（Bearer Token）
- 支持自定义超whentime
- 统一errorprocess
- 自动 JSON 序列化/反序列化

### ManagerClient（Manager after端专用客户端）

基于通用客户端封装，专门used for调用 Manager after端 API。

## use示例

### 创建 Manager 客户端

```go
import "xiaozhi-esp32-server-golang/internal/components/http"

client := http.NewManagerClient(http.ManagerClientConfig{
    BaseURL:    "http://localhost:8080",
    AuthToken:  "your-token",  // 可选
    Timeout:    10 * time.Second,
    MaxRetries: 3,
})
```

### send GET 请求

```go
var response MyResponse
err := client.DoRequest(ctx, http.RequestOptions{
    Method: "GET",
    Path:   "/api/configs",
    QueryParams: map[string]string{
        "device_id": "device123",
    },
    Response: &response,
})
```

### send POST 请求

```go
request := MyRequest{
    Field1: "value1",
    Field2: "value2",
}

err := client.DoRequest(ctx, http.RequestOptions{
    Method: "POST",
    Path:   "/api/internal/history/messages",
    Body:   request,
})
```

### get原始响应

```go
body, err := client.DoRequestRaw(ctx, http.RequestOptions{
    Method: "GET",
    Path:   "/api/system/configs",
})
```

## 重构说明

### 重构before

- `HistoryClient` 和 `ConfigManager` 各自实现 HTTP 调用逻辑
- 代码重复，维护成本高
- 重试、认证etc逻辑分散

### 重构after

- 统一of HTTP 组件，集in管理
- 代码复用，易于维护
- 统一oferrorprocess和重试机制

## already重构of模块

1. **internal/data/history/client.go** - chat history客户端
2. **internal/domain/config/manager/manager.go** - config管理器
3. **internal/domain/config/manager/auth.go** - 认证相关 API

## 注意事项

- 所有to Manager after端of HTTP 调用都应use `ManagerClient`
- 如需调用其他after端服务，can基于 `Client` 创建new专用客户端
- 重试机制default最多 3 次，可throughconfig调整

