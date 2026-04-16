<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>VAD Configuration Management</h2>
      </div>
      <div class="header-right">
        <el-button
          type="warning"
          plain
          :loading="testingAll"
          @click="testAllConfigs"
          :disabled="!getEnabledConfigs().length"
        >
          Test All
        </el-button>
        <el-button type="primary" @click="showDialog = true">
          <el-icon><Plus /></el-icon>
          Add Configuration
        </el-button>
      </div>
    </div>

    <el-table :data="configs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Configuration Name" />
      <el-table-column prop="config_id" label="Config ID" width="150" />
      <el-table-column prop="provider" label="Provider" />
      <el-table-column prop="enabled" label="Enabled" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.enabled" 
            @change="toggleEnable(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="Default" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.is_default" 
            @change="toggleDefault(scope.row)"
            :disabled="scope.row.is_default && getEnabledConfigs().length === 1"
          />
        </template>
      </el-table-column>
      <el-table-column label="Test Result" width="120" align="center">
        <template #default="scope">
          <template v-if="testResults[scope.row.config_id]">
            <el-tooltip v-if="testResults[scope.row.config_id].ok" :content="formatTestResultTip(testResults[scope.row.config_id])" placement="top">
              <span class="test-result test-ok">{{ formatTestResultLabel(testResults[scope.row.config_id]) }}</span>
            </el-tooltip>
            <el-tooltip v-else :content="testResults[scope.row.config_id].message" placement="top" :show-after="200">
              <span class="test-result test-err">Error</span>
            </el-tooltip>
          </template>
          <span v-else class="test-result test-none">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Created At" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="260">
        <template #default="scope">
          <el-button size="small" @click="editConfig(scope.row)">Edit</el-button>
          <el-button
            size="small"
            type="warning"
            :loading="testingId === scope.row.config_id"
            @click="testConfig(scope.row, 'vad')"
          >
            Test
          </el-button>
          <el-button
            size="small"
            type="danger"
            @click="deleteConfig(scope.row.id)"
          >
            Delete
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Add/Edit Configuration Dialog -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Edit VAD Configuration' : 'Add VAD Configuration'"
      width="600px"
      @close="handleDialogClose"
    >
      <VADConfigForm ref="formRef" :model="form" :rules="rules" />
      
      <template #footer>
        <el-button @click="handleDialogClose">Cancel</el-button>
        <el-button type="warning" plain @click="testCurrentConfig" :loading="testingCurrent">
          Test
        </el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          Save
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest'
import VADConfigForm from './forms/VADConfigForm.vue'

const configs = ref([])
const testingId = ref(null)
const testingAll = ref(false)
const testingCurrent = ref(false)
const testResults = ref({})
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()

const form = reactive({
  name: '',
  config_id: '',
  provider: 'ten_vad',
  is_default: false,
  enabled: true,
  webrtc_vad: {
    pool_min_size: 5,
    pool_max_size: 1000,
    pool_max_idle: 100,
    vad_sample_rate: 16000,
    vad_mode: 2
  },
  silero_vad: {
    model_path: 'config/models/vad/silero_vad.onnx',
    threshold: 0.5,
    min_silence_duration_ms: 100,
    sample_rate: 16000,
    channels: 1,
    pool_size: 10,
    acquire_timeout_ms: 3000
  },
  ten_vad: {
    hop_size: 320,
    threshold: 0.3,
    pool_size: 10,
    acquire_timeout_ms: 3000
  }
})

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Please enter config ID', trigger: 'blur' }],
  provider: [{ required: true, message: 'Please select provider', trigger: 'change' }],
  'webrtc_vad.pool_min_size': [{ required: true, message: 'Please enter minimum pool size', trigger: 'blur' }],
  'webrtc_vad.pool_max_size': [{ required: true, message: 'Please enter maximum pool size', trigger: 'blur' }],
  'webrtc_vad.pool_max_idle': [{ required: true, message: 'Please enter maximum idle connections', trigger: 'blur' }],
  'webrtc_vad.vad_sample_rate': [{ required: true, message: 'Please select VAD sample rate', trigger: 'change' }],
  'webrtc_vad.vad_mode': [{ required: true, message: 'Please select VAD mode', trigger: 'change' }],
  'silero_vad.model_path': [{ required: true, message: 'Please enter model path', trigger: 'blur' }],
  'silero_vad.threshold': [{ required: true, message: 'Please enter threshold', trigger: 'blur' }],
  'silero_vad.min_silence_duration_ms': [{ required: true, message: 'Please enter minimum silence duration', trigger: 'blur' }],
  'silero_vad.sample_rate': [{ required: true, message: 'Please select sample rate', trigger: 'change' }],
  'silero_vad.channels': [{ required: true, message: 'Please select channels', trigger: 'change' }],
  'silero_vad.pool_size': [{ required: true, message: 'Please enter pool size', trigger: 'blur' }],
  'silero_vad.acquire_timeout_ms': [{ required: true, message: 'Please enter acquire timeout', trigger: 'blur' }],
  'ten_vad.hop_size': [{ required: true, message: 'Please enter hop size', trigger: 'blur' }],
  'ten_vad.threshold': [{ required: true, message: 'Please enter VAD detection threshold', trigger: 'blur' }],
  'ten_vad.pool_size': [{ required: true, message: 'Please enter pool size', trigger: 'blur' }],
  'ten_vad.acquire_timeout_ms': [{ required: true, message: 'Please enter acquire timeout', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/vad-configs')
    configs.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load configurations')
  } finally {
    loading.value = false
  }
}

const editConfig = (config) => {
  editingConfig.value = config
  form.name = config.name
  form.config_id = config.config_id
  form.provider = config.provider
  form.is_default = config.is_default
  form.enabled = config.enabled
  
  // Parse configuration JSON and fill in corresponding fields
  try {
    const configObj = JSON.parse(config.json_data || '{}')
    if (configObj.webrtc_vad) {
      form.webrtc_vad = { ...form.webrtc_vad, ...configObj.webrtc_vad }
    } else if (configObj.silero_vad) {
      form.silero_vad = { ...form.silero_vad, ...configObj.silero_vad }
    } else if (configObj.ten_vad) {
      form.ten_vad = { ...form.ten_vad, ...configObj.ten_vad }
    } else {
      if (config.provider === 'webrtc_vad') {
        form.webrtc_vad = { ...form.webrtc_vad, ...configObj }
      } else if (config.provider === 'silero_vad') {
        form.silero_vad = { ...form.silero_vad, ...configObj }
      } else if (config.provider === 'ten_vad') {
        form.ten_vad = { ...form.ten_vad, ...configObj }
      }
    }
  } catch (error) {
    console.error('Failed to parse configuration JSON:', error)
  }
  
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        // If adding new config and no configs exist, auto-set as default
        const isFirstConfig = !editingConfig.value && configs.value.length === 0
        
        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // Auto-set as default on first add
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: formRef.value.getJsonData()
        }

        if (editingConfig.value) {
          await api.put(`/admin/vad-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('Configuration updated successfully')
        } else {
          await api.post('/admin/vad-configs', configData)
          ElMessage.success('Configuration created successfully')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        ElMessage.error('Save failed: ' + (error.response?.data?.message || error.message))
      } finally {
        saving.value = false
      }
    }
  })
}

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`)
    ElMessage.success(`${config.enabled ? 'Enabled' : 'Disabled'} successfully`)
  } catch (error) {
    // Restore switch state
    config.enabled = !config.enabled
    ElMessage.error('Operation failed')
  }
}

const toggleDefault = async (config) => {
  try {
    if (!config.enabled) {
      ElMessage.warning('Please enable this configuration first before setting it as default')
      config.is_default = false
      return
    }
    
    const configData = {
      name: config.name,
      config_id: config.config_id,
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data
    }
    
    await api.put(`/admin/vad-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? 'Set as default successfully' : 'Removed default successfully')
    
    // Refresh list to update default status of other configurations
    loadConfigs()
  } catch (error) {
    // Restore switch state
    config.is_default = !config.is_default
    ElMessage.error('Operation failed')
  }
}

const getEnabledConfigs = () => {
  return configs.value.filter(config => config.enabled)
}

function formatTestResultLabel(r) {
  if (!r?.ok) return 'Error'
  return r.first_packet_ms != null ? `OK ${r.first_packet_ms}ms` : 'OK'
}
function formatTestResultTip(r) {
  if (!r?.ok) return ''
  return r.first_packet_ms != null ? `Passed, took ${r.first_packet_ms}ms` : 'Passed'
}
function formatTestMessage(result) {
  const base = result.message || ''
  return result.first_packet_ms != null ? `${base} ${result.first_packet_ms}ms` : base
}

const testConfig = async (row, type) => {
  testingId.value = row.config_id
  try {
    const result = await testSingleConfig(type, row.config_id)
    testResults.value = { ...testResults.value, [row.config_id]: result }
    if (result.ok) {
      ElMessage.success(`${row.name || row.config_id}: ${formatTestMessage(result)}`)
    } else {
      ElMessage.warning(`${row.name || row.config_id}: ${result.message}`)
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Test request failed')
  } finally {
    testingId.value = null
  }
}

const testAllConfigs = async () => {
  const list = getEnabledConfigs()
  if (!list.length) {
    ElMessage.warning('No enabled configurations')
    return
  }
  testingAll.value = true
  testResults.value = {}
  let okCount = 0
  try {
    for (const row of list) {
      try {
        const result = await testSingleConfig('vad', row.config_id)
        testResults.value = { ...testResults.value, [row.config_id]: result }
        if (result.ok) okCount++
      } catch (_) {
        testResults.value = { ...testResults.value, [row.config_id]: { ok: false, message: 'Request failed' } }
      }
    }
    ElMessage.success(`All tests completed: ${okCount}/${list.length} passed`)
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Test request failed')
  } finally {
    testingAll.value = false
  }
}

const testCurrentConfig = async () => {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch (_) {
    return
  }
  const configId = form.config_id?.trim()
  if (!configId) {
    ElMessage.warning('Please enter config ID')
    return
  }
  const payload = {
    name: form.name,
    config_id: configId,
    provider: form.provider,
    is_default: form.is_default,
    ...parseJsonData(formRef.value.getJsonData())
  }
  testingCurrent.value = true
  try {
    const result = await testWithData('vad', { [configId]: payload })
    if (result.ok) {
      ElMessage.success(formatTestMessage(result) || 'Test passed')
    } else {
      ElMessage.warning(result.message || 'Test failed')
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Test request failed')
  } finally {
    testingCurrent.value = false
  }
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this configuration?', 'Confirm', {
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel',
      type: 'warning'
    })
    
    await api.delete(`/admin/vad-configs/${id}`)
    ElMessage.success('Deleted successfully')
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Delete failed')
    }
  }
}

const resetForm = () => {
  editingConfig.value = null
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'ten_vad',
    is_default: false,
    enabled: true,
    webrtc_vad: {
      pool_min_size: 5,
      pool_max_size: 1000,
      pool_max_idle: 100,
      vad_sample_rate: 16000,
      vad_mode: 2
    },
    silero_vad: {
      model_path: 'config/models/vad/silero_vad.onnx',
      threshold: 0.5,
      min_silence_duration_ms: 100,
      sample_rate: 16000,
      channels: 1,
      pool_size: 10,
      acquire_timeout_ms: 3000
    },
    ten_vad: {
      hop_size: 320,
      threshold: 0.3,
      pool_size: 10,
      acquire_timeout_ms: 3000
    }
  })
}

const handleDialogClose = () => {
  showDialog.value = false
  resetForm()
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('en-US')
}

onMounted(() => {
  loadConfigs()
})
</script>

<style scoped>
.config-page {
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left h2 {
  margin: 0;
  color: #333;
}

.test-result { font-size: 12px; }
.test-result.test-ok { color: var(--el-color-success); }
.test-result.test-err { color: var(--el-color-danger); cursor: help; }
.test-result.test-none { color: var(--el-text-color-placeholder); }
</style>
