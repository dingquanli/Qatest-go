<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { cn } from '@/lib/utils'
import {
  LayoutDashboard,
  Send,
  FileCheck,
  Bot,
  Settings,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  Zap,
  Bell,
  Search,
  User,
  Bug,
  ClipboardCheck,
  BookOpen,
  Wifi,
  Server,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  LogOut,
  FileSpreadsheet,
  Network,
  BarChart3,
  Radio,
  Webhook,
  type LucideIcon,
} from 'lucide-vue-next'
import { useUserStore } from '@/stores/user'
import { useAppSettings, defaultSettings, type AppSettings } from '@/composables/useAppSettings'
import * as casesApi from '@/api/cases'
import * as bugsApi from '@/api/bugs'
import * as plansApi from '@/api/plans'
import { getExecutions } from '@/api/executions'

interface SubItem {
  path: string
  label: string
  desc: string
  icon: LucideIcon
}
interface NavEntry {
  path: string
  label: string
  icon: LucideIcon
  desc: string
  children?: SubItem[]
}

const navItems: NavEntry[] = [
  { path: '/dashboard', label: '工作台', icon: LayoutDashboard, desc: '系统总览' },
  {
    path: '/testplan',
    label: '测试计划',
    icon: ClipboardCheck,
    desc: '计划执行',
    children: [
      { path: '/testplan', label: '计划管理', desc: '测试计划', icon: ClipboardCheck },
      { path: '/plan-execs', label: '执行记录', desc: '执行历史', icon: BarChart3 },
    ],
  },
  {
    path: '/cases',
    label: '测试用例',
    icon: FileCheck,
    desc: '测试用例',
    children: [
      { path: '/cases', label: '用例列表', desc: '用例列表', icon: FileCheck },
      { path: '/table-cases', label: '表格用例', desc: '表格视图', icon: FileSpreadsheet },
      { path: '/xmind-cases', label: '思维导图用例', desc: '思维导图用例', icon: Network },
    ],
  },
  {
    path: '/api',
    label: '接口平台',
    icon: Send,
    desc: 'API 测试',
    children: [
      { path: '/api-defs', label: '接口定义', desc: 'API 规范', icon: BookOpen },
      { path: '/api-test', label: '接口测试', desc: 'API 调试', icon: Send },
      { path: '/proxy-interceptor', label: '代理拦截', desc: '实时流量控制', icon: Wifi },
      { path: '/protocol-recorder', label: '协议录制', desc: '请求录制与回放', icon: Radio },
      { path: '/sdk-reports', label: 'SDK 上报', desc: '上报查看', icon: Webhook },
    ],
  },
  { path: '/automation', label: '自动化平台', icon: Bot, desc: '自动执行' },
  { path: '/bugs', label: '缺陷管理', icon: Bug, desc: '缺陷跟踪' },
  { path: '/settings', label: '系统设置', icon: Settings, desc: '环境与配置' },
]

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const appSettings = useAppSettings()

const collapsed = ref(false)
const settings = ref<AppSettings>(defaultSettings())
const activeEnv = computed(() => settings.value.environments.find((e) => e.id === settings.value.activeEnvId))

const currentPage = computed(() => {
  const found = navItems.find((item) => {
    if (route.path.startsWith(item.path) && item.path !== '/') return true
    if (item.children?.some((c) => route.path.startsWith(c.path))) return true
    return false
  })
  return found?.path.slice(1) || 'dashboard'
})
const apiExpanded = computed(() => currentPage.value === 'api')
const testplanExpanded = computed(() => currentPage.value === 'testplan')
const casesExpanded = computed(() => currentPage.value === 'cases')

function isItemActive(item: NavEntry): boolean {
  if (item.path === '/api') return route.path.startsWith('/api')
  if (currentPage.value === item.path.slice(1)) return true
  return !!item.children?.some((c) => route.path === c.path)
}

async function loadSettings() {
  try {
    settings.value = await appSettings.load()
  } catch {
    settings.value = defaultSettings()
  }
}
async function persist() {
  try {
    await appSettings.save(settings.value)
  } catch {
    /* ignore */
  }
}
function setEnv(id: string) {
  settings.value.activeEnvId = id
  envOpen.value = false
  persist()
}
function setNet(mode: 'intranet' | 'extranet') {
  settings.value.network.mode = mode
  netOpen.value = false
  persist()
}

onMounted(loadSettings)

/* ===================== 命令面板 ===================== */
const paletteOpen = ref(false)
const paletteQuery = ref('')
const debouncedQuery = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)
let debounceTimer: number | undefined

const flatItems = navItems.flatMap((item) =>
  item.children
    ? [{ path: item.path, label: item.label, icon: item.icon, desc: item.desc }, ...item.children.map((c) => ({ path: c.path, label: c.label, icon: c.icon, desc: c.desc }))]
    : [{ path: item.path, label: item.label, icon: item.icon, desc: item.desc }],
)
const filteredItems = computed(() =>
  paletteQuery.value
    ? flatItems.filter((i) => i.label.includes(paletteQuery.value) || i.desc.includes(paletteQuery.value))
    : flatItems,
)

interface EntityResult {
  id: string
  type: string
  label: string
  desc: string
  path: string
  icon: LucideIcon
}
const entityResults = ref<EntityResult[]>([])
const typeIconMap: Record<string, LucideIcon> = {
  api: Send,
  case: FileCheck,
  bug: Bug,
  plan: ClipboardCheck,
}
const typeLabelMap: Record<string, string> = {
  api: '接口',
  case: '用例',
  bug: '缺陷',
  plan: '计划',
}

async function runEntitySearch(q: string) {
  if (!q.trim()) {
    entityResults.value = []
    return
  }
  try {
    const [cs, bg, pl] = await Promise.allSettled([
      casesApi.getCases(),
      bugsApi.getBugs(),
      plansApi.getPlanExecutions(),
    ])
    const out: EntityResult[] = []
    if (cs.status === 'fulfilled') {
      for (const c of cs.value.slice(0, 30)) {
        const name = (c as any).name || (c as any).title || ''
        if (name.includes(q)) out.push({ id: (c as any).id, type: 'case', label: name, desc: '测试用例', path: '/cases', icon: FileCheck })
      }
    }
    if (bg.status === 'fulfilled') {
      for (const b of bg.value.slice(0, 30)) {
        const name = (b as any).title || (b as any).name || ''
        if (name.includes(q)) out.push({ id: (b as any).id, type: 'bug', label: name, desc: '缺陷', path: '/bugs', icon: Bug })
      }
    }
    if (pl.status === 'fulfilled') {
      for (const p of pl.value.slice(0, 30)) {
        const name = (p as any).name || (p as any).title || ''
        if (name.includes(q)) out.push({ id: (p as any).id, type: 'plan', label: name, desc: '计划执行', path: '/plan-execs', icon: ClipboardCheck })
      }
    }
    entityResults.value = out.slice(0, 20)
  } catch {
    entityResults.value = []
  }
}
const showGlobalSearch = computed(() => debouncedQuery.value.trim().length > 0)

watch(paletteQuery, (v) => {
  window.clearTimeout(debounceTimer)
  debounceTimer = window.setTimeout(() => {
    debouncedQuery.value = v
    runEntitySearch(v)
  }, 200)
})

function openPalette() {
  paletteOpen.value = true
  setTimeout(() => searchInputRef.value?.focus(), 50)
}
function closePalette() {
  paletteOpen.value = false
  paletteQuery.value = ''
  debouncedQuery.value = ''
  entityResults.value = []
}
function selectPage(path: string) {
  router.push(path)
  closePalette()
}

const isMac = computed(() => navigator.platform.toLowerCase().includes('mac'))

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    paletteOpen.value = !paletteOpen.value
    if (paletteOpen.value) openPalette()
  }
  if (e.key === 'Escape') closePalette()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

/* ===================== 通知 ===================== */
const notifOpen = ref(false)
const notifItems = ref<{ id: string; type: string; label: string; result: string; time: string; executor: string }[]>([])
const failedCount = computed(() => notifItems.value.filter((i) => i.result === 'failed').length)

async function loadNotifications() {
  try {
    const [ex, pl] = await Promise.allSettled([getExecutions(), plansApi.getPlanExecutions()])
    const items: typeof notifItems.value = []
    if (ex.status === 'fulfilled') {
      for (const e of ex.value.slice(0, 8)) {
        items.push({ id: e.id, type: 'case', label: (e as any).caseName || '执行', result: (e as any).result || '', time: (e as any).executedAt || (e as any).createdAt || '', executor: (e as any).executor || '—' })
      }
    }
    if (pl.status === 'fulfilled') {
      for (const t of pl.value.slice(0, 4)) {
        items.push({ id: t.id, type: 'plan', label: (t as any).name || '计划', result: (t as any).status || '', time: (t as any).finishedAt || (t as any).startedAt || '', executor: (t as any).triggerBy || '—' })
      }
    }
    items.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())
    notifItems.value = items.slice(0, 10)
  } catch {
    notifItems.value = []
  }
}
onMounted(loadNotifications)

/* ===================== 下拉菜单 ===================== */
const envOpen = ref(false)
const netOpen = ref(false)
const userMenuOpen = ref(false)

/* ===================== 用户菜单 ===================== */
function logout() {
  userStore.logout()
  router.push('/login')
}
const displayName = computed(() => userStore.user?.name || userStore.user?.username || '用户')

function formatTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/* 全局点击关闭下拉 */
function globalClose(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('[data-dropdown="env"]')) envOpen.value = false
  if (!target.closest('[data-dropdown="net"]')) netOpen.value = false
  if (!target.closest('[data-dropdown="user"]')) userMenuOpen.value = false
  if (!target.closest('[data-dropdown="notif"]')) notifOpen.value = false
}
onMounted(() => document.addEventListener('mousedown', globalClose))
onBeforeUnmount(() => document.removeEventListener('mousedown', globalClose))
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-background">
    <!-- 侧边栏 -->
    <aside
      :class="cn('sidebar-transition flex flex-col border-r bg-white/80 dark:bg-gray-950/80 backdrop-blur-xl relative z-20', collapsed ? 'w-16' : 'w-56')"
    >
      <!-- Logo -->
      <div :class="cn('flex items-center h-14 px-4', collapsed ? 'justify-center' : 'gap-2.5')">
        <RouterLink to="/dashboard" class="flex items-center justify-center w-7 h-7 rounded-xl bg-primary text-primary-foreground font-bold text-xs shrink-0">
          <Zap class="w-3.5 h-3.5" />
        </RouterLink>
        <div v-if="!collapsed" class="overflow-hidden">
          <h1 class="text-base font-semibold tracking-tight truncate">Qatest</h1>
          <p class="text-xs text-muted-foreground truncate leading-tight">全流程测试平台</p>
        </div>
      </div>

      <!-- 导航 -->
      <nav class="flex-1 py-2 px-2 space-y-1 overflow-y-auto">
        <template v-for="item in navItems" :key="item.path">
          <div class="space-y-0.5">
            <RouterLink
              :to="item.children ? item.children[0].path : item.path"
              :title="collapsed ? item.label : undefined"
              :class="cn(
                'flex items-center w-full rounded-xl text-sm font-medium transition-all duration-200',
                collapsed ? 'justify-center h-10 w-10 mx-auto' : 'gap-2.5 px-3 py-2',
                isItemActive(item)
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              )"
            >
              <component :is="item.icon" :class="cn('shrink-0', collapsed ? 'w-5 h-5' : 'w-4 h-4')" />
              <div v-if="!collapsed" class="flex-1 text-left flex items-center justify-between min-w-0">
                <span class="truncate text-sm">{{ item.label }}</span>
                <ChevronDown
                  v-if="item.children"
                  :class="cn('w-3 h-3 text-muted-foreground/40 transition-transform shrink-0 ml-1',
                    (item.path === '/api' && apiExpanded) ||
                    (item.path === '/testplan' && testplanExpanded) ||
                    (item.path === '/cases' && casesExpanded) ? 'rotate-0' : '-rotate-90')"
                />
              </div>
            </RouterLink>
            <template v-if="!collapsed && item.children">
              <div
                v-if="(item.path === '/api' && apiExpanded) || (item.path === '/testplan' && testplanExpanded) || (item.path === '/cases' && casesExpanded)"
                class="ml-2.5 border-l border-border/40 pl-1.5 space-y-0.5"
              >
                <RouterLink
                  v-for="child in item.children"
                  :key="child.path"
                  :to="child.path"
                  :class="cn(
                    'flex items-center w-full rounded-lg text-xs font-medium transition-all duration-200 gap-2.5 px-3 py-2',
                    route.path === child.path
                      ? 'bg-primary/10 text-primary'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  )"
                >
                  <component :is="child.icon" class="w-4 h-4 shrink-0" />
                  <span class="text-left block truncate">{{ child.label }}</span>
                </RouterLink>
              </div>
            </template>
          </div>
        </template>
      </nav>

      <!-- 折叠按钮 -->
      <button
        @click="collapsed = !collapsed"
        class="absolute -right-3 top-16 w-6 h-6 rounded-full border bg-white dark:bg-gray-900 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors shadow-sm"
      >
        <ChevronRight v-if="collapsed" class="w-3 h-3" />
        <ChevronLeft v-else class="w-3 h-3" />
      </button>
    </aside>

    <!-- 主内容区 -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <!-- 顶部栏 -->
      <header class="h-14 border-b bg-white/80 dark:bg-gray-950/80 backdrop-blur-xl flex items-center justify-between px-5 shrink-0">
        <div class="flex items-center gap-3">
          <div class="relative">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
            <button
              @click="openPalette"
              class="flex items-center h-9 w-72 rounded-xl border border-input bg-muted/30 px-3 pl-9 text-sm text-muted-foreground text-left hover:bg-muted/50 transition-colors"
            >
              搜索页面...
              <kbd class="ml-auto text-[10px] text-muted-foreground/50 border border-border rounded px-1.5 py-0.5 hidden sm:inline">{{ isMac ? '⌘K' : 'Ctrl+K' }}</kbd>
            </button>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <!-- 网络模式选择器 -->
          <div v-if="settings.network" data-dropdown="net" class="relative">
            <button @click="netOpen = !netOpen; envOpen = false">
              <span :class="cn('inline-flex items-center rounded-full border border-border px-2.5 h-7 text-[11px] gap-1.5 cursor-pointer hover:bg-accent/50 transition-colors')">
                <Server v-if="settings.network.mode === 'intranet'" class="w-3 h-3" />
                <Wifi v-else class="w-3 h-3" />
                {{ settings.network.mode === 'intranet' ? '内网' : '外网' }}
                <ChevronDown class="w-3 h-3" />
              </span>
            </button>
            <div v-if="netOpen" class="absolute right-0 top-full mt-1.5 w-44 bg-popover border rounded-xl shadow-lg z-50 animate-scale-in overflow-hidden">
              <div class="px-3 py-2 border-b"><p class="text-xs font-semibold">网络切换</p></div>
              <button
                :class="cn('w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent/50 transition-colors', settings.network.mode === 'intranet' ? 'bg-primary/5 text-primary font-medium' : '')"
                @click="setNet('intranet')"
              >
                <Server class="w-4 h-4" /> 内网
                <CheckCircle2 v-if="settings.network.mode === 'intranet'" class="w-3.5 h-3.5 ml-auto text-primary" />
              </button>
              <button
                :class="cn('w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent/50 transition-colors', settings.network.mode === 'extranet' ? 'bg-primary/5 text-primary font-medium' : '')"
                @click="setNet('extranet')"
              >
                <Wifi class="w-4 h-4" /> 外网
                <CheckCircle2 v-if="settings.network.mode === 'extranet'" class="w-3.5 h-3.5 ml-auto text-primary" />
              </button>
            </div>
          </div>
          <!-- 环境选择器 -->
          <div v-if="activeEnv" data-dropdown="env" class="relative">
            <button @click="envOpen = !envOpen; netOpen = false">
              <span :class="cn('inline-flex items-center rounded-full border border-border px-2.5 h-7 text-[11px] gap-1.5 cursor-pointer hover:bg-accent/50 transition-colors')">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                {{ activeEnv.name }}
                <ChevronDown class="w-3 h-3" />
              </span>
            </button>
            <div v-if="envOpen" class="absolute right-0 top-full mt-1.5 w-56 bg-popover border rounded-xl shadow-lg z-50 animate-scale-in overflow-hidden">
              <div class="px-3 py-2 border-b"><p class="text-xs font-semibold">环境管理</p></div>
              <button
                v-for="env in settings.environments"
                :key="env.id"
                :class="cn('w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent/50 transition-colors', env.id === settings.activeEnvId ? 'bg-primary/5 text-primary font-medium' : '')"
                @click="setEnv(env.id)"
              >
                <span :class="cn('w-2 h-2 rounded-full', env.id === settings.activeEnvId ? 'bg-emerald-500' : 'bg-muted-foreground/30')" />
                <span class="truncate">{{ env.name }}</span>
                <span class="ml-auto text-[10px] text-muted-foreground truncate max-w-[100px]">{{ env.baseUrl || '—' }}</span>
                <CheckCircle2 v-if="env.id === settings.activeEnvId" class="w-3.5 h-3.5 text-primary shrink-0" />
              </button>
            </div>
          </div>
          <!-- 通知 -->
          <div data-dropdown="notif" class="relative">
            <button class="relative h-8 w-8 rounded-xl hover:bg-accent transition-colors flex items-center justify-center" @click="notifOpen = !notifOpen">
              <Bell class="w-4 h-4" />
              <span v-if="failedCount > 0" class="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-destructive"></span>
            </button>
            <div v-if="notifOpen" class="absolute right-0 top-full mt-1.5 w-96 bg-popover border rounded-2xl shadow-lg z-50 animate-scale-in overflow-hidden">
              <div class="px-4 py-3 border-b"><p class="text-sm font-semibold">最近动态</p></div>
              <div class="max-h-80 overflow-y-auto">
                <div v-if="notifItems.length === 0" class="px-4 py-8 text-center text-sm text-muted-foreground">暂无动态</div>
                <div
                  v-for="item in notifItems"
                  :key="item.id"
                  class="flex items-center gap-3 px-4 py-3 hover:bg-accent/50 transition-colors border-b last:border-0"
                >
                  <CheckCircle2 v-if="item.result === 'passed' || item.result === 'success'" class="w-4 h-4 text-emerald-500 shrink-0" />
                  <XCircle v-else-if="item.result === 'failed'" class="w-4 h-4 text-red-500 shrink-0" />
                  <AlertTriangle v-else class="w-4 h-4 text-amber-500 shrink-0" />
                  <div class="min-w-0 flex-1">
                    <p class="text-xs font-medium truncate">{{ item.label }}</p>
                    <p class="text-xs text-muted-foreground">{{ item.executor }} · {{ formatTime(item.time) }}</p>
                  </div>
                </div>
              </div>
              <button @click="notifOpen = false; router.push('/cases')" class="w-full px-4 py-2.5 text-xs text-center text-primary hover:bg-accent/50 transition-colors border-t font-medium">
                查看全部执行记录
              </button>
            </div>
          </div>
          <div class="w-px h-5 bg-border/50" />
          <!-- 用户菜单 -->
          <div data-dropdown="user" class="relative">
            <button class="flex items-center gap-2 cursor-pointer hover:bg-accent rounded-xl px-2.5 py-1.5 transition-colors" @click="userMenuOpen = !userMenuOpen">
              <div class="w-7 h-7 rounded-full bg-primary/10 flex items-center justify-center">
                <User class="w-3.5 h-3.5 text-primary" />
              </div>
              <span class="text-sm font-medium hidden sm:block">{{ displayName }}</span>
              <ChevronDown class="w-3 h-3 text-muted-foreground/50 hidden sm:block" />
            </button>
            <div v-if="userMenuOpen" class="absolute right-0 top-full mt-1.5 w-44 bg-popover border rounded-2xl shadow-lg z-50 animate-scale-in overflow-hidden">
              <div class="px-4 py-3 border-b">
                <p class="text-sm font-medium">{{ displayName }}</p>
                <p class="text-xs text-muted-foreground">{{ userStore.user?.username || '' }}@qatest</p>
              </div>
              <div class="border-t">
                <button @click="logout" class="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-red-500 hover:bg-accent transition-colors">
                  <LogOut class="w-4 h-4" /> 退出登录
                </button>
              </div>
            </div>
          </div>
        </div>
      </header>

      <!-- 页面内容 -->
      <main class="flex-1 overflow-auto p-6">
        <RouterView />
      </main>
    </div>

    <!-- 命令面板 -->
    <div v-if="paletteOpen" class="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
      <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" @click="closePalette" />
      <div class="relative w-full max-w-lg bg-popover border rounded-2xl shadow-2xl animate-scale-in overflow-hidden">
        <div class="flex items-center gap-3 px-4 border-b">
          <Search class="w-4 h-4 text-muted-foreground shrink-0" />
          <input
            ref="searchInputRef"
            v-model="paletteQuery"
            placeholder="搜索接口、用例、缺陷、页面..."
            class="flex-1 h-9 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
          <kbd class="text-xs text-muted-foreground/50 border border-border rounded px-1.5 py-0.5">ESC</kbd>
        </div>
        <div class="max-h-80 overflow-y-auto py-2">
          <template v-if="showGlobalSearch">
            <div v-if="entityResults.length === 0" class="px-4 py-8 text-center text-sm text-muted-foreground">未找到匹配内容</div>
            <button
              v-for="item in entityResults"
              :key="item.id"
              @click="selectPage(item.path)"
              class="flex items-center gap-3 w-full px-4 py-2.5 text-sm hover:bg-accent transition-colors text-left"
            >
              <component :is="item.icon" class="w-4 h-4 text-muted-foreground shrink-0" />
              <div class="min-w-0 flex-1">
                <p class="font-medium truncate">{{ item.label }}</p>
                <p class="text-xs text-muted-foreground truncate">{{ item.desc }}</p>
              </div>
              <span class="inline-flex items-center rounded-full border border-border px-2.5 py-0.5 text-[10px] uppercase shrink-0">{{ typeLabelMap[item.type] }}</span>
            </button>
          </template>
          <template v-else>
            <div v-if="filteredItems.length === 0" class="px-4 py-8 text-center text-sm text-muted-foreground">无匹配结果</div>
            <button
              v-for="item in filteredItems"
              :key="item.path"
              @click="selectPage(item.path)"
              class="flex items-center gap-3 w-full px-4 py-2.5 text-sm hover:bg-accent transition-colors text-left"
            >
              <component :is="item.icon" class="w-4 h-4 text-muted-foreground shrink-0" />
              <div class="min-w-0">
                <p class="font-medium truncate">{{ item.label }}</p>
                <p class="text-xs text-muted-foreground truncate">{{ item.desc }}</p>
              </div>
            </button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
