<template>
  <el-form ref="formRef" :model="model" :rules="rules" label-width="120px">
    <el-form-item label="Provider" prop="provider">
      <el-select v-model="model.provider" placeholder="Select provider" style="width: 100%">
        <el-option label="TEN VAD" value="ten_vad" />
      </el-select>
    </el-form-item>
    <el-form-item label="Config Name" prop="name">
      <el-input v-model="model.name" placeholder="Enter configuration name" />
    </el-form-item>
    <el-form-item label="Config ID" prop="config_id">
      <el-input v-model="model.config_id" placeholder="Enter unique configuration ID" />
    </el-form-item>
    <template v-if="model.provider === 'webrtc_vad'">
      <el-divider content-position="left">WebRTC VAD Configuration</el-divider>
      <el-form-item label="Min Pool Size" prop="webrtc_vad.pool_min_size">
        <el-input-number v-model="model.webrtc_vad.pool_min_size" :min="1" :max="1000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Max Pool Size" prop="webrtc_vad.pool_max_size">
        <el-input-number v-model="model.webrtc_vad.pool_max_size" :min="1" :max="10000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Max Idle Connections" prop="webrtc_vad.pool_max_idle">
        <el-input-number v-model="model.webrtc_vad.pool_max_idle" :min="1" :max="1000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="VAD Sample Rate" prop="webrtc_vad.vad_sample_rate">
        <el-select v-model="model.webrtc_vad.vad_sample_rate" style="width: 100%">
          <el-option label="8000 Hz" :value="8000" />
          <el-option label="16000 Hz" :value="16000" />
          <el-option label="32000 Hz" :value="32000" />
          <el-option label="48000 Hz" :value="48000" />
        </el-select>
      </el-form-item>
      <el-form-item label="VAD Mode" prop="webrtc_vad.vad_mode">
        <el-select v-model="model.webrtc_vad.vad_mode" style="width: 100%">
          <el-option label="Mode 0 (Quality Priority)" :value="0" />
          <el-option label="Mode 1 (Low Latency)" :value="1" />
          <el-option label="Mode 2 (Balanced)" :value="2" />
          <el-option label="Mode 3 (High Precision)" :value="3" />
        </el-select>
      </el-form-item>
    </template>
    <template v-if="model.provider === 'silero_vad'">
      <el-divider content-position="left">Silero VAD Configuration</el-divider>
      <el-form-item label="Model Path" prop="silero_vad.model_path">
        <el-input v-model="model.silero_vad.model_path" placeholder="Enter model file path" />
      </el-form-item>
      <el-form-item label="Threshold" prop="silero_vad.threshold">
        <el-input-number v-model="model.silero_vad.threshold" :min="0" :max="1" :step="0.1" :precision="2" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Min Silence Duration (ms)" prop="silero_vad.min_silence_duration_ms">
        <el-input-number v-model="model.silero_vad.min_silence_duration_ms" :min="10" :max="5000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Sample Rate" prop="silero_vad.sample_rate">
        <el-select v-model="model.silero_vad.sample_rate" style="width: 100%">
          <el-option label="8000 Hz" :value="8000" />
          <el-option label="16000 Hz" :value="16000" />
          <el-option label="32000 Hz" :value="32000" />
          <el-option label="48000 Hz" :value="48000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Channels" prop="silero_vad.channels">
        <el-select v-model="model.silero_vad.channels" style="width: 100%">
          <el-option label="Mono" :value="1" />
          <el-option label="Stereo" :value="2" />
        </el-select>
      </el-form-item>
      <el-form-item label="Pool Size" prop="silero_vad.pool_size">
        <el-input-number v-model="model.silero_vad.pool_size" :min="1" :max="100" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Acquire Timeout (ms)" prop="silero_vad.acquire_timeout_ms">
        <el-input-number v-model="model.silero_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
      </el-form-item>
    </template>
    <template v-if="model.provider === 'ten_vad'">
      <el-divider content-position="left">TEN VAD Configuration</el-divider>
      <el-form-item label="Hop Size" prop="ten_vad.hop_size">
        <el-input-number v-model="model.ten_vad.hop_size" :min="128" :max="1024" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px;">Default: 320</div>
      </el-form-item>
      <el-form-item label="VAD Threshold" prop="ten_vad.threshold">
        <el-input-number v-model="model.ten_vad.threshold" :min="0" :max="1" :step="0.1" :precision="2" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px;">Recommended: 0.3</div>
      </el-form-item>
      <el-form-item label="Pool Size" prop="ten_vad.pool_size">
        <el-input-number v-model="model.ten_vad.pool_size" :min="1" :max="100" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px;">Recommended: 10</div>
      </el-form-item>
      <el-form-item label="Acquire Timeout (ms)" prop="ten_vad.acquire_timeout_ms">
        <el-input-number v-model="model.ten_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px;">Recommended: 3000</div>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  model: { type: Object, required: true },
  rules: { type: Object, default: () => ({}) }
})

const formRef = ref()

function getJsonData() {
  const m = props.model
  if (m.provider === 'webrtc_vad') return JSON.stringify(m.webrtc_vad || {})
  if (m.provider === 'silero_vad') return JSON.stringify(m.silero_vad || {})
  if (m.provider === 'ten_vad') return JSON.stringify(m.ten_vad || {})
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
