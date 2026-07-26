import dayjs from 'dayjs'

/**
 * 安全解析 JSON 字符串。后端对 steps/tags/caseIds 等字段返回的是 JSON 字符串，
 * 解析前需判空，避免 JSON.parse('') 抛错。
 */
export function safeParseJSON<T = unknown>(value: unknown, fallback: T): T {
  if (value === null || value === undefined) return fallback
  if (typeof value !== 'string') return value as unknown as T
  const str = value.trim()
  if (str === '') return fallback
  try {
    return JSON.parse(str) as T
  } catch {
    return fallback
  }
}

/** 将 RFC3339 时间格式化为指定格式，非法返回 '-'。 */
export function formatDate(value?: string | null, fmt = 'YYYY-MM-DD HH:mm:ss'): string {
  if (!value) return '-'
  const d = dayjs(value)
  return d.isValid() ? d.format(fmt) : '-'
}

/** 短日期 MM-DD，用于图表 x 轴。 */
export function formatDateShort(value?: string | null): string {
  return formatDate(value, 'MM-DD')
}

/** 取最近 N 天的日期标签（MM-DD），含今天。 */
export function lastNDates(n: number): string[] {
  const out: string[] = []
  const today = dayjs()
  for (let i = n - 1; i >= 0; i--) {
    out.push(today.subtract(i, 'day').format('MM-DD'))
  }
  return out
}

/** 把任意值格式化为可读 JSON 字符串。 */
export function prettyJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

/** 下载文本为文件。 */
export function downloadText(filename: string, content: string, mime = 'text/plain'): void {
  const blob = new Blob([content], { type: mime })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
