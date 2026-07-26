import request from '@/api/request'
import type { TableCase, TableModule } from '@/types'

export const getTableCases = (): Promise<TableCase[]> =>
  request.get<TableCase[]>('/table-cases')

export const createTableCase = (data: Partial<TableCase>): Promise<TableCase> =>
  request.post<TableCase>('/table-cases', data)

export const updateTableCase = (id: string, data: Partial<TableCase>): Promise<TableCase> =>
  request.put<TableCase>(`/table-cases/${encodeURIComponent(id)}`, data)

export const deleteTableCase = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/table-cases/${encodeURIComponent(id)}`)

export const getTableModules = (): Promise<TableModule[]> =>
  request.get<TableModule[]>('/table-modules')

export const createTableModule = (data: Partial<TableModule>): Promise<TableModule> =>
  request.post<TableModule>('/table-modules', data)

export const updateTableModule = (id: string, data: Partial<TableModule>): Promise<TableModule> =>
  request.put<TableModule>(`/table-modules/${encodeURIComponent(id)}`, data)

export const deleteTableModule = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/table-modules/${encodeURIComponent(id)}`)
