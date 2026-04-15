# OTA Interface MQTT Authentication Configuration

## Overview

The OTA interface now supports HMAC-SHA256 signature-based MQTT password verification mechanism, providing a more secure authentication method. The MQTT server also supports corresponding verification logic.

## Configuration Structure

### Configuration File (config/config.yaml)

```yaml
mqtt_server:
  signature_key: "your_ota_signature_key_here"
ota:
  signature_key: "your_ota_signature_key_here"
  test:
    websocket:
      url: "ws://192.168.208.214:8989/xiaozhi/v1/"
    mqtt:
      enable: false
      endpoint: "192.168.208.214"
  external:
    websocket:
      url: "wss://www.tb263.cn:55555/go_ws/xiaozhi/v1/"
    mqtt:
      enable: false
      endpoint: "www.youdomain.cn"
```

### Configuration Description

- `mqtt_server.signature_key`: MQTT signature key, used to generate MQTT password signature
- `ota.signature_key`: Key used when OTA issues MQTT password, needs to correspond with mqtt_server.signature_key
- `ota.test`: Test environment configuration (for internal network IP use)
- `ota.external`: External environment configuration (for external network IP use)

### Integration with xiaozhi-mqtt-gateway

This system works with the official [xiaozhi-mqtt-gateway](https://github.com/78/xiaozhi-mqtt-gateway) project to implement the complete MQTT authentication flow:

1. **Configuration Consistency Requirement**: `ota.signature_key` must be completely consistent with the signature key in the xiaozhi-mqtt-gateway project
2. **Authentication Flow**:
   - xiaozhi-mqtt-gateway is responsible for generating MQTT connection credentials
   - This system is responsible for verifying MQTT connection credentials
   - Both parties use the same signature algorithm and key to ensure authentication success
3. **Deployment Recommendation**: It is recommended to deploy both projects in the same network environment to ensure configuration synchronization updates

## Utility Functions

### 1. Password Signature Generation

```go
// Generate HMAC-SHA256 password signature
password := util.GeneratePasswordSignature(data, key)
```

### 2. MQTT Credentials Generation

```go
// Generate complete MQTT connection credentials
credentials, err := util.GenerateMqttCredentials(deviceId, clientId, ip, signatureKey)
if err != nil {
    // Handle error
}
// credentials contains: ClientId, Username, Password
```

### 3. MQTT Credentials Verification

```go
// Verify MQTT connection credentials
credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
if err != nil {
    // Verification failed
}
// credentialInfo contains: GroupId, MacAddress, UUID, UserData
```

## MQTT Authentication Logic

### 1. Client ID Format

```
GID_test@@@{deviceId}@@@{clientId}
```

Example:
```
GID_test@@@02_4A_7D_E3_89_BF@@@e3b0c442-98fc-4e1a-8c3d-6a5b6a5b6a5b
```

### 2. Username Format

Base64 encoded JSON containing client IP information:

```yaml
ip: "1.202.193.194"
```

After Base64 encoding:
```
eyJpcCI6IjEuMjAyLjE5My4xOTQifQ==
```

### 3. Password Generation

Use HMAC-SHA256 algorithm to generate password signature:

```go
signatureData := clientId + "|" + username
password := HMAC-SHA256(signatureData, signature_key)
```

### 4. Verification Logic

When client verifies, it needs to:

1. Parse clientId, extract groupId, macAddress, uuid
2. Decode username, get IP information
3. Use the same signature key and algorithm to verify password

## MQTT Server Authentication

### Authentication Flow

1. **Super Admin Verification**
   - Username: `admin` (configurable)
   - Password: `shijingbo!@#` (configurable)

2. **Regular User Verification**
   - Prioritize HMAC-SHA256 signature verification
   - If signature key is not configured, fall back to AES verification method

### Authentication Hook Implementation

```go
func (h *AuthHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
    username := string(pk.Connect.Username)
    password := string(pk.Connect.Password)
    clientId := string(pk.Connect.ClientIdentifier)

    // Super admin verification
    if username == adminUsername && password == adminPassword {
        return true
    }

    // Regular user verification - use new signature verification logic
    signatureKey := viper.GetString("mqtt_server.signature_key")
    if signatureKey != "" {
        credentialInfo, err := util.ValidateMqttCredentials(clientId, username, password, signatureKey)
        if err != nil {
            return false
        }
        return true
    }

    // Fall back to AES verification logic
    return h.validateWithAes(username, password)
}
```

## Compatibility

- If `mqtt_server.signature_key` is not configured, the system will fall back to the original SHA256/AES password generation method
- Maintains backward compatibility, will not affect existing functionality
- MQTT server supports multiple authentication methods coexisting

## Security Recommendations

1. Use strong random strings as signature keys
2. Rotate signature keys regularly
3. Use HTTPS/WSS connections in production environment
4. Monitor abnormal login attempts
5. Enable logging to track authentication success/failure status
6. **Ensure xiaozhi-mqtt-gateway and this system's signature keys are synchronized and updated**

## Data Structures

### MqttCredentials
```go
type MqttCredentials struct {
    ClientId string `json:"client_id"`
    Username string `json:"username"`
    Password string `json:"password"`
}
```

### MqttCredentialInfo
```go
type MqttCredentialInfo struct {
    GroupId    string                 `json:"groupId"`
    MacAddress string                 `json:"macAddress"`
    UUID       string                 `json:"uuid"`
    UserData   map[string]interface{} `json:"userData"`
}
``` 

# Official xiaozhi-mqtt-gateway Usage Instructions

This system can work with the official [xiaozhi-mqtt-gateway](https://github.com/78/xiaozhi-mqtt-gateway) project.

Only the MQTT username and password in the OTA interface need to pass xiaozhi-mqtt-gateway authentication. To ensure MQTT authentication works properly, **`ota.signature_key` configuration must be consistent with the signature key in xiaozhi-mqtt-gateway**.

Configuration is as follows:
1. Do not enable mqtt server (use xiaozhi-mqtt-gateway)
2. `ota.signature_key` configuration must be consistent with the signature key in xiaozhi-mqtt-gateway
3. Configure xiaozhi-mqtt-gateway's websocket backend to this project's address

```yaml
mqtt_server:
  enable: false
ota:
  signature_key: "your_ota_signature_key_here"
  test:  # Internal network test return
    websocket:
      url: "ws://192.168.208.214:8989/xiaozhi/v1/"
    mqtt:
      enable: true
      endpoint: "192.168.208.214:1883"  # xiaozhi-mqtt-gateway mqtt server address
  external:  # External network return
    websocket:
      url: "wss://www.tb263.cn:55555/go_ws/xiaozhi/v1/"
    mqtt:
      enable: true
      endpoint: "mqtt.youdomain.com:1883"  # xiaozhi-mqtt-gateway mqtt server address
```
