import request from './request'
import type { SdkItem, QaReport } from '@/types'

interface SdkEngine {
  id: string
  label: string
  files: string[]
}

/** 拉取 SDK 引擎列表，并展平为「引擎 × 文件」行，供表格展示。 */
export const getSdkList = async (): Promise<SdkItem[]> => {
  const engines = await request.get<SdkEngine[]>('/config/sdk/list')
  const items: SdkItem[] = []
  for (const e of engines || []) {
    for (const f of e.files || []) {
      items.push({ engine: e.id, engineLabel: e.label, file: f })
    }
  }
  return items
}

/**
 * 下载单个 SDK 文件。
 * 后端 /api/config/sdk/download 在鉴权组内，需携带 JWT，
 * 故用 fetch + Authorization 头拉取 blob 后触发浏览器下载
 * （直接用 <a href> 打开会因无 Authorization 头而 401）。
 */
export async function downloadSdk(engine: string, file: string): Promise<void> {
  const raw = localStorage.getItem('qt_auth')
  const headers: Record<string, string> = {}
  if (raw) {
    try {
      const auth = JSON.parse(raw) as { token?: string }
      if (auth?.token) headers['Authorization'] = `Bearer ${auth.token}`
    } catch {
      /* ignore */
    }
  }
  const url = `/api/config/sdk/download?engine=${encodeURIComponent(engine)}&file=${encodeURIComponent(file)}`
  const res = await fetch(url, { headers })
  if (!res.ok) throw new Error(`下载失败: ${res.status}`)
  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = objectUrl
  a.download = file
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(objectUrl)
}

/** 分页查询 SDK 上报记录（event 可空；返回 { total, items }）。 */
export const getQaReports = (
  params: { event?: string; limit?: number; offset?: number } = {},
): Promise<{ total: number; items: QaReport[] }> =>
  request.get<{ total: number; items: QaReport[] }>('/qa/reports', { params })
