# Branch Code Review Findings (2026-04-12)

Branch: `codex/optimize-tool-invocation-for-concurrency`

## 1. [P1] Media Playback Failure Still Marked as Media Output

- Locations:
  - `internal/app/server/chat/tool.go:257`
  - `internal/app/server/chat/tool.go:271`
  - `internal/app/server/chat/tool.go:275`
  - `internal/app/server/chat/llm.go:873`
- Problem Description:
  - When `handleAudioContent` / `handleResourceLink` calls fail, the current logic still sets:
    - `execResult.hasMediaOutput = true`
    - `execResult.shouldStopLLMProcessing = true`
  - The upper layer interprets this as "media has been output", leading to the path that suppresses `tts_stop`/does not continue LLM.
- Risk Impact:
  - Media playback actually failed, but the conversation flow ends as if media output was successful, potentially causing client silence, state inconsistency, or no subsequent response.

## 2. [P2] Empty ToolCall ID Deduplication May Mistakenly Skip Legitimate Duplicate Calls

- Locations:
  - `internal/app/server/chat/tool.go:154`
  - `internal/app/server/chat/tool.go:160`
  - `internal/app/server/chat/tool.go:44`
  - `internal/app/server/chat/tool.go:67`
- Problem Description:
  - Currently, empty `ToolCall.ID` uses `auto_<name>_<arguments>` to generate an identifier for deduplication.
  - If the model legitimately produces two calls "without ID and with same parameters", the second one will be skipped.
- Risk Impact:
  - May cause inconsistency between the number of `tool_calls` in assistant and subsequent `tool_result`, affecting context in subsequent rounds and tool invocation reliability.

