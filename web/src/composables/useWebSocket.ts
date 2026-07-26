import { ref, onUnmounted } from 'vue'

export type WsStatus = 'closed' | 'connecting' | 'open'

/**
 * 执行日志 WebSocket（只读）。连接 /api/ws?token=<JWT>。
 */
export function useWebSocket(getToken: () => string) {
  const messages = ref<string[]>([])
  const status = ref<WsStatus>('closed')
  let ws: WebSocket | null = null

  function buildUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${location.host}/api/ws?token=${encodeURIComponent(getToken())}`
  }

  function connect(): void {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }
    status.value = 'connecting'
    ws = new WebSocket(buildUrl())
    ws.onopen = () => {
      status.value = 'open'
    }
    ws.onmessage = (e: MessageEvent) => {
      messages.value.push(typeof e.data === 'string' ? e.data : String(e.data))
      if (messages.value.length > 1000) messages.value.shift()
    }
    ws.onclose = () => {
      status.value = 'closed'
    }
    ws.onerror = () => {
      status.value = 'closed'
    }
  }

  function close(): void {
    ws?.close()
    ws = null
    status.value = 'closed'
  }

  function clear(): void {
    messages.value = []
  }

  onUnmounted(close)

  return { messages, status, connect, close, clear }
}
