<template>
  <div class="vp-docs">
    <aside class="vp-sidebar">
      <div class="vp-sidebar-title">OpenAPI Documentation</div>
      <a v-for="item in nav" :key="item.id" :href="`#${item.id}`" class="vp-nav-item">{{ item.label }}</a>
    </aside>

    <main class="vp-content">
      <header class="vp-hero">
        <h1>Xiaozhi OpenAPI Documentation</h1>
        <p class="lead">Publicly accessible, providing request methods, parameters, responses, and examples for each endpoint.</p>
        <div class="hero-meta">
          <span>Base URL: <code>/api/open/v1</code></span>
          <span>Content-Type: <code>application/json</code></span>
          <el-button size="small" type="primary" plain @click="$router.push('/login')">Back to Login</el-button>
        </div>
      </header>

      <section id="auth" class="vp-section">
        <h2>Authentication</h2>
        <pre><code>Authorization: Bearer &lt;jwt-or-api-token&gt;
X-API-Token: &lt;api-token&gt;</code></pre>
      </section>

      <section id="common" class="vp-section">
        <h2>Common Response Description</h2>
        <ul>
          <li>Common error codes: <code>400</code> Parameter error, <code>401</code> Authentication failed, <code>404</code> Resource not found, <code>500</code> Server error.</li>
          <li>Pagination defaults: <code>page=1</code>, <code>page_size=50</code>.</li>
        </ul>
      </section>

      <section id="profile" class="vp-section">
        <h2>1. Get Current User Info</h2>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/profile</code></div>
        <h4>Parameters</h4><p>None (authentication header required only).</p>
        <h4>Response Example</h4>
        <pre><code>{
  "user": {"id": 1, "username": "demo", "email": "demo@example.com", "role": "user"}
}</code></pre>
      </section>

      <section id="devices" class="vp-section">
        <h2>2. Device Endpoints</h2>

        <h3>2.1 Get Device List</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/devices</code></div>
        <h4>Parameters</h4><p>None (authentication header required only).</p>
        <h4>Response Example</h4>
        <pre><code>{"data":[{"id":1,"device_name":"bedroom","device_code":"123456","agent_id":2,"activated":true}]}</code></pre>

        <h3>2.2 Create Device</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/devices</code></div>
        <h4>Body Parameters</h4>
        <table><thead><tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>device_name</td><td>string</td><td>Yes</td><td>Device name, 2-50 characters</td></tr>
          <tr><td>agent_id</td><td>number</td><td>Yes</td><td>Bound agent ID</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"success":true,"message":"Device created successfully","data":{"device_code":"654321","device":{"id":8,"device_name":"bedroom"}}}</code></pre>
      </section>

      <section id="agents" class="vp-section">
        <h2>3. Agent Endpoints</h2>

        <h3>3.1 Get Agent List</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents</code></div>
        <h4>Parameters</h4><p>None (authentication header required only).</p>
        <h4>Response Example</h4>
        <pre><code>{"data":[{"id":2,"name":"Assistant A","status":"active","llm_config_id":"llm_default"}]}</code></pre>

        <h3>3.2 Create Agent</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/agents</code></div>
        <h4>Body Parameters</h4>
        <table><thead><tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>name</td><td>string</td><td>Yes</td><td>Name, 2-50 characters</td></tr>
          <tr><td>custom_prompt</td><td>string</td><td>No</td><td>Prompt</td></tr>
          <tr><td>llm_config_id</td><td>string</td><td>No</td><td>LLM configuration ID</td></tr>
          <tr><td>tts_config_id</td><td>string</td><td>No</td><td>TTS configuration ID</td></tr>
          <tr><td>voice</td><td>string</td><td>No</td><td>Voice identifier</td></tr>
          <tr><td>asr_speed</td><td>string</td><td>No</td><td>Default normal</td></tr>
          <tr><td>memory_mode</td><td>string</td><td>No</td><td>short/long/none</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"success":true,"data":{"id":3,"name":"Assistant B","status":"active"}}</code></pre>

        <h3>3.3 Get Agent Details</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Path Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>id</td><td>number</td><td>Yes</td><td>Agent ID</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"data":{"id":2,"name":"Assistant A","custom_prompt":"..."}}</code></pre>

        <h3>3.4 Update Agent</h3>
        <div class="api-line"><span class="method put">PUT</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Path Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>id</td><td>number</td><td>Yes</td><td>Agent ID</td></tr>
        </tbody></table>
        <h4>Body Parameters</h4>
        <table><thead><tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>name</td><td>string</td><td>Yes</td><td>Name, 2-50 characters</td></tr>
          <tr><td>custom_prompt</td><td>string</td><td>No</td><td>Prompt</td></tr>
          <tr><td>llm_config_id</td><td>string</td><td>No</td><td>LLM configuration ID (can be empty)</td></tr>
          <tr><td>tts_config_id</td><td>string</td><td>No</td><td>TTS configuration ID (can be empty)</td></tr>
          <tr><td>voice</td><td>string</td><td>No</td><td>Voice identifier</td></tr>
          <tr><td>asr_speed</td><td>string</td><td>No</td><td>Empty for normal</td></tr>
          <tr><td>memory_mode</td><td>string</td><td>No</td><td>short/long/none</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"data":{"id":2,"name":"Assistant A-Updated"}}</code></pre>

        <h3>3.5 Delete Agent</h3>
        <div class="api-line"><span class="method delete">DELETE</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Path Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>id</td><td>number</td><td>Yes</td><td>Agent ID</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"message":"Deleted successfully"}</code></pre>
      </section>

      <section id="history" class="vp-section">
        <h2>4. Chat History Endpoints</h2>

        <h3>4.1 Query Messages (Paginated)</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/history/messages</code></div>
        <h4>Query Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>agent_id</td><td>string</td><td>No</td><td>Agent ID</td></tr>
          <tr><td>device_id</td><td>string</td><td>No</td><td>Device identifier (device_name)</td></tr>
          <tr><td>session_id</td><td>string</td><td>No</td><td>Session ID</td></tr>
          <tr><td>role</td><td>string</td><td>No</td><td>user/assistant</td></tr>
          <tr><td>page</td><td>number</td><td>No</td><td>Default 1</td></tr>
          <tr><td>page_size</td><td>number</td><td>No</td><td>Default 50</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"total":120,"page":1,"page_size":50,"data":[{"id":1,"role":"user","content":"Hello"}]}</code></pre>

        <h3>4.2 Export Messages</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/history/export</code></div>
        <h4>Query Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>agent_id</td><td>string</td><td>No</td><td>Agent ID</td></tr>
          <tr><td>device_id</td><td>string</td><td>No</td><td>Device identifier (device_name)</td></tr>
          <tr><td>start_date</td><td>string</td><td>No</td><td>YYYY-MM-DD</td></tr>
          <tr><td>end_date</td><td>string</td><td>No</td><td>YYYY-MM-DD</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"export_time":"2026-03-17 10:00:00","total":20,"messages":[...]}</code></pre>
      </section>

      <section id="inject" class="vp-section">
        <h2>5. Message Injection Endpoint</h2>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/devices/inject-message</code></div>
        <h4>Body Parameters</h4>
        <table><thead><tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>device_id</td><td>string</td><td>Yes</td><td>Device identifier (device_name)</td></tr>
          <tr><td>message</td><td>string</td><td>Yes</td><td>Message content</td></tr>
          <tr><td>skip_llm</td><td>boolean</td><td>No</td><td>Whether to skip LLM, default false</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"success":true,"message":"Message injection request sent","data":{"device_id":"bedroom","message":"hello","skip_llm":false}}</code></pre>
      </section>

      <section id="mcp" class="vp-section">
        <h2>6. MCP Tool Endpoints</h2>

        <h3>6.1 Get Tool List</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents/:id/mcp-tools</code></div>
        <h4>Path Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>id</td><td>number</td><td>Yes</td><td>Agent ID</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"data":{"tools":[{"name":"tool_a","description":"..."}]}}</code></pre>

        <h3>6.2 Call Tool</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/agents/:id/mcp-call</code></div>
        <h4>Path Parameters</h4>
        <table><thead><tr><th>Parameter</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>id</td><td>number</td><td>Yes</td><td>Agent ID</td></tr>
        </tbody></table>
        <h4>Body Parameters</h4>
        <table><thead><tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
          <tr><td>tool_name</td><td>string</td><td>Yes</td><td>Tool name</td></tr>
          <tr><td>arguments</td><td>object</td><td>No</td><td>Tool arguments object</td></tr>
        </tbody></table>
        <h4>Response Example</h4>
        <pre><code>{"data":{"result":"ok"}}</code></pre>
      </section>
    </main>
  </div>
</template>

<script setup>
const nav = [
  { id: 'auth', label: 'Authentication' },
  { id: 'common', label: 'Common Info' },
  { id: 'profile', label: '1. User Info' },
  { id: 'devices', label: '2. Device Endpoints' },
  { id: 'agents', label: '3. Agent Endpoints' },
  { id: 'history', label: '4. Chat History' },
  { id: 'inject', label: '5. Message Injection' },
  { id: 'mcp', label: '6. MCP Tools' }
]
</script>

<style scoped>
.vp-docs { display: flex; gap: 24px; max-width: 1280px; margin: 0 auto; padding: 24px 16px 40px; color: #213547; }
.vp-sidebar { position: sticky; top: 20px; height: calc(100vh - 40px); min-width: 220px; border-right: 1px solid #e5e7eb; padding-right: 14px; display: flex; flex-direction: column; gap: 8px; }
.vp-sidebar-title { font-weight: 700; margin-bottom: 8px; }
.vp-nav-item { color: #4b5563; text-decoration: none; font-size: 14px; }
.vp-nav-item:hover { color: #3b82f6; }
.vp-content { flex: 1; min-width: 0; }
.vp-hero h1 { margin: 0; font-size: 32px; }
.lead { margin: 10px 0; color: #4b5563; }
.hero-meta { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.vp-section { margin-top: 26px; padding-top: 6px; border-top: 1px solid #f0f2f5; }
.vp-section h2 { margin: 0 0 10px; }
.vp-section h3 { margin: 20px 0 8px; }
.vp-section h4 { margin: 14px 0 6px; font-size: 14px; color: #374151; }
.api-line { display: flex; align-items: center; gap: 10px; margin: 8px 0; }
.method { color: #fff; font-size: 12px; border-radius: 6px; padding: 2px 8px; font-weight: 600; }
.method.get { background: #10b981; }
.method.post { background: #3b82f6; }
.method.put { background: #f59e0b; }
.method.delete { background: #ef4444; }
pre { margin: 6px 0; background: #0f172a; color: #e5e7eb; border-radius: 8px; padding: 12px; overflow: auto; font-size: 12px; }
code { background: #f3f4f6; padding: 2px 6px; border-radius: 4px; }
table { width: 100%; border-collapse: collapse; font-size: 14px; }
th, td { border: 1px solid #e5e7eb; padding: 8px; text-align: left; }
thead { background: #f8fafc; }
@media (max-width: 960px) {
  .vp-docs { display: block; }
  .vp-sidebar { position: static; height: auto; min-width: auto; border-right: none; border-bottom: 1px solid #e5e7eb; padding-bottom: 12px; margin-bottom: 16px; }
}
</style>
