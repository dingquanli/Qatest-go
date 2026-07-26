import request from './request'
import type { XmindCase, XmindModule } from '@/types'

export const getXmindCases = (): Promise<XmindCase[]> =>
  request.get<XmindCase[]>('/xmind-cases')

export const createXmindCase = (data: Partial<XmindCase>): Promise<XmindCase> =>
  request.post<XmindCase>('/xmind-cases', data)

export const updateXmindCase = (id: string, data: Partial<XmindCase>): Promise<XmindCase> =>
  request.put<XmindCase>(`/xmind-cases/${encodeURIComponent(id)}`, data)

export const deleteXmindCase = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/xmind-cases/${encodeURIComponent(id)}`)

export const getXmindModules = (): Promise<XmindModule[]> =>
  request.get<XmindModule[]>('/xmind-modules')

export const createXmindModule = (data: Partial<XmindModule>): Promise<XmindModule> =>
  request.post<XmindModule>('/xmind-modules', data)

export const updateXmindModule = (id: string, data: Partial<XmindModule>): Promise<XmindModule> =>
  request.put<XmindModule>(`/xmind-modules/${encodeURIComponent(id)}`, data)

export const deleteXmindModule = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/xmind-modules/${encodeURIComponent(id)}`)
