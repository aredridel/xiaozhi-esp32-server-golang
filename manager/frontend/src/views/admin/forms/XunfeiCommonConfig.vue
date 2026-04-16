<template>
  <el-form-item label="App ID" :prop="`${prefix}.app_id`">
    <el-input v-model="modelValue[`${prefix}`].app_id" placeholder="Enter Xunfei App ID" />
  </el-form-item>
  <el-form-item label="API Key" :prop="`${prefix}.api_key`">
    <el-input v-model="modelValue[`${prefix}`].api_key" placeholder="Enter API Key" type="password" show-password />
  </el-form-item>
  <el-form-item label="API Secret" :prop="`${prefix}.api_secret`">
    <el-input v-model="modelValue[`${prefix}`].api_secret" placeholder="Enter API Secret" type="password" show-password />
  </el-form-item>
  <el-form-item label="WebSocket URL" :prop="`${prefix}.ws_url`">
    <el-input v-model="modelValue[`${prefix}`].ws_url" :placeholder="defaultWsUrl" />
  </el-form-item>
  <el-form-item label="Frame Duration" :prop="`${prefix}.frame_duration`">
    <el-input-number v-model="modelValue[`${prefix}`].frame_duration" :min="1" :max="1000" style="width: 100%" />
  </el-form-item>
  <el-form-item label="Connect Timeout" :prop="`${prefix}.connect_timeout`">
    <el-input-number v-model="modelValue[`${prefix}`].connect_timeout" :min="1" :max="60" style="width: 100%" />
  </el-form-item>
  <el-form-item label="Read Timeout" :prop="`${prefix}.read_timeout`">
    <el-input-number v-model="modelValue[`${prefix}`].read_timeout" :min="1" :max="300" style="width: 100%" />
  </el-form-item>
</template>

<script setup>
defineProps({
  modelValue: {
    type: Object,
    required: true
  },
  prefix: {
    type: String,
    required: true
  },
  defaultWsUrl: {
    type: String,
    default: ''
  }
})
</script>
