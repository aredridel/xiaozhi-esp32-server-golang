<template>
  <div class="mobile-more-page">
    <van-cell-group inset title="Common Functions">
      <van-cell
        v-for="item in commonItems"
        :key="item.path"
        :title="item.title"
        :label="item.desc"
        is-link
        @click="go(item.path)"
      />
    </van-cell-group>

    <template v-if="authStore.isAdmin">
      <van-cell-group inset title="Service Configuration">
        <van-cell
          v-for="item in serviceItems"
          :key="item.path"
          :title="item.title"
          is-link
          @click="go(item.path)"
        />
      </van-cell-group>

      <van-cell-group inset title="AI Configuration">
        <van-cell
          v-for="item in aiItems"
          :key="item.path"
          :title="item.title"
          is-link
          @click="go(item.path)"
        />
      </van-cell-group>

      <van-cell-group inset title="System Management">
        <van-cell
          v-for="item in systemItems"
          :key="item.path"
          :title="item.title"
          is-link
          @click="go(item.path)"
        />
      </van-cell-group>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const commonItems = computed(() => {
  if (authStore.isAdmin) {
    return [
      { title: 'Configuration Wizard', desc: 'Recommended for first-time deployment', path: '/admin/config-wizard' },
      { title: 'Resource Pool Stats', desc: 'View system resource pool usage', path: '/admin/pool-stats' }
    ]
  }

  return [
    { title: 'My Roles', desc: 'Manage personal role templates', path: '/user/roles' },
    { title: 'Voice Cloning', desc: 'Manage voice cloning tasks', path: '/voice-clones' },
    { title: 'My Knowledge Bases', desc: 'Manage knowledge base documents', path: '/user/knowledge-bases' }
  ]
})

const serviceItems = [
  { title: 'OTA Configuration', path: '/admin/ota-config' },
  { title: 'MQTT Configuration', path: '/admin/mqtt-config' },
  { title: 'MQTT Server Configuration', path: '/admin/mqtt-server-config' },
  { title: 'UDP Configuration', path: '/admin/udp-config' },
  { title: 'MCP Configuration', path: '/admin/mcp-config' },
  { title: 'MCP Market', path: '/admin/mcp-market' },
  { title: 'Speaker Recognition Configuration', path: '/admin/speaker-config' },
  { title: 'Chat Settings', path: '/admin/chat-settings' }
]

const aiItems = [
  { title: 'VAD Configuration', path: '/admin/vad-config' },
  { title: 'ASR Configuration', path: '/admin/asr-config' },
  { title: 'LLM Configuration', path: '/admin/llm-config' },
  { title: 'TTS Configuration', path: '/admin/tts-config' },
  { title: 'Vision Configuration', path: '/admin/vision-config' },
  { title: 'Memory Configuration', path: '/admin/memory-config' },
  { title: 'Knowledge Base Search Configuration', path: '/admin/knowledge-search-config' }
]

const systemItems = [
  { title: 'Global Roles', path: '/admin/global-roles' },
  { title: 'User Management', path: '/admin/users' },
  { title: 'Device Management', path: '/admin/devices' },
  { title: 'Agent Management', path: '/admin/agents' }
]

const go = (path) => {
  router.push(path)
}
</script>

<style scoped>
.mobile-more-page {
  padding: 12px 0 24px;
}

:deep(.van-cell-group) {
  margin-bottom: 12px;
  border-radius: 10px;
  overflow: hidden;
}

:deep(.van-cell-group__title) {
  font-weight: 600;
  color: #323233;
}
</style>
