import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { AxiosError } from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

// request.ts 依赖 router 与 element-plus 的副作用，测试中用轻量替身
vi.mock('@/router', () => ({
  default: {
    currentRoute: { value: { name: 'test' } },
    push: vi.fn(),
  },
}))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), warning: vi.fn() },
}))

import { axiosInstance } from './request'
import { ElMessage } from 'element-plus'
import router from '@/router'

const AUTH_KEY = 'qt_auth'

type AdapterCall = { url: string; auth: string }

/** 构造可编程的 axios adapter：
 *  - /auth/refresh 返回新令牌对（可配置失败）
 *  - 带 expired-token 的请求一律 401（模拟令牌过期）
 *  - 其余请求 200 并回显 Authorization */
function installAdapter(opts: { refreshFails?: boolean } = {}) {
  const calls: AdapterCall[] = []
  let refreshCount = 0
  axiosInstance.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
    const url = config.url ?? ''
    const auth = String((config.headers as Record<string, unknown>)?.Authorization ?? '')
    calls.push({ url, auth })
    if (url === '/auth/refresh') {
      refreshCount++
      if (opts.refreshFails) {
        throw new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, {
          status: 401, data: {},
        } as AxiosResponse)
      }
      return {
        data: { success: true, data: { token: 'new-token', refreshToken: 'new-refresh' } },
        status: 200, statusText: 'OK', config, headers: {},
      } as AxiosResponse
    }
    if (auth.includes('expired-token')) {
      throw new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, {
        status: 401, data: {},
      } as AxiosResponse)
    }
    return {
      data: { success: true, data: { got: auth } },
      status: 200, statusText: 'OK', config, headers: {},
    } as AxiosResponse
  }
  return {
    calls,
    refreshCount: () => refreshCount,
  }
}

function seedAuth(token: string, refreshToken?: string): void {
  localStorage.setItem(AUTH_KEY, JSON.stringify({
    loggedIn: true, username: 'admin', name: '管理员', role: 'admin',
    token, ...(refreshToken ? { refreshToken } : {}),
  }))
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  // writeAuth 刷新后会同步 Pinia store，测试里需先激活
  setActivePinia(createPinia())
})

describe('request.ts 401 刷新与重试', () => {
  it('令牌过期时自动刷新并重放原请求', async () => {
    seedAuth('expired-token', 'refresh-1')
    const adapter = installAdapter()

    const res = await axiosInstance.get('/cases')

    expect(res).toMatchObject({ got: 'Bearer new-token' })
    expect(adapter.refreshCount()).toBe(1)
    expect(adapter.calls.map((c) => c.url)).toEqual(['/cases', '/auth/refresh', '/cases'])
    // 新令牌对已持久化，且保留用户信息
    const saved = JSON.parse(localStorage.getItem(AUTH_KEY)!)
    expect(saved).toMatchObject({ token: 'new-token', refreshToken: 'new-refresh', username: 'admin' })
  })

  it('并发 401 只触发一次刷新（single-flight）', async () => {
    seedAuth('expired-token', 'refresh-1')
    const adapter = installAdapter()

    const [a, b] = await Promise.all([
      axiosInstance.get('/cases'),
      axiosInstance.get('/bugs'),
    ])
    expect(a).toMatchObject({ got: 'Bearer new-token' })
    expect(b).toMatchObject({ got: 'Bearer new-token' })
    expect(adapter.refreshCount()).toBe(1)
  })

  it('刷新失败则清除登录态并跳转登录页，不重放原请求', async () => {
    seedAuth('expired-token', 'refresh-1')
    const adapter = installAdapter({ refreshFails: true })

    await expect(axiosInstance.get('/cases')).rejects.toBeTruthy()

    expect(adapter.calls.map((c) => c.url)).toEqual(['/cases', '/auth/refresh'])
    expect(localStorage.getItem(AUTH_KEY)).toBeNull()
    expect(router.push).toHaveBeenCalledWith('/login')
    expect(ElMessage.error).toHaveBeenCalled()
  })

  it('无 refreshToken 时 401 直接登出，不尝试刷新', async () => {
    seedAuth('expired-token')
    const adapter = installAdapter()

    await expect(axiosInstance.get('/cases')).rejects.toBeTruthy()

    expect(adapter.calls.map((c) => c.url)).toEqual(['/cases'])
    expect(localStorage.getItem(AUTH_KEY)).toBeNull()
    expect(router.push).toHaveBeenCalledWith('/login')
  })
})
