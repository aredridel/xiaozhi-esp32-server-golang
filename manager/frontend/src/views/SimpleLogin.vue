<template>
  <div style="padding: 50px;">
    <h1>Simple Login Test</h1>
    <div style="max-width: 400px;">
      <div style="margin-bottom: 15px;">
        <label>Username:</label>
        <input v-model="username" type="text" style="width: 100%; padding: 8px;" />
      </div>
      <div style="margin-bottom: 15px;">
        <label>Password:</label>
        <input v-model="password" type="password" style="width: 100%; padding: 8px;" />
      </div>
      <button @click="login" style="width: 100%; padding: 10px; background: #409EFF; color: white; border: none;">
        Login
      </button>
    </div>
    <div style="margin-top: 20px;">
      <p>Debug Info:</p>
      <p>Auth Status: {{ authStore.isAuthenticated }}</p>
      <p>User Info: {{ JSON.stringify(authStore.user) }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('admin')
const password = ref('password')

const login = async () => {
  try {
    const result = await authStore.login({
      username: username.value,
      password: password.value
    })
    
    if (result.success) {
      alert('Login successful!')
      if (authStore.user?.role === 'admin') {
        router.push('/dashboard')
      } else {
        router.push('/agents')
      }
    } else {
      alert('Login failed: ' + result.message)
    }
  } catch (error) {
    alert('Login error: ' + error.message)
  }
}
</script>
