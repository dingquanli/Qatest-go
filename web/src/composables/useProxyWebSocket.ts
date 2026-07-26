import { ref, onUnmounted } from 'vue'

export type ProxyWsStatus = 'closed' | 'connecting' | 'open'

/**
 * gRPC 代理 WebSocket（双向）。连接 /api/proxy-ws?token=<JWT>。
 * 后端会推送拦截到的帧；前端可通过 send() 把决策（允许/拦截/修改）发回后端。
 */
export function useProxyWebSocket(getToken: () => string) {
  const messages = ref<string[]>([])
  const status = ref<ProxyWsStatus>('closed')
  let ws: WebSocket | null = null
  let messageHandler: ((raw: string) => void) | null = null

  function buildUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${location.host}/api/proxy-ws?token=${encodeURIComponent(getToken())}`
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
      const raw = typeof e.data === 'string' ? e.data : String(e.data)
      messages.value.push(raw)
      if (messages.value.length > 1000) messages.value.shift()
      messageHandler?.(raw)
    }
    ws.onclose = () => {
      status.value = 'closed'
    }
    ws.onerror = () => {
      status.value = 'closed'
    }
  }

  function send(raw: string): void {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(raw)
    }
  }

  function onMessage(fn: (raw: string) => void): void {
    messageHandler = fn
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

  return { messages, status, connect, send, onMessage, close, clear }
}
