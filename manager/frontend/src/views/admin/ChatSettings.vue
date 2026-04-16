<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>Chat Settings</h2>
      </div>
      <div class="header-right">
        <el-button @click="loadSettings" :loading="loading">Refresh</el-button>
        <el-button type="primary" @click="saveSettings" :loading="saving">Save Settings</el-button>
      </div>
    </div>

    <el-card v-loading="loading">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="180px" style="max-width: 720px;">
        <el-divider content-position="left">Authentication</el-divider>
        <el-form-item label="Enable Device Activation Verification" prop="auth.enable">
          <el-switch v-model="form.auth.enable" />
        </el-form-item>

        <el-divider content-position="left">Chat Parameters</el-divider>
        <el-form-item label="Session Max Idle Duration (ms)" prop="chat.max_idle_duration">
          <el-input-number v-model="form.chat.max_idle_duration" :min="0" :step="1000" style="width: 100%;" />
          <div class="form-help">
            Unit: milliseconds. Set to 0 for unlimited session idle duration (will not auto-disconnect due to idle). Recommended: 30000~120000.
          </div>
        </el-form-item>
        <el-form-item label="Sentence End Silence Threshold (ms)" prop="chat.chat_max_silence_duration">
          <el-input-number v-model="form.chat.chat_max_silence_duration" :min="0" :step="10" style="width: 100%;" />
          <div class="form-help">
            Used to determine sentence end: when silence persists for this threshold after "voice" transitions to "silence", the sentence is considered ended and subsequent processing is triggered. Default 400ms. Lower threshold means faster response but more prone to truncation; higher threshold is more stable but slower response. Recommended: 300~600ms.
          </div>
        </el-form-item>
        <el-form-item label="Real-time Interruption Mode" prop="chat.realtime_mode">
          <el-select v-model="form.chat.realtime_mode" style="width: 100%;">
            <el-option :value="1" label="1 - VAD Interruption Mode" />
            <el-option :value="2" label="2 - ASR Interruption Mode" />
            <el-option :value="3" label="3 - ASR Interruption on Voiceprint Detection" />
            <el-option :value="4" label="4 - ASR Result Interruption" />
          </el-select>
        </el-form-item>
        <el-form-item label="Global System Prompt Description" prop="chat.global_system_prompt">
          <el-input
            v-model="form.chat.global_system_prompt"
            type="textarea"
            :rows="6"
            maxlength="8000"
            show-word-limit
            placeholder="This content will be prepended to the system prompt. It is recommended to fill in platform-level constraints and identity settings."
          />
          <div class="form-help">
            Effect order: Global System Prompt Description → Role/Device Prompt → Time/Memory and other runtime information.
          </div>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../../utils/api'

const loading = ref(false)
const saving = ref(false)
const formRef = ref()

const form = reactive({
  auth: {
    enable: false
  },
  chat: {
    max_idle_duration: 30000,
    chat_max_silence_duration: 400,
    realtime_mode: 4,
    global_system_prompt: ''
  }
})

const rules = {
  'chat.max_idle_duration': [
    { required: true, message: 'Please enter session max idle duration', trigger: 'blur' }
  ],
  'chat.chat_max_silence_duration': [
    { required: true, message: 'Please enter sentence end silence threshold', trigger: 'blur' }
  ],
  'chat.realtime_mode': [
    { required: true, message: 'Please select real-time interruption mode', trigger: 'change' }
  ],
  'chat.global_system_prompt': [
    { max: 8000, message: 'Global System Prompt description cannot exceed 8000 characters', trigger: 'blur' }
  ]
}

const loadSettings = async () => {
  loading.value = true
  try {
    const res = await api.get('/admin/chat-settings')
    const data = res.data?.data || {}
    form.auth.enable = !!data.auth?.enable
    form.chat.max_idle_duration = Number(data.chat?.max_idle_duration ?? 30000)
    form.chat.chat_max_silence_duration = Number(data.chat?.chat_max_silence_duration ?? 400)
    form.chat.realtime_mode = Number(data.chat?.realtime_mode ?? 4)
    form.chat.global_system_prompt = String(data.chat?.global_system_prompt ?? '')
  } catch (error) {
    ElMessage.error('Failed to load chat settings')
    console.error(error)
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await api.put('/admin/chat-settings', {
      auth: {
        enable: !!form.auth.enable
      },
      chat: {
        max_idle_duration: Number(form.chat.max_idle_duration),
        chat_max_silence_duration: Number(form.chat.chat_max_silence_duration),
        realtime_mode: Number(form.chat.realtime_mode),
        global_system_prompt: String(form.chat.global_system_prompt || '')
      }
    })
    ElMessage.success('Chat settings saved successfully')
  } catch (error) {
    ElMessage.error('Failed to save chat settings')
    console.error(error)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadSettings()
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

.header-right {
  display: flex;
  gap: 8px;
}

.form-help {
  margin-top: 6px;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}
</style>