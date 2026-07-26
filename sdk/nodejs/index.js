// QaSDK — Node.js SDK 主入口
// 上报地址约定：POST {baseUrl}/api/qa/report
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：report(name, result, message, tags)
//   - 自由日志：log(message, tags)
//   - API 拦截事件：logRequest / logResponse / logError
//       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
//   - 任意原始上报：sendRaw(payload)（可直接转发 FileApiLogger 的 JSONL 行）
// 上报前会对 request / response / headers 中的敏感字段自动脱敏。
const cfg = require('./config')

let initialized = false

// 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
const SENSITIVE_KEYS = new Set([
  'credential', 'authtoken', 'token', 'password', 'secret', 'apikey', 'key', 'authorization',
])

function init(baseUrl, token, enabled) {
  if (baseUrl !== undefined) cfg.baseUrl = baseUrl
  if (token !== undefined) cfg.token = token
  if (enabled !== undefined) cfg.enabled = enabled
  initialized = true
}

// 递归脱敏：把对象中敏感字段的值替换为 ***
function redact(value) {
  if (Array.isArray(value)) return value.map(redact)
  if (value && typeof value === 'object') {
    const out = {}
    for (const k of Object.keys(value)) {
      if (SENSITIVE_KEYS.has(k.toLowerCase())) out[k] = '***'
      else out[k] = redact(value[k])
    }
    return out
  }
  return value
}

function buildPayload(name, result, message, tags, event) {
  return {
    event: event || 'case_result',
    name: name || '',
    result: result || 'passed',
    message: message || '',
    tags: tags || {},
    timestamp: Date.now(),
  }
}

async function send(payload) {
  if (!cfg.enabled) return false
  if (!initialized) init()
  const url = cfg.baseUrl.replace(/\/$/, '') + '/api/qa/report'
  const headers = { 'Content-Type': 'application/json' }
  if (cfg.token) headers['Authorization'] = 'Bearer ' + cfg.token
  // 脱敏：request / response / headers 中的敏感字段
  const safe = { ...payload }
  if (safe.headers) safe.headers = redact(safe.headers)
  if (safe.request !== undefined) safe.request = redact(safe.request)
  if (safe.response !== undefined) safe.response = redact(safe.response)
  try {
    const res = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(safe),
    })
    return res.ok
  } catch (e) {
    // 上报失败不应阻断主流程
    console.error('[QaSDK] 上报失败:', e.message)
    return false
  }
}

// 上报一条用例结果
function report(name, result, message, tags) {
  return send(buildPayload(name, result, message, tags, 'case_result'))
}

// 上报一条日志
function log(message, tags) {
  return send({ event: 'log', name: 'log', result: 'info', message: message || '', tags: tags || {}, timestamp: Date.now() })
}

// API 拦截事件：请求（对应 FileApiLogger REQUEST）
function logRequest(method, headers, request, opts = {}) {
  return send({
    event: 'request',
    type: 'REQUEST',
    name: method || '',
    method: method || '',
    headers: headers || {},
    request: request, // 对象或直接 JSON 字符串
    seq: opts.seq || 0,
    elapsed_ms: opts.elapsedMs || 0,
    tags: opts.tags || {},
    message: opts.message || '',
    timestamp: Date.now(),
  })
}

// API 拦截事件：响应（对应 FileApiLogger RESPONSE）
function logResponse(method, headers, response, elapsedMs = 0, opts = {}) {
  return send({
    event: 'response',
    type: 'RESPONSE',
    name: method || '',
    method: method || '',
    headers: headers || {},
    response: response,
    elapsed_ms: elapsedMs,
    seq: opts.seq || 0,
    tags: opts.tags || {},
    message: opts.message || '',
    timestamp: Date.now(),
  })
}

// API 拦截事件：错误（对应 FileApiLogger ERROR）
function logError(method, error, elapsedMs = 0, opts = {}) {
  return send({
    event: 'error',
    type: 'ERROR',
    name: method || '',
    method: method || '',
    headers: opts.headers || {},
    error: error || '',
    elapsed_ms: elapsedMs,
    seq: opts.seq || 0,
    tags: opts.tags || {},
    message: error || '',
    timestamp: Date.now(),
  })
}

// 任意原始上报（可直接转发 FileApiLogger 的 JSONL 行，字段会被服务端归一）
function sendRaw(payload) {
  return send(payload)
}

module.exports = { init, report, log, logRequest, logResponse, logError, sendRaw }
