<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Radio, Play, Send, FolderOpen,
  Pause, X, Trash2, WifiOff,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import * as proxyApi from '@/api/proxy'
import * as protoApi from '@/api/proto'
import { useProxyWebSocket } from '@/composables/useProxyWebSocket'
import { useUserStore } from '@/stores/user'
import type { ProxyStatus } from '@/types'

const userStore = useUserStore()
const { send: sendWs, onMessage: onWsMessage, connect: connectWs } = useProxyWebSocket(
  () => userStore.getToken(),
)

interface ProxyEntry {
  id: number
  method: string
  target: string
  request: unknown
  response: unknown | null
  state: string
  error: string | null
  elapsed_ms: number | null
  timestamp: string
}

const proxyStateLabels: Record<string, string> = {
  'waiting-request': '等待转发',
  'forwarded': '已转发',
  'waiting-response': '等待响应',
  'done': '已完成',
  'error': '错误',
  'dropped': '已丢弃',
}

const proxyStatus = ref<ProxyStatus | null>(null)
const targetAddr = ref('')
const protoDir = ref('')
const protoDirInput = ref('')
const protoMethodCount = ref(0)
const showProtoInput = ref(false)

const proxyEntries = ref<ProxyEntry[]>([])
const selectedProxyId = ref<number | null>(null)
const paused = ref(false)
const activeTab = ref('request')
const proxyReqEditor = ref('{}')
const proxyRespEditor = ref('{}')

const splitRatio = ref(35)
const dragging = ref(false)
const mainAreaRef = ref<HTMLElement | null>(null)

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

function handleProxyMsg(msg: Record<string, unknown>): void {
  const type = msg.type as string
  const id = msg.id as number
  if (type === 'proxy-request') {
    proxyEntries.value = [
      {
        id,
        method: (msg.method as string) || '',
        target: (msg.target as string) || '',
        request: msg.request || {},
        response: null,
        state: 'waiting-request',
        error: null,
        elapsed_ms: null,
        timestamp: (msg.timestamp as string) || new Date().toISOString(),
      },
      ...proxyEntries.value,
    ]
    if (proxyEntries.value.length > 500) proxyEntries.value.pop()
  } else if (type === 'proxy-response') {
    proxyEntries.value = proxyEntries.value.map((e) =>
      e.id === id
        ? {
            ...e,
            response: msg.response || null,
            elapsed_ms: (msg.elapsed_ms as number) || null,
            state: 'waiting-response',
            target: (msg.target as string) || e.target,
          }
        : e,
    )
  } else if (type === 'proxy-done') {
    proxyEntries.value = proxyEntries.value.map((e) =>
      e.id === id ? { ...e, state: msg.dropped ? 'dropped' : 'done' } : e,
    )
  } else if (type === 'proxy-error') {
    proxyEntries.value = proxyEntries.value.map((e) =>
      e.id === id ? { ...e, state: 'error', error: (msg.error as string) || 'Unknown error' } : e,
    )
  }
}

onWsMessage((raw: string) => {
  try { handleProxyMsg(JSON.parse(raw)) } catch { /* ignore */ }
})

const stats = computed(() => {
  const waiting = proxyEntries.value.filter((e) => e.state === 'waiting-request').length
  const done = proxyEntries.value.filter((e) => e.state === 'done').length
  const errors = proxyEntries.value.filter((e) => e.state === 'error').length
  const dropped = proxyEntries.value.filter((e) => e.state === 'dropped').length
  return { waiting, done, errors, dropped, total: proxyEntries.value.length }
})

const selectedEntry = computed(() => proxyEntries.value.find((e) => e.id === selectedProxyId.value) || null)

async function toggleProxy(): Promise<void> {
  try {
    if (proxyStatus.value?.running) {
      await proxyApi.stopProxy()
      if (proxyStatus.value) proxyStatus.value = { ...proxyStatus.value, running: false }
      ElMessage.success('代理已停止')
    } else {
      const r = await proxyApi.startProxy(targetAddr.value || undefined)
      proxyStatus.value = r
      ElMessage.success('代理已启动')
    }
  } catch (e) {
    ElMessage.error(`代理操作失败：${e instanceof Error ? e.message : '网络异常'}`)
  }
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

function proxyForward(id: number): void {
  let modified: unknown = null
  try { modified = JSON.parse(proxyReqEditor.value) } catch { ElMessage.warning('请求体 JSON 格式错误'); return }
  sendWs(JSON.stringify({ type: 'proxy-forward', id, request: modified }))
}
function proxyDrop(id: number): void {
  sendWs(JSON.stringify({ type: 'proxy-drop', id }))
}
function proxySendResponse(id: number): void {
  let modified: unknown = null
  try { modified = JSON.parse(proxyRespEditor.value) } catch { ElMessage.warning('响应体 JSON 格式错误'); return }
  sendWs(JSON.stringify({ type: 'proxy-send-response', id, response: modified }))
}

function clearProxyEntries(): void {
  proxyEntries.value = []
  selectedProxyId.value = null
}
function selectProxy(id: number): void {
  selectedProxyId.value = id
  activeTab.value = 'request'
  const entry = proxyEntries.value.find((e) => e.id === id)
  if (entry) {
    proxyReqEditor.value = JSON.stringify(entry.request || {}, null, 2)
    proxyRespEditor.value = JSON.stringify(entry.response || {}, null, 2)
  }
}

function onMouseMove(e: MouseEvent): void {
  if (!dragging.value || !mainAreaRef.value) return
  const rect = mainAreaRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  let pct = (x / rect.width) * 100
  if (pct < 25) pct = 25
  if (pct > 75) pct = 75
  splitRatio.value = Math.round(pct)
}
function onMouseUp(): void { dragging.value = false }

let pollTimer: ReturnType<typeof setInterval> | undefined

async function loadInitial(): Promise<void> {
  try {
    const s = await proxyApi.getProxyStatus()
    proxyStatus.value = s
    if (s.target) targetAddr.value = s.target
  } catch { /* ignore */ }
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
  pollTimer = setInterval(async () => {
    try {
      const s = await proxyApi.getProxyStatus()
      const prev = proxyStatus.value
      if (!prev || prev.running !== s.running || prev.target !== s.target || prev.pendingCount !== s.pendingCount) {
        proxyStatus.value = s
      }
      if (s.target && s.target !== targetAddr.value) targetAddr.value = s.target
    } catch { /* ignore */ }
  }, 3000)
}

onMounted(() => {
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
  loadInitial()
})
onUnmounted(() => {
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseup', onMouseUp)
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="flex flex-col h-[calc(100vh-6rem)] gap-3">
    <!-- Top Toolbar -->
    <Card class="p-3 flex items-center gap-3 flex-wrap shrink-0">
      <Button
        :variant="proxyStatus?.running ? 'default' : 'outline'"
        size="sm"
        :class="cn('gap-1.5 rounded-lg', proxyStatus?.running && 'bg-emerald-600 hover:bg-emerald-700 text-white')"
        @click="toggleProxy"
      >
        <Radio :class="cn('w-3.5 h-3.5', proxyStatus?.running && 'animate-pulse')" />
        <span v-if="proxyStatus?.running">运行中 :{{ proxyStatus.port || 18924 }}</span>
        <span v-else>启动代理</span>
      </Button>

      <div class="w-px h-6 bg-border/60" />

      <!-- Target Address -->
      <div class="flex items-center gap-1.5">
        <span class="text-xs text-muted-foreground whitespace-nowrap">目标地址</span>
        <Input
          :value="targetAddr"
          class="h-8 w-44 text-xs font-mono"
          placeholder="host:port"
          @input="targetAddr = ($event.target as HTMLInputElement).value"
        />
      </div>

      <!-- Proto Dir -->
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

      <!-- Stats -->
      <div class="ml-auto flex items-center gap-2 text-xs text-muted-foreground">
        <template v-if="stats.total > 0">
          <span class="text-emerald-500">✓{{ stats.done }}</span>
          <span class="text-amber-500">⏳{{ stats.waiting }}</span>
          <span class="text-red-500">✗{{ stats.errors }}</span>
          <span v-if="stats.dropped > 0" class="text-gray-400">⊘{{ stats.dropped }}</span>
          <span class="font-medium">共 {{ stats.total }}</span>
        </template>
      </div>

      <Button variant="ghost" size="sm" class="gap-1 rounded-lg" @click="paused = !paused">
        <Pause class="w-3.5 h-3.5" />
        {{ paused ? '恢复' : '暂停' }}
      </Button>
      <Button variant="ghost" size="sm" class="gap-1 rounded-lg" @click="clearProxyEntries">
        <Trash2 class="w-3.5 h-3.5" />
        清除
      </Button>
    </Card>

    <!-- Main Area -->
    <div ref="mainAreaRef" class="flex-1 flex gap-0 min-h-0 rounded-xl border bg-card overflow-hidden">
      <!-- Left Panel — Proxy List -->
      <div class="flex flex-col overflow-hidden" :style="{ width: splitRatio + '%' }">
        <div class="px-3 py-2 border-b bg-muted/20 flex items-center gap-2 shrink-0">
          <span class="text-xs font-semibold text-muted-foreground">拦截列表</span>
          <span class="text-[10px] text-muted-foreground/50">{{ proxyEntries.length }}条</span>
        </div>
        <div class="flex-1 overflow-auto">
          <div v-if="proxyEntries.length === 0" class="p-6 text-center text-sm text-muted-foreground">
            <WifiOff class="w-8 h-8 mx-auto mb-2 opacity-30" />
            <p class="font-medium">暂无拦截请求</p>
            <p class="text-xs mt-1">启动代理后，将客户端请求指向代理地址</p>
            <div class="mt-4 p-3 mx-auto max-w-sm bg-muted/30 rounded-lg text-xs text-left text-muted-foreground leading-relaxed">
              <p class="font-semibold mb-1.5 text-foreground">使用步骤</p>
              <ol class="list-decimal pl-4 space-y-1">
                <li>启动代理服务，默认监听本地端口</li>
                <li>配置被测客户端指向代理地址</li>
                <li>捕获到请求后，可修改内容并转发至真实服务</li>
                <li>支持丢弃、修改请求/响应及模拟返回</li>
              </ol>
            </div>
          </div>
          <div
            v-for="e in proxyEntries"
            v-else
            :key="e.id"
            :class="cn(
              'flex items-center gap-2 px-3 py-2 border-b border-border/30 cursor-pointer transition-colors text-xs',
              e.id === selectedProxyId
                ? 'bg-primary/10 border-l-2 border-l-primary'
                : 'hover:bg-accent/50 border-l-2 border-l-transparent',
              e.state === 'error' && 'bg-red-50/30 dark:bg-red-950/10',
            )"
            @click="selectProxy(e.id)"
          >
            <span class="text-[10px] text-muted-foreground font-mono w-8 shrink-0">#{{ e.id }}</span>
            <Badge
              variant="outline"
              :class="cn(
                'text-[10px] h-5 px-1.5 rounded shrink-0',
                e.state === 'done' && 'border-emerald-500/40 text-emerald-600',
                e.state === 'error' && 'border-red-500/40 text-red-600',
                e.state === 'dropped' && 'border-gray-400/40 text-gray-500',
                e.state === 'waiting-request' && 'border-amber-500/40 text-amber-600',
                e.state === 'waiting-response' && 'border-blue-500/40 text-blue-600',
              )"
            >
              {{ proxyStateLabels[e.state] || e.state }}
            </Badge>
            <span class="font-mono font-medium truncate flex-1">{{ e.method }}</span>
            <span class="text-[10px] text-cyan-600 dark:text-cyan-400 truncate max-w-[100px]">{{ e.target }}</span>
            <span class="text-[10px] text-muted-foreground shrink-0">{{ formatTime(e.timestamp) }}</span>
          </div>
        </div>
      </div>

      <!-- Resizer -->
      <div
        class="w-1.5 cursor-col-resize hover:bg-primary/30 bg-transparent shrink-0 transition-colors relative group"
        @mousedown="dragging = true"
      >
        <div class="absolute inset-y-0 left-1/2 -translate-x-1/2 w-0.5 bg-border/30 group-hover:bg-border" />
      </div>

      <!-- Right Panel — Detail -->
      <div class="flex-1 flex flex-col overflow-hidden" :style="{ width: 100 - splitRatio + '%' }">
        <!-- Tabs -->
        <div class="px-3 py-2 border-b bg-muted/20 flex items-center gap-1 shrink-0">
          <button
            v-for="tab in (['request', 'response'] as const)"
            :key="tab"
            :class="cn(
              'px-3 py-1 text-xs rounded-md transition-colors font-medium',
              activeTab === tab
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground',
            )"
            @click="activeTab = tab"
          >
            {{ tab === 'request' ? '拦截详情' : '响应详情' }}
          </button>
        </div>

        <!-- Tab Content -->
        <div class="flex-1 overflow-auto p-4">
          <div v-if="!selectedEntry" class="text-center text-sm text-muted-foreground py-12">
            从左侧列表选择一个拦截请求
          </div>
          <div v-else class="space-y-4">
            <!-- Header -->
            <div class="flex items-center gap-2 text-xs">
              <span class="font-semibold text-sm font-mono">{{ selectedEntry.method }}</span>
              <Badge
                variant="outline"
                :class="cn(
                  'text-[10px]',
                  selectedEntry.state === 'done' && 'text-emerald-600',
                  selectedEntry.state === 'error' && 'text-red-600',
                )"
              >
                {{ proxyStateLabels[selectedEntry.state] || selectedEntry.state }}
              </Badge>
              <span class="text-cyan-600 dark:text-cyan-400">→ {{ selectedEntry.target }}</span>
              <span v-if="selectedEntry.elapsed_ms != null" class="text-muted-foreground">{{ Math.round(selectedEntry.elapsed_ms) }}ms</span>
            </div>

            <template v-if="activeTab === 'request'">
              <div>
                <div class="text-[11px] font-semibold text-muted-foreground mb-1.5">请求体</div>
                <pre class="bg-muted/30 rounded-lg p-3 text-xs font-mono overflow-auto max-h-64 whitespace-pre-wrap">{{ formatJson(selectedEntry.request) }}</pre>
              </div>
              <div v-if="selectedEntry.state === 'waiting-request'" class="flex gap-2 mt-2">
                <Button size="sm" class="gap-1 rounded-lg bg-emerald-600 hover:bg-emerald-700" @click="proxyForward(selectedEntry.id)">
                  <Play class="w-3.5 h-3.5" /> 转发
                </Button>
                <Button size="sm" variant="destructive" class="gap-1 rounded-lg" @click="proxyDrop(selectedEntry.id)">
                  <X class="w-3.5 h-3.5" /> 丢弃
                </Button>
              </div>
            </template>

            <template v-if="activeTab === 'response'">
              <div>
                <div class="text-[11px] font-semibold text-muted-foreground mb-1.5">响应体</div>
                <pre v-if="selectedEntry.response" class="bg-muted/30 rounded-lg p-3 text-xs font-mono overflow-auto max-h-64 whitespace-pre-wrap">{{ formatJson(selectedEntry.response) }}</pre>
                <div v-else class="text-sm text-muted-foreground py-4">等待响应...</div>
              </div>
              <div v-if="selectedEntry.state === 'waiting-response'" class="flex gap-2 mt-2">
                <Button size="sm" class="gap-1 rounded-lg" @click="proxySendResponse(selectedEntry.id)">
                  <Send class="w-3.5 h-3.5" /> 发送响应
                </Button>
                <Button size="sm" variant="destructive" class="gap-1 rounded-lg" @click="proxyDrop(selectedEntry.id)">
                  <X class="w-3.5 h-3.5" /> 丢弃
                </Button>
              </div>
            </template>

            <div v-if="selectedEntry.error">
              <div class="text-[11px] font-semibold text-red-500 mb-1.5">错误</div>
              <pre class="bg-red-50 dark:bg-red-950/20 rounded-lg p-3 text-xs font-mono overflow-auto max-h-32 text-red-600 whitespace-pre-wrap">{{ selectedEntry.error }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
