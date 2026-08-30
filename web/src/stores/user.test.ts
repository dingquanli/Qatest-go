import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  logout: vi.fn().mockResolvedValue(undefined),
}))

import { login as apiLogin, logout as apiLogout } from '@/api/auth'
import { useUserStore } from './user'

const AUTH_KEY = 'qt_auth'

const loginResponse = {
  token: 'access-token',
  refreshToken: 'refresh-token',
  user: { id: 'admin', username: 'admin', name: '管理员', role: 'admin' },
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  setActivePinia(createPinia())
})

describe('user store', () => {
  it('登录持久化 token 与 refreshToken', async () => {
    vi.mocked(apiLogin).mockResolvedValue(loginResponse)
    const store = useUserStore()

    const state = await store.login({ username: 'admin', password: 'pw' })

    expect(state.token).toBe('access-token')
    expect(state.refreshToken).toBe('refresh-token')
    expect(store.isLoggedIn).toBe(true)
    const saved = JSON.parse(localStorage.getItem(AUTH_KEY)!)
    expect(saved).toMatchObject({ token: 'access-token', refreshToken: 'refresh-token', role: 'admin' })
  })

  it('登出清除本地状态并调用服务端撤销刷新令牌', async () => {
    vi.mocked(apiLogin).mockResolvedValue(loginResponse)
    const store = useUserStore()
    await store.login({ username: 'admin', password: 'pw' })

    store.logout()

    expect(store.isLoggedIn).toBe(false)
    expect(localStorage.getItem(AUTH_KEY)).toBeNull()
    expect(apiLogout).toHaveBeenCalledWith({ token: 'refresh-token' })
  })

  it('登出时撤销接口失败不阻塞本地登出', async () => {
    vi.mocked(apiLogin).mockResolvedValue(loginResponse)
    vi.mocked(apiLogout).mockRejectedValue(new Error('network down'))
    const store = useUserStore()
    await store.login({ username: 'admin', password: 'pw' })

    store.logout()

    expect(localStorage.getItem(AUTH_KEY)).toBeNull()
    expect(store.isLoggedIn).toBe(false)
  })

  it('无刷新令牌时登出不应调用撤销接口', async () => {
    vi.mocked(apiLogin).mockResolvedValue({ ...loginResponse, refreshToken: undefined })
    const store = useUserStore()
    await store.login({ username: 'admin', password: 'pw' })

    store.logout()

    expect(apiLogout).not.toHaveBeenCalled()
    expect(localStorage.getItem(AUTH_KEY)).toBeNull()
  })
})
