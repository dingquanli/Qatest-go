import request from './request'
import type { TestPlan, PlanExecution, AutoTaskExecution } from '@/types'

/* ---------- 测试计划 ---------- */
export const getTestPlans = (): Promise<TestPlan[]> => request.get<TestPlan[]>('/test-plans')

export const getTestPlan = (id: string): Promise<TestPlan> =>
  request.get<TestPlan>(`/test-plans/${encodeURIComponent(id)}`)

export const createTestPlan = (data: Partial<TestPlan>): Promise<TestPlan> =>
  request.post<TestPlan>('/test-plans', data)

export const updateTestPlan = (id: string, data: Partial<TestPlan>): Promise<TestPlan> =>
  request.put<TestPlan>(`/test-plans/${encodeURIComponent(id)}`, data)

export const deleteTestPlan = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/test-plans/${encodeURIComponent(id)}`)

/* ---------- 测试计划执行引擎 ---------- */
export interface ExecutePlanPayload {
  mode?: 'manual' | 'auto'
  deviceSerial?: string
  executor?: string
}
export const executeTestPlan = (
  id: string,
  payload: ExecutePlanPayload = {},
): Promise<{ planExecution: string; mode: string; caseExecutions: CaseExecution[] }> =>
  request.post<{ planExecution: string; mode: string; caseExecutions: CaseExecution[] }>(
    `/test-plans/${encodeURIComponent(id)}/execute`,
    payload,
  )

/* ---------- 计划执行记录 ---------- */
export const getPlanExecutions = (): Promise<PlanExecution[]> =>
  request.get<PlanExecution[]>('/plan-executions')

export const createPlanExecution = (data: Partial<PlanExecution>): Promise<PlanExecution> =>
  request.post<PlanExecution>('/plan-executions', data)

/* ---------- 自动化任务执行记录 ---------- */
export const getAutoTaskExecutions = (): Promise<AutoTaskExecution[]> =>
  request.get<AutoTaskExecution[]>('/auto-task-executions')

export const createAutoTaskExecution = (
  data: Partial<AutoTaskExecution>,
): Promise<AutoTaskExecution> => request.post<AutoTaskExecution>('/auto-task-executions', data)
