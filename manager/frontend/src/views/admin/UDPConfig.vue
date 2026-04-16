<template>
  <div class="udp-config">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Connection />
          </el-icon>
          <h1 class="page-title">UDP Configuration Management</h1>
        </div>
      </div>
    </div>

    <!-- Configuration Description -->
    <div class="config-description">
      <el-alert
        title="Configuration Instructions"
        description="Configure UDP connection parameters and network settings. This configuration page is for the main program's built-in UDP server configuration items"
        type="info"
        :closable="false"
        show-icon
      />
    </div>

    <!-- Form Container -->
    <div class="form-container">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="config-form"
        v-loading="loading"
      >
        <!-- Basic Configuration Card -->
        <el-card class="config-card basic-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon">
                <Setting />
              </el-icon>
              <span class="card-title">Basic Configuration</span>
            </div>
          </template>
          
          <div class="form-grid basic-form-grid">
            <el-form-item label="Listen Host" prop="listen_host" class="form-item">
              <el-input v-model="form.listen_host" placeholder="Please enter listen host address" />
            </el-form-item>
            
            <el-form-item label="Listen Port" prop="listen_port" class="form-item">
              <el-input-number v-model="form.listen_port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </div>
        </el-card>

        <!-- External Connection Configuration Card -->
        <el-card class="config-card external-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon external-icon">
                <Link />
              </el-icon>
              <span class="card-title">External Connection Configuration</span>
              <el-tooltip content="The IP and port sent to the terminal in the hello protocol, so the terminal must be able to access it" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
          </template>
          
          <div class="form-grid">
            <el-form-item label="External Host" prop="external_host" class="form-item">
              <el-input v-model="form.external_host" placeholder="Please enter external host address" />
            </el-form-item>
            
            <el-form-item label="External Port" prop="external_port" class="form-item">
              <el-input-number v-model="form.external_port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
          </div>
        </el-card>

        <!-- Action Buttons Area -->
        <div class="action-section">
          <el-button 
            type="primary" 
            @click="handleSave" 
            :loading="saving"
            class="save-button"
            size="large"
          >
            Save Configuration
          </el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Setting, Link, QuestionFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref(null)

const form = ref({
  name: 'UDP Configuration',
  is_default: true,
  external_host: '192.168.0.208',
  external_port: 8990,
  listen_host: '0.0.0.0',
  listen_port: 8990
})

const generateConfig = () => {
  return JSON.stringify({
    external_host: form.external_host,
    external_port: form.external_port,
    listen_host: form.listen_host,
    listen_port: form.listen_port
  })
}

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  external_host: [{ required: true, message: 'Please enter external host address', trigger: 'blur' }],
  external_port: [
    { required: true, message: 'Please enter external port number', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: 'Port number must be between 1-65535', trigger: 'blur' }
  ],
  listen_host: [{ required: true, message: 'Please enter listen host address', trigger: 'blur' }],
  listen_port: [
    { required: true, message: 'Please enter listen port number', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: 'Port number must be between 1-65535', trigger: 'blur' }
  ]
}

const loadConfig = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/udp-configs')
    const configs = response.data.data || []
    if (configs.length > 0) {
      const config = configs[0]
      configId.value = config.id
      
      // Parse JSON configuration
      let configData = {}
      try {
        configData = JSON.parse(config.json_data || '{}')
      } catch (e) {
        console.warn('Failed to parse configuration JSON:', e)
      }
      
      form.value = {
        name: config.name,
        is_default: config.is_default,
        external_host: configData.external_host || '192.168.0.208',
        external_port: configData.external_port || 8990,
        listen_host: configData.listen_host || '0.0.0.0',
        listen_port: configData.listen_port || 8990
      }
    }
  } catch (error) {
    console.error('Failed to load UDP configuration:', error)
    ElMessage.error('Failed to load UDP configuration')
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
      external_host: form.value.external_host,
      external_port: form.value.external_port,
      listen_host: form.value.listen_host,
      listen_port: form.value.listen_port
    }
    
    const payload = {
      name: form.value.name,
      config_id: `udp_${form.value.name.replace(/[^a-zA-Z0-9]/g, '_').toLowerCase()}`,
      is_default: form.value.is_default,
      json_data: JSON.stringify(configData)
    }
    
    if (configId.value) {
      await api.put(`/admin/udp-configs/${configId.value}`, payload)
      ElMessage.success('Configuration updated successfully')
    } else {
      const response = await api.post('/admin/udp-configs', payload)
      configId.value = response.data.data.id
      ElMessage.success('Configuration created successfully')
    }
  } catch (error) {
    console.error('Failed to save configuration:', error)
    ElMessage.error('Failed to save configuration')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.udp-config {
  min-height: 100vh;
  background: #f8f9fa;
  padding: 24px;
}

/* Page Header */
.page-header {
  margin-bottom: 24px;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
}

.title-section {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 8px;
}

.title-icon {
  font-size: 32px;
  color: #409eff;
}

.page-title {
  font-size: 28px;
  font-weight: 600;
  color: #1f2937;
  margin: 0;
  background: linear-gradient(135deg, #409eff 0%, #67c23a 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

/* Configuration Description */
.config-description {
  max-width: 1200px;
  margin: 0 auto 24px;
}

/* Form Container */
.form-container {
  max-width: 1200px;
  margin: 0 auto;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Configuration Card */
.config-card {
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
  overflow: hidden;
}

.config-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

.external-config {
  border-left: 4px solid #e6a23c;
}

.basic-config {
  border-left: 4px solid #409eff;
}

/* Card Header */
.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0;
}

.card-icon {
  font-size: 20px;
  color: #409eff;
}

.external-icon {
  color: #e6a23c;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
}

.help-icon {
  color: #9ca3af;
  cursor: help;
  font-size: 0.875rem;
}

.help-icon:hover {
  color: #6366f1;
}

/* Form Grid */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
  padding: 24px;
}

/* Basic Configuration Form Grid - Display in new line */
.basic-form-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 24px;
  padding: 24px;
}

.form-item {
  margin-bottom: 0;
}

/* Element Plus Component Deep Styles */
:deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  transition: all 0.2s ease;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 3px rgba(64, 158, 255, 0.1);
}

:deep(.el-select .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-input-number .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-switch) {
  --el-switch-on-color: #409eff;
}

:deep(.el-card__header) {
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-bottom: 1px solid #e2e8f0;
  padding: 20px 24px;
}

:deep(.el-card__body) {
  padding: 0;
}

/* Action Buttons Area */
.action-section {
  display: flex;
  justify-content: center;
  padding: 32px 0;
}

.save-button {
  padding: 12px 32px;
  font-size: 16px;
  font-weight: 500;
  border-radius: 8px;
  background: linear-gradient(135deg, #409eff 0%, #67c23a 100%);
  border: none;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
}

.save-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 25px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
}

/* Responsive Design */
@media (max-width: 768px) {
  .udp-config {
    padding: 16px;
  }
  
  .page-title {
    font-size: 20px;
  }
}
</style>