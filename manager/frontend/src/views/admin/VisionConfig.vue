<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>Vision Configuration Management</h2>
      </div>
    </div>

    <!-- Base Configuration Section -->
    <el-card class="base-config-card" style="margin-bottom: 20px;">
      <template #header>
        <div class="card-header">
          <span>Base Configuration</span>
        </div>
      </template>
      
      <el-form
        ref="baseFormRef"
        :model="baseForm"
        :rules="baseRules"
        label-width="120px"
        style="max-width: 600px;"
      >
        <el-form-item label="Enable Authentication" prop="enable_auth">
          <el-switch v-model="baseForm.enable_auth" />
          <div class="form-tip">Whether to enable authentication for the vision recognition API</div>
        </el-form-item>
        
        <el-form-item label="Vision URL" prop="vision_url">
          <el-input 
            v-model="baseForm.vision_url" 
            placeholder="Please enter Vision API address"
            style="width: 100%;"
          />
          <div class="form-tip">HTTP request address returned to the client for image recognition</div>
        </el-form-item>
        
        <el-form-item>
          <el-button type="primary" @click="saveBaseConfig" :loading="baseSaving">
            Save Base Configuration
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Configuration List Section -->
    <el-card>
      <template #header>
        <div class="card-header">
          <span>Model Configuration List</span>
          <el-button type="primary" @click="showDialog = true">
            <el-icon><Plus /></el-icon>
            Add Configuration
          </el-button>
        </div>
      </template>

      <el-table :data="configs" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="Configuration Name" />
        <el-table-column prop="provider" label="Provider" />
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
        <el-table-column prop="created_at" label="Created At" width="180">
          <template #default="scope">
            {{ formatDate(scope.row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="180">
          <template #default="scope">
            <el-button size="small" @click="editConfig(scope.row)">Edit</el-button>
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
    </el-card>

    <!-- Add/Edit Configuration Dialog -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Edit Vision Configuration' : 'Add Vision Configuration'"
      width="700px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="Provider" prop="provider">
          <el-select v-model="form.provider" placeholder="Please select provider" style="width: 100%">
            <el-option label="Aliyun Vision" value="aliyun_vision" />
            <el-option label="Doubao Vision" value="doubao_vision" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="Configuration Name" prop="name">
          <el-input v-model="form.name" placeholder="Please enter configuration name" />
        </el-form-item>
        
        <el-form-item label="Type" prop="type">
          <el-input v-model="form.type" placeholder="Please enter type" />
        </el-form-item>
        
        <el-form-item label="Model Name" prop="model_name">
          <el-input v-model="form.model_name" placeholder="Please enter model name" />
        </el-form-item>
        
        <el-form-item label="API Key" prop="api_key">
          <el-input v-model="form.api_key" type="password" placeholder="Please enter API key" show-password />
        </el-form-item>
        
        <el-form-item label="Base URL" prop="base_url">
          <el-input v-model="form.base_url" placeholder="Please enter base URL" />
        </el-form-item>
        
        <el-form-item label="Max Tokens" prop="max_tokens">
          <el-input-number v-model="form.max_tokens" :min="1" :max="100000" placeholder="Please enter max tokens" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="Temperature" prop="temperature">
          <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" placeholder="Please enter temperature" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="Top P" prop="top_p">
          <el-input-number v-model="form.top_p" :min="0" :max="1" :step="0.1" placeholder="Please enter Top P" style="width: 100%" />
        </el-form-item>
        
        <el-form-item label="Timeout (seconds)" prop="timeout">
          <el-input-number v-model="form.timeout" :min="1" :max="300" placeholder="Please enter timeout" style="width: 100%" />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="handleDialogClose">Cancel</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          Save
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const configs = ref([])
const loading = ref(false)
const saving = ref(false)
const baseSaving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()
const baseFormRef = ref()

// Base configuration form
const baseForm = reactive({
  enable_auth: false,
  vision_url: ''
})

// Base configuration validation rules
const baseRules = {
  vision_url: [
    { required: true, message: 'Please enter Vision URL', trigger: 'blur' },
    { type: 'url', message: 'Please enter a valid URL', trigger: 'blur' }
  ]
}

const form = reactive({
  name: '',
  provider: 'aliyun_vision',
  is_default: false,
  enabled: true,
  type: 'openai',
  model_name: 'qwen-vl-max',
  api_key: '',
  base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  max_tokens: 1000,
  temperature: 0.1,
  top_p: 0.1,
  timeout: 30
})

const generateConfig = () => {
  return JSON.stringify({
    type: form.type,
    model_name: form.model_name,
    api_key: form.api_key,
    base_url: form.base_url,
    max_tokens: form.max_tokens,
    temperature: form.temperature,
    top_p: form.top_p,
    timeout: form.timeout
  })
}

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  provider: [{ required: true, message: 'Please select provider', trigger: 'change' }],
  type: [{ required: true, message: 'Please enter type', trigger: 'blur' }],
  model_name: [{ required: true, message: 'Please enter model name', trigger: 'blur' }],
  api_key: [{ required: true, message: 'Please enter API key', trigger: 'blur' }],
  base_url: [
    { required: true, message: 'Please enter base URL', trigger: 'blur' },
    { type: 'url', message: 'Please enter a valid URL', trigger: 'blur' }
  ],
  max_tokens: [{ required: true, message: 'Please enter max tokens', trigger: 'blur' }],
  timeout: [{ required: true, message: 'Please enter timeout', trigger: 'blur' }]
}

// Load base configuration
const loadBaseConfig = async () => {
  try {
    const response = await api.get('/admin/vision-base-config')
    const data = response.data.data || {}
    baseForm.enable_auth = data.enable_auth || false
    baseForm.vision_url = data.vision_url || ''
  } catch (error) {
    console.error('Failed to load base configuration:', error)
  }
}

// Save base configuration
const saveBaseConfig = async () => {
  if (!baseFormRef.value) return
  
  await baseFormRef.value.validate(async (valid) => {
    if (valid) {
      baseSaving.value = true
      try {
        await api.put('/admin/vision-base-config', {
          enable_auth: baseForm.enable_auth,
          vision_url: baseForm.vision_url
        })
        ElMessage.success('Base configuration saved successfully')
      } catch (error) {
        ElMessage.error('Save failed, please check network connection and input content')
      } finally {
        baseSaving.value = false
      }
    }
  })
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/vision-configs')
    // Filter out vision_base config to ensure it doesn't appear in the list
    const allConfigs = response.data.data || []
    configs.value = allConfigs.filter(config => config.config_id !== 'vision_base')
  } catch (error) {
    ElMessage.error('Failed to load configurations')
  } finally {
    loading.value = false
  }
}

const editConfig = (config) => {
  editingConfig.value = config
  form.name = config.name
  form.provider = config.provider
  form.is_default = config.is_default
  form.enabled = config.enabled
  
  try {
    const configData = JSON.parse(config.json_data || '{}')
    form.type = configData.type || ''
    form.model_name = configData.model_name || ''
    form.api_key = configData.api_key || ''
    form.base_url = configData.base_url || ''
    form.max_tokens = configData.max_tokens || 4096
    form.temperature = configData.temperature !== undefined ? configData.temperature : 0.7
    form.top_p = configData.top_p !== undefined ? configData.top_p : 0.9
    form.timeout = configData.timeout || 30
  } catch (error) {
    console.error('Failed to parse configuration:', error)
    ElMessage.warning('Configuration format error, reset to default values')
  }
  
  showDialog.value = true
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
          provider: form.provider,
          is_default: isFirstConfig || form.is_default,
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: generateConfig()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/vision-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('Update successful')
        } else {
          await api.post('/admin/vision-configs', configData)
          ElMessage.success('Add successful')
        }
        
        showDialog.value = false
        loadConfigs()
      } catch (error) {
        ElMessage.error('Save failed, please check network connection and input content')
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
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data
    }
    
    await api.put(`/admin/vision-configs/${config.id}`, configData)
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

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this configuration?', 'Prompt', {
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel',
      type: 'warning'
    })
    
    await api.delete(`/admin/vision-configs/${id}`)
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
  Object.assign(form, {
    name: '',
    provider: 'aliyun_vision',
    is_default: false,
    enabled: true,
    type: 'openai',
    model_name: 'qwen-vl-max',
    api_key: '',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    max_tokens: 1000,
    temperature: 0.1,
    top_p: 0.1,
    timeout: 30
  })
  formRef.value?.clearValidate()
}

const handleDialogClose = () => {
  showDialog.value = false
  resetForm()
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadBaseConfig()
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

.base-config-card {
  background: #f8f9fa;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  color: #333;
}

.form-tip {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
  line-height: 1.4;
}
</style>
