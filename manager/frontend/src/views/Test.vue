<template>
  <div class="test-page">
    <h1>Test Page</h1>
    <p>If you can see this page, the routing system is working properly.</p>
    <div class="debug-info">
      <h3>Debug Info:</h3>
      <p>Current Path: {{ $route.path }}</p>
      <p>Auth Status: {{ authStore.isAuthenticated }}</p>
      <p>User Info: {{ authStore.user }}</p>
      <p>Is Admin: {{ authStore.isAdmin }}</p>
    </div>
    <div class="actions">
      <el-button @click="$router.push('/login')">Go to Login</el-button>
      <el-button @click="$router.push('/dashboard')">Go to Dashboard</el-button>
      <el-button @click="$router.push('/agents')">Go to Agent List</el-button>
    </div>
  </div>
</template>

<script setup>
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
</script>

<style scoped>
.test-page {
  padding: 20px;
}

.debug-info {
  background: #f5f5f5;
  padding: 15px;
  border-radius: 4px;
  margin: 20px 0;
}

.actions {
  margin-top: 20px;
}

.actions .el-button {
  margin-right: 10px;
}
</style>