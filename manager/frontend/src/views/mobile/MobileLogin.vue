<template>
  <div class="mobile-login-container">
    <div class="mobile-login-header">
      <h1>Xiaozhi Management System</h1>
      <p>Intelligent Voice Assistant Management Platform</p>
    </div>
    
    <van-tabs v-model:active="activeTab" class="mobile-login-tabs">
      <van-tab title="Login" name="login">
        <van-form @submit="handleLogin" class="mobile-login-form">
          <van-cell-group inset>
            <van-field
              v-model="loginForm.username"
              name="username"
              label="Username"
              placeholder="Please enter username"
              :rules="[{ required: true, message: 'Please enter username' }]"
            />
            <van-field
              v-model="loginForm.password"
              type="password"
              name="password"
              label="Password"
              placeholder="Please enter password"
              :rules="[{ required: true, message: 'Please enter password' }]"
            />
          </van-cell-group>
          
          <div class="mobile-login-actions">
            <van-button
              round
              block
              type="primary"
              native-type="submit"
              :loading="loading"
              loading-text="Logging in..."
              class="mobile-login-button"
            >
              Login
            </van-button>
          </div>
        </van-form>
      </van-tab>
      
      <van-tab title="Register" name="register">
        <van-form @submit="handleRegister" class="mobile-login-form">
          <van-cell-group inset>
            <van-field
              v-model="registerForm.username"
              name="username"
              label="Username"
              placeholder="Please enter username"
              :rules="[{ required: true, message: 'Please enter username' }]"
            />
            <van-field
              v-model="registerForm.email"
              name="email"
              label="Email"
              placeholder="Please enter email"
              :rules="[
                { required: true, message: 'Please enter email' },
                { pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: 'Please enter a valid email format' }
              ]"
            />
            <van-field
              v-model="registerForm.password"
              type="password"
              name="password"
              label="Password"
              placeholder="Please enter password (at least 6 characters)"
              :rules="[
                { required: true, message: 'Please enter password' },
                { pattern: /^.{6,}$/, message: 'Password must be at least 6 characters' }
              ]"
            />
            <van-field
              v-model="registerForm.confirmPassword"
              type="password"
              name="confirmPassword"
              label="Confirm Password"
              placeholder="Please confirm password"
              :rules="[
                { required: true, message: 'Please confirm password' },
                { validator: validateConfirmPassword }
              ]"
            />
          </van-cell-group>
          
          <div class="mobile-login-actions">
            <van-button
              round
              block
              type="primary"
              native-type="submit"
              :loading="loading"
              loading-text="Registering..."
              class="mobile-login-button"
            >
              Register
            </van-button>
          </div>
        </van-form>
      </van-tab>
    </van-tabs>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showSuccessToast, showFailToast } from 'vant'
import { useAuthStore } from '../../stores/auth'
import { getPostLoginRedirectPath } from '../../utils/authRedirect'
import { checkNeedsSetup } from '../../utils/setupStatus'

const router = useRouter()
const authStore = useAuthStore()

const activeTab = ref('login')
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const registerForm = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

// Custom validator: confirm password
const validateConfirmPassword = (val) => {
  if (val !== registerForm.password) {
    return 'Passwords do not match'
  }
  return true
}

const handleLogin = async () => {
  loading.value = true
  const result = await authStore.login(loginForm)
  loading.value = false
  
  if (result.success) {
    showSuccessToast('Login successful')
    router.push(getPostLoginRedirectPath(authStore.user))
  } else {
    showFailToast(result.message || 'Login failed')
  }
}

const handleRegister = async () => {
  loading.value = true
  const result = await authStore.register(registerForm)
  loading.value = false
  
  if (result.success) {
    showSuccessToast('Registration successful, please login')
    activeTab.value = 'login'
    // Clear registration form
    Object.assign(registerForm, {
      username: '',
      email: '',
      password: '',
      confirmPassword: ''
    })
  } else {
    showFailToast(result.message || 'Registration failed')
  }
}

// Check system status, redirect to setup page if not initialized
const checkSystemStatus = async () => {
  try {
    if (await checkNeedsSetup()) {
      router.push('/setup')
    }
  } catch (error) {
    console.error('Failed to check system status:', error)
  }
}

onMounted(() => {
  checkSystemStatus()
})
</script>

<style scoped>
.mobile-login-container {
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 40px 16px 20px;
  display: flex;
  flex-direction: column;
}

.mobile-login-header {
  text-align: center;
  color: white;
  margin-bottom: 30px;
}

.mobile-login-header h1 {
  font-size: 28px;
  font-weight: 600;
  margin-bottom: 8px;
}

.mobile-login-header p {
  font-size: 14px;
  opacity: 0.9;
}

.mobile-login-tabs {
  flex: 1;
  background: white;
  border-radius: 12px;
  overflow: hidden;
}

.mobile-login-form {
  padding: 20px 0;
}

.mobile-login-actions {
  padding: 20px 16px;
}

.mobile-login-button {
  height: 44px;
  font-size: 16px;
  font-weight: 500;
}

:deep(.van-tabs__nav) {
  background: white;
}

:deep(.van-tabs__line) {
  background: #409EFF;
}
</style>
