<template>
  <div class="mqtt-config">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon">
            <Connection />
          </el-icon>
          <h1 class="page-title">MQTT Configuration Management</h1>
        </div>
      </div>
    </div>

    <!-- Configuration Description -->
    <div class="config-description">
      <el-alert
        title="Configuration Instructions"
        description="Configure MQTT connection parameters and authentication information. This configuration page is for the main program to connect to the MQTT server as an MQTT client, which can be the built-in MQTT server or an external EMQX."
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
          
          <div class="form-grid">
            <el-form-item label="Enable MQTT" prop="enable" class="form-item">
              <el-switch v-model="form.enable" />
            </el-form-item>
          </div>
        </el-card>

        <!-- Connection Configuration Card -->
        <el-card class="config-card connection-config" shadow="never">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon connection-icon">
                <Link />
              </el-icon>
              <span class="card-title">Connection Configuration</span>
            </div>
          </template>
          
          <div class="form-grid">
            <el-form-item label="Configuration Name" prop="name" class="form-item">
              <el-input v-model="form.name" placeholder="Please enter configuration name" />
            </el-form-item>
            
            <el-form-item label="Broker Address" prop="broker" class="form-item">
              <el-input v-model="form.broker" placeholder="Please enter MQTT Broker address" />
            </el-form-item>
            
            <el-form-item label="Connection Type" prop="type" class="form-item">
              <el-select v-model="form.type" placeholder="Please select connection type" style="width: 100%">
                <el-option label="TCP" value="tcp" />
                <el-option label="WebSocket" value="websocket" />
                <el-option label="SSL/TLS" value="ssl" />
              </el-select>
            </el-form-item>
            
            <el-form-item label="Port" prop="port" class="form-item">
              <el-input-number v-model="form.port" :min="1" :max="65535" placeholder="Please enter port number" style="width: 100%" />
            </el-form-item>
            
            <el-form-item label="Client ID" prop="client_id" class="form-item">
              <el-input v-model="form.client_id" placeholder="Please enter client ID" />
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
              <el-tooltip content="Username and password for connecting to MQTT server, needs to have subscribe permissions" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
          </template>
          
          <div class="form-grid">
            <el-form-item label="Username" prop="username" class="form-item">
              <el-input v-model="form.username" placeholder="Please enter username" />
            </el-form-item>
            
            <el-form-item label="Password" prop="password" class="form-item">
              <el-input v-model="form.password" type="password" placeholder="Please enter password" show-password />
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Setting, Link, User, QuestionFilled } from '@element-plus/icons-vue'
import api from '@/utils/api'

const loading = ref(false)
const saving = ref(false)
const configId = ref(null)
const formRef = ref()

const form = reactive({
  name: 'MQTT Configuration',
  is_default: true,
  enable: true,
  broker: '',
  type: 'tcp',
  port: 1883,
  client_id: '',
  username: '',
  password: ''
})

const generateConfig = () => {
  return JSON.stringify({
    enable: form.enable,
    broker: form.broker,
    type: form.type,
    port: form.port,
    client_id: form.client_id,
    username: form.username,
    password: form.password
  })
}

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  broker: [{ required: true, message: 'Please enter MQTT Broker address', trigger: 'blur' }],
  type: [{ required: true, message: 'Please select connection type', trigger: 'change' }],
  port: [
    { required: true, message: 'Please enter port number', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: 'Port number must be between 1-65535', trigger: 'blur' }
  ],
  client_id: [{ required: true, message: 'Please enter client ID', trigger: 'blur' }]
}

const loadConfig = async () => {
  loading.value = true
  try {
    console.log('Starting to load MQTT configuration...')
    const response = await api.get('/admin/mqtt-configs')
    console.log('MQTT configuration API response:', response)
    const configs = response.data.data || []
    console.log('Parsed configuration list:', configs)
    
    // If there is a configuration, load the first one
    if (configs.length > 0) {
      const config = configs[0]
      console.log('Loading configuration:', config)
      configId.value = config.id
      form.name = config.name
      form.is_default = config.is_default
      
      try {
        const configData = JSON.parse(config.json_data || '{}')
        console.log('Parsed configuration data:', configData)
        form.enable = configData.enable || true
        form.broker = configData.broker || ''
        form.type = configData.type || 'tcp'
        form.port = configData.port || 1883
        form.client_id = configData.client_id || ''
        form.username = configData.username || ''
        form.password = configData.password || ''
      } catch (error) {
        console.error('Failed to parse configuration:', error)
        ElMessage.warning('Configuration format error, reset to default values')
      }
    } else {
      console.log('No configuration found, using default values')
    }
  } catch (error) {
    console.error('Failed to load configuration:', error)
    ElMessage.error('Failed to load configuration')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        // Generate config_id, format is "type_name"
        const generatedConfigId = `mqtt_${form.name.replace(/[^a-zA-Z0-9]/g, '_').toLowerCase()}`

        let configData, isUpdate = false

        // If it's an update operation, first get existing configuration, only update enable field, keep other configurations
        if (configId.value) {
          const response = await api.get('/admin/mqtt-configs')
          const configs = response.data.data || []
          const existingConfig = configs.find(c => c.id === configId.value)

          if (existingConfig) {
            // Parse existing configuration, keep other fields, only update enable
            const existingData = JSON.parse(existingConfig.json_data || '{}')
            existingData.enable = form.enable

            // Also update other fields (if form has values, use form values)
            if (form.broker) existingData.broker = form.broker
            if (form.type) existingData.type = form.type
            if (form.port) existingData.port = form.port
            if (form.client_id) existingData.client_id = form.client_id
            if (form.username) existingData.username = form.username
            if (form.password) existingData.password = form.password

            configData = {
              name: form.name,
              config_id: generatedConfigId,
              is_default: true,
              json_data: JSON.stringify(existingData)
            }
            isUpdate = true
          } else {
            // Configuration does not exist, create new
            configData = {
              name: form.name,
              config_id: generatedConfigId,
              is_default: true,
              json_data: generateConfig()
            }
          }
        } else {
          // Create new configuration, use complete form data
          configData = {
            name: form.name,
            config_id: generatedConfigId,
            is_default: true,
            json_data: generateConfig()
          }
        }

        if (isUpdate) {
          // Update existing configuration
          await api.put(`/admin/mqtt-configs/${configId.value}`, configData)
          ElMessage.success('Update successful')
        } else {
          // Create new configuration
          const response = await api.post('/admin/mqtt-configs', configData)
          configId.value = response.data.data.id
          ElMessage.success('Save successful')
        }
      } catch (error) {
        ElMessage.error(error.response?.data?.message || 'Save failed')
      } finally {
        saving.value = false
      }
    }
  })
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.mqtt-config {
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

.connection-config {
  border-left: 4px solid #67c23a;
}

.auth-config {
  border-left: 4px solid #e6a23c;
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

.connection-icon {
  color: #67c23a;
}

.auth-icon {
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
  .mqtt-config {
    padding: 16px;
  }
  
  .page-title {
    font-size: 24px;
  }
  
  .title-icon {
    font-size: 28px;
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 16px;
    padding: 16px;
  }
}

@media (max-width: 480px) {
  .title-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .page-title {
    font-size: 20px;
  }
}
</style>
