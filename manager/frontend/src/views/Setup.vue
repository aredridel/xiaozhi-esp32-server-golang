<template>
  <div class="setup-container">
    <div class="setup-card">
      <div class="setup-header">
        <h1>System Initialization</h1>
        <p>Welcome to XiaoZhi Management System, please complete the initial setup</p>
      </div>

      <!-- Check status -->
      <div v-if="!initialized" class="setup-status">
        <div class="loading-spinner" v-if="checking">
          <div class="spinner"></div>
          <p>Checking system status...</p>
        </div>
        
        <div v-else-if="needsSetup" class="setup-form">
          <h2>Create Administrator Account</h2>
          <p>Please set up administrator account information for system management</p>
          
          <form @submit.prevent="initializeSystem">
            <div class="form-group">
              <label for="username">Administrator Username</label>
              <input
                id="username"
                v-model="form.admin_username"
                type="text"
                required
                minlength="3"
                maxlength="50"
                placeholder="Please enter administrator username"
              />
            </div>
            
            <div class="form-group">
              <label for="email">Administrator Email</label>
              <input
                id="email"
                v-model="form.admin_email"
                type="email"
                required
                placeholder="Please enter administrator email"
              />
            </div>
            
            <div class="form-group">
              <label for="password">Administrator Password</label>
              <input
                id="password"
                v-model="form.admin_password"
                type="password"
                required
                minlength="6"
                maxlength="100"
                placeholder="Please enter administrator password (at least 6 characters)"
              />
            </div>
            
            <div class="form-group">
              <label for="confirmPassword">Confirm Password</label>
              <input
                id="confirmPassword"
                v-model="confirmPassword"
                type="password"
                required
                placeholder="Please enter password again"
              />
            </div>
            
            <div class="error-message" v-if="errorMessage">
              {{ errorMessage }}
            </div>
            
            <button type="submit" :disabled="initializing" class="setup-btn">
              <span v-if="initializing">Initializing...</span>
              <span v-else>Start Initialization</span>
            </button>
          </form>
        </div>
        
        <div v-else class="setup-complete">
          <div class="success-icon">✅</div>
          <h2>System Initialized</h2>
          <p>System initialization is complete, please login with administrator account</p>
          <router-link to="/login" class="login-btn">Go to Login</router-link>
        </div>
      </div>
      
      <!-- Initialization successful -->
      <div v-else class="setup-success">
        <div class="success-icon">🎉</div>
        <h2>Initialization Successful!</h2>
        <p>System has been successfully initialized, administrator account created</p>
        <div class="admin-info">
          <p><strong>Username:</strong>{{ adminInfo.username }}</p>
          <p><strong>Email:</strong>{{ adminInfo.email }}</p>
        </div>
        <router-link to="/login" class="login-btn">Go to Login</router-link>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '@/utils/api'

export default {
  name: 'Setup',
  setup() {
    const router = useRouter()
    const checking = ref(true)
    const needsSetup = ref(false)
    const initialized = ref(false)
    const initializing = ref(false)
    const errorMessage = ref('')
    
    const form = ref({
      admin_username: '',
      admin_email: '',
      admin_password: ''
    })
    
    const confirmPassword = ref('')
    const adminInfo = ref({})

    const checkSetupStatus = async () => {
      try {
        checking.value = true
        const response = await api.get('/setup/status')
        
        if (response.data.needs_setup) {
          needsSetup.value = true
        } else {
          // System initialized, redirect to login page
          router.push('/login')
        }
      } catch (error) {
        console.error('Failed to check system status:', error)
        errorMessage.value = 'Failed to check system status, please refresh and try again'
      } finally {
        checking.value = false
      }
    }

    const initializeSystem = async () => {
      // Validate password confirmation
      if (form.value.admin_password !== confirmPassword.value) {
        errorMessage.value = 'Passwords do not match'
        return
      }

      try {
        initializing.value = true
        errorMessage.value = ''
        
        const response = await api.post('/setup/initialize', form.value)
        
        adminInfo.value = response.data.admin
        initialized.value = true
      } catch (error) {
        console.error('System initialization failed:', error)
        if (error.response?.data?.error) {
          errorMessage.value = error.response.data.error
        } else {
          errorMessage.value = 'System initialization failed, please try again'
        }
      } finally {
        initializing.value = false
      }
    }

    onMounted(() => {
      checkSetupStatus()
    })

    return {
      checking,
      needsSetup,
      initialized,
      initializing,
      errorMessage,
      form,
      confirmPassword,
      adminInfo,
      initializeSystem
    }
  }
}
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.setup-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  padding: 40px;
  max-width: 500px;
  width: 100%;
}

.setup-header {
  text-align: center;
  margin-bottom: 30px;
}

.setup-header h1 {
  color: #333;
  margin-bottom: 10px;
  font-size: 28px;
}

.setup-header p {
  color: #666;
  font-size: 16px;
}

.loading-spinner {
  text-align: center;
  padding: 40px 0;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #667eea;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.setup-form h2 {
  color: #333;
  margin-bottom: 10px;
  text-align: center;
}

.setup-form p {
  color: #666;
  text-align: center;
  margin-bottom: 30px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  color: #333;
  font-weight: 500;
}

.form-group input {
  width: 100%;
  padding: 12px 16px;
  border: 2px solid #e1e5e9;
  border-radius: 8px;
  font-size: 16px;
  transition: border-color 0.3s;
  box-sizing: border-box;
}

.form-group input:focus {
  outline: none;
  border-color: #667eea;
}

.error-message {
  color: #e74c3c;
  background: #fdf2f2;
  border: 1px solid #fecaca;
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 20px;
  font-size: 14px;
}

.setup-btn {
  width: 100%;
  padding: 14px;
  background: #667eea;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.3s;
}

.setup-btn:hover:not(:disabled) {
  background: #5a6fd8;
}

.setup-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.setup-complete,
.setup-success {
  text-align: center;
  padding: 40px 0;
}

.success-icon {
  font-size: 48px;
  margin-bottom: 20px;
}

.setup-complete h2,
.setup-success h2 {
  color: #333;
  margin-bottom: 10px;
}

.setup-complete p,
.setup-success p {
  color: #666;
  margin-bottom: 30px;
}

.admin-info {
  background: #f8f9fa;
  padding: 20px;
  border-radius: 8px;
  margin-bottom: 30px;
  text-align: left;
}

.admin-info p {
  margin: 8px 0;
  color: #333;
}

.login-btn {
  display: inline-block;
  padding: 12px 24px;
  background: #667eea;
  color: white;
  text-decoration: none;
  border-radius: 8px;
  font-weight: 500;
  transition: background-color 0.3s;
}

.login-btn:hover {
  background: #5a6fd8;
}
</style>
