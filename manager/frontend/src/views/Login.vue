<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <h2>XiaoZhi Management System</h2>
        </div>
      </template>
      
      <el-tabs v-model="activeTab" class="login-tabs">
        <el-tab-pane label="Login" name="login">
          <el-form
            ref="loginFormRef"
            :model="loginForm"
            :rules="loginRules"
            label-width="80px"
          >
            <el-form-item label="Username" prop="username">
              <el-input v-model="loginForm.username" placeholder="Please enter username" />
            </el-form-item>
            <el-form-item label="Password" prop="password">
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="Please enter password"
                @keyup.enter="handleLogin"
              />
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="loading"
                @click="handleLogin"
                style="width: 100%"
              >
                Login
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
        
        <el-tab-pane label="Register" name="register">
          <el-form
            ref="registerFormRef"
            :model="registerForm"
            :rules="registerRules"
            label-width="80px"
          >
            <el-form-item label="Username" prop="username">
              <el-input v-model="registerForm.username" placeholder="Please enter username" />
            </el-form-item>
            <el-form-item label="Email" prop="email">
              <el-input v-model="registerForm.email" placeholder="Please enter email" />
            </el-form-item>
            <el-form-item label="Password" prop="password">
              <el-input
                v-model="registerForm.password"
                type="password"
                placeholder="Please enter password"
              />
            </el-form-item>
            <el-form-item label="Confirm Password" prop="confirmPassword">
              <el-input
                v-model="registerForm.confirmPassword"
                type="password"
                placeholder="Please confirm password"
                @keyup.enter="handleRegister"
              />
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="loading"
                @click="handleRegister"
                style="width: 100%"
              >
                Register
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
      <div class="public-links">
        <router-link to="/openapi-docs">View Public OpenAPI Documentation</router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import { getPostLoginRedirectPath } from '../utils/authRedirect'
import { checkNeedsSetup } from '../utils/setupStatus'

const router = useRouter()
const authStore = useAuthStore()

const activeTab = ref('login')
const loading = ref(false)
const loginFormRef = ref()
const registerFormRef = ref()

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

const loginRules = {
  username: [{ required: true, message: 'Please enter username', trigger: 'blur' }],
  password: [{ required: true, message: 'Please enter password', trigger: 'blur' }]
}

const registerRules = {
  username: [{ required: true, message: 'Please enter username', trigger: 'blur' }],
  email: [
    { required: true, message: 'Please enter email', trigger: 'blur' },
    { type: 'email', message: 'Please enter a valid email format', trigger: 'blur' }
  ],
  password: [
    { required: true, message: 'Please enter password', trigger: 'blur' },
    { min: 6, message: 'Password must be at least 6 characters', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: 'Please confirm password', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== registerForm.password) {
          callback(new Error('Passwords do not match'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  await loginFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      const result = await authStore.login(loginForm)
      loading.value = false
      
      if (result.success) {
        ElMessage.success('Login successful')
        router.push(getPostLoginRedirectPath(authStore.user))
      } else {
        ElMessage.error(result.message)
      }
    }
  })
}

const handleRegister = async () => {
  if (!registerFormRef.value) return
  
  await registerFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      const result = await authStore.register(registerForm)
      loading.value = false
      
      if (result.success) {
        ElMessage.success('Registration successful, please login')
        activeTab.value = 'login'
        Object.assign(registerForm, {
          username: '',
          email: '',
          password: '',
          confirmPassword: ''
        })
      } else {
        ElMessage.error(result.message)
      }
    }
  })
}

// Check system status, redirect to setup if not initialized
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
 .public-links {
  margin-top: 8px;
  text-align: center;
  font-size: 13px;
}
.public-links a {
  color: #409EFF;
  text-decoration: none;
}
.public-links a:hover {
  text-decoration: underline;
}

.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  width: 400px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
}

.card-header {
  text-align: center;
}

.card-header h2 {
  margin: 0;
  color: #333;
}

.login-tabs {
  margin-top: 20px;
}
</style>