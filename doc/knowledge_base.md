# Knowledge Base Feature Documentation

This document introduces the **Knowledge Base (RAG)** feature in the project, including administrator-side provider configuration, regular user-side knowledge base and document management, recall testing, and knowledge base retrieval integration in the main program chat chain.

Related documents:

- [Management Console Guide](./manager_console_guide.md)
- [MCP Architecture Documentation](./mcp.md) (Knowledge base retrieval tool `search_knowledge` will be triggered through local tool chain)

---

## 1. Feature Overview

Knowledge base feature is used to provide agents with "document-based answer" capability, containing three layers:

1. Administrator configures knowledge base retrieval provider (Dify / RAGFlow / WeKnora)
2. Regular users create knowledge bases and documents, and asynchronously sync to provider
3. Agent associates knowledge base, triggers local `search_knowledge` tool to complete recall during dialogue

Currently supported providers in frontend management page:

- `dify`
- `ragflow`
- `weknora`

---

## 2. Role Division

### 2.1 Administrator

Responsible for:

- Configure knowledge base retrieval provider (global)
- Maintain provider connection parameters and default thresholds
- (Optional) Manage knowledge bases on behalf of users

Entry point:

- Administrator -> Knowledge Base Retrieval Configuration

### 2.2 Regular User

Responsible for:

- Create/edit/delete their own knowledge bases
- Manage knowledge base documents (text entry / file upload)
- Initiate manual sync and retry
- Use "Recall Test" to verify keyword hit effect
- Select associated knowledge base in agent

Entry point:

- Regular User -> My Knowledge Bases
- Regular User -> Agent Edit (Associate Knowledge Base)

---

## 3. Administrator: Knowledge Base Retrieval Configuration (Provider Configuration)

Management console supports maintaining multiple provider configurations and specifying the default provider.

Common configuration items (vary by provider):

- Base URL
- API Key / Token
- Default retrieval threshold
- Provider-specific parameters (such as RAGFlow similarity threshold, WeKnora chunk parameters, etc.)

### 3.1 Dify

Typical configuration items:

- base_url
- api_key
- score_threshold
- Other provider parameters

### 3.2 RAGFlow

Typical configuration items:

- base_url
- api_key
- similarity_threshold

### 3.3 WeKnora

Typical configuration items:

- base_url
- api_key
- score_threshold
- Chunk parameters (chunk_size / chunk_overlap / separators)
- Parsing polling parameters (parse_poll_interval_ms / parse_timeout_ms)

Management page also supports pulling WeKnora model list (embedding / llm / rerank) to assist with configuration.

---

## 4. Regular User: My Knowledge Bases (KB Management)

Entry point:

- Regular User -> My Knowledge Bases

Supported operations:

- Add/Edit knowledge base
- Set status (active / inactive)
- Set retrieval threshold (can inherit global)
- Document management
- Manual retry sync
- Recall test
- Delete knowledge base

### 4.1 Knowledge Base Fields (User Visible)

Common display columns:

- ID
- Name
- Description
- Provider
- Status
- Sync Status
- Last Sync Time
- Operations

Description:

- When sync fails, error information will be displayed in Sync Status column as tooltip to avoid table being too wide

### 4.2 Sync Status (Common)

Knowledge bases and documents may have similar statuses:

- Pending sync
- Uploading / Uploaded / Parsing
- Synced
- Failed (including upload failure, parsing failure, etc.)

If failed, you can click Retry Sync to re-queue async task.

---

## 5. Document Management (Under Knowledge Base)

Each knowledge base can contain multiple documents, supporting:

- Text-type documents (online editing)
- File upload to create documents (format limited by provider)

Page functions:

- Add document
- Edit document (file-type documents usually do not support online editing)
- Delete document
- Retry sync
- File upload

### 5.1 File Upload Format

Frontend will display different accept prompts and upload instructions based on current knowledge base provider:

- Dify: Supports common text/document formats (such as txt/md/pdf/html/xlsx/docx/csv, etc.)
- RAGFlow: Supports wider file types (including images, logs, configuration files, etc.)
- WeKnora: Supports wider file types (including Office, images, emails, etc.)

Specific uploadable formats please refer to page prompts.

---

## 6. Recall Test (User Side)

Knowledge base list can execute Recall Test on a single knowledge base, used to directly verify provider retrieval effect.

Test items:

- query: Test keyword or question
- top_k
- threshold (only effective for this test, can be empty)

Return content:

- Hit count
- Hit source (title)
- score
- Hit text snippet
- Response time

### 6.1 Threshold Priority (Logic Description)

Usually take threshold in the following priority:

1. This test request threshold (if filled)
2. Knowledge base own threshold
3. Provider global default threshold

### 6.2 WeKnora Parameter Description (Important)

Current WeKnora recall test already uses by knowledge base dimension:

- knowledge_base_ids (knowledge base ID list)

Used to precisely limit retrieval scope to current knowledge base.

---

## 7. Agent Associate Knowledge Base

In agent edit page, you can select multiple knowledge bases for the agent (multiple select).

Behavior description:

- Supports multiple library association
- During dialogue, will trigger knowledge base retrieval based on model judgment
- If specific knowledge base can be judged, tool call will pass knowledge_base_ids
- If retrieval fails, will degrade to normal LLM dialogue (frontend has prompt text)

---

## 8. Knowledge Base Retrieval in Main Program Dialogue Chain

Main program implements knowledge base retrieval through local tool search_knowledge.

Tool call parameter core fields:

- query
- top_k
- knowledge_base_ids (optional, knowledge base ID list)

Behavior description:

- Do not pass knowledge_base_ids: Retrieve in all available knowledge bases associated with current agent
- Pass knowledge_base_ids: Only retrieve within specified knowledge bases

This allows the model to narrow retrieval scope when the question attribution is known, improving relevance and reducing irrelevant recall.

### 8.1 WeKnora Main Program Retrieval Parameters

Current WeKnora main program retrieval request already uses:

- knowledge_base_ids

Consistent with console recall test.

---

## 9. Interface List (User Side)

### 9.1 Knowledge Base CRUD

- GET /user/knowledge-bases
- POST /user/knowledge-bases
- GET /user/knowledge-bases/:id
- PUT /user/knowledge-bases/:id
- DELETE /user/knowledge-bases/:id
- POST /user/knowledge-bases/:id/sync

### 9.2 Recall Test

- POST /user/knowledge-bases/:id/test-search

### 9.3 Document Management

- GET /user/knowledge-bases/:id/documents
- POST /user/knowledge-bases/:id/documents
- POST /user/knowledge-bases/:id/documents/upload
- PUT /user/knowledge-bases/:id/documents/:doc_id
- DELETE /user/knowledge-bases/:id/documents/:doc_id
- POST /user/knowledge-bases/:id/documents/:doc_id/sync

### 9.4 Agent Associate Knowledge Base

- GET /user/agents/:id/knowledge-bases
- PUT /user/agents/:id/knowledge-bases

---

## 10. Interface List (Administrator Side)

### 10.1 Provider Configuration Management

- GET /admin/knowledge-search-configs
- POST /admin/knowledge-search-configs
- PUT /admin/knowledge-search-configs/:id
- DELETE /admin/knowledge-search-configs/:id

### 10.2 WeKnora Model Pull (Configuration Assist)

- POST /admin/knowledge-search-configs/weknora/models

### 10.3 Administrator Manage Knowledge Bases on Behalf of Users (By User Dimension)

- GET /admin/users/:id/knowledge-bases
- POST /admin/users/:id/knowledge-bases
- PUT /admin/users/:id/knowledge-bases/:kb_id
- DELETE /admin/users/:id/knowledge-bases/:kb_id

---

## 11. FAQ and Troubleshooting

### 11.1 Knowledge base created but never hits

Priority check:

1. Whether knowledge base/document has synced successfully
2. Whether external provider has completed index building
3. Whether retrieval threshold is too high
4. Whether query is too broad or deviates from document content

### 11.2 Document cannot be edited after file upload

File upload created documents are usually handled as file-type documents, frontend will restrict online editing, recommend deleting and re-uploading.

### 11.3 WeKnora retrieval scope incorrect

Confirm:

- Whether console recall test uses current knowledge base to initiate test
- Whether knowledge_base_ids is correctly passed in agent tool call

---

## 12. Usage Suggestions

- Split multiple knowledge bases for different business domains (such as after-sales, products, contracts)
- Use Recall Test to adjust threshold first, then integrate into agent
- Clearly specify in agent description when knowledge base answers are needed, can improve trigger quality
