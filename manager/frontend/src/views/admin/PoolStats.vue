<template>
  <div class="pool-stats">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>Resource Pool Statistics</span>
          <div class="header-actions">
            <el-button type="primary" size="small" @click="refreshStats">
              <el-icon><Refresh /></el-icon>
              Refresh
            </el-button>
            <el-select v-model="viewType" size="small" style="width: 120px; margin-left: 10px;" disabled>
              <el-option label="Latest Data" value="latest" />
            </el-select>
          </div>
        </div>
      </template>

      <!-- Statistics Summary -->
      <el-row :gutter="20" style="margin-bottom: 20px;">
        <el-col :span="6">
          <el-statistic title="Total Records" :value="summary.total_records || 0" />
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Storage Method</div>
            <div class="stat-value">Latest Only</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Earliest Time</div>
            <div class="stat-value">{{ formatTime(summary.oldest_timestamp) }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Latest Time</div>
            <div class="stat-value">{{ formatTime(summary.newest_timestamp) }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- Latest Statistics Data -->
      <div v-if="viewType === 'latest' && latestStats">
        <el-divider>Latest Statistics ({{ formatTime(latestStats.timestamp) }})</el-divider>
        <el-table :data="formatStatsData(latestStats.stats)" border stripe style="width: 100%" v-if="latestStats.stats">
          <el-table-column prop="poolKey" label="Resource Pool" width="200" />
          <el-table-column prop="total" label="Total Resources" width="120" />
          <el-table-column prop="available" label="Available Resources" width="120" />
          <el-table-column prop="inUse" label="In Use" width="120" />
          <el-table-column prop="maxSize" label="Max Capacity" width="120" />
          <el-table-column prop="minSize" label="Min Capacity" width="120" />
          <el-table-column prop="maxIdle" label="Max Idle" width="120" />
          <el-table-column prop="isClosed" label="Status" width="100">
            <template #default="{ row }">
              <el-tag :type="row.isClosed ? 'danger' : 'success'">
                {{ row.isClosed ? 'Closed' : 'Running' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- Empty State -->
      <el-empty v-if="!latestStats" description="No statistics data available" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import api from '@/utils/api'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

const viewType = ref('latest')
const latestStats = ref(null)
const summary = ref({
  total_records: 0,
  storage_duration: 'Only save latest data',
  oldest_timestamp: null,
  newest_timestamp: null
})

let refreshTimer = null

onMounted(() => {
  loadSummary()
  loadStats()
  // Auto refresh every 30 seconds
  refreshTimer = setInterval(() => {
    loadStats()
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})

// Load statistics summary
const loadSummary = async () => {
  try {
    const response = await api.get('/admin/pool/stats/summary')
    // Backend returns format: { data: { data: {...} } }
    summary.value = response.data?.data || {}
  } catch (error) {
    console.error('Failed to load statistics summary:', error)
  }
}

// Load statistics data
const loadStats = async () => {
  try {
    const response = await api.get('/admin/pool/stats?type=latest')
    console.log('Latest statistics response:', response)
    // Backend returns format: { data: { timestamp: "...", stats: {...} } }
    // axios will automatically parse, so response.data is the backend returned { data: {...} }
    // Need to get one more layer of data
    latestStats.value = response.data?.data || response.data || null
    console.log('Parsed latest data:', latestStats.value)
  } catch (error) {
    console.error('Failed to load statistics data:', error)
    ElMessage.error('Failed to load statistics data')
  }
}

// Refresh statistics data
const refreshStats = () => {
  loadSummary()
  loadStats()
  ElMessage.success('Refresh successful')
}

// Format statistics data
const formatStatsData = (stats) => {
  if (!stats || typeof stats !== 'object') {
    return []
  }

  const result = []
  for (const [poolKey, poolStats] of Object.entries(stats)) {
    if (poolStats && typeof poolStats === 'object') {
      result.push({
        poolKey,
        total: poolStats.total_resources || 0,
        available: poolStats.available_resources || 0,
        inUse: poolStats.in_use_resources || 0,
        maxSize: poolStats.max_size || 0,
        minSize: poolStats.min_size || 0,
        maxIdle: poolStats.max_idle || 0,
        isClosed: poolStats.is_closed || false
      })
    }
  }
  return result
}

// Format time
const formatTime = (timestamp) => {
  if (!timestamp) {
    return '-'
  }
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

</script>

<style scoped>
.pool-stats {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}

.el-statistic {
  text-align: center;
}

.el-timeline {
  padding-left: 20px;
}

.stat-item {
  text-align: center;
  padding: 10px;
}

.stat-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}
</style>
