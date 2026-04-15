# WeKnora Knowledge Base Integration Plan (Pending Confirmation)

## 1. Objectives and Constraints
- Add third provider on top of existing `dify/ragflow`: `weknora`.
- Admin can configure WeKnora connection parameters; regular user-side knowledge base/document CRUD continues async sync.
- Main program local RAG retrieval pipeline supports `weknora` (does not callback console retrieval interface).
- Maintain existing data structure: no new database columns, document sync only uses `sync_status + sync_error`.
- Continue using existing deletion strategy: delete document; if knowledge base is auto-created by system and remote is empty, delete knowledge base.

## 2. Official API References
- API Overview and Authentication (`X-API-Key`, Base URL `/api/v1`):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/README.md>
- Knowledge Base Management:
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge-base.md>
- Knowledge Management (File/URL/Manual Knowledge, Parse Status):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge.md>
- Knowledge Search:
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/knowledge-search.md>
- Model Management (for getting default Embedding model):
  - <https://github.com/Tencent/WeKnora/blob/main/docs/api/model.md>

## 3. Interface Mapping (System Actions -> WeKnora API)
1. Create remote knowledge base (external_kb_id)  
`POST /api/v1/knowledge-bases`

2. Update remote knowledge base metadata (name/description/chunking config)  
`PUT /api/v1/knowledge-bases/:id`

3. Delete remote knowledge base  
`DELETE /api/v1/knowledge-bases/:id`

4. Create document (upload file)  
`POST /api/v1/knowledge-bases/:id/knowledge/file` (multipart)

5. Query document parse status  
`GET /api/v1/knowledge/:id` (uses `parse_status`, `pending/processing/failed/completed`)

6. Delete document  
`DELETE /api/v1/knowledge/:id`

7. Check if knowledge base is empty (for auto-delete)  
`GET /api/v1/knowledge-bases/:id/knowledge?page=1&page_size=1`

8. Search (recall test + main program RAG)  
`POST /api/v1/knowledge-search`

## 4. Data and Status Mapping
1. Field Mapping
- Local `knowledge_bases.external_kb_id` <-> WeKnora `knowledge_base.id`
- Local `knowledge_base_documents.external_doc_id` <-> WeKnora `knowledge.id`
- `sync_provider = "weknora"`

2. Document Sync Status (Single Field)
- After enqueue: `pending`
- Initiate upload: `uploading`
- Upload success: `uploaded`
- Parsing: `parsing`
- Parse success: `synced`
- Upload failed: `upload_failed`
- Parse failed: `parse_failed`
- Internal failure on enqueue: `failed`

3. Parse Polling Strategy (Recommended)
- `parse_poll_interval_ms`: default `1000`
- `parse_timeout_ms`: default `120000`
- Timeout handled as `parse_failed` and written to `sync_error`

## 5. Backend Modification Plan (manager/backend)
1. `manager/backend/controllers/knowledge_sync.go`
- Add `weknoraKnowledgeSyncConfig` and `parseWeknoraKnowledgeSyncConfig`.
- Add `weknora` branch to `syncKnowledgeBaseWithProvider/syncKnowledgeBaseDeleteBestEffort/syncKnowledgeDocumentBestEffort/syncKnowledgeDocumentDeleteBestEffort`.
- Add WeKnora HTTP wrapper (auth header `X-API-Key`, request/response log format consistent with existing).
- Document upload uniformly uses `/knowledge/file`:
  - File-type documents: forward upload directly.
  - Text-type documents: convert to UTF-8 `.md` temporary byte stream then upload.
- Document update adopts "create new then delete old" strategy, avoiding dependency on unstable update request body.
- After deleting document, query if remote knowledge base is empty, delete remote knowledge base if condition met.

2. `manager/backend/controllers/knowledge.go`
- Add `weknora` to provider validation in `CreateKnowledgeBaseDocumentByUpload`.
- Add `queryKnowledgeTestByWeknora` branch to recall test `TestKnowledgeBaseSearch`.
- Threshold handling follows current rules: request threshold > knowledge base threshold > global threshold.
- If WeKnora search interface lacks native threshold parameter, perform secondary filtering locally by `score`.

3. `manager/backend/controllers/admin.go`
- Existing `knowledge_search` aggregation structure already supports multiple providers, no schema change needed.
- Keep `knowledge.default_provider + knowledge.providers` output structure unchanged.

## 6. Main Program Modification Plan (internal/domain/rag)
1. Add `internal/domain/rag/weknora_searcher.go`
- Implement `Searcher` interface.
- Call `POST /api/v1/knowledge-search`, search precisely by `knowledge_base_ids`.
- Reuse existing concurrency, timeout, fault tolerance aggregation mechanism.
- Map hit results to `KnowledgeSearchHit`:
  - `Content <- content`
  - `Title <- knowledge_title` (fallback to local knowledge base name when empty)
  - `Score <- score`

2. Modify `internal/domain/rag/manager.go`
- Add `weknora` branch to `getSearcher()`.
- Provider config reading logic remains unchanged (read from `knowledge.providers.weknora`).

## 7. Admin Console Frontend Modifications (manager/frontend)
1. `manager/frontend/src/views/admin/KnowledgeSearchConfig.vue`
- Add `weknora` to provider dropdown.
- New configuration items (recommended):
  - `base_url` (default `http://127.0.0.1:8080`)
  - `api_key`
  - `score_threshold` (default `0.2`)
  - `chunk_size` (default `1000`)
  - `chunk_overlap` (default `200`)
  - `separators` (default `["\\n\\n","\\n",".","!","?",";",";"]`)
  - `enable_multimodal` (default `true`)
  - `embedding_model_id` (recommended required)
  - `summary_model_id` (optional)
  - `rerank_model_id` (optional)
  - `vlm_model_id` (optional)
  - `parse_poll_interval_ms`, `parse_timeout_ms` (optional)

2. `manager/frontend/src/views/user/KnowledgeBases.vue`
- No new column needed for provider display (provider field already exists).
- Add `weknora` branch to file upload `accept`.
- Add description text for WeKnora upload path and async parsing explanation.

## 8. Key Implementation Decisions (Recommend Confirmation)
1. Whether text documents must use manual interface
- Recommend first version uniformly uses `/knowledge/file` (text wrapped as `.md`), reducing interface differences and compatibility risks.

2. Embedding model source
- Recommend `embedding_model_id` as admin required field first.
- Optional enhancement: if empty, call `/api/v1/models` at startup to auto-select default `Embedding` model.

3. File format restrictions
- WeKnora documentation does not provide strict whitelist; recommend first version adopts "relaxed frontend restrictions + backend/remote fallback error reporting".
- If strict whitelist is needed, can converge based on tested stable formats in phase two.

## 9. Verification Checklist
1. Admin adds `weknora` configuration and sets as default.
2. After regular user creates knowledge base, auto-creates remote knowledge-base, writes back `external_kb_id`.
3. After adding text document/uploading file document, status changes as `uploading -> uploaded -> parsing -> synced`.
4. When parsing fails, `sync_status=parse_failed`, and writes to `sync_error`.
5. After deleting document, remote document is deleted; when remote library is empty, auto-deletes library according to policy.
6. Recall test can return WeKnora hit results, threshold takes effect locally.
7. Main program chat pipeline can auto-trigger retrieval when associated with `weknora` knowledge base.

## 10. Risks and Rollback
1. Risks
- WeKnora version differences cause request body field changes (especially knowledge base creation config fields).
- Document parsing takes long time, requires polling and timeout strategy coordination.
- If search interface lacks native threshold parameter, requires local secondary filtering.

2. Rollback
- Simply disable/delete `weknora` configuration to deactivate, does not affect existing `dify/ragflow`.
- Provider branch can be independently rolled back at code level, does not involve database schema changes.

