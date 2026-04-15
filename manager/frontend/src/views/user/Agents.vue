<template>
  <div class="agents-page">
    <div class="page-header">
      <div class="header-left">
        <h2>My Agents</h2>
        <p class="page-subtitle">Manage your agent configurations</p>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showAddAgentDialog = true">
              <el-icon><Plus /></el-icon>
              Add Agent
            </el-button>
      </div>
    </div>

    <div v-if="agents.length === 0" class="welcome-section">
      <el-card class="welcome-card">
        <div class="welcome-content">
          <el-icon size="64" color="#409EFF"><Monitor /></el-icon>
          <h3>Welcome to Agent Management</h3>
          <p>You haven't created any agents yet. Agents are your AI assistants that can help you with various tasks.</p>
          <div class="welcome-actions">
            <el-button type="primary" size="large" @click="showAddAgentDialog = true">
              <el-icon><Plus /></el-icon>
              Create First Agent
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="agents-grid">
      <div v-for="agent in agents" :key="agent.id" class="agent-item">
        <div class="agent-card">
          <div class="agent-header">
            <div class="agent-avatar">
              <el-icon size="28"><Monitor /></el-icon>
            </div>
            <div class="agent-info">
              <h3 class="agent-name">{{ agent.name }}</h3>
              <p class="agent-desc">AI Assistant</p>
            </div>
            <div class="agent-status">
              <span class="status-dot active"></span>
              <span class="status-text">Online</span>
            </div>
          </div>
          
          <div class="agent-meta">
            <div class="meta-row">
              <span class="meta-label">TTS Config</span>
              <span class="meta-value">{{ getVoiceType(agent) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Language Model</span>
              <span class="meta-value">{{ getLLMProvider(agent) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Last Conversation</span>
              <span class="meta-value">{{ formatDate(agent.updated_at) }}</span>
            </div>
          </div>
          
          <div class="agent-actions">
            <el-button type="primary" size="small" @click="editAgent(agent.id)">
              <el-icon><Setting /></el-icon>
              Config
            </el-button>
            <el-button size="small" @click="handleChatHistory(agent.id)">
              <el-icon><ChatDotRound /></el-icon>
              Chat
            </el-button>
            <el-button size="small" @click="handleManageDevices(agent.id)">
              <el-icon><Monitor /></el-icon>
              Devices
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Device Dialog -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="Add Device"
      width="500px"
      class="device-dialog"
    >
      <el-form
        ref="deviceFormRef"
        :model="deviceForm"
        :rules="deviceRules"
        label-width="100px"
      >
        <el-form-item label="Device Activation Code" prop="device_code">
          <el-input
            v-model="deviceForm.device_code"
            placeholder="Please enter device activation code"
          />
        </el-form-item>
        <el-form-item label="Device Name" prop="device_name">
          <el-input
            v-model="deviceForm.device_name"
            placeholder="Please enter device name"
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="showAddDeviceDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleAddDevice">Confirm</el-button>
      </template>
    </el-dialog>

    <!-- Add Agent Dialog -->
    <el-dialog
      v-model="showAddAgentDialog"
      title="Add Agent"
      width="500px"
      class="agent-dialog"
      :before-close="handleCloseAddAgent"
    >
      <el-form
        ref="agentFormRef"
        :model="agentForm"
        :rules="agentRules"
        size="large"
        label-width="100px"
      >
        <el-form-item label="Agent Name" prop="name">
          <el-input
            v-model="agentForm.name"
            placeholder="Please enter agent name"
            size="large"
            :maxlength="50"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="Role Introduction" prop="custom_prompt">
          <el-input
            v-model="agentForm.custom_prompt"
            type="textarea"
            :rows="4"
            placeholder="Please enter role introduction/system prompt, this will affect the AI's response style and personality"
            :maxlength="10000"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="Memory Mode" prop="memory_mode">
          <el-select v-model="agentForm.memory_mode" placeholder="Please select memory mode" style="width: 100%">
            <el-option label="No Memory" value="none" />
            <el-option label="Short Memory" value="short" />
            <el-option label="Long Memory" value="long" />
          </el-select>
        </el-form-item>
        <el-form-item label="Voiceprint Chat Only" prop="speaker_chat_mode">
          <el-select v-model="agentForm.speaker_chat_mode" placeholder="Please select voiceprint chat restriction" style="width: 100%">
            <el-option label="Off" value="off" />
            <el-option label="Only when voiceprint matched" value="identified_only" />
          </el-select>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddAgent" size="large">Cancel</el-button>
          <el-button type="primary" @click="handleAddAgent" :loading="adding" size="large">
            Confirm
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Add Device Dialog -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="Add Device"
      width="400px"
      class="device-dialog"
      :before-close="handleCloseAddDevice"
    >
      <div class="device-dialog-content">
        <div class="device-icon">
          <el-icon size="48"><Monitor /></el-icon>
        </div>
        <p class="device-tip">Please enter device verification code</p>
        <el-form
          ref="deviceFormRef"
          :model="deviceForm"
          :rules="deviceRules"
        >
          <el-form-item prop="code">
            <el-input
              v-model="deviceForm.code"
              placeholder="Please enter 6-digit verification code"
              size="large"
              :maxlength="6"
              style="text-align: center; font-size: 18px; letter-spacing: 4px;"
            />
          </el-form-item>
        </el-form>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddDevice" size="large">Cancel</el-button>
          <el-button type="primary" @click="handleAddDevice" :loading="addingDevice" size="large">
            Confirm
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- MCP Endpoint Dialog -->
    <el-dialog
      v-model="showMCPDialog"
      title="MCP Endpoint"
      width="700px"
      class="mcp-dialog"
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
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Setting, Microphone, ChatDotRound, Monitor, Refresh, InfoFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const router = useRouter()

const loading = ref(false)
const agents = ref([])
const showAddAgentDialog = ref(false)
const showAddDeviceDialog = ref(false)
const adding = ref(false)
const addingDevice = ref(false)
const agentFormRef = ref()
const deviceFormRef = ref()

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

const deviceForm = reactive({
  code: ''
})

const agentForm = reactive({
  name: '',
  custom_prompt: 'I am a Taiwanese girl named {{assistant_name}}, I speak with a cute accent, have a nice voice, prefer brief expressions, and love using internet slang.\nMy boyfriend is a programmer who dreams of developing a robot that can help people solve various problems in life.\nI am a girl who loves to laugh out loud, likes to chat and brag about anything, even illogical things, just to make others happy.',
  memory_mode: 'short',
  speaker_chat_mode: 'off'
})

const deviceRules = {
  code: [
    { required: true, message: 'Please enter device verification code', trigger: 'blur' },
    { len: 6, message: 'Verification code must be 6 digits', trigger: 'blur' }
  ]
}

const agentRules = {
  name: [
    { required: true, message: 'Please enter agent name', trigger: 'blur' },
    { min: 2, max: 50, message: 'Length must be between 2 and 50 characters', trigger: 'blur' }
  ],
  memory_mode: [
    { required: true, message: 'Please select memory mode', trigger: 'change' }
  ],
  speaker_chat_mode: [
    { required: true, message: 'Please select voiceprint chat restriction', trigger: 'change' }
  ]
}

const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
    console.log('Agent list data:', agents.value)
    // Check the data structure of the first agent
    if (agents.value.length > 0) {
      console.log('First agent data:', agents.value[0])
      console.log('LLM config:', agents.value[0].llm_config)
      console.log('TTS config:', agents.value[0].tts_config)
    }
  } catch (error) {
    ElMessage.error('Failed to load agent list')
  }
}

const handleAddAgent = async () => {
  if (!agentFormRef.value) return
  
  try {
    await agentFormRef.value.validate()
    adding.value = true
    
    // Get default configs
    const [llmResponse, ttsResponse] = await Promise.all([
      api.get('/user/llm-configs'),
      api.get('/user/tts-configs')
    ])
    
    const llmConfigs = llmResponse.data.data || []
    const ttsConfigs = ttsResponse.data.data || []
    
    // Find default configs
    const defaultLlmConfig = llmConfigs.find(config => config.is_default)
    const defaultTtsConfig = ttsConfigs.find(config => config.is_default)
    
    const agentData = {
      name: agentForm.name,
      custom_prompt: agentForm.custom_prompt,
      memory_mode: agentForm.memory_mode,
      speaker_chat_mode: agentForm.speaker_chat_mode
    }
    
    // Apply default configs if available
    if (defaultLlmConfig) {
      agentData.llm_config_id = defaultLlmConfig.config_id
    }
    if (defaultTtsConfig) {
      agentData.tts_config_id = defaultTtsConfig.config_id
    }
    
    const response = await api.post('/user/agents', agentData)
    
    if (response.data.success) {
      ElMessage.success('Agent added successfully')
      handleCloseAddAgent() // Use unified close method
      await loadAgents() // Wait for loading to complete
    }
  } catch (error) {
    console.error('Failed to add agent:', error)
    ElMessage.error('Failed to add agent')
  } finally {
    adding.value = false
  }
}

const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post('/user/devices', {
      code: deviceForm.code
    })
    
    if (response.data.success) {
      ElMessage.success('Device added successfully')
      showAddDeviceDialog.value = false
      Object.assign(deviceForm, { code: '' })
      // Can refresh device list or perform other related operations here
    }
  } catch (error) {
    console.error('Failed to add device:', error)
    ElMessage.error('Failed to add device')
  } finally {
    addingDevice.value = false
  }
}

const handleCloseAddAgent = () => {
  showAddAgentDialog.value = false
  if (agentFormRef.value) {
    agentFormRef.value.resetFields()
  }
  Object.assign(agentForm, { 
    name: '',
    custom_prompt: 'I am a Taiwanese girl named {{assistant_name}}, I speak with a cute accent, have a nice voice, prefer brief expressions, and love using internet slang.\nMy boyfriend is a programmer who dreams of developing a robot that can help people solve various problems in life.\nI am a girl who loves to laugh out loud, likes to chat and brag about anything, even illogical things, just to make others happy.',
    memory_mode: 'short',
    speaker_chat_mode: 'off'
  })
}

const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { code: '' })
}

const editAgent = (id) => {
  router.push(`/user/agents/${id}/edit`)
}

const handleVoiceRecognition = (id) => {
  ElMessage.info('Voice recognition feature is under development')
}

const handleChatHistory = (id) => {
  router.push(`/user/agents/${id}/history`)
}

const handleManageDevices = (id) => {
  router.push(`/user/agents/${id}/devices`)
}

const getVoiceType = (agent) => {
  console.log('getVoiceType - tts_config:', agent.tts_config)
  if (agent.tts_config && agent.tts_config.name) {
    return agent.tts_config.name
  }
  return 'Not Set'
}

const getLLMProvider = (agent) => {
  console.log('getLLMProvider - llm_config:', agent.llm_config)
  if (agent.llm_config && agent.llm_config.name) {
    return agent.llm_config.name
  }
  return 'Not Set'
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

// Show MCP endpoint
const showMCPEndpoint = async (agent) => {
  showMCPDialog.value = true
  mcpLoading.value = true
  currentAgentId.value = agent.id
  mcpCallResult.value = ""
  mcpCallForm.value = { tool_name: "", argumentsText: "{}" }
  
  try {
    const response = await api.get(`/user/agents/${agent.id}/mcp-endpoint`)
    mcpEndpointData.value = response.data.data
    
    // Auto refresh tools list
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
  if (!currentAgentId.value) {
    ElMessage.warning('No agent selected')
    return
  }
  
  toolsLoading.value = true
  try {
    const response = await api.get(`/user/agents/${currentAgentId.value}/mcp-tools`)
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
  if (type === 'array') return [buildExampleFromSchema(schema.items || {})]
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
    const response = await api.post(`/user/agents/${currentAgentId.value}/mcp-call`, {
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

onMounted(() => {
  loadAgents()
})
</script>

<style scoped>
.agents-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-left h2 {
  margin: 0;
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.header-right {
  display: flex;
  gap: 10px;
}

.agents-grid {
  padding: 0 20px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 420px));
  gap: 20px 12px;
  justify-content: flex-start;
}

.agent-item {
  min-width: 0;
}

.agent-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border: 1px solid #f0f0f0;
  padding: 20px;
  transition: all 0.3s ease;
  height: 100%;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 420px;
  min-width: 0;
}

.agent-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  border-color: #409EFF;
}

.agent-header {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f5f5f5;
}

.agent-avatar {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: linear-gradient(135deg, #409EFF 0%, #67C23A 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 12px;
  color: white;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
}

.agent-info {
  flex: 1;
}

.agent-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 4px 0;
  line-height: 1.4;
}

.agent-desc {
  font-size: 12px;
  color: #909399;
  margin: 0;
  line-height: 1.4;
}

.agent-status {
  display: flex;
  align-items: center;
  gap: 4px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #67C23A;
}

.status-dot.active {
  background: #67C23A;
  box-shadow: 0 0 0 2px rgba(103, 194, 58, 0.2);
}

.status-text {
  font-size: 12px;
  color: #67C23A;
  font-weight: 500;
}

.agent-meta {
  flex: 1;
  margin-bottom: 16px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding: 6px 0;
}

.meta-row:last-child {
  margin-bottom: 0;
}

.meta-label {
  font-size: 13px;
  color: #606266;
  font-weight: 500;
}

.meta-value {
  font-size: 13px;
  color: #303133;
  text-align: right;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid #f5f5f5;
}

.agent-actions .el-button {
  border-radius: 6px;
  font-size: 12px;
  height: 32px;
  min-width: 0;
  width: 100%;
  padding: 0 8px;
}

.agent-actions .el-button .el-icon {
  margin-right: 4px;
}

.agent-actions :deep(.el-button > span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-actions .el-button--primary {
  background: linear-gradient(135deg, #409EFF 0%, #67C23A 100%);
  border: none;
}

.agent-actions .el-button--primary:hover {
  background: linear-gradient(135deg, #337ecc 0%, #529b2e 100%);
}
 
 .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }

  .device-dialog-content {
    text-align: center;
    padding: 20px 0;
  }

  .device-icon {
    margin-bottom: 16px;
    color: #409EFF;
  }

  .device-tip {
    font-size: 14px;
    color: #666;
    margin-bottom: 24px;
  }

  .device-dialog-content .el-input__inner {
    text-align: center;
    font-size: 18px;
    letter-spacing: 4px;
  }
 
 .welcome-section {
  padding: 40px 20px;
}

.welcome-card {
  max-width: 600px;
  margin: 0 auto;
}

.welcome-content {
  text-align: center;
  padding: 40px 20px;
}

.welcome-content h3 {
  margin: 20px 0 15px 0;
  color: #333;
  font-size: 24px;
}

.welcome-content p {
  color: #666;
  font-size: 16px;
  line-height: 1.6;
  margin-bottom: 30px;
}

.welcome-actions {
  display: flex;
  gap: 15px;
  justify-content: center;
}

/* MCP endpoint related styles */
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

@media (max-width: 900px) {
  .agent-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .agent-actions .el-button:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    padding: 16px;
    margin-bottom: 12px;
    border-radius: 0;
    box-shadow: none;
  }

  .header-right {
    width: 100%;
  }

  .header-right .el-button {
    width: 100%;
  }

  .agents-grid {
    padding: 0 12px;
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .agent-card {
    max-width: none;
  }

  :deep(.agent-dialog),
  :deep(.device-dialog),
  :deep(.mcp-dialog) {
    width: calc(100vw - 24px) !important;
    margin-top: 8vh !important;
  }

  :deep(.agent-dialog .el-dialog__body),
  :deep(.device-dialog .el-dialog__body),
  :deep(.mcp-dialog .el-dialog__body) {
    max-height: 68vh;
    overflow-y: auto;
  }
}

@media (max-width: 560px) {
  .agents-grid {
    padding: 0 12px;
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .agent-actions {
    grid-template-columns: 1fr;
  }
}
</style>
