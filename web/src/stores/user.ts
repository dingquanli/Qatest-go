import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as apiLogin } from '@/api/auth'
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
    }
    auth.value = state
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    return state
  }

  function logout(): void {
    auth.value = null
    localStorage.removeItem(STORAGE_KEY)
  }

  function getToken(): string {
    return auth.value?.token ?? ''
  }

  return { auth, token, isLoggedIn, user, login, logout, getToken }
})
