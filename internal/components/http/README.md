# HTTP Component

Unified HTTP client component, used for managing all HTTP calls to Manager backend.

## Directory Structure

```
internal/components/http/
├── client.go          # General HTTP client (supports retry, authentication, etc.)
├── manager_client.go  # Manager backend dedicated client
├── types.go           # Type definitions
└── README.md          # This document
```

## Design Description

### Client (General HTTP Client)

Provides basic HTTP request functionality:
- Supports retry mechanism (uses exponential backoff)
- Supports authentication Token (Bearer Token)
- Supports custom timeout
- Unified error processing
- Automatic JSON serialization/deserialization

### ManagerClient (Manager Backend Dedicated Client)

Based on general client encapsulation, specifically used for calling Manager backend API.

## Usage Examples

### Create Manager Client

```go
import "xiaozhi-esp32-server-golang/internal/components/http"

client := http.NewManagerClient(http.ManagerClientConfig{
    BaseURL:    "http://localhost:8080",
    AuthToken:  "your-token",  // optional
    Timeout:    10 * time.Second,
    MaxRetries: 3,
})
```

### Send GET Request

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

### Send POST Request

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

### Get Raw Response

```go
body, err := client.DoRequestRaw(ctx, http.RequestOptions{
    Method: "GET",
    Path:   "/api/system/configs",
})
```

## Refactoring Notes

### Before Refactoring

- `HistoryClient` and `ConfigManager` each implement HTTP call logic
- Code duplication, high maintenance cost
- Retry, authentication, etc. logic scattered

### After Refactoring

- Unified HTTP component, centralized management
- Code reuse, easy to maintain
- Unified error processing and retry mechanism

## Already Refactored Modules

1. **internal/data/history/client.go** - chat history client
2. **internal/domain/config/manager/manager.go** - config manager
3. **internal/domain/config/manager/auth.go** - authentication related API

## Notes

- All HTTP calls to Manager backend should use `ManagerClient`
- If need to call other backend services, can create new dedicated client based on `Client`
- Retry mechanism defaults to maximum 3 times, can be adjusted through config

