<template>
  <div class="admin-devices">
    <div class="page-header">
      <h2>Device Management</h2>
      <p class="page-subtitle">Manage all devices in the system</p>
    </div>

    <div class="toolbar">
      <el-button type="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon>
        Add Device
      </el-button>
      <el-button @click="loadDevices">
        <el-icon><Refresh /></el-icon>
        Refresh
      </el-button>
    </div>

    <el-table :data="devices" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="device_code" label="Activation Code" width="150" />
      <el-table-column prop="device_name" label="Device Name" width="150" />
      <el-table-column prop="user_id" label="User ID" width="100" />
      <el-table-column label="Linked Agent" width="150">
        <template #default="{ row }">
          <span v-if="row.agent_id > 0">
            Agent {{ row.agent_id }}
          </span>
          <el-tag v-else type="info" size="small">Not Assigned</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Activation Status" width="100">
        <template #default="{ row }">
          <el-tag :type="row.activated ? 'success' : 'warning'">
            {{ row.activated ? 'Activated' : 'Not Activated' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Online Status" width="100">
        <template #default="{ row }">
          <el-tag :type="isDeviceOnline(row.last_active_at) ? 'success' : 'danger'">
            {{ isDeviceOnline(row.last_active_at) ? 'Online' : 'Offline' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_active_at" label="Last Active Time" width="180">
        <template #default="{ row }">
          {{ row.last_active_at ? new Date(row.last_active_at).toLocaleString() : 'Never Active' }}
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Created At" width="180">
        <template #default="{ row }">
          {{ new Date(row.created_at).toLocaleString() }}
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="300">
        <template #default="{ row }">
          <el-button size="small" @click="editDevice(row)">
            Edit
          </el-button>
          <el-button size="small" type="primary" @click="showDeviceMcp(row)">
            MCP
          </el-button>
          <el-button size="small" type="danger" @click="deleteDevice(row)">
            Delete
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showMcpDialog" title="Device MCP Tools" width="760px">
      <div v-loading="mcpLoading">
        <div class="mcp-tools-header">
          <el-button size="small" type="primary" @click="refreshDeviceMcpTools" :loading="toolsLoading">Refresh Tool List</el-button>
        </div>

        <div v-if="mcpTools.length === 0" class="tools-empty">No tool data available</div>
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
          <el-form-item label="Params JSON">
            <el-input v-model="mcpCallForm.argumentsText" type="textarea" :rows="6" placeholder='e.g. {"query":"hello"}' />
          </el-form-item>
        </el-form>

        <el-button type="primary" @click="callDeviceMcpTool" :loading="callingTool">Call Tool</el-button>

        <el-divider />
        <div class="endpoint-content">{{ mcpCallResult || 'No call result yet' }}</div>
      </div>
    </el-dialog>

    <el-dialog
      v-model="showAddDialog"
      :title="editingDevice ? 'Edit Device' : 'Add Device'"
      width="500px"
    >
      <el-form :model="deviceForm" :rules="deviceRules" ref="deviceFormRef" label-width="100px">
        <el-form-item label="User ID" prop="user_id">
          <el-input-number v-model="deviceForm.user_id" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="Activation Code" prop="device_code">
          <el-input 
            v-model="deviceForm.device_code" 
            :placeholder="editingDevice ? 'Please enter activation code' : 'Please enter activation code (optional with device name)'" 
          />
        </el-form-item>
        <el-form-item label="Device Name" prop="device_name">
          <el-input 
            v-model="deviceForm.device_name" 
            :placeholder="editingDevice ? 'Please enter device name' : 'Please enter device name (optional with device code)'" 
          />
        </el-form-item>
        <el-form-item label="Activation Status" prop="activated">
          <el-switch v-model="deviceForm.activated" />
        </el-form-item>
        <el-form-item label="Linked Agent" prop="agent_id">
          <el-select v-model="deviceForm.agent_id" placeholder="Please select agent" style="width: 100%" clearable>
            <el-option label="No Agent Linked" :value="0" />
            <el-option 
              v-for="agent in agents" 
              :key="agent.id" 
              :label="`${agent.name} (User${agent.user_id})`" 
              :value="agent.id" 
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">Cancel</el-button>
        <el-button type="primary" @click="saveDevice" :loading="saving">
          {{ editingDevice ? 'Update' : 'Add' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { useAuthStore } from '../../stores/auth'

const devices = ref([])
const agents = ref([])
const loading = ref(false)
const showAddDialog = ref(false)
const editingDevice = ref(null)
const saving = ref(false)
const deviceFormRef = ref()

const showMcpDialog = ref(false)
const mcpLoading = ref(false)
const toolsLoading = ref(false)
const callingTool = ref(false)
const currentDeviceId = ref(null)
const mcpTools = ref([])
const mcpCallResult = ref('')
const mcpCallForm = ref({ tool_name: '', argumentsText: '{}' })
const authStore = useAuthStore()

const deviceForm = ref({
  user_id: authStore.user?.id || null,
  device_code: '',
  device_name: '',
  activated: true,
  agent_id: 0
})

const deviceRules = {
  user_id: [{ required: true, message: 'Please enter user ID', trigger: 'blur' }],
  device_code: [
    {
      validator: (rule, value, callback) => {
        if (editingDevice.value) {
          if (!value) {
            callback(new Error('Please enter activation code'))
          } else {
            callback()
          }
          return
        }
        
        if (!value && !deviceForm.value.device_name) {
          callback(new Error('Please enter at least one of activation code or device name'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  device_name: [
    {
      validator: (rule, value, callback) => {
        if (editingDevice.value) {
          if (!value) {
            callback(new Error('Please enter device name'))
          } else {
            callback()
          }
          return
        }
        
        if (!value && !deviceForm.value.device_code) {
          callback(new Error('Please enter at least one of activation code or device name'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadDevices = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/devices')
    devices.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load device list')
    console.error('Error loading devices:', error)
  } finally {
    loading.value = false
  }
}

const loadAgents = async () => {
  try {
    const response = await api.get('/admin/agents')
    agents.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load agent list')
    console.error('Error loading agents:', error)
  }
}

const openAddDialog = () => {
  editingDevice.value = null
  deviceForm.value = {
    user_id: authStore.user?.id || null,
    device_code: '',
    device_name: '',
    activated: true,
    agent_id: 0
  }
  showAddDialog.value = true
}

const validateDeviceCode = async (deviceCode) => {
  if (!deviceCode) return null
  
  try {
    const response = await api.get(`/admin/devices/validate-code?code=${deviceCode}`)
    return response.data.exists
  } catch (error) {
    console.error('Failed to validate activation code:', error)
    return null
  }
}

const editDevice = (device) => {
  editingDevice.value = device
  deviceForm.value = {
    user_id: device.user_id,
    device_code: device.device_code,
    device_name: device.device_name,
    activated: device.activated,
    agent_id: device.agent_id || 0
  }
  showAddDialog.value = true
}

const saveDevice = async () => {
  if (!deviceFormRef.value) return
  
  const valid = await deviceFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    if (editingDevice.value) {
      await api.put(`/admin/devices/${editingDevice.value.id}`, deviceForm.value)
      ElMessage.success('Device updated successfully')
    } else {
      const response = await api.post('/admin/devices', deviceForm.value)
      const message = response.data.message || 'Device added successfully'
      ElMessage.success(message)
    }
    showAddDialog.value = false
    resetForm()
    loadDevices()
  } catch (error) {
    const errorMessage = error.response?.data?.error || (editingDevice.value ? 'Failed to update device' : 'Failed to add device')
    ElMessage.error(errorMessage)
    console.error('Error saving device:', error)
  } finally {
    saving.value = false
  }
}

const deleteDevice = async (device) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete device "${device.device_name}"?`,
      'Confirm Delete',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/devices/${device.id}`)
    ElMessage.success('Device deleted successfully')
    loadDevices()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Failed to delete device')
      console.error('Error deleting device:', error)
    }
  }
}

const showDeviceMcp = async (device) => {
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
    const response = await api.get(`/admin/devices/${currentDeviceId.value}/mcp-tools`)
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
    const response = await api.post(`/admin/devices/${currentDeviceId.value}/mcp-call`, {
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

const resetForm = () => {
  editingDevice.value = null
  deviceForm.value = {
    user_id: authStore.user?.id || null,
    device_code: '',
    device_name: '',
    activated: true,
    agent_id: 0
  }
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
}

const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  return (now - lastActive) < 5 * 60 * 1000
}

onMounted(() => {
  loadDevices()
  loadAgents()
})
</script>

<style scoped>
.admin-devices {
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

.tools-tags { display:flex; flex-wrap:wrap; gap:8px; margin-bottom:12px; }
.tools-empty { color:#909399; margin: 8px 0 16px; }
.endpoint-content {
  white-space: pre-wrap;
  font-family: monospace;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px;
  min-height: 80px;
}
.mcp-tools-header { margin-bottom: 12px; }
</style>
