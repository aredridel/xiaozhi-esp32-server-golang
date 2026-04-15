# Knowledge Base Auto-Trigger Retrieval Plan (Device Chat, v2)

## Background
- Current retrieval tool `search_knowledge` mainly relies on whether the model actively calls it.
- Need to support "targeted retrieval by knowledge base ID" to reduce irrelevant knowledge base requests.

## Core Changes
1. Tool Parameter Upgrade
- `search_knowledge` adds optional field `knowledge_base_ids: number[]`.
- Retains `query`, `top_k`.
- Backward compatibility: When `knowledge_base_ids` is not provided, search all available knowledge bases for the current agent.

2. Targeted Retrieval Semantics
- When `knowledge_base_ids` is provided, only search within these knowledge bases.
- Invalid IDs (not associated/non-existent/inactive/missing external_kb_id) are automatically ignored (best effort).

3. Concurrent Execution Strategy
- Concurrent requests by "knowledge base dimension", each hit knowledge base initiates an independent retrieval request.
- Provider is still determined by the knowledge base's own configuration (dify/ragflow).
- Aggregate all hits, sort globally by score, then truncate to `top_k`.

4. Timeout Strategy (Confirmed Defaults)
- Single knowledge base timeout: `2500ms`
- Total timeout: `2500ms`
- Timeout/partial failure does not block main flow; returns error only if all fail.

5. LLM Routing Prompt Upgrade
- System Prompt sends "available knowledge base id:name" list.
- Guide model to pass `knowledge_base_ids` when determinable, can omit when uncertain.

## Implementation Steps
1. Add `knowledge_base_ids` to `search_knowledge` parameter structure.
2. Pass through `knowledge_base_ids` in the call chain `ChatSessionOperator -> LocalMcpSearchKnowledge -> rag.Search`.
3. Add ID filtering and total timeout control to `rag.Search`.
4. Modify `dify_searcher` and `ragflow_searcher` to search concurrently by knowledge base, with single knowledge base timeout control.
5. Adjust knowledge base retrieval rules in system prompt to support `knowledge_base_ids` guidance.

## Compatibility and Fallback
- Historical calls without `knowledge_base_ids` are not affected.
- Partial failure of any provider is only logged and skipped, preserving results from other providers.

## Acceptance Criteria
- Tool can receive and take effect `knowledge_base_ids`.
- Can perform concurrent retrieval in multi-knowledge-base scenarios and return aggregated results.
- Single knowledge base and total timeout both default to 2500ms.
- Old call paths (without `knowledge_base_ids`) maintain available behavior.
