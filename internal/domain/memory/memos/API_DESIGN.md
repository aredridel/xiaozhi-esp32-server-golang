# MemOS Standalone Provider API Integration Documentation (Based on Official Docs, Endpoint Can Be Overridden by Deployment)

> Official Documentation: `https://memos-docs.openmem.net/cn/api_docs/start/overview`
>
> Example base_url: `https://memos.memtensor.cn/api/openmem/v1`
>
> Goal: Integrate MemOS as a **standalone provider**, no longer sharing with `mem0`.

---

## 1. Principles

- No more guessing API paths.
- No more routing `memos` to `mem0` client.
- Only map fields and endpoints based on the official documentation you provide.

---

## 2. Current Repository Refactoring Constraints

The system interface `MemoryProvider` requires implementation of:

- `AddMessage`
- `GetMessages`
- `GetContext`
- `Search`
- `Flush`
- `ResetMemory`

Therefore, when integrating MemOS, you must find the corresponding API (or combination of APIs) in the official documentation item by item and complete the mapping.

---

## 3. Integration Points (According to Official Documentation)

The following fields are integrated according to fixed official paths, without exposing endpoint path editing in the console:

1. Authentication Method
   - Header Name:
   - Token Prefix (e.g., `Bearer `):

2. Write Memory API
   - Method + Path:
   - Request Body Example:
   - Key Response Fields:

3. Query Memory API
   - Method + Path:
   - Filter Parameters (agent_id / user_id / session_id etc):
   - Response Body Example:

4. Retrieve/Recall API
   - Method + Path:
   - Parameters (query/top_k/threshold/time_range):
   - Response Fields (text, score, timestamp):

5. Clear/Delete API
   - Method + Path:
   - Deletion Dimension (user-level/session-level/agent-level):

6. Whether Flush/Index Refresh API Exists
   - If not, how to degrade the semantics of `Flush`:

---

## 4. Code Implementation Plan (Execute After Confirmation)

```text
internal/domain/memory/memos/
  memos_client.go        # Real HTTP calls
  types.go               # Request/Response DTOs
  mapper.go              # API -> schema.Message
  memos_test.go          # httptest mock
```

And modify:

- `internal/domain/memory/base.go`
  - `MemoryTypeMemOS -> memos.GetWithConfig(config)`
- Admin config retains `memos` (already supported)
- Example config retains `memory.memos` (already supported)

---

## 5. Environment Notes

The current execution environment returns 403 when requesting the official documentation site, so the documentation content cannot be automatically crawled locally.

Currently implemented according to fixed paths, the console does not provide endpoint path editing.


## 6. Current Implementation Notes

- Actual request URL = `base_url + endpoint_path` (e.g., `http://host/api/v1` + `/add/message`).
- `memos_client.go` already implemented, default uses the following interfaces:
  - `/add/message`
  - `/get/messages`
  - `/search/memory`
  - `/flush`
  - `/reset/memory`
- Paths use fixed official semantics: `/add/message`, `/get/messages`, `/search/memory`, `/flush`, `/reset/memory`.


## 7. Add Message Field Constraints (Already Adjusted According to Documentation)

- `user_id` / `conversation_id` are required.
- `agent_id` is optional, only passed when it has a value.
- Current implementation maps `agentID` to both `user_id` and `conversation_id` at the same time; when `agentID` is empty, it directly reports an error and no longer uses default placeholder values.


## 8. Search Memory Field Mapping (Already Adjusted According to Documentation)

- Path: `/search/memory`
- `user_id`: mapped using `agentID`
- `conversation_id`: mapped using `agentID`
- `query`: passed through user input
- `memory_limit_number`: mapped from `topK`
- `relativity`: mapped from `search_threshold`
