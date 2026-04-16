<template>
  <div class="mobile-layout">
    <MobileNavBar
      :title="pageTitle"
      :show-back="showBack"
      class="mobile-nav-bar"
    >
      <template #right>
        <van-icon name="user-o" class="user-entry-icon" @click="handleUserClick" />
      </template>
    </MobileNavBar>
    
    <div class="mobile-content" :class="{ 'with-tabbar': showTabBar }">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </div>
    
    <MobileTabBar v-if="showTabBar" class="mobile-tabbar" />
    
    <!-- User menu popup -->
    <van-popup
      v-model:show="showUserMenu"
      position="bottom"
      :style="{ padding: '20px' }"
    >
      <div class="user-menu">
        <div class="user-info">
          <van-icon name="user-circle-o" size="48" />
          <div class="user-details">
            <div class="username">{{ authStore.user?.username }}</div>
            <div class="user-role">{{ roleText }}</div>
          </div>
        </div>
        <van-cell-group inset>
          <van-cell title="More Features" is-link @click="handleGoMore" />
          <van-cell v-if="authStore.isAdmin" title="Config Wizard" is-link @click="handleGoConfigWizard" />
          <van-cell title="Logout" is-link @click="handleLogout" />
        </van-cell-group>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast } from 'vant'
import MobileNavBar from './MobileNavBar.vue'
import MobileTabBar from './MobileTabBar.vue'
import { useAuthStore } from '../stores/auth'
import { isMobile } from '../utils/device'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const showUserMenu = ref(false)

// Page title
const pageTitle = computed(() => {
  return route.meta?.title || 'XiaoZhi Management System'
})

// Whether to show back button (shown when not on home page and not on tab bar pages)
const showBack = computed(() => {
  const hideBackPages = ['/dashboard', '/console', '/agents', '/user/speakers', '/more', '/login']
  const currentPath = route.path
  return !hideBackPages.some(path => currentPath === path || currentPath.startsWith(path + '/'))
})

// Whether to show bottom tab bar
const showTabBar = computed(() => {
  const hideTabBarPages = [
    '/login',
    '/setup',
    '/test',
    '/simple-login'
  ]
  const currentPath = route.path
  
  // Detail pages don't show tab bar
  if (currentPath.includes('/edit') || currentPath.includes('/detail') || currentPath.includes('/history')) {
    return false
  }
  
  return !hideTabBarPages.includes(currentPath)
})

// Role text
const roleText = computed(() => {
  return authStore.isAdmin ? 'Administrator' : 'Regular User'
})

// User icon click
const handleUserClick = () => {
  showUserMenu.value = true
}


const handleGoMore = () => {
  router.push('/more')
  showUserMenu.value = false
}

const handleGoConfigWizard = () => {
  router.push('/admin/config-wizard')
  showUserMenu.value = false
}

// Logout
const handleLogout = async () => {
  try {
    await showConfirmDialog({
      title: 'Confirm',
      message: 'Are you sure you want to logout?'
    })
    
    authStore.logout()
    showSuccessToast('Logged out successfully')
    router.push('/login')
    showUserMenu.value = false
  } catch {
    // User cancelled
  }
}

// Watch route changes, close user menu
watch(
  () => route.path,
  () => {
    showUserMenu.value = false
  }
)
</script>

<style scoped>
.mobile-layout {
  height: 100dvh;
  background-color: #f7f8fa;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.mobile-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 0;
  -webkit-overflow-scrolling: touch;
}

.mobile-content.with-tabbar {
  padding-bottom: calc(50px + env(safe-area-inset-bottom));   /* Reserve space for bottom tab bar */
}

/* Page transition animation */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}


.user-entry-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  line-height: 1;
}

/* User menu styles */
.user-menu {
  padding: 10px 0;
}

.user-info {
  display: flex;
  align-items: center;
  padding: 20px;
  margin-bottom: 16px;
}

.user-info .van-icon {
  margin-right: 16px;
  color: #409EFF;
}

.user-details {
  flex: 1;
}

.username {
  font-size: 18px;
  font-weight: 500;
  color: #323233;
  margin-bottom: 4px;
}

.user-role {
  font-size: 14px;
  color: #969799;
}

:deep(.van-popup) {
  border-radius: 12px 12px 0 0;
}
</style>
