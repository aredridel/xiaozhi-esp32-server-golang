<template>
  <el-form ref="formRef" :model="model" :rules="rules" label-width="140px">
    <el-form-item label="Provider" prop="provider">
      <el-select v-model="model.provider" placeholder="Select provider" style="width: 100%" @change="onProviderChange">
        <el-option label="FunASR" value="funasr" />
        <el-option label="Aliyun FunASR" value="aliyun_funasr" />
        <el-option label="Doubao" value="doubao" />
        <el-option label="Aliyun Qwen3" value="aliyun_qwen3" />
        <el-option label="Xunfei" value="xunfei" />
      </el-select>
    </el-form-item>
    <el-form-item label="Config Name" prop="name">
      <el-input v-model="model.name" placeholder="Enter configuration name" />
    </el-form-item>
    <el-form-item label="Config ID" prop="config_id">
      <el-input v-model="model.config_id" placeholder="Enter unique configuration ID" />
    </el-form-item>
    <div v-if="model.provider === 'funasr'">
      <el-form-item label="Host" prop="funasr.host">
        <el-input v-model="model.funasr.host" placeholder="Enter host address" />
      </el-form-item>
      <el-form-item label="Port" prop="funasr.port">
        <el-input-number v-model="model.funasr.port" :min="1" :max="65535" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Mode" prop="funasr.mode">
        <el-select v-model="model.funasr.mode" placeholder="Select mode" style="width: 100%">
          <el-option label="2pass" value="2pass" />
          <el-option label="offline" value="offline" />
          <el-option label="online" value="online" />
        </el-select>
      </el-form-item>
      <el-form-item label="Sample Rate" prop="funasr.sample_rate">
        <el-select v-model="model.funasr.sample_rate" placeholder="Select sample rate" style="width: 100%">
          <el-option label="8000" :value="8000" />
          <el-option label="16000" :value="16000" />
          <el-option label="44100" :value="44100" />
          <el-option label="48000" :value="48000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Chunk Size" prop="funasr.chunk_size">
        <div style="display: flex; gap: 8px; width: 100%">
          <el-input-number v-model="model.funasr.chunk_size[0]" :min="1" placeholder="Forward" style="flex: 1" />
          <el-input-number v-model="model.funasr.chunk_size[1]" :min="1" placeholder="Middle" style="flex: 1" />
          <el-input-number v-model="model.funasr.chunk_size[2]" :min="1" placeholder="Backward" style="flex: 1" />
        </div>
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          Format: [Forward, Middle, Backward], e.g., [5, 10, 5]
        </div>
      </el-form-item>
      <el-form-item label="Chunk Interval" prop="funasr.chunk_interval">
        <el-input-number v-model="model.funasr.chunk_interval" :min="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Max Connections" prop="funasr.max_connections">
        <el-input-number v-model="model.funasr.max_connections" :min="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Timeout (seconds)" prop="funasr.timeout">
        <el-input-number v-model="model.funasr.timeout" :min="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Auto End" prop="funasr.auto_end">
        <el-switch v-model="model.funasr.auto_end" />
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          Ensure FunASR is configured accordingly
        </div>
      </el-form-item>
    </div>
    <div v-if="model.provider === 'aliyun_funasr'">
      <el-form-item label="API Key" prop="aliyun_funasr.api_key">
        <el-input v-model="model.aliyun_funasr.api_key" type="password" show-password placeholder="Can be empty, reads DASHSCOPE_API_KEY" />
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          Can be empty, defaults to DASHSCOPE_API_KEY
        </div>
      </el-form-item>
      <el-form-item label="WS URL" prop="aliyun_funasr.ws_url">
        <el-input v-model="model.aliyun_funasr.ws_url" placeholder="wss://dashscope.aliyuncs.com/api-ws/v1/inference/" />
      </el-form-item>
      <el-form-item label="Model" prop="aliyun_funasr.model">
        <el-input v-model="model.aliyun_funasr.model" placeholder="fun-asr-realtime" />
      </el-form-item>
      <el-form-item label="Audio Format" prop="aliyun_funasr.format">
        <el-select v-model="model.aliyun_funasr.format" placeholder="Select format" style="width: 100%">
          <el-option label="pcm" value="pcm" />
        </el-select>
      </el-form-item>
      <el-form-item label="Sample Rate" prop="aliyun_funasr.sample_rate">
        <el-select v-model="model.aliyun_funasr.sample_rate" placeholder="Select sample rate" style="width: 100%">
          <el-option label="16000" :value="16000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Vocabulary ID" prop="aliyun_funasr.vocabulary_id">
        <el-input v-model="model.aliyun_funasr.vocabulary_id" placeholder="Can be empty" />
      </el-form-item>
      <el-form-item label="Disfluency Removal" prop="aliyun_funasr.disfluency_removal_enabled">
        <el-switch v-model="model.aliyun_funasr.disfluency_removal_enabled" />
      </el-form-item>
      <el-form-item label="Timeout (seconds)" prop="aliyun_funasr.timeout">
        <el-input-number v-model="model.aliyun_funasr.timeout" :min="1" style="width: 100%" />
      </el-form-item>
    </div>
    <div v-if="model.provider === 'doubao'">
      <el-form-item label="App ID" prop="doubao.appid">
        <el-input v-model="model.doubao.appid" placeholder="Enter App ID" />
      </el-form-item>
      <el-form-item label="Access Token" prop="doubao.access_token">
        <el-input v-model="model.doubao.access_token" type="password" placeholder="Enter access token" show-password />
      </el-form-item>
      <el-form-item label="WebSocket URL" prop="doubao.ws_url">
        <el-input v-model="model.doubao.ws_url" placeholder="Enter WebSocket URL" />
      </el-form-item>
      <el-form-item label="Resource ID" prop="doubao.resource_id">
        <el-select v-model="model.doubao.resource_id" placeholder="Select resource specification" style="width: 100%">
          <el-option label="Doubao Streaming ASR Model 1.0 Hourly" value="volc.bigasr.sauc.duration" />
          <el-option label="Doubao Streaming ASR Model 1.0 Concurrent" value="volc.bigasr.sauc.concurrent" />
          <el-option label="Doubao Streaming ASR Model 2.0 Hourly" value="volc.seedasr.sauc.duration" />
          <el-option label="Doubao Streaming ASR Model 2.0 Concurrent" value="volc.seedasr.sauc.concurrent" />
        </el-select>
      </el-form-item>
      <el-form-item label="End Window Size" prop="doubao.end_window_size">
        <el-input-number v-model="model.doubao.end_window_size" :min="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Enable Punctuation" prop="doubao.enable_punc">
        <el-switch v-model="model.doubao.enable_punc" />
      </el-form-item>
      <el-form-item label="Enable ITN" prop="doubao.enable_itn">
        <el-switch v-model="model.doubao.enable_itn" />
      </el-form-item>
      <el-form-item label="Enable DDC" prop="doubao.enable_ddc">
        <el-switch v-model="model.doubao.enable_ddc" />
      </el-form-item>
      <el-form-item label="Chunk Duration (ms)" prop="doubao.chunk_duration">
        <el-input-number v-model="model.doubao.chunk_duration" :min="1" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Timeout (seconds)" prop="doubao.timeout">
        <el-input-number v-model="model.doubao.timeout" :min="1" style="width: 100%" />
      </el-form-item>
    </div>
    <div v-if="model.provider === 'xunfei'">
      <el-form-item label="App ID" prop="xunfei.appid">
        <el-input v-model="model.xunfei.appid" placeholder="Enter Xunfei App ID" />
      </el-form-item>
      <el-form-item label="API Key" prop="xunfei.api_key">
        <el-input v-model="model.xunfei.api_key" type="password" show-password placeholder="Enter Xunfei API Key" />
      </el-form-item>
      <el-form-item label="API Secret" prop="xunfei.api_secret">
        <el-input v-model="model.xunfei.api_secret" type="password" show-password placeholder="Enter Xunfei API Secret" />
      </el-form-item>
      <el-form-item label="Host" prop="xunfei.host">
        <el-input v-model="model.xunfei.host" placeholder="iat-api.xfyun.cn" />
      </el-form-item>
      <el-form-item label="Path" prop="xunfei.path">
        <el-input v-model="model.xunfei.path" placeholder="/v2/iat" />
      </el-form-item>
      <el-form-item label="Domain" prop="xunfei.domain">
        <el-input v-model="model.xunfei.domain" placeholder="iat" />
      </el-form-item>
      <el-form-item label="Language" prop="xunfei.language">
        <el-input v-model="model.xunfei.language" placeholder="zh_cn" />
      </el-form-item>
      <el-form-item label="Accent" prop="xunfei.accent">
        <el-input v-model="model.xunfei.accent" placeholder="mandarin" />
      </el-form-item>
      <el-form-item label="Sample Rate" prop="xunfei.sample_rate">
        <el-select v-model="model.xunfei.sample_rate" placeholder="Select sample rate" style="width: 100%">
          <el-option label="16000" :value="16000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Timeout (seconds)" prop="xunfei.timeout">
        <el-input-number v-model="model.xunfei.timeout" :min="1" style="width: 100%" />
      </el-form-item>
    </div>
    <div v-if="model.provider === 'aliyun_qwen3'">
      <el-form-item label="API Key" prop="aliyun_qwen3.api_key">
        <el-input v-model="model.aliyun_qwen3.api_key" type="password" show-password placeholder="Can be empty, reads DASHSCOPE_API_KEY" />
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          Can be empty, defaults to DASHSCOPE_API_KEY
        </div>
      </el-form-item>
      <el-form-item label="WS URL" prop="aliyun_qwen3.ws_url">
        <el-input v-model="model.aliyun_qwen3.ws_url" placeholder="wss://dashscope.aliyuncs.com/api-ws/v1/realtime" />
      </el-form-item>
      <el-form-item label="Model" prop="aliyun_qwen3.model">
        <el-input v-model="model.aliyun_qwen3.model" placeholder="qwen3-asr-flash-realtime" />
      </el-form-item>
      <el-form-item label="Audio Format" prop="aliyun_qwen3.format">
        <el-select v-model="model.aliyun_qwen3.format" placeholder="Select format" style="width: 100%">
          <el-option label="pcm" value="pcm" />
          <el-option label="opus" value="opus" />
        </el-select>
      </el-form-item>
      <el-form-item label="Sample Rate" prop="aliyun_qwen3.sample_rate">
        <el-select v-model="model.aliyun_qwen3.sample_rate" placeholder="Select sample rate" style="width: 100%">
          <el-option label="8000" :value="8000" />
          <el-option label="16000" :value="16000" />
        </el-select>
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          Main program currently only supports 16000
        </div>
      </el-form-item>
      <el-form-item label="Language" prop="aliyun_qwen3.language">
        <el-input v-model="model.aliyun_qwen3.language" placeholder="zh" />
      </el-form-item>
      <el-form-item label="Auto End" prop="aliyun_qwen3.auto_end">
        <el-switch v-model="model.aliyun_qwen3.auto_end" />
        <div class="form-tip">
          <el-icon><InfoFilled /></el-icon>
          When enabled, uses server_vad; when disabled, uses Manual mode
        </div>
      </el-form-item>
      <el-form-item label="VAD Threshold" prop="aliyun_qwen3.vad_threshold" v-if="model.aliyun_qwen3?.auto_end">
        <el-input-number v-model="model.aliyun_qwen3.vad_threshold" :min="0" :max="1" :step="0.1" :precision="2" style="width: 100%" />
      </el-form-item>
      <el-form-item label="VAD Silence Time (ms)" prop="aliyun_qwen3.vad_silence_ms" v-if="model.aliyun_qwen3?.auto_end">
        <el-input-number v-model="model.aliyun_qwen3.vad_silence_ms" :min="0" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Timeout (seconds)" prop="aliyun_qwen3.timeout">
        <el-input-number v-model="model.aliyun_qwen3.timeout" :min="1" style="width: 100%" />
      </el-form-item>
    </div>
  </el-form>
</template>

<script setup>
import { ref } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'

const props = defineProps({
  model: { type: Object, required: true },
  rules: { type: Object, default: () => ({}) }
})

const formRef = ref()

function onProviderChange() {
  if (props.model.provider === 'funasr' && props.model.funasr && !props.model.funasr.mode) {
    props.model.funasr.mode = 'offline'
  }
}

function getJsonData() {
  const m = props.model
  if (m.provider === 'funasr') return JSON.stringify(m.funasr || {})
  if (m.provider === 'aliyun_funasr') return JSON.stringify(m.aliyun_funasr || {})
  if (m.provider === 'doubao') return JSON.stringify(m.doubao || {})
  if (m.provider === 'aliyun_qwen3') return JSON.stringify(m.aliyun_qwen3 || {})
  if (m.provider === 'xunfei') return JSON.stringify(m.xunfei || {})
  return '{}'
}

function validate(callback) {
  return formRef.value?.validate(callback)
}

function resetFields() {
  formRef.value?.resetFields()
}

defineExpose({ validate, getJsonData, resetFields })
</script>

<style scoped>
.form-tip {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 4px;
}
.form-tip .el-icon {
  font-size: 14px;
  color: #409eff;
}
</style>
