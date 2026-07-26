import request from './request'
import type { APIDefinition, APIDefModule } from '@/types'

/* ---------- 接口定义 ---------- */
export const getApiDefinitions = (): Promise<APIDefinition[]> =>
  request.get<APIDefinition[]>('/api-definitions')

export const getApiDefinition = (id: string): Promise<APIDefinition> =>
  request.get<APIDefinition>(`/api-definitions/${encodeURIComponent(id)}`)

export const createApiDefinition = (data: Partial<APIDefinition>): Promise<APIDefinition> =>
  request.post<APIDefinition>('/api-definitions', data)

export const updateApiDefinition = (
  id: string,
  data: Partial<APIDefinition>,
): Promise<APIDefinition> =>
  request.put<APIDefinition>(`/api-definitions/${encodeURIComponent(id)}`, data)

export const deleteApiDefinition = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/api-definitions/${encodeURIComponent(id)}`)

/* ---------- 接口定义模块 ---------- */
export const getApiDefModules = (): Promise<APIDefModule[]> =>
  request.get<APIDefModule[]>('/api-def-modules')

export const createApiDefModule = (data: Partial<APIDefModule>): Promise<APIDefModule> =>
  request.post<APIDefModule>('/api-def-modules', data)

export const updateApiDefModule = (
  id: string,
  data: Partial<APIDefModule>,
): Promise<APIDefModule> =>
  request.put<APIDefModule>(`/api-def-modules/${encodeURIComponent(id)}`, data)

export const deleteApiDefModule = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/api-def-modules/${encodeURIComponent(id)}`)
