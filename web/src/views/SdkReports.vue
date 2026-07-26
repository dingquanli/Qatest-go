<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import Button from '@/components/ui/Button.vue'
import { RefreshCw, ChevronRight, Copy, Search } from 'lucide-vue-next'
import { getQaReports } from '@/api/sdk'
import type { QaReport } from '@/types'
import { ElMessage } from 'element-plus'

const EVENT_OPTIONS = [
  { value: '', label: '全部事件' },
  { value: 'case_result', label: '用例结果' },
  { value: 'log', label: '日志' },
  { value: 'request', label: '请求 (REQUEST)' },
  { value: 'response', label: '响应 (RESPONSE)' },
  { value: 'error', label: '错误 (ERROR)' },
]

const items = ref<QaReport[]>([])
const total = ref(0)
const loading = ref(false)
const eventFilter = ref('')
const limit = ref(50)
const offset = ref(0)
const selected = ref<QaReport | null>(null)

function fmtTime(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return '—'
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

const eventBadgeClass = (e: string) =>
  ({
    case_result: 'bg-emerald-500/10 text-emerald-600',
    log: 'bg-sky-500/10 text-sky-600',
    request: 'bg-violet-500/10 text-violet-600',
    response: 'bg-blue-500/10 text-blue-600',
    error: 'bg-red-500/10 text-red-600',
  } as Record<string, string>)[e] || 'bg-muted text-muted-foreground'

const resultBadgeClass = (r: string) =>
  r === 'passed' || r === 'success'
    ? 'bg-emerald-500/10 text-emerald-600'
    : r === 'failed' || r === 'error'
      ? 'bg-red-500/10 text-red-600'
      : 'bg-muted text-muted-foreground'

function prettyJson(s: string): string {
  if (!s || s === 'null') return '—'
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

function copy(text: string, label: string) {
  if (!text || text === '—') return
  navigator.clipboard?.writeText(text).then(
    () => ElMessage.success(`${label}已复制`),
    () => ElMessage.warning('复制失败'),
  )
}

const hasMore = computed(() => offset.value + items.value.length < total.value)

async function load(reset = false) {
  if (reset) offset.value = 0
  loading.value = true
  try {
    const res = await getQaReports({
      event: eventFilter.value || undefined,
      limit: limit.value,
      offset: offset.value,
    })
    if (reset) {
      items.value = res.items
    } else {
      items.value = items.value.concat(res.items)
    }
    total.value = res.total
  } catch (e: any) {
    ElMessage.error(e?.message || '加载上报失败')
  } finally {
    loading.value = false
  }
}

function changeEvent() {
  load(true)
}
function loadMore() {
  offset.value += limit.value
  load(false)
}
function openDetail(r: QaReport) {
  selected.value = r
}

onMounted(() => load(true))
</script>

<template>
  <div class="space-y-4">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">SDK 上报</h1>
        <p class="text-sm text-muted-foreground mt-0.5">查看各引擎 SDK 上报的用例结果与 gRPC/API 拦截事件（共 {{ total }} 条）</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative">
          <select
            v-model="eventFilter"
            @change="changeEvent"
            class="h-9 rounded-lg border border-input bg-muted/30 pl-3 pr-8 text-sm focus:outline-none focus:ring-2 focus:ring-primary/30"
          >
            <option v-for="o in EVENT_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </div>
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="load(true)">
          <RefreshCw :class="['w-3.5 h-3.5', loading && 'animate-spin']" /> 刷新
        </Button>
      </div>
    </div>

    <!-- 列表 -->
    <div class="rounded-xl border bg-card overflow-hidden">
      <div class="grid grid-cols-12 px-4 py-2.5 text-xs font-medium text-muted-foreground border-b bg-muted/30">
        <div class="col-span-3">名称</div>
        <div class="col-span-2">事件</div>
        <div class="col-span-1">结果</div>
        <div class="col-span-3">方法 / 来源</div>
        <div class="col-span-3">时间</div>
      </div>

      <div v-if="items.length === 0" class="px-4 py-16 text-center text-sm text-muted-foreground">
        暂无上报数据。请在「协议录制 / 代理拦截」页下载 SDK，并将显示的上报 Token 填入 SDK 配置后上报。
      </div>

      <div
        v-for="r in items"
        :key="r.id"
        class="grid grid-cols-12 px-4 py-3 text-sm border-b last:border-0 hover:bg-accent/40 cursor-pointer transition-colors items-center"
        @click="openDetail(r)"
      >
        <div class="col-span-3 min-w-0 pr-2">
          <p class="font-medium truncate">{{ r.name }}</p>
          <p v-if="r.message" class="text-xs text-muted-foreground truncate">{{ r.message }}</p>
        </div>
        <div class="col-span-2">
          <span :class="['inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium', eventBadgeClass(r.event)]">
            {{ r.event }}
          </span>
        </div>
        <div class="col-span-1">
          <span :class="['inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium', resultBadgeClass(r.result)]">
            {{ r.result || '—' }}
          </span>
        </div>
        <div class="col-span-3 min-w-0 pr-2">
          <p class="truncate font-mono text-xs">{{ r.method || '—' }}</p>
          <p class="text-xs text-muted-foreground truncate">{{ r.source }}</p>
        </div>
        <div class="col-span-3 flex items-center justify-between">
          <span class="text-xs text-muted-foreground">{{ fmtTime(r.timestamp) }}</span>
          <ChevronRight class="w-4 h-4 text-muted-foreground/50" />
        </div>
      </div>

      <div v-if="hasMore" class="px-4 py-3 text-center">
        <Button variant="ghost" size="sm" :disabled="loading" @click="loadMore">加载更多</Button>
      </div>
    </div>

    <!-- 明细抽屉 -->
    <transition name="fade">
      <div v-if="selected" class="fixed inset-0 z-50 flex justify-end" @click.self="selected = null">
        <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" />
        <div class="relative w-full max-w-lg bg-popover border-l shadow-2xl h-full overflow-y-auto animate-slide-in p-5 space-y-4">
          <div class="flex items-start justify-between">
            <div>
              <p class="text-xs text-muted-foreground">上报明细</p>
              <h2 class="text-lg font-semibold truncate">{{ selected.name }}</h2>
            </div>
            <Button variant="ghost" size="sm" @click="selected = null">关闭</Button>
          </div>

          <div class="flex flex-wrap gap-2 text-xs">
            <span :class="['inline-flex items-center rounded-full px-2 py-0.5 font-medium', eventBadgeClass(selected.event)]">{{ selected.event }}</span>
            <span :class="['inline-flex items-center rounded-full px-2 py-0.5 font-medium', resultBadgeClass(selected.result)]">{{ selected.result || '—' }}</span>
            <span class="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-muted-foreground">{{ fmtTime(selected.timestamp) }}</span>
          </div>

          <div class="text-sm text-muted-foreground">上报令牌已脱敏，不在此展示。</div>

          <div v-if="selected.method" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">方法</p>
            <p class="text-sm font-mono break-all">{{ selected.method }}</p>
          </div>
          <div v-if="selected.message" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">消息</p>
            <p class="text-sm">{{ selected.message }}</p>
          </div>
          <div v-if="selected.elapsedMs" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">耗时</p>
            <p class="text-sm">{{ selected.elapsedMs }} ms</p>
          </div>
          <div v-if="selected.seq" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">序号 (seq)</p>
            <p class="text-sm">{{ selected.seq }}</p>
          </div>
          <div v-if="selected.tags && selected.tags !== '{}'" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">标签</p>
            <p class="text-sm">{{ selected.tags }}</p>
          </div>

          <template v-for="field in [
            { key: 'headers', label: '请求头 (headers)' },
            { key: 'reqBody', label: '请求体 (request)' },
            { key: 'respBody', label: '响应体 (response)' },
          ] as const" :key="field.key">
            <div v-if="(selected as any)[field.key] && (selected as any)[field.key] !== 'null'" class="space-y-1">
              <div class="flex items-center justify-between">
                <p class="text-xs font-semibold text-muted-foreground">{{ field.label }}</p>
                <button class="text-xs text-primary hover:underline inline-flex items-center gap-1" @click="copy(prettyJson((selected as any)[field.key]), field.label)">
                  <Copy class="w-3 h-3" /> 复制
                </button>
              </div>
              <pre class="text-xs bg-muted/50 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap break-all">{{ prettyJson((selected as any)[field.key]) }}</pre>
            </div>
          </template>

          <div v-if="selected.errMsg" class="space-y-1">
            <p class="text-xs font-semibold text-muted-foreground">错误 (error)</p>
            <pre class="text-xs bg-red-500/10 text-red-600 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap break-all">{{ selected.errMsg }}</pre>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.animate-slide-in {
  animation: slideIn 0.25s ease;
}
@keyframes slideIn {
  from {
    transform: translateX(100%);
  }
  to {
    transform: translateX(0);
  }
}
</style>
