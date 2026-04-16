<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>LLM Configuration Management</h2>
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
        <el-button type="primary" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          Add Configuration
        </el-button>
      </div>
    </div>

    <el-table :data="configs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Configuration Name" />
      <el-table-column prop="config_id" label="Config ID" width="150" />
      <el-table-column prop="provider" label="Type">
        <template #default="scope">
          {{ getProviderLabel(scope.row.provider) }}
        </template>
      </el-table-column>
      <el-table-column label="Thinking" width="100" align="center">
        <template #default="scope">
          <el-tag
            v-if="getThinkingLabel(scope.row)"
            size="small"
            effect="plain"
            :type="testResults[scope.row.config_id]?.ok ? 'success' : 'info'"
            class="thinking-tag"
          >
            {{ getThinkingLabel(scope.row) }}
          </el-tag>
          <span v-else class="test-result test-none">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="Enable Status" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.enabled" 
            @change="toggleEnable(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="Default Config" width="80" align="center">
        <template #default="scope">
          <el-switch 
            v-model="scope.row.is_default" 
            @change="toggleDefault(scope.row)"
            :disabled="scope.row.is_default && getEnabledConfigs().length === 1"
          />
        </template>
      </el-table-column>
      <el-table-column label="Duration" width="100" align="center">
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
            @click="testConfig(scope.row, 'llm')"
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

    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Edit LLM Configuration' : 'Add LLM Configuration'"
      width="600px"
      @close="handleDialogClose"
    >
      <LLMConfigForm ref="formRef" :model="form" :rules="rules" />
      
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
import { ref, reactive, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest'
import LLMConfigForm from './forms/LLMConfigForm.vue'
import { getProviderFixedType, getProviderThinkingConfig, isProviderBaseURLEditable, resolveLLMProvider } from './forms/llmCatalog'

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
  provider: '',
  is_default: false,
  enabled: true,
  type: 'openai',
  model_name: 'gpt-3.5-turbo',
  api_key: '',
  base_url: 'https://api.openai.com/v1',
  max_tokens: 4000,
  temperature: 0.7,
  top_p: 0.9,
  thinking_mode: 'default',
  thinking_budget_tokens: null,
  thinking_effort: 'medium',
  thinking_clear_thinking: 'default',
  bot_id: '',
  user_prefix: '',
  connector_id: '1024'
})

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Please enter config ID', trigger: 'blur' }],
  provider: [{ required: true, message: 'Please select provider', trigger: 'change' }],
  model_name: [{
    required: true,
    message: 'Please enter model name',
    trigger: 'change'
  }, {
    validator: (_, value, callback) => {
      const provider = resolveLLMProvider(form.provider, form.type)
      const providerType = getProviderFixedType(provider)
      if ((providerType === 'openai' || providerType === 'ollama') && !value) {
        callback(new Error('Please enter model name'))
        return
      }
      callback()
    },
    trigger: 'change'
  }],
  api_key: [{
    validator: (_, value, callback) => {
      const provider = resolveLLMProvider(form.provider, form.type)
      if (getProviderFixedType(provider) !== 'ollama' && !value) {
        callback(new Error('Please enter API key'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  base_url: [{
    validator: (_, value, callback) => {
      const provider = resolveLLMProvider(form.provider, form.type)
      if (isProviderBaseURLEditable(provider) && !value) {
        callback(new Error('Please enter base URL'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  max_tokens: [{
    validator: (_, value, callback) => {
      const provider = resolveLLMProvider(form.provider, form.type)
      const providerType = getProviderFixedType(provider)
      if ((providerType === 'openai' || providerType === 'ollama') && (!value || Number(value) < 1 || Number(value) > 100000)) {
        callback(new Error('max_tokens must be between 1-100000'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  bot_id: [{
    validator: (_, value, callback) => {
      const provider = resolveLLMProvider(form.provider, form.type)
      if (getProviderFixedType(provider) === 'coze' && !value) {
        callback(new Error('Please enter Coze Bot ID'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  temperature: [{ type: 'number', min: 0, max: 2, message: 'Temperature must be between 0-2', trigger: 'blur' }],
  top_p: [{ type: 'number', min: 0, max: 1, message: 'Top P must be between 0-1', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/llm-configs')
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
    const detectedProvider = resolveLLMProvider(config.provider, configObj.type)
    const detectedType = getProviderFixedType(detectedProvider)
    form.provider = detectedProvider
    form.type = detectedType
    form.model_name = configObj.model_name || (detectedType === 'coze' ? 'coze' : (detectedType === 'dify' ? 'dify' : ''))
    form.api_key = configObj.api_key || ''
    form.base_url = configObj.base_url || (detectedType === 'coze' ? 'https://api.coze.com' : (detectedType === 'dify' ? 'https://api.dify.ai/v1' : ''))
    form.max_tokens = configObj.max_tokens || 4000
    form.temperature = configObj.temperature || 0.7
    form.top_p = configObj.top_p || 0.9
    form.thinking_mode = configObj.thinking?.mode || 'default'
    form.thinking_budget_tokens = configObj.thinking?.budget_tokens !== undefined ? Number(configObj.thinking.budget_tokens) || null : null
    form.thinking_effort = configObj.thinking?.effort || 'medium'
    form.thinking_clear_thinking = configObj.thinking?.clear_thinking !== undefined ? configObj.thinking.clear_thinking : 'default'
    form.bot_id = configObj.bot_id || ''
    form.user_prefix = configObj.user_prefix || ''
    form.connector_id = configObj.connector_id || '1024'
  } catch (error) {
    console.error('Failed to parse configuration JSON:', error)
  }
  
  showDialog.value = true
  nextTick(() => {
    formRef.value?.clearValidate?.()
  })
}

const openCreateDialog = () => {
  resetForm()
  showDialog.value = true
  nextTick(() => {
    formRef.value?.clearValidate?.()
  })
}

const handleSave = async () => {
  if (!formRef.value) return
  
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
          await api.put(`/admin/llm-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('Configuration updated successfully')
        } else {
          await api.post('/admin/llm-configs', configData)
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
    ElMessage.success(`${config.enabled ? 'Enable' : 'Disable'} successful`)
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
    
    await api.put(`/admin/llm-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? 'Set as default successful' : 'Cancel default successful')
    
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
  return r.first_packet_ms != null ? `${r.first_packet_ms}ms` : 'Pass'
}
function formatTestResultTip(r) {
  if (!r?.ok) return ''
  const parts = []
  if (r.first_packet_ms != null) parts.push(`First packet ${r.first_packet_ms}ms`)
  if (r.reasoning_content_returned) parts.push('Upstream reasoning content detected')
  return parts.length ? parts.join(', ') : 'Pass'
}
function formatTestMessage(result) {
  const base = result.message || ''
  const suffix = []
  if (result.first_packet_ms != null) suffix.push(`${result.first_packet_ms}ms`)
  if (result.reasoning_content_returned) suffix.push('Upstream reasoning content detected')
  return suffix.length ? `${base} ${suffix.join(' · ')}` : base
}

function formatDraftTestLabel(name, configId) {
  return name?.trim() || configId?.trim() || 'Current Config'
}

function getThinkingLabel(row) {
  const config = parseJsonData(row?.json_data)
  const provider = resolveLLMProvider(row?.provider, config?.type)
  const mode = String(config?.thinking?.mode || '').trim()
  if (!mode || mode === 'default') {
    return ''
  }

  const thinkingConfig = getProviderThinkingConfig(provider, config?.model_name)
  const option = (thinkingConfig?.options || []).find(item => item.value === mode)
  return option?.label || mode
}

function getProviderLabel(provider) {
  const labels = {
    azure: 'Azure OpenAI',
    anthropic: 'Anthropic',
    zhipu: 'Zhipu AI',
    aliyun: 'Aliyun',
    doubao: 'Doubao',
    siliconflow: 'SiliconFlow',
    deepseek: 'DeepSeek',
    openai: 'OpenAI',
    ollama: 'Ollama',
    dify: 'Dify',
    coze: 'Coze'
  }
  return labels[provider] || provider
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
        const result = await testSingleConfig('llm', row.config_id)
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
    const result = await testWithData('llm', { provider: configId, [configId]: payload })
    const label = formatDraftTestLabel(form.name, configId)
    if (result.ok) {
      ElMessage.success(`${label}: ${formatTestMessage(result) || 'Test passed'}`)
    } else {
      ElMessage.warning(`${label}: ${result.message || 'Test failed'}`)
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Test request failed')
  } finally {
    testingCurrent.value = false
  }
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this configuration?', 'Prompt', {
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel',
      type: 'warning'
    })
    
    await api.delete(`/admin/llm-configs/${id}`)
    ElMessage.success('Delete successful')
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
  form.type = 'openai'
  form.model_name = ''
  form.api_key = ''
  form.base_url = ''
  form.max_tokens = 4000
  form.temperature = 0.7
  form.top_p = 0.9
  form.thinking_mode = 'default'
  form.thinking_budget_tokens = null
  form.thinking_effort = 'medium'
  form.thinking_clear_thinking = 'default'
  form.bot_id = ''
  form.user_prefix = ''
  form.connector_id = '1024'
}

const handleDialogClose = () => {
  showDialog.value = false
  resetForm()
  nextTick(() => {
    formRef.value?.clearValidate?.()
  })
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
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

.thinking-tag {
  max-width: 84px;
}

.test-result { font-size: 12px; }
.test-result.test-ok { color: var(--el-color-success); }
.test-result.test-err { color: var(--el-color-danger); cursor: help; }
.test-result.test-none { color: var(--el-text-color-placeholder); }
</style>
