import request from './request'
import type {
  ProxyStatus,
  ProxyExecution,
  ProxySendRequest,
  ProxyReplayRequest,
} from '@/types'

export const getProxyStatus = (): Promise<ProxyStatus> =>
  request.get<ProxyStatus>('/proxy/status')

export const startProxy = (target?: string): Promise<ProxyStatus> =>
  request.post<ProxyStatus>('/proxy/start', target ? { target } : {})

export const stopProxy = (): Promise<{ message: string }> =>
  request.post<{ message: string }>('/proxy/stop')

export const pauseProxy = (): Promise<ProxyStatus> =>
  request.post<ProxyStatus>('/proxy/pause')

export const sendProxyRequest = (data: ProxySendRequest): Promise<unknown> =>
  request.post<unknown>('/proxy/send', data)

export const replayProxy = (data: ProxyReplayRequest): Promise<unknown> =>
  request.post<unknown>('/proxy/replay', data)

export const getProxyLogs = (): Promise<{ logs: string[] }> =>
  request.get<{ logs: string[] }>('/proxy/logs')

export const getProxyExecutions = (): Promise<ProxyExecution[]> =>
  request.get<ProxyExecution[]>('/proxy/executions')

export const clearProxyExecutions = (): Promise<{ message: string }> =>
  request.delete<{ message: string }>('/proxy/executions')
