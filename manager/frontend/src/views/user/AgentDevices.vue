<template>
  <div class="agent-devices-page">
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" type="text" class="back-btn">
          <el-icon><ArrowLeft /></el-icon>
          Back
        </el-button>
        <div class="header-info">
          <h2>Device Management</h2>
          <p class="page-subtitle">Manage devices associated with the agent</p>
        </div>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="showAddDeviceDialog = true">
          <el-icon><Plus /></el-icon>
          Add Device
        </el-button>
      </div>
    </div>

    <div v-if="devices.length === 0" class="empty-section">
      <el-card class="empty-card">
        <div class="empty-content">
          <el-icon size="64" color="#909399"><Monitor /></el-icon>
          <h3>No Devices</h3>
          <p>This agent has no associated devices yet.</p>
          <div class="empty-actions">
            <el-button type="primary" size="large" @click="showAddDeviceDialog = true">
              <el-icon><Plus /></el-icon>
              Add First Device
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-else class="devices-grid">
      <div v-for="device in devices" :key="device.id" class="device-item">
        <div class="device-card">
          <div class="device-header">
            <div class="device-icon">
              <el-icon size="28"><Monitor /></el-icon>
            </div>
            <div class="device-info">
              <h3 class="device-name">{{ device.device_name || 'Unnamed Device' }}</h3>
              <p class="device-code">{{ device.device_code }}</p>
            </div>
            <div class="device-status">
              <span :class="['status-dot', isDeviceOnline(device.last_active_at) ? 'online' : 'offline']"></span>
              <span class="status-text">{{ isDeviceOnline(device.last_active_at) ? 'Online' : 'Offline' }}</span>
            </div>
          </div>
          
          <div class="device-meta">
            <div class="meta-row">
              <span class="meta-label">Device Type</span>
              <span class="meta-value">ESP32 Device</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Activation Status</span>
              <span class="meta-value">
                <el-tag :type="device.activated ? 'success' : 'warning'" size="small">
                  {{ device.activated ? 'Activated' : 'Not Activated' }}
                </el-tag>
              </span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Last Active</span>
              <span class="meta-value">{{ formatDate(device.last_active_at) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">Created At</span>
              <span class="meta-value">{{ formatDate(device.created_at) }}</span>
            </div>
          </div>
          
          <div class="device-actions">
            <el-button size="small" @click="handleDeviceRole(device.id)">
              <el-icon><User /></el-icon>
              Role
            </el-button>
            <el-button size="small" @click="handleDeviceMcp(device)">
              <el-icon><Setting /></el-icon>
              MCP
            </el-button>
            <el-button size="small" type="danger" @click="handleRemoveDevice(device.id)">
              <el-icon><Delete /></el-icon>
              Remove
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Device Dialog -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="Add Device"
      width="400px"
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

    <!-- Device MCP Dialog -->

    <el-dialog
      v-model="showMcpDialog"
      title="Device MCP Tools"
      width="760px"
    >
      <div v-loading="mcpLoading">
        <div class="mcp-tools-header">
          <el-button size="small" type="primary" @click="refreshDeviceMcpTools" :loading="toolsLoading">Refresh Tools List</el-button>
        </div>

        <div v-if="mcpTools.length === 0" class="tools-empty">No tools data available</div>
        <div v-else class="tools-tags">
          <el-tag v-for="tool in mcpTools" :key="tool.name" class="tool-tag">{{ tool.name }}</el-tag>
        </div>

        <el-divider />
        <el-form :model="mcpCallForm" label-width="90px">
          <el-form-item label="Tool">
            <el-select v-model="mcpCallForm.tool_name" placeholder="Please select tool" style="width:100%" @change="handleMcpToolChange">
              <el-option v-for="tool in mcpTools" :key="tool.name" :label="tool.name" :value="tool.name" />
            </el-select>
          </el-form-item>
          <el-form-item label="Parameters JSON">
            <el-input v-model="mcpCallForm.argumentsText" type="textarea" :rows="6" placeholder='e.g.: {"query":"hello"}' />
          </el-form-item>
        </el-form>

        <el-button type="primary" @click="callDeviceMcpTool" :loading="callingTool">Call Tool</el-button>

        <el-divider />
        <div class="mcp-result-box">{{ mcpCallResult || 'No call results yet' }}</div>
      </div>
    </el-dialog>


    <!-- Device Role Config Dialog -->
    <el-dialog
      v-model="showRoleConfigDialog"
      title="Device Role Configuration"
      width="700px"
      @close="handleCloseRoleConfig"
    >
      <div v-loading="roleConfigLoading">
        <div class="role-config-content">
          <el-alert
            title="Configuration Instructions"
            type="info"
            :closable="false"
            style="margin-bottom: 16px"
          >
            After associating a role with the device, the device will use the role's configuration (Prompt, LLM, TTS) to override the agent's configuration. To use the agent's configuration, please cancel the role association.
          </el-alert>

          <el-form label-width="120px">
            <el-form-item label="Current Role">
              <div v-if="currentDevice.role_id">
                <el-tag type="success" size="large">Role Associated</el-tag>
                <div class="current-role-info">
                  <p><strong>Role ID:</strong> {{ currentDevice.role_id }}</p>
                </div>
              </div>
              <el-tag v-else type="info" size="large">No Role Associated (Using Agent Configuration)</el-tag>
            </el-form-item>

            <el-form-item label="Select Role">
              <el-select
                v-model="selectedRoleId"
                placeholder="Select role (optional)"
                style="width: 100%"
                clearable
                filterable
                @change="handleRoleSelect"
              >
                <el-option
                  v-for="role in availableRoles"
                  :key="role.id"
                  :label="role.name"
                  :value="role.id"
                >
                  <div class="role-option-item">
                    <div class="role-option-main">
                      <span>{{ role.name }}</span>
                      <el-tag v-if="role.role_type === 'global'" size="small" type="success">Global</el-tag>
                    </div>
                    <el-tag size="small" type="info">LLM: {{ role.llm_config_id || 'Default' }}</el-tag>
                  </div>
                </el-option>
              </el-select>
              <div class="form-help">
                After selecting a role, the device will use the role's configuration. Leave empty to cancel role association.
              </div>
            </el-form-item>

            <el-form-item label="Role Details" v-if="selectedRole">
              <el-card class="role-preview-card">
                <div class="role-preview-content">
                  <p><strong>Name:</strong> {{ selectedRole.name }}</p>
                  <p v-if="selectedRole.description"><strong>Description:</strong> {{ selectedRole.description }}</p>
                  <el-divider />
                  <p><strong>Prompt:</strong></p>
                  <p class="prompt-preview">{{ selectedRole.prompt.substring(0, 200) }}{{ selectedRole.prompt.length > 200 ? '...' : '' }}</p>
                  <div class="role-configs-preview">
                    <el-tag size="small">LLM: {{ selectedRole.llm_config_id || 'Default' }}</el-tag>
                    <el-tag size="small">TTS: {{ selectedRole.tts_config_id || 'Default' }}</el-tag>
                    <el-tag v-if="selectedRole.voice" size="small">Voice: {{ selectedRole.voice }}</el-tag>
                  </div>
                </div>
              </el-card>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <template #footer>
        <el-button @click="handleCloseRoleConfig">Cancel</el-button>
        <el-button
          type="primary"
          @click="handleApplyRole"
          :loading="roleConfigLoading"
          :disabled="!selectedRoleId && !currentDevice.role_id"
        >
          {{ selectedRoleId ? 'Apply Role' : 'Cancel Role' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Plus, Monitor, Setting, Delete, User } from '@element-plus/icons-vue'
import api from '../../utils/api'

const router = useRouter()
const route = useRoute()

const agentId = route.params.id
const devices = ref([])
const showAddDeviceDialog = ref(false)
const addingDevice = ref(false)
const deviceFormRef = ref()

const showMcpDialog = ref(false)
const mcpLoading = ref(false)
const toolsLoading = ref(false)
const callingTool = ref(false)
const currentDeviceId = ref(null)
const mcpTools = ref([])
const mcpCallResult = ref('')
const mcpCallForm = ref({ tool_name: '', argumentsText: '{}' })

// Device role config related
const showRoleConfigDialog = ref(false)
const roleConfigLoading = ref(false)
const currentDevice = ref({})
const selectedRoleId = ref(null)
const selectedRole = ref(null)
const availableRoles = ref([])
const isRoleActive = (role) => role?.status === 'active' || !role?.status

const deviceForm = reactive({
  code: ''
})

const deviceRules = {
  code: [
    { required: true, message: 'Please enter device verification code', trigger: 'blur' },
    { len: 6, message: 'Verification code must be 6 digits', trigger: 'blur' }
  ]
}

const loadDevices = async () => {
  try {
    const response = await api.get(`/user/agents/${agentId}/devices`)
    devices.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load device list')
  }
}

const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post(`/user/agents/${agentId}/devices`, {
      code: deviceForm.code
    })
    
    if (response.data.success) {
      ElMessage.success('Device added successfully')
      handleCloseAddDevice()
      await loadDevices()
    }
  } catch (error) {
    console.error('Failed to add device:', error)
    ElMessage.error('Failed to add device')
  } finally {
    addingDevice.value = false
  }
}

const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { code: '' })
}

const handleDeviceMcp = async (device) => {
  currentDeviceId.value = device.id
  showMcpDialog.value = true
  mcpLoading.value = true
  mcpCallResult.value = ''
  mcpCallForm.value = { tool_name: '', argumentsText: '{}' }
  try {
    await refreshDeviceMcpTools()
  } finally {
    mcpLoading.value = false
  }
}

const refreshDeviceMcpTools = async () => {
  if (!currentDeviceId.value) return
  toolsLoading.value = true
  try {
    const response = await api.get(`/user/devices/${currentDeviceId.value}/mcp-tools`)
    mcpTools.value = response.data.data?.tools || []
    if (!mcpCallForm.value.tool_name && mcpTools.value.length > 0) {
      mcpCallForm.value.tool_name = mcpTools.value[0].name
    }
  } catch (error) {
    ElMessage.error('Failed to get device MCP tools')
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

const callDeviceMcpTool = async () => {
  if (!currentDeviceId.value || !mcpCallForm.value.tool_name) {
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
    const response = await api.post(`/user/devices/${currentDeviceId.value}/mcp-call`, {
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

// Load role list
const loadRoles = async () => {
  try {
    const response = await api.get('/user/roles')
    const globalRoles = response.data.data?.global_roles || []
    const userRoles = response.data.data?.user_roles || []
    availableRoles.value = [...globalRoles, ...userRoles].filter(isRoleActive)
  } catch (error) {
    console.error('Failed to load role list:', error)
  }
}

// Open device role config dialog
const handleDeviceRole = async (deviceId) => {
  const device = devices.value.find(d => d.id === deviceId)
  if (!device) return

  currentDevice.value = { ...device }
  selectedRoleId.value = device.role_id || null
  selectedRole.value = null

  // Load role list (if not loaded yet)
  if (availableRoles.value.length === 0) {
    await loadRoles()
  }

  // If role already associated, find role info
  if (device.role_id) {
    const role = availableRoles.value.find(r => r.id === device.role_id)
    if (role) {
      selectedRole.value = role
    }
  }

  showRoleConfigDialog.value = true
}

// Handle role selection change
const handleRoleSelect = (roleId) => {
  if (!roleId) {
    selectedRole.value = null
    return
  }
  const role = availableRoles.value.find(r => r.id === roleId)
  if (role) {
    selectedRole.value = role
  }
}

// Apply role to device
const handleApplyRole = async () => {
  if (!currentDevice.value.id) return

  roleConfigLoading.value = true
  try {
    const data = {
      role_id: selectedRoleId.value || null
    }

    await api.post(`/devices/${currentDevice.value.id}/apply-role`, data)
    ElMessage.success(selectedRoleId.value ? 'Role applied to device' : 'Device role cancelled')
    showRoleConfigDialog.value = false
    await loadDevices()
  } catch (error) {
    ElMessage.error('Operation failed: ' + (error.response?.data?.error || error.message))
  } finally {
    roleConfigLoading.value = false
  }
}

// Close role config dialog
const handleCloseRoleConfig = () => {
  showRoleConfigDialog.value = false
  currentDevice.value = {}
  selectedRoleId.value = null
  selectedRole.value = null
}

const handleRemoveDevice = async (deviceId) => {
  try {
    await ElMessageBox.confirm(
      'Are you sure you want to remove this device?',
      'Confirm Remove',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning',
      }
    )
    
    const response = await api.delete(`/user/agents/${agentId}/devices/${deviceId}`)
    if (response.data.success) {
      ElMessage.success('Device removed successfully')
      await loadDevices()
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Failed to remove device')
    }
  }
}

const goBack = () => {
  router.push('/agents')
}

const formatDate = (dateString) => {
  if (!dateString) return 'Never'
  return new Date(dateString).toLocaleString('zh-CN')
}

// Determine if device is online (based on last active time)
const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  // Consider online if active within 5 minutes
  return (now - lastActive) < 5 * 60 * 1000
}

onMounted(() => {
  loadDevices()
  loadRoles()
})
</script>

<style scoped>
.agent-devices-page {
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

.header-left {
  display: flex;
  align-items: center;
  gap: 15px;
}

.back-btn {
  padding: 8px;
  color: #409EFF;
}

.header-info h2 {
  margin: 0;
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.empty-section {
  margin-top: 40px;
}

.empty-card {
  text-align: center;
  padding: 40px 20px;
}

.empty-content h3 {
  margin: 20px 0 10px 0;
  color: #333;
}

.empty-content p {
  color: #666;
  margin-bottom: 30px;
}

.devices-grid {
  margin-top: 20px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 440px));
  gap: 20px 12px;
  justify-content: flex-start;
}

.device-item {
  min-width: 0;
}

.device-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
  height: 100%;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 440px;
  min-width: 0;
}

.device-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.device-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.device-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.device-info {
  flex: 1;
  min-width: 0;
}

.device-name {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.device-code {
  margin: 0;
  font-size: 12px;
  color: #999;
  font-family: monospace;
}

.device-status {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ddd;
}

.status-dot.online {
  background: #67c23a;
}

.status-dot.offline {
  background: #f56c6c;
}

.status-text {
  font-size: 12px;
  color: #666;
}

.device-meta {
  flex: 1;
  margin-bottom: 16px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.meta-row:last-child {
  margin-bottom: 0;
}

.meta-label {
  font-size: 12px;
  color: #999;
}

.meta-value {
  font-size: 12px;
  color: #666;
  font-weight: 500;
}

.mcp-tools-header { margin-bottom: 12px; }
.tools-tags { display:flex; flex-wrap:wrap; gap:8px; margin-bottom:12px; }
.tools-empty { color:#909399; margin: 8px 0 16px; }
.mcp-result-box {
  white-space: pre-wrap;
  font-family: monospace;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px;
  min-height: 80px;
}

.device-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: auto;
}

.device-actions .el-button {
  min-width: 0;
  width: 100%;
  padding: 0 8px;
}

.device-actions :deep(.el-button > span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-dialog-content {
  text-align: center;
  padding: 20px 0;
}

.device-dialog-content .device-icon {
  margin: 0 auto 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.device-tip {
  margin-bottom: 20px;
  color: #666;
  font-size: 14px;
}

.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.dialog-footer .el-button {
  min-width: 80px;
}

/* Device role config related styles */
.role-config-content {
  padding: 20px 0;
}

.current-role-info {
  margin-bottom: 16px;
}

.current-role-info p {
  margin: 4px 0;
  color: #666;
}

.role-option-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
}

.role-option-main {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.role-preview-card {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
}

.role-preview-content {
  font-size: 14px;
}

.role-preview-content p {
  margin: 8px 0;
}

.role-preview-content strong {
  color: #333;
  margin-right: 8px;
}

.role-configs-preview {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.prompt-preview {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #666;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: 15px;
  }
  
  .header-left {
    justify-content: flex-start;
  }
  
  .header-right {
    align-self: flex-end;
  }

  .devices-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

.device-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .device-actions .el-button:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .devices-grid {
    gap: 10px;
  }

.device-actions {
    grid-template-columns: 1fr;
  }
}
</style>
