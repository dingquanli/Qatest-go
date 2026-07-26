import request from './request'
import type { Bug, BugStats } from '@/types'

export const getBugs = (): Promise<Bug[]> => request.get<Bug[]>('/bugs')

export const getBugStats = (): Promise<BugStats> => request.get<BugStats>('/bugs/stats')

export const getBug = (id: string): Promise<Bug> =>
  request.get<Bug>(`/bugs/${encodeURIComponent(id)}`)

export const createBug = (data: Partial<Bug>): Promise<Bug> =>
  request.post<Bug>('/bugs', data)

export const updateBug = (id: string, data: Partial<Bug>): Promise<Bug> =>
  request.put<Bug>(`/bugs/${encodeURIComponent(id)}`, data)

export const deleteBug = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/bugs/${encodeURIComponent(id)}`)

/** 同步到外部系统（Jira 等）。 */
export const syncBug = (id: string): Promise<unknown> =>
  request.post<unknown>(`/bugs/${encodeURIComponent(id)}/sync`)

/** 查询 Jira 是否已配置（脱敏，不返回 token）。 */
export const getJiraStatus = (): Promise<{ configured: boolean; baseUrl: string; project: string }> =>
  request.get<{ configured: boolean; baseUrl: string; project: string }>('/config/jira/status')
