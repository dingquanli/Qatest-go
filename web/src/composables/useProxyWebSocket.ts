import { ref, onUnmounted } from 'vue'

export type ProxyWsStatus = 'closed' | 'connecting' | 'open'

/**
 * gRPC 代理 WebSocket（双向）。连接 /api/proxy-ws，升级后首条消息发送
 * {"type":"auth","token":<JWT>} 认证（避免 ?token= 泄漏 JWT），收到 auth_ok 后
 * 服务端开始推送拦截帧；前端可通过 send() 把决策（允许/拦截/修改）发回后端。
 * WebSocket 消息按序到达，首帧一定是认证帧，无需在客户端做发送队列。
 */
export function useProxyWebSocket(getToken: () => string) {
  const messages = ref<string[]>([])
  const status = ref<ProxyWsStatus>('closed')
  let ws: WebSocket | null = null
  let messageHandler: ((raw: string) => void) | null = null

  function buildUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${location.host}/api/proxy-ws`
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
        /* 非 JSON 即业务帧，继续入列 */
      }
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
