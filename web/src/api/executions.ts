import request from './request'
import type { Execution, CreateExecutionRequest } from '@/types'

export const getExecutions = (): Promise<Execution[]> => request.get<Execution[]>('/executions')

export const getExecution = (id: string): Promise<Execution> =>
  request.get<Execution>(`/executions/${encodeURIComponent(id)}`)

export const createExecution = (data: CreateExecutionRequest): Promise<Execution> =>
  request.post<Execution>('/executions', data)

export const cancelExecution = (id: string): Promise<{ message: string }> =>
  request.post<{ message: string }>(`/executions/${encodeURIComponent(id)}/cancel`)
