# config管理器use说明

## 概述

本包provide两个主要of管理器：
1. **ConfigManager** - config管理器，provide高层级ofconfig管理功能
2. **AuthManager** - 认证管理器，专门processdevice激活和认证相关功能

## 核心特性

### ConfigManager config管理器
- ✅ configcache机制，提高访问性能
- ✅ config验证功能
- ✅ 单例模式of全局管理
- ✅ cache清理和失效机制
- ✅ 线程安全of并发访问

### AuthManager 认证管理器
- ✅ device激活状态check（throughHTTP接口）
- ✅ 实whenget激活信息（无cache）
- ✅ 挑战码验证和HMAC安全验证
- ✅ 直接调用after端接口，确保data实when性
- ✅ **HTTP接口集成** - 调用after端管理系统of激活接口

## HTTP接口集成

AuthManager 现atthroughHTTP接口调用after端管理系统，支持以下接口：

### 1. checkdevice激活状态
```http
GET /api/internal/device/check-activation?device_id=xxx&client_id=xxx
```

### 2. getdevice激活信息
```http
GET /api/internal/device/activation-info?device_id=xxx&client_id=xxx
```

### 3. device激活
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

## config说明

atconfig文件（config.yaml）in添加以下config：

```yaml
manager:
  backend_url: "http://localhost:8080"  # after端管理系统of基础URL
```

if未config，defaultuse `http://localhost:8080`。

## use示例

```go
package main

import (
    "context"
    "xiaozhi-esp32-server-golang/internal/domain/config/manager"
)

func main() {
    ctx := context.Background()
    
    // 初始化管理器
    err := manager.Init()
    if err != nil {
        panic(err)
    }
    
    err = manager.InitAuth()
    if err != nil {
        panic(err)
    }
    
    // get管理器实例
    configManager := manager.GetInstance()
    authManager := manager.GetAuthInstance()
    
    // useconfig管理器
    config, err := configManager.GetUserConfig(ctx, "device_001")
    if err != nil {
        // processerror
    }
    
    // use认证管理器（throughHTTP接口）
    activated, err := authManager.IsDeviceActivated(ctx, "device_001", "client_001")
    if err != nil {
        // processerror
    }
    
    if !activated {
        // get激活信息
        code, challenge, message, timeout := authManager.GetActivationInfo(ctx, "device_001", "client_001")
        // 显示激活码给user...
        
        // userinput激活码afterperform验证
        activationPayload := types.ActivationPayload{
            Algorithm:    "hmac-sha256",
            SerialNumber: "ABC123",
            Challenge:    challenge,
            HMAC:         "calculated_hmac",
        }
        
        success, err := authManager.VerifyChallenge(ctx, "device_001", "client_001", fmt.Sprintf("%d", code), activationPayload)
        // process激活result...
    }
}
```

## 架构优势

### before端系统集成
- ESP32device或其他before端系统直接调用 AuthManager
- AuthManager 内部throughHTTP调用after端管理系统
- 实现before端系统andafter端管理系统of解耦

### 实whendata
- 直接调用HTTP接口，get最新状态
- 无cache设计，确保data实when性
- 简化架构，减少复杂性

### errorprocess
- 完善oferrorprocess和日志record
- HTTP请求failedwhenof降级process
- 详细oferror信息和调试日志

### 安全性
- 支持HMAC验证
- 安全of激活流程
- 实when状态验证

## 注意事项

1. 确保after端管理系统is运行and可访问
2. 正确config `manager.backend_url`
3. HTTP客户端default超whenis10秒
4. 无cache模式，每次调用都会请求after端接口
5. 确保网络连接稳定，避免频繁of接口调用failed
