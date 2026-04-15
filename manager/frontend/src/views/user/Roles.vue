<template>
  <div class="roles-page">
    <div class="page-header">
      <h2>My Roles</h2>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        Create Role
      </el-button>
    </div>

    <!-- Role Card List -->
    <el-row :gutter="12" class="roles-grid" v-loading="loading">
      <el-col :xs="24" :sm="12" :lg="8" v-for="role in userRoles" :key="role.id" class="role-col">
        <el-card class="role-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="role-name">{{ role.name }}</span>
              <el-dropdown @command="(cmd) => handleCardAction(cmd, role)">
                <el-icon class="more-icon"><MoreFilled /></el-icon>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="edit">
                      <el-icon><Edit /></el-icon>
                      Edit
                    </el-dropdown-item>
                  <el-dropdown-item command="duplicate">
                    <el-icon><CopyDocument /></el-icon>
                    Duplicate
                  </el-dropdown-item>
                  <el-dropdown-item command="toggle-status">
                    <el-icon><SwitchButton /></el-icon>
                    {{ isRoleActive(role) ? 'Disable' : 'Enable' }}
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided>
                    <el-icon><Delete /></el-icon>
                    Delete
                  </el-dropdown-item>
                </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>

          <div class="role-content">
            <p class="description">{{ role.description || 'No description' }}</p>

            <div class="role-config">
              <el-tag size="small" :type="isRoleActive(role) ? 'success' : 'info'">
                {{ isRoleActive(role) ? 'Enabled' : 'Disabled' }}
              </el-tag>
              <el-tag size="small" type="primary">LLM: {{ role.llm_config_id || 'Default' }}</el-tag>
              <el-tag size="small" type="success">TTS: {{ role.tts_config_id || 'Default' }}</el-tag>
              <el-tag v-if="role.voice" size="small" type="warning">Voice: {{ role.voice }}</el-tag>
            </div>

            <div class="role-prompt">
              <p class="prompt-label">Prompt</p>
              <p class="prompt-text">{{ role.prompt || 'No prompt set' }}</p>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Empty State -->
    <el-empty v-if="!loading && userRoles.length === 0" description="No roles yet, click the button in the top right to create">
      <el-button type="primary" @click="showCreateDialog = true">Create First Role</el-button>
    </el-empty>

    <!-- Create/Edit Role Dialog -->
    <el-dialog
      v-model="showCreateDialog"
      :title="editingRole ? 'Edit Role' : 'Create Role'"
      width="800px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <div class="dialog-sections">
          <section class="dialog-section">
            <h4 class="dialog-section-title">Basic Information</h4>
            <el-form-item label="Role Name" prop="name">
              <el-input v-model="form.name" placeholder="Please enter role name" />
            </el-form-item>

            <el-form-item label="Description" prop="description">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="3"
                placeholder="Please enter role description"
              />
            </el-form-item>
          </section>

          <el-divider />

          <section class="dialog-section">
            <h4 class="dialog-section-title">Prompt Configuration</h4>
            <el-form-item label="System Prompt" prop="prompt">
              <el-input
                v-model="form.prompt"
                type="textarea"
                :rows="6"
                placeholder="Please enter system prompt, used to define the role's behavior and personality"
              />
              <div class="prompt-tips">
                <el-text size="small" type="info">
                  Tip: You can use &#123;&#123;assistant_name&#125;&#125; as a placeholder for the agent name
                </el-text>
              </div>
            </el-form-item>
          </section>

          <el-divider />

          <section class="dialog-section">
            <h4 class="dialog-section-title">Model Configuration</h4>
            <el-form-item label="LLM Configuration">
              <el-select v-model="form.llm_config_id" placeholder="Please select LLM configuration (optional)" clearable style="width: 100%">
                <el-option
                  v-for="config in llmConfigs"
                  :key="config.id"
                  :label="`${config.name} (${config.config_id})`"
                  :value="config.config_id"
                  :disabled="!config.enabled"
                >
                  <span>{{ config.name }}</span>
                  <el-tag v-if="config.is_default" size="small" type="success" style="margin-left: 8px">Default</el-tag>
                </el-option>
              </el-select>
              <div class="form-tip">
                <el-text size="small" type="info">Leave empty to use default configuration</el-text>
              </div>
            </el-form-item>

            <el-form-item label="TTS Configuration">
              <el-select
                v-model="form.tts_config_id"
                placeholder="Please select TTS configuration (optional)"
                clearable
                style="width: 100%"
                @change="handleTtsConfigChange"
              >
                <el-option
                  v-for="config in ttsConfigs"
                  :key="config.id"
                  :label="`${config.name} (${config.config_id})`"
                  :value="config.config_id"
                  :disabled="!config.enabled"
                >
                  <span>{{ config.name }}</span>
                  <el-tag v-if="config.is_default" size="small" type="success" style="margin-left: 8px">Default</el-tag>
                </el-option>
              </el-select>
              <div class="form-tip">
                <el-text size="small" type="info">Leave empty to use default configuration</el-text>
              </div>
            </el-form-item>

            <el-form-item label="Voice" v-if="form.tts_config_id">
              <el-select
                v-model="form.voice"
                placeholder="Please select or enter voice (supports search and custom input)"
                clearable
                filterable
                allow-create
                default-first-option
                reserve-keyword
                :loading="voiceLoading"
                :filter-method="filterVoice"
                style="width: 100%"
              >
                <el-option
                  v-for="voice in filteredVoices"
                  :key="voice.value"
                  :label="voice.label"
                  :value="voice.value"
                >
                  <span>{{ voice.label }}</span>
                  <span style="color: #8492a6; font-size: 13px; margin-left: 8px;">{{ voice.value }}</span>
                </el-option>
              </el-select>
              <div class="form-tip">
                <el-text size="small" type="info">Voice list is automatically loaded based on current TTS configuration, you can search or manually enter a custom value</el-text>
              </div>
            </el-form-item>
          </section>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="handleDialogClose">Cancel</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          Save
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, MoreFilled, Edit, CopyDocument, Delete, SwitchButton } from '@element-plus/icons-vue'
import api from '../../utils/api'

const userRoles = ref([])
const loading = ref(false)
const saving = ref(false)
const showCreateDialog = ref(false)
const editingRole = ref(null)
const formRef = ref()

// Config lists
const llmConfigs = ref([])
const ttsConfigs = ref([])
const availableVoices = ref([])
const filteredVoices = ref([])
const voiceLoading = ref(false)
const previousTtsConfigId = ref(null)

const form = reactive({
  name: '',
  description: '',
  prompt: '',
  llm_config_id: null,
  tts_config_id: null,
  voice: ''
})

const rules = {
  name: [{ required: true, message: 'Please enter role name', trigger: 'blur' }],
  prompt: [{ required: true, message: 'Please enter system prompt', trigger: 'blur' }]
}

// Load role list
const loadRoles = async () => {
  loading.value = true
  try {
    const response = await api.get('/user/roles')
    userRoles.value = response.data.data?.user_roles || []
  } catch (error) {
    ElMessage.error('Failed to load roles')
  } finally {
    loading.value = false
  }
}

// Load config lists
const loadConfigs = async () => {
  try {
    const [llmRes, ttsRes] = await Promise.all([
      api.get('/user/llm-configs'),
      api.get('/user/tts-configs')
    ])
    llmConfigs.value = llmRes.data.data || []
    ttsConfigs.value = ttsRes.data.data || []
  } catch (error) {
    console.error('Failed to load config lists', error)
  }
}

// Card actions
const handleCardAction = (command, role) => {
  switch (command) {
    case 'edit':
      editRole(role)
      break
    case 'duplicate':
      duplicateRole(role)
      break
    case 'toggle-status':
      toggleRoleStatus(role)
      break
    case 'delete':
      deleteRole(role.id)
      break
  }
}

const isRoleActive = (role) => role?.status !== 'inactive'

const toggleRoleStatus = async (role) => {
  if (!role?.id) return

  const action = isRoleActive(role) ? 'Disable' : 'Enable'
  try {
    await api.patch(`/user/roles/${role.id}/toggle`)
    ElMessage.success(`Role ${action.toLowerCase()}d successfully`)
    await loadRoles()
  } catch (error) {
    ElMessage.error('Status toggle failed: ' + (error.response?.data?.error || error.message))
  }
}

const clearVoiceOptions = () => {
  availableVoices.value = []
  filteredVoices.value = []
}

const filterVoice = (val) => {
  if (!val) {
    filteredVoices.value = availableVoices.value
    return
  }

  const keyword = val.toLowerCase()
  filteredVoices.value = availableVoices.value.filter((voice) =>
    voice.label.toLowerCase().includes(keyword) || voice.value.toLowerCase().includes(keyword)
  )
}

const loadVoices = async (provider) => {
  if (!provider) {
    clearVoiceOptions()
    return
  }

  voiceLoading.value = true
  try {
    const params = { provider }
    if (form.tts_config_id) {
      params.config_id = form.tts_config_id
    }
    const response = await api.get('/user/voice-options', { params })
    availableVoices.value = response.data.data || []
    filteredVoices.value = availableVoices.value
  } catch (error) {
    clearVoiceOptions()
    console.error('Failed to load voice list', error)
  } finally {
    voiceLoading.value = false
  }
}

const handleTtsConfigChange = async () => {
  let previousProvider = null
  if (previousTtsConfigId.value) {
    const prevConfig = ttsConfigs.value.find((config) => config.config_id === previousTtsConfigId.value)
    previousProvider = prevConfig?.provider || null
  }

  if (!form.tts_config_id) {
    form.voice = ''
    previousTtsConfigId.value = null
    clearVoiceOptions()
    return
  }

  const ttsConfig = ttsConfigs.value.find((config) => config.config_id === form.tts_config_id)
  if (!ttsConfig || !ttsConfig.provider) {
    form.voice = ''
    previousTtsConfigId.value = form.tts_config_id
    clearVoiceOptions()
    return
  }

  if (previousProvider && previousProvider !== ttsConfig.provider) {
    form.voice = ''
  }

  await loadVoices(ttsConfig.provider)

  if (form.voice && availableVoices.value.length > 0) {
    const voiceExists = availableVoices.value.some((voice) => voice.value === form.voice)
    if (!voiceExists) {
      form.voice = ''
    }
  }

  previousTtsConfigId.value = form.tts_config_id
}

const editRole = (role) => {
  editingRole.value = role
  Object.assign(form, {
    name: role.name,
    description: role.description || '',
    prompt: role.prompt || '',
    llm_config_id: role.llm_config_id || null,
    tts_config_id: role.tts_config_id || null,
    voice: role.voice || ''
  })
  previousTtsConfigId.value = form.tts_config_id
  handleTtsConfigChange()
  showCreateDialog.value = true
}

const duplicateRole = (role) => {
  editingRole.value = null
  Object.assign(form, {
    name: `${role.name} (Copy)`,
    description: role.description || '',
    prompt: role.prompt || '',
    llm_config_id: role.llm_config_id || null,
    tts_config_id: role.tts_config_id || null,
    voice: role.voice || ''
  })
  previousTtsConfigId.value = form.tts_config_id
  handleTtsConfigChange()
  showCreateDialog.value = true
}

const handleSave = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        const data = { ...form }

        if (editingRole.value) {
          await api.put(`/user/roles/${editingRole.value.id}`, data)
          ElMessage.success('Update successful')
        } else {
          await api.post('/user/roles', data)
          ElMessage.success('Create successful')
        }

        showCreateDialog.value = false
        loadRoles()
      } catch (error) {
        ElMessage.error('Save failed: ' + (error.response?.data?.error || error.message))
      } finally {
        saving.value = false
      }
    }
  })
}

const deleteRole = async (id) => {
  try {
    await ElMessageBox.confirm('Are you sure you want to delete this role?', 'Confirm', {
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel',
      type: 'warning'
    })

    await api.delete(`/user/roles/${id}`)
    ElMessage.success('Delete successful')
    loadRoles()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Delete failed')
    }
  }
}

const resetForm = () => {
  editingRole.value = null
  Object.assign(form, {
    name: '',
    description: '',
    prompt: '',
    llm_config_id: null,
    tts_config_id: null,
    voice: ''
  })
  previousTtsConfigId.value = null
  clearVoiceOptions()
}

const handleDialogClose = () => {
  showCreateDialog.value = false
  resetForm()
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

onMounted(() => {
  loadRoles()
  loadConfigs()
})
</script>

<style scoped>
.roles-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  color: #333;
}

.roles-grid {
  align-items: stretch;
}

.role-col {
  display: flex;
  min-width: 0;
}

.role-card {
  margin-bottom: 20px;
  width: 100%;
  max-width: 440px;
  border-radius: 12px;
  border: 1px solid #ebeef5;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.role-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.08);
}

:deep(.role-card .el-card__header) {
  padding: 12px 14px;
}

:deep(.role-card .el-card__body) {
  padding: 12px 14px 14px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.role-name {
  font-weight: 700;
  font-size: 15px;
  color: #111827;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.more-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 16px;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  color: #6b7280;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.more-icon:hover {
  color: #1f2937;
  background: #f3f4f6;
}

.role-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 170px;
}

.description {
  color: #4b5563;
  font-size: 14px;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.role-config {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.role-prompt {
  margin-top: auto;
  border: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 8px;
  padding: 8px 10px;
}

.prompt-label {
  margin: 0 0 4px 0;
  color: #6b7280;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.prompt-text {
  margin: 0;
  color: #374151;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.prompt-tips {
  margin-top: 4px;
}

.form-tip {
  margin-top: 4px;
}

.dialog-sections {
  display: flex;
  flex-direction: column;
}

.dialog-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dialog-section-title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 700;
  color: #374151;
}

:deep(.dialog-sections .el-divider--horizontal) {
  margin: 8px 0 16px;
}
</style>
