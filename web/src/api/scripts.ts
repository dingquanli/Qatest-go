import request from './request'
import type { Script } from '@/types'

export const getScripts = (): Promise<Script[]> => request.get<Script[]>('/scripts')

export const getScript = (id: string): Promise<Script> =>
  request.get<Script>(`/scripts/${encodeURIComponent(id)}`)

export const createScript = (data: Partial<Script>): Promise<Script> =>
  request.post<Script>('/scripts', data)

export const updateScript = (id: string, data: Partial<Script>): Promise<Script> =>
  request.put<Script>(`/scripts/${encodeURIComponent(id)}`, data)

export const deleteScript = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/scripts/${encodeURIComponent(id)}`)
