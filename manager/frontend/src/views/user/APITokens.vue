<template>
  <div class="api-tokens-page">
    <div class="page-header">
      <div>
        <h2>API Token Management</h2>
        <p class="page-subtitle">Used to access /api/open/v1 external interfaces, plaintext is only displayed once during creation.</p>
      </div>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        Create Token
      </el-button>
    </div>

    <el-alert type="info" :closable="false" show-icon>
      <template #title>
        Supports two calling methods: Authorization: Bearer &lt;token&gt; or X-API-Token: &lt;token&gt;
      </template>
    </el-alert>

    <el-card class="table-card" shadow="never">
      <el-table :data="tokens" v-loading="loading" empty-text="No Tokens, please create one first">
        <el-table-column prop="name" label="Name" min-width="180" />
        <el-table-column prop="token_prefix" label="Prefix" min-width="140" />
        <el-table-column label="Status" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'">{{ row.is_active ? 'Active' : 'Revoked' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Last Used" min-width="170">
          <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
        </el-table-column>
        <el-table-column label="Expires At" min-width="170">
          <template #default="{ row }">{{ formatTime(row.expires_at) }}</template>
        </el-table-column>
        <el-table-column label="Created At" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="Actions" width="120" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="danger"
              :disabled="!row.is_active"
              @click="handleRevoke(row)"
            >
              Revoke
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreate" title="Create API Token" width="480px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="Token Name" prop="name">
          <el-input v-model="form.name" maxlength="100" placeholder="e.g., Production Environment Call" />
        </el-form-item>
        <el-form-item label="Valid Days">
          <el-input-number v-model="form.expires_in_days" :min="0" :max="3650" />
          <div class="form-tip">0 means never expires</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">Cancel</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">Create</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showPlainToken" title="Please Save Token Immediately" width="640px">
      <el-alert type="warning" :closable="false" show-icon>
        Plaintext Token cannot be viewed again later, please copy and save it securely immediately.
      </el-alert>
      <el-input class="token-input" v-model="latestToken" type="textarea" :rows="3" readonly />
      <template #footer>
        <el-button @click="showPlainToken = false">Close</el-button>
        <el-button type="primary" @click="copyToken">Copy Token</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '../../utils/api'

const loading = ref(false)
const creating = ref(false)
const tokens = ref([])
const showCreate = ref(false)
const showPlainToken = ref(false)
const latestToken = ref('')
const formRef = ref()

const form = reactive({
  name: '',
  expires_in_days: 0
})

const rules = {
  name: [{ required: true, message: 'Please enter Token name', trigger: 'blur' }]
}

const formatTime = (val) => {
  if (!val) return '-'
  return new Date(val).toLocaleString()
}

const loadTokens = async () => {
  loading.value = true
  try {
    const res = await api.get('/user/api-tokens')
    tokens.value = res.data.data || []
  } finally {
    loading.value = false
  }
}

const openCreateDialog = () => {
  form.name = ''
  form.expires_in_days = 0
  showCreate.value = true
}

const handleCreate = async () => {
  if (!formRef.value) return
  await formRef.value.validate()

  creating.value = true
  try {
    const res = await api.post('/user/api-tokens', form)
    latestToken.value = res.data?.data?.token || ''
    showCreate.value = false
    showPlainToken.value = true
    ElMessage.success('Token created successfully')
    await loadTokens()
  } finally {
    creating.value = false
  }
}

const handleRevoke = async (row) => {
  await ElMessageBox.confirm(`Confirm revoke Token "${row.name}"?`, 'Tip', {
    confirmButtonText: 'Confirm',
    cancelButtonText: 'Cancel',
    type: 'warning'
  })
  await api.delete(`/user/api-tokens/${row.id}`)
  ElMessage.success('Token revoked')
  await loadTokens()
}

const copyToken = async () => {
  if (!latestToken.value) return
  await navigator.clipboard.writeText(latestToken.value)
  ElMessage.success('Token copied')
}

onMounted(loadTokens)
</script>

<style scoped>
.api-tokens-page { padding: 8px; }
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.page-subtitle { margin: 4px 0 0; color: #909399; }
.table-card { margin-top: 12px; }
.form-tip { color: #909399; font-size: 12px; margin-top: 6px; }
.token-input { margin-top: 12px; }
</style>
