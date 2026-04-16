<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>ASR Configuration Management</h2>
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
            @click="testConfig(scope.row, 'asr')"
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
      :title="editingConfig ? 'Edit ASR Configuration' : 'Add ASR Configuration'"
      width="720px"
      @close="handleDialogClose"
    >
      <ASRConfigForm ref="formRef" :model="form" :rules="rules" />
      
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
import ASRConfigForm from './forms/ASRConfigForm.vue'

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

const validateAliyunPcm = (rule, value, callback) => {
  if (value !== 'pcm') {
    callback(new Error('Format must be pcm'))
    return
  }
  callback()
}

const validateAliyun16000 = (rule, value, callback) => {
  if (Number(value) !== 16000) {
    callback(new Error('Sample rate must be 16000'))
    return
  }
  callback()
}

const form = reactive({
  name: '',
  config_id: '',
  provider: '',
  is_default: false,
  enabled: true,
  funasr: {
    host: 'localhost',
    port: 10095,
    mode: 'offline',
    sample_rate: 16000,
    chunk_size: [5, 10, 5],
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: false
  },
  aliyun_funasr: {
    api_key: '',
    ws_url: 'wss://dashscope.aliyuncs.com/api-ws/v1/inference/',
    model: 'fun-asr-realtime',
    format: 'pcm',
    sample_rate: 16000,
    vocabulary_id: '',
    disfluency_removal_enabled: false,
    timeout: 30
  },
  doubao: {
    appid: '',
    access_token: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async',
    model_name: 'bigmodel',
    end_window_size: 800,
    enable_punc: true,
    enable_itn: true,
    enable_ddc: false,
    chunk_duration: 200,
    timeout: 30
  },
  aliyun_qwen3: {
    api_key: '',
    ws_url: 'wss://dashscope.aliyuncs.com/api-ws/v1/realtime',
    model: 'qwen3-asr-flash-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language: 'zh',
    auto_end: true,
    vad_threshold: 0.0,
    vad_silence_ms: 400,
    timeout: 30
  },
  xunfei: {
    appid: '',
    api_key: '',
    api_secret: '',
    host: 'iat-api.xfyun.cn',
    path: '/v2/iat',
    domain: 'iat',
    language: 'zh_cn',
    accent: 'mandarin',
    sample_rate: 16000,
    timeout: 30
  }
})

const rules = computed(() => {
  const base = {
    name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
    config_id: [{ required: true, message: 'Please enter config ID', trigger: 'blur' }],
    provider: [{ required: true, message: 'Please select provider', trigger: 'change' }]
  }
  if (form.provider === 'funasr') {
    return {
      ...base,
      'funasr.host': [{ required: true, message: 'Please enter host address', trigger: 'blur' }],
      'funasr.port': [{ required: true, message: 'Please enter port', trigger: 'blur' }],
      'funasr.mode': [{ required: true, message: 'Please select mode', trigger: 'change' }],
      'funasr.sample_rate': [{ required: true, message: 'Please select sample rate', trigger: 'change' }],
      'funasr.chunk_size': [{ required: true, message: 'Please enter chunk size', trigger: 'blur' }],
      'funasr.chunk_interval': [{ required: true, message: 'Please enter chunk interval', trigger: 'blur' }],
      'funasr.max_connections': [{ required: true, message: 'Please enter maximum connections', trigger: 'blur' }],
      'funasr.timeout': [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
    }
  }
  if (form.provider === 'aliyun_funasr') {
    return {
      ...base,
      'aliyun_funasr.ws_url': [{ required: true, message: 'Please enter WS URL', trigger: 'blur' }],
      'aliyun_funasr.model': [{ required: true, message: 'Please enter model name', trigger: 'blur' }],
      'aliyun_funasr.format': [
        { required: true, message: 'Please select audio format', trigger: 'change' },
        { validator: validateAliyunPcm, trigger: 'change' }
      ],
      'aliyun_funasr.sample_rate': [
        { required: true, message: 'Please select sample rate', trigger: 'change' },
        { validator: validateAliyun16000, trigger: 'change' }
      ],
      'aliyun_funasr.timeout': [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
    }
  }
  if (form.provider === 'doubao') {
    return {
      ...base,
      'doubao.appid': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
      'doubao.access_token': [{ required: true, message: 'Please enter access token', trigger: 'blur' }],
      'doubao.ws_url': [{ required: true, message: 'Please enter WebSocket URL', trigger: 'blur' }],
      'doubao.resource_id': [{ required: true, message: 'Please select resource specification', trigger: 'change' }],
      'doubao.end_window_size': [{ required: true, message: 'Please enter end window size', trigger: 'blur' }],
      'doubao.timeout': [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
    }
  }
  if (form.provider === 'aliyun_qwen3') {
    return {
      ...base,
      'aliyun_qwen3.ws_url': [{ required: true, message: 'Please enter WS URL', trigger: 'blur' }],
      'aliyun_qwen3.model': [{ required: true, message: 'Please enter model name', trigger: 'blur' }],
      'aliyun_qwen3.format': [{ required: true, message: 'Please select audio format', trigger: 'change' }],
      'aliyun_qwen3.sample_rate': [{ required: true, message: 'Please select sample rate', trigger: 'change' }],
      'aliyun_qwen3.language': [{ required: true, message: 'Please enter language', trigger: 'blur' }],
      'aliyun_qwen3.timeout': [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
    }
  }
  if (form.provider === 'xunfei') {
    return {
      ...base,
      'xunfei.appid': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
      'xunfei.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
      'xunfei.api_secret': [{ required: true, message: 'Please enter API Secret', trigger: 'blur' }],
      'xunfei.host': [{ required: true, message: 'Please enter Host', trigger: 'blur' }],
      'xunfei.path': [{ required: true, message: 'Please enter Path', trigger: 'blur' }],
      'xunfei.sample_rate': [{ required: true, message: 'Please enter sample rate', trigger: 'change' }],
      'xunfei.timeout': [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
    }
  }
  return base
})

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/asr-configs')
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
  
  try {
    const configObj = JSON.parse(config.json_data || '{}')
    
    if (configObj.funasr) {
      const funasrConfig = { ...form.funasr, ...configObj.funasr }
      if (typeof funasrConfig.chunk_size === 'number') {
        funasrConfig.chunk_size = [5, 10, 5]
      } else if (!Array.isArray(funasrConfig.chunk_size) || funasrConfig.chunk_size.length !== 3) {
        funasrConfig.chunk_size = [5, 10, 5]
      }
      form.funasr = funasrConfig
    } else if (configObj.aliyun_funasr) {
      form.aliyun_funasr = { ...form.aliyun_funasr, ...configObj.aliyun_funasr }
    } else if (configObj.doubao) {
      form.doubao = { ...form.doubao, ...configObj.doubao }
    } else if (config.provider === 'funasr' && configObj.host) {
      const funasrConfig = { ...form.funasr, ...configObj }
      if (typeof funasrConfig.chunk_size === 'number') {
        funasrConfig.chunk_size = [5, 10, 5]
      } else if (!Array.isArray(funasrConfig.chunk_size) || funasrConfig.chunk_size.length !== 3) {
        funasrConfig.chunk_size = [5, 10, 5]
      }
      form.funasr = funasrConfig
    } else if (config.provider === 'aliyun_funasr' && (configObj.ws_url || configObj.model || configObj.api_key)) {
      form.aliyun_funasr = { ...form.aliyun_funasr, ...configObj }
    } else if (config.provider === 'doubao' && (configObj.appid || configObj.access_token)) {
      form.doubao = { ...form.doubao, ...configObj }
    } else if (configObj.aliyun_qwen3) {
      form.aliyun_qwen3 = { ...form.aliyun_qwen3, ...configObj.aliyun_qwen3 }
    } else if (config.provider === 'aliyun_qwen3' && (configObj.ws_url || configObj.model || configObj.api_key)) {
      form.aliyun_qwen3 = { ...form.aliyun_qwen3, ...configObj }
    } else if (configObj.xunfei) {
      form.xunfei = { ...form.xunfei, ...configObj.xunfei }
    } else if (config.provider === 'xunfei' && (configObj.appid || configObj.api_key || configObj.api_secret)) {
      form.xunfei = { ...form.xunfei, ...configObj }
    }
  } catch (error) {
    console.error('Failed to parse configuration JSON:', error)
  }
  
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) {
    ElMessage.warning('Form not ready, please try again later')
    return
  }
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        const isFirstConfig = !editingConfig.value && configs.value.length === 0
        
        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default,
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: formRef.value.getJsonData()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/asr-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('Configuration updated successfully')
        } else {
          await api.post('/admin/asr-configs', configData)
          ElMessage.success('Configuration created successfully')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        const msg = error.response?.data?.error || error.response?.data?.message || error.message
        ElMessage.error('Save failed: ' + msg)
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
    
    await api.put(`/admin/asr-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? 'Set as default successfully' : 'Removed default successfully')
    
    loadConfigs()
  } catch (error) {
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
        const result = await testSingleConfig('asr', row.config_id)
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
    const result = await testWithData('asr', { [configId]: payload })
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
    
    await api.delete(`/admin/asr-configs/${id}`)
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
  form.name = ''
  form.config_id = ''
  form.provider = ''
  form.is_default = false
  form.enabled = true
  form.funasr = {
    host: 'localhost',
    port: 10095,
    mode: 'offline',
    sample_rate: 16000,
    chunk_size: [5, 10, 5],
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: false
  }
  form.aliyun_funasr = {
    api_key: '',
    ws_url: 'wss://dashscope.aliyuncs.com/api-ws/v1/inference/',
    model: 'fun-asr-realtime',
    format: 'pcm',
    sample_rate: 16000,
    vocabulary_id: '',
    disfluency_removal_enabled: false,
    timeout: 30
  }
  form.doubao = {
    appid: '',
    access_token: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async',
    resource_id: 'volc.bigasr.sauc.duration',
    model_name: 'bigmodel',
    end_window_size: 800,
    enable_punc: true,
    enable_itn: true,
    enable_ddc: false,
    chunk_duration: 200,
    timeout: 30
  }
  form.aliyun_qwen3 = {
    api_key: '',
    ws_url: 'wss://dashscope.aliyuncs.com/api-ws/v1/realtime',
    model: 'qwen3-asr-flash-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language: 'zh',
    auto_end: true,
    vad_threshold: 0.0,
    vad_silence_ms: 400,
    timeout: 30
  }
  form.xunfei = {
    appid: '',
    api_key: '',
    api_secret: '',
    host: 'iat-api.xfyun.cn',
    path: '/v2/iat',
    domain: 'iat',
    language: 'zh_cn',
    accent: 'mandarin',
    sample_rate: 16000,
    timeout: 30
  }
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
