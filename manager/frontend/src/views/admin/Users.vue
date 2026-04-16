<template>
  <div class="config-page">
    <div class="page-header">
      <div class="header-left">
        <h2>User Management</h2>
      </div>
      <div class="header-right">
        <el-input
          v-model="searchKeyword"
          placeholder="Search users..."
          style="width: 200px; margin-right: 10px"
          prefix-icon="Search"
          clearable
        />
        <el-button type="primary" @click="openAddDialog">
          <el-icon><Plus /></el-icon>
          Add User
        </el-button>
      </div>
    </div>

    <el-table :data="filteredUserList" v-loading="tableLoading" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="Username" width="150" />
      <el-table-column prop="email" label="Email" width="200" />
      <el-table-column prop="role" label="Role" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'">
            {{ row.role === 'admin' ? 'Admin' : 'Regular User' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Created At" width="180">
        <template #default="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="360">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">Edit</el-button>
          <el-button size="small" type="success" @click="openQuotaDialog(row)" :disabled="row.role === 'admin'">Clone Quota</el-button>
          <el-button size="small" type="warning" @click="openResetPasswordDialog(row)">
            Reset Password
          </el-button>
          <el-button 
            size="small" 
            type="danger" 
            @click="handleDeleteUser(row)"
            :disabled="row.role === 'admin'"
          >
            Delete
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog 
      v-model="userDialogVisible" 
      :title="isEditMode ? 'Edit User' : 'Add User'"
      width="500px"
      @close="resetUserForm"
    >
      <el-form 
        ref="userFormRef" 
        :model="userForm" 
        :rules="userFormRules" 
        label-width="80px"
      >
        <el-form-item label="Username" prop="username">
          <el-input 
            v-model="userForm.username" 
            :disabled="isEditMode"
            placeholder="Please enter username"
          />
        </el-form-item>
        
        <el-form-item label="Email" prop="email">
          <el-input v-model="userForm.email" placeholder="Please enter email" />
        </el-form-item>
        
        <el-form-item v-if="!isEditMode" label="Password" prop="password">
          <el-input 
            v-model="userForm.password" 
            type="password" 
            placeholder="Please enter password (at least 6 characters)"
            show-password
          />
        </el-form-item>
        
        <el-form-item label="Role" prop="role">
          <el-select v-model="userForm.role" placeholder="Please select role" style="width: 100%">
            <el-option label="Regular User" value="user" />
            <el-option label="Admin" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="userDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleUserSubmit" :loading="userSubmitLoading">
          {{ isEditMode ? 'Save' : 'Add' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog 
      v-model="resetPasswordDialogVisible" 
      title="Reset Password" 
      width="400px"
      @close="resetPasswordForm"
    >
      <el-form 
        ref="passwordFormRef" 
        :model="passwordForm" 
        :rules="passwordFormRules" 
        label-width="80px"
      >
        <el-form-item label="User">
          <el-input v-model="currentUser.username" disabled />
        </el-form-item>
        
        <el-form-item label="New Password" prop="newPassword">
          <el-input 
            v-model="passwordForm.newPassword" 
            type="password" 
            placeholder="Please enter new password (at least 6 characters)"
            show-password
          />
        </el-form-item>
        
        <el-form-item label="Confirm" prop="confirmPassword">
          <el-input 
            v-model="passwordForm.confirmPassword" 
            type="password" 
            placeholder="Please enter new password again"
            show-password
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleResetPassword" :loading="resetPasswordLoading">
          Confirm Reset
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="quotaDialogVisible"
      :title="`Voice Clone Quota - ${quotaUser.username || ''}`"
      width="900px"
      @close="resetQuotaDialog"
    >
      <div class="quota-hint">Allocate clone quota by TTS config: -1 unlimited, 0 forbidden, positive integer for max clone count.</div>
      <el-table :data="quotaRows" v-loading="quotaLoading" style="margin-top: 12px">
        <el-table-column prop="tts_config_name" label="TTS Config Name" min-width="180" />
        <el-table-column prop="tts_config_id" label="TTS Config ID" min-width="180" />
        <el-table-column prop="provider" label="Provider" width="120" />
        <el-table-column label="Used" width="100">
          <template #default="{ row }">{{ row.used_count }}</template>
        </el-table-column>
        <el-table-column label="Remaining" width="100">
          <template #default="{ row }">{{ row.remaining_count < 0 ? 'Unlimited' : row.remaining_count }}</template>
        </el-table-column>
        <el-table-column label="Max Count" width="180">
          <template #default="{ row }">
            <el-input-number v-model="row.max_count" :min="-1" :step="1" :precision="0" controls-position="right" style="width: 140px" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="quotaDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="quotaSaving" @click="saveQuotaSettings">Save Quota</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const userList = ref([])
const tableLoading = ref(false)
const userDialogVisible = ref(false)
const resetPasswordDialogVisible = ref(false)
const userSubmitLoading = ref(false)
const resetPasswordLoading = ref(false)
const quotaDialogVisible = ref(false)
const quotaLoading = ref(false)
const quotaSaving = ref(false)
const quotaRows = ref([])
const quotaUser = ref({})
const quotaOriginalMaxMap = ref({})
const isEditMode = ref(false)
const currentUser = ref({})
const searchKeyword = ref('')

const filteredUserList = computed(() => {
  if (!searchKeyword.value) {
    return userList.value
  }
  return userList.value.filter(user => 
    user.username.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    user.email.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const userFormRef = ref()
const passwordFormRef = ref()

const userForm = reactive({
  username: '',
  email: '',
  password: '',
  role: ''
})

const passwordForm = reactive({
  newPassword: '',
  confirmPassword: ''
})

const userFormRules = {
  username: [
    { required: true, message: 'Please enter username', trigger: 'blur' }
  ],
  email: [
    { required: true, message: 'Please enter email', trigger: 'blur' },
    { type: 'email', message: 'Please enter a valid email format', trigger: 'blur' }
  ],
  password: [
    { required: true, message: 'Please enter password', trigger: 'blur' },
    { min: 6, message: 'Password must be at least 6 characters', trigger: 'blur' }
  ],
  role: [
    { required: true, message: 'Please select role', trigger: 'change' }
  ]
}

const passwordFormRules = {
  newPassword: [
    { required: true, message: 'Please enter new password', trigger: 'blur' },
    { min: 6, message: 'Password must be at least 6 characters', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: 'Please confirm password', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error('Passwords do not match'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const loadUserList = async () => {
  tableLoading.value = true
  try {
    const response = await api.get('/admin/users')
    userList.value = response.data.data || []
  } catch (error) {
    ElMessage.error('Failed to load user list')
  } finally {
    tableLoading.value = false
  }
}

const openAddDialog = () => {
  isEditMode.value = false
  userDialogVisible.value = true
}

const openEditDialog = (user) => {
  isEditMode.value = true
  currentUser.value = user
  userForm.username = user.username
  userForm.email = user.email
  userForm.role = user.role
  userDialogVisible.value = true
}

const resetUserForm = () => {
  userForm.username = ''
  userForm.email = ''
  userForm.password = ''
  userForm.role = ''
  currentUser.value = {}
  if (userFormRef.value) {
    userFormRef.value.resetFields()
  }
}

const handleUserSubmit = async () => {
  if (!userFormRef.value) return
  
  try {
    await userFormRef.value.validate()
    userSubmitLoading.value = true
    
    if (isEditMode.value) {
      await api.put(`/admin/users/${currentUser.value.id}`, {
        email: userForm.email,
        role: userForm.role
      })
      ElMessage.success('User updated successfully')
    } else {
      await api.post('/admin/users', {
        username: userForm.username,
        email: userForm.email,
        password: userForm.password,
        role: userForm.role
      })
      ElMessage.success('User added successfully')
    }
    
    userDialogVisible.value = false
    loadUserList()
  } catch (error) {
    ElMessage.error(isEditMode.value ? 'Failed to update user' : 'Failed to add user')
  } finally {
    userSubmitLoading.value = false
  }
}

const handleDeleteUser = async (user) => {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete user "${user.username}"?`,
      'Delete Confirmation',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )
    
    await api.delete(`/admin/users/${user.id}`)
    ElMessage.success('User deleted successfully')
    loadUserList()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Failed to delete user')
    }
  }
}

const openResetPasswordDialog = (user) => {
  currentUser.value = user
  resetPasswordDialogVisible.value = true
}

const openQuotaDialog = async (user) => {
  quotaUser.value = user
  quotaDialogVisible.value = true
  await loadQuotaSettings(user.id)
}

const loadQuotaSettings = async (userID) => {
  quotaLoading.value = true
  try {
    const response = await api.get(`/admin/users/${userID}/voice-clone-quotas`)
    const quotas = response.data?.data?.quotas || []
    quotaRows.value = quotas.map((item) => ({
      ...item,
      max_count: Number.isFinite(Number(item.max_count)) ? Number(item.max_count) : -1,
      used_count: Number(item.used_count || 0),
      remaining_count: Number.isFinite(Number(item.remaining_count)) ? Number(item.remaining_count) : -1
    }))
    quotaOriginalMaxMap.value = quotaRows.value.reduce((acc, row) => {
      acc[row.tts_config_id] = Number(row.max_count)
      return acc
    }, {})
  } catch (error) {
    ElMessage.error('Failed to load clone quota')
    quotaRows.value = []
    quotaOriginalMaxMap.value = {}
  } finally {
    quotaLoading.value = false
  }
}

const saveQuotaSettings = async () => {
  if (!quotaUser.value?.id) return
  const normalizedItems = quotaRows.value.map((row) => ({
    tts_config_id: row.tts_config_id,
    max_count: Number(row.max_count)
  }))
  for (const item of normalizedItems) {
    if (!item.tts_config_id) {
      ElMessage.error('Invalid tts_config_id exists')
      return
    }
    if (!Number.isInteger(item.max_count) || item.max_count < -1) {
      ElMessage.error('max_count must be an integer greater than or equal to -1')
      return
    }
  }

  const items = normalizedItems.filter((item) => quotaOriginalMaxMap.value[item.tts_config_id] !== item.max_count)
  if (items.length === 0) {
    ElMessage.info('Quota unchanged')
    return
  }

  quotaSaving.value = true
  try {
    await api.put(`/admin/users/${quotaUser.value.id}/voice-clone-quotas`, { items })
    ElMessage.success('Clone quota saved successfully')
    await loadQuotaSettings(quotaUser.value.id)
  } catch (error) {
    ElMessage.error('Failed to save clone quota')
  } finally {
    quotaSaving.value = false
  }
}

const resetQuotaDialog = () => {
  quotaRows.value = []
  quotaUser.value = {}
  quotaOriginalMaxMap.value = {}
}

const resetPasswordForm = () => {
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  if (passwordFormRef.value) {
    passwordFormRef.value.resetFields()
  }
}

const handleResetPassword = async () => {
  if (!passwordFormRef.value) return
  
  try {
    await passwordFormRef.value.validate()
    
    await ElMessageBox.confirm(
      `Are you sure you want to reset password for user "${currentUser.value.username}"?`,
      'Reset Password Confirmation',
      {
        confirmButtonText: 'Confirm',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )
    
    resetPasswordLoading.value = true
    
    await api.post(`/admin/users/${currentUser.value.id}/reset-password`, {
      new_password: passwordForm.newPassword
    })
    
    ElMessage.success('Password reset successfully')
    resetPasswordDialogVisible.value = false
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Failed to reset password')
    }
  } finally {
    resetPasswordLoading.value = false
  }
}

const formatDateTime = (dateString) => {
  if (!dateString) return '--'
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadUserList()
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

.header-right {
  display: flex;
  align-items: center;
}

.quota-hint {
  color: #666;
  font-size: 13px;
}
</style>
