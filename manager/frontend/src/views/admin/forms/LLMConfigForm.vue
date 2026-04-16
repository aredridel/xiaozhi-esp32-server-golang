<template>
  <el-form ref="formRef" :model="model" :rules="rules" label-width="120px">
    <el-form-item label="Provider" prop="provider" required>
      <el-select v-model="model.provider" placeholder="Select provider" style="width: 100%" @change="onProviderChange">
        <el-option label="OpenAI" value="openai" />
        <el-option label="Ollama" value="ollama" />
        <el-option label="Azure OpenAI" value="azure" />
        <el-option label="Anthropic" value="anthropic" />
        <el-option label="Zhipu AI" value="zhipu" />
        <el-option label="Aliyun" value="aliyun" />
        <el-option label="Doubao" value="doubao" />
        <el-option label="SiliconFlow" value="siliconflow" />
        <el-option label="DeepSeek" value="deepseek" />
        <el-option label="Dify" value="dify" />
        <el-option label="Coze" value="coze" />
      </el-select>
    </el-form-item>
    <el-form-item label="Config Name" prop="name">
      <el-input v-model="model.name" placeholder="Enter configuration name" />
    </el-form-item>
    <el-form-item label="Config ID" prop="config_id">
      <el-input v-model="model.config_id" placeholder="Enter unique configuration ID" />
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama" :label="modelFieldLabel" prop="model_name" required>
      <el-select
        v-model="model.model_name"
        filterable
        allow-create
        default-first-option
        clearable
        :placeholder="modelPlaceholder"
        style="width: 100%"
      >
        <el-option
          v-for="option in modelOptions"
          :key="option.value"
          :label="option.label"
          :value="option.value"
        />
      </el-select>
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama && modelHint">
      <el-alert
        :title="modelHint"
        type="info"
        :closable="false"
        show-icon
      />
    </el-form-item>

    <el-form-item label="API Key" prop="api_key" :required="apiKeyRequired">
      <el-input v-model="model.api_key" type="password" placeholder="Enter API key" show-password />
    </el-form-item>

    <el-form-item v-if="showBaseURL" label="Base URL" prop="base_url" required>
      <el-input v-model="model.base_url" placeholder="Enter base URL" style="width: 100%" />
    </el-form-item>

    <el-form-item v-if="isCoze" label="Bot ID" prop="bot_id" required>
      <el-input v-model="model.bot_id" placeholder="Enter Coze Bot ID" />
    </el-form-item>

    <el-form-item v-if="isDify || isCoze" label="User Prefix" prop="user_prefix">
      <el-input v-model="model.user_prefix" placeholder="Optional, default xiaozhi" />
    </el-form-item>

    <el-form-item v-if="isCoze" label="Connector ID" prop="connector_id">
      <el-input v-model="model.connector_id" placeholder="Optional, default 1024" />
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama && requestConfig.allowMaxTokens" label="max_tokens" prop="max_tokens" required>
      <el-input-number v-model="model.max_tokens" :min="1" :max="100000" placeholder="max_tokens" style="width: 100%" />
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama && requestConfig.allowTemperature" label="Temperature" prop="temperature">
      <el-input-number v-model="model.temperature" :min="0" :max="requestConfig.temperatureMax" :step="0.1" placeholder="Temperature" style="width: 100%" />
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama && requestConfig.allowTopP" label="Top P" prop="top_p">
      <el-input-number v-model="model.top_p" :min="0" :max="1" :step="0.1" placeholder="Top P" style="width: 100%" />
    </el-form-item>

    <el-form-item v-if="isOpenAIOrOllama && requestCapabilityHint" label="Parameter Hint">
      <el-alert
        :title="requestCapabilityHint"
        type="info"
        :closable="false"
        show-icon
      />
    </el-form-item>

    <template v-if="thinkingConfig.visible">
      <el-form-item :label="thinkingConfig.label" prop="thinking_mode">
        <el-select v-model="model.thinking_mode" placeholder="Select deep thinking mode" style="width: 100%">
          <el-option
            v-for="option in thinkingConfig.options"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
      </el-form-item>

      <el-form-item v-if="thinkingConfig.showEffort" label="Thinking Effort" prop="thinking_effort">
        <el-select v-model="model.thinking_effort" placeholder="Select thinking effort" style="width: 100%">
          <el-option
            v-for="option in thinkingConfig.effortOptions"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
      </el-form-item>

      <el-form-item v-if="thinkingConfig.showBudget" label="Thinking Budget" prop="thinking_budget_tokens">
        <el-input-number
          v-model="model.thinking_budget_tokens"
          :min="thinkingConfig.budgetMin"
          :max="thinkingConfig.budgetMax"
          :step="thinkingConfig.budgetStep"
          placeholder="Budget tokens"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item v-if="thinkingConfig.showClearThinking" label="History Thinking Chain">
        <el-select v-model="model.thinking_clear_thinking" placeholder="Select history thinking chain handling" style="width: 100%">
          <el-option
            v-for="option in thinkingConfig.clearThinkingOptions"
            :key="String(option.value)"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
      </el-form-item>

      <el-form-item>
        <el-alert
          :title="thinkingConfig.hint"
          type="warning"
          :closable="false"
          show-icon
        />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import {
  getProviderFixedType,
  getProviderModelFieldLabel,
  getProviderModelHint,
  getProviderModelOptions,
  getProviderModelPlaceholder,
  getProviderQuickUrl,
  getProviderRequestConfig,
  getProviderThinkingConfig,
  isProviderBaseURLEditable,
  resolveLLMProvider
} from './llmCatalog'

const props = defineProps({
  model: { type: Object, required: true },
  rules: { type: Object, default: () => ({}) }
})

const formRef = ref()

const resolvedProvider = computed(() => resolveLLMProvider(props.model?.provider, props.model?.type))
const effectiveType = computed(() => getProviderFixedType(resolvedProvider.value))
const isOpenAIOrOllama = computed(() => effectiveType.value === 'openai' || effectiveType.value === 'ollama')
const isOllama = computed(() => effectiveType.value === 'ollama')
const isDify = computed(() => effectiveType.value === 'dify')
const isCoze = computed(() => effectiveType.value === 'coze')
const showBaseURL = computed(() => isProviderBaseURLEditable(resolvedProvider.value))
const apiKeyRequired = computed(() => !isOllama.value)

const defaultThinkingMode = 'default'

const provider = computed(() => resolvedProvider.value || '')
const modelFieldLabel = computed(() => getProviderModelFieldLabel(provider.value))
const modelPlaceholder = computed(() => getProviderModelPlaceholder(provider.value))
const modelOptions = computed(() => getProviderModelOptions(provider.value))
const thinkingDefinition = computed(() => getProviderThinkingConfig(provider.value, props.model?.model_name))
const modelHint = computed(() => {
  if (!thinkingDefinition.value?.visible && thinkingDefinition.value?.hint) {
    return thinkingDefinition.value.hint
  }
  return getProviderModelHint(provider.value)
})
const requestConfig = computed(() => getProviderRequestConfig(provider.value, props.model?.model_name))

const requestCapabilityHint = computed(() => {
  const blockedFields = []
  if (!requestConfig.value.allowMaxTokens) {
    blockedFields.push('max_tokens')
  }
  if (!requestConfig.value.allowTemperature) {
    blockedFields.push('temperature')
  }
  if (!requestConfig.value.allowTopP) {
    blockedFields.push('top_p')
  }
  if (!blockedFields.length) {
    return ''
  }
  return `Current model documentation recommends not setting ${blockedFields.join(', ')} separately; these fields will not be passed when saving.`
})

const thinkingConfig = computed(() => {
  const config = thinkingDefinition.value
  if (!config?.visible) {
    return {
      visible: false,
      label: 'Deep Thinking',
      options: [],
      showBudget: false,
      budgetMin: 1,
      budgetMax: 100000,
      budgetStep: 1,
      budgetRequired: false,
      showEffort: false,
      effortOptions: [],
      showClearThinking: false,
      clearThinkingOptions: [],
      hint: config?.hint || ''
    }
  }

  const showBudget = config.showBudgetFor.includes(props.model?.thinking_mode)
  const showEffort = config.showEffortFor.includes(props.model?.thinking_mode)
  const showClearThinking = config.showClearThinkingFor.includes(props.model?.thinking_mode)
  const budgetRequired = config.budgetRequiredFor.includes(props.model?.thinking_mode)

  return {
    visible: true,
    label: config.label,
    options: config.options,
    showBudget,
    budgetMin: config.budgetMin,
    budgetMax: config.budgetMax,
    budgetStep: config.budgetStep,
    budgetRequired,
    showEffort,
    effortOptions: config.effortOptions,
    showClearThinking,
    clearThinkingOptions: config.clearThinkingOptions,
    hint: config.hint || 'When stronger deep thinking is enabled, the model usually takes more time to reason, and both first packet and overall response will be noticeably slower.'
  }
})

function normalizeThinkingState(provider) {
  if (!props.model) {
    return
  }

  const config = getProviderThinkingConfig(provider, props.model?.model_name)
  if (!config?.visible) {
    props.model.thinking_mode = defaultThinkingMode
    props.model.thinking_budget_tokens = null
    props.model.thinking_clear_thinking = 'default'
    return
  }

  const options = config.options.map(option => option.value)
  if (!options.includes(props.model.thinking_mode)) {
    props.model.thinking_mode = defaultThinkingMode
  }

  if (props.model.thinking_budget_tokens !== null && props.model.thinking_budget_tokens !== undefined && props.model.thinking_budget_tokens !== '') {
    const budgetValue = Number(props.model.thinking_budget_tokens)
    if (Number.isNaN(budgetValue) || budgetValue < (config.budgetMin || 1)) {
      props.model.thinking_budget_tokens = null
    }
  }

  if (!props.model.thinking_effort) {
    props.model.thinking_effort = 'medium'
  }

  if (props.model.thinking_clear_thinking === undefined || props.model.thinking_clear_thinking === null || props.model.thinking_clear_thinking === '') {
    props.model.thinking_clear_thinking = 'default'
  }
}

function applyProviderDefaults(value, forceEditableURL = false, resetModel = false) {
  if (!value || !props.model) {
    return
  }

  props.model.type = getProviderFixedType(value)
  if (isProviderBaseURLEditable(value)) {
    const quickUrl = getProviderQuickUrl(value)
    if (forceEditableURL || !props.model.base_url) {
      props.model.base_url = quickUrl
    }
  } else {
    props.model.base_url = ''
  }

  if (resetModel) {
    props.model.model_name = ''
  }
  if (value === 'dify' && (!props.model.model_name || resetModel)) {
    props.model.model_name = 'dify'
  }
  if (value === 'coze') {
    if (!props.model.model_name || resetModel) {
      props.model.model_name = 'coze'
    }
    if (!props.model.connector_id) {
      props.model.connector_id = '1024'
    }
  }
}

watch(() => props.model?.provider, (value) => {
  if (!value || !props.model) {
    return
  }
  applyProviderDefaults(value, false)
  normalizeThinkingState(value)
}, { immediate: true })

watch(() => props.model?.model_name, () => {
  if (!provider.value) {
    return
  }
  normalizeThinkingState(provider.value)
}, { immediate: true })

function onProviderChange(value) {
  if (!value || !props.model) {
    return
  }
  applyProviderDefaults(value, true, true)
}

function getJsonData() {
  const m = props.model
  const providerName = resolveLLMProvider(m?.provider, m?.type)
  const providerType = getProviderFixedType(providerName)
  const thinking = buildThinkingPayload(m)
  if (providerType === 'dify') {
    const config = {
      api_key: m.api_key,
      user_prefix: m.user_prefix
    }
    if (m.base_url) config.base_url = m.base_url
    return JSON.stringify(config, null, 2)
  }
  if (providerType === 'coze') {
    const config = {
      api_key: m.api_key,
      bot_id: m.bot_id,
      user_prefix: m.user_prefix,
      connector_id: m.connector_id
    }
    if (m.base_url) config.base_url = m.base_url
    return JSON.stringify(config, null, 2)
  }

  const config = {
    model_name: m.model_name,
    api_key: m.api_key
  }
  if (isProviderBaseURLEditable(providerName) && m.base_url) config.base_url = m.base_url
  if (requestConfig.value.allowMaxTokens && m.max_tokens !== undefined && m.max_tokens !== null && m.max_tokens !== '') {
    config.max_tokens = m.max_tokens
  }
  if (requestConfig.value.allowTemperature && m.temperature !== undefined && m.temperature !== null) config.temperature = m.temperature
  if (requestConfig.value.allowTopP && m.top_p !== undefined && m.top_p !== null) config.top_p = m.top_p
  if (thinking) config.thinking = thinking
  return JSON.stringify(config, null, 2)
}

function buildThinkingPayload(model) {
  const providerName = resolveLLMProvider(model?.provider, model?.type)
  const config = getProviderThinkingConfig(providerName, model?.model_name)
  if (!config?.visible) {
    return undefined
  }

  const mode = model?.thinking_mode || defaultThinkingMode
  if (mode === defaultThinkingMode) {
    return undefined
  }

  const payload = { mode }
  if ((config.showBudgetFor || []).includes(mode) && model?.thinking_budget_tokens !== null && model?.thinking_budget_tokens !== undefined && model?.thinking_budget_tokens !== '') {
    payload.budget_tokens = Number(model.thinking_budget_tokens)
  }
  if ((config.showEffortFor || []).includes(mode) && model?.thinking_effort) {
    payload.effort = model.thinking_effort
  }
  if ((config.showClearThinkingFor || []).includes(mode) && typeof model?.thinking_clear_thinking === 'boolean') {
    payload.clear_thinking = model.thinking_clear_thinking
  }
  return payload
}

function validate(callback) {
  if (callback) {
    return formRef.value?.validate((valid) => {
      let finalValid = valid
      if (finalValid && isCoze.value && !props.model?.bot_id) {
        finalValid = false
      }
      if (finalValid && thinkingConfig.value.showBudget && thinkingConfig.value.budgetRequired && (props.model?.thinking_budget_tokens === null || props.model?.thinking_budget_tokens === undefined || props.model?.thinking_budget_tokens === '')) {
        finalValid = false
      }
      if (finalValid && thinkingConfig.value.showBudget && props.model?.thinking_budget_tokens !== null && props.model?.thinking_budget_tokens !== undefined && props.model?.thinking_budget_tokens !== '' && Number(props.model?.thinking_budget_tokens) < thinkingConfig.value.budgetMin) {
        finalValid = false
      }
      callback(finalValid)
      return finalValid
    })
  }

  return formRef.value?.validate().then(() => {
    if (isCoze.value && !props.model?.bot_id) {
      return Promise.reject(new Error('Please enter Coze Bot ID'))
    }
    if (thinkingConfig.value.showBudget && thinkingConfig.value.budgetRequired && (props.model?.thinking_budget_tokens === null || props.model?.thinking_budget_tokens === undefined || props.model?.thinking_budget_tokens === '')) {
      return Promise.reject(new Error('Please fill in thinking budget'))
    }
    if (thinkingConfig.value.showBudget && props.model?.thinking_budget_tokens !== null && props.model?.thinking_budget_tokens !== undefined && props.model?.thinking_budget_tokens !== '' && Number(props.model?.thinking_budget_tokens) < thinkingConfig.value.budgetMin) {
      return Promise.reject(new Error(`Thinking budget cannot be less than ${thinkingConfig.value.budgetMin}`))
    }
    return true
  })
}

function resetFields() {
  formRef.value?.resetFields()
}

defineExpose({ validate, getJsonData, resetFields })
</script>
