export const TTS_PROVIDERS_WITH_VOICE_CLONE = ['doubao_ws', 'minimax', 'cosyvoice', 'aliyun_qwen', 'indextts_vllm']

const voiceCloneProviderSet = new Set(TTS_PROVIDERS_WITH_VOICE_CLONE)

export const TTS_PROVIDER_OPTIONS = [
  { label: 'Doubao WebSocket', value: 'doubao_ws' },
  { label: 'Edge TTS', value: 'edge' },
  { label: 'Edge Offline', value: 'edge_offline' },
  { label: 'CosyVoice', value: 'cosyvoice' },
  { label: 'Xunfei', value: 'xunfei' },
  { label: 'Xunfei Super TTS', value: 'xunfei_super_tts' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'Qwen', value: 'aliyun_qwen' },
  { label: 'Zhipu', value: 'zhipu' },
  { label: 'Minimax', value: 'minimax' },
  { label: 'IndexTTS(vLLM)', value: 'indextts_vllm' }
].map((item) => ({
  ...item,
  supports_voice_clone: voiceCloneProviderSet.has(item.value)
}))

export const TTS_PROVIDERS_WITH_VOICES = ['minimax', 'edge', 'doubao', 'doubao_ws', 'zhipu', 'openai', 'indextts_vllm', 'xunfei_super_tts']
