import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

/**
 * 统一响应信封：{ success, data, error }
 * 成功时响应拦截器直接返回 res.data.data（即业务数据），
 * 失败（success=false 或 HTTP 非 2xx）则 reject(Error)。
 * 401 时若存在 refreshToken 则自动轮换刷新一次并重放原请求（single-flight，并发 401 只刷一次）。
 */
const instance: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 供测试注入自定义 adapter（模拟 401/刷新流程），业务代码请使用默认导出的 request
export const axiosInstance = instance

function readAuth(): { token?: string; refreshToken?: string } | null {
  const raw = localStorage.getItem('qt_auth')
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

function writeAuth(auth: { token: string; refreshToken?: string }): void {
  const prev = readAuth()
  localStorage.setItem(
    'qt_auth',
    JSON.stringify({ ...(prev ?? {}), loggedIn: true, token: auth.token, refreshToken: auth.refreshToken }),
  )
  // 保持 Pinia store 同步（WebSocket 连接从 store 取 token）
  try {
    // 动态 import 避免与 stores 形成模块初始化循环
    import('@/stores/user')
      .then(({ useUserStore }) => {
        useUserStore().syncFromStorage()
      })
      .catch(() => {
        /* store 未初始化或 Pinia 未激活时忽略（localStorage 已是事实来源） */
      })
  } catch {
    /* 同上 */
  }
}

function clearAuthAndRedirect(): void {
  localStorage.removeItem('qt_auth')
  if (router.currentRoute.value.name !== 'login') {
    router.push('/login')
  }
  ElMessage.error('登录已过期，请重新登录')
}

// single-flight：并发多个 401 共享同一次刷新
let refreshInFlight: Promise<boolean> | null = null

function tryRefresh(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight
  const auth = readAuth()
  if (!auth?.refreshToken) return Promise.resolve(false)
  // 直接用底层 instance 调 /auth/refresh，避免与 api/auth.ts 形成模块循环依赖；
  // 轮换式刷新：旧 refreshToken 用后即废，响应下发新的 token + refreshToken
  refreshInFlight = instance
    .post('/auth/refresh', { token: auth.refreshToken })
    .then((res) => {
      // 响应拦截器已把 AxiosResponse 解包为业务数据 { token, refreshToken }
      const body = res as unknown as { token?: string; refreshToken?: string }
      if (!body?.token || !body?.refreshToken) return false
      writeAuth({ token: body.token, refreshToken: body.refreshToken })
      return true
    })
    .catch(() => false)
    .finally(() => {
      refreshInFlight = null
    })
  return refreshInFlight
}

instance.interceptors.request.use((config) => {
  const auth = readAuth()
  if (auth?.token) {
    config.headers = config.headers ?? {}
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

instance.interceptors.response.use(
  (response: AxiosResponse) => {
    const body = response.data
    if (body && typeof body === 'object' && 'success' in body) {
      if (!body.success) {
        return Promise.reject(new Error(body.error || '请求失败'))
      }
      // 返回业务数据本体
      return body.data
    }
    return body
  },
  async (error) => {
    const status = error.response?.status
    if (status === 401) {
      const original = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined
      const isRefreshCall = original?.url?.includes('/auth/refresh')
      if (original && !original._retried && !isRefreshCall && (await tryRefresh())) {
        original._retried = true
        const auth = readAuth()
        original.headers = original.headers ?? {}
        original.headers.Authorization = `Bearer ${auth?.token ?? ''}`
        return instance.request(original)
      }
      clearAuthAndRedirect()
    } else if (status === 501) {
      ElMessage.warning('该功能后端尚未实现（501）')
    } else {
      const msg = error.response?.data?.error || error.message || '网络错误'
      // 静默 501 已由上面处理；其余错误交由调用方 catch
      void msg
    }
    return Promise.reject(error)
  },
)

/**
 * 类型安全的请求封装。由于响应拦截器已把 AxiosResponse 解包为业务数据，
 * 这里把返回类型断言为 Promise<T>，调用方直接拿到 data 字段的值。
 */
function http<T = unknown>(config: AxiosRequestConfig): Promise<T> {
  return instance(config) as unknown as Promise<T>
}

export const request = {
  get: <T = unknown>(url: string, params?: Record<string, unknown>): Promise<T> =>
    http<T>({ url, method: 'GET', params }),
  post: <T = unknown>(url: string, data?: unknown): Promise<T> =>
    http<T>({ url, method: 'POST', data }),
  put: <T = unknown>(url: string, data?: unknown): Promise<T> =>
    http<T>({ url, method: 'PUT', data }),
  delete: <T = unknown>(url: string): Promise<T> => http<T>({ url, method: 'DELETE' }),
}

export default request
