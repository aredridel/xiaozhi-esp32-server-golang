# ChatSession Lazy Creation and Long-Term Reuse Refactor Plan

## Goal

- Change `ChatSession` from "create on connection establishment" to "lazy create after successful `hello`, with long-term reuse during the connection lifetime".
- `ChatSession.Close()` only releases chat-domain resources, no longer closes `serverTransport`, and no longer closes device-side IoT-over-MCP.
- Split `hello` into two phases by responsibility:
  - Transport-level: returns handshake information such as `transport/udp(server, port, key, nonce)`.
  - Chat-level: writes `audio_params`, initializes or reuses `SessionID`, refreshes device configuration, triggers session creation.
- Decouple `mcp/iot/goodbye` from `ChatSession`'s main execution path, handing them to `ChatManager` for processing.

## Design Boundaries

### ChatManager

- Connection-level owner.
- Holds `transport`, `serverTransport`, `clientState`, `mcpTransport`, `hookHub`, `transformRegistry`.
- Responsible for starting and holding the command loop and audio loop.
- Responsible for handling:
  - `hello`
  - `mcp`
  - `iot`
  - `goodbye`
- Routes `listen/abort`, calling `ensureSession()` when necessary.

### ChatSession

- Only responsible for the chat domain:
  - `listen`
  - `abort`
  - ASR/VAD
  - LLM/TTS
  - Session-level media playback
- `Start()` no longer starts connection-level `CmdMessageLoop/AudioMessageLoop`.
- `Start()` launches the following when input audio format is ready:
  - VAD/ASR background loop
  - `processChatText`
  - `llmManager.Start`
  - `ttsManager.Start`

## Lifecycle Conventions

- First `hello`:
  - Write `clientState.InputAudioFormat`
  - Create `SessionID`
  - Optionally initialize device-side MCP
  - `ensureSession()`
  - Reply `hello`
- Repeated `hello`:
  - Update `audio_params`
  - Refresh device configuration
  - If no active `ChatSession` exists, re-run `ensureSession()`
  - Optionally re-trigger device-side MCP initialization
- `mqtt_udp`:
  - Normal chat completion does not close transport
  - Explicit exit / fatal error only destroys `ChatSession`
  - Connection can continue to be reused and `ChatSession` rebuilt
- `websocket`:
  - After explicit exit / fatal error, `ChatManager` closes transport after session cleanup completes

## Code Change Points

- `internal/app/server/chat/chat.go`
  - `ChatManager` holds connection-level resources and message routing
  - Add `ensureSession()`, `HandleHelloMessage()`, connection-level `cmd/audio` loop
- `internal/app/server/chat/session.go`
  - `Start()` only retains chat-domain background tasks
  - `Close()` changed to pure chat resource release
  - Add session close callback for `ChatManager` to handle protocol-specific differences
- `internal/app/server/chat/server_transport.go`
  - Add a "close path without closing underlying transport" for scenarios where the remote end has disconnected
- `internal/app/server/event_handle.go`
  - Exit chat events now executed by `ChatManager` instead of directly using `ChatSession`

## Verification Points

- After a new connection is established, `ChatSession` is not created immediately.
- `ChatSession` is created after the first `hello`, and can continue with `listen/start`.
- Under `mqtt_udp`, after `ChatSession.Close()`, transport can still send and receive commands.
- Under `websocket`, the connection is closed after explicit exit.
- MCP tool lookup remains transport-aware and does not fall back to a no-transport dimension.
