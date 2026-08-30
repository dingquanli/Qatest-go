import request from './request'
import type { LoginRequest, LoginResponse, RefreshRequest, RefreshResponse } from '@/types'

export const login = (data: LoginRequest): Promise<LoginResponse> =>
  request.post<LoginResponse>('/auth/login', data)

/** 轮换式刷新：旧 refreshToken 用后即废，响应返回新的 token + refreshToken */
export const refreshToken = (data: RefreshRequest): Promise<RefreshResponse> =>
  request.post<RefreshResponse>('/auth/refresh', data)

/** 登出：撤销 refreshToken（幂等，令牌无效也返回成功） */
export const logout = (data: RefreshRequest): Promise<void> =>
  request.post<void>('/auth/logout', data)
