<template>
  <div class="speakers-page">
    <div class="page-header">
      <div class="header-left">
        <h2>Voiceprint Management</h2>
        <p class="page-subtitle">Manage your voiceprint recognition configuration</p>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="handleAddGroup">
          <el-icon><Plus /></el-icon>
          Create Voiceprint Group
        </el-button>
      </div>
    </div>

    <!-- Filter Bar -->
    <div class="filter-bar">
      <el-select
        v-model="filterAgentId"
        placeholder="Filter by Agent"
        clearable
        style="width: 200px; margin-right: 10px;"
        @change="loadSpeakerGroups"
      >
        <el-option label="All Agents" value="" />
        <el-option
          v-for="agent in agents"
          :key="agent.id"
          :label="agent.name"
          :value="agent.id"
        />
      </el-select>
      <el-input
        v-model="searchKeyword"
        placeholder="Search voiceprint group name"
        clearable
        style="width: 250px;"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      </div>

    <!-- Voiceprint Group List -->
    <div v-loading="loading" class="speakers-content">
      <el-table :data="filteredGroups" stripe style="width: 100%">
        <el-table-column prop="name" label="Voiceprint Group Name" min-width="150" />
        <el-table-column prop="agent_name" label="Associated Agent" min-width="120" />
        <el-table-column label="Prompt" min-width="200">
          <template #default="{ row }">
            <el-popover
              placement="top"
              :width="300"
              trigger="hover"
              v-if="row.prompt"
            >
              <template #reference>
                <span class="prompt-text">{{ truncateText(row.prompt, 30) }}</span>
              </template>
              <div class="prompt-popover">{{ row.prompt }}</div>
            </el-popover>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="sample_count" label="Sample Count" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info">{{ row.sample_count }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="Created At" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="360" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
            <el-button
              type="success"
              size="small"
              @click="handleVerifyGroup(row)"
            >
              <el-icon><VideoPlay /></el-icon>
              Verify
            </el-button>
            <el-button
              type="primary"
              size="small"
              @click="handleViewSamples(row)"
            >
              <el-icon><View /></el-icon>
              Manage Voiceprints
            </el-button>
              <el-button
                type="primary"
                size="small"
                plain
                @click="handleEditGroup(row)"
              >
                  <el-icon><Edit /></el-icon>
                  Edit
                </el-button>
              <el-button
                type="danger"
                size="small"
                @click="handleDeleteGroup(row)"
              >
                <el-icon><Delete /></el-icon>
                Delete
                </el-button>
              </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="filteredGroups.length === 0 && !loading" class="empty-state">
        <el-empty description="No voiceprint group data" />
      </div>
    </div>

    <!-- Create/Edit Voiceprint Group Dialog -->
    <el-dialog
      v-model="showGroupDialog"
      :title="groupDialogMode === 'add' ? 'Create Voiceprint Group' : 'Edit Voiceprint Group'"
      width="600px"
    >
      <el-form
        ref="groupFormRef"
        :model="groupForm"
        :rules="groupRules"
        label-width="100px"
      >
        <el-form-item label="Associated Agent" prop="agent_id">
          <el-select
            v-model="groupForm.agent_id"
            placeholder="Please select agent"
            style="width: 100%"
          >
            <el-option
              v-for="agent in agents"
              :key="agent.id"
              :label="agent.name"
              :value="agent.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Voiceprint Name" prop="name">
          <el-input
            v-model="groupForm.name"
            placeholder="Please enter voiceprint name"
            :maxlength="100"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="Prompt" prop="prompt">
          <el-input
            v-model="groupForm.prompt"
            type="textarea"
            :rows="4"
            placeholder="Please enter role prompt (optional)"
          />
        </el-form-item>
        <el-form-item label="Description" prop="description">
          <el-input
            v-model="groupForm.description"
            type="textarea"
            :rows="3"
            placeholder="Please enter description (optional)"
            :maxlength="200"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="My Cloned Voices" v-if="cloneVoicePresets.length > 0">
          <div class="clone-voice-line" v-loading="cloneVoicesLoading">
            <button
              v-for="clone in cloneVoicePresets"
              :key="clone.id"
              type="button"
              class="clone-voice-item"
              :class="{ active: isCloneVoiceSelected(clone) }"
              :title="`${clone.tts_config_name || clone.tts_config_id} · ${clone.provider_voice_id}`"
              @click="applyCloneVoice(clone)"
            >
              <span class="clone-voice-name">{{ clone.name || clone.provider_voice_id }}</span>
            </button>
          </div>
          <div class="form-help">Clicking will automatically fill in TTS configuration and voice</div>
        </el-form-item>
        <el-form-item label="TTS Configuration" prop="tts_config_id">
          <el-select
            v-model="groupForm.tts_config_id"
            placeholder="Please select TTS configuration (optional)"
            clearable
            style="width: 100%"
            @change="handleTtsConfigChange"
          >
            <el-option
              v-for="ttsConfig in ttsConfigs"
              :key="ttsConfig.config_id"
              :label="ttsConfig.is_default ? `${ttsConfig.name} (Default)` : ttsConfig.name"
              :value="ttsConfig.config_id"
            >
              <div class="config-option">
                {{ ttsConfig.name }}
                <el-tag v-if="ttsConfig.is_default" type="success" size="small" style="margin-left: 8px;">Default</el-tag>
              </div>
              <span class="config-desc">{{ ttsConfig.provider || 'No description' }}</span>
            </el-option>
          </el-select>
          <div class="form-help" v-if="groupForm.tts_config_id">
            {{ getCurrentTtsConfigInfo() }}
          </div>
        </el-form-item>
        <el-form-item label="Voice" prop="voice" v-if="groupForm.tts_config_id">
          <el-select
            v-model="groupForm.voice"
            placeholder="Please select or enter voice"
            filterable
            allow-create
            clearable
            style="width: 100%"
          >
            <el-option
              v-for="voice in currentVoiceOptions"
              :key="voice.value"
              :label="voice.label"
              :value="voice.value"
            />
          </el-select>
          <div class="form-help">
            Current TTS config: {{ getCurrentTtsConfigName() }}, you can search for voice name or value, or manually enter a custom voice value.
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGroupDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleSubmitGroup" :loading="submitting">
          {{ groupDialogMode === 'add' ? 'Create' : 'Save' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Sample Management Drawer -->
    <el-drawer
      v-model="showSampleDrawer"
      title="Sample Management"
      :size="800"
      :before-close="handleCloseSampleDrawer"
    >
      <div v-if="currentGroup" class="sample-drawer">
        <!-- Voiceprint Group Info -->
        <el-card class="group-info-card" shadow="never">
          <div class="group-info">
            <h3>{{ currentGroup.name }}</h3>
            <div v-if="currentGroup.prompt" class="prompt-section">
              <strong>Prompt:</strong>
              <p>{{ currentGroup.prompt }}</p>
            </div>
            <div v-if="currentGroup.description" class="description-section">
              <strong>Description:</strong>
              <p>{{ currentGroup.description }}</p>
            </div>
          </div>
        </el-card>

        <!-- Sample List -->
        <div class="samples-section">
          <div class="samples-header">
            <h4>Sample List</h4>
            <div class="samples-header-actions">
              <el-button type="success" @click="handleVerifyFromSamples">
                <el-icon><VideoPlay /></el-icon>
                Verify Voiceprint
              </el-button>
              <el-button type="primary" @click="handleAddSample">
                <el-icon><Plus /></el-icon>
                Upload New Sample
              </el-button>
            </div>
          </div>

          <el-table :data="samples" stripe style="width: 100%">
            <el-table-column prop="uuid" label="UUID" min-width="200">
              <template #default="{ row }">
                <el-tooltip :content="row.uuid" placement="top">
                  <span class="uuid-text">{{ truncateId(row.uuid) }}</span>
                </el-tooltip>
                <el-button
                  type="text"
                  size="small"
                  @click="copyToClipboard(row.uuid)"
                  style="margin-left: 8px;"
                >
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </template>
            </el-table-column>
            <el-table-column prop="file_name" label="File Name" min-width="150" />
            <el-table-column prop="file_size" label="File Size" width="100">
              <template #default="{ row }">
                {{ formatFileSize(row.file_size) }}
              </template>
            </el-table-column>
            <el-table-column prop="duration" label="Duration" width="80">
              <template #default="{ row }">
                {{ row.duration ? row.duration + 's' : '-' }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="Created At" width="180">
              <template #default="{ row }">
                {{ formatDate(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column label="Actions" width="180" fixed="right">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  size="small"
                  link
                  @click="handlePlaySample(row)"
                >
                  <el-icon><VideoPlay /></el-icon>
                  Play
                </el-button>
                <el-button
                  type="primary"
                  size="small"
                  link
                  @click="handleDownloadSample(row)"
                >
                  <el-icon><Download /></el-icon>
                  Download
                </el-button>
                <el-button
                  type="danger"
                  size="small"
                  link
                  @click="handleDeleteSample(row)"
                >
                  <el-icon><Delete /></el-icon>
                  Delete
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="samples.length === 0" class="empty-samples">
            <el-empty description="No samples, please upload audio files" />
          </div>
        </div>
      </div>
    </el-drawer>

    <!-- Upload Sample Dialog -->
    <el-dialog
      v-model="showUploadDialog"
      title="Add Voiceprint Sample"
      width="600px"
      :before-close="handleCloseUploadDialog"
    >
      <el-tabs v-model="uploadMode" class="upload-tabs">
        <!-- Select from History -->
        <el-tab-pane label="Select from History" name="history">
          <div class="history-section">
            <el-form :model="historyForm" label-width="100px">
              <el-form-item label="Agent">
                <el-select
                  v-model="historyForm.agent_id"
                  placeholder="Please select agent"
                  style="width: 100%"
                  @change="loadHistoryMessages"
                  clearable
                >
                  <el-option
                    v-for="agent in agents"
                    :key="agent.id"
                    :label="agent.name"
                    :value="agent.id"
                  />
                </el-select>
              </el-form-item>
            </el-form>
            
            <div v-loading="loadingHistory" class="history-list">
              <div v-if="historyMessages.length === 0 && !loadingHistory" class="empty-history">
                <el-empty description="No chat history, please select an agent first" />
              </div>
              <el-table
                v-else
                :data="historyMessages"
                row-key="message_id"
                stripe
                style="width: 100%"
                max-height="400"
                @row-click="handleSelectHistoryMessage"
              >
                <el-table-column label="Select" width="80" align="center">
                  <template #default="{ row }">
                    <el-radio
                      :model-value="historyForm.selected_message_id"
                      :label="row.message_id"
                      @change="historyForm.selected_message_id = row.message_id"
                    />
                  </template>
                </el-table-column>
                <el-table-column prop="content" label="Message Content" min-width="200">
                  <template #default="{ row }">
                    <div class="message-content">{{ truncateText(row.content, 50) }}</div>
                  </template>
                </el-table-column>
                <el-table-column prop="device_id" label="Device ID" width="150">
                  <template #default="{ row }">
                    <el-tooltip :content="row.device_id" placement="top">
                      <span>{{ truncateId(row.device_id) }}</span>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column prop="created_at" label="Time" width="180">
                  <template #default="{ row }">
                    {{ formatDate(row.created_at) }}
                  </template>
                </el-table-column>
                <el-table-column label="Action" width="100">
                  <template #default="{ row }">
                    <el-button
                      type="primary"
                      size="small"
                      link
                      @click.stop="handlePreviewHistoryAudio(row)"
                    >
                      <el-icon><VideoPlay /></el-icon>
                      Preview
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </el-tab-pane>
        
        <!-- Upload File -->
        <el-tab-pane label="Upload File" name="upload">
          <el-form
            ref="uploadFormRef"
            :model="uploadForm"
            :rules="uploadRules"
            label-width="0"
          >
            <el-form-item prop="audio">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
            :limit="1"
            accept=".wav,audio/wav"
            drag
                class="audio-upload"
          >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
                  Drag WAV audio file here, or <em>click to select file</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                Only WAV format audio files are supported, recommended duration 3-10 seconds, file size not exceeding 10MB
              </div>
            </template>
          </el-upload>
              <div v-if="uploadForm.audioFile" class="file-info">
            <el-icon><Document /></el-icon>
                <span>{{ uploadForm.audioFile.name }}</span>
                <span class="file-size">({{ formatFileSize(uploadForm.audioFile.size) }})</span>
          </div>
        </el-form-item>
      </el-form>
        </el-tab-pane>

        <!-- Record Audio -->
        <el-tab-pane label="Record Audio" name="record">
          <div class="record-section">
            <div class="record-status">
              <div v-if="!isRecording && !recordedBlob" class="record-ready">
                <el-icon size="48" color="#409EFF"><Microphone /></el-icon>
                <p>Click the button below to start recording</p>
                <p class="record-tip">Recommended to record 3-10 seconds of clear audio</p>
              </div>
              <div v-else-if="isRecording" class="record-recording">
                <div class="recording-indicator">
                  <span class="recording-dot"></span>
                  <span class="recording-text">Recording...</span>
                </div>
                <div class="record-time">{{ formatRecordTime(recordTime) }}</div>
                <p class="record-tip">Click stop button to end recording</p>
              </div>
              <div v-else-if="recordedBlob" class="record-complete">
                <el-icon size="48" color="#67C23A"><CircleCheck /></el-icon>
                <p>Recording completed</p>
                <p class="record-tip">Duration: {{ formatRecordTime(recordTime) }}</p>
                <audio :src="recordedBlobUrl" controls class="record-preview"></audio>
              </div>
            </div>

            <div class="record-controls">
          <el-button 
                v-if="!isRecording && !recordedBlob"
            type="primary" 
            size="large"
                @click="startRecording"
                :disabled="!canRecord"
              >
                <el-icon><VideoPlay /></el-icon>
                Start Recording
              </el-button>
              <el-button
                v-if="isRecording"
                type="danger"
                size="large"
                @click="stopRecording"
              >
                <el-icon><VideoPause /></el-icon>
                Stop Recording
              </el-button>
              <el-button
                v-if="recordedBlob"
                type="primary"
                size="large"
                @click="startRecording"
                :disabled="!canRecord"
              >
                <el-icon><Refresh /></el-icon>
                Re-record
          </el-button>
        </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="handleCloseUploadDialog">Cancel</el-button>
        <el-button
          type="primary"
          @click="handleSubmitSample"
          :loading="submitting"
          :disabled="!hasAudioFile"
        >
          Confirm
        </el-button>
      </template>
    </el-dialog>

    <!-- Verify Voiceprint Group Dialog -->
    <el-dialog
      v-model="showVerifyDialog"
      :title="`Verify Voiceprint Group: ${currentVerifyGroup?.name || ''}`"
      width="600px"
      :before-close="handleCloseVerifyDialog"
    >
      <el-tabs v-model="verifyMode" class="verify-tabs">
        <!-- Upload File -->
        <el-tab-pane label="Upload File" name="upload">
          <el-form
            ref="verifyFormRef"
            :model="verifyForm"
            :rules="verifyRules"
            label-width="0"
          >
            <el-form-item prop="audio">
              <el-upload
                ref="verifyUploadRef"
                :auto-upload="false"
                :on-change="handleVerifyFileChange"
                :on-remove="handleVerifyFileRemove"
                :limit="1"
                accept=".wav,audio/wav"
                drag
                class="audio-upload"
                :file-list="verifyFileList"
              >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                <div class="el-upload__text">
                  Drag WAV audio file here, or <em>click to select file</em>
                </div>
                <template #tip>
                  <div class="el-upload__tip">
                    Only WAV format audio files are supported, recommended duration 3-10 seconds, file size not exceeding 10MB
                  </div>
                </template>
              </el-upload>
              <div v-if="verifyForm.audioFile" class="file-info">
                <el-icon><Document /></el-icon>
                <span>{{ verifyForm.audioFile.name }}</span>
                <span class="file-size">({{ formatFileSize(verifyForm.audioFile.size) }})</span>
              </div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Record Audio -->
        <el-tab-pane label="Record Audio" name="record">
          <div class="record-section">
            <div class="record-status">
              <div v-if="!isVerifyRecording && !verifyRecordedBlob" class="record-ready">
                <el-icon size="48" color="#409EFF"><Microphone /></el-icon>
                <p>Click the button below to start recording</p>
                <p class="record-tip">Recommended to record 3-10 seconds of clear audio</p>
              </div>
              <div v-else-if="isVerifyRecording" class="record-recording">
                <div class="recording-indicator">
                  <span class="recording-dot"></span>
                  <span class="recording-text">Recording...</span>
                </div>
                <div class="record-time">{{ formatRecordTime(verifyRecordTime) }}</div>
                <p class="record-tip">Click stop button to end recording</p>
              </div>
              <div v-else-if="verifyRecordedBlob" class="record-complete">
                <el-icon size="48" color="#67C23A"><CircleCheck /></el-icon>
                <p>Recording completed</p>
                <p class="record-tip">Duration: {{ formatRecordTime(verifyRecordTime) }}</p>
                <audio :src="verifyRecordedBlobUrl" controls class="record-preview"></audio>
              </div>
            </div>

            <div class="record-controls">
              <el-button
                v-if="!isVerifyRecording && !verifyRecordedBlob"
                type="primary"
                size="large"
                @click="startVerifyRecording"
                :disabled="!canRecord"
              >
                <el-icon><VideoPlay /></el-icon>
                Start Recording
              </el-button>
              <el-button
                v-if="isVerifyRecording"
                type="danger"
                size="large"
                @click="stopVerifyRecording"
              >
                <el-icon><VideoPause /></el-icon>
                Stop Recording
              </el-button>
              <el-button
                v-if="verifyRecordedBlob"
                type="primary"
                size="large"
                @click="startVerifyRecording"
                :disabled="!canRecord"
              >
                <el-icon><Refresh /></el-icon>
                Re-record
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- Verification Result Display -->
      <div v-if="verifyResult" class="verify-result">
        <el-divider>Verification Result</el-divider>
        <div :class="['result-content', verifyResult.verified ? 'result-success' : 'result-failed']">
          <div class="result-icon">
            <el-icon v-if="verifyResult.verified" size="48" color="#67C23A"><CircleCheck /></el-icon>
            <el-icon v-else size="48" color="#F56C6C"><CircleClose /></el-icon>
          </div>
          <div class="result-info">
            <div class="result-status">
              {{ verifyResult.verified ? 'Verification Passed' : 'Verification Failed' }}
            </div>
            <div class="result-details">
              <div>Confidence: <strong>{{ (verifyResult.confidence * 100).toFixed(1) }}%</strong></div>
              <div>Threshold: {{ (verifyResult.threshold * 100).toFixed(1) }}%</div>
            </div>
            <div class="result-message">{{ verifyResult.message }}</div>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="handleCloseVerifyDialog">Cancel</el-button>
        <el-button
          type="primary"
          @click="handleSubmitVerify"
          :loading="verifying"
          :disabled="!hasVerifyAudioFile"
        >
          Verify
        </el-button>
      </template>
    </el-dialog>

    <!-- Audio Player (hidden) -->
    <audio ref="audioPlayer" style="display: none;" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Plus, 
  Edit, 
  Delete, 
  View,
  Search,
  UploadFilled,
  Document,
  DocumentCopy,
  VideoPlay,
  Download,
  Microphone,
  CircleCheck,
  CircleClose,
  Refresh,
  VideoPause
} from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const submitting = ref(false)
const speakerGroups = ref([])
const agents = ref([])
const samples = ref([])
const filterAgentId = ref('')
const searchKeyword = ref('')

// Dialog states
const showGroupDialog = ref(false)
const groupDialogMode = ref('add') // 'add' | 'edit'
const currentGroup = ref(null)
const showSampleDrawer = ref(false)
const showUploadDialog = ref(false)
const uploadMode = ref('history') // 'upload' | 'record' | 'history'

// Verify dialog related
const showVerifyDialog = ref(false)
const verifyMode = ref('upload') // 'upload' | 'record'
const currentVerifyGroup = ref(null)
const verifying = ref(false)
const verifyResult = ref(null)

// Verify form
const verifyForm = reactive({
  audioFile: null,
  audio: null
})

// Verify file list (for el-upload component)
const verifyFileList = ref([])

const verifyRules = {
  audio: [
    {
      validator: (rule, value, callback) => {
        if (!verifyForm.audioFile && !verifyRecordedBlob.value) {
          callback(new Error('Please upload or record audio file'))
        } else {
          callback()
        }
      },
      trigger: ['change', 'blur']
    }
  ]
}

// Verify recording related
const isVerifyRecording = ref(false)
const verifyMediaRecorder = ref(null)
const verifyRecordedBlob = ref(null)
const verifyRecordedBlobUrl = ref('')
const verifyRecordTime = ref(0)
const verifyRecordTimer = ref(null)

// Recording related
const isRecording = ref(false)
const mediaRecorder = ref(null)
const recordedBlob = ref(null)
const recordedBlobUrl = ref('')
const recordTime = ref(0)
const recordTimer = ref(null)
const canRecord = ref(false)

// Form refs
const groupFormRef = ref()
const uploadFormRef = ref()
const uploadRef = ref()
const verifyFormRef = ref()
const verifyUploadRef = ref()
const audioPlayer = ref()

// Voiceprint group form
const groupForm = reactive({
  agent_id: null,
  name: '',
  prompt: '',
  description: '',
  tts_config_id: null,
  voice: null
})

const groupRules = {
  agent_id: [
    { required: true, message: 'Please select associated agent', trigger: 'change' }
  ],
  name: [
    { required: true, message: 'Please enter voiceprint name', trigger: 'blur' },
    { min: 1, max: 100, message: 'Length must be between 1 and 100 characters', trigger: 'blur' }
  ]
}

// TTS config related
const ttsConfigs = ref([])
const currentVoiceOptions = ref([])
const cloneVoicePresets = ref([])
const cloneVoicesLoading = ref(false)

// Upload form
const uploadForm = reactive({
  audioFile: null,
  audio: null
})

const uploadRules = {
  audio: [
    { 
      validator: (rule, value, callback) => {
        if (!uploadForm.audioFile && !recordedBlob.value) {
          callback(new Error('Please upload or record audio file'))
        } else {
          callback()
        }
      }, 
      trigger: ['change', 'blur']
    }
  ]
}

// History related
const loadingHistory = ref(false)
const historyMessages = ref([])
const historyForm = reactive({
  agent_id: null,
  selected_message_id: null
})

// Calculate if has audio file
const hasAudioFile = computed(() => {
  if (uploadMode.value === 'history') {
    return historyForm.selected_message_id !== null
  }
  return uploadForm.audioFile !== null || recordedBlob.value !== null
})

// Filtered voiceprint group list
const filteredGroups = computed(() => {
  let result = speakerGroups.value

  // Filter by agent
  if (filterAgentId.value) {
    result = result.filter(g => g.agent_id === filterAgentId.value)
  }

  // Search by keyword
  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase()
    result = result.filter(g =>
      g.name.toLowerCase().includes(keyword) ||
      (g.prompt && g.prompt.toLowerCase().includes(keyword)) ||
      (g.description && g.description.toLowerCase().includes(keyword))
    )
  }

  return result
})

// Load agent list
const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents')
    agents.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load agent list:', error)
    ElMessage.error('Failed to load agent list')
  }
}

// Load TTS config list
const loadTtsConfigs = async () => {
  try {
    const response = await api.get('/user/tts-configs')
    ttsConfigs.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load TTS config:', error)
    ElMessage.error('Failed to load TTS config')
  }
}

const normalizeCloneStatus = (clone) => {
  const status = String(clone?.status || '').trim().toLowerCase()
  const taskStatus = String(clone?.task_status || '').trim().toLowerCase()
  if (status === 'failed' || taskStatus === 'failed') return 'failed'
  if (status === 'active' || taskStatus === 'succeeded') return 'active'
  if (taskStatus === 'queued' || taskStatus === 'processing') return taskStatus
  if (status === 'queued' || status === 'processing') return status
  return status || taskStatus || 'unknown'
}

const loadCloneVoicePresets = async () => {
  cloneVoicesLoading.value = true
  try {
    const response = await api.get('/user/voice-clones')
    const cloneList = response.data.data || []
    cloneVoicePresets.value = cloneList
      .filter(clone => normalizeCloneStatus(clone) === 'active')
      .filter(clone => clone?.tts_config_id && clone?.provider_voice_id)
      .map(clone => ({
        id: clone.id,
        name: clone.name || clone.provider_voice_id,
        provider_voice_id: clone.provider_voice_id,
        tts_config_id: clone.tts_config_id,
        tts_config_name: clone.tts_config_name || ''
      }))
  } catch (error) {
    console.error('Failed to load cloned voices:', error)
    cloneVoicePresets.value = []
  } finally {
    cloneVoicesLoading.value = false
  }
}

const isCloneVoiceSelected = (clone) => {
  return groupForm.tts_config_id === clone?.tts_config_id && groupForm.voice === clone?.provider_voice_id
}

const applyCloneVoice = async (clone) => {
  if (!clone) return
  const ttsConfig = ttsConfigs.value.find(config => config.config_id === clone.tts_config_id)
  if (!ttsConfig) {
    return
  }
  groupForm.tts_config_id = clone.tts_config_id
  await handleTtsConfigChange(clone.tts_config_id)
  groupForm.voice = clone.provider_voice_id
}

// When TTS config changes, load corresponding voice options
const handleTtsConfigChange = async (configId) => {
  if (!configId) {
    currentVoiceOptions.value = []
    groupForm.voice = null
    return
  }
  
  const config = ttsConfigs.value.find(c => c.config_id === configId)
  if (!config) {
    currentVoiceOptions.value = []
    return
  }

  try {
    // Get full voice list for this provider from backend API
    const params = { provider: config.provider }
    // Always include config_id parameter
    if (configId) {
      params.config_id = configId
    }
    const response = await api.get('/user/voice-options', { params })
    currentVoiceOptions.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load voice list:', error)
    currentVoiceOptions.value = []
    ElMessage.warning('Failed to load voice list, please try again later')
  }
}

// Extract voice options based on different providers
const extractVoiceOptions = (provider, config) => {
  const options = []
  
  if (!config) return options
  
  // Extract voices based on different TTS providers
  switch (provider) {
    case 'edge':
    case 'microsoft':
      // Edge TTS common voices
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice })
      }
      // Add common Chinese voices
      const edgeVoices = [
        { label: 'zh-CN-XiaoxiaoNeural (Xiaoxiao)', value: 'zh-CN-XiaoxiaoNeural' },
        { label: 'zh-CN-YunxiNeural (Yunxi)', value: 'zh-CN-YunxiNeural' },
        { label: 'zh-CN-YunyangNeural (Yunyang)', value: 'zh-CN-YunyangNeural' },
        { label: 'zh-CN-XiaoyiNeural (Xiaoyi)', value: 'zh-CN-XiaoyiNeural' },
        { label: 'zh-CN-YunjianNeural (Yunjian)', value: 'zh-CN-YunjianNeural' },
        { label: 'zh-CN-XiaochenNeural (Xiaochen)', value: 'zh-CN-XiaochenNeural' },
        { label: 'zh-CN-XiaohanNeural (Xiaohan)', value: 'zh-CN-XiaohanNeural' }
      ]
      edgeVoices.forEach(v => {
        if (!options.find(o => o.value === v.value)) {
          options.push(v)
        }
      })
      break
      
    case 'doubao':
    case 'doubao_ws':
      // Doubao TTS voices
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice })
      }
      const doubaoVoices = [
        { label: 'Shuangkuaisisi (Sweet Female)', value: 'zh_female_shuangkuaisisi_moon_bigtts' },
        { label: 'BV700 V2 (Male)', value: 'BV700_V2_streaming' },
        { label: 'BV001 (Female)', value: 'BV001_streaming' },
        { label: 'BV002 (Male)', value: 'BV002_streaming' }
      ]
      doubaoVoices.forEach(v => {
        if (!options.find(o => o.value === v.value)) {
          options.push(v)
        }
      })
      break
      
    case 'cosyvoice':
      // CosyVoice uses spk_id
      if (config.spk_id) {
        options.push({ label: config.spk_id, value: config.spk_id })
      }
      const cosyVoices = [
        { label: 'Chinese Female', value: 'Chinese Female' },
        { label: 'Chinese Male', value: 'Chinese Male' },
        { label: 'Cantonese Female', value: 'Cantonese Female' },
        { label: 'English Female', value: 'English Female' },
        { label: 'English Male', value: 'English Male' },
        { label: 'Japanese Male', value: 'Japanese Male' },
        { label: 'Korean Female', value: 'Korean Female' }
      ]
      cosyVoices.forEach(v => {
        if (!options.find(o => o.value === v.value)) {
          options.push(v)
        }
      })
      break
      
    case 'minimax':
      // Minimax TTS uses voice
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice })
      }
      const minimaxVoices = [
        { label: 'Youthful (Male)', value: 'male-qn-qingse' },
        { label: 'Youthful (Female)', value: 'female-qn-qingse' },
        { label: 'Young (Male)', value: 'male-shaonian' },
        { label: 'Young (Female)', value: 'female-shaonian' },
        { label: 'Mature (Male)', value: 'male-chengshu' },
        { label: 'Mature (Female)', value: 'female-chengshu' },
        { label: 'Warm (Male)', value: 'male-wennuan' },
        { label: 'Warm (Female)', value: 'female-wennuan' },
        { label: 'Clear (Male)', value: 'male-qinglang' },
        { label: 'Clear (Female)', value: 'female-qinglang' },
        { label: 'Deep (Male)', value: 'male-houzhong' },
        { label: 'Deep (Female)', value: 'female-houzhong' }
      ]
      minimaxVoices.forEach(v => {
        if (!options.find(o => o.value === v.value)) {
          options.push(v)
        }
      })
      break
      
    default:
      // Other providers, try to extract from config
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice })
      }
      if (config.spk_id) {
        options.push({ label: config.spk_id, value: config.spk_id })
      }
  }
  
  return options
}

// Get current TTS config name
const getCurrentTtsConfigName = () => {
  if (!groupForm.tts_config_id) return ''
  const config = ttsConfigs.value.find(c => c.config_id === groupForm.tts_config_id)
  return config ? config.name : ''
}

// Get current TTS config info
const getCurrentTtsConfigInfo = () => {
  if (!groupForm.tts_config_id) return ''
  const config = ttsConfigs.value.find(c => c.config_id === groupForm.tts_config_id)
  if (!config) return ''
  return `TTS Provider: ${config.provider || 'Unknown'}`
}

// Load voiceprint group list
const loadSpeakerGroups = async () => {
  try {
    loading.value = true
    const params = {}
    if (filterAgentId.value) {
      params.agent_id = filterAgentId.value
    }
    const response = await api.get('/user/speaker-groups', { params })
    speakerGroups.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load voiceprint group list:', error)
    ElMessage.error('Failed to load voiceprint group list: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

// Search handling
const handleSearch = () => {
  // Search is client-side filtering, no need to re-request
}

// Create voiceprint group
const handleAddGroup = async () => {
  groupDialogMode.value = 'add'
  resetGroupForm()
  await loadCloneVoicePresets()
  showGroupDialog.value = true
}

// Edit voiceprint group
const handleEditGroup = async (group) => {
  groupDialogMode.value = 'edit'
  currentGroup.value = group
  groupForm.agent_id = group.agent_id
  groupForm.name = group.name
  groupForm.prompt = group.prompt || ''
  groupForm.description = group.description || ''
  groupForm.tts_config_id = group.tts_config_id || null
  groupForm.voice = group.voice || null
  await loadCloneVoicePresets()
  
  // If TTS config exists, load corresponding voice options
  if (groupForm.tts_config_id) {
    await handleTtsConfigChange(groupForm.tts_config_id)
  }
  
  showGroupDialog.value = true
}

// Submit voiceprint group
const handleSubmitGroup = async () => {
  if (!groupFormRef.value) return

  try {
    await groupFormRef.value.validate()
    submitting.value = true

    if (groupDialogMode.value === 'add') {
      const response = await api.post('/user/speaker-groups', groupForm)
      ElMessage.success('Create successful')
      showGroupDialog.value = false
      await loadSpeakerGroups()
    } else {
      const response = await api.put(`/user/speaker-groups/${currentGroup.value.id}`, groupForm)
      ElMessage.success('Update successful')
      showGroupDialog.value = false
      await loadSpeakerGroups()
    }
  } catch (error) {
    if (error.fields) {
      // Form validation error
      return
    }
    console.error('Submit failed:', error)
    ElMessage.error('Operation failed: ' + (error.response?.data?.error || error.message))
  } finally {
    submitting.value = false
  }
}

// Verify voiceprint group
const handleVerifyGroup = async (group) => {
  // Clear previous data first
  resetVerifyForm()
  
  // Wait for DOM update to complete
  await nextTick()
  
  currentVerifyGroup.value = group
  verifyResult.value = null
  verifyMode.value = 'upload'
  showVerifyDialog.value = true
  
  // Ensure upload component is cleared again
  await nextTick()
  verifyUploadRef.value?.clearFiles()
  verifyFileList.value = []
  
  // Check if browser supports recording
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach(track => track.stop())
    canRecord.value = true
  } catch (error) {
    console.warn('Browser does not support recording:', error)
    canRecord.value = false
    if (verifyMode.value === 'record') {
      ElMessage.warning('Your browser does not support recording, please use file upload instead')
      verifyMode.value = 'upload'
    }
  }
}

// Close verify dialog
const handleCloseVerifyDialog = () => {
  if (isVerifyRecording.value) {
    stopVerifyRecording()
  }
  resetVerifyForm()
  showVerifyDialog.value = false
}

// Verify file change handling
const handleVerifyFileChange = async (file, fileList) => {
  // Clear file list first to ensure old files are removed
  verifyFileList.value = []
  await nextTick()
  
  // If file already exists, clean up previous file first
  if (verifyForm.audioFile) {
    verifyForm.audioFile = null
    verifyForm.audio = null
  }
  
  // Clean up recording related
  if (verifyRecordedBlob.value) {
    if (verifyRecordedBlobUrl.value) {
      URL.revokeObjectURL(verifyRecordedBlobUrl.value)
      verifyRecordedBlobUrl.value = ''
    }
    verifyRecordedBlob.value = null
    verifyRecordTime.value = 0
  }
  
  // Clean up verify result
  verifyResult.value = null
  
  const fileObj = file.raw || file
  if (!fileObj) {
    ElMessage.warning('Invalid file object')
    verifyUploadRef.value?.clearFiles()
    verifyForm.audioFile = null
    verifyFileList.value = []
    return
  }

  // Validate file type
  const fileName = fileObj.name || file.name || ''
  const fileType = fileObj.type || file.type || ''
  if (!fileType.includes('wav') && !fileName.toLowerCase().endsWith('.wav')) {
    ElMessage.warning('Only WAV format audio files are supported')
    verifyUploadRef.value?.clearFiles()
    verifyForm.audioFile = null
    verifyFileList.value = []
    return
  }

  // Validate file size (10MB)
  const fileSize = fileObj.size || file.size || 0
  if (fileSize > 10 * 1024 * 1024) {
    ElMessage.warning('File size cannot exceed 10MB')
    verifyUploadRef.value?.clearFiles()
    verifyForm.audioFile = null
    verifyFileList.value = []
    return
  }

  // Set new file
  verifyForm.audioFile = file
  verifyForm.audio = file
  
  // Update file list display (only show latest file)
  verifyFileList.value = [file]
  
  await nextTick()

  if (verifyFormRef.value) {
    verifyFormRef.value.clearValidate('audio')
  }
}

// Verify file remove handling
const handleVerifyFileRemove = () => {
  verifyForm.audioFile = null
  verifyForm.audio = null
  verifyFileList.value = []
  verifyResult.value = null // Clean up verify result
  if (verifyFormRef.value) {
    verifyFormRef.value.validateField('audio')
  }
}

// Start verify recording
const startVerifyRecording = async () => {
  try {
    // Stop previous recording (if any)
    if (verifyMediaRecorder.value && verifyMediaRecorder.value.state !== 'inactive') {
      verifyMediaRecorder.value.stop()
    }

    // Clean up previous recording
    if (verifyRecordedBlobUrl.value) {
      URL.revokeObjectURL(verifyRecordedBlobUrl.value)
      verifyRecordedBlobUrl.value = ''
    }
    verifyRecordedBlob.value = null
    verifyRecordTime.value = 0

    // Get microphone permission
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true
      }
    })

    // Create MediaRecorder
    const chunks = []
    const options = {
      mimeType: 'audio/webm;codecs=opus'
    }

    if (!MediaRecorder.isTypeSupported(options.mimeType)) {
      verifyMediaRecorder.value = new MediaRecorder(stream)
    } else {
      verifyMediaRecorder.value = new MediaRecorder(stream, options)
    }

    verifyMediaRecorder.value.ondataavailable = (e) => {
      if (e.data.size > 0) {
        chunks.push(e.data)
      }
    }

    verifyMediaRecorder.value.onstop = async () => {
      stream.getTracks().forEach(track => track.stop())
      
      try {
        // Convert recorded audio to WAV format
        const blob = new Blob(chunks, { type: chunks[0]?.type || 'audio/webm' })
        const wavBlob = await convertToWav(blob)
        
        verifyRecordedBlob.value = wavBlob
        verifyRecordedBlobUrl.value = URL.createObjectURL(wavBlob)
        
        // Create File object for upload
        const fileName = `verify_recording_${Date.now()}.wav`
        const file = new File([wavBlob], fileName, { type: 'audio/wav' })
        verifyForm.audioFile = { raw: file, name: fileName, size: wavBlob.size }
        verifyForm.audio = file

        if (verifyFormRef.value) {
          verifyFormRef.value.clearValidate('audio')
        }
      } catch (error) {
        console.error('Failed to process recording data:', error)
        ElMessage.error('Failed to process recording data, please try again')
        verifyRecordedBlob.value = null
        verifyRecordedBlobUrl.value = ''
        verifyForm.audioFile = null
        verifyForm.audio = null
      }

      chunks.length = 0
    }

    // Start recording
    verifyMediaRecorder.value.start(100)
    isVerifyRecording.value = true

    // Start timer
    verifyRecordTimer.value = setInterval(() => {
      verifyRecordTime.value += 0.1
    }, 100)

    ElMessage.success('Recording started')
  } catch (error) {
    console.error('Recording failed:', error)
    ElMessage.error('Recording failed: ' + error.message)
    canRecord.value = false
  }
}

// Stop verify recording
const stopVerifyRecording = () => {
  if (verifyMediaRecorder.value && verifyMediaRecorder.value.state !== 'inactive') {
    verifyMediaRecorder.value.stop()
  }
  isVerifyRecording.value = false
  
  if (verifyRecordTimer.value) {
    clearInterval(verifyRecordTimer.value)
    verifyRecordTimer.value = null
  }

  ElMessage.success('Recording completed')
}

// Submit verify
const handleSubmitVerify = async () => {
  if (!verifyFormRef.value) return

  try {
    await verifyFormRef.value.validate()

    if (!verifyForm.audioFile && !verifyRecordedBlob.value) {
      ElMessage.warning('Please upload or record audio file')
      return
    }

    verifying.value = true
    verifyResult.value = null

    let file
    if (verifyForm.audioFile) {
      // Use uploaded file
      file = verifyForm.audioFile.raw || verifyForm.audioFile
    } else if (verifyRecordedBlob.value) {
      // Use recorded audio
      const fileName = `verify_recording_${Date.now()}.wav`
      file = new File([verifyRecordedBlob.value], fileName, { type: 'audio/wav' })
    } else {
      ElMessage.warning('Please upload or record audio file')
      return
    }

    const formData = new FormData()
    formData.append('audio', file)

    const response = await api.post(`/user/speaker-groups/${currentVerifyGroup.value.id}/verify`, formData)
    
    if (response.data.success && response.data.data) {
      verifyResult.value = {
        verified: response.data.data.verified,
        confidence: response.data.data.confidence,
        threshold: response.data.data.threshold,
        message: response.data.data.message
      }
      
      if (verifyResult.value.verified) {
        ElMessage.success('Verification passed!')
      } else {
        ElMessage.warning('Verification failed')
      }
    } else {
      ElMessage.error('Verification failed')
    }
  } catch (error) {
    if (error.fields) {
      return
    }
    console.error('Verification failed:', error)
    ElMessage.error('Verification failed: ' + (error.response?.data?.error || error.message))
  } finally {
    verifying.value = false
  }
}

// Reset verify form
const resetVerifyForm = () => {
  if (verifyFormRef.value) {
    verifyFormRef.value.resetFields()
  }
  if (verifyUploadRef.value) {
    verifyUploadRef.value.clearFiles()
  }
  verifyForm.audioFile = null
  verifyForm.audio = null
  
  // Clean up verify recording related
  if (isVerifyRecording.value) {
    stopVerifyRecording()
  }
  if (verifyRecordedBlobUrl.value) {
    URL.revokeObjectURL(verifyRecordedBlobUrl.value)
    verifyRecordedBlobUrl.value = ''
  }
  verifyRecordedBlob.value = null
  verifyRecordTime.value = 0
  verifyMode.value = 'upload'
  verifyResult.value = null
}

// Calculate if has verify audio file
const hasVerifyAudioFile = computed(() => {
  return verifyForm.audioFile !== null || verifyRecordedBlob.value !== null
})

// Delete voiceprint group
const handleDeleteGroup = async (group) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete voiceprint group "${group.name}"? This action will delete all samples under this group and cannot be undone.`,
      'Confirm Delete',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )

    loading.value = true
    await api.delete(`/user/speaker-groups/${group.id}`)
    ElMessage.success('Delete successful')
    await loadSpeakerGroups()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Delete failed:', error)
      ElMessage.error('Delete failed: ' + (error.response?.data?.error || error.message))
    }
  } finally {
    loading.value = false
  }
}

// View samples
const handleViewSamples = async (group) => {
  currentGroup.value = group
  showSampleDrawer.value = true
  await loadSamples(group.id)
}

// Verify voiceprint group from sample management drawer
const handleVerifyFromSamples = () => {
  if (currentGroup.value) {
    showSampleDrawer.value = false
    handleVerifyGroup(currentGroup.value)
  }
}

// Load sample list
const loadSamples = async (groupId) => {
  try {
    const response = await api.get(`/user/speaker-groups/${groupId}/samples`)
    samples.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load sample list:', error)
    ElMessage.error('Failed to load sample list')
  }
}

// Close sample drawer
const handleCloseSampleDrawer = () => {
  showSampleDrawer.value = false
  currentGroup.value = null
  samples.value = []
}

// Add sample
const handleAddSample = async () => {
  resetUploadForm()
  uploadMode.value = 'history'
  showUploadDialog.value = true
  
  // Initialize history form
  historyForm.agent_id = currentGroup.value?.agent_id || null
  historyForm.selected_message_id = null
  historyMessages.value = []
  
  // If voiceprint group has associated agent, auto load history
  if (currentGroup.value?.agent_id) {
    historyForm.agent_id = currentGroup.value.agent_id
    await loadHistoryMessages()
  }
  
  // Check if browser supports recording
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach(track => track.stop())
    canRecord.value = true
  } catch (error) {
    console.warn('Browser does not support recording:', error)
    canRecord.value = false
  }
}

// Load history messages
const loadHistoryMessages = async () => {
  if (!historyForm.agent_id) {
    historyMessages.value = []
    return
  }
  
  loadingHistory.value = true
  try {
    const response = await api.get(`/user/history/agents/${historyForm.agent_id}/messages`, {
      params: {
        page: 1,
        page_size: 100
      }
    })
    historyMessages.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load history messages:', error)
    ElMessage.error('Failed to load history messages')
    historyMessages.value = []
  } finally {
    loadingHistory.value = false
  }
}

// Select history message
const handleSelectHistoryMessage = (row) => {
  historyForm.selected_message_id = row.message_id
}

// Preview history audio
const handlePreviewHistoryAudio = async (row) => {
  try {
    const response = await api.get(`/user/history/messages/${row.message_id}/audio`, {
      responseType: 'blob'
    })
    const blobUrl = URL.createObjectURL(response.data)
    audioPlayer.value.src = blobUrl
    audioPlayer.value.play()
    
    // Clean up blob URL after playback
    audioPlayer.value.onended = () => {
      URL.revokeObjectURL(blobUrl)
    }
  } catch (error) {
    console.error('Failed to preview audio:', error)
    ElMessage.error('Failed to preview audio')
  }
}

// File change handling
const handleFileChange = (file) => {
  const fileObj = file.raw || file
  if (!fileObj) {
    uploadForm.audioFile = null
    return
  }
  
  // Validate file type
  if (!fileObj.type.includes('wav') && !fileObj.name.toLowerCase().endsWith('.wav')) {
    ElMessage.warning('Only WAV format audio files are supported')
    uploadForm.audioFile = null
    return
  }
  
  // Validate file size (10MB)
  if (fileObj.size > 10 * 1024 * 1024) {
    ElMessage.warning('File size cannot exceed 10MB')
    uploadForm.audioFile = null
    return
  }
  
  uploadForm.audioFile = fileObj
}

// File remove handling
const handleFileRemove = () => {
  uploadForm.audioFile = null
}

// Convert to WAV format
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

// AudioBuffer to WAV
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
    for (let i = 0; i < str.length; i++) {
      view.setUint8(offset + i, str.charCodeAt(i))
    }
  }
  
  // WAV header
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
  
  // Write audio data
  let offset = 44
  for (let i = 0; i < length; i++) {
    for (let channel = 0; channel < numberOfChannels; channel++) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]))
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7FFF, true)
      offset += 2
    }
  }
  
  return arrayBuffer
}

// Start recording
const startRecording = async () => {
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    const chunks = []
    
    mediaRecorder.value = new MediaRecorder(stream)
    
    mediaRecorder.value.ondataavailable = (e) => {
      if (e.data.size > 0) {
        chunks.push(e.data)
      }
    }
    
    mediaRecorder.value.onstop = async () => {
      const blob = new Blob(chunks, { type: 'audio/webm' })
      const wavBlob = await convertToWav(blob)
      
      recordedBlob.value = wavBlob
      recordedBlobUrl.value = URL.createObjectURL(wavBlob)
      
      stream.getTracks().forEach(track => track.stop())
    }
    
    mediaRecorder.value.start()
    isRecording.value = true
    recordTime.value = 0
    
    // Start timer
    recordTimer.value = setInterval(() => {
      recordTime.value += 1
    }, 1000)
    
    ElMessage.success('Recording started')
  } catch (error) {
    console.error('Recording failed:', error)
    ElMessage.error('Recording failed: ' + error.message)
    canRecord.value = false
  }
}

// Stop recording
const stopRecording = () => {
  if (mediaRecorder.value && mediaRecorder.value.state !== 'inactive') {
    mediaRecorder.value.stop()
  }
  isRecording.value = false
  
  if (recordTimer.value) {
    clearInterval(recordTimer.value)
    recordTimer.value = null
  }
  
  ElMessage.success('Recording completed')
}

// Format record time
const formatRecordTime = (seconds) => {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
}

// Submit sample
const handleSubmitSample = async () => {
  if (!uploadForm.audioFile && !recordedBlob.value && !historyForm.selected_message_id) {
    ElMessage.warning('Please select audio file or record')
    return
  }
  
  submitting.value = true
  
  try {
    let formData = new FormData()
    
    if (uploadMode.value === 'history' && historyForm.selected_message_id) {
      // Use history message
      formData.append('message_id', historyForm.selected_message_id)
    } else if (uploadForm.audioFile) {
      // Use uploaded file
      formData.append('audio', uploadForm.audioFile)
    } else if (recordedBlob.value) {
      // Use recorded audio
      const fileName = `recording_${Date.now()}.wav`
      const file = new File([recordedBlob.value], fileName, { type: 'audio/wav' })
      formData.append('audio', file)
    }
    
    await api.post(`/user/speaker-groups/${currentGroup.value.id}/samples`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    
    ElMessage.success('Sample added successfully')
    showUploadDialog.value = false
    await loadSamples(currentGroup.value.id)
  } catch (error) {
    console.error('Failed to add sample:', error)
    ElMessage.error('Failed to add sample: ' + (error.response?.data?.error || error.message))
  } finally {
    submitting.value = false
  }
}

// Close upload dialog
const handleCloseUploadDialog = () => {
  if (isRecording.value) {
    stopRecording()
  }
  resetUploadForm()
  showUploadDialog.value = false
}

// Reset upload form
const resetUploadForm = () => {
  uploadForm.audioFile = null
  if (recordedBlobUrl.value) {
    URL.revokeObjectURL(recordedBlobUrl.value)
  }
  recordedBlob.value = null
  recordedBlobUrl.value = ''
  recordTime.value = 0
  
  historyForm.agent_id = null
  historyForm.selected_message_id = null
  historyMessages.value = []
  
  if (uploadFormRef.value) {
    uploadFormRef.value.resetFields()
  }
  if (uploadRef.value) {
    uploadRef.value.clearFiles()
  }
}

// Play sample
const handlePlaySample = async (row) => {
  try {
    const response = await api.get(`/user/speaker-groups/samples/${row.id}/audio`, {
      responseType: 'blob'
    })
    const blobUrl = URL.createObjectURL(response.data)
    audioPlayer.value.src = blobUrl
    audioPlayer.value.play()
    
    // Clean up blob URL after playback
    audioPlayer.value.onended = () => {
      URL.revokeObjectURL(blobUrl)
    }
  } catch (error) {
    console.error('Failed to play sample:', error)
    ElMessage.error('Failed to play sample')
  }
}

// Download sample
const handleDownloadSample = async (row) => {
  try {
    const response = await api.get(`/user/speaker-groups/samples/${row.id}/audio`, {
      responseType: 'blob'
    })
    const blobUrl = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = row.file_name || `sample_${row.id}.wav`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(blobUrl)
    
    ElMessage.success('Download started')
  } catch (error) {
    console.error('Failed to download sample:', error)
    ElMessage.error('Failed to download sample')
  }
}

// Delete sample
const handleDeleteSample = async (row) => {
  try {
    await ElMessageBox.confirm(
      'Are you sure you want to delete this sample?',
      'Confirm Delete',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )
    
    await api.delete(`/user/speaker-groups/${currentGroup.value.id}/samples/${row.id}`)
    ElMessage.success('Delete successful')
    await loadSamples(currentGroup.value.id)
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Failed to delete sample:', error)
      ElMessage.error('Failed to delete sample')
    }
  }
}

// Copy to clipboard
const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('Copied to clipboard')
  } catch (error) {
    console.error('Failed to copy:', error)
    ElMessage.error('Failed to copy')
  }
}

// Truncate text
const truncateText = (text, length) => {
  if (!text) return ''
  if (text.length <= length) return text
  return text.substring(0, length) + '...'
}

// Truncate ID
const truncateId = (id) => {
  if (!id) return ''
  if (id.length <= 8) return id
  return id.substring(0, 4) + '...' + id.substring(id.length - 4)
}

// Format file size
const formatFileSize = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Format date
const formatDate = (dateString) => {
  if (!dateString) return '-'
  const date = new Date(dateString)
  return date.toLocaleString('zh-CN')
}

// Reset group form
const resetGroupForm = () => {
  currentGroup.value = null
  groupForm.agent_id = null
  groupForm.name = ''
  groupForm.prompt = ''
  groupForm.description = ''
  groupForm.tts_config_id = null
  groupForm.voice = null
  currentVoiceOptions.value = []
}

onMounted(async () => {
  await Promise.all([
    loadAgents(),
    loadTtsConfigs(),
    loadSpeakerGroups()
  ])
})

onBeforeUnmount(() => {
  // Clean up blob URLs
  if (recordedBlobUrl.value) {
    URL.revokeObjectURL(recordedBlobUrl.value)
  }
  if (verifyRecordedBlobUrl.value) {
    URL.revokeObjectURL(verifyRecordedBlobUrl.value)
  }
})
</script>

<style scoped>
.speakers-page {
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
  color: #333;
}

.page-subtitle {
  margin: 5px 0 0 0;
  color: #666;
  font-size: 14px;
}

.filter-bar {
  margin-bottom: 20px;
  display: flex;
  align-items: center;
}

.speakers-content {
  background: white;
  border-radius: 8px;
  padding: 20px;
}

.empty-state {
  padding: 60px 0;
  text-align: center;
}

.prompt-text {
  color: #666;
  font-size: 14px;
}

.text-muted {
  color: #999;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.clone-voice-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.clone-voice-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 16px;
  background: #fff;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
}

.clone-voice-item:hover {
  border-color: #67c23a;
  color: #67c23a;
}

.clone-voice-item.active {
  background: #67c23a;
  border-color: #67c23a;
  color: #fff;
}

.form-help {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.sample-drawer {
  padding: 20px;
}

.group-info-card {
  margin-bottom: 20px;
}

.group-info h3 {
  margin: 0 0 10px 0;
  color: #333;
}

.prompt-section,
.description-section {
  margin-top: 10px;
}

.prompt-section strong,
.description-section strong {
  color: #666;
}

.prompt-section p,
.description-section p {
  margin: 5px 0 0 0;
  color: #333;
  white-space: pre-wrap;
}

.samples-section {
  margin-top: 20px;
}

.samples-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.samples-header h4 {
  margin: 0;
  color: #333;
}

.samples-header-actions {
  display: flex;
  gap: 10px;
}

.empty-samples {
  padding: 40px 0;
  text-align: center;
}

.uuid-text {
  font-family: monospace;
  color: #666;
}

.message-content {
  color: #333;
  font-size: 14px;
}

.empty-history {
  padding: 40px 0;
  text-align: center;
}

.audio-upload {
  width: 100%;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 4px;
}

.file-size {
  color: #909399;
  font-size: 12px;
}

.record-section {
  padding: 20px;
}

.record-status {
  text-align: center;
  margin-bottom: 20px;
}

.record-ready p {
  margin: 10px 0;
  color: #666;
}

.record-tip {
  color: #909399;
  font-size: 12px;
}

.record-recording {
  text-align: center;
}

.recording-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-bottom: 10px;
}

.recording-dot {
  width: 12px;
  height: 12px;
  background: #f56c6c;
  border-radius: 50%;
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.recording-text {
  color: #f56c6c;
  font-weight: bold;
}

.record-time {
  font-size: 24px;
  font-weight: bold;
  color: #333;
  margin: 10px 0;
}

.record-complete {
  text-align: center;
}

.record-complete p {
  margin: 10px 0;
  color: #666;
}

.record-preview {
  width: 100%;
  margin-top: 10px;
}

.record-controls {
  display: flex;
  justify-content: center;
  gap: 10px;
}

.verify-result {
  margin-top: 20px;
}

.result-content {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 20px;
  border-radius: 8px;
}

.result-success {
  background: #f0f9ff;
  border: 1px solid #409eff;
}

.result-failed {
  background: #fef0f0;
  border: 1px solid #f56c6c;
}

.result-info {
  flex: 1;
}

.result-status {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 10px;
}

.result-success .result-status {
  color: #409eff;
}

.result-failed .result-status {
  color: #f56c6c;
}

.result-details {
  margin-bottom: 10px;
}

.result-details div {
  margin: 5px 0;
  color: #666;
}

.result-message {
  color: #909399;
  font-size: 14px;
}
</style>
