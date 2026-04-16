# Config Manager Usage Guide

## Overview

This package provides two main managers:
1. **ConfigManager** - Config manager, provides high-level configuration management features
2. **AuthManager** - Authentication manager, specifically handles device activation and authentication related functions

## Core Features

### ConfigManager - Config Manager
- ✅ Config cache mechanism, improves access performance
- ✅ Config validation features
- ✅ Singleton pattern for global management
- ✅ Cache cleanup and invalidation mechanism
- ✅ Thread-safe concurrent access

### AuthManager - Authentication Manager
- ✅ Device activation status check (via HTTP interface)
- ✅ Real-time activation info retrieval (no cache)
- ✅ Challenge code verification and HMAC security validation
- ✅ Direct backend API calls, ensures data real-time accuracy
- ✅ **HTTP Interface Integration** - Calls backend management system activation interfaces

## HTTP Interface Integration

AuthManager now calls backend management system through HTTP interfaces, supporting the following endpoints:

### 1. Check Device Activation Status
```http
GET /api/internal/device/check-activation?device_id=xxx&client_id=xxx
```

### 2. Get Device Activation Info
```http
GET /api/internal/device/activation-info?device_id=xxx&client_id=xxx
```

### 3. Device Activation
```http
POST /api/internal/device/activate
Content-Type: application/json

{
  "device_id": "xxx",
  "client_id": "xxx",
  "code": "123456",
  "challenge": "uuid",
  "algorithm": "hmac-sha256",
  "serial_number": "ABC123",
  "hmac": "signature"
}
```

## Configuration

Add the following configuration to the config file (config.yaml):

```yaml
manager:
  backend_url: "http://localhost:8080"  # Backend management system base URL
```

If not configured, defaults to `http://localhost:8080`.

## Usage Example

```go
package main

import (
    "context"
    "xiaozhi-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()
    
    // Initialize managers
    err := manager.Init()
    if err != nil {
        panic(err)
    }
    
    err = manager.InitAuth()
    if err != nil {
        panic(err)
    }
    
    // Get manager instances
    configManager := manager.GetInstance()
    authManager := manager.GetAuthInstance()
    
    // Use config manager
    config, err := configManager.GetUserConfig(ctx, "device_001")
    if err != nil {
        // Handle error
    }
    
    // Use authentication manager (via HTTP interface)
    activated, err := authManager.IsDeviceActivated(ctx, "device_001", "client_001")
    if err != nil {
        // Handle error
    }
    
    if !activated {
        // Get activation info
        code, challenge, message, timeout := authManager.GetActivationInfo(ctx, "device_001", "client_001")
        // Display activation code to user...
        
        // Verify after user inputs activation code
        activationPayload := types.ActivationPayload{
            Algorithm:    "hmac-sha256",
            SerialNumber: "ABC123",
            Challenge:    challenge,
            HMAC:         "calculated_hmac",
        }
        
        success, err := authManager.VerifyChallenge(ctx, "device_001", "client_001", fmt.Sprintf("%d", code), activationPayload)
        // Process activation result...
    }
}
```

## Architecture Advantages

### Frontend System Integration
- ESP32 devices or other frontend systems directly call AuthManager
- AuthManager internally calls backend management system via HTTP
- Decouples frontend systems from backend management systems

### Real-time Data
- Direct HTTP API calls, gets latest status
- No cache design, ensures data real-time accuracy
- Simplified architecture, reduces complexity

### Error Handling
- Comprehensive error handling and logging
- Degradation handling for HTTP request failures
- Detailed error information and debug logs

### Security
- Supports HMAC verification
- Secure activation process
- Real-time status validation

## Notes

1. Ensure backend management system is running and accessible
2. Configure `manager.backend_url` correctly
3. HTTP client default timeout is 10 seconds
4. No cache mode, each call will request backend interface
5. Ensure stable network connection to avoid frequent interface call failures
