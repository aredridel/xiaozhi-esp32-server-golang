<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>Voiceprint Recognition Configuration</h2>
      </div>
    </div>

    <el-card v-loading="loading" class="config-card">
      <el-alert
        title="Tip"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 20px;"
      >
        <template #default>
          If deployed in a docker-compose environment, the API address will be read from environment variables, no configuration needed
        </template>
      </el-alert>
      
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="Service Address" prop="base_url">
          <el-input 
            v-model="form.base_url" 
            placeholder="Please enter HTTP service address, e.g.: http://192.168.208.214:8080"
            style="width: 100%"
          />
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            Please enter HTTP address, the system will automatically convert it to WebSocket address
          </div>
        </el-form-item>
        
        <el-form-item label="Recognition Threshold" prop="threshold">
          <el-input-number 
            v-model="form.threshold" 
            :min="0" 
            :max="1" 
            :step="0.1" 
            :precision="2"
            placeholder="0.4"
            style="width: 100%"
          />
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            Voiceprint recognition threshold, range 0.0-1.0, default 0.4. Higher values mean stricter recognition
          </div>
        </el-form-item>
        
        <el-form-item label="Enable Status">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      
      <div class="form-actions">
        <el-button type="primary" @click="handleSave" :loading="saving">
          Save Configuration
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const formRef = ref()
const currentConfig = ref(null)

const form = reactive({
  base_url: 'http://192.168.208.214:8080',
  threshold: 0.4,
  enabled: true
})

const rules = {
  base_url: [
    { required: true, message: 'Please enter service address', trigger: 'blur' },
    { 
      pattern: /^https?:\/\/.+/, 
      message: 'Please enter a valid HTTP address, e.g.: http://192.168.208.214:8080', 
      trigger: 'blur' 
    }
  ],
  threshold: [
    { required: true, message: 'Please enter recognition threshold', trigger: 'blur' },
    { 
      type: 'number', 
      min: 0, 
      max: 1, 
      message: 'Threshold must be between 0.0 and 1.0', 
      trigger: 'blur' 
    }
  ]
}

const loadConfig = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/speaker-configs')
    const configs = response.data.data || []
    
    if (configs.length > 0) {
      // If there is a configuration, use the first one (should only be one)
      currentConfig.value = configs[0]
      const configObj = JSON.parse(configs[0].json_data || '{}')
      
      // Parse configuration
      if (configObj.service && configObj.service.base_url) {
        form.base_url = configObj.service.base_url
      } else if (configObj.base_url) {
        // Compatible with old format
        form.base_url = configObj.base_url
      }
      // Read threshold configuration
      if (configObj.service && configObj.service.threshold !== undefined) {
        form.threshold = configObj.service.threshold
      } else if (configObj.threshold !== undefined) {
        // Compatible with old format
        form.threshold = configObj.threshold
      } else {
        // Default value
        form.threshold = 0.4
      }
      // Switch corresponds to json_data.enable (business enable), do not use the enabled column returned by the API
      form.enabled = configObj.enable !== undefined ? configObj.enable : true
    }
  } catch (error) {
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
        // Build configuration data: switch writes to json_data.enable, external output uses this field
        const configData = {
          service: {
            base_url: form.base_url,
            threshold: form.threshold
          },
          enable: form.enabled
        }
        
        const saveData = {
          name: 'Voiceprint Recognition Configuration',
          config_id: 'asr_server',
          provider: 'asr_server',
          is_default: true,
          enabled: form.enabled,
          json_data: JSON.stringify(configData)
        }
        
        if (currentConfig.value) {
          // Update existing configuration
          await api.put(`/admin/speaker-configs/${currentConfig.value.id}`, saveData)
          ElMessage.success('Configuration updated successfully')
        } else {
          // Create new configuration
          await api.post('/admin/speaker-configs', saveData)
          ElMessage.success('Configuration created successfully')
        }
        
        // Reload configuration
        await loadConfig()
      } catch (error) {
        ElMessage.error('Save failed: ' + (error.response?.data?.message || error.message))
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

.config-card {
  max-width: 800px;
}

.form-tip {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 4px;
}

.form-tip .el-icon {
  font-size: 14px;
  color: #409eff;
}

.form-actions {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #ebeef5;
}
</style>