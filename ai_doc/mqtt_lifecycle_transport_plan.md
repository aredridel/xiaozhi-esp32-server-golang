# MQTT Lifecycle-Driven Transport Pre-Creation Plan

## Goal

When devices connect to / disconnect from `mqtt_server`, the `mqtt_server` publishes lifecycle messages to an MQTT topic that the main program is already listening on via callbacks. After the main program receives them:

1. Pre-create `mqtt udp transport` when device comes online
2. Best-effort warm up MCP when device comes online
3. Immediately map device offline status when device goes offline
4. Retain transport for a period after device goes offline, avoiding frequent creation/destruction from short reconnections
5. Do not change existing `hello` / `listen` / `abort` / `goodbye` signaling semantics

## Topic Design

No new root prefix is added; reuse the existing `"/p2p/device_public/"` prefix.

New lifecycle topic:

`/p2p/device_public/_server/lifecycle`

Suggested code constant:

- `MDeviceLifecycleTopic = MDevicePubTopicPrefix + "_server/lifecycle"`

## Lifecycle Message Format

Message body uses JSON:

```json
{
  "type": "mqtt_lifecycle",
  "device_id": "ba:8f:17:de:94:94",
  "state": "online",
  "client_id": "GID_test@@@ba_8f_17_de_94_94@@@uuid",
  "ts": 1710000000000
}
```

Field descriptions:

- `type`: Fixed as `mqtt_lifecycle`
- `device_id`: Normalized device ID, using colon format consistently
- `state`: `online` / `offline`
- `client_id`: Original MQTT client ID, for troubleshooting
- `ts`: Event timestamp in milliseconds

## End-to-End Flow

### 1. mqtt_server Publishes Lifecycle Message

In `DeviceHook`:

- `OnSessionEstablished`
- `OnDisconnect`

Publish the lifecycle event to `/p2p/device_public/_server/lifecycle` via callback.

The `mqtt_server` is still responsible for publishing, but the publish action is consolidated into a callback invoked within the hook, avoiding scattering topic concatenation logic across multiple locations.

### 2. Main Program Reuses Existing Subscription

`MqttUdpAdapter` continues to subscribe only to the existing:

`/p2p/device_public/#`

After receiving a message, first check the topic:

- If it is `/p2p/device_public/_server/lifecycle`, route to the lifecycle handling branch
- Otherwise, continue to the existing device business message branch

This does not affect subsequent normal signal parsing such as `hello` / `listen`.

### 3. Pre-create Transport on Device Online

After receiving an `online` lifecycle message:

1. First perform lifecycle debouncing
2. If transport does not exist, immediately create `MqttUdpConn + UdpSession`
3. Trigger `onNewConnection`, letting the main program create `ChatManager`
4. Mark broker online
5. Trigger a best-effort MCP warm-up
6. Map device online status

Note:

- This creates `transport` and `ChatManager`
- `ChatSession` is still lazily created after `hello`

### 4. Delayed Transport Reclamation on Device Offline

After receiving an `offline` lifecycle message:

1. First mark broker offline
2. Immediately map device offline status
3. Start delayed cleanup timer
4. Retain `transport + udp session` during grace period
5. If `online` is received again within the grace period, cancel the cleanup timer and reuse the original transport

The default retention period is recommended as `2m`, and can be made configurable later.

## Online Status Semantics

MQTT-UDP device online status is now driven by MQTT lifecycle events, rather than by `ChatManager` creation/destruction.

That is:

- MQTT `online` -> Device online
- MQTT `offline` -> Device offline

To avoid duplicate notifications:

- `App.OnNewConnection()` maintains original logic for `websocket`
- `mqtt udp` `DeviceOnline / DeviceOffline` is now triggered by `MqttUdpAdapter` lifecycle callbacks

## Relationship with hello / listen

Existing chat signaling logic remains unchanged:

- Transport can pre-exist after MQTT connection is established
- `ChatManager` can pre-exist
- `ChatSession` is still created after successful `hello`
- `listen` still requires `hello` to be completed

This achieves "transport pre-creation" without changing session-layer semantics.

## MCP Warm-Up Strategy

A best-effort MCP warm-up is triggered when the lifecycle `online` event arrives.

At the same time, the existing MCP initialization fallback logic in `hello` is retained.

When both paths coexist, they rely on the MCP idempotency and state machine capabilities already present in the current branch to avoid duplicate initialization:

- Prioritize warm-up on online, improving console tool visibility
- `hello` continues as a fallback, preventing missing warm-up from affecting business

## High Concurrency and Debouncing

Maintain lifecycle state per device dimension:

- `brokerOnline`
- `lastEventTs`
- `cleanupTimer`
- `cleanupVersion`

Debouncing rules:

- Old timestamp events are ignored directly
- Duplicate `online` does not trigger duplicate online notification
- Duplicate `offline` only refreshes the cleanup timer, does not trigger duplicate offline notification
- Timer callback verifies `cleanupVersion` when executing, preventing old timers from mistakenly deleting new connections

## Points That Need to Be Fixed Together

Since transport is briefly retained after going offline, "current online transport" resolution cannot solely rely on whether `ChatManager` exists.

`MqttUdpConn` needs to expose broker online status, and `ChatManager.GetTransportType()` should return an empty string when MQTT transport is offline. This way, device-dimension MCP queries/calls still strictly depend on the "current online transport".

## Affected Files

- `internal/data/msg/message_types.go`
- `internal/app/mqtt_server/device_hook.go`
- `internal/app/mqtt_server/mqtt_server.go`
- `internal/app/server/mqtt_udp/mqtt_udp_adapter.go`
- `internal/app/server/mqtt_udp/mqtt_udp_conn.go`
- `internal/app/server/app.go`
- `internal/app/server/chat/chat.go`
