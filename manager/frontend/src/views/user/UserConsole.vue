<template>
  <div class="user-console">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-bg"></div>
      <div class="header-content">
        <div class="welcome-section">
          <div class="avatar-section">
            <div class="user-avatar">
              <el-icon><Avatar /></el-icon>
            </div>
            <div class="welcome-text">
              <h1 class="welcome-title">Welcome Back!</h1>
              <p class="welcome-subtitle">Manage your smart devices and AI assistants</p>
            </div>
          </div>
          <div class="quick-stats">
            <div class="stat-item online">
              <div class="stat-icon">
                <el-icon><Connection /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ onlineDevicesCount }}</div>
                <div class="stat-label">Online Devices</div>
              </div>
            </div>
            <div class="stat-item agents">
              <div class="stat-icon">
                <el-icon><Monitor /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ agents.length }}</div>
                <div class="stat-label">Agents</div>
              </div>
            </div>
            <div class="stat-item active">
              <div class="stat-icon">
                <el-icon><CircleCheck /></el-icon>
              </div>
              <div class="stat-info">
                <div class="stat-number">{{ activeAgentsCount }}</div>
                <div class="stat-label">Active Assistants</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Main Content Area -->
    <div class="main-content">
      <!-- Device Management -->
      <div class="content-section">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="title-icon"><Cpu /></el-icon>
            <span>Smart Devices</span>
            <span class="device-count">{{ allDevicesData.length }}</span>
          </div>
          <div class="action-buttons">
            <el-button type="success" @click="openInjectMessageDialog" class="add-btn">
              <el-icon><ChatDotRound /></el-icon>
              Message Injection
            </el-button>
            <el-button type="primary" @click="addDevice" class="add-btn">
              <el-icon><Plus /></el-icon>
              Add Device
            </el-button>
          </div>
        </div>
        
        <div v-if="devices.length === 0" class="empty-container">
          <div class="empty-content">
            <div class="empty-icon">
              <el-icon><Monitor /></el-icon>
            </div>
            <h3>No devices yet</h3>
            <p>Add your first smart device to start your AI interaction journey</p>
            <el-button type="primary" size="large" @click="addDevice">
              <el-icon><Plus /></el-icon>
              Add Device
            </el-button>
          </div>
        </div>
        
        <div v-else class="devices-grid">
          <div v-for="device in devices" :key="device.id" class="device-item">
            <div class="device-card">
              <div class="device-status">
                <div class="status-indicator" :class="isDeviceOnline(device.last_active_at) ? 'online' : 'offline'"></div>
                <span class="status-text">{{ isDeviceOnline(device.last_active_at) ? 'Online' : 'Offline' }}</span>
              </div>
              
              <div class="device-info">
                <div class="device-icon">
                  <el-icon><Monitor /></el-icon>
                </div>
                <div class="device-details">
                  <h3 class="device-name">{{ device.device_name || 'Unnamed Device' }}</h3>
                  <p class="device-desc">{{ device.device_code }}</p>
                </div>
              </div>
              
              <div class="device-features">
                <div class="feature-item">
                  <el-icon class="feature-icon"><Microphone /></el-icon>
                  <span class="feature-label">Voice Recognition</span>
                  <el-switch 
                    v-model="device.vad_status" 
                    @change="toggleVAD(device)"
                    :loading="device.loading"
                    size="small"
                  />
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><User /></el-icon>
                  <span class="feature-label">Agent</span>
                  <span class="feature-value">{{ device.agent_name || 'Not Bound' }}</span>
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><CircleCheck /></el-icon>
                  <span class="feature-label">Activation Status</span>
                  <span class="feature-value">
                    <el-tag :type="device.activated ? 'success' : 'warning'" size="small">
                      {{ device.activated ? 'Activated' : 'Not Activated' }}
                    </el-tag>
                  </span>
                </div>
                
                <div class="feature-item">
                  <el-icon class="feature-icon"><Clock /></el-icon>
                  <span class="feature-label">Last Active</span>
                  <span class="feature-value">{{ formatTime(device.last_active_at) }}</span>
                </div>
              </div>
              
              <div class="device-actions">
                <el-button type="primary" size="small" @click="openDeviceControl(device)" class="control-btn">
                  <el-icon><Setting /></el-icon>
                  Control Panel
                </el-button>
              </div>
            </div>
          </div>
        </div>
        
        <!-- View More -->
        <div v-if="allDevicesData.length > 6" class="load-more">
          <el-button 
            type="text" 
            @click="toggleShowAllDevices"
            class="load-more-btn"
          >
            <span v-if="!showAllDevices">
              Show All Devices ({{ allDevicesData.length - 6 }}+)
              <el-icon><ArrowDown /></el-icon>
            </span>
            <span v-else>
              Collapse Device List
              <el-icon><ArrowUp /></el-icon>
            </span>
          </el-button>
        </div>
      </div>

      <!-- Agent Management -->
      <div class="content-section">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="title-icon"><User /></el-icon>
            <span>AI Agents</span>
            <span class="device-count">{{ agents.length }}</span>
          </div>
          <el-button type="primary" @click="$router.push('/agents')" class="add-btn">
            <el-icon><Setting /></el-icon>
            Manage Agents
          </el-button>
        </div>
        
        <div v-if="agents.length === 0" class="empty-container">
          <div class="empty-content">
            <div class="empty-icon">
              <el-icon><User /></el-icon>
            </div>
            <h3>No agents yet</h3>
            <p>Create your exclusive AI assistant and enjoy personalized service</p>
            <el-button type="primary" size="large" @click="$router.push('/agents')">
              <el-icon><Plus /></el-icon>
              Create Agent
            </el-button>
          </div>
        </div>
        
        <div v-else class="agents-grid">
          <div v-for="agent in agents.slice(0, 4)" :key="agent.id" class="agent-item">
            <div class="agent-card" @click="selectAgent(agent)">
              <div class="agent-status">
                <div class="status-indicator" :class="agent.status === 'active' ? 'online' : 'offline'"></div>
                <span class="status-text">{{ agent.status === 'active' ? 'Active' : 'Standby' }}</span>
              </div>
              
              <div class="agent-avatar">
                <div class="avatar-bg" :class="getAgentAvatarClass(agent)">
                  <el-icon><User /></el-icon>
                </div>
              </div>
              
              <div class="agent-info">
                <h3 class="agent-name">{{ agent.name }}</h3>
                <p class="agent-desc">{{ agent.description || 'Intelligent AI Assistant' }}</p>
              </div>
              
              <div class="agent-stats">
                <div class="stat-row">
                  <span class="stat-label">Conversations</span>
                  <span class="stat-value">{{ agent.conversation_count || 0 }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">Created</span>
                  <span class="stat-value">{{ formatDate(agent.created_at) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div v-if="agents.length > 4" class="load-more">
          <el-button type="text" @click="$router.push('/agents')" class="load-more-btn">
            View All Agents ({{ agents.length - 4 }}+)
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </div>
    </div>



    <!-- Device Control Dialog -->
    <el-dialog
      v-model="showDeviceControl"
      :title="`Control Device: ${currentDevice?.name}`"
      width="600px"
    >
      <div v-if="currentDevice" class="device-control-panel">
        <div class="control-section">
          <h4>Basic Control</h4>
          <div class="control-buttons">
            <el-button type="success" @click="sendCommand('wake_up')">
              <el-icon><VideoPlay /></el-icon>
              Wake Device
            </el-button>
            <el-button type="warning" @click="sendCommand('sleep')">
              <el-icon><VideoPause /></el-icon>
              Sleep Device
            </el-button>
            <el-button type="info" @click="sendCommand('restart')">
              <el-icon><Refresh /></el-icon>
              Restart Device
            </el-button>
          </div>
        </div>
        
        <div class="control-section">
          <h4>Voice Control</h4>
          <div class="voice-settings">
            <el-form label-width="100px">
              <el-form-item label="Volume">
                <el-slider v-model="currentDevice.volume" :max="100" />
              </el-form-item>
              <el-form-item label="Voice Recognition">
                <el-switch 
                  v-model="currentDevice.vad_status"
                  @change="toggleVAD(currentDevice)"
                />
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- Message Injection Dialog -->
    <el-dialog
      v-model="showInjectMessageDialog"
      title="Message Injection"
      width="600px"
      class="inject-message-dialog"
      :close-on-click-modal="false"
    >
      <el-form
        ref="injectFormRef"
        :model="injectForm"
        :rules="injectRules"
        label-width="100px"
      >
        <el-form-item label="Select Device" prop="device_id">
          <el-select
            v-model="injectForm.device_id"
            placeholder="Please select device to inject message"
            style="width: 100%"
            popper-class="inject-device-select-popper"
            filterable
          >
            <el-option
              v-for="device in allDevicesData"
              :key="device.device_code"
              :label="`${device.device_name || 'Unnamed Device'} (${device.device_code})`"
              :value="device.device_name || 'Unnamed Device'"
            >
              <div class="device-option">
                <div class="device-option-header">
                  <span class="device-name">{{ device.device_name || 'Unnamed Device' }}</span>
                  <el-tag 
                    :type="isDeviceOnline(device.last_active_at) ? 'success' : 'danger'" 
                    size="small"
                  >
                    {{ isDeviceOnline(device.last_active_at) ? 'Online' : 'Offline' }}
                  </el-tag>
                </div>
                <div class="device-code">{{ device.device_code }}</div>
                <div class="device-agent">Agent: {{ device.agent_name || 'Not Bound' }}</div>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        
        <el-form-item label="Message Content" prop="message">
          <el-input
            v-model="injectForm.message"
            type="textarea"
            :rows="4"
            placeholder="Please enter message content to inject"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
        
        <el-form-item label="Processing Mode" prop="skip_llm">
          <el-radio-group v-model="injectForm.skip_llm">
            <el-radio :label="false">
              <div class="radio-option">
                <div class="radio-title">Process via LLM</div>
                <div class="radio-desc">Message will be processed by AI agent to generate intelligent response</div>
              </div>
            </el-radio>
            <el-radio :label="true">
              <div class="radio-option">
                <div class="radio-title">Direct Playback</div>
                <div class="radio-desc">Message will be directly converted to voice playback without AI processing</div>
              </div>
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseInjectMessage">Cancel</el-button>
          <el-button
            type="primary"
            :loading="injectingMessage"
            @click="handleInjectMessage"
          >
            {{ injectingMessage ? 'Injecting...' : 'Inject Message' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Add Device Dialog -->
    <el-dialog
      v-model="showAddDeviceDialog"
      title="Add Device"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="deviceFormRef"
        :model="deviceForm"
        :rules="deviceRules"
        label-width="100px"
      >
        <el-form-item label="Device Name" prop="device_name">
          <el-input
            v-model="deviceForm.device_name"
            placeholder="Please enter device name"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
        
        <el-form-item label="Associated Agent" prop="agent_id">
          <el-select
            v-model="deviceForm.agent_id"
            placeholder="Please select associated agent"
            style="width: 100%"
          >
            <el-option
              v-for="agent in agents"
              :key="agent.id"
              :label="agent.name"
              :value="agent.id"
            >
              <div class="agent-option">
                <span class="agent-name">{{ agent.name }}</span>
                <span class="agent-desc">{{ agent.description || 'Intelligent AI Assistant' }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseAddDevice">Cancel</el-button>
          <el-button
            type="primary"
            :loading="addingDevice"
            @click="handleAddDevice"
          >
            {{ addingDevice ? 'Adding...' : 'Add Device' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Monitor,
  Connection,
  Plus,
  Setting,
  Microphone,
  VideoPlay,
  VideoPause,
  Refresh,
  ArrowDown,
  ArrowUp,
  ArrowRight,
  Avatar,
  CircleCheck,
  Cpu,
  User,
  Clock,
  ChatDotRound
} from '@element-plus/icons-vue'
import api from '../../utils/api'

const devices = ref([])
const agents = ref([])
const allDevicesData = ref([])
const showDeviceControl = ref(false)
const currentDevice = ref(null)
const showAllDevices = ref(false)

// Load device list
const loadDevices = async () => {
  try {
    const response = await api.get('/user/devices')
    const allDevices = response.data.data || []
    // Save all device data
    allDevicesData.value = allDevices.map(device => ({
      ...device,
      loading: false,
      volume: device.volume || 80
    }))
    // Limit display to max 6 devices
    devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
    // Update statistics
    updateStats()
  } catch (error) {
    console.error('Failed to load devices:', error)
    ElMessage.error('Failed to load devices')
    devices.value = []
    allDevicesData.value = []
  }
}

// Load agent list
const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
    // Update statistics
    updateStats()
  } catch (error) {
    console.error('Failed to load agents:', error)
    ElMessage.error('Failed to load agents')
    agents.value = []
  }
}

// Toggle voice recognition status
const toggleVAD = async (device) => {
  device.loading = true
  try {
    // Simulate API call
    await new Promise(resolve => setTimeout(resolve, 1000))
    device.vad_status = !device.vad_status
    ElMessage.success(`${device.vad_status ? 'Enabled' : 'Disabled'} voice recognition successfully`)
  } catch (error) {
    console.error('Failed to toggle voice recognition:', error)
    ElMessage.error('Operation failed')
  } finally {
    device.loading = false
  }
}

// Open device control panel
const openDeviceControl = (device) => {
  currentDevice.value = device
  showDeviceControl.value = true
}

// Send device command
const sendCommand = async (command) => {
  try {
    // Simulate API call
    await new Promise(resolve => setTimeout(resolve, 500))
    ElMessage.success(`Command ${command} sent successfully`)
  } catch (error) {
    console.error('Failed to send command:', error)
    ElMessage.error('Failed to send command')
  }
}

// Select agent
const selectAgent = (agent) => {
  ElMessage.info(`Selected agent: ${agent.name}`)
  // Can navigate to agent detail page or perform other actions
}

// Add device related state
const showAddDeviceDialog = ref(false)
const addingDevice = ref(false)
const deviceFormRef = ref()

// Message injection related state
const showInjectMessageDialog = ref(false)
const injectingMessage = ref(false)
const injectFormRef = ref()

const deviceForm = reactive({
  device_name: '',
  agent_id: ''
})

const deviceRules = {
  device_name: [
    { required: true, message: 'Please enter device name', trigger: 'blur' },
    { min: 2, max: 50, message: 'Device name must be between 2-50 characters', trigger: 'blur' }
  ],
  agent_id: [
    { required: true, message: 'Please select associated agent', trigger: 'change' }
  ]
}

const injectForm = reactive({
  device_id: '',
  message: '',
  skip_llm: false
})

const injectRules = {
  device_id: [
    { required: true, message: 'Please select device', trigger: 'change' }
  ],
  message: [
    { required: true, message: 'Please enter message content', trigger: 'blur' },
    { min: 1, max: 500, message: 'Message must be between 1-500 characters', trigger: 'blur' }
  ]
}

// Open add device dialog
const addDevice = () => {
  if (agents.value.length === 0) {
    ElMessage.warning('Please create an agent first, then add a device')
    return
  }
  showAddDeviceDialog.value = true
}

// Handle add device
const handleAddDevice = async () => {
  if (!deviceFormRef.value) return
  
  try {
    await deviceFormRef.value.validate()
    addingDevice.value = true
    
    const response = await api.post('/user/devices', {
      device_name: deviceForm.device_name,
      agent_id: parseInt(deviceForm.agent_id)
    })
    
    if (response.data.success) {
      ElMessage.success('Device added successfully')
      handleCloseAddDevice()
      await loadDevices()
    }
  } catch (error) {
    console.error('Failed to add device:', error)
    ElMessage.error(error.response?.data?.error || 'Failed to add device')
  } finally {
    addingDevice.value = false
  }
}

// Close add device dialog
const handleCloseAddDevice = () => {
  showAddDeviceDialog.value = false
  if (deviceFormRef.value) {
    deviceFormRef.value.resetFields()
  }
  Object.assign(deviceForm, { device_name: '', agent_id: '' })
}

// Open message injection dialog
const openInjectMessageDialog = () => {
  if (allDevicesData.value.length === 0) {
    ElMessage.warning('Please add a device first, then inject messages')
    return
  }
  
  showInjectMessageDialog.value = true
}

// Handle message injection
const handleInjectMessage = async () => {
  if (!injectFormRef.value) return
  
  try {
    await injectFormRef.value.validate()
    injectingMessage.value = true
    
    const response = await api.post('/user/devices/inject-message', {
      device_id: injectForm.device_id,
      message: injectForm.message,
      skip_llm: injectForm.skip_llm
    })
    
    if (response.data.success) {
      ElMessage.success('Message injected successfully')
      handleCloseInjectMessage()
    }
  } catch (error) {
    console.error('Message injection failed:', error)
    ElMessage.error(error.response?.data?.error || 'Message injection failed')
  } finally {
    injectingMessage.value = false
  }
}

// Close message injection dialog
const handleCloseInjectMessage = () => {
  showInjectMessageDialog.value = false
  if (injectFormRef.value) {
    injectFormRef.value.resetFields()
  }
  Object.assign(injectForm, { device_id: '', message: '', skip_llm: false })
}

// Toggle show all devices
const toggleShowAllDevices = () => {
  showAllDevices.value = !showAllDevices.value
  devices.value = showAllDevices.value ? allDevicesData.value : allDevicesData.value.slice(0, 6)
}

// Computed properties
const onlineDevicesCount = ref(0)
const activeAgentsCount = ref(0)

// Check if device is online (based on last active time)
const isDeviceOnline = (lastActiveAt) => {
  if (!lastActiveAt) return false
  const now = new Date()
  const lastActive = new Date(lastActiveAt)
  // Consider online if active within 5 minutes
  return (now - lastActive) < 5 * 60 * 1000
}

// Update statistics
const updateStats = () => {
  onlineDevicesCount.value = allDevicesData.value.filter(device => isDeviceOnline(device.last_active_at)).length
  activeAgentsCount.value = agents.value.filter(agent => agent.status === 'active').length
}

// Get agent avatar class
const getAgentAvatarClass = (agent) => {
  const classes = ['avatar-blue', 'avatar-green', 'avatar-purple', 'avatar-orange']
  return classes[agent.id % classes.length] || 'avatar-blue'
}

// Format time
const formatTime = (date) => {
  if (!date) return 'Unknown'
  const now = new Date()
  const diff = now - new Date(date)
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  if (minutes < 1) return 'Just now'
  if (minutes < 60) return `${minutes} minutes ago`
  if (hours < 24) return `${hours} hours ago`
  if (days < 30) return `${days} days ago`
  return `${Math.floor(days / 30)} months ago`
}

// Format date
const formatDate = (dateString) => {
  if (!dateString) return '--'
  return new Date(dateString).toLocaleDateString('en-US')
}

onMounted(() => {
  loadDevices()
  loadAgents()
})
</script>

<style scoped>
.user-console {
  min-height: 100vh;
  background: #f8f9fa;
  padding: 0;
  overflow-x: hidden;
}

/* Page Header Styles */
.page-header {
  background: #ffffff;
  padding: 24px 0;
  margin-bottom: 0;
  border-bottom: 1px solid #e9ecef;
}

.header-bg {
  display: none;
}

.header-content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
  color: #495057;
}

.welcome-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 40px;
}

.avatar-section {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-avatar {
  width: 48px;
  height: 48px;
  background: #e9ecef;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: #6c757d;
  border: 1px solid #dee2e6;
}

.welcome-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: #212529;
}

.welcome-subtitle {
  font-size: 14px;
  margin: 0;
  color: #6c757d;
}

.quick-stats {
  display: flex;
  gap: 20px;
}

.quick-stats .stat-item {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #ffffff;
  padding: 16px 20px;
  border-radius: 8px;
  border: 1px solid #dee2e6;
}

.quick-stats .stat-icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: #ffffff;
}

.quick-stats .stat-item.online .stat-icon {
  background: #28a745;
}

.quick-stats .stat-item.agents .stat-icon {
  background: #007bff;
}

.quick-stats .stat-item.active .stat-icon {
  background: #17a2b8;
}

.quick-stats .stat-number {
  font-size: 18px;
  font-weight: 600;
  line-height: 1;
  color: #212529;
}

.quick-stats .stat-label {
  font-size: 12px;
  color: #6c757d;
  margin-top: 2px;
}

/* Main Content Area */
.main-content {
  max-width: 1200px;
  margin: 24px auto 40px;
  padding: 0 24px;
}

.content-section {
  background: white;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 24px;
  border: 1px solid #dee2e6;
}

/* Section Header */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e9ecef;
}

.action-buttons {
  display: flex;
  gap: 12px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  color: #212529;
}

.title-icon {
  width: 24px;
  height: 24px;
  background: #007bff;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 12px;
}

.device-count {
  background: #f8f9fa;
  color: #6c757d;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  margin-left: 8px;
}

.add-btn {
  height: 36px;
  padding: 0 16px;
  border-radius: 4px;
  font-weight: 500;
  font-size: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.add-btn :deep(.el-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  line-height: 1;
}

/* Device Grid */
.devices-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px 12px;
}

.device-item {
  position: relative;
}

.device-card {
  background: white;
  border-radius: 6px;
  padding: 16px;
  border: 1px solid #dee2e6;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.device-card:hover {
  border-color: #007bff;
}

.device-status {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.online {
  background: #28a745;
}

.status-indicator.offline {
  background: #dc3545;
}

.status-text {
  font-size: 12px;
  font-weight: 500;
  color: #6c757d;
}

.device-info {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.device-icon {
  width: 40px;
  height: 40px;
  background: #007bff;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 18px;
}

.device-name {
  font-size: 16px;
  font-weight: 600;
  color: #212529;
  margin: 0 0 2px 0;
}

.device-desc {
  font-size: 12px;
  color: #6c757d;
  margin: 0;
}

.device-features {
  flex: 1;
  margin-bottom: 16px;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid #f8f9fa;
}

.feature-item:last-child {
  border-bottom: none;
}

.feature-icon {
  width: 20px;
  height: 20px;
  background: #f8f9fa;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #007bff;
  font-size: 12px;
}

.feature-label {
  font-size: 12px;
  color: #6c757d;
  flex: 1;
}

.feature-value {
  font-size: 12px;
  color: #212529;
  font-weight: 500;
}

.device-actions {
  margin-top: auto;
}

.control-btn {
  width: 100%;
  height: 32px;
  border-radius: 4px;
  font-weight: 500;
  font-size: 12px;
}

/* Agent Grid */
.agents-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px 12px;
}

.agent-item {
  position: relative;
}

.agent-card {
  background: white;
  border-radius: 6px;
  padding: 16px;
  border: 1px solid #dee2e6;
  cursor: pointer;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.agent-card:hover {
  border-color: #007bff;
}

.agent-status {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.agent-avatar {
  display: flex;
  justify-content: center;
  margin-bottom: 12px;
}

.avatar-bg {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 20px;
}

.avatar-bg.avatar-blue {
  background: #007bff;
}

.avatar-bg.avatar-green {
  background: #28a745;
}

.avatar-bg.avatar-purple {
  background: #6f42c1;
}

.avatar-bg.avatar-orange {
  background: #fd7e14;
}

.agent-info {
  text-align: center;
  margin-bottom: 12px;
}

.agent-name {
  font-size: 14px;
  font-weight: 600;
  color: #212529;
  margin: 0 0 4px 0;
}

.agent-desc {
  font-size: 12px;
  color: #6c757d;
  margin: 0;
  line-height: 1.4;
}

.agent-stats {
  flex: 1;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid #f8f9fa;
}

.stat-row:last-child {
  border-bottom: none;
}

.stat-label {
  font-size: 12px;
  color: #6c757d;
}

.stat-value {
  font-size: 12px;
  color: #212529;
  font-weight: 500;
}

/* Empty State */
.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
}

.empty-content {
  text-align: center;
  max-width: 300px;
}

.empty-icon {
  width: 64px;
  height: 64px;
  background: #f8f9fa;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #adb5bd;
  margin: 0 auto 16px;
}

.empty-content h3 {
  font-size: 16px;
  color: #212529;
  margin: 0 0 8px 0;
  font-weight: 600;
}

.empty-content p {
  font-size: 14px;
  color: #6c757d;
  margin: 0 0 16px 0;
  line-height: 1.4;
}

/* Load More */
.load-more {
  text-align: center;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #e9ecef;
}

.load-more-btn {
  font-size: 14px;
  color: #007bff;
  font-weight: 500;
}

.load-more-btn:hover {
  color: #0056b3;
}

/* Responsive Design */
@media (max-width: 1024px) {
  .welcome-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  
  .quick-stats {
    flex-wrap: wrap;
    gap: 12px;
  }
  
  .devices-grid {
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  }
}

@media (max-width: 768px) {
  .user-console {
    min-height: auto;
    padding-bottom: calc(72px + env(safe-area-inset-bottom));
  }

  .page-header {
    padding: 16px 0;
  }
  
  .header-content {
    padding: 0 16px;
  }
  
  .welcome-title {
    font-size: 20px;
  }

  .quick-stats {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .quick-stats .stat-item {
    width: 100%;
    min-width: 0;
    padding: 12px 10px;
    gap: 8px;
  }

  .quick-stats .stat-number {
    font-size: 16px;
  }

  .quick-stats .stat-label {
    white-space: nowrap;
    font-size: 11px;
  }
  
  .main-content {
    margin: 16px auto 24px;
    padding: 0 16px;
  }
  
  .content-section {
    padding: 16px;
    margin-bottom: 16px;
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .action-buttons {
    width: 100%;
    flex-wrap: wrap;
    gap: 8px;
  }

  .action-buttons .add-btn,
  .section-header > .add-btn {
    flex: 1;
    min-width: 120px;
  }

  .feature-item {
    align-items: flex-start;
    gap: 6px;
  }

  .feature-label {
    min-width: 56px;
    flex: none;
  }

  .feature-value {
    flex: 1;
    min-width: 0;
    text-align: right;
    word-break: break-all;
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 8px;
  }

  :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
    margin-top: 8vh !important;
  }

  :deep(.el-dialog__body) {
    max-height: 65vh;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }

  :deep(.el-dialog__header) {
    margin-right: 0;
  }

  :deep(.el-form-item__label) {
    line-height: 20px;
    padding-bottom: 4px;
  }
}

/* Agent Option Styles */
.agent-option {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.agent-option .agent-name {
  font-weight: 500;
  color: #212529;
}

.agent-option .agent-desc {
  font-size: 12px;
  color: #6c757d;
}

/* Dialog Styles */
.dialog-footer {
  text-align: right;
}

/* Message Injection Dialog Styles */
.device-option {
  padding: 8px 0;
}

.device-option-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.device-option .device-name {
  font-weight: 500;
  color: #212529;
}

.device-code {
  font-size: 12px;
  color: #6c757d;
  margin-bottom: 2px;
}

.device-agent {
  font-size: 12px;
  color: #6c757d;
}

.radio-option {
  margin-left: 0;
}

.radio-title {
  font-weight: 500;
  color: #212529;
  margin-bottom: 2px;
}

.radio-desc {
  font-size: 12px;
  color: #6c757d;
  line-height: 1.4;
  word-break: break-word;
}

:deep(.inject-device-select-popper .el-select-dropdown__item) {
  height: auto;
  line-height: 1.4;
  padding-top: 8px;
  padding-bottom: 8px;
  white-space: normal;
}

:deep(.inject-device-select-popper .device-option) {
  padding: 0;
}

:deep(.inject-message-dialog .el-radio-group) {
  display: flex;
  flex-direction: column;
  width: 100%;
  gap: 10px;
}

:deep(.inject-message-dialog .el-radio) {
  margin-right: 0;
  align-items: flex-start;
  height: auto;
  line-height: 1.4;
}

:deep(.inject-message-dialog .el-radio__input) {
  margin-top: 2px;
}

:deep(.inject-message-dialog .el-radio__label) {
  display: block;
  padding-left: 8px;
  white-space: normal;
  line-height: 1.4;
}

:deep(.inject-message-dialog .el-form-item__content) {
  min-width: 0;
}

@media (max-width: 480px) {
  .avatar-section {
    align-items: flex-start;
  }

  .welcome-title {
    font-size: 18px;
  }

  .welcome-subtitle {
    font-size: 13px;
  }

  .quick-stats .stat-item {
    padding: 10px 8px;
    gap: 6px;
  }

  .quick-stats .stat-icon {
    width: 24px;
    height: 24px;
    font-size: 12px;
  }

  .quick-stats .stat-number {
    font-size: 14px;
  }

  .quick-stats .stat-label {
    font-size: 10px;
  }

  .section-title {
    font-size: 16px;
  }

  .action-buttons .add-btn,
  .section-header > .add-btn {
    width: 100%;
    margin-left: 0;
  }

  .radio-option {
    margin-left: 0;
  }

  .radio-desc {
    white-space: normal;
  }
  
  .devices-grid {
    grid-template-columns: 1fr;
    gap: 10px;
  }
  
  .agents-grid {
    grid-template-columns: 1fr;
  }
}
</style>
