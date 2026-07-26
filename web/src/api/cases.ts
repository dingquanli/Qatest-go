import request from './request'
import type {
  TestCase,
  CaseModule,
  CaseExecution,
  CaseExecStats,
} from '@/types'

/* ---------- 测试用例 ---------- */
export const getCases = (): Promise<TestCase[]> => request.get<TestCase[]>('/cases')

export const getTestCase = (id: string): Promise<TestCase> =>
  request.get<TestCase>(`/cases/${encodeURIComponent(id)}`)

export const createCase = (data: Partial<TestCase>): Promise<TestCase> =>
  request.post<TestCase>('/cases', data)

export const updateCase = (id: string, data: Partial<TestCase>): Promise<TestCase> =>
  request.put<TestCase>(`/cases/${encodeURIComponent(id)}`, data)

export const deleteCase = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/cases/${encodeURIComponent(id)}`)

export const batchImportCases = (cases: Partial<TestCase>[]): Promise<{ count: number }> =>
  request.post<{ count: number }>('/cases/batch', { cases })

/* ---------- 用例模块 ---------- */
export const getCaseModules = (): Promise<CaseModule[]> =>
  request.get<CaseModule[]>('/case-modules')

export const createCaseModule = (data: Partial<CaseModule>): Promise<CaseModule> =>
  request.post<CaseModule>('/case-modules', data)

export const updateCaseModule = (id: string, data: Partial<CaseModule>): Promise<CaseModule> =>
  request.put<CaseModule>(`/case-modules/${encodeURIComponent(id)}`, data)

export const deleteCaseModule = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/case-modules/${encodeURIComponent(id)}`)

/* ---------- 用例执行记录 ---------- */
export const getCaseExecutions = (): Promise<CaseExecution[]> =>
  request.get<CaseExecution[]>('/case-executions')

export const getCaseExecutionsStats = (): Promise<CaseExecStats> =>
  request.get<CaseExecStats>('/case-executions/stats')

export const createCaseExecution = (data: Partial<CaseExecution>): Promise<CaseExecution> =>
  request.post<CaseExecution>('/case-executions', data)

export const updateCaseExecution = (
  id: string,
  data: Partial<CaseExecution>,
): Promise<CaseExecution> =>
  request.put<CaseExecution>(`/case-executions/${encodeURIComponent(id)}`, data)

export const deleteCaseExecution = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/case-executions/${encodeURIComponent(id)}`)
