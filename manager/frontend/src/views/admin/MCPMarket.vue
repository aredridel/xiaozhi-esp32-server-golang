<template>
  <div class="mcp-market-page">
    <div class="page-header">
      <h2>MCP Market</h2>
      <p class="subtitle">Connect to multiple MCP markets and import available SSE/StreamableHTTP services</p>
    </div>

    <el-tabs v-model="activeTab" class="market-tabs">
      <el-tab-pane name="discover">
        <template #label>
          <span>Market Discovery</span>
        </template>

        <el-row :gutter="16">
          <el-col :xs="24" :lg="11">
            <el-card shadow="never" class="panel-card">
              <template #header>
                <div class="panel-header">
                  <span>MCP Markets</span>
                  <div>
                    <el-button type="primary" size="small" @click="openCreateDialog">Add Connection</el-button>
                    <el-button size="small" @click="loadMarkets">
                      <el-icon><Refresh /></el-icon>
                    </el-button>
                  </div>
                </div>
              </template>

              <el-table :data="markets" stripe v-loading="marketsLoading" height="560">
                <el-table-column prop="name" label="Name" min-width="140" />
                <el-table-column prop="provider_id" label="Provider" width="130">
                  <template #default="{ row }">
                    <el-tag size="small">{{ row.provider_id || 'generic' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="catalog_url" label="Catalog URL" min-width="220" show-overflow-tooltip />
                <el-table-column label="Auth" width="120">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.has_token ? 'success' : 'info'">
                      {{ row.auth_type || 'none' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="Status" width="90">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.enabled ? 'success' : 'info'">
                      {{ row.enabled ? 'Enabled' : 'Disabled' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="Actions" width="96" fixed="right">
                  <template #default="{ row }">
                    <el-dropdown trigger="click" @command="(cmd) => handleMarketAction(cmd, row)">
                      <el-button link type="primary" class="market-action-btn">
                        <el-icon><MoreFilled /></el-icon>
                      </el-button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item command="edit">Edit</el-dropdown-item>
                          <el-dropdown-item command="test">Test</el-dropdown-item>
                          <el-dropdown-item command="delete" divided>Delete</el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="13">
            <el-card shadow="never" class="panel-card">
              <template #header>
                <div class="panel-header">
                  <span>Aggregated Service List</span>
                  <div class="search-actions">
                    <el-input
                      v-model="serviceQuery"
                      placeholder="Search service name/description/ID"
                      clearable
                      size="small"
                      style="width: 240px"
                      @keyup.enter="loadServices(1)"
                    >
                      <template #append>
                        <el-button @click="loadServices(1)">
                          <el-icon><Search /></el-icon>
                        </el-button>
                      </template>
                    </el-input>
                    <el-button size="small" @click="loadServices(servicePage)">
                      <el-icon><Refresh /></el-icon>
                    </el-button>
                  </div>
                </div>
              </template>

              <el-table :data="services" stripe v-loading="servicesLoading" height="500">
                <el-table-column prop="name" label="Service" min-width="180" show-overflow-tooltip />
                <el-table-column prop="market_name" label="Source Market" min-width="120" show-overflow-tooltip />
                <el-table-column prop="service_id" label="Service ID" min-width="180" show-overflow-tooltip />
                <el-table-column label="Actions" width="90" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" @click.stop="loadServiceDetail(row)">Details</el-button>
                  </template>
                </el-table-column>
              </el-table>

              <div class="pagination-wrap">
                <el-pagination
                  layout="prev, pager, next, total"
                  :current-page="servicePage"
                  :page-size="servicePageSize"
                  :total="serviceTotal"
                  @current-change="loadServices"
                />
              </div>

              <el-alert
                v-if="serviceWarnings.length > 0"
                type="warning"
                :closable="false"
                title="Some markets failed to fetch"
                class="warning-alert"
              >
                <template #default>
                  <div v-for="(warn, idx) in serviceWarnings" :key="idx">{{ warn }}</div>
                </template>
              </el-alert>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane name="imported">
        <template #label>
          <div class="tab-label-with-badge">
            <span>Imported Services</span>
            <el-badge :value="importedTotal" :max="999" class="tab-badge" />
          </div>
        </template>

        <el-card shadow="never" class="panel-card">
          <template #header>
            <div class="panel-header">
              <span>Imported Services</span>
              <div class="search-actions">
                <el-input
                  v-model="importedQuery"
                  placeholder="Search name / service_id / URL"
                  clearable
                  size="small"
                  style="width: 320px"
                  @keyup.enter="loadImportedItems(1)"
                >
                  <template #append>
                    <el-button @click="loadImportedItems(1)">
                      <el-icon><Search /></el-icon>
                    </el-button>
                  </template>
                </el-input>
                <el-button size="small" @click="loadImportedItems(importedPage)">
                  <el-icon><Refresh /></el-icon>
                </el-button>
                <el-button type="primary" size="small" @click="openCreateImportedDialog">Add Service</el-button>
              </div>
            </div>
          </template>

          <el-table :data="importedItems" stripe v-loading="importedLoading" height="560">
            <el-table-column prop="name" label="Name" min-width="160" show-overflow-tooltip />
            <el-table-column prop="transport" label="Transport" width="140" />
            <el-table-column prop="url" label="URL" min-width="320" show-overflow-tooltip />
            <el-table-column prop="service_id" label="Service ID" min-width="180" show-overflow-tooltip />
            <el-table-column label="Tools" width="120">
              <template #default="{ row }">
                <el-tag size="small" :type="row.allowed_tools?.length ? 'warning' : 'info'">
                  {{ row.allowed_tools?.length ? `${row.allowed_tools.length} selected` : 'All Tools' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="provider_id" label="Provider" width="120">
              <template #default="{ row }">
                <el-tag size="small">{{ row.provider_id || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="enabled" label="Enabled" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.enabled ? 'success' : 'info'">
                  {{ row.enabled ? 'Enabled' : 'Disabled' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="updated_at" label="Updated At" width="180" />
            <el-table-column label="Actions" width="280" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openEditImportedDialog(row)">Edit</el-button>
                <el-button link type="primary" @click="openImportedToolsDialog(row)">Tool Selection</el-button>
                <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggleImportedEnabled(row)">
                  {{ row.enabled ? 'Disable' : 'Enable' }}
                </el-button>
                <el-button link type="danger" @click="deleteImportedItem(row)">Delete</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-wrap">
            <el-pagination
              layout="prev, pager, next, total"
              :current-page="importedPage"
              :page-size="importedPageSize"
              :total="importedTotal"
              @current-change="loadImportedItems"
            />
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="detailDialogVisible" title="Service Details" width="900px">
      <div v-loading="detailLoading">
        <el-empty v-if="!serviceDetail && !detailLoading" description="No service details available" />
        <template v-else-if="serviceDetail">
          <div class="detail-grid">
            <div><strong>Service:</strong>{{ serviceDetail.name || '-' }}</div>
            <div><strong>Source Market:</strong>{{ serviceDetail.market_name || '-' }}</div>
            <div><strong>Service ID:</strong>{{ serviceDetail.service_id || '-' }}</div>
          </div>
          <div v-if="serviceDetail.description" class="detail-desc">{{ serviceDetail.description }}</div>
          <el-table :data="serviceDetail.endpoints || []" size="small" stripe>
            <el-table-column prop="name" label="Resource Name" min-width="120" show-overflow-tooltip />
            <el-table-column prop="transport" label="Transport" width="140" />
            <el-table-column prop="url" label="URL" min-width="360" show-overflow-tooltip />
          </el-table>
        </template>
      </div>
      <template #footer>
        <el-button @click="detailDialogVisible = false">Close</el-button>
        <el-button type="primary" :loading="detailImporting" :disabled="!serviceDetail" @click="importFromDetail">
          Import Service Config and Hot Update
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importedDialogVisible" :title="editingImported ? 'Edit Imported Service' : 'Add Imported Service'" width="700px">
      <el-form ref="importedFormRef" :model="importedForm" :rules="importedRules" label-width="120px">
        <el-form-item label="Name" prop="name">
          <el-input v-model="importedForm.name" placeholder="Service display name" />
        </el-form-item>
        <el-form-item label="Enabled">
          <el-switch v-model="importedForm.enabled" />
        </el-form-item>
        <el-form-item label="Transport" prop="transport">
          <el-select v-model="importedForm.transport" style="width: 100%">
            <el-option label="SSE" value="sse" />
            <el-option label="StreamableHTTP" value="streamablehttp" />
          </el-select>
        </el-form-item>
        <el-form-item label="URL" prop="url">
          <el-input v-model="importedForm.url" placeholder="https://example.com/mcp" />
        </el-form-item>
        <el-form-item label="Source Market">
          <el-select v-model="importedForm.market_id" clearable filterable style="width: 100%" placeholder="Optional">
            <el-option v-for="item in markets" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Provider">
          <el-input v-model="importedForm.provider_id" placeholder="e.g. modelscope" />
        </el-form-item>
        <el-form-item label="Service ID">
          <el-input v-model="importedForm.service_id" placeholder="Upstream service ID (optional)" />
        </el-form-item>
        <el-form-item label="Service Name">
          <el-input v-model="importedForm.service_name" placeholder="Upstream service name (optional)" />
        </el-form-item>
        <el-form-item label="Headers (JSON)">
          <el-input
            v-model="importedHeadersText"
            type="textarea"
            :rows="4"
            placeholder='e.g. {"Authorization":"Bearer xxx"}'
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="importedDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="importedSaving" @click="saveImportedItem">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importedToolsDialogVisible" :title="toolDialogTitle" width="760px">
      <div class="tool-selector-card">
        <div class="tool-selector-header">
          <div class="tool-selector-meta">
            <span class="tool-selector-title">Tool Access Policy</span>
            <span class="tool-selector-tip">Empty list means allow all tools from this service.</span>
          </div>
          <div class="tool-selector-actions">
            <el-tag size="small" :type="importedToolMode === 'all' ? 'info' : 'warning'">
              {{ importedToolMode === 'all' ? 'All Tools' : `${importedSelectedTools.length} selected` }}
            </el-tag>
            <el-button size="small" :loading="importedToolsLoading" @click="refreshImportedTools">
              Discover Tools
            </el-button>
          </div>
        </div>

        <el-radio-group v-model="importedToolMode" size="small" class="tool-mode-group" @change="handleImportedToolModeChange">
          <el-radio-button label="all">All Tools</el-radio-button>
          <el-radio-button label="selected">Selected Tools</el-radio-button>
        </el-radio-group>

        <template v-if="importedToolMode === 'selected'">
          <div class="tool-picker-search">
            <el-input v-model="importedToolQuery" clearable placeholder="Search tool name or description">
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>

          <div v-if="filteredImportedToolOptions.length === 0" class="tool-picker-empty">
            {{ importedToolOptions.length === 0 ? 'No tools discovered yet, please click "Discover Tools" first.' : 'No matching tools found.' }}
          </div>
          <el-checkbox-group v-else v-model="importedSelectedTools" class="tool-grid">
            <el-checkbox
              v-for="tool in filteredImportedToolOptions"
              :key="tool.name"
              :label="tool.name"
              border
              class="tool-tile"
            >
              <div class="tool-tile-body">
                <span class="tool-tile-name">{{ tool.name }}</span>
                <span class="tool-tile-desc">{{ tool.description || 'No description' }}</span>
              </div>
            </el-checkbox>
          </el-checkbox-group>
        </template>
      </div>

      <template #footer>
        <el-button @click="importedToolsDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="importedSaving" @click="saveImportedToolSelection">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="marketDialogVisible" :title="editingMarket ? 'Edit MCP Market' : 'Add MCP Market'" width="640px">
      <el-form ref="marketFormRef" :model="marketForm" :rules="marketRules" label-width="130px">
        <el-form-item label="Provider">
          <el-select v-model="marketForm.provider_id" style="width: 100%" @change="handleProviderChange">
            <el-option v-for="provider in selectableProviderOptions" :key="provider.id" :label="provider.name" :value="provider.id" />
          </el-select>
          <div v-if="currentProvider?.description" class="provider-desc">{{ currentProvider.description }}</div>
        </el-form-item>
        <el-form-item label="Name" prop="name">
          <el-input v-model="marketForm.name" placeholder="e.g. ModelScope MCP Market" />
        </el-form-item>
        <el-form-item label="Catalog URL" prop="catalog_url">
          <el-input v-model="marketForm.catalog_url" placeholder="https://example.com/api/services" />
        </el-form-item>
        <el-form-item label="Detail URL Template" prop="detail_url_template">
          <el-input v-model="marketForm.detail_url_template" placeholder="https://example.com/api/services/{id} (optional)" />
        </el-form-item>
        <el-form-item label="Enabled">
          <el-switch v-model="marketForm.enabled" />
        </el-form-item>

        <el-divider>Authentication Configuration</el-divider>
        <el-form-item label="Token">
          <el-input
            v-model="marketForm.auth.token"
            :placeholder="editingMarket ? `Leave empty to keep current value (current: ${editingMarket.token_mask || 'Not set'})` : 'Please enter ModelScope Token'"
            show-password
            clearable
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="marketDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="marketSaving" @click="saveMarket">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, MoreFilled } from '@element-plus/icons-vue'
import api from '@/utils/api'

const activeTab = ref('discover')

const markets = ref([])
const marketsLoading = ref(false)
const providerOptions = ref([])
const marketDialogVisible = ref(false)
const marketSaving = ref(false)
const editingMarket = ref(null)
const marketFormRef = ref()

const marketForm = reactive({
  name: '',
  provider_id: 'modelscope',
  catalog_url: '',
  detail_url_template: '',
  enabled: true,
  auth: {
    type: 'bearer',
    token: '',
    header_name: 'Authorization'
  }
})

const marketRules = {
  name: [{ required: true, message: 'Please enter name', trigger: 'blur' }],
  catalog_url: [{ required: true, message: 'Please enter catalog URL', trigger: 'blur' }]
}

const selectableProviderOptions = computed(() => {
  return providerOptions.value.filter(item => item.id !== 'generic')
})

const currentProvider = computed(() => {
  return selectableProviderOptions.value.find(item => item.id === marketForm.provider_id) || null
})

const services = ref([])
const servicesLoading = ref(false)
const serviceWarnings = ref([])
const servicePage = ref(1)
const servicePageSize = ref(20)
const serviceTotal = ref(0)
const serviceQuery = ref('')
const detailDialogVisible = ref(false)
const detailLoading = ref(false)
const detailImporting = ref(false)
const serviceDetail = ref(null)

const importedLoading = ref(false)
const importedSaving = ref(false)
const importedDialogVisible = ref(false)
const importedToolsDialogVisible = ref(false)
const importedToolsLoading = ref(false)
const editingImported = ref(null)
const importedToolTarget = ref(null)
const importedFormRef = ref()
const importedItems = ref([])
const importedToolOptions = ref([])
const importedPage = ref(1)
const importedPageSize = ref(20)
const importedTotal = ref(0)
const importedQuery = ref('')
const importedHeadersText = ref('')
const importedToolMode = ref('all')
const importedToolQuery = ref('')
const importedSelectedTools = ref([])

const importedForm = reactive({
  name: '',
  enabled: true,
  transport: 'streamablehttp',
  url: '',
  allowed_tools: [],
  market_id: null,
  provider_id: '',
  service_id: '',
  service_name: ''
})

const importedRules = {
  name: [{ required: true, message: 'Please enter name', trigger: 'blur' }],
  transport: [{ required: true, message: 'Please select transport type', trigger: 'change' }],
  url: [{ required: true, message: 'Please enter URL', trigger: 'blur' }]
}

const toolDialogTitle = computed(() => {
  return importedToolTarget.value ? `Tool Selection · ${importedToolTarget.value.name}` : 'Tool Selection'
})

const filteredImportedToolOptions = computed(() => {
  const query = importedToolQuery.value.trim().toLowerCase()
  if (!query) return importedToolOptions.value
  return importedToolOptions.value.filter((tool) => {
    return tool.name.toLowerCase().includes(query) || (tool.description || '').toLowerCase().includes(query)
  })
})

const getDefaultProviderId = () => {
  if (selectableProviderOptions.value.length === 0) return 'modelscope'
  if (selectableProviderOptions.value.some(item => item.id === 'modelscope')) return 'modelscope'
  return selectableProviderOptions.value[0].id
}

const loadProviders = async () => {
  try {
    const resp = await api.get('/admin/mcp-market/providers')
    providerOptions.value = resp.data.data || []
    if (!marketForm.provider_id) {
      marketForm.provider_id = getDefaultProviderId()
    }
    if (!selectableProviderOptions.value.some(item => item.id === marketForm.provider_id)) {
      marketForm.provider_id = getDefaultProviderId()
    }
  } catch (error) {
    providerOptions.value = [{ id: 'modelscope', name: 'ModelScope' }]
    marketForm.provider_id = marketForm.provider_id || 'modelscope'
    ElMessage.error(error.response?.data?.error || 'Failed to load providers')
  }
}

const applyProviderPreset = (providerId, force = false) => {
  const provider = selectableProviderOptions.value.find(item => item.id === providerId)
  if (!provider) return

  if (force || !marketForm.catalog_url) {
    marketForm.catalog_url = provider.catalog_url || ''
  }
  if (force || !marketForm.detail_url_template) {
    marketForm.detail_url_template = provider.detail_url_template || ''
  }
  if (force || !marketForm.auth.type) {
    marketForm.auth.type = 'bearer'
  }
  marketForm.auth.header_name = 'Authorization'

  if (force) {
    marketForm.auth.token = ''
  }

  if (!editingMarket.value && (force || !marketForm.name) && provider.id === 'modelscope') {
    marketForm.name = 'ModelScope MCP Market'
  }
}

const handleProviderChange = (providerId) => {
  applyProviderPreset(providerId, true)
}

const handleMarketAction = async (command, row) => {
  if (command === 'edit') {
    openEditDialog(row)
    return
  }
  if (command === 'test') {
    await testMarket(row)
    return
  }
  if (command === 'delete') {
    await deleteMarket(row)
  }
}

const loadMarkets = async () => {
  marketsLoading.value = true
  try {
    const resp = await api.get('/admin/mcp-markets')
    markets.value = resp.data.data || []
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Failed to load MCP markets')
  } finally {
    marketsLoading.value = false
  }
}

const resetMarketForm = () => {
  marketForm.name = ''
  marketForm.provider_id = getDefaultProviderId()
  marketForm.catalog_url = ''
  marketForm.detail_url_template = ''
  marketForm.enabled = true
  marketForm.auth.type = 'bearer'
  marketForm.auth.token = ''
  marketForm.auth.header_name = 'Authorization'
}

const openCreateDialog = () => {
  editingMarket.value = null
  resetMarketForm()
  applyProviderPreset(marketForm.provider_id, true)
  marketDialogVisible.value = true
}

const openEditDialog = (row) => {
  editingMarket.value = row
  marketForm.name = row.name
  const rowProviderId = row.provider_id || getDefaultProviderId()
  marketForm.provider_id = selectableProviderOptions.value.some(item => item.id === rowProviderId)
    ? rowProviderId
    : getDefaultProviderId()
  marketForm.catalog_url = row.catalog_url
  marketForm.detail_url_template = row.detail_url_template || ''
  marketForm.enabled = !!row.enabled
  marketForm.auth.type = 'bearer'
  marketForm.auth.header_name = 'Authorization'
  marketForm.auth.token = ''
  marketDialogVisible.value = true
}

const saveMarket = async () => {
  if (!marketFormRef.value) return
  const valid = await marketFormRef.value.validate().catch(() => false)
  if (!valid) return

  const payload = {
    name: marketForm.name,
    provider_id: marketForm.provider_id,
    catalog_url: marketForm.catalog_url,
    detail_url_template: marketForm.detail_url_template,
    enabled: marketForm.enabled,
    auth: {
      type: 'bearer',
      token: marketForm.auth.token,
      header_name: 'Authorization'
    }
  }

  marketSaving.value = true
  try {
    if (editingMarket.value) {
      await api.put(`/admin/mcp-markets/${editingMarket.value.id}`, payload)
      ElMessage.success('Update successful')
    } else {
      await api.post('/admin/mcp-markets', payload)
      ElMessage.success('Create successful')
    }
    marketDialogVisible.value = false
    await loadMarkets()
    await loadServices(1)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Save failed')
  } finally {
    marketSaving.value = false
  }
}

const deleteMarket = async (row) => {
  try {
    await ElMessageBox.confirm(`Confirm delete MCP market "${row.name}"?`, 'Prompt', {
      type: 'warning',
      confirmButtonText: 'Delete',
      cancelButtonText: 'Cancel'
    })
    await api.delete(`/admin/mcp-markets/${row.id}`)
    ElMessage.success('Delete successful')
    await loadMarkets()
    await loadServices(1)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || 'Delete failed')
    }
  }
}

const testMarket = async (row) => {
  try {
    const resp = await api.post(`/admin/mcp-markets/${row.id}/test`)
    const count = resp.data?.data?.service_count ?? 0
    ElMessage.success(`Connection successful, discovered ${count} services`)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Connection test failed')
  }
}

const loadServices = async (page = 1) => {
  servicePage.value = page
  servicesLoading.value = true
  try {
    const resp = await api.get('/admin/mcp-market/services', {
      params: {
        q: serviceQuery.value,
        page: servicePage.value,
        page_size: servicePageSize.value
      }
    })
    const data = resp.data.data || {}
    services.value = data.items || []
    serviceTotal.value = data.total || 0
    serviceWarnings.value = data.warnings || []
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Failed to load aggregated services')
  } finally {
    servicesLoading.value = false
  }
}

const loadServiceDetail = async (row) => {
  detailDialogVisible.value = true
  detailLoading.value = true
  serviceDetail.value = null
  try {
    const resp = await api.get(`/admin/mcp-market/services/${row.market_id}/${encodeURIComponent(row.service_id)}`)
    serviceDetail.value = resp.data?.data || null
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Failed to load service details')
  } finally {
    detailLoading.value = false
  }
}

const importFromDetail = async () => {
  const row = serviceDetail.value
  if (!row?.market_id || !row?.service_id) {
    ElMessage.error('Service identifier missing, cannot import')
    return
  }

  detailImporting.value = true
  try {
    const payload = {
      market_id: row.market_id,
      service_id: row.service_id,
      name_override: ''
    }
    const resp = await api.post('/admin/mcp-market/import', payload)
    const result = resp.data.data || {}
    ElMessage.success(`Import successful: ${result.imported_count || 0} services applied`)
    await loadServices(servicePage.value)
    await loadImportedItems(1)
    detailDialogVisible.value = false
    activeTab.value = 'imported'
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Import failed')
  } finally {
    detailImporting.value = false
  }
}

const loadImportedItems = async (page = 1) => {
  importedPage.value = page
  importedLoading.value = true
  try {
    const resp = await api.get('/admin/mcp-market/imported-services', {
      params: {
        q: importedQuery.value,
        page: importedPage.value,
        page_size: importedPageSize.value
      }
    })
    const data = resp.data.data || {}
    importedItems.value = data.items || []
    importedTotal.value = data.total || 0
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Failed to load imported services')
  } finally {
    importedLoading.value = false
  }
}

const parseImportedHeaders = () => {
  const txt = importedHeadersText.value.trim()
  if (!txt) return null
  try {
    const parsed = JSON.parse(txt)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('headers must be a JSON object')
    }
    return parsed
  } catch (error) {
    throw new Error('Headers is not a valid JSON object')
  }
}

const resetImportedForm = () => {
  importedForm.name = ''
  importedForm.enabled = true
  importedForm.transport = 'streamablehttp'
  importedForm.url = ''
  importedForm.market_id = null
  importedForm.provider_id = ''
  importedForm.service_id = ''
  importedForm.service_name = ''
  importedHeadersText.value = ''
}

const mergeImportedToolOptions = (tools = [], selected = []) => {
  const merged = new Map()

  ;(tools || []).forEach((tool) => {
    if (!tool?.name) return
    merged.set(tool.name, {
      name: tool.name,
      description: tool.description || ''
    })
  })

  ;(selected || []).forEach((name) => {
    if (!name || merged.has(name)) return
    merged.set(name, {
      name,
      description: 'Currently selected in config'
    })
  })

  importedToolOptions.value = Array.from(merged.values()).sort((a, b) => a.name.localeCompare(b.name))
}

const loadImportedToolOptions = async (serviceId) => {
  if (!serviceId) {
    mergeImportedToolOptions([], importedSelectedTools.value)
    return
  }

  importedToolsLoading.value = true
  try {
    const resp = await api.get(`/admin/mcp-market/imported-services/${serviceId}/tools`)
    const data = resp.data?.data || {}
    mergeImportedToolOptions(data.tools || [], importedSelectedTools.value)
  } catch (error) {
    mergeImportedToolOptions([], importedSelectedTools.value)
    ElMessage.error(error.response?.data?.error || 'Failed to load tool list')
  } finally {
    importedToolsLoading.value = false
  }
}

const openCreateImportedDialog = () => {
  editingImported.value = null
  resetImportedForm()
  importedDialogVisible.value = true
}

const openEditImportedDialog = (row) => {
  editingImported.value = row
  importedForm.name = row.name || ''
  importedForm.enabled = !!row.enabled
  importedForm.transport = row.transport || 'streamablehttp'
  importedForm.url = row.url || ''
  importedForm.market_id = row.market_id || null
  importedForm.provider_id = row.provider_id || ''
  importedForm.service_id = row.service_id || ''
  importedForm.service_name = row.service_name || ''
  importedHeadersText.value = row.headers ? JSON.stringify(row.headers, null, 2) : ''
  importedDialogVisible.value = true
}

const syncImportedToolMode = (selected = importedSelectedTools.value) => {
  importedToolMode.value = selected.length > 0 ? 'selected' : 'all'
}

const handleImportedToolModeChange = (mode) => {
  importedToolQuery.value = ''
  if (mode === 'all') {
    importedSelectedTools.value = []
  }
}

const openImportedToolsDialog = async (row) => {
  importedToolTarget.value = row
  importedSelectedTools.value = Array.isArray(row.allowed_tools) ? [...row.allowed_tools] : []
  importedToolQuery.value = ''
  syncImportedToolMode(importedSelectedTools.value)
  mergeImportedToolOptions([], importedSelectedTools.value)
  importedToolsDialogVisible.value = true
  await loadImportedToolOptions(row.id)
}

const refreshImportedTools = async () => {
  if (!importedToolTarget.value?.id) {
    ElMessage.warning('Please select an imported service first')
    return
  }
  await loadImportedToolOptions(importedToolTarget.value.id)
}

const saveImportedItem = async () => {
  if (!importedFormRef.value) return
  const valid = await importedFormRef.value.validate().catch(() => false)
  if (!valid) return

  let headers = null
  try {
    headers = parseImportedHeaders()
  } catch (error) {
    ElMessage.error(error.message)
    return
  }

  const payload = {
    name: importedForm.name,
    enabled: importedForm.enabled,
    transport: importedForm.transport,
    url: importedForm.url,
    headers,
    allowed_tools: editingImported.value?.allowed_tools || [],
    market_id: importedForm.market_id || null,
    provider_id: importedForm.provider_id,
    service_id: importedForm.service_id,
    service_name: importedForm.service_name
  }

  importedSaving.value = true
  try {
    if (editingImported.value) {
      await api.put(`/admin/mcp-market/imported-services/${editingImported.value.id}`, payload)
      ElMessage.success('Update successful')
    } else {
      await api.post('/admin/mcp-market/imported-services', payload)
      ElMessage.success('Create successful')
    }
    importedDialogVisible.value = false
    await loadImportedItems(importedPage.value)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Save failed')
  } finally {
    importedSaving.value = false
  }
}

const saveImportedToolSelection = async () => {
  if (!importedToolTarget.value) return
  if (importedToolMode.value === 'selected' && importedSelectedTools.value.length === 0) {
    ElMessage.warning('Please select at least one tool, or switch to "All Tools"')
    return
  }

  const row = importedToolTarget.value
  const payload = {
    name: row.name,
    enabled: row.enabled,
    transport: row.transport,
    url: row.url,
    headers: row.headers || null,
    allowed_tools: importedToolMode.value === 'all' ? [] : importedSelectedTools.value,
    market_id: row.market_id || null,
    provider_id: row.provider_id || '',
    service_id: row.service_id || '',
    service_name: row.service_name || ''
  }

  importedSaving.value = true
  try {
    await api.put(`/admin/mcp-market/imported-services/${row.id}`, payload)
    ElMessage.success('Tool policy updated')
    importedToolsDialogVisible.value = false
    importedToolTarget.value = null
    importedToolQuery.value = ''
    await loadImportedItems(importedPage.value)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Save failed')
  } finally {
    importedSaving.value = false
  }
}

const toggleImportedEnabled = async (row) => {
  const payload = {
    name: row.name,
    enabled: !row.enabled,
    transport: row.transport,
    url: row.url,
    headers: row.headers || null,
    allowed_tools: row.allowed_tools || [],
    market_id: row.market_id || null,
    provider_id: row.provider_id || '',
    service_id: row.service_id || '',
    service_name: row.service_name || ''
  }
  try {
    await api.put(`/admin/mcp-market/imported-services/${row.id}`, payload)
    ElMessage.success(row.enabled ? 'Disabled' : 'Enabled')
    await loadImportedItems(importedPage.value)
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Failed to update status')
  }
}

const deleteImportedItem = async (row) => {
  try {
    await ElMessageBox.confirm(`Confirm delete imported service "${row.name}"?`, 'Prompt', {
      type: 'warning',
      confirmButtonText: 'Delete',
      cancelButtonText: 'Cancel'
    })
    await api.delete(`/admin/mcp-market/imported-services/${row.id}`)
    ElMessage.success('Delete successful')
    await loadImportedItems(importedPage.value)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || 'Delete failed')
    }
  }
}

onMounted(async () => {
  await loadProviders()
  await loadMarkets()
  await loadServices(1)
  await loadImportedItems(1)
})
</script>

<style scoped>
.mcp-market-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
  color: #1f2937;
}

.subtitle {
  margin-top: 6px;
  color: #6b7280;
  font-size: 14px;
}

.market-tabs {
  --el-tabs-header-height: 44px;
}

.tab-label-with-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.tab-badge {
  line-height: 1;
}

.market-action-btn {
  padding: 0;
  min-width: 22px;
}

.panel-card {
  margin-bottom: 16px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.search-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.pagination-wrap {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.warning-alert {
  margin-top: 12px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(180px, 1fr));
  gap: 8px 12px;
  margin-bottom: 10px;
}

.detail-desc {
  margin-bottom: 12px;
  color: #4b5563;
  line-height: 1.6;
}

.provider-desc {
  margin-top: 6px;
  line-height: 1.5;
  color: #6b7280;
  font-size: 12px;
}

.tool-selector-card {
  border: 1px solid #e5edf5;
  border-radius: 16px;
  padding: 16px;
  background: linear-gradient(180deg, #fbfdff 0%, #f4f8fc 100%);
}

.tool-selector-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.tool-selector-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tool-selector-title {
  color: #172033;
  font-size: 15px;
  font-weight: 600;
}

.tool-selector-tip {
  color: #667085;
  font-size: 12px;
  line-height: 1.5;
}

.tool-selector-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.tool-mode-group {
  margin-top: 14px;
}

.tool-picker-search {
  margin-top: 14px;
  max-width: 320px;
}

.tool-picker-empty {
  margin-top: 14px;
  border: 1px dashed #d6deeb;
  border-radius: 12px;
  padding: 18px 16px;
  color: #667085;
  font-size: 13px;
  text-align: center;
  background: rgba(255, 255, 255, 0.72);
}

.tool-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 14px;
}

.tool-tile {
  margin-right: 0;
  min-width: 0;
}

.tool-tile-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}

.tool-tile-name {
  display: -webkit-box;
  color: #182230;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-word;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.tool-tile-desc {
  display: -webkit-box;
  color: #667085;
  font-size: 12px;
  line-height: 1.45;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-word;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

:deep(.tool-grid .el-checkbox.is-bordered) {
  width: 100%;
  height: auto;
  margin-right: 0;
  padding: 12px 14px;
  border-radius: 14px;
  align-items: flex-start;
  background: rgba(255, 255, 255, 0.9);
  border-color: #d7e1ec;
}

:deep(.tool-grid .el-checkbox__label) {
  padding-left: 10px;
  width: 100%;
  min-width: 0;
  overflow: hidden;
}

:deep(.tool-grid .el-checkbox.is-bordered.is-checked) {
  background: #eff7ff;
  border-color: #7ab8ff;
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.08);
}

@media (max-width: 992px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }

  .search-actions {
    flex-wrap: wrap;
  }

  .panel-header {
    flex-wrap: wrap;
  }

  .tool-selector-header {
    flex-direction: column;
  }

  .tool-selector-actions {
    width: 100%;
    justify-content: space-between;
  }

  .tool-grid {
    grid-template-columns: 1fr;
  }

  .tool-picker-search {
    max-width: none;
  }
}
</style>
