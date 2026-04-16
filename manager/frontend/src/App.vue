<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/utils/api'

export default {
  name: 'App',
  setup() {
    const router = useRouter()

    const checkSystemStatus = async () => {
      try {
        // Check if system needs initialization
        const response = await api.get('/setup/status')
        
        if (response.data.needs_setup) {
          // If setup needed and not on setup page, redirect to setup
          if (router.currentRoute.value.path !== '/setup') {
            router.push('/setup')
          }
        }
      } catch (error) {
        console.error('Failed to check system status:', error)
        // If check fails, might be network issue, don't force redirect
      }
    }

    onMounted(() => {
      checkSystemStatus()
    })
  }
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #2c3e50;
  height: 100dvh;
}

html,
body {
  height: 100%;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}


/* Mobile styles optimization */
@media (max-width: 767px) {
  /* Mobile font size optimization */
  body {
    font-size: 14px;
    -webkit-text-size-adjust: 100%;
    -webkit-tap-highlight-color: transparent;
  }
  
  /* Mobile scroll optimization */
  * {
    -webkit-overflow-scrolling: touch;
  }
  
  /* Mobile touch delay optimization */
  a, button, input, textarea {
    touch-action: manipulation;
  }
  
  /* Hide desktop-only elements */
  .desktop-only {
    display: none !important;
  }
}

/* Desktop styles */
@media (min-width: 768px) {
  /* Hide mobile-only elements */
  .mobile-only {
    display: none !important;
  }
}

/* Global animation */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Mobile safe area adaptation */
@supports (padding: max(0px)) {
  .mobile-safe-top {
    padding-top: max(20px, env(safe-area-inset-top));
  }
  
  .mobile-safe-bottom {
    padding-bottom: max(20px, env(safe-area-inset-bottom));
  }
}
</style>
