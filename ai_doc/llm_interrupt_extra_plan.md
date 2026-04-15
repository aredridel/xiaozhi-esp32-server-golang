# LLM Interrupt Flag and History Content Enhancement Plan (Pending Confirmation)

## 1. Objectives

Implement two things in the current project:

1. When LLM streaming is interrupted midway, write the interrupt flag into the `Extra` of that assistant history message.  
2. When assembling LLM request history later, if this flag is detected, append `" [user interrupted]"` to the end of that message's `content` before sending to the model.

Note: This plan describes the implementation path first, without directly modifying code.

---

## 2. Current Code Path (Key Points)

- Interrupt trigger: `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/common.go:3`  
  `StopSpeaking()` will cancel `SessionCtx`, causing LLM/TTS processing context to end.

- LLM streaming processing and history persistence: `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go:323`  
  Current `handleLLMResponse()` only saves assistant message when `llmResponse.IsEnd=true`;  
  `ctx.Done()` branch returns directly, not saving the "already output but interrupted" assistant.

- History assembly entry: `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go:1050`  
  `GetMessages()` currently directly appends historical `msg` to request, without content enhancement based on `Extra`.

---

## 3. Design Principles

1. **Minimal intrusion**: Only modify history saving and history assembly logic in `llm.go`.  
2. **Do not pollute original history**: Copy message when assembling request before modifying `content`, do not modify historical objects in memory in place.  
3. **Avoid duplicate persistence**: Interrupt history persistence only happens once, and is mutually exclusive with normal `IsEnd` path.  
4. **Backward compatible**: History without `Extra.interrupt` maintains original behavior.

---

## 4. Plan Details

### 4.1 Write to Extra When Interrupted (LLM Stage)

Modification location: `handleLLMResponse()` at `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go:324`

New logic:

1. Introduce local state in function:
   - `assistantSaved bool`: Prevent duplicate saving in same processing.

2. Extract an internal helper (local closure in function or private method), executed when `ctx.Done()` triggers:
   - Get currently accumulated text from `fullText.String()`;
   - Skip if text is empty;
   - Construct `assistantMsg := schema.AssistantMessage(text, nil)`;
   - Set:
     - `assistantMsg.Extra["interrupt"] = true`
     - `assistantMsg.Extra["interrupt_by"] = "user"`
     - `assistantMsg.Extra["interrupt_stage"] = "llm"`
   - `AddLlmMessage(ctx, assistantMsg)` to save history.

3. Call this helper at multiple return points of `ctx.Done()`, then return.

Notes:
- Normal `IsEnd` path remains unchanged (no interrupt flag added).
- Only save when there is actually accumulated text, avoid empty assistant messages.

---

### 4.2 Enhance Content by Extra When Assembling History

Modification location: `GetMessages()` at `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go:1050`

New logic:

1. When iterating historical messages, do not directly append original `msg`, but do shallow copy first (copy necessary fields).  
2. If satisfied:
   - `msg.Role == schema.Assistant`
   - `msg.Extra != nil`
   - `msg.Extra["interrupt"] == true`
   - `msg.Content` is not empty
   
   Then change request-side content to:
   - `newMsg.Content = msg.Content + " [user interrupted]"`

3. To avoid duplicate appending, if content already ends with `" [user interrupted]"`, do not append again.

Note:
- Only modify `content` in "request assembly copy", do not modify original history.

---

### 4.3 Filter Trailing User in History (Avoid Polluting Current Round User)

Modification location: `GetMessages()` at `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go:1050`

New logic:

1. After `messageList := l.clientState.GetMessages(count)`, first check "last message in history":
   - If last message `Role == schema.User`, remove that message from `messageList`.

2. Only filter "trailing consecutive users":
   - Recommend looping backward from tail, deleting consecutive `user`, until tail is not `user` or list is empty.

Purpose:
- Prevent residual previous round user text in history from mixing with current round `userMessage`, polluting current session intent.

Note:
- This is "filter when assembling request", do not modify original historical data in memory.

---

## 5. Suggested Helper Functions

Recommended to place in `llm.go` private method area:

1. `isInterruptedMessage(msg *schema.Message) bool`  
   Uniformly determine `Extra.interrupt` (supports bool/string `"true"` fault tolerance).

2. `decorateInterruptedContent(content string) string`  
   Uniform append logic, avoid duplicate `" [user interrupted]"`.

3. `cloneMessageForRequest(msg *schema.Message) *schema.Message`  
   Copy `Role/Content/Name/ToolCalls/ToolCallID/Extra/ResponseMeta` (at minimum guarantee `Content` and `Extra` can be safely rewritten).

---

## 6. Compatibility and Risks

1. Whether `Extra` takes effect directly on model:  
   Current OpenAI adapter layer does not pass through `Extra` when assembling request, so model behavior is mainly determined by `" [user interrupted]"` we append to `content`.

2. History storage differences:  
   - In `redis` mode, `schema.Message` is directly JSON persisted, `Extra` can be preserved.  
   - In `manager` mode, currently only stores `role/content/tool_calls`, `Extra` may be lost.  
   Therefore if this capability is needed in manager mode in the future, manager history protocol needs to be extended simultaneously.

3. Copy impact:  
   `" [user interrupted]"` is an explicit prompt, will affect model continuation style; this is the expected behavior of this requirement.

---

## 7. Acceptance Criteria (Implement After Confirmation)

1. Scenario: user already in history, assistant streaming interrupted midway  
   - New assistant entry in history, `Extra.interrupt=true`.

2. Check request message before next round LLM  
   - Corresponding assistant content becomes `"<original fragment> [user interrupted]"`.

3. Non-interrupted completion message  
   - `Extra.interrupt` does not exist, `content` has no prefix.

4. When history tail is user, that tail user is filtered in request, not repeated/mixed with current round user.

5. No duplicate markers, no empty assistant records.

---

## 8. Implementation File List (After Confirmation)

- `/Users/shijingbo/git/xiaozhi-esp32-server-golang/internal/app/server/chat/llm.go`
- (Optional) `/Users/shijingbo/git/xiaozhi-esp32-server-golang/test/interrupt_history/main.go` for verification demo
