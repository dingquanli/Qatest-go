<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { RotateCcw, Send, Trash2, BookOpen, ChevronDown, Pause, Download, FolderOpen, X, Copy } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import * as proxyApi from '@/api/proxy'
import * as sdkApi from '@/api/sdk'
import * as logsApi from '@/api/logs'
import * as protoApi from '@/api/proto'
import { useProxyWebSocket } from '@/composables/useProxyWebSocket'
import { useUserStore } from '@/stores/user'
import type { SdkItem } from '@/types'

const userStore = useUserStore()
const { send: sendWs, onMessage: onWsMessage, connect: connectWs } = useProxyWebSocket(
  () => userStore.getToken(),
)

interface LogEntry {
  _meta?: boolean
  seq?: number
  ts?: string
  type?: string
  method?: string
  target?: string
  request?: unknown
  response?: unknown
  error?: string | null
  elapsed_ms?: number | null
  source?: string
  [key: string]: unknown
}

interface LogFileInfo {
  name: string
  size: number
  modTime: string
}

interface ForwardRecord {
  id: number
  method: string
  target: string
  request: unknown
  response?: unknown | null
  error?: string | null
  elapsed_ms?: number | null
  success: boolean | null
  timestamp: string
}

interface SendResult {
  success: boolean
  method: string
  target: string
  request: unknown
  response?: unknown
  error?: string | null
  elapsed_ms?: number | null
}

const logEntries = ref<LogEntry[]>([])
const logFiles = ref<LogFileInfo[]>([])
const selectedLogFile = ref('')
const logFilter = ref('all')
const selectedLogIndex = ref<number | null>(null)

const sendTarget = ref('')
const sendBody = ref('{}')
const sendResult = ref<SendResult | null>(null)
const sending = ref(false)
const forwardRecords = ref<ForwardRecord[]>([])
const selectedForwardId = ref<number | null>(null)

const activeTab = ref('detail')
const paused = ref(false)

/* 协议定义（proto 目录） */
const protoDir = ref('')
const protoDirInput = ref('')
const protoMethodCount = ref(0)
const showProtoInput = ref(false)

/* SDK download */
const sdkOpen = ref(false)
const sdks = ref<SdkItem[]>([])
const reportToken = computed(() => sdks.value[0]?.reportToken || '')
const sdkGroups = computed(() => {
  const map = new Map<string, { engine: string; label: string; files: { file: string; size?: number }[] }>()
  for (const s of sdks.value) {
    if (!map.has(s.engine)) map.set(s.engine, { engine: s.engine, label: s.engineLabel || s.engine, files: [] })
    map.get(s.engine)!.files.push({ file: s.file, size: s.size })
  }
  return Array.from(map.values())
})

function copyReportToken() {
  if (!reportToken.value) return
  navigator.clipboard?.writeText(reportToken.value).then(
    () => ElMessage.success('上报 Token 已复制，请填入 SDK 的 token 配置'),
    () => ElMessage.warning('复制失败'),
  )
}

function formatJson(v: unknown): string {
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'string') {
    try { return JSON.stringify(JSON.parse(v), null, 2) } catch { return v }
  }
  try { return JSON.stringify(v, null, 2) } catch { return String(v) }
}
function formatTime(iso: string): string {
  if (!iso) return ''
  const parts = iso.split('T')[1]
  return parts ? parts.split('.')[0] : iso
}

/* WebSocket → live protocol recording */
function onWs(raw: string): void {
  let msg: Record<string, unknown>
  try { msg = JSON.parse(raw) } catch { return }
  const type = msg.type as string
  const id = msg.id as number
  if (type === 'proxy-request') {
    if (paused.value) return
    const entry: LogEntry = {
      seq: id,
      ts: (msg.timestamp as string) || new Date().toISOString(),
      type: 'REQUEST_RESPONSE',
      method: (msg.method as string) || '',
      target: (msg.target as string) || '',
      request: msg.request ?? {},
      response: null,
      error: null,
      elapsed_ms: null,
    }
    const idx = logEntries.value.findIndex((e) => e.seq === id)
    if (idx >= 0) logEntries.value[idx] = entry
    else logEntries.value = [entry, ...logEntries.value]
    if (logEntries.value.length > 2000) logEntries.value.shift()
  } else if (type === 'proxy-response') {
    logEntries.value = logEntries.value.map((e) =>
      e.seq === id
        ? {
            ...e,
            response: msg.response ?? null,
            elapsed_ms: (msg.elapsed_ms as number) ?? null,
            target: (msg.target as string) || e.target,
          }
        : e,
    )
  } else if (type === 'proxy-done') {
    logEntries.value = logEntries.value.map((e) =>
      e.seq === id ? { ...e, type: (msg.dropped ? 'DROPPED' : (e.type || 'REQUEST_RESPONSE')) } : e,
    )
  } else if (type === 'proxy-error') {
    logEntries.value = logEntries.value.map((e) =>
      e.seq === id ? { ...e, type: 'ERROR', error: (msg.error as string) || 'Unknown error' } : e,
    )
  }
}
onWsMessage(onWs)

const filteredEntries = computed(() =>
  logEntries.value
    .filter((e) => !e._meta)
    .filter((e) => {
      if (logFilter.value === 'request') return e.type === 'REQUEST_RESPONSE' && !e.error && !e.response
      if (logFilter.value === 'response') return e.type === 'REQUEST_RESPONSE' && !e.error && e.response
      if (logFilter.value === 'error') return e.type === 'ERROR' || !!e.error
      return true
    }),
)

const selectedEntry = computed(() =>
  selectedLogIndex.value !== null ? filteredEntries.value[selectedLogIndex.value] || null : null,
)
const selectedForward = computed(() => forwardRecords.value.find((r) => r.id === selectedForwardId.value) || null)

async function loadFiles(): Promise<void> {
  try {
    const d = await logsApi.getLogFiles()
    logFiles.value = d || []
    if (logFiles.value.length) {
      selectedLogFile.value = logFiles.value[0].name
      await loadFileContent(logFiles.value[0].name)
    }
  } catch { /* ignore */ }
}

async function loadFileContent(name: string): Promise<void> {
  if (!name) return
  try {
    const d = await logsApi.getLogFile(name)
    const lines = (d.content || '').split('\n')
    const arr: LogEntry[] = []
    for (const ln of lines) {
      const s = ln.trim()
      if (!s) continue
      try { arr.push(JSON.parse(s) as LogEntry) } catch { /* ignore non-JSON line */ }
    }
    logEntries.value = arr
  } catch (e) {
    ElMessage.error(`读取日志文件失败：${e instanceof Error ? e.message : '未知错误'}`)
  }
}

async function onFileChange(e: Event): Promise<void> {
  const val = (e.target as HTMLSelectElement).value
  selectedLogFile.value = val
  selectedLogIndex.value = null
  await loadFileContent(val)
}

async function refresh(): Promise<void> {
  await loadFiles()
  ElMessage.success('已刷新')
}

function handleSelectLog(index: number): void {
  selectedLogIndex.value = index
  const entry = filteredEntries.value[index]
  if (entry) {
    sendTarget.value = entry.target || ''
    sendBody.value =
      typeof entry.request === 'string'
        ? entry.request
        : JSON.stringify(entry.request || {}, null, 2)
    sendResult.value = null
    activeTab.value = 'detail'
  }
}

function pushForward(r: ForwardRecord): void {
  forwardRecords.value = [r, ...forwardRecords.value]
  if (forwardRecords.value.length > 500) forwardRecords.value.pop()
}

async function doSend(): Promise<void> {
  const entry = selectedEntry.value
  if (!entry) return
  const method = entry.method || ''
  const target = sendTarget.value || entry.target || ''
  if (!target) { ElMessage.warning('请输入目标服务器地址'); return }

  let parsedBody: unknown
  try { parsedBody = JSON.parse(sendBody.value) } catch { ElMessage.warning('请求体 JSON 格式错误'); return }

  sending.value = true
  sendResult.value = null
  try {
    const data = (await proxyApi.sendProxyRequest({
      method: method.startsWith('/') ? method.slice(1) : method,
      request: JSON.stringify(parsedBody),
      target,
      timeout: 30000,
    })) as { response?: unknown; elapsed_ms?: number }
    const result: SendResult = {
      success: true,
      method,
      target,
      request: parsedBody,
      response: data?.response ?? null,
      error: null,
      elapsed_ms: data?.elapsed_ms ?? null,
    }
    sendResult.value = result
    pushForward({ ...result, id: Date.now(), timestamp: new Date().toISOString() })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    const result: SendResult = {
      success: false,
      method,
      target,
      request: parsedBody,
      response: null,
      error: msg,
      elapsed_ms: 0,
    }
    sendResult.value = result
    pushForward({ ...result, id: Date.now(), timestamp: new Date().toISOString() })
  } finally {
    sending.value = false
  }
}

async function clearExecutions(): Promise<void> {
  try { await proxyApi.clearProxyExecutions() } catch { /* ignore */ }
  forwardRecords.value = []
  selectedForwardId.value = null
}

function clearEntries(): void {
  logEntries.value = []
  selectedLogIndex.value = null
}

function selectForward(id: number): void {
  selectedForwardId.value = id
}

function downloadSdk(item: SdkItem): void {
  sdkApi
    .downloadSdk(item.engine, item.file)
    .catch((err: Error) => ElMessage.error(err.message || '下载失败'))
}

async function handleSetProtoDir(): Promise<void> {
  showProtoInput.value = true
  try {
    const d = await protoApi.getProtoDir()
    protoDirInput.value = ((d as { dir?: string }).dir) || ''
  } catch { /* ignore */ }
}

async function confirmProtoDir(): Promise<void> {
  const dir = protoDirInput.value.trim()
  if (!dir) { showProtoInput.value = false; return }
  try {
    const r = await protoApi.setProtoDir(dir)
    protoDir.value = ((r as { dir?: string }).dir) || dir
    showProtoInput.value = false
    const desc = await protoApi.getProtoDescribe().catch(() => null)
    protoMethodCount.value = ((desc as { methodCount?: number })?.methodCount) ?? 0
    ElMessage.success('协议定义已加载')
  } catch (e) {
    ElMessage.error(`协议定义加载失败：${e instanceof Error ? e.message : '未知错误'}`)
  }
}

async function loadInitial(): Promise<void> {
  await loadFiles()
  try {
    const execs = (await proxyApi.getProxyExecutions()) as unknown as Array<{
      id: number
      method: string
      target: string
      timestamp: string
      elapsed_ms?: number
    }>
    forwardRecords.value = (execs || []).map((e) => ({
      id: e.id,
      method: e.method,
      target: e.target,
      request: null,
      response: null,
      error: null,
      elapsed_ms: e.elapsed_ms ?? null,
      success: null,
      timestamp: e.timestamp,
    }))
  } catch { /* ignore */ }
  try { sdks.value = await sdkApi.getSdkList() } catch { /* ignore */ }
  try {
    const d = await protoApi.getProtoDir()
    protoDir.value = ((d as { dir?: string }).dir) || ''
    protoDirInput.value = protoDir.value
  } catch { /* ignore */ }
  try {
    const desc = await protoApi.getProtoDescribe()
    protoMethodCount.value = ((desc as { methodCount?: number })?.methodCount) ?? 0
  } catch { /* ignore */ }
  connectWs()
}

onMounted(loadInitial)
onUnmounted(() => { /* ws closed by composable */ })
</script>

<template>
  <div class="flex flex-col h-[calc(100vh-6rem)] gap-3">
    <!-- Top toolbar -->
    <Card class="p-3 flex items-center gap-2 shrink-0">
      <div class="relative">
        <select
          class="h-8 text-xs border rounded-lg pl-2 pr-7 bg-background font-mono appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary w-44"
          :value="selectedLogFile"
          @change="onFileChange"
        >
          <option v-for="f in logFiles" :key="f.name" :value="f.name">{{ f.name }}</option>
        </select>
        <ChevronDown class="absolute right-1.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
      </div>
      <Button variant="outline" size="sm" class="gap-1 rounded-lg" @click="refresh">
        <RotateCcw class="w-3.5 h-3.5" /> 刷新
      </Button>
      <Button variant="ghost" size="sm" class="gap-1 rounded-lg" @click="paused = !paused">
        <Pause class="w-3.5 h-3.5" />
        {{ paused ? '恢复' : '暂停' }}
      </Button>
      <Button variant="ghost" size="sm" class="gap-1 rounded-lg" @click="clearEntries">
        <Trash2 class="w-3.5 h-3.5" />
        清除
      </Button>
      <!-- 协议定义 -->
      <div class="flex items-center gap-1.5">
        <Button variant="outline" size="sm" class="gap-1.5 rounded-lg" @click="handleSetProtoDir">
          <FolderOpen class="w-3.5 h-3.5" />
          <span class="text-xs">协议定义</span>
        </Button>
        <Badge v-if="protoMethodCount > 0" variant="secondary" class="text-[10px]">{{ protoMethodCount }} 方法</Badge>
      </div>
      <div v-if="showProtoInput" class="flex items-center gap-1.5">
        <Input
          :value="protoDirInput"
          class="h-8 w-72 text-xs font-mono"
          placeholder="选择协议定义目录..."
          @input="protoDirInput = ($event.target as HTMLInputElement).value"
          @keyup.enter="confirmProtoDir"
        />
        <Button size="sm" class="h-8 rounded-lg" @click="confirmProtoDir">确定</Button>
        <Button size="sm" variant="ghost" class="h-8" @click="showProtoInput = false">
          <X class="w-3.5 h-3.5" />
        </Button>
      </div>
      <div class="ml-auto flex gap-2">
        <a
          href="/docs/sdk.md"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 rounded-lg border border-input bg-background px-3 py-1.5 text-xs font-medium shadow-sm hover:bg-accent hover:text-accent-foreground"
        >
          <BookOpen class="w-3 h-3 shrink-0" /> SDK 文件使用说明
        </a>
        <div
          v-if="reportToken"
          class="inline-flex items-center gap-1.5 rounded-lg border border-input bg-background px-2.5 py-1.5 text-xs"
          :title="reportToken"
        >
          <span class="text-muted-foreground">上报 Token</span>
          <code class="font-mono max-w-[120px] truncate">{{ reportToken }}</code>
          <button class="text-primary hover:text-primary/80" title="复制上报 Token" @click="copyReportToken">
            <Copy class="w-3 h-3" />
          </button>
        </div>
        <div class="relative">
          <Button variant="outline" size="sm" class="gap-1 rounded-lg" @click="sdkOpen = !sdkOpen">
            <Download class="w-3 h-3" /> 下载 SDK
            <ChevronDown class="w-3 h-3" />
          </Button>
          <template v-if="sdkOpen">
            <div class="fixed inset-0 z-40" @click="sdkOpen = false" />
            <div class="absolute right-0 top-full mt-1 z-50 w-80 max-h-96 overflow-auto rounded-lg border bg-popover shadow-lg">
              <div v-if="sdkGroups.length === 0" class="px-3 py-4 text-xs text-center text-muted-foreground">加载中...</div>
              <div v-for="eng in sdkGroups" v-else :key="eng.engine" class="border-b last:border-0">
                <div class="px-3 py-1.5 text-xs font-semibold text-muted-foreground bg-accent/30">
                  {{ eng.label }} <span class="ml-1 text-muted-foreground/60">{{ eng.engine }}</span>
                </div>
                <button
                  v-for="f in eng.files"
                  :key="f.file"
                  class="w-full text-left px-3 py-2 text-xs hover:bg-accent/50 flex items-center justify-between transition-colors"
                  @click="downloadSdk({ engine: eng.engine, file: f.file } as SdkItem); sdkOpen = false"
                >
                  <span class="font-mono">{{ f.file }}</span>
                  <span class="text-muted-foreground">{{ f.size && f.size > 0 ? `${(f.size / 1024).toFixed(1)}KB` : '-' }}</span>
                </button>
              </div>
            </div>
          </template>
        </div>
      </div>
    </Card>

    <!-- Filter bar -->
    <div class="flex gap-1 shrink-0 px-1">
      <button
        v-for="f in (['all', 'request', 'response', 'error'] as const)"
        :key="f"
        :class="cn(
          'px-3 py-1 text-xs rounded-md font-medium transition-colors',
          logFilter === f ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-accent/50',
        )"
        @click="logFilter = f; selectedLogIndex = null"
      >
        {{ f === 'all' ? '全部' : f === 'request' ? '请求' : f === 'response' ? '响应' : '错误' }}
      </button>
      <span class="ml-2 text-xs text-muted-foreground self-center">{{ filteredEntries.length }} 条</span>
    </div>

    <!-- Main area -->
    <div class="flex-1 flex gap-0 min-h-0 border rounded-xl bg-card overflow-hidden">
      <!-- Left: entry list -->
      <div class="w-72 shrink-0 border-r overflow-auto">
        <div v-if="filteredEntries.length === 0" class="p-6 text-center text-sm text-muted-foreground">暂无日志条目</div>
        <div
          v-for="(e, i) in filteredEntries"
          v-else
          :key="i"
          :class="cn(
            'flex items-center gap-2 px-3 py-1.5 border-b border-border/20 cursor-pointer transition-colors text-xs',
            selectedLogIndex === i ? 'bg-primary/10' : 'hover:bg-accent/50',
          )"
          @click="handleSelectLog(i)"
        >
          <span :class="cn('w-4 text-center font-bold', e.type === 'ERROR' || e.error ? 'text-red-500' : e.response ? 'text-blue-500' : 'text-emerald-500')">{{ e.type === 'ERROR' || e.error ? '✗' : e.response ? '↓' : '↑' }}</span>
          <span class="text-[10px] text-muted-foreground font-mono w-8">{{ e.seq || '-' }}</span>
          <span class="font-mono truncate flex-1 text-[11px]">{{ e.method || '-' }}</span>
          <span class="text-[10px] text-muted-foreground">{{ e.ts ? e.ts.split('T')[1]?.split('.')[0] : '' }}</span>
        </div>
      </div>

      <!-- Right: detail / forward / records -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- Tabs -->
        <div class="px-3 py-2 border-b bg-muted/20 flex items-center gap-1 shrink-0">
          <button
            v-for="tab in (['detail', 'forward', 'records'] as const)"
            :key="tab"
            :class="cn(
              'px-3 py-1 text-xs rounded-md transition-colors font-medium',
              activeTab === tab ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground',
            )"
            @click="activeTab = tab"
          >
            {{ tab === 'detail' ? '详情' : tab === 'forward' ? '协议转发' : '执行记录' }}
          </button>
        </div>

        <!-- Tab content -->
        <div class="flex-1 overflow-auto p-3">
          <!-- Detail tab -->
          <div v-if="activeTab === 'detail'">
            <div v-if="!selectedEntry" class="text-sm text-muted-foreground py-8 text-center">选择一个日志条目查看详情</div>
            <div v-else class="space-y-3">
              <div class="flex items-center gap-2 text-xs">
                <span class="font-semibold font-mono">{{ selectedEntry.method || '-' }}</span>
                <Badge
                  variant="outline"
                  :class="cn('text-[10px]', (selectedEntry.type === 'ERROR' || !!selectedEntry.error) ? 'text-red-600' : 'text-emerald-600')"
                >
                  {{ selectedEntry.type || 'REQUEST_RESPONSE' }}
                </Badge>
                <span v-if="selectedEntry.target" class="text-cyan-600">→ {{ selectedEntry.target }}</span>
                <span v-if="selectedEntry.elapsed_ms != null" class="text-muted-foreground">{{ selectedEntry.elapsed_ms }}ms</span>
              </div>
              <div v-if="selectedEntry.request != null">
                <div class="text-[11px] font-semibold text-muted-foreground mb-1">请求体</div>
                <pre class="bg-muted/30 rounded-lg p-3 text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap">{{ formatJson(selectedEntry.request) }}</pre>
              </div>
              <div v-if="selectedEntry.response != null">
                <div class="text-[11px] font-semibold text-muted-foreground mb-1">响应体</div>
                <pre class="bg-muted/30 rounded-lg p-3 text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap">{{ formatJson(selectedEntry.response) }}</pre>
              </div>
              <div v-if="selectedEntry.error">
                <div class="text-[11px] font-semibold text-red-500 mb-1">错误</div>
                <pre class="bg-red-50 dark:bg-red-950/20 rounded-lg p-3 text-xs font-mono overflow-auto max-h-32 text-red-600 whitespace-pre-wrap">{{ typeof selectedEntry.error === 'string' ? selectedEntry.error : JSON.stringify(selectedEntry.error, null, 2) }}</pre>
              </div>
              <details>
                <summary class="text-[11px] text-muted-foreground cursor-pointer">原始 JSON</summary>
                <pre class="bg-muted/20 rounded-lg p-2 mt-1 text-[10px] font-mono overflow-auto max-h-64 whitespace-pre-wrap">{{ JSON.stringify(selectedEntry, null, 2) }}</pre>
              </details>
            </div>
          </div>

          <!-- Forward tab -->
          <div v-if="activeTab === 'forward'">
            <div v-if="!selectedEntry" class="text-sm text-muted-foreground py-8 text-center">选择一个日志条目后进行协议转发</div>
            <div v-else class="space-y-3">
              <div>
                <label class="text-[11px] font-semibold text-muted-foreground block mb-1">目标地址</label>
                <Input
                  :value="sendTarget"
                  class="h-8 text-xs font-mono"
                  placeholder="host:port"
                  @input="sendTarget = ($event.target as HTMLInputElement).value"
                />
              </div>
              <div>
                <label class="text-[11px] font-semibold text-muted-foreground block mb-1">请求体</label>
                <textarea
                  v-model="sendBody"
                  class="w-full min-h-[160px] bg-muted/30 border rounded-lg p-3 text-xs font-mono resize-y outline-none focus:ring-1 focus:ring-primary/30"
                  spellcheck="false"
                />
              </div>
              <Button class="gap-1.5 rounded-lg" size="sm" :disabled="sending" @click="doSend">
                <Send class="w-3.5 h-3.5" />
                {{ sending ? '发送中...' : '发送' }}
              </Button>
              <div
                v-if="sendResult"
                :class="cn(
                  'rounded-lg border p-3 space-y-1',
                  sendResult.success ? 'border-emerald-200 bg-emerald-50/30 dark:border-emerald-800 dark:bg-emerald-950/10' : 'border-red-200 bg-red-50/30 dark:border-red-800 dark:bg-red-950/10',
                )"
              >
                <div :class="cn('text-xs font-semibold', sendResult.success ? 'text-emerald-600' : 'text-red-600')">
                  {{ sendResult.success ? '发送成功' : '发送失败' }}
                  <span class="font-normal ml-2">{{ sendResult.elapsed_ms }}ms</span>
                </div>
                <pre class="text-[11px] font-mono whitespace-pre-wrap max-h-48 overflow-auto">{{ formatJson(sendResult.success ? sendResult.response : sendResult.error) }}</pre>
              </div>
            </div>
          </div>

          <!-- Records tab -->
          <div v-if="activeTab === 'records'">
            <div v-if="forwardRecords.length === 0" class="text-center text-sm text-muted-foreground py-12">
              暂无执行记录，使用「协议转发」发送请求后记录会显示在这里
            </div>
            <div v-else class="flex h-full gap-0">
              <div class="w-72 shrink-0 border-r overflow-auto">
                <div
                  v-for="r in forwardRecords"
                  :key="r.id"
                  :class="cn(
                    'flex items-center gap-2 px-3 py-2 border-b border-border/30 cursor-pointer transition-colors text-xs',
                    r.id === selectedForwardId ? 'bg-primary/10' : 'hover:bg-accent/50',
                  )"
                  @click="selectForward(r.id)"
                >
                  <span class="text-[10px] text-muted-foreground font-mono">#{{ r.id }}</span>
                  <Badge
                    variant="outline"
                    :class="cn(
                      'text-[10px] h-5 px-1.5 rounded shrink-0',
                      r.success ? 'border-emerald-500/40 text-emerald-600' : 'border-red-500/40 text-red-600',
                    )"
                  >
                    {{ r.success ? '成功' : '失败' }}
                  </Badge>
                  <span class="font-mono truncate flex-1">{{ r.method }}</span>
                  <span class="text-[10px] text-muted-foreground">{{ r.elapsed_ms != null ? `${r.elapsed_ms}ms` : '--' }}</span>
                  <span class="text-[10px] text-muted-foreground">{{ formatTime(r.timestamp) }}</span>
                </div>
              </div>
              <div class="flex-1 overflow-auto p-3">
                <div v-if="!selectedForward" class="text-sm text-muted-foreground py-8 text-center">选择一个执行记录查看详情</div>
                <div v-else class="space-y-3">
                  <div class="flex items-center gap-2 text-xs text-muted-foreground">
                    <span class="font-semibold text-foreground">{{ selectedForward.method }}</span>
                    <span>→ {{ selectedForward.target }}</span>
                    <span>|</span>
                    <span>{{ selectedForward.elapsed_ms != null ? `${selectedForward.elapsed_ms}ms` : '--' }}</span>
                    <Badge
                      variant="outline"
                      :class="cn('text-[10px]', selectedForward.success ? 'text-emerald-600' : 'text-red-600')"
                    >
                      {{ selectedForward.success ? '成功' : '失败' }}
                    </Badge>
                  </div>
                  <div v-if="selectedForward.request != null">
                    <div class="text-[11px] font-semibold text-muted-foreground mb-1">请求体</div>
                    <pre class="bg-muted/30 rounded-lg p-3 text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap">{{ formatJson(selectedForward.request) }}</pre>
                  </div>
                  <div>
                    <div class="text-[11px] font-semibold text-muted-foreground mb-1">{{ selectedForward.success ? '响应体' : '错误' }}</div>
                    <pre
                      :class="cn(
                        'rounded-lg p-3 text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap',
                        selectedForward.success ? 'bg-muted/30' : 'bg-red-50 dark:bg-red-950/20 text-red-600',
                      )"
                    >{{ formatJson(selectedForward.success ? selectedForward.response : selectedForward.error) }}</pre>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Records footer -->
        <div v-if="activeTab === 'records' && forwardRecords.length > 0" class="px-3 py-2 border-t bg-muted/10 flex shrink-0">
          <Button variant="ghost" size="sm" class="gap-1 text-xs" @click="clearExecutions">
            <Trash2 class="w-3 h-3" /> 清除记录
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
