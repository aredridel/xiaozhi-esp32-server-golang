<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>Memory Configuration Management</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="handleAddConfig">
          <el-icon><Plus /></el-icon>
          Add Configuration
        </el-button>
      </div>
    </div>

    <el-table :data="safeConfigs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Configuration Name" />
      <el-table-column prop="config_id" label="Config ID" width="150" />
      <el-table-column prop="provider" label="Provider" width="120">
        <template #default="scope">
          <el-tag :type="getProviderTagType(scope.row.provider)">
            {{ scope.row.provider }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="Status" width="80" align="center">
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
      
      <!-- Empty state slot -->
      <template #empty>
        <div class="empty-state">
          <el-icon size="64" color="#C0C4CC" class="empty-icon">
            <Box />
          </el-icon>
          <div class="empty-text">No Memory Configurations</div>
          <div class="empty-description">Click the "Add Configuration" button above to create your first Memory configuration</div>
          <el-button type="primary" @click="handleAddConfig" class="empty-action">
            <el-icon><Plus /></el-icon>
            Add Configuration
          </el-button>
        </div>
      </template>
    </el-table>

    <!-- Add/Edit Configuration Dialog -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Edit Memory Configuration' : 'Add Memory Configuration'"
      width="600px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="Provider" prop="provider">
          <el-select v-model="form.provider" placeholder="Please select provider" style="width: 100%" @change="handleProviderChange">
            <el-option label="Memobase" value="memobase" />
            <el-option label="Mem0" value="mem0" />
            <el-option label="MemOS" value="memos" />
          </el-select>
        </el-form-item>
        
        <el-form-item label="Configuration Name" prop="name">
          <el-input v-model="form.name" placeholder="Please enter configuration name" />
        </el-form-item>
        
        <el-form-item label="Config ID" prop="config_id">
          <el-input v-model="form.config_id" placeholder="Please enter a unique configuration ID" />
        </el-form-item>
        
        <!-- Memobase configuration fields -->
        <template v-if="form.provider === 'memobase'">
          <el-form-item label="API Key" prop="api_key">
            <el-input v-model="form.api_key" type="password" placeholder="Please enter Memobase API key" show-password />
          </el-form-item>
          
          <el-form-item label="Base URL" prop="base_url">
            <el-input v-model="form.base_url" placeholder="Please enter Memobase base URL" />
          </el-form-item>
          
          <el-form-item label="Enable Search" prop="enable_search">
            <el-switch v-model="form.enable_search" />
          </el-form-item>
          
          <el-form-item label="Search Threshold" prop="search_threshold">
            <el-input-number v-model="form.search_threshold" :min="0" :max="1" :step="0.1" :precision="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="Search TopK" prop="search_top_k">
            <el-input-number v-model="form.search_top_k" :min="1" :step="1" style="width: 100%" />
          </el-form-item>
        </template>
        
        <!-- Mem0 configuration fields -->
        <template v-if="form.provider === 'mem0' || form.provider === 'memos'">
          <el-form-item label="API Key" prop="api_key">
            <el-input v-model="form.api_key" type="password" :placeholder="form.provider === 'memos' ? 'Please enter MemOS compatible API key' : 'Please enter Mem0 API key'" show-password />
          </el-form-item>
          
          <el-form-item label="Base URL" prop="base_url">
            <el-input v-model="form.base_url" :placeholder="form.provider === 'memos' ? 'Please enter MemOS service base URL' : 'Please enter Mem0 base URL'" />
          </el-form-item>

          

          <el-form-item label="Enable Search" prop="enable_search">
            <el-switch v-model="form.enable_search" />
          </el-form-item>
          
          <el-form-item label="Search Threshold" prop="search_threshold">
            <el-input-number v-model="form.search_threshold" :min="0" :max="1" :step="0.1" :precision="1" style="width: 100%" />
          </el-form-item>
          
          <el-form-item label="Search TopK" prop="search_top_k">
            <el-input-number v-model="form.search_top_k" :min="1" :step="1" style="width: 100%" />
          </el-form-item>
        </template>
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
import { ref, reactive, onMounted, computed, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Box } from '@element-plus/icons-vue'
import api from '../../utils/api'

const configs = ref([])
const loading = ref(false)
const saving = ref(false)
const showDialog = ref(false)
const editingConfig = ref(null)
const formRef = ref()

// Ensure configs is always an array
const safeConfigs = computed(() => {
  return Array.isArray(configs.value) ? configs.value : []
})

const form = reactive({
  name: '',
  config_id: '',
  provider: 'memobase',
  is_default: false,
  enabled: true,
  api_key: '',
  base_url: '',
  enable_search: true,
  search_threshold: 0.5,
  search_top_k: 3,
  timeout_ms: 10000
})

// Default URL configuration
const defaultUrls = {
  memobase: 'https://api.memobase.dev',
  mem0: 'https://api.mem0.ai',
  memos: 'https://memos.memtensor.cn/api/openmem/v1'
}


const getProviderTagType = (provider) => {
  if (provider === 'memobase') return 'primary'
  if (provider === 'memos') return 'warning'
  return 'success'
}

const handleProviderChange = (value) => {
  // Clear form fields
  form.api_key = ''
  form.base_url = defaultUrls[value] || ''
  form.enable_search = true
  form.search_threshold = 0.5
  form.search_top_k = 3
  form.timeout_ms = 10000
}

// Generate configuration JSON string
const generateConfig = () => {
  const config = {
    api_key: form.api_key,
    base_url: form.base_url,
    enable_search: form.enable_search,
    search_threshold: form.search_threshold,
    search_top_k: form.search_top_k
  }

  if (form.provider === 'memos') {
    config.timeout_ms = form.timeout_ms
  }

  return JSON.stringify(config)
}

// Parse configuration JSON string
const parseConfig = (jsonData) => {
  try {
    const config = JSON.parse(jsonData)
    form.api_key = config.api_key || ''
    form.base_url = config.base_url || defaultUrls[form.provider] || ''
    form.enable_search = config.enable_search !== undefined ? config.enable_search : true
    form.search_threshold = config.search_threshold !== undefined ? config.search_threshold : 0.5
    form.search_top_k = config.search_top_k !== undefined ? config.search_top_k : 3
    form.timeout_ms = config.timeout_ms !== undefined ? config.timeout_ms : 10000
  } catch (error) {
    console.error('Failed to parse configuration:', error)
  }
}

const rules = {
  name: [
    { required: true, message: 'Please enter configuration name', trigger: 'blur' }
  ],
  config_id: [
    { required: true, message: 'Please enter configuration ID', trigger: 'blur' }
  ],
  provider: [
    { required: true, message: 'Please select provider', trigger: 'change' }
  ],
  api_key: [
    { required: true, message: 'Please enter API key', trigger: 'blur' }
  ],
  base_url: [
    { required: true, message: 'Please enter base URL', trigger: 'blur' }
  ]
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('en-US')
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/memory-configs')
    console.log('API response:', response)
    
    // Use nextTick to ensure reactive update safety
    await nextTick()
    
    // The backend returns { data: configs }, so we need to access response.data.data
    if (response && response.data && response.data.data && Array.isArray(response.data.data)) {
      // Use Object.freeze to prevent accidental modification, then create new array
      const newConfigs = [...response.data.data]
      configs.value = newConfigs
    } else if (response && response.data && response.data.data) {
      // If response.data.data exists but is not an array, wrap it in an array
      configs.value = [response.data.data]
    } else {
      // If no valid data, set to empty array
      configs.value = []
    }
    console.log('Loaded configs:', configs.value)
  } catch (error) {
    console.error('Failed to load configuration:', error)
    ElMessage.error('Failed to load configuration: ' + (error.message || 'Unknown error'))
    // Ensure configs is always an array to prevent render errors
    configs.value = []
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
  } catch (error) {
    return
  }
  
  saving.value = true
  try {
    const configData = {
      name: form.name,
      config_id: form.config_id,
      provider: form.provider,
      enabled: form.enabled,
      is_default: form.is_default,
      json_data: generateConfig()
    }
    
    if (editingConfig.value) {
      await api.put(`/admin/memory-configs/${editingConfig.value.id}`, configData)
      ElMessage.success('Configuration updated successfully')
    } else {
      await api.post('/admin/memory-configs', configData)
      ElMessage.success('Configuration created successfully')
    }
    
    showDialog.value = false
    await loadConfigs()
  } catch (error) {
    ElMessage.error('Save failed: ' + error.message)
  } finally {
    saving.value = false
  }
}

const editConfig = (config) => {
  editingConfig.value = config
  form.name = config.name
  form.config_id = config.config_id
  form.provider = config.provider
  form.enabled = config.enabled
  form.is_default = config.is_default
  
  if (config.json_data) {
    parseConfig(config.json_data)
  }
  
  showDialog.value = true
}

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this configuration?', 'Confirm Delete', {
      type: 'warning'
    })
    
    await api.delete(`/admin/memory-configs/${id}`)
    ElMessage.success('Deleted successfully')
    await loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Delete failed: ' + error.message)
    }
  }
}

const toggleEnable = async (config) => {
  try {
    await api.put(`/admin/memory-configs/${config.id}`, {
      ...config,
      enabled: config.enabled
    })
    ElMessage.success(config.enabled ? 'Enabled' : 'Disabled')
  } catch (error) {
    config.enabled = !config.enabled
    ElMessage.error('Operation failed: ' + error.message)
  }
}

const toggleDefault = async (config) => {
  try {
    if (config.is_default) {
      await api.post(`/admin/memory-configs/${config.id}/set-default`)
      ElMessage.success('Set as default configuration')
      await loadConfigs()
    } else {
      await api.put(`/admin/memory-configs/${config.id}`, {
        name: config.name,
        config_id: config.config_id,
        provider: config.provider,
        enabled: config.enabled,
        is_default: false,
        json_data: config.json_data || ''
      })
      ElMessage.success('Default configuration cancelled (long-term memory disabled)')
      await loadConfigs()
    }
  } catch (error) {
    config.is_default = !config.is_default
    ElMessage.error('Operation failed: ' + error.message)
  }
}

const handleAddConfig = () => {
  // Reset form and set default values
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'memobase',
    is_default: false,
    enabled: true,
    api_key: '',
    base_url: defaultUrls['memobase'], // Set default URL
    enable_search: true,
    search_threshold: 0.5,
    search_top_k: 3,
    timeout_ms: 10000,
  })
  
  editingConfig.value = null
  showDialog.value = true
}

const handleDialogClose = () => {
  showDialog.value = false
  editingConfig.value = null
  
  // Reset form
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'memobase',
    is_default: false,
    enabled: true,
    api_key: '',
    base_url: '',
    enable_search: true,
    search_threshold: 0.5,
    search_top_k: 3,
    timeout_ms: 10000,
  })
  
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

onMounted(() => {
  loadConfigs()
})
</script>

<style scoped>
.config-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left h2 {
  margin: 0;
  color: #303133;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
}

.empty-icon {
  margin-bottom: 16px;
}

.empty-text {
  font-size: 16px;
  color: #606266;
  margin-bottom: 8px;
  font-weight: 500;
}

.empty-description {
  font-size: 14px;
  color: #909399;
  margin-bottom: 24px;
  line-height: 1.5;
}

.empty-action {
  margin-top: 8px;
}
</style>