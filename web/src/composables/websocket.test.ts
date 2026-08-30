import { beforeEach, describe, expect, it, vi } from 'vitest'

/** 可编程的 WebSocket 替身：记录 send 的帧、允许测试侧模拟服务端下发/关闭 */
class FakeWebSocket {
  static OPEN = 1
  static CONNECTING = 0
  static instances: FakeWebSocket[] = []

  readyState = 0
  sent: string[] = []
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(public url: string) {
    FakeWebSocket.instances.push(this)
  }
  send(data: string): void {
    this.sent.push(data)
  }
  close(): void {
    this.closed = true
    this.readyState = 3
    this.onclose?.()
  }
  // 测试辅助：模拟服务端帧
  serverSend(obj: unknown): void {
    this.onmessage?.({ data: JSON.stringify(obj) })
  }
}

vi.stubGlobal('WebSocket', FakeWebSocket)

import { useWebSocket } from './useWebSocket'
import { useProxyWebSocket } from './useProxyWebSocket'

beforeEach(() => {
  FakeWebSocket.instances = []
})

describe('useWebSocket（执行日志）首消息认证', () => {
  it('连接后首帧必须是 auth 消息，URL 不携带 token', () => {
    const { connect, status } = useWebSocket(() => 'jwt-123')
    connect()

    const ws = FakeWebSocket.instances[0]
    expect(ws.url).toBe(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/ws`)
    expect(ws.url).not.toContain('token=')
    expect(ws.sent).toEqual([]) // open 之前不发送

    ws.onopen?.()
    const frame = JSON.parse(ws.sent[0])
    expect(frame).toEqual({ type: 'auth', token: 'jwt-123' })
    expect(status.value).toBe('connecting') // 认证未通过前不视为 open
  })

  it('auth_ok 后进入 open 并接收业务帧；控制帧不入消息列表', () => {
    const { connect, status, messages } = useWebSocket(() => 'jwt-123')
    connect()
    const ws = FakeWebSocket.instances[0]
    ws.onopen?.()
    ws.serverSend({ type: 'auth_ok' })
    expect(status.value).toBe('open')

    ws.serverSend({ type: 'log', message: 'hello' })
    expect(messages.value).toHaveLength(1)
    expect(JSON.parse(messages.value[0])).toMatchObject({ type: 'log' })
  })

  it('auth_failed 时关闭连接', () => {
    const { connect, status } = useWebSocket(() => 'jwt-123')
    connect()
    const ws = FakeWebSocket.instances[0]
    ws.onopen?.()
    ws.serverSend({ type: 'auth_failed', error: '令牌无效' })
    expect(ws.closed).toBe(true)
    expect(status.value).toBe('closed')
  })
})

describe('useProxyWebSocket（协议录制）首消息认证', () => {
  it('open 后首帧发送 auth，auth_ok 前不视为 open', () => {
    const { connect, status } = useProxyWebSocket(() => 'jwt-abc')
    connect()
    const ws = FakeWebSocket.instances[0]
    expect(ws.url).not.toContain('token=')

    ws.readyState = FakeWebSocket.OPEN // 真实 WebSocket 在握手完成后自动变为 OPEN
    ws.onopen?.()
    expect(JSON.parse(ws.sent[0])).toEqual({ type: 'auth', token: 'jwt-abc' })
    expect(status.value).toBe('connecting')

    ws.serverSend({ type: 'auth_ok' })
    expect(status.value).toBe('open')
  })

  it('OPEN 状态下 send 用于发送决策帧；业务帧进入消息列表并触发 handler', () => {
    const { connect, send, messages, onMessage } = useProxyWebSocket(() => 'jwt-abc')
    const seen: string[] = []
    onMessage((raw) => seen.push(raw))
    connect()
    const ws = FakeWebSocket.instances[0]
    ws.readyState = FakeWebSocket.OPEN
    ws.onopen?.()
    ws.serverSend({ type: 'auth_ok' })

    send('{"type":"allow"}')
    expect(ws.sent[1]).toBe('{"type":"allow"}')

    ws.serverSend({ type: 'frame', data: 'x' })
    expect(messages.value).toHaveLength(1)
    expect(seen).toHaveLength(1)
  })
})
