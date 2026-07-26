import * as settingsApi from '@/api/settings'
import type { ThemeMode } from '@/composables/useTheme'

export interface EnvVariable {
  id: string
  key: string
  value: string
  enabled: boolean
}
export interface Environment {
  id: string
  name: string
  baseUrl: string
  variables: EnvVariable[]
}
export interface Notifications {
  taskFailed: boolean
  apiError: boolean
  taskStart: boolean
}
export interface NetworkConfig {
  mode: 'intranet' | 'extranet'
  intranetUrl: string
  extranetUrl: string
}
export interface JiraSync {
  enabled: boolean
  baseUrl: string
  project: string
  username: string
  apiToken: string
}
export interface WebhookSync {
  enabled: boolean
  webhookUrl: string
}
export interface BugSync {
  jira: JiraSync
  feishu: WebhookSync
  wecom: WebhookSync
}
export interface AppSettings {
  theme: ThemeMode
  defaultTimeout: number
  environments: Environment[]
  activeEnvId: string
  notifications: Notifications
  network: NetworkConfig
  bugSync: BugSync
}

export function uid(prefix = 'id'): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
}

export function defaultSettings(): AppSettings {
  const envId = 'env-default'
  return {
    theme: 'light',
    defaultTimeout: 30000,
    environments: [
      {
        id: envId,
        name: '默认环境',
        baseUrl: '',
        variables: [{ id: uid('v'), key: '', value: '', enabled: true }],
      },
    ],
    activeEnvId: envId,
    notifications: { taskFailed: true, apiError: true, taskStart: false },
    network: { mode: 'intranet', intranetUrl: '', extranetUrl: '' },
    bugSync: {
      jira: { enabled: false, baseUrl: '', project: '', username: '', apiToken: '' },
      feishu: { enabled: false, webhookUrl: '' },
      wecom: { enabled: false, webhookUrl: '' },
    },
  }
}

/** 结构化设置 → 后端扁平 map（复杂字段以 JSON 字符串存储） */
function serialize(s: AppSettings): Record<string, string> {
  return {
    theme: s.theme,
    defaultTimeout: String(s.defaultTimeout),
    activeEnvId: s.activeEnvId,
    environments: JSON.stringify(s.environments),
    notifications: JSON.stringify(s.notifications),
    network: JSON.stringify(s.network),
    bugSync: JSON.stringify(s.bugSync),
  }
}

/** 后端扁平 map → 结构化设置（失败回退默认） */
function deserialize(raw: Record<string, string>): AppSettings {
  const d = defaultSettings()
  if (!raw || Object.keys(raw).length === 0) return d
  try {
    return {
      theme: (['light', 'dark', 'system'].includes(raw.theme) ? raw.theme : d.theme) as AppSettings['theme'],
      defaultTimeout: raw.defaultTimeout ? parseInt(raw.defaultTimeout, 10) || d.defaultTimeout : d.defaultTimeout,
      environments: raw.environments ? JSON.parse(raw.environments) : d.environments,
      activeEnvId: raw.activeEnvId || d.activeEnvId,
      notifications: raw.notifications ? JSON.parse(raw.notifications) : d.notifications,
      network: raw.network ? JSON.parse(raw.network) : d.network,
      bugSync: raw.bugSync ? JSON.parse(raw.bugSync) : d.bugSync,
    }
  } catch {
    return d
  }
}

export function useAppSettings() {
  async function load(): Promise<AppSettings> {
    const raw = (await settingsApi.getSettings()) as unknown as Record<string, string>
    return deserialize(raw)
  }
  async function save(s: AppSettings): Promise<void> {
    await settingsApi.updateSettings(serialize(s) as unknown as Record<string, unknown>)
  }
  return { load, save, defaultSettings, uid }
}
