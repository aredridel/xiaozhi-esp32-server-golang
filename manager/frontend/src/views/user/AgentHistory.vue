<template>
  <div class="agent-history-page">
    <div class="page-header">
      <div class="header-left">
        <el-button 
          @click="$router.back()" 
          :icon="ArrowLeft" 
          circle 
          size="large"
        />
        <div class="header-info">
          <h1>{{ agentName || 'Chat History' }}</h1>
          <p class="page-subtitle" v-if="total > 0">Total {{ total }} messages</p>
        </div>
      </div>
      <div class="header-right">
        <el-button @click="handleExport" :loading="exporting">
          <el-icon><Download /></el-icon>
          Export Records
        </el-button>
      </div>
    </div>

    <!-- Filter Panel -->
    <el-card class="filter-card" shadow="never">
      <el-form :model="filters" inline>
        <el-form-item label="Role">
          <el-select v-model="filters.role" placeholder="All" clearable style="width: 120px">
            <el-option label="All" value="" />
            <el-option label="User" value="user" />
            <el-option label="Assistant" value="assistant" />
          </el-select>
        </el-form-item>
        <el-form-item label="Device">
          <el-select v-model="filters.device_id" placeholder="All" clearable style="width: 150px">
            <el-option label="All" value="" />
            <el-option 
              v-for="device in devices" 
              :key="device.id" 
              :label="device.device_name || device.device_code" 
              :value="device.device_name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Start Date">
          <el-date-picker
            v-model="filters.start_date"
            type="date"
            placeholder="Select date"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            style="width: 150px"
            clearable
          />
        </el-form-item>
        <el-form-item label="End Date">
          <el-date-picker
            v-model="filters.end_date"
            type="date"
            placeholder="Select date"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            style="width: 150px"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">Search</el-button>
          <el-button @click="handleReset">Reset</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Message List - WeChat Style -->
    <el-card class="messages-card" shadow="never" v-loading="loading">
      <div v-if="messages.length === 0" class="empty-state">
        <el-empty description="No chat records" />
      </div>
      <div v-else class="chat-container">
        <div class="chat-messages" ref="chatMessagesRef">
          <div 
            v-for="(message, index) in messages" 
            :key="message.id" 
            class="message-wrapper"
            :class="{ 'message-right': message.role === 'user', 'message-left': message.role === 'assistant' }"
          >
            <!-- Timestamp (if interval with previous message exceeds 5 minutes, show time) -->
            <div v-if="shouldShowTime(message, index)" class="message-time-divider">
              {{ formatTimeShort(message.created_at) }}
            </div>
            
            <div class="message-bubble-wrapper">
              <!-- Left side: Assistant message -->
              <template v-if="message.role === 'assistant'">
                <div class="message-bubble message-bubble-left">
                  <div class="message-content-wrapper">
                    <!-- Text content -->
                    <div v-if="message.content" class="message-text">{{ message.content }}</div>
                    <!-- Audio player -->
                    <div v-if="message.audio_path" class="audio-bubble">
                      <audio
                        :ref="el => audioRefs[message.id] = el"
                        :src="audioBlobUrls[message.id]"
                        @ended="handleAudioEnded(message.id)"
                        @error="handleAudioError(message.id)"
                      />
                      <el-button 
                        :icon="playingAudioId === message.id ? VideoPause : VideoPlay"
                        circle
                        size="small"
                        @click="toggleAudio(message.id)"
                        class="audio-play-btn-simple"
                      />
                    </div>
                    <div class="message-meta">
                      <span class="message-time-small">{{ formatTimeShort(message.created_at) }}</span>
                      <el-dropdown trigger="click" @command="handleMessageAction">
                        <el-icon class="message-more"><MoreFilled /></el-icon>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item :command="{action: 'delete', id: message.id}">Delete</el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                    </div>
                  </div>
                </div>
              </template>
              
              <!-- Right side: User message -->
              <template v-else>
                <div class="message-bubble message-bubble-right">
                  <div class="message-content-wrapper">
                    <!-- Text content -->
                    <div v-if="message.content" class="message-text">{{ message.content }}</div>
                    <!-- Audio player -->
                    <div v-if="message.audio_path" class="audio-bubble">
                      <audio
                        :ref="el => audioRefs[message.id] = el"
                        :src="audioBlobUrls[message.id]"
                        @ended="handleAudioEnded(message.id)"
                        @error="handleAudioError(message.id)"
                      />
                      <el-button 
                        :icon="playingAudioId === message.id ? VideoPause : VideoPlay"
                        circle
                        size="small"
                        @click="toggleAudio(message.id)"
                        class="audio-play-btn-simple"
                      />
                    </div>
                    <div class="message-meta">
                      <el-dropdown trigger="click" @command="handleMessageAction">
                        <el-icon class="message-more"><MoreFilled /></el-icon>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item :command="{action: 'delete', id: message.id}">Delete</el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                      <span class="message-time-small">{{ formatTimeShort(message.created_at) }}</span>
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div class="pagination" v-if="total > 0">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="total"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Download, User, Service, VideoPlay, VideoPause, MoreFilled } from '@element-plus/icons-vue'
import api from '../../utils/api'

const route = useRoute()
const router = useRouter()

const agentId = computed(() => {
  const id = route.params.id
  return id ? String(id) : null
})
const agentName = ref('')
const loading = ref(false)
const exporting = ref(false)
const messages = ref([])
const total = ref(0)
const devices = ref([])
const deletingId = ref(null)

// Filter conditions
const filters = reactive({
  role: '',
  device_id: '',
  start_date: '',
  end_date: ''
})

// Pagination
const pagination = reactive({
  page: 1,
  pageSize: 50
})

// Calculate total pages
const totalPages = computed(() => {
  return Math.ceil(total.value / pagination.pageSize)
})

// Audio playback related
const audioRefs = ref({})
const playingAudioId = ref(null)
const chatMessagesRef = ref(null)
const audioBlobUrls = ref({}) // Store audio Blob URLs

// Load agent info
const loadAgent = async () => {
  if (!agentId.value) {
    ElMessage.error('Invalid agent ID')
    router.back()
    return
  }
  try {
    const response = await api.get(`/user/agents/${agentId.value}`)
    agentName.value = response.data.data?.name || 'Agent'
  } catch (error) {
    console.error('Failed to load agent info:', error)
    ElMessage.error('Failed to load agent info')
  }
}

// Load device list
const loadDevices = async () => {
  try {
    const response = await api.get(`/user/agents/${agentId.value}/devices`)
    devices.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load device list:', error)
  }
}

// Load message list
const loadMessages = async () => {
  if (!agentId.value) {
    return
  }
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize
    }
    if (filters.role) params.role = filters.role
    if (filters.device_id) params.device_id = filters.device_id
    if (filters.start_date) params.start_date = filters.start_date
    if (filters.end_date) params.end_date = filters.end_date

    const response = await api.get(`/user/history/agents/${agentId.value}/messages`, { params })
    // Backend returns in reverse chronological order (newest first), need to reverse array so newest is at bottom
    const data = response.data.data || []
    messages.value = [...data].reverse() // Reverse array, newest at bottom
    total.value = response.data.total || 0
    
    // Preload messages with audio
    await preloadAudioMessages()
    
    // Scroll to bottom after loading (show newest messages)
    await nextTick()
    scrollToBottom()
  } catch (error) {
    ElMessage.error('Failed to load message list: ' + (error.response?.data?.error || error.message))
    console.error('Failed to load message list:', error)
    messages.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// Search
const handleSearch = () => {
  pagination.page = 1
  loadMessages()
}

// Reset filters
const handleReset = () => {
  filters.role = ''
  filters.device_id = ''
  filters.start_date = ''
  filters.end_date = ''
  pagination.page = 1
  loadMessages()
}

// Page change
const handlePageChange = (page) => {
  pagination.page = page
  loadMessages()
}

const handleSizeChange = (size) => {
  pagination.pageSize = size
  pagination.page = 1
  loadMessages()
}

// Delete message
const handleDelete = async (messageId) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this message?', 'Confirm', {
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel',
      type: 'warning'
    })
    
    deletingId.value = messageId
    await api.delete(`/user/history/messages/${messageId}`)
    ElMessage.success('Delete successful')
    loadMessages()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Delete failed')
      console.error('Failed to delete message:', error)
    }
  } finally {
    deletingId.value = null
  }
}

// Export records
const handleExport = async () => {
  exporting.value = true
  try {
    const params = {
      agent_id: agentId.value
    }
    if (filters.role) params.role = filters.role
    if (filters.device_id) params.device_id = filters.device_id
    if (filters.start_date) params.start_date = filters.start_date
    if (filters.end_date) params.end_date = filters.end_date

    const response = await api.get('/user/history/export', { 
      params,
      responseType: 'blob'
    })
    
    // Create download link
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `chat_history_${new Date().toISOString().slice(0, 10)}.json`)
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    
    ElMessage.success('Export successful')
  } catch (error) {
    ElMessage.error('Export failed')
    console.error('Export failed:', error)
  } finally {
    exporting.value = false
  }
}

// Format time (full)
const formatTime = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// Format time (short, for message bubbles)
const formatTimeShort = (dateString) => {
  const date = new Date(dateString)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const msgDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  
  // If today, only show time
  if (msgDate.getTime() === today.getTime()) {
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit'
    })
  }
  
  // If yesterday
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  if (msgDate.getTime() === yesterday.getTime()) {
    return 'Yesterday ' + date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit'
    })
  }
  
  // If this year, show month/day and time
  if (date.getFullYear() === now.getFullYear()) {
    return `${date.getMonth() + 1}/${date.getDate()} ${date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit'
    })}`
  }
  
  // Otherwise show full date and time
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// Determine whether to show time divider
const shouldShowTime = (message, index) => {
  if (index === 0) return true
  const currentTime = new Date(message.created_at).getTime()
  const prevTime = new Date(messages.value[index - 1].created_at).getTime()
  // If interval with previous message exceeds 5 minutes, show time
  return (currentTime - prevTime) > 5 * 60 * 1000
}

// Handle message action
const handleMessageAction = (command) => {
  if (command.action === 'delete') {
    handleDelete(command.id)
  }
}

// Scroll to bottom
const scrollToBottom = () => {
  if (chatMessagesRef.value) {
    nextTick(() => {
      chatMessagesRef.value.scrollTop = chatMessagesRef.value.scrollHeight
    })
  }
}

// Get audio URL (using Blob URL to support authentication)
const getAudioUrl = async (messageId) => {
  // If Blob URL already exists, return directly
  if (audioBlobUrls.value[messageId]) {
    return audioBlobUrls.value[messageId]
  }
  
  try {
    // Use axios to get audio data (will automatically carry auth token)
    const response = await api.get(`/user/history/messages/${messageId}/audio`, {
      responseType: 'blob' // Important: specify response type as blob
    })
    
    // Create Blob URL
    const blobUrl = URL.createObjectURL(response.data)
    audioBlobUrls.value[messageId] = blobUrl
    
    return blobUrl
  } catch (error) {
    // Silent handling, only log, don't show error
    console.warn('Failed to load audio:', messageId, error)
    return null
  }
}


// Preload audio messages
const preloadAudioMessages = async () => {
  const audioMessages = messages.value.filter(msg => msg.audio_path)
  // Concurrent preload, but limit concurrency
  const promises = audioMessages.slice(0, 10).map(msg => getAudioUrl(msg.id).catch(err => {
    console.warn('Failed to preload audio:', msg.id, err)
    return null
  }))
  await Promise.all(promises)
}

// Audio playback ended
const handleAudioEnded = (messageId) => {
  playingAudioId.value = null
}

// Audio load error handling
const handleAudioError = async (messageId) => {
  // Silent handling, only log, don't show error
  console.warn('Audio load failed:', messageId)
  // Try to reload
  try {
    const url = await getAudioUrl(messageId)
    if (url) {
      const audio = audioRefs.value[messageId]
      if (audio) {
        audio.load() // Reload audio
      }
    }
  } catch (error) {
    // Silent handling, only log
    console.warn('Audio reload failed:', messageId, error)
  }
}

// Toggle audio playback
const toggleAudio = async (messageId) => {
  const audio = audioRefs.value[messageId]
  if (!audio) return

  // If audio not loaded yet, load first
  if (!audioBlobUrls.value[messageId]) {
    const url = await getAudioUrl(messageId)
    if (!url) {
      // Silent handling, only log, don't show error
      console.warn('Audio load failed, cannot play:', messageId)
      return
    }
    // Wait for audio element to load
    await new Promise((resolve) => {
      audio.onloadeddata = resolve
      audio.load()
    })
  }

  // Stop other audios
  if (playingAudioId.value && playingAudioId.value !== messageId) {
    const otherAudio = audioRefs.value[playingAudioId.value]
    if (otherAudio) {
      otherAudio.pause()
      otherAudio.currentTime = 0
    }
  }

  if (playingAudioId.value === messageId) {
    // Pause current audio
    audio.pause()
    playingAudioId.value = null
  } else {
    // Play audio
    try {
      await audio.play()
      playingAudioId.value = messageId
    } catch (error) {
      // Silent handling, only log, don't show error
      console.warn('Failed to play audio:', messageId, error)
    }
  }
}


onMounted(async () => {
  if (!agentId.value) {
    ElMessage.error('Invalid agent ID')
    router.push('/user/agents')
    return
  }
  try {
    await Promise.all([
      loadAgent(),
      loadDevices(),
      loadMessages()
    ])
  } catch (error) {
    console.error('Initialization failed:', error)
  }
})

// Clean up Blob URLs on component unmount to avoid memory leaks
onBeforeUnmount(() => {
  Object.values(audioBlobUrls.value).forEach(url => {
    if (url) {
      URL.revokeObjectURL(url)
    }
  })
  audioBlobUrls.value = {}
})
</script>

<style scoped>
.agent-history-page {
  padding: 20px;
  background: #f5f5f5;
  min-height: 100vh;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 20px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-info h1 {
  margin: 0;
  font-size: 24px;
  color: #303133;
}

.page-subtitle {
  margin: 4px 0 0 0;
  color: #909399;
  font-size: 14px;
}

.filter-card {
  margin-bottom: 20px;
}

.messages-card {
  min-height: 400px;
}

.empty-state {
  padding: 60px 0;
  text-align: center;
}

/* WeChat style chat container */
.chat-container {
  background: #ededed;
  min-height: 500px;
  border-radius: 8px;
  overflow: hidden;
}

.chat-messages {
  padding: 20px;
  max-height: 70vh;
  overflow-y: auto;
}

.message-wrapper {
  display: flex;
  flex-direction: column;
  margin-bottom: 16px;
}

.message-time-divider {
  text-align: center;
  margin: 16px 0;
  font-size: 12px;
  color: #999;
}

.message-bubble-wrapper {
  display: flex;
  align-items: flex-start;
  max-width: 75%;
}

.message-right {
  margin-left: auto;
  justify-content: flex-end;
  width: 100%;
  display: flex;
}

.message-left {
  margin-right: auto;
  justify-content: flex-start;
  width: 100%;
  display: flex;
}

/* Message bubble */
.message-bubble {
  position: relative;
  padding: 10px 14px;
  border-radius: 8px;
  word-wrap: break-word;
  word-break: break-word;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  max-width: 100%;
}

.message-bubble-left {
  background: white;
  border-top-left-radius: 0;
}

.message-bubble-right {
  background: #95ec69;
  border-top-right-radius: 0;
  margin-left: auto;
}

.message-content-wrapper {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.message-text {
  color: #333;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 14px;
}

.message-bubble-right .message-text {
  color: #000;
}

/* Audio bubble */
.audio-bubble {
  margin: 4px 0;
  display: flex;
  align-items: center;
}

.audio-play-btn-simple {
  flex-shrink: 0;
}

/* Message meta */
.message-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  opacity: 0.7;
}

.message-meta:hover {
  opacity: 1;
}

.message-time-small {
  font-size: 11px;
  color: #999;
}

.message-bubble-right .message-time-small {
  color: #666;
}

.message-more {
  font-size: 14px;
  color: #999;
  cursor: pointer;
  padding: 2px;
  border-radius: 4px;
  transition: all 0.2s;
}

.message-more:hover {
  background: rgba(0, 0, 0, 0.1);
  color: #666;
}

.message-bubble-right .message-more {
  color: #666;
}

.message-bubble-right .message-more:hover {
  background: rgba(0, 0, 0, 0.15);
}

/* Pagination */
.pagination {
  margin-top: 20px;
  padding: 20px;
  display: flex;
  justify-content: center;
  background: white;
  border-top: 1px solid #e4e7ed;
}

/* Scrollbar style */
.chat-messages::-webkit-scrollbar {
  width: 6px;
}

.chat-messages::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.chat-messages::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.chat-messages::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* Element Plus component style overrides */
:deep(.el-slider__runway) {
  margin: 0;
  height: 4px;
}

:deep(.el-slider__bar) {
  height: 4px;
}

:deep(.el-slider__button) {
  width: 12px;
  height: 12px;
  border: 2px solid #409eff;
}

:deep(.el-slider__button-wrapper) {
  width: 24px;
  height: 24px;
  top: -10px;
}

:deep(.el-dropdown-menu__item) {
  padding: 8px 20px;
}
</style>

