import request from './request'
import type { SettingsMap } from '@/types'

export const getSettings = (): Promise<SettingsMap> => request.get<SettingsMap>('/settings')

export const updateSettings = (data: SettingsMap): Promise<SettingsMap> =>
  request.put<SettingsMap>('/settings', data)

export const getSetting = <T = unknown>(key: string): Promise<T> =>
  request.get<T>(`/settings/${encodeURIComponent(key)}`)

export const updateSetting = <T = unknown>(key: string, value: unknown): Promise<T> =>
  request.put<T>(`/settings/${encodeURIComponent(key)}`, { value })
