import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as apiLogin, logout as apiLogout } from '@/api/auth'
import type { AuthState, LoginRequest, UserInfo } from '@/types'

// P1-12 说明：JWT 存储于 localStorage，存在 XSS 风险。
// 内部测试工具可接受，但生产环境建议改用 HttpOnly Cookie + CSRF Token 方案。
const STORAGE_KEY = 'qt_auth'

function loadAuth(): AuthState | null {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as AuthState
  } catch {
    return null
  }
}

export const useUserStore = defineStore('user', () => {
  const auth = ref<AuthState | null>(loadAuth())

  const token = computed(() => auth.value?.token ?? '')
  const isLoggedIn = computed(() => !!auth.value?.loggedIn && !!auth.value?.token)
  const user = computed<UserInfo | null>(() =>
    auth.value
      ? {
          id: auth.value.username,
          username: auth.value.username,
          name: auth.value.name,
          role: auth.value.role,
        }
      : null,
  )

  async function login(data: LoginRequest): Promise<AuthState> {
    const res = await apiLogin(data)
    const state: AuthState = {
      loggedIn: true,
      username: res.user.username,
      name: res.user.name,
      role: res.user.role,
      token: res.token,
      refreshToken: res.refreshToken,
    }
    auth.value = state
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    return state
  }

  function logout(): void {
    // 先撤销服务端刷新令牌（轮换式令牌，撤销后旧令牌无法再换新），
    // 再清除本地状态；撤销请求失败不阻塞本地登出。
    const refreshToken = auth.value?.refreshToken
    auth.value = null
    localStorage.removeItem(STORAGE_KEY)
    if (refreshToken) {
      apiLogout({ token: refreshToken }).catch(() => {})
    }
  }

  function getToken(): string {
    return auth.value?.token ?? ''
  }

  /** 从 localStorage 重新加载认证状态（request.ts 后台刷新令牌后调用，保持 Pinia 同步） */
  function syncFromStorage(): void {
    auth.value = loadAuth()
  }

  return { auth, token, isLoggedIn, user, login, logout, getToken, syncFromStorage }
})
