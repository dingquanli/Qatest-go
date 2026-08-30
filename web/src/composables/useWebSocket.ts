import { ref, onUnmounted } from 'vue'

export type WsStatus = 'closed' | 'connecting' | 'open'

/**
 * 执行日志 WebSocket（只读）。连接 /api/ws，升级后首条消息发送 {"type":"auth","token":<JWT>} 认证
 * （避免 ?token= 把 JWT 泄漏进访问日志/浏览器历史），收到 auth_ok 后服务端才开始推送。
 */
export function useWebSocket(getToken: () => string) {
  const messages = ref<string[]>([])
  const status = ref<WsStatus>('closed')
  let ws: WebSocket | null = null

  function buildUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${location.host}/api/ws`
  }

  function connect(): void {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }
    status.value = 'connecting'
    ws = new WebSocket(buildUrl())
    ws.onopen = () => {
      // 首消息认证：必须是连接建立后的第一帧
      ws?.send(JSON.stringify({ type: 'auth', token: getToken() }))
    }
    ws.onmessage = (e: MessageEvent) => {
      const raw = typeof e.data === 'string' ? e.data : String(e.data)
      // 控制帧（认证结果）不进入业务消息列表
      try {
        const ctrl = JSON.parse(raw)
        if (ctrl?.type === 'auth_ok') {
          status.value = 'open'
          return
        }
        if (ctrl?.type === 'auth_failed') {
          ws?.close()
          return
        }
      } catch {
        /* 非 JSON 即业务日志帧，继续入列 */
      }
      messages.value.push(raw)
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
