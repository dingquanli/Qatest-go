import request from './request'
import type { Spreadsheet } from '@/types'

export const getSpreadsheets = (): Promise<Spreadsheet[]> =>
  request.get<Spreadsheet[]>('/spreadsheets')

export const getSpreadsheet = (id: string): Promise<Spreadsheet> =>
  request.get<Spreadsheet>(`/spreadsheets/${encodeURIComponent(id)}`)

export const createSpreadsheet = (data: Partial<Spreadsheet>): Promise<Spreadsheet> =>
  request.post<Spreadsheet>('/spreadsheets', data)

export const updateSpreadsheet = (id: string, data: Partial<Spreadsheet>): Promise<Spreadsheet> =>
  request.put<Spreadsheet>(`/spreadsheets/${encodeURIComponent(id)}`, data)

export const deleteSpreadsheet = (id: string): Promise<{ message: string }> =>
  request.delete<{ message: string }>(`/spreadsheets/${encodeURIComponent(id)}`)
