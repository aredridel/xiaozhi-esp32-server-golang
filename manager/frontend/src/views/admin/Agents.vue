<template>
  <div class="admin-agents">
    <div class="page-header">
      <h2>Agent Management</h2>
      <p class="page-subtitle">Manage all agents in the system</p>
    </div>

    <div class="toolbar">
      <el-button type="primary" @click="showAddDialog = true">
        <el-icon><Plus /></el-icon>
        Add Agent
      </el-button>
      <el-button @click="loadAgents">
        <el-icon><Refresh /></el-icon>
        Refresh
      </el-button>
    </div>

    <el-table :data="agents" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Name" width="150" />
      <el-table-column prop="user_id" label="User ID" width="100" />
      <el-table-column label="Role Description" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.custom_prompt || 'Not Set' }}
        </template>
      </el-table-column>
      <el-table-column label="Language Model" width="150">
        <template #default="{ row }">
          {{ row.llm_config?.name || 'Not Set' }}
        </template>
      </el-table-column>
      <el-table-column label="Voice" width="150">
        <template #default="{ row }">
          {{ row.tts_config?.name || 'Not Set' }}
        </template>
      </el-table-column>
      <el-table-column label="ASR Speed" width="120">
        <template #default="{ row }">
          <el-tag :type="getASRSpeedType(row.asr_speed)">
            {{ getASRSpeedText(row.asr_speed) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Memory Mode" width="120">
        <template #default="{ row }">
          <el-tag :type="getMemoryModeType(row.memory_mode)">
            {{ getMemoryModeText(row.memory_mode) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Speaker Chat" width="180">
        <template #default="{ row }">
          <el-tag :type="getSpeakerChatModeType(row.speaker_chat_mode)">
            {{ getSpeakerChatModeText(row.speaker_chat_mode) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="Status" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">
            {{ row.status === 'active' ? 'Active' : 'Inactive' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="280">
        <template #default="{ row }">
          <el-button size="small" @click="editAgent(row)">
            Edit
          </el-button>
          <el-button size="small" type="primary" @click="showMCPEndpoint(row)">
            MCP Endpoint
          </el-button>
          <el-button size="small" type="danger" @click="deleteAgent(row)">
            Delete
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Add/Edit Agent Dialog -->
    <el-dialog
      v-model="showAddDialog"
      :title="editingAgent ? 'Edit Agent' : 'Add Agent'"
      width="600px"
    >
      <el-form :model="agentForm" :rules="agentRules" ref="agentFormRef" label-width="120px">
        <el-form-item label="User ID" prop="user_id">
          <el-input-number v-model="agentForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="Name" prop="name">
          <el-input v-model="agentForm.name" placeholder="Enter agent name" />
        </el-form-item>
        <el-form-item label="Role Description" prop="custom_prompt">
          <el-input
            v-model="agentForm.custom_prompt"
            type="textarea"
            :rows="4"
            placeholder="Enter role description/system prompt"
          />
        </el-form-item>
        <el-form-item label="Language Model" prop="llm_config_id">
          <el-select v-model="agentForm.llm_config_id" placeholder="Select language model" style="width: 100%">
            <el-option 
              v-for="config in llmConfigs" 
              :key="config.config_id" 
              :label="config.name" 
              :value="config.config_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Voice" prop="tts_config_id">
          <el-select v-model="agentForm.tts_config_id" placeholder="Select voice" style="width: 100%">
            <el-option 
              v-for="config in ttsConfigs" 
              :key="config.config_id" 
              :label="config.name" 
              :value="config.config_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="ASR Speed" prop="asr_speed">
          <el-select v-model="agentForm.asr_speed" style="width: 100%">
            <el-option label="Normal" value="normal" />
            <el-option label="Patient" value="patient" />
            <el-option label="Fast" value="fast" />
          </el-select>
        </el-form-item>
        <el-form-item label="Memory Mode" prop="memory_mode">
          <el-select v-model="agentForm.memory_mode" style="width: 100%">
            <el-option label="No Memory" value="none" />
            <el-option label="Short Memory" value="short" />
            <el-option label="Long Memory" value="long" />
          </el-select>
        </el-form-item>
        <el-form-item label="Speaker Chat Only" prop="speaker_chat_mode">
          <el-select v-model="agentForm.speaker_chat_mode" style="width: 100%">
            <el-option label="Off" value="off" />
            <el-option label="Only when speaker identified" value="identified_only" />
          </el-select>
        </el-form-item>
        <el-form-item label="MCP Services">
          <el-input
            v-model="agentForm.mcp_service_names"
            clearable
            placeholder="Separate multiple services with commas, leave empty to use all enabled services"
          />
          <div style="margin-top: 6px; color: #909399; font-size: 12px;">
            Example: Discovery Report, Amap. Leaving empty will use all enabled services.
          </div>
        </el-form-item>
        <el-form-item label="OpenClaw">
          <el-button type="primary" size="large" style="width: 100%" @click="showOpenClawSettings">
            View OpenClaw
          </el-button>
          <div style="margin-top: 6px; color: #909399; font-size: 12px;">
            Configured: {{ agentForm.openclaw_allowed ? 'On' : 'Off' }}, Enter keywords: {{ agentForm.openclaw_enter_keywords.length }}, Exit keywords: {{ agentForm.openclaw_exit_keywords.length }}.
          </div>
        </el-form-item>
        <el-form-item label="Status" prop="status">
          <el-select v-model="agentForm.status" style="width: 100%">
            <el-option label="Active" value="active" />
            <el-option label="Inactive" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">Cancel</el-button>
        <el-button type="primary" @click="saveAgent" :loading="saving">
          {{ editingAgent ? 'Update' : 'Add' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showOpenClawDialog"
      title="OpenClaw Settings"
      width="680px"
    >
      <div>
        <div class="openclaw-tip-row">
          <span class="openclaw-tip-title">Integration Tips</span>
          <el-tooltip effect="light" placement="top-start" :show-after="200" :enterable="true" popper-class="openclaw-tip-popper">
            <template #content>
              <div class="openclaw-tip-content">
                <div>Architecture: Device Voice -> Server Routing -> OpenClaw Session -> Xiaozhi Plugin.</div>
                <div>Role Configuration: Use the four commands below in the OpenClaw console role configuration, then execute `openclaw gateway restart` to apply changes.</div>
                <div>Entry Logic: After matching entry keywords (default "Open Lobster/Enter Lobster"), enter OpenClaw mode; subsequent text will be routed to OpenClaw first.</div>
                <div>Exit Logic: In OpenClaw mode, matching exit keywords (default "Close Lobster/Exit Lobster") will exit and return to normal LLM conversation.</div>
                <el-link :href="openClawDocURL" target="_blank" type="primary" :underline="false">
                  View Full Documentation
                </el-link>
              </div>
            </template>
            <el-icon class="openclaw-tip-icon"><QuestionFilled /></el-icon>
          </el-tooltip>
        </div>

        <el-form label-width="100px">
          <el-form-item label="Switch">
            <el-switch v-model="agentForm.openclaw_allowed" />
          </el-form-item>
          <el-form-item label="Entry Keywords">
            <el-select
              v-model="agentForm.openclaw_enter_keywords"
              multiple
              filterable
              allow-create
              default-first-option
              clearable
              style="width: 100%"
              placeholder="Press Enter after typing, can add multiple keywords"
            />
          </el-form-item>
          <el-form-item label="Exit Keywords">
            <el-select
              v-model="agentForm.openclaw_exit_keywords"
              multiple
              filterable
              allow-create
              default-first-option
              clearable
              style="width: 100%"
              placeholder="Press Enter after typing, can add multiple keywords"
            />
          </el-form-item>
        </el-form>

        <el-divider />

        <div v-loading="openClawEndpointLoading">
          <div class="openclaw-status-bar">
            <div class="endpoint-label">Connection Status:</div>
            <el-tag :type="openClawStatusTagType">{{ openClawStatusText }}</el-tag>
          </div>
          <div v-if="openClawEndpointData.status_message" class="openclaw-status-message">
            {{ openClawEndpointData.status_message }}
          </div>
          <div class="mcp-endpoint-display">
            <div class="endpoint-header">
              <div class="endpoint-label">OpenClaw Role Configuration Commands:</div>
              <div class="endpoint-actions">
                <el-button size="small" @click="fetchOpenClawEndpoint" :disabled="!editingAgent" :loading="openClawEndpointLoading">Refresh</el-button>
                <el-button size="small" type="primary" @click="copyOpenClawCommands" :disabled="!openClawCommandData.ready">Copy Commands</el-button>
              </div>
            </div>
            <div v-if="openClawCommandData.ready" class="openclaw-command-hint">Execute the following commands in the OpenClaw console role configuration:</div>
            <div v-if="openClawCommandData.ready" class="openclaw-command-steps">
              <div
                v-for="(step, index) in openClawCommandData.steps"
                :key="`${step.title}-${index}`"
                class="openclaw-command-step"
              >
                <div class="openclaw-command-step-title">Line {{ index + 1 }}: {{ step.title }}</div>
                <pre class="openclaw-command-content">{{ step.command }}</pre>
              </div>
            </div>
            <pre v-else class="openclaw-command-content">{{ openClawCommandDisplayText }}</pre>
          </div>
        </div>

        <el-divider />
        <el-alert
          title="Conversation Test"
          description="Send a text test request to OpenClaw and view the response."
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 12px"
        />
        <el-form label-width="100px">
          <el-form-item label="Test Message">
            <el-input
              v-model="openClawChatTestForm.message"
              type="textarea"
              :rows="3"
              placeholder="Enter test message"
            />
          </el-form-item>
        </el-form>
        <el-button
          type="primary"
          @click="testOpenClawChat"
          :loading="openClawChatTesting"
          :disabled="!editingAgent"
        >
          Send Test
        </el-button>
        <div class="mcp-result-box">{{ openClawChatTestResult || 'No test results yet' }}</div>
      </div>
      <template #footer>
        <el-button @click="showOpenClawDialog = false">Close</el-button>
      </template>
    </el-dialog>

    <!-- MCP Endpoint Dialog -->
    <el-dialog
      v-model="showMCPDialog"
      title="MCP Endpoint"
      width="700px"
    >
      <div v-loading="mcpLoading">
        <!-- Tools List Section -->
        <div class="mcp-tools-section">
          <div class="tools-header">
            <div class="tools-title">MCP Tools List</div>
            <el-button 
              size="small" 
              type="primary" 
              @click="refreshMcpTools"
              :loading="toolsLoading"
            >
              <el-icon><Refresh /></el-icon>
              Refresh Tools List
            </el-button>
          </div>
          
          <div class="tools-list">
            <div v-if="mcpTools.length === 0" class="tools-empty">
              <el-tag type="info" size="large" class="tool-tag">
                No tools data available
              </el-tag>
            </div>
            
            <div v-else class="tools-tags">
              <el-tag
                v-for="tool in mcpTools"
                :key="tool.name"
                :type="tool.schema ? 'success' : 'info'"
                size="large"
                class="tool-tag"
                :title="tool.description"
              >
                {{ tool.name }}
                <el-tooltip
                  v-if="tool.description"
                  :content="tool.description"
                  placement="top"
                  :show-after="500"
                >
                  <el-icon class="tool-info-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </el-tag>
            </div>
          </div>
        </div>

        <el-alert
          title="Endpoint Information"
          description="This is the agent's MCP WebSocket endpoint URL, which can be used for device connection"
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 20px; margin-top: 24px;"
        />
        
        <div class="mcp-endpoint-display">
          <div class="endpoint-header">
            <div class="endpoint-label">MCP Endpoint URL:</div>
            <el-button size="small" type="primary" @click="copyMCPEndpoint">Copy URL</el-button>
          </div>
          <div class="endpoint-content">
            {{ mcpEndpointData.endpoint }}
          </div>
        </div>

        <el-divider />
        <el-form :model="mcpCallForm" label-width="90px">
          <el-form-item label="Tool">
            <el-select v-model="mcpCallForm.tool_name" placeholder="Select tool" style="width: 100%" @change="handleMcpToolChange">
              <el-option v-for="tool in mcpTools" :key="tool.name" :label="tool.name" :value="tool.name" />
            </el-select>
          </el-form-item>
          <el-form-item label="Params JSON">
            <el-input v-model="mcpCallForm.argumentsText" type="textarea" :rows="6" placeholder='e.g.: {"query":"hello"}' />
          </el-form-item>
        </el-form>
        <el-button type="primary" @click="callAgentMcpTool" :loading="callingTool">Call Tool</el-button>
        <div class="endpoint-content" style="margin-top: 12px">{{ mcpCallResult || "No call results yet" }}</div>

      </div>
      
      <template #footer>
        <el-button @click="showMCPDialog = false">Close</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, InfoFilled, QuestionFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { postJSONWithSSE } from '../../utils/sse'
import { buildOpenClawCommands } from '../../utils/openclaw'

const agents = ref([])
const llmConfigs = ref([])
const ttsConfigs = ref([])
const loading = ref(false)
const showAddDialog = ref(false)
const editingAgent = ref(null)
const saving = ref(false)
const agentFormRef = ref()

// MCP endpoint related
const showMCPDialog = ref(false)
const mcpLoading = ref(false)
const mcpEndpointData = ref({
  endpoint: ''
})

// MCP tools related
const toolsLoading = ref(false)
const mcpTools = ref([])
const currentAgentId = ref(null)
const callingTool = ref(false)
const mcpCallResult = ref('')
const mcpCallForm = ref({ tool_name: '', argumentsText: '{}' })
const showOpenClawDialog = ref(false)
const openClawEndpointLoading = ref(false)
const openClawEndpointData = ref({
  endpoint: '',
  connected: false,
  status: 'unknown',
  status_message: ''
})
const openClawChatTesting = ref(false)
const openClawChatTestResult = ref('')
const openClawChatTestForm = ref({
  message: ''
})
const openClawStatusText = computed(() => {
  const status = String(openClawEndpointData.value.status || '').toLowerCase()
  if (status === 'online') return 'Connected'
  if (status === 'offline') return 'Disconnected'
  return 'Unknown Status'
})
const openClawStatusTagType = computed(() => {
  const status = String(openClawEndpointData.value.status || '').toLowerCase()
  if (status === 'online') return 'success'
  if (status === 'offline') return 'danger'
  return 'info'
})
const openClawCommandData = computed(() => buildOpenClawCommands(openClawEndpointData.value.endpoint))
const openClawCommandDisplayText = computed(() => {
  if (openClawCommandData.value.ready) {
    return openClawCommandData.value.copyText
  }
  if (!editingAgent.value?.id) {
    return 'Installation commands are not yet generated for new agents. Save to view them.'
  }
  return 'No installation commands available. Please refresh and try again.'
})
const OPENCLAW_DEFAULT_ENTER_KEYWORDS = ['Open Lobster', 'Enter Lobster']
const OPENCLAW_DEFAULT_EXIT_KEYWORDS = ['Close Lobster', 'Exit Lobster']
const openClawDocURL = 'https://github.com/hackers365/xiaozhi-esp32-server-golang/blob/main/doc/openclaw_integration.md'

const agentForm = ref({
  user_id: null,
  name: '',
  custom_prompt: '',
  llm_config_id: null,
  tts_config_id: null,
  asr_speed: 'normal',
  memory_mode: 'short',
  speaker_chat_mode: 'off',
  mcp_service_names: '',
  openclaw_allowed: false,
  openclaw_enter_keywords: [...OPENCLAW_DEFAULT_ENTER_KEYWORDS],
  openclaw_exit_keywords: [...OPENCLAW_DEFAULT_EXIT_KEYWORDS],
  status: 'active'
})

const agentRules = {
  user_id: [{ required: true, message: 'Please enter user ID', trigger: 'blur' }],
  name: [{ required: true, message: 'Please enter agent name', trigger: 'blur' }],
  asr_speed: [{ required: true, message: 'Please select ASR speed', trigger: 'change' }],
  memory_mode: [{ required: true, message: 'Please select memory mode', trigger: 'change' }],
  speaker_chat_mode: [{ required: true, message: 'Please select speaker chat restriction', trigger: 'change' }],
  status: [{ required: true, message: 'Please select status', trigger: 'change' }]
}

const loadAgents = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/agents')
    agents.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load agent list')
    console.error('Error loading agents:', error)
  } finally {
    loading.value = false
  }
}

const loadConfigs = async () => {
  try {
    const [llmResponse, ttsResponse] = await Promise.all([
      api.get('/admin/llm-configs'),
      api.get('/admin/tts-configs')
    ])
    llmConfigs.value = llmResponse.data.data || []
    ttsConfigs.value = ttsResponse.data.data || []
    
    // Sort configs, default configs first
    llmConfigs.value.sort((a, b) => {
      if (a.is_default && !b.is_default) return -1
      if (!a.is_default && b.is_default) return 1
      return a.name.localeCompare(b.name)
    })
    
    ttsConfigs.value.sort((a, b) => {
      if (a.is_default && !b.is_default) return -1
      if (!a.is_default && b.is_default) return 1
      return a.name.localeCompare(b.name)
    })
  } catch (error) {
    console.error('Error loading configs:', error)
  }
}

const editAgent = (agent) => {
  editingAgent.value = agent
  const openclawConfig = parseOpenClawConfigFromAgent(agent)
  agentForm.value = {
    user_id: agent.user_id,
    name: agent.name,
    custom_prompt: agent.custom_prompt || '',
    llm_config_id: agent.llm_config_id,
    tts_config_id: agent.tts_config_id,
    asr_speed: agent.asr_speed || 'normal',
    memory_mode: agent.memory_mode || 'short',
    speaker_chat_mode: agent.speaker_chat_mode || 'off',
    mcp_service_names: agent.mcp_service_names || '',
    openclaw_allowed: !!openclawConfig.allowed,
    openclaw_enter_keywords: normalizeKeywordList(openclawConfig.enter_keywords),
    openclaw_exit_keywords: normalizeKeywordList(openclawConfig.exit_keywords),
    status: agent.status
  }
  showAddDialog.value = true
  openClawEndpointData.value = {
    endpoint: '',
    connected: false,
    status: 'unknown',
    status_message: ''
  }
}

const saveAgent = async () => {
  if (!agentFormRef.value) return
  
  const valid = await agentFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const payload = {
      ...agentForm.value,
      openclaw: {
        allowed: !!agentForm.value.openclaw_allowed,
        enter_keywords: normalizeKeywordList(agentForm.value.openclaw_enter_keywords),
        exit_keywords: normalizeKeywordList(agentForm.value.openclaw_exit_keywords)
      }
    }
    delete payload.openclaw_allowed
    delete payload.openclaw_enter_keywords
    delete payload.openclaw_exit_keywords

    if (editingAgent.value) {
      await api.put(`/admin/agents/${editingAgent.value.id}`, payload)
      ElMessage.success('Agent updated successfully')
    } else {
      await api.post('/admin/agents', payload)
      ElMessage.success('Agent added successfully')
    }
    showAddDialog.value = false
    resetForm()
    loadAgents()
  } catch (error) {
    ElMessage.error(editingAgent.value ? 'Failed to update agent' : 'Failed to add agent')
    console.error('Error saving agent:', error)
  } finally {
    saving.value = false
  }
}

const deleteAgent = async (agent) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete agent "${agent.name}"?`,
      'Confirm Delete',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/agents/${agent.id}`)
    ElMessage.success('Agent deleted successfully')
    loadAgents()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Failed to delete agent')
      console.error('Error deleting agent:', error)
    }
  }
}

const resetForm = () => {
  editingAgent.value = null
  openClawEndpointData.value = {
    endpoint: '',
    connected: false,
    status: 'unknown',
    status_message: ''
  }
  openClawChatTestResult.value = ''
  openClawChatTestForm.value.message = ''
  agentForm.value = {
    user_id: null,
    name: '',
    custom_prompt: '',
    llm_config_id: null,
    tts_config_id: null,
    asr_speed: 'normal',
    memory_mode: 'short',
    speaker_chat_mode: 'off',
    mcp_service_names: '',
    openclaw_allowed: false,
    openclaw_enter_keywords: [...OPENCLAW_DEFAULT_ENTER_KEYWORDS],
    openclaw_exit_keywords: [...OPENCLAW_DEFAULT_EXIT_KEYWORDS],
    status: 'active'
  }
  
  // Auto-select default config for new agents
  if (!editingAgent.value) {
    const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
    const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
    
    if (defaultLlmConfig) {
      agentForm.value.llm_config_id = defaultLlmConfig.config_id
    }
    if (defaultTtsConfig) {
      agentForm.value.tts_config_id = defaultTtsConfig.config_id
    }
  }
  
  if (agentFormRef.value) {
    agentFormRef.value.resetFields()
  }
}

const getASRSpeedText = (speed) => {
  const speedMap = {
    'normal': 'Normal',
    'patient': 'Patient',
    'fast': 'Fast'
  }
  return speedMap[speed] || 'Normal'
}

const getASRSpeedType = (speed) => {
  const typeMap = {
    'normal': '',
    'patient': 'warning',
    'fast': 'success'
  }
  return typeMap[speed] || ''
}

const getMemoryModeText = (mode) => {
  const modeMap = {
    none: 'No Memory',
    short: 'Short Memory',
    long: 'Long Memory'
  }
  return modeMap[mode] || 'Short Memory'
}

const getMemoryModeType = (mode) => {
  const typeMap = {
    none: 'info',
    short: '',
    long: 'success'
  }
  return typeMap[mode] || ''
}

const getSpeakerChatModeText = (mode) => {
  const modeMap = {
    off: 'Off',
    identified_only: 'Speaker ID Only'
  }
  return modeMap[mode] || 'Off'
}

const getSpeakerChatModeType = (mode) => {
  const typeMap = {
    off: 'info',
    identified_only: 'warning'
  }
  return typeMap[mode] || 'info'
}

const normalizeKeywordList = (keywords) => {
  if (!Array.isArray(keywords)) return []
  const unique = []
  const seen = new Set()
  for (const item of keywords) {
    const keyword = String(item || '').trim()
    if (!keyword || seen.has(keyword)) continue
    seen.add(keyword)
    unique.push(keyword)
  }
  return unique
}

const buildDefaultOpenClawConfig = () => ({
  allowed: false,
  enter_keywords: [...OPENCLAW_DEFAULT_ENTER_KEYWORDS],
  exit_keywords: [...OPENCLAW_DEFAULT_EXIT_KEYWORDS]
})

const normalizeOpenClawConfig = (raw) => {
  const enterKeywords = normalizeKeywordList(raw?.enter_keywords)
  const exitKeywords = normalizeKeywordList(raw?.exit_keywords)
  return {
    allowed: !!raw?.allowed,
    enter_keywords: enterKeywords.length > 0 ? enterKeywords : [...OPENCLAW_DEFAULT_ENTER_KEYWORDS],
    exit_keywords: exitKeywords.length > 0 ? exitKeywords : [...OPENCLAW_DEFAULT_EXIT_KEYWORDS]
  }
}

const parseOpenClawConfigFromAgent = (agent) => {
  if (agent && agent.openclaw && typeof agent.openclaw === 'object') {
    return normalizeOpenClawConfig(agent.openclaw)
  }

  if (!agent || !agent.openclaw_config || typeof agent.openclaw_config !== 'string') {
    return buildDefaultOpenClawConfig()
  }

  try {
    const parsed = JSON.parse(agent.openclaw_config)
    if (parsed && typeof parsed === 'object') {
      return normalizeOpenClawConfig(parsed)
    }
  } catch (_) {
    // ignore invalid payload
  }

  return buildDefaultOpenClawConfig()
}

const fetchOpenClawEndpoint = async () => {
  if (!editingAgent.value?.id) {
    openClawEndpointData.value = {
      endpoint: '',
      connected: false,
      status: 'unknown',
      status_message: 'Endpoint not yet generated for new agents. Save to view it.'
    }
    return
  }
  openClawEndpointLoading.value = true
  try {
    const response = await api.get(`/admin/agents/${editingAgent.value.id}/openclaw-endpoint`)
    const data = response.data?.data || {}
    const connected = !!data.connected
    const status = String(data.status || '').trim().toLowerCase()
    openClawEndpointData.value.endpoint = data.endpoint || ''
    openClawEndpointData.value.connected = connected
    openClawEndpointData.value.status = status || (connected ? 'online' : 'offline')
    openClawEndpointData.value.status_message = typeof data.status_message === 'string' ? data.status_message : ''
  } catch (error) {
    console.error('Failed to get OpenClaw endpoint:', error)
    openClawEndpointData.value.endpoint = ''
    openClawEndpointData.value.connected = false
    openClawEndpointData.value.status = 'unknown'
    openClawEndpointData.value.status_message = error.response?.data?.error || ''
    ElMessage.error('Failed to get OpenClaw endpoint')
  } finally {
    openClawEndpointLoading.value = false
  }
}

const copyOpenClawCommands = async () => {
  const commands = openClawCommandData.value.copyText
  if (!commands) {
    ElMessage.warning('No OpenClaw role configuration commands available to copy')
    return
  }
  try {
    await navigator.clipboard.writeText(commands)
    ElMessage.success('OpenClaw role configuration commands copied')
  } catch (error) {
    console.error('Failed to copy OpenClaw role configuration commands:', error)
    ElMessage.error('Copy failed, please copy manually')
  }
}

const showOpenClawSettings = async () => {
  showOpenClawDialog.value = true
  openClawChatTestResult.value = ''
  if (editingAgent.value?.id) {
    await fetchOpenClawEndpoint()
  } else {
    openClawEndpointData.value = {
      endpoint: '',
      connected: false,
      status: 'unknown',
      status_message: 'Endpoint not yet generated for new agents. Save to view it.'
    }
  }
}

const formatOpenClawChatResult = (reply, latency) => {
  const lines = [`Reply: ${String(reply || '') || '(empty)'}`]
  if (Number.isFinite(latency)) {
    lines.push(`Time: ${latency}ms`)
  }
  return lines.join('\n')
}

const testOpenClawChat = async () => {
  if (!editingAgent.value?.id) {
    ElMessage.warning('Please save the agent first before testing')
    return
  }

  const message = String(openClawChatTestForm.value.message || '').trim()
  if (!message) {
    ElMessage.warning('Please enter test message')
    return
  }

  openClawChatTesting.value = true
  openClawChatTestResult.value = 'Connecting...'
  try {
    const requestTimeoutMs = 610000
    const timeoutMs = 600000
    const token = String(localStorage.getItem('token') || '')
    const chunks = []
    let finalData = null
    let streamError = ''

    const normalizePayload = (payload) => (payload && typeof payload === 'object' ? payload : {})

    const response = await postJSONWithSSE({
      url: `/api/admin/agents/${editingAgent.value.id}/openclaw-chat-test?stream=1`,
      body: {
        message,
        timeout_ms: timeoutMs
      },
      timeoutMs: requestTimeoutMs,
      token,
      onEvent: (event, payload) => {
        const envelope = normalizePayload(payload)
        if (event === 'start') {
          openClawChatTestResult.value = 'Connected, waiting for reply...'
          return
        }
        if (event === 'chunk') {
          const data = normalizePayload(envelope.data)
          const chunk = typeof data.chunk === 'string' ? data.chunk : ''
          if (chunk) {
            chunks.push(chunk)
          }
          const reply = String(data.reply || chunks.join(''))
          const latency = Number(data.latency_ms)
          openClawChatTestResult.value = `Streaming reply...\n${formatOpenClawChatResult(reply, latency)}`
          return
        }
        if (event === 'result') {
          finalData = normalizePayload(envelope.data)
          const reply = String(finalData.reply || chunks.join(''))
          const latency = Number(finalData.latency_ms)
          openClawChatTestResult.value = formatOpenClawChatResult(reply, latency)
          return
        }
        if (event === 'error') {
          const data = normalizePayload(envelope.data)
          const messageText = String(envelope.error || data.error || 'OpenClaw conversation test failed')
          const partialReply = String(data.reply || chunks.join(''))
          streamError = messageText
          openClawChatTestResult.value = partialReply
            ? `Error: ${messageText}\nReceived: ${partialReply}`
            : `Error: ${messageText}`
          return
        }
        if (event === 'done') {
          if (!finalData) {
            finalData = normalizePayload(envelope.data)
          }
          if (envelope.ok === false && !streamError) {
            streamError = 'OpenClaw conversation test failed'
          }
        }
      }
    })

    if (response.mode === 'json') {
      const data = response.payload?.data || {}
      const reply = String(data.reply || '')
      const latency = Number(data.latency_ms)
      openClawChatTestResult.value = formatOpenClawChatResult(reply, latency)
      ElMessage.success('OpenClaw conversation test successful')
      return
    }

    if (streamError) {
      throw new Error(streamError)
    }

    if (finalData && typeof finalData === 'object') {
      const reply = String(finalData.reply || chunks.join(''))
      const latency = Number(finalData.latency_ms)
      openClawChatTestResult.value = formatOpenClawChatResult(reply, latency)
    } else if (chunks.length > 0) {
      openClawChatTestResult.value = formatOpenClawChatResult(chunks.join(''), Number.NaN)
    } else {
      throw new Error('No content received from OpenClaw')
    }

    ElMessage.success('OpenClaw conversation test successful')
  } catch (error) {
    const msg = error.response?.data?.error || error.message || 'OpenClaw conversation test failed'
    openClawChatTestResult.value = `Error: ${msg}`
    ElMessage.error(msg)
  } finally {
    openClawChatTesting.value = false
    await fetchOpenClawEndpoint()
  }
}

// Show MCP Endpoint
const showMCPEndpoint = async (agent) => {
  showMCPDialog.value = true
  mcpLoading.value = true
  currentAgentId.value = agent.id
  mcpCallResult.value = ""
  mcpCallForm.value = { tool_name: "", argumentsText: "{}" }
  
  try {
    const response = await api.get(`/admin/agents/${agent.id}/mcp-endpoint`)
    mcpEndpointData.value = response.data.data
    
    // Auto-refresh tools list
    await refreshMcpTools()
  } catch (error) {
    ElMessage.error('Failed to get MCP endpoint')
    console.error('Error getting MCP endpoint:', error)
    showMCPDialog.value = false
  } finally {
    mcpLoading.value = false
  }
}

// Refresh MCP Tools List
const refreshMcpTools = async () => {
  if (!currentAgentId.value) {
    ElMessage.warning('No agent selected')
    return
  }
  
  toolsLoading.value = true
  try {
    const response = await api.get(`/admin/agents/${currentAgentId.value}/mcp-tools`)
    if (response.data.data && response.data.data.tools) {
      mcpTools.value = response.data.data.tools
      if (mcpTools.value.length > 0) {
        if (!mcpCallForm.value.tool_name) {
          mcpCallForm.value.tool_name = mcpTools.value[0].name
        }
        updateMcpExampleByTool(mcpCallForm.value.tool_name)
      }
      ElMessage.success(`Successfully retrieved ${mcpTools.value.length} tools`)
    } else {
      mcpTools.value = []
      ElMessage.info('No tools data found')
    }
  } catch (error) {
    ElMessage.error('Failed to get tools list: ' + (error.response?.data?.error || error.message))
    console.error('Error refreshing MCP tools:', error)
    mcpTools.value = []
  } finally {
    toolsLoading.value = false
  }
}







const buildExampleFromSchema = (schema = {}) => {
  if (!schema || typeof schema !== 'object') return {}
  if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0]

  const type = schema.type || 'object'
  if (type === 'object') {
    const props = schema.properties || {}
    const result = {}
    Object.keys(props).sort().forEach((key) => {
      result[key] = buildExampleFromSchema(props[key])
    })
    return result
  }
  if (type === 'array') {
    return [buildExampleFromSchema(schema.items || {})]
  }
  if (type === 'number') return 0.1
  if (type === 'integer') return 0
  if (type === 'boolean') return false
  return ''
}

const updateMcpExampleByTool = (toolName) => {
  const selectedTool = mcpTools.value.find(item => item.name === toolName)
  if (!selectedTool) return

  const example = buildExampleFromSchema(selectedTool.input_schema || {})
  mcpCallForm.value.argumentsText = JSON.stringify(example ?? {}, null, 2)
}

const handleMcpToolChange = (toolName) => {
  updateMcpExampleByTool(toolName)
}

const formatMcpCallResult = (payload) => {
  const MAX_PARSE_DEPTH = 8

  const tryParseJSONString = (value) => {
    if (typeof value !== 'string') return { parsed: false, value }
    let text = value.trim()
    if (!text) return { parsed: false, value }

    const fenced = text.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i)
    if (fenced) {
      text = fenced[1].trim()
    }

    const looksLikeJSON =
      (text.startsWith('{') && text.endsWith('}')) ||
      (text.startsWith('[') && text.endsWith(']'))
    if (!looksLikeJSON) return { parsed: false, value }

    try {
      return { parsed: true, value: JSON.parse(text) }
    } catch (_) {
      return { parsed: false, value }
    }
  }

  const deepParseJSONStrings = (value, depth = 0) => {
    if (depth >= MAX_PARSE_DEPTH || value == null) return value

    if (typeof value === 'string') {
      const parsed = tryParseJSONString(value)
      if (!parsed.parsed) return value
      return deepParseJSONStrings(parsed.value, depth + 1)
    }

    if (Array.isArray(value)) {
      return value.map((item) => deepParseJSONStrings(item, depth + 1))
    }

    if (typeof value === 'object') {
      const out = {}
      Object.keys(value).forEach((key) => {
        out[key] = deepParseJSONStrings(value[key], depth + 1)
      })

      if (Array.isArray(out.content) && out.content.length === 1) {
        const first = out.content[0]
        if (first && typeof first === 'object' && !Array.isArray(first) && first.type === 'text' && Object.prototype.hasOwnProperty.call(first, 'text')) {
          const textValue = first.text
          if (textValue && typeof textValue === 'object') {
            return textValue
          }
        }
      }

      return out
    }

    return value
  }

  const data = payload ?? {}
  const raw = (data && typeof data === 'object' && !Array.isArray(data) && Object.prototype.hasOwnProperty.call(data, 'result'))
    ? data.result
    : data

  return JSON.stringify(deepParseJSONStrings(raw), null, 2)
}

const callAgentMcpTool = async () => {
  if (!currentAgentId.value || !mcpCallForm.value.tool_name) {
    ElMessage.warning('Please select a tool')
    return
  }

  let argumentsObj = {}
  try {
    argumentsObj = mcpCallForm.value.argumentsText ? JSON.parse(mcpCallForm.value.argumentsText) : {}
  } catch (e) {
    ElMessage.error('Invalid JSON format for parameters')
    return
  }

  callingTool.value = true
  try {
    const response = await api.post(`/admin/agents/${currentAgentId.value}/mcp-call`, {
      tool_name: mcpCallForm.value.tool_name,
      arguments: argumentsObj
    })
    mcpCallResult.value = formatMcpCallResult(response.data.data || {})
    ElMessage.success('MCP tool called successfully')
  } catch (error) {
    mcpCallResult.value = JSON.stringify(error.response?.data || { error: error.message }, null, 2)
    ElMessage.error('Failed to call MCP tool')
  } finally {
    callingTool.value = false
  }
}

// Copy MCP Endpoint URL
const copyMCPEndpoint = async () => {
  try {
    await navigator.clipboard.writeText(mcpEndpointData.value.endpoint)
    ElMessage.success('MCP endpoint URL copied to clipboard')
  } catch (error) {
    ElMessage.error('Copy failed')
    console.error('Error copying to clipboard:', error)
  }
}

onMounted(() => {
  loadAgents()
  loadConfigs()
})
</script>

<style scoped>
.admin-agents {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  color: #303133;
  font-size: 24px;
  font-weight: 600;
}

.page-subtitle {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.toolbar {
  margin-bottom: 20px;
  display: flex;
  gap: 12px;
}

.mcp-endpoint-display {
  margin: 20px 0;
}

.openclaw-status-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.openclaw-status-message {
  margin-bottom: 12px;
  color: #6b7280;
  font-size: 12px;
  line-height: 1.4;
}

.openclaw-tip-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.openclaw-tip-title {
  font-size: 13px;
  color: #6b7280;
}

.openclaw-tip-icon {
  font-size: 16px;
  color: #1d4ed8;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  padding: 2px;
  cursor: help;
  transition: all 0.2s ease;
}

.openclaw-tip-icon:hover {
  color: #1e40af;
  background: #dbeafe;
  border-color: #93c5fd;
}

.openclaw-tip-content {
  max-width: 420px;
  color: #111827;
  font-size: 12px;
  line-height: 1.7;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.openclaw-tip-content code {
  background: #f3f4f6;
  border-radius: 4px;
  padding: 0 4px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

:deep(.openclaw-tip-popper) {
  max-width: 460px;
  background: #ffffff !important;
  border: 1px solid #dbeafe !important;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12) !important;
}

:deep(.openclaw-tip-popper .el-popper__arrow::before) {
  background: #ffffff !important;
  border: 1px solid #dbeafe !important;
}

:deep(.openclaw-tip-popper .el-link) {
  color: #2563eb !important;
}

.endpoint-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.endpoint-header .endpoint-label {
  margin-bottom: 0;
}

.endpoint-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.endpoint-label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 8px;
}

.openclaw-command-hint {
  margin-bottom: 8px;
  color: #6b7280;
  font-size: 12px;
}

.openclaw-command-steps {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.openclaw-command-step-title {
  margin-bottom: 6px;
  color: #374151;
  font-size: 13px;
  font-weight: 500;
}

.openclaw-command-content {
  margin: 0;
  padding: 12px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  color: #1e293b;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}

.endpoint-content {
  padding: 12px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  color: #1e293b;
  word-break: break-all;
  line-height: 1.5;
  min-height: 60px;
  display: flex;
  align-items: center;
}

.mcp-tools-section {
  margin-top: 24px;
  border-top: 1px solid #e2e8f0;
  padding-top: 20px;
}

.tools-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.tools-title {
  font-size: 16px;
  font-weight: 600;
  color: #374151;
}



.tools-empty {
  margin: 20px 0;
  text-align: center;
}

.tools-list {
  margin-top: 16px;
}

.tools-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}

.tool-tag {
  position: relative;
  padding: 8px 16px;
  font-size: 14px;
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.3s ease;
}

.tool-tag:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.tool-info-icon {
  margin-left: 6px;
  font-size: 12px;
  opacity: 0.7;
}

.mcp-result-box {
  margin-top: 12px;
  white-space: pre-wrap;
  font-family: monospace;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px;
  min-height: 80px;
}

.tool-tag:hover .tool-info-icon {
  opacity: 1;
}
</style>
