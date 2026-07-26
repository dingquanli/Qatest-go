import request from './request'

export interface LogFileInfo {
  name: string
  size: number
  modTime: string
}

export interface RawLogLine {
  raw: string
  file: string
}

export const getLogFiles = (): Promise<LogFileInfo[]> =>
  request.get<LogFileInfo[]>('/files')

export const getLogFile = (
  name: string,
): Promise<{ name: string; content: string }> =>
  request.get<{ name: string; content: string }>('/file', { name })

export const getLogEntries = (): Promise<RawLogLine[]> =>
  request.get<RawLogLine[]>('/logs')
