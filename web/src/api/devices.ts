import request from './request'
import type { DeviceInfo } from '@/types'

export const getDevices = (): Promise<DeviceInfo[]> => request.get<DeviceInfo[]>('/devices')

export const scanDevices = (): Promise<DeviceInfo[]> => request.get<DeviceInfo[]>('/devices/scan')

export const getDevice = (serial: string): Promise<DeviceInfo> =>
  request.get<DeviceInfo>(`/devices/${encodeURIComponent(serial)}`)

export const takeScreenshot = (serial: string): Promise<{ path: string }> =>
  request.post<{ path: string }>(`/devices/${encodeURIComponent(serial)}/screenshot`)

export const execDeviceCommand = (
  serial: string,
  command: string,
): Promise<{ output: string }> =>
  request.post<{ output: string }>(`/devices/${encodeURIComponent(serial)}/exec`, {
    command,
  })

export const installApk = (
  serial: string,
  apkPath: string,
): Promise<{ message: string }> =>
  request.post<{ message: string }>(`/devices/${encodeURIComponent(serial)}/install`, {
    apkPath,
  })
