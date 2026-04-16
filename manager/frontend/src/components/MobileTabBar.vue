<template>
  <van-tabbar
    v-model="activeTab"
    @change="handleTabChange"
    fixed
    placeholder
    safe-area-inset-bottom
    class="mobile-tabbar"
  >
    <van-tabbar-item
      v-for="tab in tabs"
      :key="tab.name"
      :icon="tab.icon"
      :name="tab.name"
    >
      {{ tab.label }}
    </van-tabbar-item>
  </van-tabbar>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const activeTab = ref('')

// Define tab bar based on user role
const tabs = computed(() => {
  if (authStore.isAdmin) {
    // Admin tab bar
    return [
      { name: 'dashboard', label: 'Home', icon: 'home-o', path: '/dashboard' },
      { name: 'config', label: 'Config', icon: 'setting-o', path: '/admin/vad-config' },
      { name: 'manage', label: 'Manage', icon: 'apps-o', path: '/admin/users' },
      { name: 'more', label: 'More', icon: 'ellipsis', path: '/more' }
    ]
  } else {
    // Regular user tab bar
    return [
      { name: 'console', label: 'Home', icon: 'home-o', path: '/console' },
      { name: 'agents', label: 'Agents', icon: 'apps-o', path: '/agents' },
      { name: 'speakers', label: 'Voiceprints', icon: 'user-o', path: '/user/speakers' },
      { name: 'more', label: 'More', icon: 'ellipsis', path: '/more' }
    ]
  }
})

// Set active tab based on current route
const updateActiveTab = () => {
  const currentPath = route.path
  const currentTab = tabs.value.find(tab => {
    if (tab.path === currentPath) {
      return true
    }
    // Support path prefix matching
    if (currentPath.startsWith(tab.path)) {
      return true
    }
    return false
  })
  
  if (currentTab) {
    activeTab.value = currentTab.name
  }
}

// Tab change handler
const handleTabChange = (name) => {
  const tab = tabs.value.find(item => item.name === name)
  if (tab && tab.path !== route.path) {
    router.push(tab.path)
  }
}

// Watch route changes
watch(
  () => route.path,
  () => {
    updateActiveTab()
  },
  { immediate: true }
)

onMounted(() => {
  updateActiveTab()
})
</script>

<style scoped>
.mobile-tabbar {
  border-top: 1px solid #ebedf0;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.1);
}

:deep(.van-tabbar) {
  z-index: 1200;
}

:deep(.van-tabbar-item--active) {
  color: #409EFF;
}
</style>
