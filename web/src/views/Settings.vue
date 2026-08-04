<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Globe,
  Bell,
  Palette,
  Sun,
  Moon,
  Monitor,
  Trash2,
  Plus,
  Clock,
  Info,
  Network,
  Wifi,
  Server,
  Bug,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Badge from '@/components/ui/Badge.vue'
import { useTheme, type ThemeMode } from '@/composables/useTheme'
import { cn } from '@/lib/utils'
import { useAppSettings, defaultSettings, type AppSettings, type Environment } from '@/composables/useAppSettings'
import { getSetting, updateSetting } from '@/api/settings'
import { getJiraStatus } from '@/api/bugs'

// 与后端 handlers/settings.go 中的 maskedSecretValue 保持一致
const MASKED = '********'

const { theme, setTheme } = useTheme()
const appSettings = useAppSettings()
const settings = ref<AppSettings>(defaultSettings())
const userAgent = typeof navigator !== 'undefined' ? navigator.userAgent : ''

async function load() {
  try {
    settings.value = await appSettings.load()
  } catch {
    settings.value = defaultSettings()
  }
}
async function save() {
  try {
    await appSettings.save(settings.value)
  } catch {
    /* ignore */
  }
}
function update(partial: Partial<AppSettings>) {
  settings.value = { ...settings.value, ...partial }
  save()
}
function changeTheme(t: ThemeMode) {
  setTheme(t)
  update({ theme: t })
}

function uid(p = 'id') {
  return `${p}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
}

function addEnv() {
  update({
    environments: [
      ...settings.value.environments,
      { id: uid('env'), name: '新环境', baseUrl: '', variables: [{ id: uid('v'), key: '', value: '', enabled: true }] },
    ],
  })
}
function removeEnv(id: string) {
  const list = settings.value.environments.filter((e) => e.id !== id)
  update({ environments: list, activeEnvId: settings.value.activeEnvId === id ? list[0]?.id ?? '' : settings.value.activeEnvId })
}
function updateEnv(id: string, partial: Partial<Environment>) {
  update({ environments: settings.value.environments.map((e) => (e.id === id ? { ...e, ...partial } : e)) })
}
function updateEnvVar(envId: string, varId: string, field: 'key' | 'value' | 'enabled', value: string | boolean) {
  update({
    environments: settings.value.environments.map((e) =>
      e.id === envId
        ? { ...e, variables: e.variables.map((v) => (v.id === varId ? { ...v, [field]: value } : v)) }
        : e,
    ),
  })
}
function addEnvVar(envId: string) {
  update({
    environments: settings.value.environments.map((e) =>
      e.id === envId ? { ...e, variables: [...e.variables, { id: uid('v'), key: '', value: '', enabled: true }] } : e,
    ),
  })
}
function removeEnvVar(envId: string, varId: string) {
  update({
    environments: settings.value.environments.map((e) =>
      e.id === envId ? { ...e, variables: e.variables.filter((v) => v.id !== varId) } : e,
    ),
  })
}

// Jira 同步配置（持久化到服务端 settings 表，key 前缀 jira_）
const jiraBaseUrl = ref('')
const jiraProject = ref('')
const jiraEmail = ref('')
const jiraToken = ref('')
// 服务端已配置状态（来自 /config/jira/status，避免 token 脱敏后丢失"已配置"判断）
const jiraServerConfigured = ref(false)
const jiraConfigured = computed(
  () => !!(jiraBaseUrl.value && jiraEmail.value && (jiraToken.value || jiraServerConfigured.value) && jiraProject.value),
)
async function loadJira(): Promise<void> {
  const map: Record<string, ReturnType<typeof ref<string>>> = {
    jira_base_url: jiraBaseUrl,
    jira_project: jiraProject,
    jira_email: jiraEmail,
    jira_api_token: jiraToken,
  }
  for (const [k, r] of Object.entries(map)) {
    try {
      const v = await getSetting<{ key: string; value: string }>(k)
      if (v && v.value) {
        // 服务端对敏感字段返回脱敏占位符，切勿写回表单，避免覆盖真实值
        if (k === 'jira_api_token' && v.value === MASKED) continue
        r.value = v.value
      }
    } catch {
      /* 未配置则留空 */
    }
  }
  // 同步服务端真实配置状态（token 不回显，用 /status 判断已配置）
  try {
    const s = await getJiraStatus()
    jiraServerConfigured.value = !!s?.configured
  } catch {
    /* ignore */
  }
}
function saveJiraKey(suffix: string, value: string): void {
  // 拒绝写回脱敏占位符，防止覆盖真实 token
  if (value === MASKED) return
  updateSetting(`jira_${suffix}`, value).catch(() => {})
}

const themeOptions = [
  { value: 'light' as const, label: '浅色模式', icon: Sun, desc: '明亮简洁' },
  { value: 'dark' as const, label: '深色模式', icon: Moon, desc: '护眼舒适' },
  { value: 'system' as const, label: '跟随系统', icon: Monitor, desc: '自动切换' },
]
const notifyItems = [
  { key: 'taskFailed' as const, label: '任务执行失败', desc: '自动化任务执行失败时发送通知' },
  { key: 'apiError' as const, label: '接口异常告警', desc: '接口连续失败或响应超时时告警' },
  { key: 'taskStart' as const, label: '定时任务启动', desc: '定时任务开始执行时通知' },
]

onMounted(() => {
  load()
  loadJira()
})
</script>

<template>
  <div class="space-y-6 max-w-4xl">
    <div>
      <h1 class="text-2xl font-bold tracking-tight">系统设置</h1>
      <p class="text-muted-foreground mt-1">配置环境变量、主题和通知偏好</p>
    </div>

    <!-- 主题设置 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Palette class="w-4 h-4" /> 主题设置</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-3 gap-3">
          <button
            v-for="opt in themeOptions"
            :key="opt.value"
            @click="changeTheme(opt.value)"
            :class="cn('p-4 rounded-xl border-2 transition-all text-left', theme === opt.value ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/30')"
          >
            <component :is="opt.icon" :class="cn('w-6 h-6 mb-2', theme === opt.value ? 'text-primary' : 'text-muted-foreground')" />
            <p class="text-sm font-medium">{{ opt.label }}</p>
            <p class="text-xs text-muted-foreground mt-0.5">{{ opt.desc }}</p>
          </button>
        </div>
      </CardContent>
    </Card>

    <!-- 环境管理 -->
    <Card>
      <CardHeader>
        <div class="flex items-center justify-between">
          <div>
            <CardTitle class="text-base flex items-center gap-2"><Globe class="w-4 h-4" /> 环境管理</CardTitle>
            <CardDescription>管理不同环境的 Base URL 和变量，接口测试中用 <span v-pre>{{变量名}}</span> 引用</CardDescription>
          </div>
          <Button variant="outline" size="sm" class="gap-1" @click="addEnv">
            <Plus class="w-3 h-3" /> 添加环境
          </Button>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div
          v-for="env in settings.environments"
          :key="env.id"
          :class="cn('p-4 rounded-xl border-2 transition-all', settings.activeEnvId === env.id ? 'border-primary/40 bg-primary/5' : 'border-border')"
        >
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <button
                @click="update({ activeEnvId: env.id })"
                :class="cn('w-3 h-3 rounded-full border-2', settings.activeEnvId === env.id ? 'bg-primary border-primary' : 'border-muted-foreground/30')"
                title="设为当前环境"
              />
              <Input v-model="env.name" @update:model-value="(v: string) => updateEnv(env.id, { name: v })" class="h-8 w-40 font-medium" />
              <Badge v-if="settings.activeEnvId === env.id" variant="success" class="text-xs">当前</Badge>
            </div>
            <Button variant="ghost" size="icon" class="h-8 w-8" @click="removeEnv(env.id)">
              <Trash2 class="w-3 h-3 text-red-500" />
            </Button>
          </div>
          <div class="mb-3">
            <label class="text-xs text-muted-foreground mb-1 block">Base URL</label>
            <Input
              :model-value="env.baseUrl"
              @update:model-value="(v: string) => updateEnv(env.id, { baseUrl: v })"
              placeholder="https://api.example.com"
              class="h-8 font-mono text-sm"
            />
          </div>
          <div>
            <label class="text-xs text-muted-foreground mb-1 block">变量</label>
            <div class="space-y-1.5">
              <div v-for="v in env.variables" :key="v.id" class="flex items-center gap-2">
                <input
                  type="checkbox"
                  :checked="v.enabled"
                  @change="updateEnvVar(env.id, v.id, 'enabled', ($event.target as HTMLInputElement).checked)"
                  class="w-3.5 h-3.5 accent-primary"
                />
                <Input :model-value="v.key" @update:model-value="(val: string) => updateEnvVar(env.id, v.id, 'key', val)" placeholder="变量名" class="h-8 w-36 font-mono text-xs" />
                <span class="text-muted-foreground">=</span>
                <Input :model-value="v.value" @update:model-value="(val: string) => updateEnvVar(env.id, v.id, 'value', val)" placeholder="变量值" class="h-8 flex-1 font-mono text-xs" />
                <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeEnvVar(env.id, v.id)">
                  <Trash2 class="w-3 h-3 text-red-400" />
                </Button>
              </div>
              <Button variant="ghost" size="sm" class="gap-1 text-xs text-muted-foreground" @click="addEnvVar(env.id)">
                <Plus class="w-3 h-3" /> 添加变量
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- 通知设置 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Bell class="w-4 h-4" /> 通知设置</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <div v-for="item in notifyItems" :key="item.key" class="flex items-center justify-between py-2">
          <div>
            <p class="text-sm font-medium">{{ item.label }}</p>
            <p class="text-xs text-muted-foreground">{{ item.desc }}</p>
          </div>
          <button
            @click="update({ notifications: { ...settings.notifications, [item.key]: !settings.notifications[item.key] } })"
            :class="cn('relative inline-flex h-6 w-11 items-center rounded-full transition-colors', settings.notifications[item.key] ? 'bg-primary' : 'bg-muted')"
          >
            <span :class="cn('inline-block h-4 w-4 rounded-full bg-white transition-transform', settings.notifications[item.key] ? 'translate-x-6' : 'translate-x-1')" />
          </button>
        </div>
      </CardContent>
    </Card>

    <!-- 超时设置 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Clock class="w-4 h-4" /> 超时设置</CardTitle>
        <CardDescription>接口请求和任务执行的默认超时时间</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="flex items-center gap-3">
          <Input
            type="number"
            min="1000"
            max="300000"
            step="1000"
            :model-value="settings.defaultTimeout"
            @update:model-value="(v: string) => update({ defaultTimeout: Math.max(1000, parseInt(v) || 30000) })"
            class="h-9 w-32 font-mono text-sm"
          />
          <span class="text-sm text-muted-foreground">毫秒（{{ settings.defaultTimeout / 1000 }} 秒）</span>
        </div>
      </CardContent>
    </Card>

    <!-- 网络切换 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Network class="w-4 h-4" /> 网络切换</CardTitle>
        <CardDescription>切换内网/外网访问地址，当前模式会自动应用到平台 API 调用</CardDescription>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex items-center gap-3 p-3 rounded-xl bg-muted/50">
          <div
            :class="cn('flex items-center gap-2 px-4 py-2 rounded-lg cursor-pointer transition-all', settings.network.mode === 'intranet' ? 'bg-primary text-primary-foreground shadow-sm' : 'bg-background border hover:border-primary/30')"
            @click="update({ network: { ...settings.network, mode: 'intranet' } })"
          >
            <Server class="w-4 h-4" />
            <span class="text-sm font-medium">内网</span>
          </div>
          <div
            :class="cn('flex items-center gap-2 px-4 py-2 rounded-lg cursor-pointer transition-all', settings.network.mode === 'extranet' ? 'bg-primary text-primary-foreground shadow-sm' : 'bg-background border hover:border-primary/30')"
            @click="update({ network: { ...settings.network, mode: 'extranet' } })"
          >
            <Wifi class="w-4 h-4" />
            <span class="text-sm font-medium">外网</span>
          </div>
          <Badge :variant="settings.network.mode === 'intranet' ? 'default' : 'secondary'" class="ml-auto text-xs">
            当前：{{ settings.network.mode === 'intranet' ? '内网' : '外网' }}
          </Badge>
        </div>
        <div class="space-y-3">
          <div>
            <label class="text-xs text-muted-foreground mb-1.5 block font-medium">内网地址</label>
            <div class="flex items-center gap-2">
              <Server class="w-4 h-4 text-muted-foreground shrink-0" />
              <Input :model-value="settings.network.intranetUrl" @update:model-value="(v: string) => update({ network: { ...settings.network, intranetUrl: v } })" placeholder="http://192.168.1.100:3000" class="h-9 font-mono text-sm" />
            </div>
            <p class="text-xs text-muted-foreground mt-1">局域网内访问后端服务的地址（默认使用当前页面地址）</p>
          </div>
          <div>
            <label class="text-xs text-muted-foreground mb-1.5 block font-medium">外网地址</label>
            <div class="flex items-center gap-2">
              <Wifi class="w-4 h-4 text-muted-foreground shrink-0" />
              <Input :model-value="settings.network.extranetUrl" @update:model-value="(v: string) => update({ network: { ...settings.network, extranetUrl: v } })" placeholder="https://qatest.company.com:3000" class="h-9 font-mono text-sm" />
            </div>
            <p class="text-xs text-muted-foreground mt-1">公网或 VPN 访问后端服务的地址</p>
          </div>
        </div>
        <div class="p-3 rounded-xl bg-amber-50 dark:bg-amber-950/20 border border-amber-200/60 dark:border-amber-900/40">
          <p class="text-xs text-amber-700 dark:text-amber-400">
            <strong>提示：</strong>切换网络模式后，平台自身的 API 调用将使用对应地址作为基准路径。请在部署时分别配置好内网可达和外网可达的地址。
          </p>
        </div>
      </CardContent>
    </Card>

    <!-- 缺陷同步 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Bug class="w-4 h-4" /> 缺陷同步</CardTitle>
        <CardDescription>Jira 同步已可用；飞书、企业微信暂未支持</CardDescription>
      </CardHeader>
      <CardContent class="space-y-6">
        <!-- Jira -->
        <div class="space-y-3">
          <div class="flex items-center justify-between py-2 border-b">
            <div>
              <p class="text-sm font-medium">Jira</p>
              <p class="text-xs text-muted-foreground">同步缺陷到 Jira Issue Tracker（配置保存在服务端）</p>
            </div>
            <Badge :variant="jiraConfigured ? 'success' : 'secondary'" class="text-xs">
              {{ jiraConfigured ? '已配置' : '未配置' }}
            </Badge>
          </div>
          <div class="space-y-3 pl-0">
            <div>
              <label class="text-xs text-muted-foreground mb-1.5 block font-medium">Jira 地址</label>
              <Input v-model="jiraBaseUrl" @update:model-value="(v: string) => saveJiraKey('base_url', v)" placeholder="https://yourcompany.atlassian.net" class="h-9 font-mono text-sm" />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-xs text-muted-foreground mb-1.5 block font-medium">项目 Key</label>
                <Input v-model="jiraProject" @update:model-value="(v: string) => saveJiraKey('project', (v || '').toUpperCase())" placeholder="QA" class="h-9 font-mono text-sm" />
              </div>
              <div>
                <label class="text-xs text-muted-foreground mb-1.5 block font-medium">账号邮箱</label>
                <Input v-model="jiraEmail" @update:model-value="(v: string) => saveJiraKey('email', v)" placeholder="your@email.com" class="h-9 text-sm" />
              </div>
            </div>
            <div>
              <label class="text-xs text-muted-foreground mb-1.5 block font-medium">API Token</label>
              <Input type="password" v-model="jiraToken" @update:model-value="(v: string) => saveJiraKey('api_token', v)" placeholder="从 Jira 账户设置中生成" class="h-9 font-mono text-sm" />
              <p class="text-xs text-muted-foreground mt-1">{{ jiraToken ? '•••••••• 已保存（服务端不回显）' : '在 https://id.atlassian.com/manage-profile/security/api-tokens 生成' }}</p>
            </div>
          </div>
        </div>

        <!-- 飞书 -->
        <div class="space-y-3">
          <div class="flex items-center justify-between py-2 border-b">
            <div>
              <p class="text-sm font-medium">飞书</p>
              <p class="text-xs text-muted-foreground">通过 Webhook 推送缺陷卡片到飞书群</p>
            </div>
            <button
              @click="update({ bugSync: { ...settings.bugSync, feishu: { ...settings.bugSync.feishu, enabled: !settings.bugSync.feishu.enabled } } })"
              :class="cn('relative inline-flex h-6 w-11 items-center rounded-full transition-colors', settings.bugSync.feishu.enabled ? 'bg-primary' : 'bg-muted')"
            >
              <span :class="cn('inline-block h-4 w-4 rounded-full bg-white transition-transform', settings.bugSync.feishu.enabled ? 'translate-x-6' : 'translate-x-1')" />
            </button>
          </div>
          <div v-if="settings.bugSync.feishu.enabled" class="space-y-3 pl-0">
            <div>
              <label class="text-xs text-muted-foreground mb-1.5 block font-medium">Webhook 地址</label>
              <Input :model-value="settings.bugSync.feishu.webhookUrl" @update:model-value="(v: string) => update({ bugSync: { ...settings.bugSync, feishu: { ...settings.bugSync.feishu, webhookUrl: v } } })" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/xxx" class="h-9 font-mono text-sm" />
              <p class="text-xs text-muted-foreground mt-1">在飞书群设置 → 群机器人 → 添加自定义机器人 获取</p>
            </div>
          </div>
        </div>

        <!-- 企微 -->
        <div class="space-y-3">
          <div class="flex items-center justify-between py-2 border-b">
            <div>
              <p class="text-sm font-medium">企业微信</p>
              <p class="text-xs text-muted-foreground">通过 Webhook 推送缺陷消息到企微群</p>
            </div>
            <button
              @click="update({ bugSync: { ...settings.bugSync, wecom: { ...settings.bugSync.wecom, enabled: !settings.bugSync.wecom.enabled } } })"
              :class="cn('relative inline-flex h-6 w-11 items-center rounded-full transition-colors', settings.bugSync.wecom.enabled ? 'bg-primary' : 'bg-muted')"
            >
              <span :class="cn('inline-block h-4 w-4 rounded-full bg-white transition-transform', settings.bugSync.wecom.enabled ? 'translate-x-6' : 'translate-x-1')" />
            </button>
          </div>
          <div v-if="settings.bugSync.wecom.enabled" class="space-y-3 pl-0">
            <div>
              <label class="text-xs text-muted-foreground mb-1.5 block font-medium">Webhook 地址</label>
              <Input :model-value="settings.bugSync.wecom.webhookUrl" @update:model-value="(v: string) => update({ bugSync: { ...settings.bugSync, wecom: { ...settings.bugSync.wecom, webhookUrl: v } } })" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" class="h-9 font-mono text-sm" />
              <p class="text-xs text-muted-foreground mt-1">在企微群设置 → 群机器人 → 添加 webhook 获取</p>
            </div>
          </div>
        </div>

        <div class="p-3 rounded-xl bg-amber-50 dark:bg-amber-950/20 border border-amber-200/60 dark:border-amber-900/40">
          <p class="text-xs text-amber-700 dark:text-amber-400">
            <strong>说明：</strong>Webhook 地址不会上传至服务器，仅在本地发起同步请求时使用。飞书和企微以 Markdown 卡片形式推送缺陷信息。
          </p>
        </div>
      </CardContent>
    </Card>

    <!-- 关于 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2"><Info class="w-4 h-4" /> 关于</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="space-y-1 text-sm">
          <p><span class="text-muted-foreground">应用：</span>Qatest</p>
          <p><span class="text-muted-foreground">版本：</span>1.0.0</p>
          <p><span class="text-muted-foreground">数据存储：</span>服务端数据库 (SQLite)</p>
          <p><span class="text-muted-foreground">环境：</span>{{ userAgent }}</p>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
