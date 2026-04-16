<template>
  <div class="mqtt-server-config">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Monitor />
          </el-icon>
          <h1 class="page-title">MQTT Server Configuration Management</h1>
        </div>
      </div>
    </div>

    <!-- Configuration Description -->
    <div class="config-description">
      <el-alert
        title="Configuration Instructions"
        description="Configure MQTT server parameters and security settings. Built-in mqtt server configuration items"
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
            <el-form-item label="Enabled" prop="enable" class="form-item">
              <el-switch v-model="form.enable" />
            </el-form-item>
            
            <el-form-item label="Listen Host" prop="listen_host" class="form-item">
              <el-input v-model="form.listen_host" placeholder="Please enter listen host address" style="max-width: 300px" />
            </el-form-item>
            
            <el-form-item label="Listen Port" prop="listen_port" class="form-item">
              <el-input-number v-model="form.listen_port" :min="1" :max="65535" placeholder="Please enter listen port number" style="max-width: 200px" />
            </el-form-item>
          </div>
        </el-card>

        <!-- Authentication Configuration Card -->
        <el-card class="config-card auth-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon auth-icon">
                <User />
              </el-icon>
              <span class="card-title">Authentication Configuration</span>
            </div>
          </template>
          
          <!-- Hint Information -->
          <div class="config-tip">
            <el-icon class="tip-icon">
              <InfoFilled />
            </el-icon>
            <span class="tip-text">Username and password used by the main program to connect to mqtt server</span>
          </div>
          
          <div class="form-grid auth-form-grid">
            <el-form-item label="Enable Auth" prop="enable_auth" class="form-item">
              <div class="form-item-with-help">
                <el-switch v-model="form.enable_auth" />
                <el-tooltip content="Will validate mqtt client connection username and password" placement="top">
                  <el-icon class="help-icon"><QuestionFilled /></el-icon>
                </el-tooltip>
              </div>
            </el-form-item>
            
            <div class="form-row">
              <el-form-item label="Admin User" prop="username" class="form-item">
                <el-input v-model="form.username" placeholder="Please enter admin username" style="max-width: 250px" />
              </el-form-item>
              
              <el-form-item label="Admin Password" prop="password" class="form-item">
                <el-input v-model="form.password" type="password" placeholder="Please enter admin password" show-password style="max-width: 250px" />
              </el-form-item>
            </div>
            
            <el-form-item label="Signature Key" prop="signature_key" class="form-item">
              <el-input v-model="form.signature_key" placeholder="Please enter signature key" style="max-width: 400px" />
              <div class="form-item-hint">
                Corresponds to the signature key on the ota configuration page
              </div>
            </el-form-item>
          </div>
        </el-card>

        <!-- TLS Configuration Card -->
        <el-card class="config-card tls-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon tls-icon">
                <Lock />
              </el-icon>
              <span class="card-title">TLS Configuration</span>
              <el-tooltip content="Enable mqtts connection for mqtt server" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
          </template>
          
          <div class="form-grid tls-form-grid">
            <div class="form-row">
              <el-form-item label="Enable TLS" prop="tls.enable" class="form-item">
                <el-switch v-model="form.tls.enable" />
              </el-form-item>
              
              <el-form-item label="TLS Port" prop="tls.port" v-if="form.tls.enable" class="form-item">
                <el-input-number v-model="form.tls.port" :min="1" :max="65535" placeholder="Please enter TLS port number" style="max-width: 200px" />
              </el-form-item>
            </div>
            
            <el-form-item label="Certificate File" prop="tls.pem" v-if="form.tls.enable" class="form-item">
              <el-input v-model="form.tls.pem" placeholder="Please enter certificate file path" style="max-width: 400px" />
            </el-form-item>
            
            <el-form-item label="Key File" prop="tls.key" v-if="form.tls.enable" class="form-item">
              <el-input v-model="form.tls.key" placeholder="Please enter key file path" style="max-width: 400px" />
            </el-form-item>
          </div>
        </el-card>

        <!-- Action Buttons -->
        <div class="action-section">
          <el-button type="primary" @click="handleSave" :loading="saving" class="save-button">
            Save Configuration
          </el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Monitor, Setting, Platform, User, Lock, InfoFilled, QuestionFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref(null)

const form = reactive({
  enable: true,
  listen_host: '0.0.0.0',
  listen_port: 1883,
  username: '',
  password: '',
  signature_key: 'xiaozhi_ota_signature_key',
  enable_auth: false,
  tls: {
    enable: false,
    port: 8883,
    pem: '',
    key: ''
  }
})



const rules = {
  listen_host: [{ required: true, message: 'Please enter listen host address', trigger: 'blur' }],
  listen_port: [
    { required: true, message: 'Please enter listen port number', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: 'Port number must be between 1-65535', trigger: 'blur' }
  ],
  username: [{ required: true, message: 'Please enter admin username', trigger: 'blur' }],
  password: [{ required: true, message: 'Please enter admin password', trigger: 'blur' }],
  signature_key: [{ required: true, message: 'Please enter signature key', trigger: 'blur' }],
  'tls.port': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && (!value || value < 1 || value > 65535)) {
          callback(new Error('Port number must be between 1-65535 when TLS is enabled'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'tls.pem': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && !value) {
          callback(new Error('Certificate file path cannot be empty when TLS is enabled'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'tls.key': [
    {
      validator: (rule, value, callback) => {
        if (form.tls.enable && !value) {
          callback(new Error('Key file path cannot be empty when TLS is enabled'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadConfig = async () => {
  try {
    loading.value = true
    const response = await api.get('/admin/mqtt-server-configs')
    const configs = response.data.data || []
    if (configs.length > 0) {
      const config = configs[0]
      configId.value = config.id
      
      // Parse JSON configuration data
      try {
        const configData = JSON.parse(config.json_data || '{}')
        form.enable = configData.enable !== undefined ? configData.enable : true
        form.listen_host = configData.listen_host || '0.0.0.0'
        form.listen_port = Number(configData.listen_port) || 1883 // Ensure port is number type
        form.username = configData.username || ''
        form.password = configData.password || ''
        form.signature_key = configData.signature_key || 'xiaozhi_ota_signature_key'
        form.enable_auth = configData.enable_auth !== undefined ? configData.enable_auth : false
        
        if (configData.tls) {
          form.tls.enable = configData.tls.enable !== undefined ? configData.tls.enable : false
          form.tls.port = Number(configData.tls.port) || 8883 // Ensure TLS port is number type
          form.tls.pem = configData.tls.pem || ''
          form.tls.key = configData.tls.key || ''
        }
      } catch (error) {
        console.error('Failed to parse configuration JSON:', error)
        ElMessage.warning('Configuration format error, reset to default values')
      }
    }
  } catch (error) {
    ElMessage.error('Failed to load configuration: ' + error.message)
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
    saving.value = true
    
    // Clear related fields if TLS is disabled
    if (!form.tls.enable) {
      form.tls.pem = ''
      form.tls.key = ''
    }
    
    // Removed logic to clear username/password when auth is disabled, as admin username/password is independent of auth enablement
    
    const configData = {
      enable: form.enable,
      listen_host: form.listen_host,
      listen_port: Number(form.listen_port), // Ensure port is number type
      username: form.username,
      password: form.password,
      signature_key: form.signature_key,
      enable_auth: form.enable_auth,
      tls: {
        enable: form.tls.enable,
        port: Number(form.tls.port), // Ensure TLS port is number type
        pem: form.tls.pem,
        key: form.tls.key
      }
    }
    
    console.log('Saving configuration data:', configData) // Debug info
    console.log('Listen port value:', form.listen_port, 'Type:', typeof form.listen_port) // Debug port info
    
    const payload = {
      name: 'MQTT Server Configuration',
      config_id: 'mqtt_server_mqtt_server_config',
      provider: 'mqtt_server',
      json_data: JSON.stringify(configData),
      enabled: true,
      is_default: true
    }
    
    console.log('Sending payload:', payload) // Debug info
    
    if (configId.value) {
      const response = await api.put(`/admin/mqtt-server-configs/${configId.value}`, payload)
      console.log('Update response:', response) // Debug info
      ElMessage.success('Configuration updated successfully')
    } else {
      const response = await api.post('/admin/mqtt-server-configs', payload)
      console.log('Create response:', response) // Debug info
      configId.value = response.data.data.id
      ElMessage.success('Configuration created successfully')
    }
  } catch (error) {
    console.error('Save error:', error) // Debug info
    if (error.message) {
      ElMessage.error('Save failed: ' + error.message)
    }
  } finally {
    saving.value = false
  }
}



// Watch TLS switch state changes, clear related fields
watch(() => form.tls.enable, (enabled) => {
  if (!enabled) {
    // Clear certificate and key fields and reset validation when TLS is disabled
    form.tls.pem = ''
    form.tls.key = ''
    formRef.value?.clearValidate(['tls.pem', 'tls.key'])
  }
})

// Watch listen port changes, for debugging
watch(() => form.listen_port, (newValue) => {
  console.log('Listen port changed:', newValue, 'Type:', typeof newValue)
})

// Removed auth switch state listener, as admin username/password is independent of auth enablement

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.mqtt-server-config {
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

.basic-config {
  border-left: 4px solid #409eff;
}

.server-config {
  border-left: 4px solid #67c23a;
}

.auth-config {
  border-left: 4px solid #e6a23c;
}

.tls-config {
  border-left: 4px solid #f56c6c;
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

.server-icon {
  color: #67c23a;
}

.auth-icon {
  color: #e6a23c;
}

.tls-icon {
  color: #f56c6c;
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

/* Configuration Hint */
.config-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 24px;
  background: #f0f9ff;
  border-left: 4px solid #0ea5e9;
  margin-bottom: 16px;
}

.tip-icon {
  font-size: 16px;
  color: #0ea5e9;
  flex-shrink: 0;
}

.tip-text {
  font-size: 14px;
  color: #0369a1;
  line-height: 1.5;
}

/* Form Grid */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
  padding: 24px;
}

/* Basic Configuration Form Grid - Vertical Layout */
.basic-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* Authentication Configuration Form Grid */
.auth-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* TLS Configuration Form Grid */
.tls-form-grid {
  grid-template-columns: 1fr;
  gap: 20px;
}

/* Form Row - Horizontal Layout */
.form-row {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  flex-wrap: wrap;
}

.form-item {
  margin-bottom: 0;
}

.form-item-hint {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
  line-height: 1.5;
}

.form-item-with-help {
  display: flex;
  align-items: center;
  gap: 8px;
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

/* Action Button Area */
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
  .mqtt-server-config {
    padding: 16px;
  }
  
  .page-title {
    font-size: 20px;
  }
}
</style>
