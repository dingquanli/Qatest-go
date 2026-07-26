<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Send, Save, Plus, FolderOpen, ChevronDown, ChevronRight, Trash2,
  Clock, Globe, Loader2, X, Search, FileText, Shield, BookOpen, Bug,
  Download, Upload, Copy, Check, Braces,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Badge from '@/components/ui/Badge.vue'
import { cn } from '@/lib/utils'
import { safeParseJSON, formatDate, downloadText, prettyJSON } from '@/utils'
import KeyValueEditor, { type KeyValue } from '@/components/KeyValueEditor.vue'
import JsonViewer from '@/components/JsonViewer.vue'
import ReportBugModal from '@/components/ReportBugModal.vue'
import * as apitestApi from '@/api/apitest'
import type { APIRequest, APIFolder, APIHistory, HttpMethod, ApiTestResult } from '@/types'

/* ===================== Local types ===================== */
type AuthType = 'none' | 'bearer' | 'basic' | 'apikey'
interface AuthConfig {
  type: AuthType
  bearerToken?: string
  basicUser?: string
  basicPass?: string
  apiKeyName?: string
  apiKeyValue?: string
  apiKeyIn?: 'header' | 'query'
}
interface ApiRequestDraft {
  id: string
  name: string
  method: HttpMethod
  url: string
  params: KeyValue[]
  headers: KeyValue[]
  body: string
  bodyType: 'none' | 'json' | 'form' | 'raw'
  auth: AuthConfig
  folderId: string | null
  createdAt: string
  updatedAt: string
}

const METHODS: HttpMethod[] = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']

/* ===================== Helpers ===================== */
function genId(prefix: string): string {
  return prefix + Math.random().toString(36).slice(2, 10)
}
function methodClass(method: string): string {
  return `method-${method.toLowerCase()}`
}
function statusColor(status: number): string {
  if (status >= 200 && status < 300) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  if (status >= 300 && status < 400) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
}
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
function blankRequest(folderId: string | null = null): ApiRequestDraft {
  return {
    id: genId('api-'),
    name: '未命名请求',
    method: 'GET',
    url: '',
    params: [{ key: '', value: '' }],
    headers: [{ key: '', value: '' }],
    body: '',
    bodyType: 'none',
    auth: { type: 'none' },
    folderId,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
}
function kvToRecord(pairs: KeyValue[]): Record<string, string> {
  const out: Record<string, string> = {}
  pairs.forEach((p) => { if (p.key) out[p.key] = p.value })
  return out
}
function parseKV(str: unknown): KeyValue[] {
  const arr = safeParseJSON<{ key?: string; value?: string }[]>(str, [])
  if (arr.length) return arr.map((p) => ({ key: p.key || '', value: p.value || '' }))
  return [{ key: '', value: '' }]
}
function formatJson(body: unknown): string {
  if (typeof body === 'string') {
    try { return JSON.stringify(JSON.parse(body), null, 2) } catch { return body }
  }
  return JSON.stringify(body, null, 2)
}

/* ===================== State ===================== */
const folders = ref<APIFolder[]>([])
const requests = ref<APIRequest[]>([])
const history = ref<APIHistory[]>([])

const expandedFolders = ref<Set<string>>(new Set())
const selectedRequestId = ref<string | null>(null)
const searchTerm = ref('')

const currentRequest = ref<ApiRequestDraft>(blankRequest())
const requestTab = ref<'params' | 'headers' | 'body' | 'auth'>('params')

const response = ref<ApiTestResult | null>(null)
const responseError = ref<string | null>(null)
const responseTab = ref<'body' | 'headers'>('body')
const sending = ref(false)

const showHistory = ref(false)
const methodDropdownOpen = ref(false)
const bugModalReq = ref<APIRequest | null>(null)

const activeEnvName = ref('')
const envVars = ref<{ key: string; value: string }[]>([])

const copiedResponse = ref(false)
const prettyPrint = ref(true)
const historySearch = ref('')
const urlInputRef = ref<HTMLInputElement | null>(null)

const reportVisible = computed<boolean>({
  get: () => !!bugModalReq.value,
  set: (v) => { if (!v) bugModalReq.value = null },
})
const bugModalInitial = computed<Partial<import('@/types').Bug>>(() =>
  bugModalReq.value
    ? {
        title: `[接口] ${bugModalReq.value.name}`,
        description: `${bugModalReq.value.method} ${bugModalReq.value.url}`,
        steps: bugModalReq.value.url,
        module: '接口测试',
      }
    : {},
)

/* ===================== Derived ===================== */
const folderRequests = computed(() =>
  folders.value.map((folder) => ({
    folder,
    requests: requests.value.filter((r) => r.folderId === folder.id),
  })),
)
const ungroupedRequests = computed(() => requests.value.filter((r) => !r.folderId))
const paramsCount = computed(() => currentRequest.value.params.filter((p) => p.key).length)
const headersCount = computed(() => currentRequest.value.headers.filter((p) => p.key).length)
const filteredHistory = computed(() =>
  history.value.filter((entry) => {
    if (!historySearch.value) return true
    const term = historySearch.value.toLowerCase()
    return entry.url.toLowerCase().includes(term) || entry.method.toLowerCase().includes(term)
  }),
)
const isJsonBody = computed(() => {
  const b = response.value?.body
  if (!b || typeof b !== 'string') return false
  try { JSON.parse(b); return true } catch { return false }
})
const responseBodyObj = computed(() => {
  const b = response.value?.body
  if (typeof b === 'string') { try { return JSON.parse(b) } catch { return b } }
  return b
})

function matchesSearch(name: string, url: string): boolean {
  if (!searchTerm.value) return true
  const term = searchTerm.value.toLowerCase()
  return name.toLowerCase().includes(term) || url.toLowerCase().includes(term)
}

/* ===================== Data loading ===================== */
async function loadAll(): Promise<void> {
  const [f, r, h] = await Promise.all([
    apitestApi.getApiFolders(),
    apitestApi.getApiRequests(),
    apitestApi.getApiHistory(),
  ])
  folders.value = f
  requests.value = r
  history.value = h
  expandedFolders.value = new Set(f.map((folder) => folder.id))
}

/* ===================== Request actions ===================== */
function selectRequest(req: APIRequest): void {
  selectedRequestId.value = req.id
  currentRequest.value = {
    id: req.id,
    name: req.name,
    method: req.method,
    url: req.url,
    params: parseKV(req.params),
    headers: parseKV(req.headers),
    body: req.body || '',
    bodyType: 'none',
    auth: { type: 'none' },
    folderId: req.folderId,
    createdAt: req.createdAt,
    updatedAt: req.updatedAt,
  }
  response.value = null
  responseError.value = null
  requestTab.value = 'params'
  responseTab.value = 'body'
}

async function handleNewRequest(folderId: string | null = null): Promise<void> {
  const req = blankRequest(folderId)
  const created = await apitestApi.createApiRequest({
    name: req.name,
    method: req.method,
    url: req.url,
    params: JSON.stringify(req.params),
    headers: JSON.stringify(req.headers),
    body: req.body,
    folderId: folderId || '',
  })
  requests.value = [...requests.value, created]
  selectRequest(created)
}

async function handleSave(): Promise<void> {
  const draft = currentRequest.value
  const payload = {
    name: draft.name,
    method: draft.method,
    url: draft.url,
    params: JSON.stringify(draft.params),
    headers: JSON.stringify(draft.headers),
    body: draft.body,
    folderId: draft.folderId || '',
  }
  if (requests.value.some((r) => r.id === draft.id)) {
    await apitestApi.updateApiRequest(draft.id, payload)
  } else {
    const created = await apitestApi.createApiRequest(payload)
    draft.id = created.id
  }
  requests.value = await apitestApi.getApiRequests()
  selectedRequestId.value = draft.id
  ElMessage.success('已保存请求')
}

async function handleDeleteRequest(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除该请求？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apitestApi.deleteApiRequest(id)
  requests.value = requests.value.filter((r) => r.id !== id)
  if (selectedRequestId.value === id) {
    selectedRequestId.value = null
    currentRequest.value = blankRequest()
    response.value = null
    responseError.value = null
  }
}

async function handleSend(): Promise<void> {
  if (!currentRequest.value.url.trim()) return
  sending.value = true
  response.value = null
  responseError.value = null
  try {
    const res = await apitestApi.sendHttpRequest({
      method: currentRequest.value.method,
      url: currentRequest.value.url,
      headers: kvToRecord(currentRequest.value.headers),
      params: kvToRecord(currentRequest.value.params),
      body: currentRequest.value.bodyType !== 'none' ? currentRequest.value.body : undefined,
    })
    if (res.error) {
      responseError.value = res.error
    } else {
      response.value = {
        status: res.status,
        statusText: res.statusText,
        headers: res.headers,
        body: res.body,
        duration: res.duration,
      }
      responseTab.value = 'body'
    }
    await apitestApi.createApiHistory({
      method: currentRequest.value.method,
      url: currentRequest.value.url,
      response: res.body || '',
      statusCode: res.status || 0,
      duration: res.duration || 0,
      requestId: currentRequest.value.id,
    })
    history.value = await apitestApi.getApiHistory()
  } catch (err) {
    responseError.value = (err as Error).message
    await apitestApi.createApiHistory({
      method: currentRequest.value.method,
      url: currentRequest.value.url,
      response: '',
      statusCode: 0,
      duration: 0,
      requestId: currentRequest.value.id,
    })
    history.value = await apitestApi.getApiHistory()
  } finally {
    sending.value = false
  }
}

function toggleFolder(folderId: string): void {
  const next = new Set(expandedFolders.value)
  if (next.has(folderId)) next.delete(folderId)
  else next.add(folderId)
  expandedFolders.value = next
}

async function handleNewFolder(): Promise<void> {
  const folder = await apitestApi.createApiFolder({
    name: '新文件夹',
    parentId: '',
    sortOrder: folders.value.length,
  })
  folders.value = [...folders.value, folder]
  const next = new Set(expandedFolders.value)
  next.add(folder.id)
  expandedFolders.value = next
}

async function handleDeleteFolder(folderId: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除该文件夹？其下请求将移至未分组', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apitestApi.deleteApiFolder(folderId)
  folders.value = folders.value.filter((f) => f.id !== folderId)
  const moved = requests.value.filter((r) => r.folderId === folderId)
  for (const r of moved) await apitestApi.updateApiRequest(r.id, { folderId: '' })
  requests.value = requests.value.map((r) => (r.folderId === folderId ? { ...r, folderId: '' } : r))
}

async function handleRenameFolder(folderId: string, currentName: string): Promise<void> {
  try {
    const { value } = await ElMessageBox.prompt('文件夹名称', '重命名', { inputValue: currentName })
    if (value?.trim()) {
      await apitestApi.updateApiFolder(folderId, { name: value.trim() })
      folders.value = folders.value.map((f) => (f.id === folderId ? { ...f, name: value.trim() } : f))
    }
  } catch {
    /* cancelled */
  }
}

function loadFromHistory(entry: APIHistory): void {
  selectedRequestId.value = null
  currentRequest.value = {
    ...blankRequest(),
    method: entry.method,
    url: entry.url,
    name: entry.url,
  }
  if (entry.response) {
    response.value = {
      status: entry.statusCode,
      statusText: '',
      headers: {},
      body: entry.response,
      duration: entry.duration,
    }
    responseError.value = null
  } else {
    response.value = null
    responseError.value = entry.statusCode === 0 ? '请求失败' : null
  }
  showHistory.value = false
}

/* ===================== Update helpers ===================== */
function updateRequest(patch: Partial<ApiRequestDraft>): void {
  currentRequest.value = { ...currentRequest.value, ...patch }
}
function updateAuth(patch: Partial<AuthConfig>): void {
  currentRequest.value = { ...currentRequest.value, auth: { ...currentRequest.value.auth, ...patch } }
}
function braces(k: string): string {
  return `{{${k}}}`
}
function insertEnvVar(key: string): void {
  updateRequest({ url: currentRequest.value.url + `{{${key}}}` })
}

/* ===================== Copy / Export / Import ===================== */
async function handleCopyResponse(): Promise<void> {
  if (!response.value) return
  const text = typeof response.value.body === 'string' ? response.value.body : JSON.stringify(response.value.body, null, 2)
  await navigator.clipboard.writeText(text)
  copiedResponse.value = true
  setTimeout(() => { copiedResponse.value = false }, 2000)
}

function handleExportCollection(): void {
  const data = {
    type: 'qatest-collection',
    version: '1.0',
    exportedAt: new Date().toISOString(),
    folders: folders.value,
    requests: requests.value,
  }
  downloadText(`api-collection-${formatDate(new Date().toISOString())}.json`, JSON.stringify(data, null, 2), 'application/json')
  ElMessage.success('已导出集合')
}

async function handleImportCollection(): Promise<void> {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.json'
  input.onchange = async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = async (ev) => {
      try {
        const data = JSON.parse(ev.target?.result as string)
        if (data.type === 'qatest-collection') {
          const importedFolders = data.folders || []
          const idMap = new Map<string, string>()
          for (const f of importedFolders) {
            const created = await apitestApi.createApiFolder({
              name: f.name,
              parentId: f.parentId || '',
              sortOrder: f.sortOrder || 0,
            })
            idMap.set(f.id, created.id)
          }
          const importedRequests = (data.requests || []).map((r: Record<string, unknown>) => ({
            name: r.name,
            method: r.method,
            url: r.url,
            params: typeof r.params === 'string' ? r.params : JSON.stringify(r.params || []),
            headers: typeof r.headers === 'string' ? r.headers : JSON.stringify(r.headers || []),
            body: (r.body as string) || '',
            folderId: idMap.get(r.folderId as string) || '',
          }))
          for (const r of importedRequests) await apitestApi.createApiRequest(r)
          await loadAll()
          ElMessage.success('导入成功')
        } else {
          ElMessage.error('导入失败，文件格式不正确')
        }
      } catch {
        ElMessage.error('导入失败，请检查文件格式')
      }
    }
    reader.readAsText(file)
  }
  input.click()
}

async function handleClearHistory(): Promise<void> {
  try {
    await ElMessageBox.confirm('确定清空所有历史记录？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apitestApi.clearApiHistory()
  history.value = []
  ElMessage.success('已清空历史')
}

/* ===================== Keyboard shortcuts ===================== */
function onKeydown(e: KeyboardEvent): void {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter' && !sending.value && currentRequest.value.url.trim()) {
    e.preventDefault()
    handleSend()
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    handleSave()
  }
}

onMounted(() => {
  loadAll()
  window.addEventListener('keydown', onKeydown)
})
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold tracking-tight">接口测试</h1>
      <Button variant="outline" size="sm" class="gap-2">
        <BookOpen class="w-4 h-4" /> 接口定义
      </Button>
    </div>
    <div class="flex h-[calc(100vh-12rem)] gap-0 overflow-hidden">
      <!-- ====== Left Panel: Collection Tree ====== -->
      <div class="w-64 shrink-0 border-r bg-muted/20 flex flex-col">
        <!-- Header -->
        <div class="p-3 border-b flex items-center justify-between">
          <h2 class="text-sm font-semibold">接口集合</h2>
          <div class="flex gap-1">
            <Button variant="ghost" size="icon" class="h-7 w-7" @click="handleNewFolder()" title="新建文件夹">
              <FolderOpen class="w-3.5 h-3.5" />
            </Button>
            <Button variant="ghost" size="icon" class="h-7 w-7" @click="handleNewRequest(null)" title="新建请求">
              <Plus class="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>

        <!-- Search -->
        <div class="p-2 border-b">
          <div class="relative">
            <Search class="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              placeholder="搜索接口..."
              class="h-8 pl-7 text-xs"
              v-model="searchTerm"
            />
          </div>
        </div>

        <!-- Environment Badge -->
        <div v-if="activeEnvName" class="px-3 py-2 border-b flex items-center gap-2">
          <Globe class="w-3 h-3 text-muted-foreground" />
          <span class="text-xs text-muted-foreground">环境：</span>
          <Badge variant="outline" class="text-xs h-5">{{ activeEnvName }}</Badge>
        </div>

        <!-- Tree -->
        <div class="flex-1 overflow-y-auto py-1">
          <!-- Folders -->
          <template v-for="({ folder, requests: folderReqs }) in folderRequests" :key="folder.id">
            <div v-if="!(searchTerm && folderReqs.filter(r => matchesSearch(r.name, r.url)).length === 0)">
              <div
                class="flex items-center gap-1 px-2 py-2 cursor-pointer hover:bg-accent/50 transition-colors group"
                @click="toggleFolder(folder.id)"
              >
                <ChevronDown v-if="expandedFolders.has(folder.id)" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <ChevronRight v-else class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <FolderOpen class="w-3.5 h-3.5 text-amber-500 shrink-0" />
                <span
                  class="text-xs font-medium flex-1 truncate"
                  @dblclick.stop="handleRenameFolder(folder.id, folder.name)"
                >
                  {{ folder.name }}
                </span>
                <span class="text-xs text-muted-foreground mr-1">{{ folderReqs.length }}</span>
                <button
                  class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-red-500"
                  @click.stop="handleDeleteFolder(folder.id)"
                >
                  <X class="w-3 h-3" />
                </button>
              </div>
              <div v-if="expandedFolders.has(folder.id)" class="ml-4">
                <div
                  v-for="req in folderReqs.filter(r => matchesSearch(r.name, r.url))"
                  :key="req.id"
                  :class="cn(
                    'flex items-center gap-2 px-2 py-2 cursor-pointer transition-colors group',
                    selectedRequestId === req.id ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50',
                  )"
                  @click="selectRequest(req)"
                >
                  <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold shrink-0', methodClass(req.method))">
                    {{ req.method }}
                  </span>
                  <span class="text-xs truncate flex-1">{{ req.name }}</span>
                  <button
                    class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive/70 shrink-0"
                    @click.stop="bugModalReq = req"
                  >
                    <Bug class="w-3 h-3" />
                  </button>
                  <button
                    class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-red-500 shrink-0"
                    @click.stop="handleDeleteRequest(req.id)"
                  >
                    <Trash2 class="w-3 h-3" />
                  </button>
                </div>
                <div v-if="folderReqs.filter(r => matchesSearch(r.name, r.url)).length === 0 && folderReqs.length > 0 && !searchTerm" class="px-2 py-1 text-xs text-muted-foreground">暂无接口</div>
                <div class="px-2 py-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-6 text-xs gap-1 text-muted-foreground w-full justify-start"
                    @click="handleNewRequest(folder.id)"
                  >
                    <Plus class="w-3 h-3" /> 添加请求
                  </Button>
                </div>

                <!-- Environment variable quick insert -->
                <div v-if="envVars.length > 0" class="flex items-center gap-1.5 flex-wrap">
                  <span class="text-xs text-muted-foreground mr-1">变量:</span>
                  <button
                    v-for="v in envVars.slice(0, 8)"
                    :key="v.key"
                    class="px-1.5 py-0.5 rounded bg-primary/10 text-primary text-xs font-mono hover:bg-primary/20 transition-colors"
                    :title="`插入 {{${v.key}}} — 值: ${v.value}`"
                    @click="insertEnvVar(v.key)"
                  >
                    {{ braces(v.key) }}
                  </button>
                </div>
              </div>
            </div>
          </template>

          <!-- Ungrouped requests -->
          <div
            v-for="req in ungroupedRequests.filter(r => matchesSearch(r.name, r.url))"
            :key="req.id"
            :class="cn(
              'flex items-center gap-2 px-3 py-1.5 cursor-pointer transition-colors group',
              selectedRequestId === req.id ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50',
            )"
            @click="selectRequest(req)"
          >
            <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold shrink-0', methodClass(req.method))">
              {{ req.method }}
            </span>
            <span class="text-xs truncate flex-1">{{ req.name }}</span>
            <button
              class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive/70 shrink-0"
              @click.stop="bugModalReq = req"
            >
              <Bug class="w-3 h-3" />
            </button>
            <button
              class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-red-500 shrink-0"
              @click.stop="handleDeleteRequest(req.id)"
            >
              <Trash2 class="w-3 h-3" />
            </button>
          </div>

          <div v-if="requests.length === 0" class="px-3 py-8 text-center text-muted-foreground">
            <FileText class="w-8 h-8 mx-auto mb-2 opacity-30" />
            <p class="text-xs">暂无接口</p>
            <Button variant="link" size="sm" class="text-xs mt-1" @click="handleNewRequest(null)">
              创建第一个请求
            </Button>
          </div>
        </div>

        <!-- History toggle -->
        <div class="border-t p-2">
          <Button
            :variant="showHistory ? 'secondary' : 'ghost'"
            size="sm"
            class="w-full gap-2 text-xs"
            @click="showHistory = !showHistory"
          >
            <Clock class="w-3.5 h-3.5" />
            历史记录 ({{ history.length }})
          </Button>
        </div>
      </div>

      <!-- ====== Center: Request Builder + Response ====== -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- Request Name + URL Bar -->
        <div class="border-b bg-background p-3 space-y-3">
          <div class="flex items-center gap-3">
            <Input
              v-model="currentRequest.name"
              class="h-8 text-sm font-medium border-dashed"
              placeholder="请求名称"
            />
            <Button variant="outline" size="sm" class="gap-1.5 shrink-0" @click="handleSave" title="Ctrl+S 保存">
              <Save class="w-3.5 h-3.5" /> 保存
            </Button>
            <Button variant="outline" size="sm" class="gap-1.5 shrink-0" @click="showHistory = !showHistory">
              <Clock class="w-3.5 h-3.5" /> 历史
            </Button>
            <Button variant="outline" size="sm" class="gap-1.5 shrink-0" @click="handleExportCollection" title="导出集合">
              <Download class="w-3.5 h-3.5" /> 导出
            </Button>
            <Button variant="outline" size="sm" class="gap-1.5 shrink-0" @click="handleImportCollection" title="导入集合">
              <Upload class="w-3.5 h-3.5" /> 导入
            </Button>
          </div>

          <!-- Method + URL + Send -->
          <div class="flex gap-2">
            <div class="relative shrink-0">
              <button
                :class="cn('flex items-center gap-1 h-9 px-3 rounded-md border font-bold text-sm transition-colors', methodClass(currentRequest.method))"
                @click="methodDropdownOpen = !methodDropdownOpen"
              >
                {{ currentRequest.method }}
                <ChevronDown class="w-3.5 h-3.5" />
              </button>
              <template v-if="methodDropdownOpen">
                <div class="fixed inset-0 z-40" @click="methodDropdownOpen = false" />
                <div class="absolute top-full left-0 mt-1 z-50 bg-popover border rounded-md shadow-lg py-1 w-28">
                  <button
                    v-for="m in METHODS"
                    :key="m"
                    :class="cn(
                      'w-full text-left px-3 py-1.5 text-sm font-bold hover:bg-accent transition-colors',
                      methodClass(m),
                      currentRequest.method === m ? 'bg-accent' : '',
                    )"
                    @click="updateRequest({ method: m }); methodDropdownOpen = false"
                  >
                    {{ m }}
                  </button>
                </div>
              </template>
            </div>

            <Input
              ref="urlInputRef"
              v-model="currentRequest.url"
              placeholder="输入请求 URL，例如 https://api.example.com/users"
              class="h-9 font-mono text-sm flex-1"
              @keydown.enter="!sending && handleSend()"
            />

            <Button
              class="gap-2 shrink-0 h-9 px-5"
              @click="handleSend"
              :disabled="sending || !currentRequest.url.trim()"
              title="Ctrl+Enter 发送"
            >
              <template v-if="sending">
                <Loader2 class="w-4 h-4 animate-spin" /> 发送中
              </template>
              <template v-else>
                <Send class="w-4 h-4" /> 发送
              </template>
            </Button>
          </div>
        </div>

        <!-- Request Tabs -->
        <div class="border-b">
          <div class="flex">
            <button
              v-for="tab in ([{ key: 'params', label: 'Params', count: paramsCount }, { key: 'headers', label: 'Headers', count: headersCount }, { key: 'body', label: 'Body' }, { key: 'auth', label: 'Auth' }])"
              :key="tab.key"
              @click="requestTab = tab.key as 'params' | 'headers' | 'body' | 'auth'"
              :class="cn(
                'px-4 py-2.5 text-xs font-medium transition-colors border-b-2 -mb-px',
                requestTab === tab.key
                  ? 'border-primary text-primary'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )"
            >
              {{ tab.label }}
              <span v-if="tab.count !== undefined && tab.count > 0" class="ml-1.5 px-1.5 py-0.5 rounded-full bg-primary/10 text-primary text-xs">
                {{ tab.count }}
              </span>
            </button>
          </div>
        </div>

        <!-- Request Tab Content -->
        <div class="flex-1 overflow-y-auto p-4 min-h-0" style="max-height: 40%">
          <!-- Params Tab -->
          <KeyValueEditor
            v-if="requestTab === 'params'"
            v-model="currentRequest.params"
            key-placeholder="参数名"
            value-placeholder="值"
          />

          <!-- Headers Tab -->
          <KeyValueEditor
            v-else-if="requestTab === 'headers'"
            v-model="currentRequest.headers"
            key-placeholder="Header 名"
            value-placeholder="值"
            show-description
          />

          <!-- Body Tab -->
          <div v-else-if="requestTab === 'body'" class="space-y-3">
            <div class="flex items-center gap-4">
              <label v-for="type in (['none', 'json', 'form', 'raw'] as const)" :key="type" class="flex items-center gap-1.5 cursor-pointer">
                <input
                  type="radio"
                  name="bodyType"
                  :checked="currentRequest.bodyType === type"
                  @change="updateRequest({ bodyType: type })"
                  class="accent-primary"
                />
                <span class="text-xs font-medium">
                  {{ type === 'none' ? 'None' : type === 'json' ? 'JSON' : type === 'form' ? 'Form' : 'Raw' }}
                </span>
              </label>
            </div>

            <textarea
              v-if="currentRequest.bodyType !== 'none'"
              :value="currentRequest.body"
              @input="updateRequest({ body: ($event.target as HTMLTextAreaElement).value })"
              :placeholder="currentRequest.bodyType === 'json' ? '{\n  \&quot;key\&quot;: \&quot;value\&quot;\n}' : currentRequest.bodyType === 'form' ? 'key1=value1&key2=value2' : '请求体内容...'"
              class="w-full h-48 p-3 rounded-md border border-input bg-background font-mono text-sm resize-y focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <div v-else class="py-8 text-center text-muted-foreground text-sm">该请求无请求体</div>
          </div>

          <!-- Auth Tab -->
          <div v-else-if="requestTab === 'auth'" class="space-y-4">
            <div class="flex items-center gap-2">
              <Shield class="w-4 h-4 text-muted-foreground" />
              <span class="text-xs font-medium text-muted-foreground">认证类型</span>
              <select
                :value="currentRequest.auth.type"
                @change="updateAuth({ type: ($event.target as HTMLSelectElement).value as AuthConfig['type'] })"
                class="h-8 px-2 rounded-md border border-input bg-background text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="none">无认证</option>
                <option value="bearer">Bearer Token</option>
                <option value="basic">Basic Auth</option>
                <option value="apikey">API Key</option>
              </select>
            </div>

            <div v-if="currentRequest.auth.type === 'bearer'" class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">Token</label>
              <Input
                v-model="currentRequest.auth.bearerToken"
                placeholder="输入 Bearer Token，支持 {{变量名}}"
                class="font-mono text-sm"
              />
            </div>

            <div v-else-if="currentRequest.auth.type === 'basic'" class="space-y-3">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">用户名</label>
                <Input v-model="currentRequest.auth.basicUser" placeholder="Username" class="text-sm" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">密码</label>
                <Input v-model="currentRequest.auth.basicPass" type="password" placeholder="Password" class="text-sm" />
              </div>
            </div>

            <div v-else-if="currentRequest.auth.type === 'apikey'" class="space-y-3">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Key 名称</label>
                <Input v-model="currentRequest.auth.apiKeyName" placeholder="例如 X-API-Key" class="text-sm" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Key 值</label>
                <Input v-model="currentRequest.auth.apiKeyValue" placeholder="输入 API Key 值" class="font-mono text-sm" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">添加到</label>
                <select
                  :value="currentRequest.auth.apiKeyIn || 'header'"
                  @change="updateAuth({ apiKeyIn: ($event.target as HTMLSelectElement).value as 'header' | 'query' })"
                  class="h-8 px-2 rounded-md border border-input bg-background text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  <option value="header">请求头 (Header)</option>
                  <option value="query">查询参数 (Query)</option>
                </select>
              </div>
            </div>

            <div v-else class="py-8 text-center text-muted-foreground text-sm">该请求无需认证</div>
          </div>
        </div>

        <!-- ====== Response Section ====== -->
        <div class="border-t flex flex-col min-h-0" style="flex: 1 1 0">
          <div class="flex items-center gap-3 px-4 py-2 border-b bg-muted/20">
            <span class="text-xs font-semibold text-muted-foreground">响应</span>

            <template v-if="response">
              <Badge :class="cn('text-xs font-bold', statusColor(response.status))">
                {{ response.status }} {{ response.statusText }}
              </Badge>
              <span class="text-xs text-muted-foreground flex items-center gap-1">
                <Clock class="w-3 h-3" /> {{ response.duration }} ms
              </span>
              <span class="text-xs text-muted-foreground">
                {{ formatSize((response.body || '').length) }}
              </span>
            </template>

            <Badge v-if="responseError" class="bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 text-xs font-bold">
              请求失败
            </Badge>

            <span v-if="!response && !responseError && !sending" class="text-xs text-muted-foreground">点击发送按钮查看响应</span>

            <span v-if="sending" class="text-xs text-muted-foreground flex items-center gap-1">
              <Loader2 class="w-3 h-3 animate-spin" /> 请求中...
            </span>

            <div v-if="response" class="ml-auto flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                class="h-6 w-6"
                @click="handleCopyResponse"
                title="复制响应"
              >
                <Check v-if="copiedResponse" class="w-3 h-3 text-emerald-500" />
                <Copy v-else class="w-3 h-3" />
              </Button>
              <Button
                :variant="prettyPrint ? 'secondary' : 'ghost'"
                size="icon"
                class="h-6 w-6"
                @click="prettyPrint = !prettyPrint"
                title="格式化 JSON"
              >
                <Braces class="w-3 h-3" />
              </Button>
              <button
                v-for="tab in (['body', 'headers'] as const)"
                :key="tab"
                @click="responseTab = tab"
                :class="cn(
                  'px-3 py-1 text-xs font-medium transition-colors border-b-2 -mb-2',
                  responseTab === tab
                    ? 'border-primary text-primary'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
                )"
              >
                {{ tab === 'body' ? 'Body' : 'Headers' }}
              </button>
            </div>
          </div>

          <div class="flex-1 overflow-y-auto p-4 min-h-0">
            <div v-if="responseError" class="p-4 rounded-lg border border-red-200 bg-red-50 dark:border-red-900/50 dark:bg-red-950/30">
              <p class="text-sm font-medium text-red-700 dark:text-red-400">请求失败</p>
              <p class="text-sm text-red-600 dark:text-red-300 mt-1">{{ responseError }}</p>
            </div>

            <template v-else-if="response && responseTab === 'body'">
              <JsonViewer v-if="prettyPrint && isJsonBody" :value="responseBodyObj" max-height="100%" />
              <pre v-else class="raw-pre">{{ prettyPrint ? formatJson(response.body) : (response.body || '') }}</pre>
            </template>

            <div v-else-if="response && responseTab === 'headers'" class="space-y-1">
              <div v-for="[key, value] in Object.entries(response.headers || {})" :key="key" class="flex items-center gap-2 text-xs">
                <code class="font-medium text-primary min-w-[180px] truncate">{{ key }}</code>
                <span class="text-muted-foreground">:</span>
                <code class="text-foreground break-all">{{ value }}</code>
              </div>
            </div>

            <div v-if="!response && !responseError && !sending" class="flex items-center justify-center h-full text-muted-foreground">
              <div class="text-center">
                <Send class="w-10 h-10 mx-auto mb-3 opacity-20" />
                <p class="text-sm">输入 URL 并发送请求</p>
                <p class="text-xs mt-1">支持环境变量 <span v-pre>{{变量名}}</span> 语法</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ====== Right Panel: History (collapsible) ====== -->
      <div v-if="showHistory" class="w-72 shrink-0 border-l bg-muted/20 flex flex-col">
        <div class="p-3 border-b flex items-center justify-between">
          <h2 class="text-sm font-semibold flex items-center gap-2">
            <Clock class="w-4 h-4" /> 历史记录 ({{ filteredHistory.length }})
          </h2>
          <div class="flex items-center gap-1">
            <Button variant="ghost" size="icon" class="h-6 w-6" @click="handleClearHistory" title="清空历史">
              <Trash2 class="w-3 h-3" />
            </Button>
            <Button variant="ghost" size="icon" class="h-7 w-7" @click="showHistory = false">
              <X class="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
        <div class="p-2 border-b">
          <div class="relative">
            <Search class="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
            <Input
              placeholder="搜索历史..."
              class="h-8 pl-7 text-xs"
              v-model="historySearch"
            />
          </div>
        </div>
        <div class="flex-1 overflow-y-auto">
          <div v-if="filteredHistory.length === 0" class="p-4 text-center text-muted-foreground text-xs">暂无历史记录</div>
          <div
            v-for="entry in filteredHistory"
            :key="entry.id"
            class="flex items-center gap-2 px-3 py-2 border-b hover:bg-accent/50 cursor-pointer transition-colors"
            @click="loadFromHistory(entry)"
          >
            <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold shrink-0', methodClass(entry.method))">
              {{ entry.method }}
            </span>
            <div class="flex-1 min-w-0">
              <p class="text-xs font-mono truncate">{{ entry.url }}</p>
              <div class="flex items-center gap-2 mt-0.5">
                <span v-if="entry.statusCode === 0" class="text-xs text-red-500">失败</span>
                <span
                  v-else
                  :class="cn('text-xs font-medium', entry.statusCode >= 400 ? 'text-red-500' : 'text-emerald-600')"
                >
                  {{ entry.statusCode }}
                </span>
                <span v-if="entry.duration > 0" class="text-xs text-muted-foreground">{{ entry.duration }}ms</span>
                <span class="text-xs text-muted-foreground">{{ formatDate(entry.createdAt) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ReportBugModal v-model="reportVisible" :initial="bugModalInitial" />
  </div>
</template>

<style scoped>
.raw-pre {
  margin: 0;
  overflow: auto;
  background: #0b1220;
  border: 1px solid var(--qt-border, #1e293b);
  border-radius: 8px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  color: #cbd5e1;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
