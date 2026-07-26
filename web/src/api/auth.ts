import request from './request'
import type { LoginRequest, LoginResponse, RefreshRequest } from '@/types'

export const login = (data: LoginRequest): Promise<LoginResponse> =>
  request.post<LoginResponse>('/auth/login', data)

export const refreshToken = (data: RefreshRequest): Promise<{ token: string }> =>
  request.post<{ token: string }>('/auth/refresh', data)
