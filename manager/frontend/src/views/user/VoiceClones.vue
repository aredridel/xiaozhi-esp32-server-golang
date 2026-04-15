<template>
  <div class="voice-clones-page">
    <div class="page-header">
      <div>
        <h2>Voice Cloning</h2>
        <p class="subtitle">Supports Minimax/CosyVoice/Qwen/IndexTTS, supports audio upload and browser recording</p>
      </div>
      <el-button type="primary" @click="openCreateDialog">Create Cloned Voice</el-button>
    </div>

    <el-table :data="voiceClones" v-loading="loading" stripe style="width: 100%" table-layout="fixed">
      <el-table-column prop="name" label="Name" min-width="120" show-overflow-tooltip />
      <el-table-column prop="provider" label="Provider" width="100" show-overflow-tooltip />
      <el-table-column label="TTS Config" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">{{ `${row.tts_config_name || '-'} (${row.tts_config_id || '-'})` }}</template>
      </el-table-column>
      <el-table-column prop="provider_voice_id" label="Cloned Voice ID" min-width="160" show-overflow-tooltip />
      <el-table-column v-if="authStore.isAdmin" label="Share to Everyone" width="140" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="!!row.shared_to_all"
            :disabled="normalizeCloneStatus(row) !== 'active' || shareSubmittingID === row.id"
            @change="(val) => toggleSharedToAll(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="Task Status" width="100">
        <template #default="{ row }">
          <el-tag :type="getCloneStatusTagType(row)" size="small">{{ formatCloneStatus(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Failure Reason" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ getCloneLastError(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="Created At" width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="Actions" width="460">
        <template #default="{ row }">
          <div class="action-buttons">
            <el-button
              size="small"
              type="primary"
              plain
              :loading="previewUploadSubmittingID === row.id"
              @click="previewUploadedAudio(row)"
            >
              Original Audio
            </el-button>
            <el-button
              v-if="canPreviewClonedVoice(row)"
              size="small"
              type="success"
              :loading="previewClonedSubmittingID === row.id"
              @click="previewClonedVoice(row)"
            >
              Preview Clone
            </el-button>
            <el-button size="small" type="primary" plain @click="openEditDialog(row)">Edit</el-button>
            <el-button
              v-if="canRetryClone(row)"
              size="small"
              type="warning"
              plain
              :loading="retrySubmittingID === row.id"
              @click="retryClone(row)"
            >
              Retry Clone
            </el-button>
            <el-button
              v-if="canAppendRefAudio(row)"
              size="small"
              type="primary"
              plain
              :loading="appendAudioSubmittingID === row.id"
              @click="openAppendAudioDialog(row)"
            >
              Append Reference Audio
            </el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :loading="deleteSubmittingID === row.id"
              @click="deleteClone(row)"
            >
              Delete
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <input
      ref="appendAudioInputRef"
      type="file"
      :accept="uploadAcceptTypes"
      style="display:none"
      @change="handleAppendAudioFileChange"
    />

    <el-dialog v-model="createDialogVisible" title="Create Cloned Voice" width="680px">
      <el-form label-width="140px">
        <el-form-item label="Clone Name">
          <el-input v-model="form.name" placeholder="Optional, will auto-use filename if empty" />
        </el-form-item>
        <el-form-item label="TTS Config" required>
          <el-select v-model="form.tts_config_id" placeholder="Please select TTS config that supports cloning" style="width: 100%" @change="onConfigChange">
            <el-option v-for="cfg in cloneEnabledConfigs" :key="cfg.config_id" :label="`${cfg.name} (${cfg.config_id})`" :value="cfg.config_id" />
          </el-select>
          <div v-if="isAliyunQwenProvider" class="help">Tip: After selecting this cloned voice, it will automatically switch to model {{ qwenCloneRuntimeModel }}</div>
          <el-alert
            v-if="createChargeNotice.message"
            class="clone-charge-alert"
            :title="createChargeNotice.message"
            :type="createChargeNotice.type"
            :closable="false"
            show-icon
          />
        </el-form-item>
        <el-form-item label="Audio Source">
          <el-radio-group v-model="form.source_type">
            <el-radio label="upload">Upload Audio</el-radio>
            <el-radio label="record">Browser Recording</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="form.source_type === 'upload'" label="Audio File" required>
          <input type="file" :accept="uploadAcceptTypes" @change="handleFileChange" />
          <div class="help">{{ audioRequirementText }}</div>
        </el-form-item>

        <el-form-item v-else label="Browser Recording" required>
          <el-button :disabled="isRecording" @click="startRecording">Start Recording</el-button>
          <el-button :disabled="!isRecording" type="warning" @click="stopRecording">Stop Recording</el-button>
          <audio v-if="recordPreviewUrl" :src="recordPreviewUrl" controls style="display:block;width:100%;margin-top:10px" />
          <div class="help">{{ audioRequirementText }}</div>
        </el-form-item>

        <el-form-item :label="capability.requires_transcript ? 'Audio Text *' : 'Audio Text'">
          <el-input
            v-model="form.transcript"
            type="textarea"
            :rows="4"
            :placeholder="capability.requires_transcript ? 'This provider requires audio text' : 'Optional, can submit without filling'"
          />
          <div class="help">Required: {{ capability.min_text_len || 0 }} - {{ capability.max_text_len || 4000 }} characters</div>
        </el-form-item>

        <el-form-item label="Text Language">
          <el-select v-model="form.transcript_lang" style="width: 220px">
            <el-option label="Chinese (zh-CN)" value="zh-CN" />
            <el-option label="English (en-US)" value="en-US" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="submitting" @click="submitClone">Submit Clone</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="audioDialogVisible" title="Clone Original Audio" width="720px">
      <el-table :data="currentAudios" stripe>
        <el-table-column prop="source_type" label="Source" width="90" />
        <el-table-column prop="file_name" label="Filename" min-width="220" />
        <el-table-column prop="transcript" label="Text" min-width="240" show-overflow-tooltip />
        <el-table-column label="Play" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="playAudio(row)">Play</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="Edit Cloned Voice" width="620px" @close="resetEditForm">
      <el-form label-width="120px">
        <el-form-item label="Name">
          <el-input v-model="editForm.name" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="Provider">
          <el-input v-model="editForm.provider" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="TTS Config">
          <el-input v-model="editForm.ttsConfigDisplay" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Cloned Voice ID">
          <el-input v-model="editForm.providerVoiceID" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Task Status">
          <el-input v-model="editForm.statusText" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Created At">
          <el-input v-model="editForm.createdAtText" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item v-if="editForm.lastError" label="Failure Reason">
          <el-input v-model="editForm.lastError" type="textarea" :rows="3" readonly class="readonly-field" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="submitEditClone">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewPlayerVisible" title="Audio Preview" width="560px" @close="closePreviewPlayerDialog">
      <div class="preview-player">
        <div class="preview-player-meta">
          <el-tag size="small" effect="plain">{{ previewPlayerSourceLabel || '-' }}</el-tag>
          <span class="preview-player-name">{{ previewPlayerCloneLabel || '-' }}</span>
        </div>
        <audio
          ref="previewPlayerRef"
          class="preview-player-audio"
          :src="previewPlayerURL"
          controls
          preload="metadata"
          @play="onPreviewAudioPlay"
          @pause="onPreviewAudioPause"
          @ended="onPreviewAudioEnded"
          @timeupdate="onPreviewAudioTimeUpdate"
          @loadedmetadata="onPreviewAudioLoadedMetadata"
        />
        <div class="preview-player-actions">
          <el-button type="primary" :disabled="!previewPlayerURL" @click="togglePreviewPlayback">
            {{ previewPlayerPlaying ? 'Pause' : 'Play' }}
          </el-button>
          <el-button :disabled="!previewPlayerURL" @click="stopPreviewPlayback">Stop</el-button>
          <span class="preview-player-time">{{ formatPlayerTime(previewPlayerCurrentTime) }} / {{ formatPlayerTime(previewPlayerDuration) }}</span>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../../utils/api'
import { useAuthStore } from '../../stores/auth'

const authStore = useAuthStore()
const loading = ref(false)
const submitting = ref(false)
const createDialogVisible = ref(false)
const audioDialogVisible = ref(false)
const editDialogVisible = ref(false)
const voiceClones = ref([])
const currentAudios = ref([])
const ttsConfigs = ref([])
const MIN_AUDIO_DURATION_SECONDS = 10
const cloneEnabledProviders = ['doubao_ws', 'minimax', 'cosyvoice', 'aliyun_qwen', 'indextts_vllm']
const pendingStatuses = ['queued', 'processing']
let clonePollingTimer = null
const clonePollingBusy = ref(false)
const editSubmitting = ref(false)
const retrySubmittingID = ref(null)
const previewUploadSubmittingID = ref(null)
const previewClonedSubmittingID = ref(null)
const appendAudioSubmittingID = ref(null)
const shareSubmittingID = ref(null)
const deleteSubmittingID = ref(null)
const appendAudioInputRef = ref(null)
const appendAudioTargetClone = ref(null)
const previewPlayerVisible = ref(false)
const previewPlayerRef = ref(null)
const previewPlayerURL = ref('')
const previewPlayerSourceLabel = ref('')
const previewPlayerCloneLabel = ref('')
const previewPlayerPlaying = ref(false)
const previewPlayerCurrentTime = ref(0)
const previewPlayerDuration = ref(0)

const form = ref({
  name: '',
  tts_config_id: '',
  source_type: 'upload',
  transcript: '',
  transcript_lang: 'zh-CN',
  audioFile: null,
  recordBlob: null,
  audioDurationSec: 0
})

const editForm = ref({
  id: null,
  originalName: '',
  name: '',
  provider: '',
  ttsConfigDisplay: '',
  providerVoiceID: '',
  statusText: '',
  createdAtText: '',
  lastError: ''
})

const capability = ref({ enabled: true, requires_transcript: false, min_text_len: 0, max_text_len: 0 })

const cloneEnabledConfigs = computed(() => ttsConfigs.value.filter(item => cloneEnabledProviders.includes(item.provider)))
const selectedCloneConfig = computed(() => cloneEnabledConfigs.value.find(item => item.config_id === form.value.tts_config_id) || null)
const currentCloneProvider = computed(() => selectedCloneConfig.value?.provider || '')
const normalizeProvider = (provider) => String(provider || '').trim().toLowerCase()
const resolveChargeNotice = (provider, scene = 'create') => {
  const normalized = normalizeProvider(provider)
  if (normalized === 'aliyun_qwen') {
    return {
      message: scene === 'create'
        ? 'Billing reminder: Qwen voice cloning charges per voice, 0.01 yuan per voice.'
        : 'Billing reminder: Qwen voice cloning charges per voice, 0.01 yuan per voice, please confirm to continue preview.',
      type: 'warning'
    }
  }
  if (normalized === 'minimax') {
    return {
      message: scene === 'create'
        ? 'Billing reminder: Minimax cloning is free, first preview of this cloned voice costs 9.9 yuan.'
        : 'Billing reminder: Minimax cloning is free, but first preview of this cloned voice costs 9.9 yuan, please confirm to continue.',
      type: 'warning'
    }
  }
  if (normalized === 'cosyvoice') {
    return {
      message: scene === 'create'
        ? 'Billing reminder: CosyVoice voice cloning and preview are free.'
        : 'Billing reminder: CosyVoice voice cloning and preview are free, please confirm to continue.',
      type: 'info'
    }
  }
  return { message: '', type: 'info' }
}
const createChargeNotice = computed(() => resolveChargeNotice(currentCloneProvider.value, 'create'))
const requiresMinimaxDuration = computed(() => currentCloneProvider.value === 'minimax')
const isAliyunQwenProvider = computed(() => currentCloneProvider.value === 'aliyun_qwen')
const qwenCloneRuntimeModel = 'qwen3-tts-vc-2026-01-22'
const uploadAcceptTypes = computed(() => {
  if (isAliyunQwenProvider.value) {
    return '.wav,.mp3,.m4a,audio/wav,audio/wave,audio/mpeg,audio/mp4,audio/x-m4a'
  }
  return '.wav,audio/wav,audio/wave'
})
const audioRequirementText = computed(() => {
  if (requiresMinimaxDuration.value) {
    return `Required: WAV format, duration at least ${MIN_AUDIO_DURATION_SECONDS} seconds`
  }
  if (isAliyunQwenProvider.value) {
    return 'Required: WAV/MP3/M4A, recommended 10-20 seconds (max 60 seconds)'
  }
  return 'Required: WAV format (CosyVoice requires audio text)'
})

const isRecording = ref(false)
const mediaRecorder = ref(null)
const recordChunks = ref([])
const recordPreviewUrl = ref('')

const formatDate = (value) => (value ? new Date(value).toLocaleString() : '-')
const parseMetaJSON = (metaJSON) => {
  if (!metaJSON || typeof metaJSON !== 'string') return {}
  try {
    return JSON.parse(metaJSON)
  } catch (error) {
    return {}
  }
}
const normalizeCloneStatus = (row) => {
  const status = String(row?.status || '').trim().toLowerCase()
  const taskStatus = String(row?.task_status || '').trim().toLowerCase()
  if (status === 'failed' || taskStatus === 'failed') return 'failed'
  if (status === 'active' || taskStatus === 'succeeded') return 'active'
  if (taskStatus === 'queued' || taskStatus === 'processing') return taskStatus
  if (status === 'queued' || status === 'processing') return status
  return status || taskStatus || 'unknown'
}
const formatCloneStatus = (row) => {
  const status = normalizeCloneStatus(row)
  if (status === 'queued') return 'Queued'
  if (status === 'processing') return 'Processing'
  if (status === 'active') return 'Success'
  if (status === 'failed') return 'Failed'
  return 'Unknown'
}
const getCloneStatusTagType = (row) => {
  const status = normalizeCloneStatus(row)
  if (status === 'queued') return 'info'
  if (status === 'processing') return 'warning'
  if (status === 'active') return 'success'
  if (status === 'failed') return 'danger'
  return 'info'
}
const getCloneLastError = (row) => {
  const status = normalizeCloneStatus(row)
  if (status !== 'failed') return '-'
  if (row?.task_last_error) return row.task_last_error
  const meta = parseMetaJSON(row?.meta_json)
  return meta.last_error || '-'
}
const canRetryClone = (row) => normalizeCloneStatus(row) === 'failed'
const canPreviewClonedVoice = (row) => normalizeCloneStatus(row) === 'active'
const canAppendRefAudio = (row) => normalizeCloneStatus(row) === 'active' && normalizeProvider(row?.provider) === 'indextts_vllm'
const formatPlayerTime = (seconds) => {
  const value = Number(seconds || 0)
  if (!Number.isFinite(value) || value < 0) return '00:00'
  const total = Math.floor(value)
  const minute = String(Math.floor(total / 60)).padStart(2, '0')
  const second = String(total % 60).padStart(2, '0')
  return `${minute}:${second}`
}
const pauseAllOtherAudios = () => {
  const current = previewPlayerRef.value
  document.querySelectorAll('audio').forEach(audioEl => {
    if (audioEl !== current) {
      try {
        audioEl.pause()
      } catch (error) {
        // ignore pause errors from detached nodes
      }
    }
  })
}
const revokePreviewPlayerURL = () => {
  if (!previewPlayerURL.value) return
  URL.revokeObjectURL(previewPlayerURL.value)
  previewPlayerURL.value = ''
}
const stopPreviewPlayback = () => {
  const audioEl = previewPlayerRef.value
  if (!audioEl) return
  audioEl.pause()
  audioEl.currentTime = 0
  previewPlayerCurrentTime.value = 0
}
const closePreviewPlayerDialog = () => {
  stopPreviewPlayback()
  previewPlayerPlaying.value = false
  previewPlayerCurrentTime.value = 0
  previewPlayerDuration.value = 0
  previewPlayerSourceLabel.value = ''
  previewPlayerCloneLabel.value = ''
  revokePreviewPlayerURL()
}
const setPreviewPlayerSource = async (blob, sourceLabel, cloneLabel) => {
  stopPreviewPlayback()
  revokePreviewPlayerURL()
  previewPlayerURL.value = URL.createObjectURL(blob)
  previewPlayerSourceLabel.value = sourceLabel
  previewPlayerCloneLabel.value = cloneLabel
  previewPlayerCurrentTime.value = 0
  previewPlayerDuration.value = 0
  previewPlayerVisible.value = true
  await nextTick()
  pauseAllOtherAudios()
  const audioEl = previewPlayerRef.value
  if (!audioEl) return
  try {
    await audioEl.play()
  } catch (error) {
    ElMessage.info('Audio loaded, click play to preview')
  }
}
const togglePreviewPlayback = async () => {
  const audioEl = previewPlayerRef.value
  if (!audioEl) return
  if (audioEl.paused) {
    pauseAllOtherAudios()
    await audioEl.play()
    return
  }
  audioEl.pause()
}
const onPreviewAudioPlay = () => {
  pauseAllOtherAudios()
  previewPlayerPlaying.value = true
}
const onPreviewAudioPause = () => {
  previewPlayerPlaying.value = false
}
const onPreviewAudioEnded = () => {
  previewPlayerPlaying.value = false
  previewPlayerCurrentTime.value = previewPlayerDuration.value
}
const onPreviewAudioTimeUpdate = () => {
  const audioEl = previewPlayerRef.value
  if (!audioEl) return
  previewPlayerCurrentTime.value = Number(audioEl.currentTime || 0)
}
const onPreviewAudioLoadedMetadata = () => {
  const audioEl = previewPlayerRef.value
  if (!audioEl) return
  previewPlayerDuration.value = Number(audioEl.duration || 0)
}
const hasPendingCloneTask = (row) => pendingStatuses.includes(normalizeCloneStatus(row))
const clearClonePollingTimer = () => {
  if (!clonePollingTimer) return
  window.clearTimeout(clonePollingTimer)
  clonePollingTimer = null
}
const scheduleClonePolling = () => {
  if (clonePollingTimer) return
  clonePollingTimer = window.setTimeout(async () => {
    clonePollingTimer = null
    if (!voiceClones.value.some(hasPendingCloneTask)) return
    if (clonePollingBusy.value) {
      scheduleClonePolling()
      return
    }
    clonePollingBusy.value = true
    try {
      await loadVoiceClones(true)
    } finally {
      clonePollingBusy.value = false
      if (voiceClones.value.some(hasPendingCloneTask)) {
        scheduleClonePolling()
      }
    }
  }, 2000)
}

const loadVoiceClones = async (silent = false) => {
  if (!silent) loading.value = true
  try {
    const res = await api.get('/user/voice-clones')
    voiceClones.value = res.data.data || []
  } finally {
    if (!silent) loading.value = false
    if (voiceClones.value.some(hasPendingCloneTask)) {
      scheduleClonePolling()
    } else {
      clearClonePollingTimer()
    }
  }
}

const loadTtsConfigs = async () => {
  const res = await api.get('/user/tts-configs')
  ttsConfigs.value = res.data.data || []
}

const openCreateDialog = async () => {
  createDialogVisible.value = true
  await loadTtsConfigs()
  if (!cloneEnabledConfigs.value.length) {
    form.value.tts_config_id = ''
    return
  }
  const selectedConfig = cloneEnabledConfigs.value.find(item => item.config_id === form.value.tts_config_id)
  if (!selectedConfig) {
    form.value.tts_config_id = cloneEnabledConfigs.value[0].config_id
  }
  await onConfigChange(form.value.tts_config_id)
}

const onConfigChange = async (configId) => {
  const cfg = cloneEnabledConfigs.value.find(item => item.config_id === configId)
  if (!cfg) {
    capability.value = { enabled: true, requires_transcript: false, min_text_len: 0, max_text_len: 0 }
    return
  }
  const res = await api.get('/user/voice-clone/capabilities', { params: { provider: cfg.provider } })
  capability.value = res.data.data || capability.value
}

const isWavFile = (file) => {
  const name = (file?.name || '').toLowerCase()
  const type = (file?.type || '').toLowerCase()
  return type.includes('audio/wav') || type.includes('audio/wave') || name.endsWith('.wav')
}

const isSupportedAliyunQwenAudio = (file) => {
  const name = (file?.name || '').toLowerCase()
  const type = (file?.type || '').toLowerCase()
  if (name.endsWith('.wav') || name.endsWith('.mp3') || name.endsWith('.m4a')) {
    return true
  }
  return type.includes('audio/wav') || type.includes('audio/wave') || type.includes('audio/mpeg') || type.includes('audio/mp4') || type.includes('audio/x-m4a')
}

const isSupportedUploadAudio = (file) => {
  if (isAliyunQwenProvider.value) {
    return isSupportedAliyunQwenAudio(file)
  }
  return isWavFile(file)
}

const getAudioDurationSeconds = (blobOrFile) => new Promise((resolve, reject) => {
  const url = URL.createObjectURL(blobOrFile)
  const audio = new Audio()
  audio.preload = 'metadata'
  audio.onloadedmetadata = () => {
    const duration = Number(audio.duration || 0)
    URL.revokeObjectURL(url)
    if (!Number.isFinite(duration) || duration <= 0) {
      reject(new Error('Cannot read audio duration'))
      return
    }
    resolve(duration)
  }
  audio.onerror = () => {
    URL.revokeObjectURL(url)
    reject(new Error('Cannot parse audio file'))
  }
  audio.src = url
})

const handleFileChange = async (event) => {
  const file = event.target.files?.[0] || null
  if (!file) {
    form.value.audioFile = null
    form.value.audioDurationSec = 0
    return
  }
  if (!isSupportedUploadAudio(file)) {
    ElMessage.warning(isAliyunQwenProvider.value ? 'Only WAV/MP3/M4A audio supported' : 'Only WAV format audio supported')
    form.value.audioFile = null
    form.value.audioDurationSec = 0
    event.target.value = ''
    return
  }
  if (!requiresMinimaxDuration.value) {
    form.value.audioFile = file
    form.value.audioDurationSec = 0
    return
  }
  try {
    const duration = await getAudioDurationSeconds(file)
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(`Audio duration must be at least ${MIN_AUDIO_DURATION_SECONDS} seconds, current is about ${duration.toFixed(2)} seconds`)
      form.value.audioFile = null
      form.value.audioDurationSec = 0
      event.target.value = ''
      return
    }
    form.value.audioFile = file
    form.value.audioDurationSec = duration
  } catch (error) {
    ElMessage.warning(error.message || 'Failed to read audio duration')
    form.value.audioFile = null
    form.value.audioDurationSec = 0
    event.target.value = ''
  }
}

const convertToWav = async (blob) => {
  const arrayBuffer = await blob.arrayBuffer()
  const audioContext = new (window.AudioContext || window.webkitAudioContext)()
  try {
    const audioBuffer = await audioContext.decodeAudioData(arrayBuffer)
    const wav = audioBufferToWav(audioBuffer)
    return new Blob([wav], { type: 'audio/wav' })
  } finally {
    await audioContext.close()
  }
}

const audioBufferToWav = (buffer) => {
  const length = buffer.length
  const numberOfChannels = buffer.numberOfChannels
  const sampleRate = buffer.sampleRate
  const bytesPerSample = 2
  const blockAlign = numberOfChannels * bytesPerSample
  const byteRate = sampleRate * blockAlign
  const dataSize = length * blockAlign
  const bufferSize = 44 + dataSize
  const arrayBuffer = new ArrayBuffer(bufferSize)
  const view = new DataView(arrayBuffer)
  const writeString = (offset, str) => {
    for (let i = 0; i < str.length; i += 1) {
      view.setUint8(offset + i, str.charCodeAt(i))
    }
  }

  writeString(0, 'RIFF')
  view.setUint32(4, bufferSize - 8, true)
  writeString(8, 'WAVE')
  writeString(12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, numberOfChannels, true)
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, byteRate, true)
  view.setUint16(32, blockAlign, true)
  view.setUint16(34, 16, true)
  writeString(36, 'data')
  view.setUint32(40, dataSize, true)

  let offset = 44
  for (let i = 0; i < length; i += 1) {
    for (let channel = 0; channel < numberOfChannels; channel += 1) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]))
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7FFF, true)
      offset += 2
    }
  }
  return arrayBuffer
}

const startRecording = async () => {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
  recordChunks.value = []
  form.value.audioDurationSec = 0
  const recorderOptions = { mimeType: 'audio/webm;codecs=opus' }
  const recorder = MediaRecorder.isTypeSupported(recorderOptions.mimeType) ? new MediaRecorder(stream, recorderOptions) : new MediaRecorder(stream)
  mediaRecorder.value = recorder
  recorder.ondataavailable = (evt) => {
    if (evt.data && evt.data.size > 0) recordChunks.value.push(evt.data)
  }
  recorder.onstop = async () => {
    const blob = new Blob(recordChunks.value, { type: recordChunks.value[0]?.type || 'audio/webm' })
    try {
      const wavBlob = await convertToWav(blob)
      const duration = await getAudioDurationSeconds(wavBlob)
      if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
        ElMessage.warning(`Recording duration must be at least ${MIN_AUDIO_DURATION_SECONDS} seconds, current is about ${duration.toFixed(2)} seconds`)
        form.value.recordBlob = null
        form.value.audioDurationSec = 0
        if (recordPreviewUrl.value) {
          URL.revokeObjectURL(recordPreviewUrl.value)
          recordPreviewUrl.value = ''
        }
      } else {
        form.value.recordBlob = wavBlob
        form.value.audioDurationSec = duration
        if (recordPreviewUrl.value) URL.revokeObjectURL(recordPreviewUrl.value)
        recordPreviewUrl.value = URL.createObjectURL(wavBlob)
      }
    } catch (error) {
      ElMessage.error('Recording conversion failed, please try again')
      form.value.recordBlob = null
      form.value.audioDurationSec = 0
      if (recordPreviewUrl.value) {
        URL.revokeObjectURL(recordPreviewUrl.value)
        recordPreviewUrl.value = ''
      }
    }
    stream.getTracks().forEach(t => t.stop())
  }
  recorder.start()
  isRecording.value = true
}

const stopRecording = () => {
  if (mediaRecorder.value) mediaRecorder.value.stop()
  isRecording.value = false
}

const submitClone = async () => {
  if (!form.value.tts_config_id) {
    ElMessage.warning('Please select a TTS config that supports cloning')
    return
  }
  const createNotice = resolveChargeNotice(currentCloneProvider.value, 'create')
  if (createNotice.message) {
    try {
      await ElMessageBox.confirm(createNotice.message, 'Create Clone Reminder', {
        confirmButtonText: 'I understand, continue',
        cancelButtonText: 'Cancel',
        type: createNotice.type
      })
    } catch (error) {
      return
    }
  }
  if (capability.value.requires_transcript && !form.value.transcript.trim()) {
    ElMessage.warning('This provider requires audio text')
    return
  }

  const fd = new FormData()
  fd.append('name', form.value.name)
  fd.append('tts_config_id', form.value.tts_config_id)
  fd.append('source_type', form.value.source_type)
  fd.append('transcript', form.value.transcript)
  fd.append('transcript_lang', form.value.transcript_lang)

  if (form.value.source_type === 'upload') {
    if (!form.value.audioFile) {
      ElMessage.warning('Please upload audio file')
      return
    }
    let duration = form.value.audioDurationSec
    if (requiresMinimaxDuration.value && !duration) {
      try {
        duration = await getAudioDurationSeconds(form.value.audioFile)
      } catch (error) {
        ElMessage.warning(error.message || 'Failed to read audio duration')
        return
      }
    }
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(`Audio duration must be at least ${MIN_AUDIO_DURATION_SECONDS} seconds, current is about ${duration.toFixed(2)} seconds`)
      return
    }
    fd.append('audio_file', form.value.audioFile)
  } else {
    if (!form.value.recordBlob) {
      ElMessage.warning('Please record first')
      return
    }
    let duration = form.value.audioDurationSec
    if (requiresMinimaxDuration.value && !duration) {
      try {
        duration = await getAudioDurationSeconds(form.value.recordBlob)
      } catch (error) {
        ElMessage.warning(error.message || 'Failed to read recording duration')
        return
      }
    }
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(`Recording duration must be at least ${MIN_AUDIO_DURATION_SECONDS} seconds, current is about ${duration.toFixed(2)} seconds`)
      return
    }
    fd.append('audio_blob', form.value.recordBlob, `recording_${Date.now()}.wav`)
  }

  submitting.value = true
  try {
    const res = await api.post('/user/voice-clones', fd, { timeout: 120000 })
    const queued = res.status === 202 || pendingStatuses.includes(normalizeCloneStatus(res.data?.data || {}))
    ElMessage.success(queued ? 'Clone task submitted, processing in background' : 'Cloned voice created successfully')
    createDialogVisible.value = false
    await loadVoiceClones()
  } finally {
    submitting.value = false
  }
}

const loadAudios = async (clone) => {
  const res = await api.get(`/user/voice-clones/${clone.id}/audios`)
  currentAudios.value = res.data.data || []
  audioDialogVisible.value = true
}

const openEditDialog = (clone) => {
  if (!clone) return
  editForm.value = {
    id: clone.id,
    originalName: String(clone.name || ''),
    name: String(clone.name || ''),
    provider: String(clone.provider || '-'),
    ttsConfigDisplay: `${clone.tts_config_name || '-'} (${clone.tts_config_id || '-'})`,
    providerVoiceID: String(clone.provider_voice_id || '-'),
    statusText: formatCloneStatus(clone),
    createdAtText: formatDate(clone.created_at),
    lastError: String(getCloneLastError(clone) === '-' ? '' : getCloneLastError(clone))
  }
  editDialogVisible.value = true
}

const resetEditForm = () => {
  editForm.value = {
    id: null,
    originalName: '',
    name: '',
    provider: '',
    ttsConfigDisplay: '',
    providerVoiceID: '',
    statusText: '',
    createdAtText: '',
    lastError: ''
  }
  editSubmitting.value = false
}

const submitEditClone = async () => {
  const cloneID = editForm.value.id
  if (!cloneID) return
  const nextName = String(editForm.value.name || '').trim()
  if (!nextName) {
    ElMessage.warning('Name cannot be empty')
    return
  }
  if ([...nextName].length > 100) {
    ElMessage.warning('Name length cannot exceed 100 characters')
    return
  }
  if (nextName === String(editForm.value.originalName || '').trim()) {
    editDialogVisible.value = false
    return
  }

  editSubmitting.value = true
  try {
    await api.put(`/user/voice-clones/${cloneID}`, { name: nextName })
    ElMessage.success('Name updated successfully')
    editDialogVisible.value = false
    await loadVoiceClones(true)
  } finally {
    editSubmitting.value = false
  }
}

const retryClone = async (clone) => {
  if (!clone?.id || !canRetryClone(clone) || retrySubmittingID.value) return
  retrySubmittingID.value = clone.id
  try {
    await api.post(`/user/voice-clones/${clone.id}/retry`)
    ElMessage.success('Retry clone task submitted, processing in background')
    await loadVoiceClones(true)
  } finally {
    retrySubmittingID.value = null
  }
}

const toggleSharedToAll = async (clone, nextValue) => {
  if (!authStore.isAdmin || !clone?.id) return
  shareSubmittingID.value = clone.id
  try {
    await api.put(`/user/voice-clones/${clone.id}`, { shared_to_all: !!nextValue })
    clone.shared_to_all = !!nextValue
    ElMessage.success(nextValue ? 'Enabled for everyone' : 'Sharing disabled')
  } finally {
    shareSubmittingID.value = null
  }
}

const deleteClone = async (clone) => {
  if (!clone?.id || deleteSubmittingID.value) return
  try {
    await ElMessageBox.confirm(
      `Confirm delete cloned voice "${clone.name || clone.provider_voice_id || clone.id}"? After deletion, it will no longer appear in the list and selectable voices.`,
      'Delete Cloned Voice',
      {
        type: 'warning',
        confirmButtonText: 'Delete',
        cancelButtonText: 'Cancel'
      }
    )
  } catch {
    return
  }
  deleteSubmittingID.value = clone.id
  try {
    await api.delete(`/user/voice-clones/${clone.id}`)
    ElMessage.success('Delete successful')
    await loadVoiceClones(true)
  } finally {
    deleteSubmittingID.value = null
  }
}

const openAppendAudioDialog = (clone) => {
  if (!clone?.id || !canAppendRefAudio(clone) || appendAudioSubmittingID.value) return
  appendAudioTargetClone.value = clone
  const input = appendAudioInputRef.value
  if (!input) {
    ElMessage.error('File selector not ready')
    return
  }
  input.value = ''
  input.click()
}

const handleAppendAudioFileChange = async (event) => {
  const file = event?.target?.files?.[0]
  const clone = appendAudioTargetClone.value
  if (!file || !clone?.id) {
    appendAudioTargetClone.value = null
    return
  }
  appendAudioSubmittingID.value = clone.id
  try {
    const fd = new FormData()
    fd.append('source_type', 'upload')
    fd.append('audio_file', file)
    await api.post(`/user/voice-clones/${clone.id}/append-audio`, fd, { timeout: 120000 })
    ElMessage.success('Append reference audio successful')
    await loadVoiceClones(true)
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Failed to append reference audio')
  } finally {
    appendAudioSubmittingID.value = null
    appendAudioTargetClone.value = null
    if (event?.target) event.target.value = ''
  }
}

const playAudio = async (audio) => {
  const response = await api.get(`/user/voice-clones/audios/${audio.id}/file`, { responseType: 'blob' })
  const label = String(audio?.file_name || '')
  await setPreviewPlayerSource(response.data, 'Original Audio', label || 'Clone Original Audio')
}

const previewUploadedAudio = async (clone) => {
  if (!clone?.id || previewUploadSubmittingID.value) return
  previewUploadSubmittingID.value = clone.id
  try {
    const res = await api.get(`/user/voice-clones/${clone.id}/audios`)
    const audios = res.data.data || []
    if (!audios.length) {
      ElMessage.warning('No uploaded audio found')
      return
    }
    const audioRes = await api.get(`/user/voice-clones/audios/${audios[0].id}/file`, { responseType: 'blob' })
    await setPreviewPlayerSource(audioRes.data, 'Original Audio', String(clone?.name || 'Clone Task'))
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Failed to preview uploaded audio')
  } finally {
    previewUploadSubmittingID.value = null
  }
}

const previewClonedVoice = async (clone) => {
  if (!clone?.id || !canPreviewClonedVoice(clone) || previewClonedSubmittingID.value) return
  const previewNotice = resolveChargeNotice(clone?.provider, 'preview')
  if (previewNotice.message) {
    try {
      await ElMessageBox.confirm(previewNotice.message, 'Preview Clone Reminder', {
        confirmButtonText: 'Continue Preview',
        cancelButtonText: 'Cancel',
        type: previewNotice.type
      })
    } catch (error) {
      return
    }
  }
  previewClonedSubmittingID.value = clone.id
  try {
    const response = await api.get(`/user/voice-clones/${clone.id}/preview`, { responseType: 'blob' })
    await setPreviewPlayerSource(response.data, 'Preview Clone', String(clone?.name || 'Clone Task'))
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Failed to preview cloned audio')
  } finally {
    previewClonedSubmittingID.value = null
  }
}

onMounted(async () => {
  await loadVoiceClones()
})

onBeforeUnmount(() => {
  clearClonePollingTimer()
  closePreviewPlayerDialog()
})
</script>

<style scoped>
.voice-clones-page {
  padding: 20px;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.subtitle {
  color: #666;
  margin-top: 4px;
}
.help {
  color: #999;
  font-size: 12px;
  margin-top: 4px;
}

.clone-charge-alert {
  margin-top: 8px;
}

.voice-clones-page :deep(.el-table .cell) {
  white-space: nowrap;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.action-buttons :deep(.el-button) {
  margin: 0;
  white-space: nowrap;
}

.readonly-field :deep(.el-input__wrapper) {
  background-color: var(--el-fill-color-light);
  box-shadow: 0 0 0 1px var(--el-border-color-light) inset;
}

.readonly-field :deep(.el-input__inner) {
  color: var(--el-text-color-secondary);
}

.readonly-field :deep(.el-textarea__inner) {
  background-color: var(--el-fill-color-light);
  border-color: var(--el-border-color-light);
  color: var(--el-text-color-secondary);
}

.preview-player {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.preview-player-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.preview-player-name {
  color: var(--el-text-color-regular);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-player-audio {
  width: 100%;
}

.preview-player-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.preview-player-time {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
