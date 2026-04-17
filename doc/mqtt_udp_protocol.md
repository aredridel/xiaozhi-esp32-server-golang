# 🚦 Data Flow

1. **Call OTA Interface**
   - Obtain **MQTT** and **WebSocket** addresses

2. **Connect to MQTT**
   - The built-in `mqtt_server` publishes a lifecycle event to `/p2p/device_public/_server/lifecycle`
   - The main program creates or reuses MQTT transport based on `device_id`, and best-effort warms up device-side MCP

3. **Send `hello` Message**
   - Obtain:
     - 🎵 `audio_params`
     - 🌐 UDP server address
     - 🔑 `aes_key`
     - 🧩 `nonce`

4. **Connect to UDP Server**
   - Send and receive voice data

5. **Send `listen`, `abort`, and other subsequent signals**
   - Signal semantics remain unchanged, still based on chat-level initialization after `hello` completion

---

# 🧭 Lifecycle Topic

- **Topic**: `/p2p/device_public/_server/lifecycle`
- **Purpose**: For server internal use only, used to convey device MQTT online/offline events
- **Message Body Example**:
  ```json
  {
    "type": "mqtt_lifecycle",
    "device_id": "11:22:33:44:55:66",
    "state": "online",
    "client_id": "GID_test@@@11_22_33_44_55_66@@@uuid",
    "ts": 1710000000000
  }
  ```

- **State Definition**
  - `online`: Device just connected to `mqtt_server`, main program can prepare transport and MCP in advance
  - `offline`: Device disconnected from `mqtt_server`, main program immediately maps offline status, but transport is retained for a period of time for short reconnection reuse

- **Boundary Notes**
  - Lifecycle events do not replace `hello`
  - Lifecycle events only maintain connection-level resources and do not carry chat-level information such as `audio_params` or UDP negotiation

---

# 🛠️ Server Flow

| Step | Description |
| :--- | :--- |
| 1. MQTT Lifecycle Listening | On receiving an `online` event, create or reuse transport, and best-effort warm up device-side MCP |
| 2. `hello` Processing | Return `audio_params`, UDP address, key and `nonce`, and prepare chat-level session state |
| 3. MQTT Message Listening | On receiving `type: listen, state: start`, initialize `clientState` structure with state `start` |
| 4. UDP Service | On receiving a packet, parse `nonce`, find the corresponding `clientState`, fill in remote address, state is `recv` |
| 5. Stop Receiving | On receiving `type: listen, state: stop` or automatically detecting silence, stop receiving |
| 6. MQTT Lifecycle Offline | On receiving an `offline` event, immediately map offline status, and reclaim transport after the retention period |

---

# 🔗 Association Relationships

- OTA validates **MAC address** and **clientId**, and associates them with **uid**
- OTA-delivered **MQTT address** and **mqtt_clientId** are associated with **MAC address** and **clientId**
- Through **MQTT connection lifecycle messages**, **MAC address**, `device_id`, and `client_id` can be associated in advance
- Through **MQTT `hello` message**, they can be associated with `audio_params`, `aes_key`, `nonce`
- Through **UDP audio messages**, they can be associated with `nonce`

---

> **Notes:**
> - The `clientState` structure is used to maintain each client's chat-level session state and resources.
> - Transport and MCP can be prepared in advance during the MQTT online phase, but actual chat-level negotiation still follows `hello`.
> - `nonce` is a unique identifier between the client and server, used for secure association and data routing.
