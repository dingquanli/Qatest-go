// QaSDK — Cocos Creator (TypeScript) SDK 主入口
// 上报地址约定：POST {baseUrl}/api/qa/report
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：report(name, result, message, tags)
//   - 自由日志：log(message, tags)
//   - API 拦截事件：logRequest / logResponse / logError
//       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
//   - 任意原始上报：sendRaw(payload)（可直接转发 FileApiLogger 的 JSONL 行）
// 上报前会对 request / response / headers 中的敏感字段自动脱敏。
import { QaConfig } from './QaConfig'

let initialized = false

// 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
const SENSITIVE_KEYS = new Set([
  'credential', 'authtoken', 'token', 'password', 'secret', 'apikey', 'key', 'authorization',
])

export function init(baseUrl?: string, token?: string, enabled?: boolean): void {
  if (baseUrl !== undefined) QaConfig.baseUrl = baseUrl
  if (token !== undefined) QaConfig.token = token
  if (enabled !== undefined) QaConfig.enabled = enabled
  initialized = true
}

// 递归脱敏：把对象中敏感字段的值替换为 ***
function redact(value: any): any {
  if (Array.isArray(value)) return value.map(redact)
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const k of Object.keys(value)) {
      if (SENSITIVE_KEYS.has(k.toLowerCase())) out[k] = '***'
      else out[k] = redact(value[k])
    }
    return out
  }
  return value
}

interface ReportPayload {
  event: string
  name: string
  result: string
  message: string
  tags: Record<string, unknown>
  timestamp: number
  type?: string
  method?: string
  seq?: number
  headers?: Record<string, unknown>
  request?: unknown
  response?: unknown
  error?: string
  elapsed_ms?: number
}

function buildPayload(name: string, result: string, message: string, tags: Record<string, unknown>, event: string): ReportPayload {
  return {
    event: event || 'case_result',
    name: name || '',
    result: result || 'passed',
    message: message || '',
    tags: tags || {},
    timestamp: Date.now(),
  }
}

async function send(payload: ReportPayload): Promise<boolean> {
  if (!QaConfig.enabled) return false
  if (!initialized) init()
  const url = QaConfig.baseUrl.replace(/\/$/, '') + '/api/qa/report'
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (QaConfig.token) headers['Authorization'] = 'Bearer ' + QaConfig.token
  // 脱敏：request / response / headers 中的敏感字段
  const safe: ReportPayload = { ...payload }
  if (safe.headers) safe.headers = redact(safe.headers)
  if (safe.request !== undefined) safe.request = redact(safe.request)
  if (safe.response !== undefined) safe.response = redact(safe.response)
  try {
    const resp = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(safe),
    })
    return resp.ok
  } catch (e) {
    console.error('[QaSDK] 上报失败:', (e as Error).message)
    return false
  }
}

export function report(name: string, result = 'passed', message = '', tags: Record<string, unknown> = {}): Promise<boolean> {
  return send(buildPayload(name, result, message, tags, 'case_result'))
}

export function log(message: string, tags: Record<string, unknown> = {}): Promise<boolean> {
  return send(buildPayload('log', 'info', message, tags, 'log'))
}

export function logRequest(method: string, headers: Record<string, unknown> = {}, request?: unknown, opts: { seq?: number; elapsedMs?: number; tags?: Record<string, unknown>; message?: string } = {}): Promise<boolean> {
  return send({
    event: 'request', type: 'REQUEST', name: method || '', method: method || '',
    headers, request, seq: opts.seq || 0, elapsed_ms: opts.elapsedMs || 0,
    tags: opts.tags || {}, message: opts.message || '', timestamp: Date.now(),
  })
}

export function logResponse(method: string, headers: Record<string, unknown> = {}, response?: unknown, elapsedMs = 0, opts: { seq?: number; tags?: Record<string, unknown>; message?: string } = {}): Promise<boolean> {
  return send({
    event: 'response', type: 'RESPONSE', name: method || '', method: method || '',
    headers, response, elapsed_ms: elapsedMs, seq: opts.seq || 0,
    tags: opts.tags || {}, message: opts.message || '', timestamp: Date.now(),
  })
}

export function logError(method: string, error: string, elapsedMs = 0, opts: { headers?: Record<string, unknown>; seq?: number; tags?: Record<string, unknown> } = {}): Promise<boolean> {
  return send({
    event: 'error', type: 'ERROR', name: method || '', method: method || '',
    headers: opts.headers || {}, error: error || '', elapsed_ms: elapsedMs,
    seq: opts.seq || 0, tags: opts.tags || {}, message: error || '', timestamp: Date.now(),
  })
}

export function sendRaw(payload: ReportPayload): Promise<boolean> {
  return send(payload)
}

export const QaSDK = { init, report, log, logRequest, logResponse, logError, sendRaw }
export default QaSDK
