import api from './api'

/** Parse API item into unified result (including first_packet_ms) */
function normItem(item) {
  if (!item || typeof item !== 'object') return { ok: false, message: '', first_packet_ms: undefined, reasoning_content_returned: false }
  const ms = item.first_packet_ms
  return {
    ok: !!item.ok,
    message: item.message || '',
    first_packet_ms: typeof ms === 'number' ? ms : (ms != null ? Number(ms) : undefined),
    reasoning_content_returned: !!item.reasoning_content_returned
  }
}

/**
 * Test a single or single-type configuration
 * @param {string} type - Type: ota | vad | asr | llm | tts
 * @param {string} [configId] - Optional, specify config_id to test only that entry
 * @returns {Promise<{ ok: boolean, message: string, first_packet_ms?: number }>} Returns result directly for single entry; returns first or summary for multiple
 */
export async function testSingleConfig(type, configId) {
  const body = {
    types: [type],
    config_ids: configId ? { [type]: [configId] } : {}
  }
  const res = await api.post('/admin/configs/test', body, { timeout: 30000 })
  const data = res.data?.data ?? res.data
  const typeResult = data?.[type]
  if (!typeResult || typeof typeResult !== 'object') {
    return { ok: false, message: 'No test results returned' }
  }
  const entries = Object.entries(typeResult).filter(([k]) => !k.startsWith('_'))
  if (configId && typeResult[configId]) {
    return normItem(typeResult[configId])
  }
  if (entries.length === 0) {
    const err = typeResult._error || typeResult._no_client || typeResult._none
    const msg = err && typeof err === 'object' ? (err.message || '').trim() : ''
    const fallback = typeResult._none ? 'Not configured or not enabled' : 'No test results'
    return { ok: false, message: msg || fallback }
  }
  return normItem(entries[0][1])
}

/**
 * Test all configurations of a type, return results by config_id (for "Test All" and display per row)
 * @param {string} type - Type: vad | asr | llm | tts
 * @returns {Promise<Record<string, { ok: boolean, message: string, first_packet_ms?: number }>>} config_id -> { ok, message, first_packet_ms? }
 */
export async function testAllConfigs(type) {
  const body = { types: [type] }
  const res = await api.post('/admin/configs/test', body, { timeout: 60000 })
  const data = res.data?.data ?? res.data
  const typeResult = data?.[type]
  const out = {}
  if (!typeResult || typeof typeResult !== 'object') {
    return out
  }
  const err = typeResult._error || typeResult._no_client || typeResult._none
  const errMsg = err && typeof err === 'object' ? (err.message || '').trim() : 'No test results returned'
  for (const [k, v] of Object.entries(typeResult)) {
    if (k.startsWith('_')) continue
    out[k] = normItem(v)
  }
  if (Object.keys(out).length === 0 && errMsg) {
    out._global = { ok: false, message: errMsg }
  }
  return out
}

/**
 * Convert getJsonData() return value into a mergeable object (form returns JSON string)
 * @param {string|object} jsonData - getJsonData() return value
 * @returns {object}
 */
export function parseJsonData(jsonData) {
  if (jsonData == null) return {}
  if (typeof jsonData === 'object') return jsonData
  if (typeof jsonData !== 'string') return {}
  try {
    return JSON.parse(jsonData) || {}
  } catch {
    return {}
  }
}

/**
 * Test with custom data (unsaved draft / current step of wizard)
 * @param {string} type - Type: ota | vad | asr | llm | tts
 * @param {Record<string, object>} typeData - config_id -> config object under this type, consistent with API data[type]
 * @returns {Promise<{ ok: boolean, message: string, first_packet_ms?: number }>} Single result (single entry only)
 */
export async function testWithData(type, typeData) {
  const body = { types: [type], data: { [type]: typeData } }
  const res = await api.post('/admin/configs/test', body, { timeout: 30000 })
  const data = res.data?.data ?? res.data
  const typeResult = data?.[type]
  if (!typeResult || typeof typeResult !== 'object') {
    return { ok: false, message: 'No test results returned' }
  }
  const err = typeResult._error || typeResult._no_client
  if (err && typeof err === 'object' && err.message) {
    return { ok: false, message: err.message }
  }
  const entries = Object.entries(typeResult).filter(([k]) => !k.startsWith('_'))
  if (entries.length === 0) {
    return { ok: false, message: typeResult._none?.message || 'No test results' }
  }
  return normItem(entries[0][1])
}
