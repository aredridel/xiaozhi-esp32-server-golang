<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>TTS Configuration Management</h2>
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
            @click="testConfig(scope.row, 'tts')"
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
      :title="editingConfig ? 'Edit TTS Configuration' : 'Add TTS Configuration'"
      width="600px"
      @close="handleDialogClose"
    >
      <TTSConfigForm
        ref="formRef"
        :model="form"
        :rules="rules"
        :voice-options="voiceOptions"
        :voice-loading="voiceLoading"
        @request-voice-options="handleVoiceOptionsRequest"
      />
      
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
import { ref, reactive, onMounted, computed, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest'
import TTSConfigForm from './forms/TTSConfigForm.vue'
import { TTS_PROVIDERS_WITH_VOICES } from './forms/ttsProviderOptions'

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

// Voice options related
const voiceOptions = ref([])
const voiceLoading = ref(false)

const form = reactive({
  name: '',
  config_id: '',
  provider: 'doubao_ws',
  is_default: false,
  enabled: true,
  double_stream: false,
  cosyvoice: {
    api_url: 'https://tts.linkerai.cn/tts',
    spk_id: 'spk_id',
    frame_duration: 60,
    target_sr: 24000,
    audio_format: 'mp3',
    instruct_text: 'Hello'
  },
  qwen_tts: {
    api_key: '',
    api_url: 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation',
    region: 'beijing',
    model: 'qwen3-tts-flash',
    voice: 'Cherry',
    language_type: 'Chinese',
    stream: true,
    frame_duration: 60
  },
  doubao: {
    appid: '6886011847',
    access_token: 'access_token',
    model: 'seed-tts-2.0-standard',
    voice: 'BV001_streaming',
    api_url: 'https://openspeech.bytedance.com/api/v3/tts/unidirectional'
  },
  doubao_ws: {
    appid: '6886011847',
    access_token: 'access_token',
    model: 'seed-tts-2.0-standard',
    resource_id: '',
    voice: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream'
  },
  edge: {
    voice: 'zh-CN-XiaoxiaoNeural',
    rate: '+0%',
    volume: '+0%',
    pitch: '+0Hz',
    connect_timeout: 10,
    receive_timeout: 60
  },
  edge_offline: {
    server_url: 'ws://localhost:8080/tts',
    timeout: 30,
    sample_rate: 16000,
    channels: 1,
    frame_duration: 20
  },
  openai: {
    api_key: '',
    api_url: 'https://api.openai.com/v1/audio/speech',
    model: 'tts-1',
    voice: 'alloy',
    response_format: 'mp3',
    speed: 1.0,
    stream: true,
    frame_duration: 60
  },
  xunfei: {
    app_id: '',
    api_key: '',
    api_secret: '',
    ws_url: 'wss://tts-api.xfyun.cn/v2/tts',
    voice: 'xiaoyan',
    audio_encoding: 'raw',
    sample_rate: 16000,
    speed: 50,
    volume: 50,
    pitch: 50,
    tte: 'UTF8',
    reg: 0,
    rdn: 0,
    frame_duration: 60,
    connect_timeout: 10,
    read_timeout: 30
  },
  xunfei_super_tts: {
    app_id: '',
    api_key: '',
    api_secret: '',
    ws_url: 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6',
    voice: 'x6_lingxiaoxue_pro',
    audio_encoding: 'raw',
    sample_rate: 24000,
    speed: 50,
    volume: 50,
    pitch: 50,
    bgs: 0,
    reg: 0,
    rdn: 0,
    rhy: 0,
    oral_level: 'mid',
    spark_assist: 1,
    stop_split: 0,
    remain: 0,
    frame_duration: 60,
    connect_timeout: 10,
    read_timeout: 30
  },
  indextts_vllm: {
    api_url: 'http://127.0.0.1:7860',
    api_key: '',
    model: 'indextts-vllm',
    voice: '',
    frame_duration: 60
  },
  zhipu: {
    api_key: '',
    api_url: 'https://open.bigmodel.cn/api/paas/v4/audio/speech',
    model: 'glm-tts',
    voice: 'tongtong',
    response_format: 'pcm',
    speed: 1.0,
    volume: 1.0,
    stream: true,
    encode_format: 'base64',
    frame_duration: 60
  },
  minimax: {
    api_key: '',
    model: 'speech-2.8-hd',
    voice: 'male-qn-qingse',
    speed: 1.0,
    vol: 1.0,
    pitch: 0,
    sample_rate: 32000,
    bitrate: 128000,
    format: 'mp3',
    channel: 1
  }
})

const rules = {
  name: [{ required: true, message: 'Please enter configuration name', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Please enter config ID', trigger: 'blur' }],
  provider: [{ required: true, message: 'Please select provider', trigger: 'change' }],
  // CosyVoice validation rules
  'cosyvoice.api_url': [{ required: true, message: 'Please enter API URL', trigger: 'blur' }],
  'cosyvoice.spk_id': [{ required: true, message: 'Please enter speaker ID', trigger: 'blur' }],
  // Doubao TTS validation rules
  'doubao.appid': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
  'doubao.access_token': [{ required: true, message: 'Please enter access token', trigger: 'blur' }],
  'doubao.model': [{ required: true, message: 'Please select model', trigger: 'change' }],
  'doubao.voice': [{ required: true, message: 'Please enter voice', trigger: 'blur' }],
  'doubao.api_url': [{ required: true, message: 'Please enter API URL', trigger: 'blur' }],
  // Doubao WebSocket validation rules
  'doubao_ws.appid': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
  'doubao_ws.access_token': [{ required: true, message: 'Please enter access token', trigger: 'blur' }],
  'doubao_ws.model': [{ required: true, message: 'Please select model', trigger: 'change' }],
  'doubao_ws.voice': [{ required: true, message: 'Please enter voice', trigger: 'blur' }],
  'doubao_ws.ws_url': [{ required: true, message: 'Please enter WebSocket URL', trigger: 'blur' }],
  // Edge TTS validation rules
  'edge.voice': [{ required: true, message: 'Please enter voice', trigger: 'blur' }],
  'edge.rate': [{ required: true, message: 'Please enter rate', trigger: 'blur' }],
  'edge.volume': [{ required: true, message: 'Please enter volume', trigger: 'blur' }],
  // Edge Offline validation rules
  'edge_offline.server_url': [{ required: true, message: 'Please enter server URL', trigger: 'blur' }],
  // OpenAI TTS validation rules
  'openai.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  // Xunfei TTS validation rules
  'xunfei.app_id': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
  'xunfei.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  'xunfei.api_secret': [{ required: true, message: 'Please enter API Secret', trigger: 'blur' }],
  'xunfei.ws_url': [{ required: true, message: 'Please enter WebSocket URL', trigger: 'blur' }],
  'xunfei.voice': [{ required: true, message: 'Please enter voice', trigger: 'blur' }],
  'xunfei_super_tts.app_id': [{ required: true, message: 'Please enter app ID', trigger: 'blur' }],
  'xunfei_super_tts.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  'xunfei_super_tts.api_secret': [{ required: true, message: 'Please enter API Secret', trigger: 'blur' }],
  'xunfei_super_tts.ws_url': [{ required: true, message: 'Please enter WebSocket URL', trigger: 'blur' }],
  'xunfei_super_tts.voice': [{ required: true, message: 'Please enter voice', trigger: 'blur' }],
  // Zhipu TTS validation rules
  'zhipu.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  // Minimax TTS validation rules
  'minimax.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  // Qwen TTS validation rules
  'qwen_tts.api_key': [{ required: true, message: 'Please enter API Key', trigger: 'blur' }],
  'indextts_vllm.api_url': [{ required: true, message: 'Please enter API URL', trigger: 'blur' }]
}

const loadConfigs = async () => {
  loading.value = true
  try {
    const response = await api.get('/admin/tts-configs')
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
  form.double_stream = false

  // IndexTTS: request voice options when clicking voice dropdown
  loadVoiceOptions(config.provider)

  // Parse configuration JSON and fill in corresponding form fields
  try {
    const configData = JSON.parse(config.json_data || '{}')
    form.double_stream = configData.double_stream === true
    
    switch (config.provider) {
      case 'cosyvoice':
        form.cosyvoice.api_url = configData.api_url || ''
        form.cosyvoice.spk_id = configData.spk_id || ''
        form.cosyvoice.frame_duration = configData.frame_duration || 60
        form.cosyvoice.target_sr = configData.target_sr || 24000
        form.cosyvoice.audio_format = configData.audio_format || 'mp3'
        form.cosyvoice.instruct_text = configData.instruct_text || ''
        break
      case 'doubao':
        form.doubao.appid = configData.appid || ''
        form.doubao.access_token = configData.access_token || ''
        form.doubao.model = configData.model || 'seed-tts-2.0-standard'
        form.doubao.voice = configData.voice || ''
        form.doubao.api_url = configData.api_url || 'https://openspeech.bytedance.com/api/v3/tts/unidirectional'
        break
      case 'doubao_ws':
        form.doubao_ws.appid = configData.appid || ''
        form.doubao_ws.access_token = configData.access_token || ''
        form.doubao_ws.model = configData.model || 'seed-tts-2.0-standard'
        form.doubao_ws.resource_id = configData.resource_id || ''
        form.doubao_ws.voice = configData.voice || ''
        form.doubao_ws.ws_url = configData.ws_url || (configData.ws_host ? `wss://${configData.ws_host}/api/v3/tts/unidirectional/stream` : 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream')
        break
      case 'edge':
        form.edge.voice = configData.voice || ''
        form.edge.rate = configData.rate || '+0%'
        form.edge.volume = configData.volume || '+0%'
        form.edge.pitch = configData.pitch || '+0Hz'
        form.edge.connect_timeout = configData.connect_timeout || 10
        form.edge.receive_timeout = configData.receive_timeout || 60
        break
      case 'edge_offline':
        form.edge_offline.server_url = configData.server_url || ''
        form.edge_offline.timeout = configData.timeout || 30
        form.edge_offline.sample_rate = configData.sample_rate || 16000
        form.edge_offline.channels = configData.channels || 1
        form.edge_offline.frame_duration = configData.frame_duration || 20
        break
      case 'aliyun_qwen':
        form.qwen_tts.api_key = configData.api_key || ''
        form.qwen_tts.api_url = configData.api_url || 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation'
        form.qwen_tts.region = configData.region || 'beijing'
        form.qwen_tts.model = configData.model || 'qwen3-tts-flash'
        form.qwen_tts.voice = configData.voice || 'Cherry'
        form.qwen_tts.language_type = configData.language_type || 'Chinese'
        form.qwen_tts.stream = configData.stream !== undefined ? configData.stream : true
        form.qwen_tts.frame_duration = configData.frame_duration || 60
        break
      case 'openai':
        form.openai.api_key = configData.api_key || ''
        form.openai.api_url = configData.api_url || 'https://api.openai.com/v1/audio/speech'
        form.openai.model = configData.model || 'tts-1'
        form.openai.voice = configData.voice || 'alloy'
        form.openai.response_format = configData.response_format || 'mp3'
        form.openai.speed = configData.speed || 1.0
        form.openai.stream = configData.stream !== undefined ? configData.stream : true
        form.openai.frame_duration = configData.frame_duration || 60
        break
      case 'xunfei':
        form.xunfei.app_id = configData.app_id || ''
        form.xunfei.api_key = configData.api_key || ''
        form.xunfei.api_secret = configData.api_secret || ''
        form.xunfei.ws_url = configData.ws_url || 'wss://tts-api.xfyun.cn/v2/tts'
        form.xunfei.voice = configData.voice || 'xiaoyan'
        form.xunfei.audio_encoding = configData.audio_encoding || 'raw'
        form.xunfei.sample_rate = configData.sample_rate || 16000
        form.xunfei.speed = configData.speed ?? 50
        form.xunfei.volume = configData.volume ?? 50
        form.xunfei.pitch = configData.pitch ?? 50
        form.xunfei.tte = configData.tte || 'UTF8'
        form.xunfei.reg = configData.reg ?? 0
        form.xunfei.rdn = configData.rdn ?? 0
        form.xunfei.frame_duration = configData.frame_duration || 60
        form.xunfei.connect_timeout = configData.connect_timeout || 10
        form.xunfei.read_timeout = configData.read_timeout || 30
        break
      case 'xunfei_super_tts':
        form.xunfei_super_tts.app_id = configData.app_id || ''
        form.xunfei_super_tts.api_key = configData.api_key || ''
        form.xunfei_super_tts.api_secret = configData.api_secret || ''
        form.xunfei_super_tts.ws_url = configData.ws_url || 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6'
        form.xunfei_super_tts.voice = configData.voice || 'x6_lingxiaoxue_pro'
        form.xunfei_super_tts.audio_encoding = configData.audio_encoding || 'raw'
        form.xunfei_super_tts.sample_rate = configData.sample_rate || 24000
        form.xunfei_super_tts.speed = configData.speed ?? 50
        form.xunfei_super_tts.volume = configData.volume ?? 50
        form.xunfei_super_tts.pitch = configData.pitch ?? 50
        form.xunfei_super_tts.bgs = configData.bgs ?? 0
        form.xunfei_super_tts.reg = configData.reg ?? 0
        form.xunfei_super_tts.rdn = configData.rdn ?? 0
        form.xunfei_super_tts.rhy = configData.rhy ?? 0
        form.xunfei_super_tts.oral_level = configData.oral_level || 'mid'
        form.xunfei_super_tts.spark_assist = configData.spark_assist ?? 1
        form.xunfei_super_tts.stop_split = configData.stop_split ?? 0
        form.xunfei_super_tts.remain = configData.remain ?? 0
        form.xunfei_super_tts.frame_duration = configData.frame_duration || 60
        form.xunfei_super_tts.connect_timeout = configData.connect_timeout || 10
        form.xunfei_super_tts.read_timeout = configData.read_timeout || 30
        break
      case 'indextts_vllm':
        form.indextts_vllm.api_url = configData.api_url || 'http://127.0.0.1:7860'
        form.indextts_vllm.api_key = configData.api_key || ''
        form.indextts_vllm.model = configData.model || 'indextts-vllm'
        form.indextts_vllm.voice = configData.voice || ''
        form.indextts_vllm.frame_duration = configData.frame_duration || 60
        break
      case 'zhipu':
        // Zhipu configuration read from json_data
        form.zhipu.api_key = configData.api_key || ''
        form.zhipu.api_url = configData.api_url || 'https://open.bigmodel.cn/api/paas/v4/audio/speech'
        form.zhipu.model = configData.model || 'glm-tts'
        form.zhipu.voice = configData.voice || 'tongtong'
        form.zhipu.response_format = configData.response_format || 'pcm'
        form.zhipu.speed = configData.speed || 1.0
        form.zhipu.volume = configData.volume || 1.0
        form.zhipu.stream = configData.stream !== undefined ? configData.stream : true
        form.zhipu.encode_format = configData.encode_format || 'base64'
        form.zhipu.frame_duration = configData.frame_duration || 60
        break
      case 'minimax':
        form.minimax.api_key = configData.api_key || ''
        form.minimax.model = configData.model || 'speech-2.8-hd'
        form.minimax.voice = configData.voice || 'male-qn-qingse'
        form.minimax.speed = configData.speed || 1.0
        form.minimax.vol = configData.vol || configData.volume || 1.0
        form.minimax.pitch = configData.pitch || 0
        form.minimax.sample_rate = configData.sample_rate || 32000
        form.minimax.bitrate = configData.bitrate || 128000
        form.minimax.format = configData.format || 'mp3'
        form.minimax.channel = configData.channel || 1
        break
    }
  } catch (error) {
    console.error('Failed to parse configuration JSON:', error)
  }
  
  showDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        // If adding new config and no configs exist, auto-set as default
        const isFirstConfig = !editingConfig.value && configs.value.length === 0
        
        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // Auto-set as default on first add
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: formRef.value.getJsonData()
        }
        
        if (editingConfig.value) {
          await api.put(`/admin/tts-configs/${editingConfig.value.id}`, configData)
          ElMessage.success('Configuration updated successfully')
        } else {
          await api.post('/admin/tts-configs', configData)
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
    ElMessage.success(`${config.enabled ? 'Enabled' : 'Disabled'} successfully`)
  } catch (error) {
    // Restore switch state
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
    
    await api.put(`/admin/tts-configs/${config.id}`, configData)
    ElMessage.success(config.is_default ? 'Set as default successfully' : 'Removed default successfully')
    
    // Refresh list to update default status of other configurations
    loadConfigs()
  } catch (error) {
    // Restore switch state
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
        const result = await testSingleConfig('tts', row.config_id)
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
    const result = await testWithData('tts', { [configId]: payload })
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
    
    await api.delete(`/admin/tts-configs/${id}`)
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
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'doubao_ws',
    is_default: false,
    enabled: true,
    cosyvoice: {
      api_url: 'https://tts.linkerai.top/tts',
      spk_id: 'spk_id',
      frame_duration: 60,
      target_sr: 24000,
      audio_format: 'mp3',
      instruct_text: 'Hello'
    },
    qwen_tts: {
      api_key: '',
      api_url: 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation',
      region: 'beijing',
      model: 'qwen3-tts-flash',
      voice: 'Cherry',
      language_type: 'Chinese',
      stream: true,
      frame_duration: 60
    },
    doubao: {
      appid: '6886011847',
      access_token: 'access_token',
      model: 'seed-tts-2.0-standard',
      voice: 'BV001_streaming',
      api_url: 'https://openspeech.bytedance.com/api/v3/tts/unidirectional'
    },
    doubao_ws: {
      appid: '6886011847',
      access_token: 'access_token',
      model: 'seed-tts-2.0-standard',
      resource_id: '',
      voice: '',
      ws_url: 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream'
    },
    edge: {
      voice: 'zh-CN-XiaoxiaoNeural',
      rate: '+0%',
      volume: '+0%',
      pitch: '+0Hz',
      connect_timeout: 10,
      receive_timeout: 60
    },
    edge_offline: {
      server_url: 'ws://localhost:8080/tts',
      timeout: 30,
      sample_rate: 16000,
      channels: 1,
      frame_duration: 20
    },
    openai: {
      api_key: '',
      api_url: 'https://api.openai.com/v1/audio/speech',
      model: 'tts-1',
      voice: 'alloy',
      response_format: 'mp3',
      speed: 1.0,
      stream: true,
      frame_duration: 60
    },
    xunfei: {
      app_id: '',
      api_key: '',
      api_secret: '',
      ws_url: 'wss://tts-api.xfyun.cn/v2/tts',
      voice: 'xiaoyan',
      audio_encoding: 'raw',
      sample_rate: 16000,
      speed: 50,
      volume: 50,
      pitch: 50,
      tte: 'UTF8',
      reg: 0,
      rdn: 0,
      frame_duration: 60,
      connect_timeout: 10,
      read_timeout: 30
    },
    xunfei_super_tts: {
      app_id: '',
      api_key: '',
      api_secret: '',
      ws_url: 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6',
      voice: 'x6_lingxiaoxue_pro',
      audio_encoding: 'raw',
      sample_rate: 24000,
      speed: 50,
      volume: 50,
      pitch: 50,
      bgs: 0,
      reg: 0,
      rdn: 0,
      rhy: 0,
      oral_level: 'mid',
      spark_assist: 1,
      stop_split: 0,
      remain: 0,
      frame_duration: 60,
      connect_timeout: 10,
      read_timeout: 30
    },
    indextts_vllm: {
      api_url: 'http://127.0.0.1:7860',
      api_key: '',
      model: 'indextts-vllm',
      voice: '',
      frame_duration: 60
    },
    zhipu: {
      api_key: '',
      api_url: 'https://open.bigmodel.cn/api/paas/v4/audio/speech',
      model: 'glm-tts',
      voice: 'tongtong',
      response_format: 'pcm',
      speed: 1.0,
      volume: 1.0,
      stream: true,
      frame_duration: 60
    },
    minimax: {
      api_key: '',
      model: 'speech-2.8-hd',
      voice: 'male-qn-qingse',
      speed: 1.0,
      vol: 1.0,
      pitch: 0,
      sample_rate: 32000,
      bitrate: 128000,
      format: 'mp3',
      channel: 1
    }
  })
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

// Load voice options list
const loadVoiceOptions = async (provider, options = {}) => {
  const trigger = options?.trigger || 'auto'
  if (!provider) {
    voiceOptions.value = []
    return
  }

  // IndexTTS: only request when dropdown expands
  if (provider === 'indextts_vllm' && trigger !== 'dropdown') {
    voiceOptions.value = []
    return
  }
  
  // Only these providers need to fetch voice options from backend
  if (!TTS_PROVIDERS_WITH_VOICES.includes(provider)) {
    voiceOptions.value = []
    return
  }
  
  voiceLoading.value = true
  try {
    const params = { provider, config_id: form.config_id || undefined }
    if (provider === 'indextts_vllm') {
      const apiURL = String(form.indextts_vllm?.api_url || '').trim()
      const apiKey = String(form.indextts_vllm?.api_key || '').trim()
      if (apiURL) {
        params.api_url = apiURL
      }
      if (apiKey) {
        params.api_key = apiKey
      }
    }
    const response = await api.get(`/user/voice-options`, {
      params
    })
    voiceOptions.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load voice options:', error)
    voiceOptions.value = []
  } finally {
    voiceLoading.value = false
  }
}

const handleVoiceOptionsRequest = (provider) => {
  if (!showDialog.value) return
  loadVoiceOptions(provider || form.provider, { trigger: 'dropdown' })
}

// Watch provider changes, auto-load corresponding voice options
watch(() => form.provider, (newProvider) => {
  if (showDialog.value) {
    loadVoiceOptions(newProvider)
  }
}, { immediate: false })

// Watch dialog open, load current provider's voice options (nextTick ensures dialog is rendered before request)
watch(showDialog, (isOpen) => {
  if (isOpen && form.provider) {
    nextTick(() => loadVoiceOptions(form.provider))
  }
})

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
