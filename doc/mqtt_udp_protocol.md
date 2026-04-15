# 🚦 Data Flow

1. **Call OTA Interface**
   - Get **MQTT**, **WebSocket** addresses

2. **Connect MQTT**
   - Send `hello` message to get:
     - 🎵 `audio_params`
     - 🌐 UDP server address
     - 🔑 `aes_key`
     - 🧩 `nonce`

3. **Connect UDP Server**
   - Send and receive voice data

---

# 🛠️ Server-side Flow

| Step | Description |
| :--- | :--- |
| 1. MQTT Service | Generate `aes_key`, `nonce`, and associate with `device_id`, `client_id` |
| 2. MQTT Message Listening | When receiving `type: listen, state: start`, initialize `clientState` structure with status `start` |
| 3. UDP Service | After receiving packet, parse `nonce`, find corresponding `clientState`, fill remote address, status becomes `recv` |
| 4. Stop Receiving | When receiving `type: listen, state: stop` or automatically detecting no sound, stop receiving |

---

# 🔗 Association Relationships

- OTA verifies **MAC Address** and **clientId**, and associates with **uid**
- OTA issued **MQTT address** and **mqtt_clientId** associate **MAC Address** and **clientId**
- **MQTT connection** can parse **clientId** and **MAC Address**
- **MQTT hello message** can associate to `aes_key`, `nonce`
- **UDP audio message** can associate to `nonce`

---

> **Note:**
> - `clientState` structure is used to maintain each client's session state and resources.
> - `nonce` is the unique identifier between client and server, used for security association and data routing.
