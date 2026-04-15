<template>
  <div class="agent-config">
    <div class="config-header">
      <div class="header-left">
        <el-button 
          @click="$router.back()" 
          :icon="ArrowLeft" 
          circle 
          size="large"
        />
        <h1>Agent Configuration</h1>
      </div>
      <el-button type="primary" @click="handleSave" :loading="saving" size="large">
        Save Configuration
      </el-button>
    </div>

    <div class="config-content">
      <div class="config-form">
        <!-- Quick Role Selection -->
        <div class="form-section quick-config-section" v-if="hasAvailableRoles">
          <h3 class="section-title">
            Quick Configuration
            <el-tooltip content="Click a role to quickly apply its configuration to the agent" placement="top">
              <el-icon class="help-icon"><QuestionFilled /></el-icon>
            </el-tooltip>
          </h3>

          <div class="role-selector role-selector-compact" v-loading="rolesLoading">
            <div class="role-inline-line role-inline-line-compact">
              <button
                v-for="role in allRoles"
                :key="role.id"
                type="button"
                class="role-inline-item"
                :class="{ active: selectedRoleId === role.id }"
                @click="applyRoleConfig(role)"
              >
                <span class="role-inline-name">{{ role.name }}</span>
                <span class="role-inline-type" :class="role.role_type === 'global' ? 'global' : 'user'">
                  {{ role.role_type === 'global' ? 'Global' : 'My' }}
                </span>
              </button>
            </div>
            <div class="form-help quick-config-help">
              Role names are displayed in a flat layout. Clicking any role will immediately fill in the Prompt, LLM, TTS, and voice configuration (will not auto-save)
            </div>
          </div>
        </div>

        <!-- Basic Information -->
        <div class="form-section">
          <h3 class="section-title">Basic Information</h3>
          
          <div class="form-group">
            <label class="form-label">Nickname</label>
            <el-input 
              v-model="form.name" 
              placeholder="Please enter agent nickname" 
              size="large"
              :maxlength="50"
              show-word-limit
            />
          </div>

          <div class="form-group">
            <label class="form-label">Role Introduction (prompt)</label>
            <el-input
              v-model="form.custom_prompt"
              type="textarea"
              :rows="4"
              placeholder="Please enter role introduction/system prompt, this will affect the AI's response style and personality"
              :maxlength="10000"
              show-word-limit
            />
          </div>
        </div>

        <!-- Configuration Settings -->
        <div class="form-section">
          <h3 class="section-title">Configuration Settings</h3>
          
          <div class="form-group">
            <label class="form-label">Language Model</label>
            <el-select 
              v-model="form.llm_config_id" 
              placeholder="Please select language model" 
              size="large" 
              style="width: 100%"
              clearable
            >
              <el-option
                v-for="llmConfig in llmConfigs"
                :key="llmConfig.config_id"
                :label="llmConfig.is_default ? `${llmConfig.name} (Default)` : llmConfig.name"
                :value="llmConfig.config_id"
              >
                <div class="config-option">
                  <span class="config-name">
                    {{ llmConfig.name }}
                    <el-tag v-if="llmConfig.is_default" type="success" size="small" style="margin-left: 8px;">Default</el-tag>
                  </span>
                  <span class="config-desc">{{ llmConfig.provider || 'No description' }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="form-help" v-if="getCurrentLlmConfigName()">
              {{ getCurrentLlmConfigInfo() }}
            </div>
          </div>

          <div class="form-group" v-if="myCloneVoices.length > 0">
            <label class="form-label">My Cloned Voices</label>
            <div class="clone-voice-line" v-loading="cloneVoicesLoading">
              <button
                v-for="clone in myCloneVoices"
                :key="clone.id"
                type="button"
                class="clone-voice-item"
                :class="{ active: isCloneVoiceSelected(clone) }"
                :title="`${clone.tts_config_name || clone.tts_config_id} · ${clone.provider_voice_id}`"
                @click="applyCloneVoice(clone)"
              >
                <span class="clone-voice-name">{{ clone.name || clone.provider_voice_id }}</span>
              </button>
            </div>
            <div class="form-help">Clicking will automatically fill in TTS configuration and voice</div>
          </div>

          <div class="form-group">
            <label class="form-label">TTS Configuration</label>
            <el-select 
              v-model="form.tts_config_id" 
              placeholder="Please select TTS configuration" 
              size="large" 
              style="width: 100%"
              clearable
              @change="handleTtsConfigChange"
            >
              <el-option
                v-for="ttsConfig in ttsConfigs"
                :key="ttsConfig.config_id"
                :label="ttsConfig.is_default ? `${ttsConfig.name} (Default)` : ttsConfig.name"
                :value="ttsConfig.config_id"
              >
                <div class="config-option">
                  <span class="config-name">
                    {{ ttsConfig.name }}
                    <el-tag v-if="ttsConfig.is_default" type="success" size="small" style="margin-left: 8px;">Default</el-tag>
                  </span>
                  <span class="config-desc">{{ ttsConfig.provider || 'No description' }}</span>
                </div>
              </el-option>
            </el-select>
            <div class="form-help" v-if="getCurrentTtsConfigName()">
              {{ getCurrentTtsConfigInfo() }}
            </div>
          </div>

          <div class="form-group" v-if="form.tts_config_id">
            <label class="form-label">Voice</label>
            <el-select 
              v-model="form.voice" 
              placeholder="Please select or enter voice (supports search and custom input)" 
              size="large" 
              style="width: 100%"
              filterable
              allow-create
              default-first-option
              reserve-keyword
              clearable
              :loading="voiceLoading"
              :filter-method="filterVoice"
            >
              <el-option
                v-for="voice in filteredVoices"
                :key="voice.value"
                :label="voice.label"
                :value="voice.value"
              >
                <span>{{ voice.label }}</span>
                <span style="color: #8492a6; font-size: 13px; margin-left: 8px;">{{ voice.value }}</span>
              </el-option>
            </el-select>
            <div class="form-help">
              Current TTS config: {{ getCurrentTtsConfigName() }}, you can search for voice name or value, or manually enter a custom voice value.
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Linked Knowledge Bases</label>
            <el-select
              v-model="form.knowledge_base_ids"
              multiple
              collapse-tags
              collapse-tags-tooltip
              placeholder="Please select knowledge bases to link (multiple selection supported)"
              size="large"
              style="width: 100%"
            >
              <el-option
                v-for="kb in knowledgeBases"
                :key="kb.id"
                :label="kb.name"
                :value="kb.id"
              />
            </el-select>
            <div class="form-help">Supports multi-library linking. Will automatically fall back to regular LLM conversation if knowledge base retrieval fails.</div>
          </div>

          <div class="form-group">
            <label class="form-label">Speech Recognition Speed</label>
            <el-select v-model="form.asr_speed" placeholder="Please select speech recognition speed" size="large" style="width: 100%">
              <el-option label="Normal" value="normal" />
              <el-option label="Patient" value="patient" />
              <el-option label="Fast" value="fast" />
            </el-select>
            <div class="form-help">Set the response speed of speech recognition</div>
          </div>

          <div class="form-group">
            <label class="form-label">Memory</label>
            <el-select v-model="form.memory_mode" placeholder="Please select memory mode" size="large" style="width: 100%">
              <el-option label="No Memory" value="none" />
              <el-option label="Short Memory" value="short" />
              <el-option label="Long Memory" value="long" />
            </el-select>
            <div class="form-help">
              No Memory: LLM doesn't load history; Short Memory: Load history without loading long memory; Long Memory: Load history and long memory.
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Voiceprint Chat Only</label>
            <el-select v-model="form.speaker_chat_mode" placeholder="Please select voiceprint chat restriction" size="large" style="width: 100%">
              <el-option label="Off" value="off" />
              <el-option label="Only when voiceprint matched" value="identified_only" />
            </el-select>
            <div class="form-help">
              When the agent has a voiceprint group configured, you can restrict it so that only speakers who match the configured voiceprints are allowed to continue chatting.
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">OpenClaw</label>
            <el-button type="primary" size="large" style="width: 100%" @click="showOpenClawSettings">
              View OpenClaw
            </el-button>
            <div class="form-help">
              Configured: {{ form.openclaw_allowed ? 'On' : 'Off' }}, Enter keywords: {{ form.openclaw_enter_keywords.length }}, Exit keywords: {{ form.openclaw_exit_keywords.length }}.
            </div>
          </div>

          <div class="form-group" v-loading="mcpServiceOptionsLoading">
            <label class="form-label">MCP Services</label>
            <el-select
              v-model="selectedMcpServices"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              clearable
              size="large"
              style="width: 100%"
              placeholder="Leave empty to use all enabled services"
              @change="handleMcpServiceSelectionChange"
            >
              <el-option
                v-for="serviceName in mcpServiceOptions"
                :key="serviceName"
                :label="serviceName"
                :value="serviceName"
              />
            </el-select>
            <div class="form-help">
              Leave empty to use all enabled global MCP services, currently {{ mcpServiceOptions.length }} services available.
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">MCP Endpoint</label>
            <el-button 
              type="primary" 
              @click="showMCPEndpoint" 
              size="large"
              style="width: 100%"
            >
              View MCP Endpoint
            </el-button>
            <div class="form-help">Get the agent's MCP WebSocket endpoint URL, which can be used for device connections</div>
          </div>
        </div>
      </div>
    </div>

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
          description="This is the agent's MCP WebSocket endpoint URL, which can be used for device connections"
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
            <el-select v-model="mcpCallForm.tool_name" placeholder="Please select tool" style="width: 100%" @change="handleMcpToolChange">
              <el-option v-for="tool in mcpTools" :key="tool.name" :label="tool.name" :value="tool.name" />
            </el-select>
          </el-form-item>
          <el-form-item label="Parameters JSON">
            <el-input v-model="mcpCallForm.argumentsText" type="textarea" :rows="6" placeholder='e.g.: {"query":"hello"}' />
          </el-form-item>
        </el-form>
        <el-button type="primary" @click="callAgentMcpTool" :loading="callingTool">Call Tool</el-button>
        <div class="mcp-result-box">{{ mcpCallResult || 'No call results yet' }}</div>
      </div>

      <template #footer>
        <el-button @click="showMCPDialog = false">Close</el-button>
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
                <div>Architecture: Device Voice -> Server Routing -> OpenClaw Session -> xiaozhi Plugin.</div>
                <div>Role Configuration: Use the four commands below in the OpenClaw console role configuration, then execute `openclaw gateway restart` to make the configuration take effect.</div>
                <div>Entry Logic: After hitting the entry keywords (default "Open Lobster/Enter Lobster"), enter OpenClaw mode, subsequent text will prioritize OpenClaw.</div>
                <div>Exit Logic: In OpenClaw mode, hitting the exit keywords (default "Close Lobster/Exit Lobster") will exit and return to regular LLM conversation.</div>
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
            <el-switch v-model="form.openclaw_allowed" />
          </el-form-item>
          <el-form-item label="Entry Keywords">
            <el-select
              v-model="form.openclaw_enter_keywords"
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
              v-model="form.openclaw_exit_keywords"
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
                <el-button size="small" @click="fetchOpenClawEndpoint" :loading="openClawEndpointLoading">Refresh</el-button>
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
          description="Send a text message to openclaw to verify connectivity and response."
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
              placeholder="Please enter test message"
            />
          </el-form-item>
        </el-form>
        <el-button
          type="primary"
          @click="testOpenClawChat"
          :loading="openClawChatTesting"
        >
          Send Test
        </el-button>
        <div class="mcp-result-box">{{ openClawChatTestResult || 'No test results yet' }}</div>
      </div>
      <template #footer>
        <el-button @click="showOpenClawDialog = false">Close</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, VideoPlay, Refresh, InfoFilled, QuestionFilled } from '@element-plus/icons-vue'
import api from '@/utils/api'
import { postJSONWithSSE } from '@/utils/sse'
import { buildOpenClawCommands } from '@/utils/openclaw'

const route = useRoute()
const router = useRouter()
const saving = ref(false)
const applyingRoleConfig = ref(false)

// Role related data
const globalRoles = ref([])
const userRoles = ref([])
const selectedRoleId = ref(null)
const rolesLoading = ref(false)

const isRoleEnabled = (role) => role?.status === "active" || !role?.status

// Calculate all roles list (for selector)
const allRoles = computed(() => {
  return [...globalRoles.value, ...userRoles.value].filter(isRoleEnabled)
})
const hasAvailableRoles = computed(() => allRoles.value.length > 0)
const OPENCLAW_DEFAULT_ENTER_KEYWORDS = ['Open Lobster', 'Enter Lobster']
const OPENCLAW_DEFAULT_EXIT_KEYWORDS = ['Close Lobster', 'Exit Lobster']
const openClawDocURL = 'https://github.com/hackers365/xiaozhi-esp32-server-golang/blob/main/doc/openclaw_integration.md'

// Form data
const form = reactive({
  name: '',
  custom_prompt: '',
  llm_config_id: null,
  tts_config_id: null,
  voice: null,
  asr_speed: 'normal',
  knowledge_base_ids: [],
  memory_mode: 'short',
  speaker_chat_mode: 'off',
  mcp_service_names: '',
  openclaw_allowed: false,
  openclaw_enter_keywords: [...OPENCLAW_DEFAULT_ENTER_KEYWORDS],
  openclaw_exit_keywords: [...OPENCLAW_DEFAULT_EXIT_KEYWORDS]
})

// LLM config data
const llmConfigs = ref([])

// TTS config data
const ttsConfigs = ref([])

// Knowledge base data
const knowledgeBases = ref([])

const loadKnowledgeBases = async () => {
  try {
    const response = await api.get('/user/knowledge-bases')
    knowledgeBases.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load knowledge bases:', error)
  }
}

// Voice related data
const availableVoices = ref([])
const filteredVoices = ref([])
const voiceSearchKeyword = ref('')
const voiceLoading = ref(false)
const previousTtsConfigId = ref(null) // For tracking TTS config changes
const myCloneVoices = ref([])
const cloneVoicesLoading = ref(false)

// MCP service selection
const mcpServiceOptions = ref([])
const selectedMcpServices = ref([])
const mcpServiceOptionsLoading = ref(false)

// MCP endpoint related
const showMCPDialog = ref(false)
const mcpLoading = ref(false)
const mcpEndpointData = ref({
  endpoint: ''
})
const toolsLoading = ref(false)
const mcpTools = ref([])
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
  if (status === 'offline') return 'Not Connected'
  return 'Status Unknown'
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
  return 'No installation commands available, please refresh and try again.'
})

// Load LLM configs
const loadLlmConfigs = async () => {
  try {
    const response = await api.get('/user/llm-configs')
    llmConfigs.value = response.data.data || []
    // Don't auto-select default config here, let specific usage scenarios handle it
  } catch (error) {
    console.error('Failed to load LLM configs:', error)
  }
}

// Load TTS configs
const loadTtsConfigs = async () => {
  try {
    const response = await api.get('/user/tts-configs')
    ttsConfigs.value = response.data.data || []
    // Don't auto-select default config here, let specific usage scenarios handle it
  } catch (error) {
    console.error('Failed to load TTS configs:', error)
  }
}



// Load agent data
const loadAgent = async () => {
  try {
    const response = await api.get(`/user/agents/${route.params.id}`)
    const agent = response.data.data
    const openclawConfig = parseOpenClawConfigFromAgent(agent)
    
    // Map basic fields
    Object.assign(form, {
      name: agent.name || '',
      custom_prompt: agent.custom_prompt || '',
      asr_speed: agent.asr_speed || 'normal',
      voice: agent.voice || null,
      knowledge_base_ids: agent.knowledge_base_ids || [],
      memory_mode: agent.memory_mode || 'short',
      speaker_chat_mode: agent.speaker_chat_mode || 'off',
      mcp_service_names: agent.mcp_service_names || '',
      openclaw_allowed: !!openclawConfig.allowed,
      openclaw_enter_keywords: normalizeKeywordList(openclawConfig.enter_keywords),
      openclaw_exit_keywords: normalizeKeywordList(openclawConfig.exit_keywords)
    })
    selectedMcpServices.value = normalizeMcpServiceNames((form.mcp_service_names || '').split(','))
    syncMcpServiceNamesToForm()
    
    // Handle LLM config association
    const hasValidLlmConfigId = agent.llm_config_id && 
                               agent.llm_config_id !== '' && 
                               agent.llm_config_id !== 'null' && 
                               agent.llm_config_id !== 'undefined'
    
    if (hasValidLlmConfigId) {
      // Verify if config_id exists in available configs
      const llmConfig = llmConfigs.value.find(config => config.config_id === agent.llm_config_id)
      if (llmConfig) {
        form.llm_config_id = agent.llm_config_id
        console.log(`✅ Agent uses LLM config: ${llmConfig.name}`)
      } else {
        console.warn(`⚠️ Agent's LLM config ID ${agent.llm_config_id} does not exist, will use default config`)
        // If config_id is invalid, use default config
        const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
        form.llm_config_id = defaultLlmConfig ? defaultLlmConfig.config_id : null
        if (defaultLlmConfig) {
          console.log(`🔄 Switched to default LLM config: ${defaultLlmConfig.name}`)
        }
      }
    } else {
      // If no config, use default config
      const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
      form.llm_config_id = defaultLlmConfig ? defaultLlmConfig.config_id : null
      if (defaultLlmConfig) {
        console.log(`🎯 Agent LLM config is empty, using default config: ${defaultLlmConfig.name}`)
      } else {
        console.warn(`❌ No default LLM config found`)
      }
    }
    
    // Handle TTS config association
    const hasValidTtsConfigId = agent.tts_config_id && 
                               agent.tts_config_id !== '' && 
                               agent.tts_config_id !== 'null' && 
                               agent.tts_config_id !== 'undefined'
    
    if (hasValidTtsConfigId) {
      // Verify if config_id exists in available configs
      const ttsConfig = ttsConfigs.value.find(config => config.config_id === agent.tts_config_id)
      if (ttsConfig) {
        form.tts_config_id = agent.tts_config_id
        console.log(`✅ Agent uses TTS config: ${ttsConfig.name}`)
      } else {
        console.warn(`⚠️ Agent's TTS config ID ${agent.tts_config_id} does not exist, will use default config`)
        // If config_id is invalid, use default config
        const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
        form.tts_config_id = defaultTtsConfig ? defaultTtsConfig.config_id : null
        if (defaultTtsConfig) {
          console.log(`🔄 Switched to default TTS config: ${defaultTtsConfig.name}`)
        }
      }
    } else {
      // If no config, use default config
      const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
      form.tts_config_id = defaultTtsConfig ? defaultTtsConfig.config_id : null
      if (defaultTtsConfig) {
        console.log(`🎯 Agent TTS config is empty, using default config: ${defaultTtsConfig.name}`)
      } else {
        console.warn(`❌ No default TTS config found`)
      }
    }
  } catch (error) {
    console.error('Failed to load agent:', error)
    ElMessage.error('Failed to load agent')
  }
}

const normalizeMcpServiceNames = (names) => {
  if (!Array.isArray(names)) return []
  const unique = []
  const seen = new Set()
  for (const item of names) {
    const name = String(item || '').trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    unique.push(name)
  }
  return unique
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

const syncMcpServiceNamesToForm = () => {
  selectedMcpServices.value = normalizeMcpServiceNames(selectedMcpServices.value)
  form.mcp_service_names = selectedMcpServices.value.join(',')
}

const handleMcpServiceSelectionChange = (values) => {
  selectedMcpServices.value = normalizeMcpServiceNames(values || [])
  syncMcpServiceNamesToForm()
}

const loadMcpServiceOptions = async () => {
  if (!route.params.id) return

  mcpServiceOptionsLoading.value = true
  try {
    const response = await api.get(`/user/agents/${route.params.id}/mcp-services/options`)
    const data = response.data.data || {}

    mcpServiceOptions.value = Array.isArray(data.options)
      ? normalizeMcpServiceNames(data.options)
      : []

    if (Array.isArray(data.selected)) {
      selectedMcpServices.value = normalizeMcpServiceNames(data.selected)
    } else if (typeof data.mcp_service_names === 'string') {
      selectedMcpServices.value = normalizeMcpServiceNames(data.mcp_service_names.split(','))
    } else {
      selectedMcpServices.value = normalizeMcpServiceNames((form.mcp_service_names || '').split(','))
    }
    syncMcpServiceNamesToForm()
  } catch (error) {
    console.error('Failed to load MCP service options:', error)
    ElMessage.warning('Failed to load MCP service options')
  } finally {
    mcpServiceOptionsLoading.value = false
  }
}

const fetchOpenClawEndpoint = async () => {
  openClawEndpointLoading.value = true
  try {
    const response = await api.get(`/user/agents/${route.params.id}/openclaw-endpoint`)
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
  await fetchOpenClawEndpoint()
}

const formatOpenClawChatResult = (reply, latency) => {
  const lines = [`Reply: ${String(reply || '') || '(empty)'}`]
  if (Number.isFinite(latency)) {
    lines.push(`Time: ${latency}ms`)
  }
  return lines.join('\n')
}

const testOpenClawChat = async () => {
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
      url: `/api/user/agents/${route.params.id}/openclaw-chat-test?stream=1`,
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

// Load role list (global + user roles)
const loadRoles = async () => {
  rolesLoading.value = true
  try {
    const response = await api.get('/user/roles')
    globalRoles.value = response.data.data?.global_roles || []
    userRoles.value = response.data.data?.user_roles || []
  } catch (error) {
    console.error('Failed to load role list:', error)
  } finally {
    rolesLoading.value = false
  }
}

const normalizeCloneStatus = (clone) => {
  const status = String(clone?.status || '').trim().toLowerCase()
  const taskStatus = String(clone?.task_status || '').trim().toLowerCase()
  if (status === 'failed' || taskStatus === 'failed') return 'failed'
  if (status === 'active' || taskStatus === 'succeeded') return 'active'
  if (taskStatus === 'queued' || taskStatus === 'processing') return taskStatus
  if (status === 'queued' || status === 'processing') return status
  return status || taskStatus || 'unknown'
}

const loadMyCloneVoices = async () => {
  cloneVoicesLoading.value = true
  try {
    const response = await api.get('/user/voice-clones')
    const cloneList = response.data.data || []
    myCloneVoices.value = cloneList
      .filter((clone) => normalizeCloneStatus(clone) === 'active')
      .filter((clone) => clone?.provider_voice_id && clone?.tts_config_id)
      .map((clone) => ({
        id: clone.id,
        name: clone.name || clone.provider_voice_id,
        provider_voice_id: clone.provider_voice_id,
        tts_config_id: clone.tts_config_id,
        tts_config_name: clone.tts_config_name || ''
      }))
  } catch (error) {
    console.error('Failed to load cloned voices:', error)
    myCloneVoices.value = []
  } finally {
    cloneVoicesLoading.value = false
  }
}

const isCloneVoiceSelected = (clone) => {
  return form.tts_config_id === clone?.tts_config_id && form.voice === clone?.provider_voice_id
}

const applyCloneVoice = async (clone) => {
  if (!clone) return
  const ttsConfig = ttsConfigs.value.find(config => config.config_id === clone.tts_config_id)
  if (!ttsConfig) {
    return
  }

  form.tts_config_id = clone.tts_config_id
  await handleTtsConfigChange()
  form.voice = clone.provider_voice_id
}

// Apply role config to agent form
const applyRoleConfig = async (role) => {
  if (!role) return
  applyingRoleConfig.value = true
  try {
    selectedRoleId.value = role.id

    // Fill config into form
    form.custom_prompt = role.prompt || ''

    // LLM config
    if (role.llm_config_id) {
      const llmConfig = llmConfigs.value.find(c => c.config_id === role.llm_config_id)
      if (llmConfig) {
        form.llm_config_id = role.llm_config_id
      }
    }

    // TTS config
    if (role.tts_config_id) {
      const ttsConfig = ttsConfigs.value.find(c => c.config_id === role.tts_config_id)
      if (ttsConfig) {
        form.tts_config_id = role.tts_config_id
      } else {
        form.tts_config_id = null
      }
    } else {
      form.tts_config_id = null
    }

    // Refresh voice list according to TTS config, then fill in role voice
    await handleTtsConfigChange()
    form.voice = role.voice || null
  } finally {
    applyingRoleConfig.value = false
  }
}

// Save agent
const handleSave = async () => {
  if (applyingRoleConfig.value) {
    ElMessage.info('Currently only filling role config, will not auto-save, please click "Save Configuration" to submit')
    return
  }

  if (!form.name.trim()) {
    ElMessage.error('Please enter agent nickname')
    return
  }
  
  try {
    saving.value = true
    syncMcpServiceNamesToForm()

    const payload = {
      ...form,
      openclaw: {
        allowed: !!form.openclaw_allowed,
        enter_keywords: normalizeKeywordList(form.openclaw_enter_keywords),
        exit_keywords: normalizeKeywordList(form.openclaw_exit_keywords)
      }
    }
    delete payload.openclaw_allowed
    delete payload.openclaw_enter_keywords
    delete payload.openclaw_exit_keywords

    await api.put(`/user/agents/${route.params.id}`, payload)
    
    ElMessage.success('Save successful')
    router.push('/user/agents')
  } catch (error) {
    console.error('Save failed:', error)
    ElMessage.error('Save failed')
  } finally {
    saving.value = false
  }
}



// Get current LLM config name
const getCurrentLlmConfigName = () => {
  if (!form.llm_config_id) return null
  const config = llmConfigs.value.find(c => c.config_id === form.llm_config_id)
  return config ? config.name : null
}

// Get current LLM config info
const getCurrentLlmConfigInfo = () => {
  if (!form.llm_config_id) return ''
  const config = llmConfigs.value.find(c => c.config_id === form.llm_config_id)
  if (!config) return ''
  
  if (config.is_default) {
    return `Currently using default LLM config: ${config.name}`
  } else {
    return `Currently using LLM config: ${config.name}`
  }
}

// Get current TTS config name
const getCurrentTtsConfigName = () => {
  if (!form.tts_config_id) return null
  const config = ttsConfigs.value.find(c => c.config_id === form.tts_config_id)
  return config ? config.name : null
}

// Get current TTS config info
const getCurrentTtsConfigInfo = () => {
  if (!form.tts_config_id) return ''
  const config = ttsConfigs.value.find(c => c.config_id === form.tts_config_id)
  if (!config) return ''
  
  if (config.is_default) {
    return `Currently using default TTS config: ${config.name}`
  } else {
    return `Currently using TTS config: ${config.name}`
  }
}

// Auto-select default configs
const autoSelectDefaultConfigs = () => {
  // Select default LLM config
  if (!form.llm_config_id && llmConfigs.value.length > 0) {
    const defaultLlmConfig = llmConfigs.value.find(config => config.is_default)
    if (defaultLlmConfig) {
      form.llm_config_id = defaultLlmConfig.config_id
    }
  }
  
  // Select default TTS config
  if (!form.tts_config_id && ttsConfigs.value.length > 0) {
    const defaultTtsConfig = ttsConfigs.value.find(config => config.is_default)
    if (defaultTtsConfig) {
      form.tts_config_id = defaultTtsConfig.config_id
    }
  }
}

// Show MCP endpoint
const showMCPEndpoint = async () => {
  showMCPDialog.value = true
  mcpLoading.value = true
  mcpCallResult.value = ""
  mcpCallForm.value = { tool_name: "", argumentsText: "{}" }
  
  try {
    const response = await api.get(`/user/agents/${route.params.id}/mcp-endpoint`)
    mcpEndpointData.value = response.data.data
    
    // Get tools list
    await refreshMcpTools()
  } catch (error) {
    ElMessage.error('Failed to get MCP endpoint')
    console.error('Error getting MCP endpoint:', error)
    showMCPDialog.value = false
  } finally {
    mcpLoading.value = false
  }
}

// Refresh MCP tools list
const refreshMcpTools = async () => {
  toolsLoading.value = true
  try {
    const response = await api.get(`/user/agents/${route.params.id}/mcp-tools`)
    mcpTools.value = response.data.data.tools || []
    if (!mcpCallForm.value.tool_name && mcpTools.value.length > 0) {
      mcpCallForm.value.tool_name = mcpTools.value[0].name
    }
  } catch (error) {
    console.error('Failed to get MCP tools list:', error)
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
  if (!mcpCallForm.value.tool_name) {
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
    const response = await api.post(`/user/agents/${route.params.id}/mcp-call`, {
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

// Copy MCP endpoint URL
const copyMCPEndpoint = async () => {
  try {
    await navigator.clipboard.writeText(mcpEndpointData.value.endpoint)
    ElMessage.success('MCP endpoint URL copied to clipboard')
  } catch (error) {
    ElMessage.error('Copy failed')
    console.error('Error copying to clipboard:', error)
  }
}

// Handle TTS config change, load corresponding voice list
const handleTtsConfigChange = async () => {
  // Get previous provider (if any)
  let previousProvider = null
  if (previousTtsConfigId.value) {
    const prevConfig = ttsConfigs.value.find(config => config.config_id === previousTtsConfigId.value)
    previousProvider = prevConfig?.provider
  }
  
  if (!form.tts_config_id) {
    availableVoices.value = []
    filteredVoices.value = []
    form.voice = null // Clear voice
    previousTtsConfigId.value = null
    return
  }
  
  // Get current TTS config's provider
  const ttsConfig = ttsConfigs.value.find(config => config.config_id === form.tts_config_id)
  if (!ttsConfig || !ttsConfig.provider) {
    availableVoices.value = []
    filteredVoices.value = []
    form.voice = null // Clear voice
    previousTtsConfigId.value = form.tts_config_id
    return
  }
  
  // If provider changed, clear current voice value
  if (previousProvider && previousProvider !== ttsConfig.provider) {
    form.voice = null
  }
  
  // Load voice list
  await loadVoices(ttsConfig.provider)
  
  // If current voice value doesn't exist in new list, clear it too
  if (form.voice && availableVoices.value.length > 0) {
    const voiceExists = availableVoices.value.some(v => v.value === form.voice)
    if (!voiceExists) {
      form.voice = null
    }
  }
  
  // Update previousTtsConfigId
  previousTtsConfigId.value = form.tts_config_id
}

// Voice search filter function
const filterVoice = (val) => {
  voiceSearchKeyword.value = val
  if (!val) {
    filteredVoices.value = availableVoices.value
    return
  }
  
  const keyword = val.toLowerCase()
  filteredVoices.value = availableVoices.value.filter(voice => {
    // Search both label and value
    return voice.label.toLowerCase().includes(keyword) || 
           voice.value.toLowerCase().includes(keyword)
  })
}

// Load voice list
const loadVoices = async (provider) => {
  if (!provider) {
    availableVoices.value = []
    filteredVoices.value = []
    return
  }
  
  voiceLoading.value = true
  try {
    const params = { provider }
    if (form.tts_config_id) {
      params.config_id = form.tts_config_id
    }
    const response = await api.get('/user/voice-options', { params })
    availableVoices.value = response.data.data || []
    filteredVoices.value = availableVoices.value
  } catch (error) {
    console.error('Failed to load voice list:', error)
    availableVoices.value = []
    filteredVoices.value = []
  } finally {
    voiceLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([
    loadLlmConfigs(),
    loadTtsConfigs(),
    loadKnowledgeBases(),
    loadRoles(),
    loadMyCloneVoices()
  ])
  await loadAgent()
  await loadMcpServiceOptions()
  autoSelectDefaultConfigs()
})
</script>

<style scoped>
.agent-config {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.config-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e4e7ed;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h1 {
  margin: 0;
  font-size: 24px;
  color: #303133;
}

.config-content {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.form-section {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 20px;
}

.section-title {
  margin: 0 0 16px 0;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
}

.help-icon {
  color: #909399;
  cursor: help;
}

.form-group {
  margin-bottom: 16px;
}

.form-group:last-child {
  margin-bottom: 0;
}

.form-label {
  display: block;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 500;
  color: #606266;
}

.form-help {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.config-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-name {
  font-weight: 500;
}

.config-desc {
  font-size: 12px;
  color: #909399;
}

/* Role selector styles */
.role-selector {
  margin-bottom: 16px;
}

.role-inline-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.role-inline-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 16px;
  background: #fff;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
}

.role-inline-item:hover {
  border-color: #409eff;
  color: #409eff;
}

.role-inline-item.active {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
}

.role-inline-type {
  font-size: 11px;
  padding: 1px 5px;
  border-radius: 10px;
  background: #f0f0f0;
  color: #666;
}

.role-inline-item.active .role-inline-type {
  background: rgba(255,255,255,0.25);
  color: #fff;
}

.quick-config-help {
  font-size: 12px;
  color: #909399;
  margin-top: 8px;
}

/* Clone voice styles */
.clone-voice-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.clone-voice-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 16px;
  background: #fff;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
}

.clone-voice-item:hover {
  border-color: #67c23a;
  color: #67c23a;
}

.clone-voice-item.active {
  background: #67c23a;
  border-color: #67c23a;
  color: #fff;
}

/* MCP endpoint styles */
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

.mcp-endpoint-display {
  margin: 20px 0;
}

.endpoint-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.endpoint-label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 8px;
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

.tool-tag:hover .tool-info-icon {
  opacity: 1;
}

/* OpenClaw styles */
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
</style>
