import request from './request'
import type { ProtoService } from '@/types'

export const getProtoServices = (): Promise<ProtoService[]> =>
  request.get<ProtoService[]>('/proto/services')

export const getProtoDescribe = (): Promise<unknown> =>
  request.get<unknown>('/proto/describe')

export const getProtoDir = (): Promise<{ dir: string }> =>
  request.get<{ dir: string }>('/proto/setdir')

export const setProtoDir = (dir: string): Promise<{ dir: string }> =>
  request.post<{ dir: string }>('/proto/setdir', { dir })

export const describeProtoMethod = (method: string): Promise<unknown> =>
  request.post<unknown>('/proto/describe-method', { method })
