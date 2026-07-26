import request from './request'
import type { APIRequest, APIFolder, APIHistory, SendApiRequest } from '@/types'

/* ---------- 接口请求 ---------- */
export const getApiRequests = (): Promise<APIRequest[]> =>
  request.get<APIRequest[]>('/api-requests')

export const getApiRequest = (id: string): Promise<APIRequest> =>
  request.get<APIRequest>(`/api-requests/${encodeURIComponent(id)}`)

export const createApiRequest = (data: Partial<APIRequest>): Promise<APIRequest> =>
  request.post<APIRequest>('/api-requests', data)

export const updateApiRequest = (id: string, data: Partial<APIRequest>): Promise<APIRequest> =>
  request.put<APIRequest>(`/api-requests/${encodeURIComponent(id)}`, data)

export const deleteApiRequest = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/api-requests/${encodeURIComponent(id)}`)

/* ---------- 文件夹 ---------- */
export const getApiFolders = (): Promise<APIFolder[]> => request.get<APIFolder[]>('/api-folders')

export const createApiFolder = (data: Partial<APIFolder>): Promise<APIFolder> =>
  request.post<APIFolder>('/api-folders', data)

export const updateApiFolder = (id: string, data: Partial<APIFolder>): Promise<APIFolder> =>
  request.put<APIFolder>(`/api-folders/${encodeURIComponent(id)}`, data)

export const deleteApiFolder = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/api-folders/${encodeURIComponent(id)}`)

/* ---------- 历史 ---------- */
export const getApiHistory = (): Promise<APIHistory[]> => request.get<APIHistory[]>('/api-history')

export const createApiHistory = (data: Partial<APIHistory>): Promise<APIHistory> =>
  request.post<APIHistory>('/api-history', data)

export const clearApiHistory = (): Promise<{ message: string }> =>
  request.delete<{ message: string }>('/api-history')

/* ---------- 发送请求（由后端代理执行，返回响应） ---------- */
export const sendApiRequest = (data: SendApiRequest): Promise<unknown> =>
  request.post<unknown>('/proxy/send', data)

/** 便捷：向任意 URL 直接发请求（经后端 /proxy/send）。 */
export const sendHttpRequest = (data: SendApiRequest): Promise<{
  status: number
  statusText?: string
  headers?: Record<string, string>
  body?: string
  duration?: number
  error?: string
}> => request.post('/proxy/send', data)
