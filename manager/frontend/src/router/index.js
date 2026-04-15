import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { isMobile } from '../utils/device'

// Dynamically load components based on device type
const getLoginComponent = () => {
  return isMobile()
    ? import('../views/mobile/MobileLogin.vue')
    : import('../views/Login.vue')
}

const routes = [
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('../views/Setup.vue')
  },
  {
    path: '/test',
    name: 'Test',
    component: () => import('../views/Test.vue')
  },
  {
    path: '/test-route',
    name: 'TestRoute',
    component: () => import('../views/TestRoute.vue')
  },
  {
    path: '/simple-login',
    name: 'SimpleLogin',
    component: () => import('../views/SimpleLogin.vue')
  },
  {
    path: '/login',
    name: 'Login',
    component: getLoginComponent
  },

  {
    path: '/openapi-docs',
    name: 'OpenAPIDocs',
    component: () => import('../views/OpenAPIDocs.vue'),
    meta: { title: 'OpenAPI Documentation' }
  },
  {
    path: '/',
    name: 'Layout',
    component: () => import('../components/Layout.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: '/dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: 'Dashboard' }
      },
      // Admin routes
      {
        path: '/admin',
        name: 'Admin',
        meta: { requiresAuth: true, requiresAdmin: true },
        children: [
          {
            path: 'config-wizard',
            name: 'ConfigWizard',
            component: () => import('../views/admin/ConfigWizard.vue'),
            meta: { title: 'Config Wizard' }
          },
          {
            path: 'vad-config',
            name: 'VADConfig',
            component: () => import('../views/admin/VADConfig.vue'),
            meta: { title: 'VAD Config Management' }
          },
          {
            path: 'asr-config',
            name: 'ASRConfig',
            component: () => import('../views/admin/ASRConfig.vue'),
            meta: { title: 'ASR Config Management' }
          },
          {
            path: 'llm-config',
            name: 'LLMConfig',
            component: () => import('../views/admin/LLMConfig.vue'),
            meta: { title: 'LLM Config Management' }
          },
          {
            path: 'tts-config',
            name: 'TTSConfig',
            component: () => import('../views/admin/TTSConfig.vue'),
            meta: { title: 'TTS Config Management' }
          },
          {
            path: 'speaker-config',
            name: 'SpeakerConfig',
            component: () => import('../views/admin/SpeakerConfig.vue'),
            meta: { title: 'Voiceprint Config Management' }
          },
          {
            path: 'ota-config',
            name: 'OTAConfig',
            component: () => import('../views/admin/OTAConfig.vue'),
            meta: { title: 'OTA Config Management' }
          },
          {
            path: 'mqtt-config',
            name: 'MQTTConfig',
            component: () => import('../views/admin/MQTTConfig.vue'),
            meta: { title: 'MQTT Config Management' }
          },
          {
            path: 'udp-config',
            name: 'UDPConfig',
            component: () => import('../views/admin/UDPConfig.vue'),
            meta: { title: 'UDP Config Management' }
          },
          {
            path: 'mqtt-server-config',
            name: 'MQTTServerConfig',
            component: () => import('../views/admin/MQTTServerConfig.vue'),
            meta: { title: 'MQTT Server Config Management' }
          },
          {
            path: 'mcp-config',
            name: 'MCPConfig',
            component: () => import('../views/admin/MCPConfig.vue'),
            meta: { title: 'MCP Config Management' }
          },
          {
            path: 'mcp-market',
            name: 'MCPMarket',
            component: () => import('../views/admin/MCPMarket.vue'),
            meta: { title: 'MCP Market' }
          },
          {
            path: 'memory-config',
            name: 'MemoryConfig',
            component: () => import('../views/admin/MemoryConfig.vue'),
            meta: { title: 'Memory Config Management' }
          },
          {
            path: 'knowledge-search-config',
            name: 'KnowledgeSearchConfig',
            component: () => import('../views/admin/KnowledgeSearchConfig.vue'),
            meta: { title: 'Knowledge Base Search Config' }
          },
          {
            path: 'chat-settings',
            name: 'ChatSettings',
            component: () => import('../views/admin/ChatSettings.vue'),
            meta: { title: 'Chat Settings' }
          },
          {
            path: 'vision-config',
            name: 'VisionConfig',
            component: () => import('../views/admin/VisionConfig.vue'),
            meta: { title: 'Vision Config Management' }
          },
          {
            path: 'pool-stats',
            name: 'PoolStats',
            component: () => import('../views/admin/PoolStats.vue'),
            meta: { title: 'Resource Pool Stats' }
          },
          {
            path: 'global-roles',
            name: 'GlobalRoles',
            component: () => import('../views/admin/GlobalRoles.vue'),
            meta: { title: 'Global Roles Management' }
          },
          {
            path: 'users',
            name: 'Users',
            component: () => import('../views/admin/Users.vue'),
            meta: { title: 'User Management' }
          },
          {
            path: 'devices',
            name: 'AdminDevices',
            component: () => import('../views/admin/Devices.vue'),
            meta: { title: 'Device Management' }
          },
          {
            path: 'agents',
            name: 'AdminAgents',
            component: () => import('../views/admin/Agents.vue'),
            meta: { title: 'Agent Management' }
          }
        ]
      },
      // User routes
      {
        path: '/console',
        name: 'UserConsole',
        component: () => import('../views/user/UserConsole.vue'),
        meta: { title: 'User Console' }
      },
      {
        path: '/agents',
        name: 'Agents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: 'My Agents' }
      },
      {
        path: '/user/agents',
        name: 'UserAgents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: 'My Agents' }
      },
      {
        path: '/agents/:id/edit',
        name: 'AgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: 'Edit Agent' }
      },
      {
        path: '/user/agents/:id/edit',
        name: 'UserAgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: 'Edit Agent' }
      },
      {
        path: '/user/agents/:id/devices',
        name: 'AgentDevices',
        component: () => import('../views/user/AgentDevices.vue'),
        meta: { title: 'Agent Device Management' }
      },
      {
        path: '/speakers',
        name: 'Speakers',
        component: () => import('../views/user/Speakers.vue'),
        meta: { title: 'Voiceprint Management' }
      },
      {
        path: '/user/speakers',
        name: 'UserSpeakers',
        component: () => import('../views/user/Speakers.vue'),
        meta: { title: 'Voiceprint Management' }
      },
      {
        path: '/voice-clones',
        name: 'VoiceClones',
        component: () => import('../views/user/VoiceClones.vue'),
        meta: { title: 'Voice Cloning' }
      },
      {
        path: '/more',
        name: 'MobileMore',
        component: () => import('../views/mobile/MobileMore.vue'),
        meta: { title: 'More Features' }
      },
      {
        path: '/user/agents/:id/history',
        name: 'AgentHistory',
        component: () => import('../views/user/AgentHistory.vue'),
        meta: { title: 'Chat History' }
      },

      {
        path: '/user/api-tokens',
        name: 'UserAPITokens',
        component: () => import('../views/user/APITokens.vue'),
        meta: { title: 'API Token Management' }
      },
      {
        path: '/user/knowledge-bases',
        name: 'UserKnowledgeBases',
        component: () => import('../views/user/KnowledgeBases.vue'),
        meta: { title: 'My Knowledge Bases' }
      },
      {
        path: 'user/roles',
        name: 'UserRoles',
        component: () => import('../views/user/Roles.vue'),
        meta: { title: 'My Roles' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  
  // If accessing setup page, allow directly
  if (to.path === '/setup') {
    next()
    return
  }
  
  // If accessing login page and already logged in, redirect based on role (admin first login goes to config wizard)
  if (to.path === '/login' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      if (!localStorage.getItem('admin_first_login_done')) {
        next('/admin/config-wizard')
      } else {
        next('/dashboard')
      }
    } else {
      next('/console')
    }
    return
  }
  
  // If authentication required
  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      // No token, redirect to login
      next('/login')
      return
    }
    
    // Has token but no user info, try to validate token
    if (!authStore.user && !authStore.isValidating) {
      try {
        await authStore.getProfile()
      } catch (error) {
        // If 401 error (invalid token), redirect to login
        if (error.response?.status === 401) {
          next('/login')
          return
        }
        // If network error (backend connection failed), allow access (will show error)
        if (error.code === 'ERR_NETWORK' || error.message?.includes('Failed to fetch') || error.message?.includes('ERR_CONNECTION_REFUSED')) {
          // On network error, if local user info exists, allow access
          if (!authStore.user) {
            next('/login')
            return
          }
          // Note: Don't call next() here, let code continue to final next()
        } else {
          // Other errors, allow access (backend may be temporarily unavailable)
          // Note: Don't call next() here, let code continue to final next()
        }
      }
    }
    
    // If validating, wait for completion (max 2 seconds)
    if (authStore.isValidating) {
      let waitCount = 0
      while (authStore.isValidating && waitCount < 20) {
        await new Promise(resolve => setTimeout(resolve, 100))
        waitCount++
      }
    }
  }
  
  // If accessing root path, redirect based on role (admin first login goes to config wizard)
  if (to.path === '/' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      if (!localStorage.getItem('admin_first_login_done')) {
        next('/admin/config-wizard')
      } else {
        next('/dashboard')
      }
    } else {
      next('/console')
    }
    return
  }
  
  // If regular user accesses admin page, redirect to user console
  if (to.meta.requiresAdmin && authStore.user?.role !== 'admin') {
    next('/console')
    return
  }
  
  next()
})

export default router
