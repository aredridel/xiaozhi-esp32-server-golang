# WebSocket Server and OTA Configuration Guide

This guide is for beginners and provides detailed instructions on configuring the WebSocket server and OTA (Over-The-Air) related parameters.

---

## 1. Configuration File Location

All main configurations are in:

- `config/config.yaml`

If you cannot find this file, you can also refer to `config/config.json.git`.

---

## 2. WebSocket Server Configuration

### 2.1 Purpose
The WebSocket server is used for real-time communication between devices and the server.

### 2.2 Key Configuration Items
Find the following content in the `config/config.yaml` file:

```yaml
websocket:
  host: "0.0.0.0"
  port: 8989
```
- `host`: Listening address, usually keep as `0.0.0.0`.
- `port`: Listening port, default `8989`, can be modified as needed.

### 2.3 Modification Method
To change the port to 9000:
```yaml
websocket:
  host: "0.0.0.0"
  port: 9000
```

---

## 3. OTA (Firmware Upgrade) Configuration

### 3.1 Purpose
OTA is used for devices to automatically obtain WebSocket/MQTT connection parameters and firmware upgrade information from the server.

### 3.2 Key Configuration Items
Find the `ota` section in the `config/config.yaml` file:

```yaml
ota:
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
- `test`: Parameters for devices in internal network environment; in the program, the condition is determined by whether it starts with 192.168 or 127.0.
- `external`: Parameters for devices in external network environment.
- `websocket.url`: The WebSocket server address that devices should connect to.
- `mqtt.enable`: If enabled, the configured MQTT address will be returned in the OTA interface, and devices will prefer MQTT+UDP mode.
- `mqtt.endpoint`: MQTT server address; the device side defaults to port 8883 (TLS connection). If a non-8883 port is specified, it will use unencrypted TCP connection.

### 3.3 Common Modification Examples
- Modify internal network WebSocket address:
  ```yaml
  ota:
    test:
      websocket:
        url: "ws://192.168.1.100:8989/xiaozhi/v1/"
  ```
- Modify external network WebSocket address:
  ```yaml
  ota:
    external:
      websocket:
        url: "wss://yourdomain.com:55555/go_ws/xiaozhi/v1/"
  ```

---

## 4. OTA Interface Description (How Devices Obtain Configuration)

1. Devices make an HTTP POST request to `http://server_address:port/xiaozhi/ota/`.
2. Request headers must include:
   - `Device-Id`: Device unique ID (e.g., MAC address)
   - `Client-Id`: Client unique ID
3. The server automatically selects `test` or `external` configuration based on device IP and returns WebSocket/MQTT and other parameters.
4. Devices parse the returned content and connect to the WebSocket server according to `websocket.url`.

---

## 5. FAQ

- **Port occupied?**
  - Modify `websocket.port` and restart the service.
- **Device cannot connect to server?**
  - Check if `websocket.url` in `ota` configuration is correct and if the server port is open.
- **Need MQTT?**
  - Set `mqtt.enable` to `true` and configure `endpoint`.

---

If you have questions, it is recommended to first check the `config/config.yaml` configuration items, then refer to this guide.
