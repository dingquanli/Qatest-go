import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

/**
 * 统一响应信封：{ success, data, error }
 * 成功时响应拦截器直接返回 res.data.data（即业务数据），
 * 失败（success=false 或 HTTP 非 2xx）则 reject(Error)。
 */
const instance: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

instance.interceptors.request.use((config) => {
  const raw = localStorage.getItem('qt_auth')
  if (raw) {
    try {
      const auth = JSON.parse(raw) as { token?: string }
      if (auth?.token) {
        config.headers = config.headers ?? {}
        config.headers.Authorization = `Bearer ${auth.token}`
      }
    } catch {
      /* ignore corrupted storage */
    }
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
  (error) => {
    const status = error.response?.status
    if (status === 401) {
      localStorage.removeItem('qt_auth')
      if (router.currentRoute.value.name !== 'login') {
        router.push('/login')
      }
      ElMessage.error('登录已过期，请重新登录')
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
