const defaultOption = { label: 'Default', value: 'default' }
const enableOption = { label: 'Enabled', value: 'enabled' }
const disableOption = { label: 'Disabled', value: 'disabled' }
const clearHistoryOptions = [
  { label: 'Default', value: 'default' },
  { label: 'Clear', value: true },
  { label: 'Keep', value: false }
]

function withDefault(options) {
  return [defaultOption, ...options]
}

function createModel(value, thinking, extra = {}) {
  return {
    value,
    label: value,
    thinking,
    ...extra
  }
}

const openAIReasoningStandard = withDefault([
  { label: 'Minimal', value: 'minimal' },
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' }
])

const openAIReasoningCodex = withDefault([
  { label: 'Disabled', value: 'none' },
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' }
])

const openAIReasoningCodexMax = withDefault([
  { label: 'Disabled', value: 'none' },
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' },
  { label: 'Very High', value: 'xhigh' }
])

const openAIReasoningLegacy = withDefault([
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' }
])

const openAIReasoningHighOnly = withDefault([
  { label: 'High', value: 'high' }
])

const booleanThinkingOptions = withDefault([
  enableOption,
  disableOption
])

const doubaoReasoningOptions = withDefault([
  { label: 'Disabled', value: 'minimal' },
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' }
])

const anthropicAdaptiveOptions = [
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' },
  { label: 'Very High', value: 'max' }
]

const openAIReasoningLatest = withDefault([
  { label: 'Disabled', value: 'none' },
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' },
  { label: 'Very High', value: 'xhigh' }
])

const openAIReasoningLatestPro = withDefault([
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' },
  { label: 'Very High', value: 'xhigh' }
])

const openAIReasoningRequest = {
  allowMaxTokens: false,
  allowTemperature: false,
  allowTopP: false
}

const anthropicManualThinking = {
  label: 'Deep Thinking',
  options: withDefault([{ label: 'Manual Thinking', value: 'enabled' }]),
  showBudgetFor: ['enabled'],
  budgetMin: 1024,
  budgetRequiredFor: ['enabled']
}

const anthropicAdaptiveThinking = {
  label: 'Deep Thinking',
  options: withDefault([
    { label: 'Manual Thinking', value: 'enabled' },
    { label: 'Adaptive Thinking', value: 'adaptive' }
  ]),
  showBudgetFor: ['enabled'],
  budgetMin: 1024,
  budgetRequiredFor: ['enabled'],
  showEffortFor: ['adaptive'],
  effortOptions: anthropicAdaptiveOptions
}

const zhipuThinkingConfig = {
  label: 'Deep Thinking',
  options: booleanThinkingOptions,
  showClearThinkingFor: ['enabled'],
  clearThinkingOptions: clearHistoryOptions
}

const aliyunThinkingConfig = {
  label: 'Deep Thinking',
  options: booleanThinkingOptions,
  showBudgetFor: ['enabled'],
  budgetMin: 1,
  budgetStep: 256
}

const siliconflowThinkingConfig = {
  label: 'Deep Thinking',
  options: booleanThinkingOptions,
  showBudgetFor: ['enabled'],
  budgetMin: 128,
  budgetMax: 32768,
  budgetStep: 128
}

const providerTypeMap = {
  openai: 'openai',
  ollama: 'ollama',
  azure: 'openai',
  anthropic: 'openai',
  zhipu: 'openai',
  aliyun: 'openai',
  doubao: 'openai',
  siliconflow: 'openai',
  deepseek: 'openai',
  dify: 'dify',
  coze: 'coze'
}

const editableBaseURLProviders = new Set(['openai', 'ollama', 'azure', 'dify', 'coze'])

const catalog = {
  openai: {
    quickUrl: 'https://api.openai.com/v1',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'By default, official stable aliases are preferred; for locked behavior, manually enter exact snapshot model ID.'
    models: [
      createModel('gpt-5.4', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.4-pro', { label: 'Reasoning Effort', options: openAIReasoningLatestPro }, { request: openAIReasoningRequest }),
      createModel('gpt-5.4-mini', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.4-nano', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2-pro', { label: 'Reasoning Effort', options: openAIReasoningLatestPro }, { request: openAIReasoningRequest }),
      createModel('gpt-5-chat-latest', false, { hint: 'ChatGPT-specific alias, suitable for compatibility with legacy workflows; new integrations should prefer mainline GPT-5.* models.' }),
      createModel('gpt-5-pro', { label: 'Reasoning Effort', options: openAIReasoningHighOnly }, { request: openAIReasoningRequest }),
      createModel('gpt-5', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5-mini', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5-nano', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5.3-codex', { label: 'Reasoning Effort', options: openAIReasoningCodexMax }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2-codex', { label: 'Reasoning Effort', options: openAIReasoningCodexMax }, { request: openAIReasoningRequest }),
      createModel('gpt-5-codex', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('gpt-5.1', { label: 'Reasoning Effort', options: openAIReasoningCodex }, { request: openAIReasoningRequest }),
      createModel('gpt-5.1-codex', { label: 'Reasoning Effort', options: openAIReasoningCodex }, { request: openAIReasoningRequest }),
      createModel('gpt-5.1-codex-mini', { label: 'Reasoning Effort', options: openAIReasoningCodex }, { request: openAIReasoningRequest }),
      createModel('gpt-5.1-codex-max', { label: 'Reasoning Effort', options: openAIReasoningCodexMax }, { request: openAIReasoningRequest }),
      createModel('o3', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o4-mini', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o3-mini', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o1', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest })
    ],
    fallbackThinking: {
      label: 'Reasoning Effort',
      options: openAIReasoningCodex,
      hint: 'Custom model not found in documentation list, falling back to generic reasoning_effort configuration; effectiveness depends on actual model.'
    }
  },
  ollama: {
    quickUrl: 'http://127.0.0.1:11434/v1',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'Ollama uses local or private model services; both model list and address can be customized.'
    models: [],
    fallbackThinking: null
  },
  azure: {
    quickUrl: 'https://your-resource-name.openai.azure.com/openai/v1/',
    modelPlaceholder: 'Select official model name or enter custom deployment name',
    modelHint: 'For Azure, enter the deployment name here; list names are mainly for reference to underlying model capabilities.'
    models: [
      createModel('gpt-5.4', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.4-pro', { label: 'Reasoning Effort', options: openAIReasoningLatestPro }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2', { label: 'Reasoning Effort', options: openAIReasoningLatest }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2-chat', false, { hint: 'Chat models in Azure documentation are usually accessed via deployment name; availability depends on region and quota.' }),
      createModel('gpt-5.3-codex', { label: 'Reasoning Effort', options: openAIReasoningCodexMax }, { request: openAIReasoningRequest }),
      createModel('gpt-5.2-codex', { label: 'Reasoning Effort', options: openAIReasoningCodexMax }, { request: openAIReasoningRequest }),
      createModel('gpt-5-mini', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5-nano', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5-chat', { label: 'Reasoning Effort', options: openAIReasoningStandard }, { request: openAIReasoningRequest }),
      createModel('gpt-5-pro', { label: 'Reasoning Effort', options: openAIReasoningHighOnly }, { request: openAIReasoningRequest }),
      createModel('o4-mini', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o3', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o3-mini', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest }),
      createModel('o1', { label: 'Reasoning Effort', options: openAIReasoningLegacy }, { request: openAIReasoningRequest })
    ],
    fallbackThinking: {
      label: 'Reasoning Effort',
      options: openAIReasoningCodex,
      hint: 'When Azure custom deployment does not match documented models, falls back to generic reasoning_effort configuration; actual compatibility depends on deployed model.'
    }
  },
  anthropic: {
    quickUrl: 'https://api.anthropic.com/v1/',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'By default, official stable aliases are preferred; for fixed versions or regression testing, enter exact model ID with date suffix.'
    models: [
      createModel('claude-opus-4-6', anthropicAdaptiveThinking),
      createModel('claude-sonnet-4-6', anthropicAdaptiveThinking),
      createModel('claude-haiku-4-5', anthropicManualThinking),
      createModel('claude-3-7-sonnet', anthropicManualThinking),
      createModel('claude-sonnet-4', anthropicManualThinking),
      createModel('claude-opus-4', anthropicManualThinking),
      createModel('claude-opus-4-1', anthropicManualThinking)
    ],
    fallbackThinking: {
      ...anthropicAdaptiveThinking,
      hint: 'Custom model not found in documentation list. If using manual thinking, budget_tokens must be explicitly filled; Adaptive should only be used on models confirmed supported by documentation.'
    }
  },
  zhipu: {
    quickUrl: 'https://open.bigmodel.cn/api/paas/v4',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'Zhipu documentation supports controlling thinking mode via thinking.type and clear_thinking.'
    models: [
      createModel('glm-5', zhipuThinkingConfig),
      createModel('glm-4.7', zhipuThinkingConfig),
      createModel('glm-4.7-flashx', zhipuThinkingConfig),
      createModel('glm-4.7-flash', zhipuThinkingConfig),
      createModel('glm-4.6', zhipuThinkingConfig),
      createModel('glm-4.6v', zhipuThinkingConfig),
      createModel('glm-4.5', zhipuThinkingConfig),
      createModel('glm-4.5-air', zhipuThinkingConfig),
      createModel('glm-4.5-airx', zhipuThinkingConfig),
      createModel('glm-4.5v', zhipuThinkingConfig)
    ],
    fallbackThinking: {
      ...zhipuThinkingConfig,
      hint: 'Custom model not found in documentation list, falling back to generic thinking.type / clear_thinking configuration.'
    }
  },
  aliyun: {
    quickUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'By default, official stable aliases are preferred; to lock specific versions, manually enter model ID with date or minor version suffix.'
    models: [
      createModel('qwen-plus-latest', aliyunThinkingConfig),
      createModel('qwen-turbo-latest', aliyunThinkingConfig),
      createModel('qwen3-max', aliyunThinkingConfig),
      createModel('qwen3-235b-a22b', aliyunThinkingConfig),
      createModel('qwen3-30b-a3b', aliyunThinkingConfig),
      createModel('qwen3-next-80b-a3b-thinking', aliyunThinkingConfig),
      createModel('glm-4.7', aliyunThinkingConfig),
      createModel('glm-4.6', aliyunThinkingConfig),
      createModel('glm-4.5', aliyunThinkingConfig),
      createModel('glm-4.5-air', aliyunThinkingConfig),
      createModel('kimi-k2-thinking', aliyunThinkingConfig),
      createModel('qwen3-235b-a22b-thinking-2507', aliyunThinkingConfig, { label: 'qwen3-235b-a22b-thinking-2507 (Versioned)' }),
      createModel('qwen3-30b-a3b-thinking-2507', aliyunThinkingConfig, { label: 'qwen3-30b-a3b-thinking-2507 (Versioned)' }),
      createModel('kimi/kimi-k2.5', aliyunThinkingConfig, { label: 'kimi/kimi-k2.5 (Versioned)' })
    ],
    fallbackThinking: {
      ...aliyunThinkingConfig,
      hint: 'Custom model not found in documentation list. If model supports thinking_budget, fill according to documentation; field will not be sent when left empty.'
    }
  },
  doubao: {
    quickUrl: 'https://ark.cn-beijing.volces.com/api/v3',
    modelPlaceholder: 'Select or enter model ID (usually with version suffix)',
    modelHint: 'For Doubao, prefer using official real Model ID. Currently no stable alias confirmed for universal substitution; recommended to use Model ID from console or model list.'
    models: [
      createModel('doubao-seed-2-0-pro-260215', { label: 'Reasoning Effort', options: doubaoReasoningOptions }, { label: 'Doubao Seed 2.0 Pro (doubao-seed-2-0-pro-260215)' }),
      createModel('doubao-seed-2-0-lite-260215', { label: 'Reasoning Effort', options: doubaoReasoningOptions }, { label: 'Doubao Seed 2.0 Lite (doubao-seed-2-0-lite-260215)' }),
      createModel('doubao-seed-2-0-mini-260215', { label: 'Reasoning Effort', options: doubaoReasoningOptions }, { label: 'Doubao Seed 2.0 Mini (doubao-seed-2-0-mini-260215)' }),
      createModel('doubao-seed-1-6-251015', { label: 'Reasoning Effort', options: doubaoReasoningOptions }, { label: 'Doubao Seed 1.6 (doubao-seed-1-6-251015)' })
    ],
    fallbackThinking: {
      label: 'Reasoning Effort',
      options: doubaoReasoningOptions,
      hint: 'Custom model not found in documentation list, falling back to generic reasoning_effort configuration; effectiveness depends on actual model.'
    }
  },
  siliconflow: {
    quickUrl: 'https://api.siliconflow.cn/v1',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'SiliconFlow documentation directly lists enable_thinking supported models; budget configuration is only shown for models listed in documentation.'
    models: [
      createModel('Pro/zai-org/GLM-5', siliconflowThinkingConfig),
      createModel('Pro/zai-org/GLM-4.7', siliconflowThinkingConfig),
      createModel('deepseek-ai/DeepSeek-V3.2', siliconflowThinkingConfig),
      createModel('Pro/deepseek-ai/DeepSeek-V3.2', siliconflowThinkingConfig),
      createModel('zai-org/GLM-4.6', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-8B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-14B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-32B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-30B-A3B', siliconflowThinkingConfig),
      createModel('tencent/Hunyuan-A13B-Instruct', siliconflowThinkingConfig),
      createModel('zai-org/GLM-4.5V', siliconflowThinkingConfig),
      createModel('deepseek-ai/DeepSeek-V3.1-Terminus', siliconflowThinkingConfig),
      createModel('Pro/deepseek-ai/DeepSeek-V3.1-Terminus', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-397B-A17B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-122B-A10B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-35B-A3B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-27B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-9B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-4B', siliconflowThinkingConfig)
    ],
    fallbackThinking: {
      ...siliconflowThinkingConfig,
      hint: 'Custom model not found in documentation list. If model supports enable_thinking / thinking_budget, fill according to documentation; thinking_budget will not be sent when left empty.'
    }
  },
  deepseek: {
    quickUrl: 'https://api.deepseek.com/v1',
    modelPlaceholder: 'Select or enter model name',
    modelHint: 'Official DeepSeek switches thinking mode by selecting different models: deepseek-chat is non-thinking, deepseek-reasoner is thinking.'
    models: [
      createModel('deepseek-chat', false, {
        hint: 'deepseek-chat is a non-thinking model, no additional thinking parameters needed.'
      }),
      createModel('deepseek-reasoner', false, {
        hint: 'deepseek-reasoner has built-in thinking mode, no additional thinking parameters needed.'
      })
    ],
    fallbackThinking: {
      label: 'Deep Thinking',
      options: booleanThinkingOptions,
      hint: 'Official DeepSeek recommends switching thinking mode via model name. If custom proxy additionally supports thinking.type, compatibility toggle can be enabled here.'
    }
  }
}

function cloneOptions(options = []) {
  return options.map(option => ({ ...option }))
}

function normalizeModelName(modelName) {
  return String(modelName || '').trim().toLowerCase()
}

export function resolveLLMProvider(provider, type) {
  const normalizedProvider = String(provider || '').trim().toLowerCase()
  const normalizedType = String(type || '').trim().toLowerCase()

  if (normalizedProvider === 'openai' && ['ollama', 'dify', 'coze'].includes(normalizedType)) {
    return normalizedType
  }
  if (normalizedProvider) {
    return normalizedProvider
  }
  if (['ollama', 'dify', 'coze'].includes(normalizedType)) {
    return normalizedType
  }
  return 'openai'
}

export function getProviderFixedType(provider) {
  return providerTypeMap[provider] || 'openai'
}

export function isProviderBaseURLEditable(provider) {
  return editableBaseURLProviders.has(provider)
}

export function getProviderQuickUrl(provider) {
  return catalog[provider]?.quickUrl || ''
}

export function getProviderModelOptions(provider) {
  return (catalog[provider]?.models || []).map(model => ({
    label: model.label,
    value: model.value
  }))
}

export function getProviderModelHint(provider) {
  return catalog[provider]?.modelHint || ''
}

export function getProviderModelFieldLabel(provider) {
  if (provider === 'azure') {
    return 'Deployment Name'
  }
  if (provider === 'doubao') {
    return 'Model ID'
  }
  return 'Model Name'
}

export function getProviderModelPlaceholder(provider) {
  return catalog[provider]?.modelPlaceholder || 'Select or enter model name'
}

export function resolveProviderModel(provider, modelName) {
  const normalized = normalizeModelName(modelName)
  if (!normalized) {
    return null
  }

  const models = catalog[provider]?.models || []
  return models.find(model => normalizeModelName(model.value) === normalized) || null
}

export function getProviderRequestConfig(provider, modelName) {
  const model = resolveProviderModel(provider, modelName)
  return {
    allowMaxTokens: true,
    allowTemperature: true,
    allowTopP: true,
    temperatureMax: 2,
    ...(model?.request || {})
  }
}

export function getProviderThinkingConfig(provider, modelName) {
  const model = resolveProviderModel(provider, modelName)
  if (model?.thinking === false) {
    return {
      visible: false,
      hint: model.hint || ''
    }
  }

  const source = model?.thinking || catalog[provider]?.fallbackThinking
  if (!source) {
    return {
      visible: false,
      hint: model?.hint || ''
    }
  }

  return {
    visible: true,
    label: source.label || 'Deep Thinking',
    options: cloneOptions(source.options),
    showBudgetFor: [...(source.showBudgetFor || [])],
    budgetMin: source.budgetMin || 1,
    budgetMax: source.budgetMax || 100000,
    budgetStep: source.budgetStep || 1,
    budgetRequiredFor: [...(source.budgetRequiredFor || [])],
    showEffortFor: [...(source.showEffortFor || [])],
    effortOptions: cloneOptions(source.effortOptions || []),
    showClearThinkingFor: [...(source.showClearThinkingFor || [])],
    clearThinkingOptions: cloneOptions(source.clearThinkingOptions || clearHistoryOptions),
    hint: model?.hint || source.hint || ''
  }
}
