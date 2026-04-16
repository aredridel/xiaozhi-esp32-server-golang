<template>
  <div class="ota-config">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <div class="title-section">
          <el-icon class="title-icon"><Setting /></el-icon>
          <h1 class="page-title">OTA Configuration Management</h1>
        </div>
      </div>
    </div>

    <!-- Configuration Description -->
    <div class="config-description">
      <el-alert
        title="Configuration Instructions"
        description="Configure OTA upgrade related parameters, including Test and External environment settings. WebSocket configuration refers to the websocket address issued to the terminal, MQTT configuration refers to the mqtt connection issued to the terminal (requires enabling mqtt server and udp server), firmware defaults to mqtt priority"
        type="info"
        :closable="false"
        show-icon
      />
    </div>

    <!-- Configuration Form -->
    <div class="form-container">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="140px"
        class="config-form"
        label-position="left"
      >
        <!-- Basic Configuration Card -->
        <el-card class="config-card basic-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Tools /></el-icon>
              <span class="card-title">Basic Configuration</span>
            </div>
          </template>
          
          <el-form-item label="Signature Key" prop="signature_key" class="form-item full-width">
            <el-input 
              v-model="form.signature_key" 
              placeholder="Please enter signature key"
              size="large"
              :prefix-icon="Key"
              show-password
            />
            <div class="form-item-hint">
              Used to generate username and password for connecting to mqtt server, must match the 'Signature Key' in the mqtt server configuration page
            </div>
          </el-form-item>
        </el-card>
        
        <!-- Test Environment Configuration Card -->
        <el-card class="config-card test-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon test-icon"><Monitor /></el-icon>
              <span class="card-title">Test Environment Configuration</span>
              <el-tag type="warning" size="small">Test Environment</el-tag>
            </div>
          </template>
          
          <!-- WebSocket Configuration -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Connection /></el-icon>
              <span>WebSocket Configuration</span>
              <el-tooltip content="Websocket address issued to the terminal" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="WebSocket URL" prop="test.websocket.url" class="form-item full-width">
                 <el-input 
                   v-model="form.test.websocket.url" 
                   placeholder="e.g.: ws://host:port/xiaozhi/v1/"
                   size="large"
                   :prefix-icon="Link"
                 />
               </el-form-item>
            </div>
          </div>
          
          <!-- MQTT Configuration -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Message /></el-icon>
              <span>MQTT Configuration</span>
              <el-tooltip content="MQTT connection issued to the terminal (requires enabling mqtt server and udp server), firmware defaults to mqtt priority" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="MQTT Enabled" class="form-item">
                <el-switch 
                  v-model="form.test.mqtt.enable" 
                  size="large"
                  active-text="Enabled"
                  inactive-text="Disabled"
                />
              </el-form-item>
               
              <el-form-item label="MQTT Endpoint" prop="test.mqtt.endpoint" class="form-item" v-if="form.test.mqtt.enable">
                <el-input 
                  v-model="form.test.mqtt.endpoint" 
                  placeholder="Please enter Test environment MQTT endpoint, format: ip:port"
                  size="large"
                  :prefix-icon="Link"
                />
              </el-form-item>
            </div>
          </div>
          <div class="card-actions">
            <el-button type="warning" size="large" :loading="otaTestingTest" @click="testOtaEnv('test')" class="env-test-btn">
              <el-icon><CircleCheck /></el-icon>
              Test Test Environment
            </el-button>
          </div>
        </el-card>
        
        <!-- External Environment Configuration Card -->
        <el-card class="config-card external-config" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon external-icon"><Platform /></el-icon>
              <span class="card-title">External Environment Configuration</span>
              <el-tag type="success" size="small">Production Environment</el-tag>
            </div>
          </template>
          
          <!-- WebSocket Configuration -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Connection /></el-icon>
              <span>WebSocket Configuration</span>
              <el-tooltip content="Websocket address issued to the terminal" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="WebSocket URL" prop="external.websocket.url" class="form-item full-width">
                 <el-input 
                   v-model="form.external.websocket.url" 
                   placeholder="e.g.: ws://host:port/xiaozhi/v1/"
                   size="large"
                   :prefix-icon="Link"
                 />
               </el-form-item>
            </div>
          </div>
          
          <!-- MQTT Configuration -->
          <div class="config-section">
            <div class="section-title">
              <el-icon><Message /></el-icon>
              <span>MQTT Configuration</span>
              <el-tooltip content="MQTT connection issued to the terminal (requires enabling mqtt server and udp server), firmware defaults to mqtt priority" placement="top">
                <el-icon class="help-icon"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
            <div class="form-grid">
              <el-form-item label="MQTT Enabled" class="form-item">
                <el-switch 
                  v-model="form.external.mqtt.enable" 
                  size="large"
                  active-text="Enabled"
                  inactive-text="Disabled"
                />
              </el-form-item>
               
              <el-form-item label="MQTT Endpoint" prop="external.mqtt.endpoint" class="form-item" v-if="form.external.mqtt.enable">
                <el-input 
                  v-model="form.external.mqtt.endpoint" 
                  placeholder="Please enter External environment MQTT endpoint, format: ip:port"
                  size="large"
                  :prefix-icon="Link"
                />
              </el-form-item>
            </div>
          </div>
          <div class="card-actions">
            <el-button type="warning" size="large" :loading="otaTestingExternal" @click="testOtaEnv('external')" class="env-test-btn">
              <el-icon><CircleCheck /></el-icon>
              Test External Environment
            </el-button>
          </div>
        </el-card>
        
        <!-- Action Buttons -->
        <div class="action-section">
          <el-button 
            type="primary" 
            @click="saveConfig" 
             :loading="saving"
            size="large"
            class="save-button"
          >
            <el-icon><Check /></el-icon>
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
import { 
  Setting, Tools, Monitor, Platform, Connection, Message, 
  Edit, Key, Link, User, Lock, Check, QuestionFilled, CircleCheck 
} from '@element-plus/icons-vue'
import api from '@/utils/api'
import { testWithData } from '@/utils/configTest'

const loading = ref(false)
const saving = ref(false)
const otaTestingTest = ref(false)
const otaTestingExternal = ref(false)
const configId = ref(null)
const formRef = ref()

const form = reactive({
  signature_key: 'xiaozhi_ota_signature_key',
  test: {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: true,
      endpoint: '127.0.0.1:1883'
    }
  },
  external: {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: false,
      endpoint: '127.0.0.1:1883'
    }
  }
})

const generateConfig = () => {
  return JSON.stringify({
    signature_key: form.signature_key,
    test: {
      websocket: {
        url: form.test.websocket.url
      },
      mqtt: {
        enable: form.test.mqtt.enable,
        endpoint: form.test.mqtt.endpoint
      }
    },
    external: {
      websocket: {
        url: form.external.websocket.url
      },
      mqtt: {
        enable: form.external.mqtt.enable,
        endpoint: form.external.mqtt.endpoint
      }
    }
  }, null, 2)
}

const rules = {
  signature_key: [
    { required: true, message: 'Please enter signature key', trigger: 'blur' }
  ],
  'test.websocket.url': [
    { required: true, message: 'Please enter Test environment WebSocket URL', trigger: 'blur' }
  ],
  'test.mqtt.endpoint': [
    {
      validator: (rule, value, callback) => {
        if (form.test.mqtt.enable && !value) {
          callback(new Error('Endpoint cannot be empty when MQTT is enabled'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  'external.websocket.url': [
    { required: true, message: 'Please enter External environment WebSocket URL', trigger: 'blur' }
  ],
  'external.mqtt.endpoint': [
    {
      validator: (rule, value, callback) => {
        if (form.external.mqtt.enable && !value) {
          callback(new Error('Endpoint cannot be empty when MQTT is enabled'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadConfig = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/ota-configs')
    const configs = response.data.data || []
    
    if (configs.length > 0) {
      const config = configs[0]
      configId.value = config.id
      
      try {
        const configData = JSON.parse(config.json_data || '{}')
        form.signature_key = configData.signature_key || 'xiaozhi_ota_signature_key'
        
        // Test environment configuration
        if (configData.test) {
          form.test.websocket.url = configData.test.websocket?.url || 'ws://127.0.0.1:8989/xiaozhi/v1/'
          form.test.mqtt.enable = configData.test.mqtt?.enable !== undefined ? configData.test.mqtt.enable : true
          form.test.mqtt.endpoint = configData.test.mqtt?.endpoint || '127.0.0.1:1883'
        }
        
        // External environment configuration
        if (configData.external) {
          form.external.websocket.url = configData.external.websocket?.url || 'ws://127.0.0.1:8989/xiaozhi/v1/'
          form.external.mqtt.enable = configData.external.mqtt?.enable !== undefined ? configData.external.mqtt.enable : false
          form.external.mqtt.endpoint = configData.external.mqtt?.endpoint || '127.0.0.1:1883'
        }
      } catch (error) {
        console.error('Failed to parse configuration:', error)
        ElMessage.error('Configuration format error')
      }
    }
  } catch (error) {
    ElMessage.error('Failed to load configuration')
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  if (!formRef.value) return
  
  try {
    await formRef.value.validate()
    saving.value = true
    
    // Clear endpoint values if MQTT is disabled
    if (!form.test.mqtt.enable) {
      form.test.mqtt.endpoint = ''
    }
    if (!form.external.mqtt.enable) {
      form.external.mqtt.endpoint = ''
    }
    
    const configData = {
      name: 'OTA Configuration',
      config_id: 'ota_ota_config',
      provider: form.provider || 'default',
      json_data: generateConfig(),
      enabled: true,
      is_default: true
    }
    
    if (configId.value) {
      await api.put(`/admin/ota-configs/${configId.value}`, configData)
      ElMessage.success('Configuration updated successfully')
    } else {
      const response = await api.post('/admin/ota-configs', configData)
      configId.value = response.data.data.id
      ElMessage.success('Configuration created successfully')
    }
  } catch (error) {
    if (error.message) {
      ElMessage.error('Save failed: ' + error.message)
    }
  } finally {
    saving.value = false
  }
}

// env: 'test' | 'external', test corresponding environment's WebSocket and MQTT UDP (if enabled)
const testOtaEnv = async (env) => {
  const envConfig = env === 'test' ? form.test : form.external
  const mqttEnabled = envConfig.mqtt.enable

  const payload = {
    signature_key: form.signature_key,
    test: {
      websocket: { url: env === 'test' ? form.test.websocket.url : '' },
      mqtt: { enable: form.test.mqtt.enable, endpoint: form.test.mqtt.endpoint }
    },
    external: {
      websocket: { url: env === 'external' ? form.external.websocket.url : '' },
      mqtt: { enable: form.external.mqtt.enable, endpoint: form.external.mqtt.endpoint }
    }
  }
  const loadingRef = env === 'test' ? otaTestingTest : otaTestingExternal
  loadingRef.value = true
  try {
    // Directly call API to get raw response, including complete websocket and mqtt_udp results
    const body = { types: ['ota'], data: { ota: { ota_ota_config: payload } } }
    const res = await api.post('/admin/configs/test', body, { timeout: 30000 })
    const data = res.data?.data ?? res.data
    const otaResult = data?.ota?.ota_ota_config

    const label = env === 'test' ? 'Test Environment' : 'External Environment'

    if (!otaResult) {
      ElMessage.error(`${label}: No test result returned`)
      return
    }

    // Parse WebSocket result
    const wsResult = otaResult.websocket || {}
    const wsOk = wsResult.ok || false
    const wsMsg = wsResult.message || 'WebSocket test failed'
    const wsMs = wsResult.first_packet_ms

    // Parse MQTT UDP result
    const mqttResult = otaResult.mqtt_udp
    let mqttOk = true
    let mqttMsg = ''
    let mqttMs = 0

    if (mqttEnabled && mqttResult) {
      mqttOk = mqttResult.ok || false
      mqttMsg = mqttResult.message || 'MQTT UDP test failed'
      mqttMs = mqttResult.first_packet_ms || 0
    } else if (mqttEnabled) {
      mqttOk = false
      mqttMsg = 'MQTT UDP result not returned'
    }

    // Build result display
    let message = ''
    if (wsOk) {
      message += `WebSocket: ${wsMsg}`
      if (wsMs != null) message += ` (${wsMs}ms)`
    } else {
      message += `WebSocket: ${wsMsg}`
    }

    if (mqttEnabled) {
      message += ' | '
      if (mqttOk) {
        message += `MQTT UDP: ${mqttMsg}`
        if (mqttMs != null) message += ` (${mqttMs}ms)`
      } else {
        message += `MQTT UDP: ${mqttMsg}`
      }
    }

    if (wsOk && (!mqttEnabled || mqttOk)) {
      ElMessage.success(`${label}: ${message}`)
    } else {
      ElMessage.warning(`${label}: ${message}`)
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Test request failed')
  } finally {
    loadingRef.value = false
  }
}

// Watch provider changes, reset form to defaults
watch(() => form.provider, (newProvider) => {
  if (newProvider) {
    // Reset form to defaults
    form.signature_key = 'your_signature_key_here'
    form.test = {
      websocket: {
        url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
      },
      mqtt: {
        enable: false,
        endpoint: '127.0.0.1:1883'
      }
    }
    form.external = {
      websocket: {
        url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
      },
      mqtt: {
        enable: true,
        endpoint: '127.0.0.1:1883'
      }
    }
  }
})

// Watch MQTT switch state changes, reset related validation
watch(() => form.test.mqtt.enable, (enabled) => {
  if (!enabled) {
    // Clear endpoint and reset validation when MQTT is disabled
    form.test.mqtt.endpoint = ''
    formRef.value?.clearValidate('test.mqtt.endpoint')
  }
})

watch(() => form.external.mqtt.enable, (enabled) => {
  if (!enabled) {
    // Clear endpoint and reset validation when MQTT is disabled
    form.external.mqtt.endpoint = ''
    formRef.value?.clearValidate('external.mqtt.endpoint')
  }
})

const resetForm = () => {
  editingConfig.value = null
  form.provider = ''
  form.signature_key = 'your_signature_key_here'
  form.test = {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: false,
      endpoint: '127.0.0.1:1883'
    }
  }
  form.external = {
    websocket: {
      url: 'ws://127.0.0.1:8989/xiaozhi/v1/'
    },
    mqtt: {
      enable: true,
      endpoint: '127.0.0.1:1883'
    }
  }
  
  // Clear form validation state
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.ota-config {
  min-height: 100vh;
  background: #f8fafc;
  padding: 0;
}

/* Page Header Area */
.page-header {
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  padding: 2rem 0;
  margin-bottom: 2rem;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 2rem;
}

.title-section {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.5rem;
}

.title-icon {
  font-size: 2rem;
  color: #667eea;
}

.page-title {
  font-size: 2.5rem;
  font-weight: 700;
  color: #1f2937;
  margin: 0;
}

/* Configuration Description */
.config-description {
  max-width: 1200px;
  margin: 0 auto 2rem;
  padding: 0 2rem;
}

/* Form Container */
.form-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 2rem 2rem;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

/* Configuration Card */
.config-card {
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  overflow: hidden;
}

.config-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.15);
}

.config-card.basic-config {
  border-left: 4px solid #3b82f6;
}

.config-card.test-config {
  border-left: 4px solid #f59e0b;
}

.config-card.external-config {
  border-left: 4px solid #10b981;
}

/* Card Header */
.card-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-weight: 600;
  font-size: 1.1rem;
  color: #1f2937;
}

.card-icon {
  font-size: 1.25rem;
}

.card-icon.test-icon {
  color: #f59e0b;
}

.card-icon.external-icon {
  color: #10b981;
}

.card-title {
  flex: 1;
}

/* Form Grid Layout */
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1.5rem;
  margin-bottom: 1.5rem;
}

.form-item.full-width {
  grid-column: 1 / -1;
}

/* Configuration Section */
.config-section {
  margin-bottom: 2rem;
}

.config-section:last-child {
  margin-bottom: 0;
}

.card-actions {
  margin-top: 1.25rem;
  padding-top: 1.25rem;
  border-top: 1px solid #eee;
}

.env-test-btn {
  font-size: 1rem;
  padding: 12px 24px;
  min-width: 160px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 2px solid #e5e7eb;
}

.section-title .el-icon {
  color: #6366f1;
}

.help-icon {
  color: #9ca3af;
  cursor: help;
  font-size: 0.875rem;
}

.help-icon:hover {
  color: #6366f1;
}

/* Form Item Styles */
.form-item {
  margin-bottom: 0;
}

.form-item-hint {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
  line-height: 1.5;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
  line-height: 1.5;
}

:deep(.el-input) {
  border-radius: 8px;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.25);
}

:deep(.el-input-number) {
  width: 100%;
}

:deep(.el-input-number .el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-switch) {
  --el-switch-on-color: #10b981;
  --el-switch-off-color: #d1d5db;
}

/* Action Button Area */
.action-section {
  display: flex;
  justify-content: center;
  padding: 2rem 0;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.save-button {
  padding: 12px 32px;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 8px;
  background: #3b82f6;
  border: none;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.2);
  transition: all 0.3s ease;
}

.save-button:hover {
  background: #2563eb;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.save-button:active {
  transform: translateY(0);
}

/* Responsive Design */
@media (max-width: 1024px) {
  .page-title {
    font-size: 2.2rem;
  }
}

@media (max-width: 768px) {
  .header-content {
    padding: 0 1rem;
  }
  
  .form-container {
    padding: 0 1rem 1rem;
  }
  
  .page-title {
    font-size: 1.6rem;
    max-width: calc(100vw - 5rem);
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
  
  .config-form {
    gap: 1.5rem;
  }
}

@media (max-width: 600px) {
  .title-section {
    gap: 0.75rem;
  }
  
  .page-title {
    font-size: 1.6rem;
    max-width: calc(100vw - 5rem);
  }
  
  .form-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
}

@media (max-width: 480px) {
  .title-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }
  
  .page-title {
    font-size: 1.5rem;
    max-width: 100%;
    white-space: normal;
    word-break: keep-all;
    overflow-wrap: break-word;
  }
}
</style>
