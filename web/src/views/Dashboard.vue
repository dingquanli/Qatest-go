<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import BaseChart from '@/components/BaseChart.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import { cn } from '@/lib/utils'
import {
  Send,
  FileCheck,
  Bot,
  Activity,
  TrendingUp,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Bug,
  ClipboardCheck,
  BookOpen,
  PieChart,
  AlertCircle,
} from 'lucide-vue-next'
import * as casesApi from '@/api/cases'
import * as bugsApi from '@/api/bugs'
import * as apidefsApi from '@/api/apidefs'
import * as apitestApi from '@/api/apitest'
import * as plansApi from '@/api/plans'
import * as devicesApi from '@/api/devices'
import { formatDateShort, lastNDates } from '@/utils'
import type { EChartsOption } from 'echarts'
import type { CaseExecution, CaseExecStats, Bug as BugType, APIDefinition, TestCase, DeviceInfo } from '@/types'

const C = { primary: '#1a9fff', success: '#10b981', danger: '#ef4444', warning: '#f59e0b', muted: '#8f8f8f' }

const caseExecs = ref<CaseExecution[]>([])
const caseStats = ref<CaseExecStats>({})
const bugsList = ref<BugType[]>([])
const apiDefs = ref<APIDefinition[]>([])
const casesList = ref<TestCase[]>([])
const devicesList = ref<DeviceInfo[]>([])
const apiRequests = ref<any[]>([])
const apiHistory = ref<any[]>([])
const planExecs = ref<any[]>([])
const loading = ref(false)

const dates = lastNDates(14)

const passRate = computed(() => {
  const s = caseStats.value
  const passed = Number(s.passed || 0)
  const total = passed + Number(s.failed || 0) + Number(s.blocked || 0) + Number(s.skipped || 0)
  return total ? ((passed / total) * 100).toFixed(1) : '0.0'
})
const todayStr = new Date().toISOString().slice(0, 10)
const todayExecs = computed(() => caseExecs.value.filter((e) => formatDateShort((e as any).executedAt) === todayStr))
const todayFailures = computed(() => todayExecs.value.filter((e) => e.result === 'failed' || e.result === 'blocked'))
const unresolvedBugs = computed(() => bugsList.value.filter((b) => b.status !== 'resolved' && b.status !== 'closed'))
const apiSuccessRate = computed(() =>
  apiHistory.value.length > 0
    ? Math.round(apiHistory.value.filter((h) => h.status >= 200 && h.status < 300).length / apiHistory.value.length * 100)
    : 0,
)
const planProgressList = computed(() => {
  const map = new Map<string, any>()
  for (const pe of planExecs.value) {
    const id = pe.planId || pe.id
    if (!map.has(id)) map.set(id, { id, name: pe.planName || pe.name || id, total: 0, done: 0, passed: 0, failed: 0, blocked: 0 })
    const p = map.get(id)
    p.total += 1
    if (pe.result && pe.result !== 'pending') p.done += 1
    if (pe.result === 'passed') p.passed += 1
    if (pe.result === 'failed') p.failed += 1
    if (pe.result === 'blocked') p.blocked += 1
  }
  return Array.from(map.values())
    .map((p) => ({ ...p, pct: p.total > 0 ? Math.round(p.done / p.total * 100) : 0 }))
    .slice(0, 6)
})

const trendOption = computed<EChartsOption>(() => {
  const countMap: Record<string, number> = {}
  const rateMap: Record<string, { pass: number; total: number }> = {}
  for (const d of dates) {
    countMap[d] = 0
    rateMap[d] = { pass: 0, total: 0 }
  }
  for (const e of caseExecs.value) {
    const d = formatDateShort((e as any).executedAt)
    if (d in countMap) {
      countMap[d] += 1
      rateMap[d].total += 1
      if (e.result === 'passed') rateMap[d].pass += 1
    }
  }
  return {
    tooltip: { trigger: 'axis' },
    legend: { data: ['执行数', '通过率%'], textStyle: { color: C.muted } },
    grid: { left: 40, right: 40, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: dates, axisLine: { lineStyle: { color: C.muted } }, axisLabel: { color: C.muted } },
    yAxis: [
      { type: 'value', axisLine: { lineStyle: { color: C.muted } }, splitLine: { lineStyle: { color: '#1f1f1f' } }, axisLabel: { color: C.muted } },
      { type: 'value', max: 100, axisLabel: { formatter: '{value}%', color: C.muted }, splitLine: { show: false } },
    ],
    series: [
      { name: '执行数', type: 'line', smooth: true, areaStyle: { color: 'rgba(26,159,255,0.18)' }, itemStyle: { color: C.primary }, data: dates.map((d) => countMap[d]) },
      { name: '通过率%', type: 'line', yAxisIndex: 1, smooth: true, itemStyle: { color: C.success }, data: dates.map((d) => (rateMap[d].total ? +((rateMap[d].pass / rateMap[d].total) * 100).toFixed(1) : 0)) },
    ],
  }
})
const donutOption = computed<EChartsOption>(() => {
  const s = caseStats.value
  const data = [
    { name: '通过', value: Number(s.passed || 0), itemStyle: { color: C.success } },
    { name: '失败', value: Number(s.failed || 0), itemStyle: { color: C.danger } },
    { name: '阻塞', value: Number(s.blocked || 0), itemStyle: { color: C.warning } },
    { name: '跳过', value: Number(s.skipped || 0), itemStyle: { color: C.muted } },
  ]
  const pr = Number(passRate.value)
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    series: [{ type: 'pie', radius: ['55%', '75%'], center: ['50%', '50%'], label: { show: true, position: 'center', formatter: `${pr}%\n通过率`, fontSize: 20, fontWeight: 'bold', color: C.success }, labelLine: { show: false }, data }],
  }
})
const bugTrendOption = computed<EChartsOption>(() => {
  const newMap: Record<string, number> = {}
  const resolvedMap: Record<string, number> = {}
  for (const d of dates) {
    newMap[d] = 0
    resolvedMap[d] = 0
  }
  for (const b of bugsList.value) {
    const d = formatDateShort(b.createdAt)
    if (d in newMap) newMap[d] += 1
    if (b.status === 'resolved' || b.status === 'closed') {
      const rd = formatDateShort(b.updatedAt)
      if (rd in resolvedMap) resolvedMap[rd] += 1
    }
  }
  return {
    tooltip: { trigger: 'axis' },
    legend: { data: ['新建', '解决'], textStyle: { color: C.muted } },
    grid: { left: 40, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: dates, axisLine: { lineStyle: { color: C.muted } }, axisLabel: { color: C.muted } },
    yAxis: { type: 'value', axisLine: { lineStyle: { color: C.muted } }, splitLine: { lineStyle: { color: '#1f1f1f' } }, axisLabel: { color: C.muted } },
    series: [
      { name: '新建', type: 'bar', itemStyle: { color: C.danger }, data: dates.map((d) => newMap[d]) },
      { name: '解决', type: 'bar', itemStyle: { color: C.success }, data: dates.map((d) => resolvedMap[d]) },
    ],
  }
})
const methodDistOption = computed<EChartsOption>(() => {
  const count: Record<string, number> = {}
  for (const d of apiDefs.value) {
    const m = (d.method || 'other').toUpperCase()
    count[m] = (count[m] || 0) + 1
  }
  const palette = [C.primary, C.success, C.warning, C.danger, C.muted, '#8b5cf6']
  const data = Object.entries(count).map(([name, value], i) => ({ name, value, itemStyle: { color: palette[i % palette.length] } }))
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    series: [{ type: 'pie', radius: ['40%', '68%'], center: ['50%', '50%'], data }],
  }
})

const statCards = computed(() => [
  { title: '接口总数', value: apiRequests.value.length, suffix: '个', change: `${apiHistory.value.length} 次调用`, icon: Send, color: 'text-blue-600', bg: 'bg-blue-50 dark:bg-blue-900/20' },
  { title: '接口成功率', value: apiSuccessRate.value, suffix: '%', change: apiHistory.value.length > 0 ? '平均响应' : '暂无数据', icon: TrendingUp, color: 'text-emerald-600', bg: 'bg-emerald-50 dark:bg-emerald-900/20' },
  { title: '用例总数', value: casesList.value.length, suffix: '条', change: `通过率 ${passRate.value}%`, icon: FileCheck, color: 'text-violet-600', bg: 'bg-violet-50 dark:bg-violet-900/20' },
  { title: '今日执行', value: todayExecs.value.length, suffix: '次', change: todayFailures.value.length > 0 ? `${todayFailures.value.length} 次异常` : '全部通过', icon: Activity, color: todayFailures.value.length > 0 ? 'text-amber-600' : 'text-emerald-600', bg: todayFailures.value.length > 0 ? 'bg-amber-50 dark:bg-amber-900/20' : 'bg-emerald-50 dark:bg-emerald-900/20' },
  { title: '缺陷总数', value: bugsList.value.length, suffix: '个', change: `${unresolvedBugs.value.length} 未解决`, icon: Bug, color: 'text-red-600', bg: 'bg-red-50 dark:bg-red-900/20' },
  { title: '测试计划', value: planProgressList.value.length, suffix: '个', change: '进行中', icon: ClipboardCheck, color: 'text-indigo-600', bg: 'bg-indigo-50 dark:bg-indigo-900/20' },
])

const overviewItems = computed(() => [
  { label: '测试计划', count: planProgressList.value.length, icon: ClipboardCheck, color: 'text-indigo-500' },
  { label: '测试用例', count: casesList.value.length, icon: FileCheck, color: 'text-violet-500' },
  { label: '接口定义', count: apiDefs.value.length, icon: BookOpen, color: 'text-amber-500' },
  { label: '接口测试', count: apiRequests.value.length, icon: Send, color: 'text-blue-500' },
  { label: '自动化平台', count: devicesList.value.length, icon: Bot, color: 'text-emerald-500' },
  { label: '缺陷管理', count: unresolvedBugs.value.length, icon: Bug, color: 'text-red-500' },
])

const donutSummary = computed(() => {
  const s = caseStats.value
  const total = Number(s.passed || 0) + Number(s.failed || 0) + Number(s.blocked || 0) + Number(s.skipped || 0)
  const passed = Number(s.passed || 0)
  const failed = Number(s.failed || 0)
  const blocked = Number(s.blocked || 0)
  return { total, passed, failed, blocked, passPct: total ? Math.round((passed / total) * 100) : 0, failPct: total ? Math.round((failed / total) * 100) : 0, blockPct: total ? Math.round((blocked / total) * 100) : 0 }
})

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [execs, stats, bugs, defs, cs, devs, reqs, hist, pes] = await Promise.all([
      casesApi.getCaseExecutions(),
      casesApi.getCaseExecutionsStats(),
      bugsApi.getBugs(),
      apidefsApi.getApiDefinitions(),
      casesApi.getCases(),
      devicesApi.getDevices().catch(() => [] as DeviceInfo[]),
      apitestApi.getApiRequests().catch(() => [] as any[]),
      apitestApi.getApiHistory().catch(() => [] as any[]),
      plansApi.getPlanExecutions().catch(() => [] as any[]),
    ])
    caseExecs.value = execs
    caseStats.value = stats
    bugsList.value = bugs
    apiDefs.value = defs
    casesList.value = cs
    devicesList.value = devs
    apiRequests.value = reqs
    apiHistory.value = hist
    planExecs.value = pes
  } catch (err) {
    ElMessage.error((err as Error).message || '加载仪表盘失败')
  } finally {
    loading.value = false
  }
}
onMounted(loadAll)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold tracking-tight">工作台</h1>
      <p class="text-muted-foreground mt-1">Qatest 全流程测试平台总览</p>
    </div>

    <!-- Stat Cards -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Card v-for="stat in statCards" :key="stat.title" class="card-hover cursor-pointer">
        <CardContent class="p-6">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-muted-foreground">{{ stat.title }}</p>
              <div class="flex items-baseline gap-1 mt-2">
                <span class="text-3xl font-bold">{{ stat.value }}</span>
                <span class="text-sm text-muted-foreground">{{ stat.suffix }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-1">{{ stat.change }}</p>
            </div>
            <div :class="cn(stat.bg, 'p-3 rounded-xl')">
              <component :is="stat.icon" :class="cn('w-6 h-6', stat.color)" />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Execution Overview -->
    <Card v-if="caseExecs.length > 0">
      <CardHeader class="pb-3">
        <CardTitle class="text-base flex items-center gap-2"><PieChart class="w-4 h-4 text-primary" /> 执行结果总览</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex items-center gap-6 flex-wrap">
          <div class="w-32 h-32 shrink-0"><BaseChart :option="donutOption" height="128px" /></div>
          <div class="flex-1 grid grid-cols-2 gap-4 min-w-[240px]">
            <div class="flex items-center gap-3 p-3 rounded-lg bg-emerald-50 dark:bg-emerald-900/10 border border-emerald-200 dark:border-emerald-900/30">
              <CheckCircle2 class="w-5 h-5 text-emerald-500" />
              <div><p class="text-xl font-bold text-emerald-600">{{ donutSummary.passed }}</p><p class="text-xs text-emerald-600/70">通过 ({{ donutSummary.passPct }}%)</p></div>
            </div>
            <div class="flex items-center gap-3 p-3 rounded-lg bg-red-50 dark:bg-red-900/10 border border-red-200 dark:border-red-900/30">
              <XCircle class="w-5 h-5 text-red-500" />
              <div><p class="text-xl font-bold text-red-600">{{ donutSummary.failed }}</p><p class="text-xs text-red-600/70">失败 ({{ donutSummary.failPct }}%)</p></div>
            </div>
            <div class="flex items-center gap-3 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-900/30">
              <AlertCircle class="w-5 h-5 text-amber-500" />
              <div><p class="text-xl font-bold text-amber-600">{{ donutSummary.blocked }}</p><p class="text-xs text-amber-600/70">阻塞 ({{ donutSummary.blockPct }}%)</p></div>
            </div>
            <div class="flex items-center gap-3 p-3 rounded-lg bg-muted/50 border border-border">
              <Activity class="w-5 h-5 text-muted-foreground" />
              <div><p class="text-xl font-bold">{{ donutSummary.total }}</p><p class="text-xs text-muted-foreground">总执行次数</p></div>
            </div>
          </div>
          <div class="w-full max-w-[200px] shrink-0 space-y-1">
            <div class="h-4 rounded-full overflow-hidden flex">
              <div v-if="donutSummary.passPct > 0" class="bg-emerald-500 transition-all duration-500" :style="{ width: donutSummary.passPct + '%' }" />
              <div v-if="donutSummary.failPct > 0" class="bg-red-500 transition-all duration-500" :style="{ width: donutSummary.failPct + '%' }" />
              <div v-if="donutSummary.blockPct > 0" class="bg-amber-500 transition-all duration-500" :style="{ width: donutSummary.blockPct + '%' }" />
            </div>
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>{{ donutSummary.passPct }}% 通过</span>
              <span>{{ 100 - donutSummary.passPct }}% 未通过</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Charts Row -->
    <div class="grid gap-6 lg:grid-cols-3">
      <Card class="lg:col-span-2">
        <CardHeader><CardTitle class="text-base">近14日执行趋势</CardTitle></CardHeader>
        <CardContent>
          <BaseChart :option="trendOption" height="260px" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-base">平台概览</CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <div v-for="item in overviewItems" :key="item.label" class="flex items-center justify-between rounded-lg px-2 py-1.5 -mx-2 hover:bg-accent/50 transition-colors">
            <div class="flex items-center gap-2"><component :is="item.icon" :class="cn('w-4 h-4', item.color)" /><span class="text-sm">{{ item.label }}</span></div>
            <Badge variant="secondary">{{ item.count }}</Badge>
          </div>
          <div class="border-t pt-3 space-y-2">
            <p class="text-xs text-muted-foreground font-medium">最近执行</p>
            <div v-for="ex in caseExecs.slice(0, 4)" :key="ex.id" class="flex items-center justify-between text-xs">
              <div class="flex items-center gap-1.5">
                <CheckCircle2 v-if="ex.result === 'passed'" class="w-3 h-3 text-emerald-500" />
                <XCircle v-else-if="ex.result === 'failed'" class="w-3 h-3 text-red-500" />
                <AlertTriangle v-else class="w-3 h-3 text-amber-500" />
                <span class="truncate max-w-[140px]">{{ (ex as any).caseName }}</span>
              </div>
              <span class="text-muted-foreground">{{ (ex as any).executor }}</span>
            </div>
            <p v-if="caseExecs.length === 0" class="text-xs text-muted-foreground text-center py-2">暂无执行记录</p>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Alerts -->
    <Card v-if="todayFailures.length > 0">
      <CardHeader class="pb-3">
        <CardTitle class="text-base flex items-center gap-2"><AlertTriangle class="w-4 h-4 text-amber-500" /> 今日异常</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="space-y-2">
          <div v-for="ex in todayFailures.slice(0, 5)" :key="ex.id" class="flex items-center justify-between p-3 rounded-lg bg-red-50 dark:bg-red-900/10 border border-red-200 dark:border-red-900/30">
            <div class="flex items-center gap-2">
              <XCircle class="w-4 h-4 text-red-500" />
              <div>
                <p class="text-sm font-medium">{{ (ex as any).caseName }}</p>
                <p class="text-xs text-muted-foreground">{{ (ex as any).executor }} · {{ (ex as any).remark || '执行失败' }}</p>
              </div>
            </div>
            <span class="text-xs text-muted-foreground">{{ new Date((ex as any).executedAt).toLocaleTimeString() }}</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Bug Trend + API Distribution -->
    <div class="grid gap-6 lg:grid-cols-3">
      <Card class="lg:col-span-2">
        <CardHeader><CardTitle class="text-base flex items-center gap-2"><Bug class="w-4 h-4 text-red-500" /> 缺陷趋势</CardTitle></CardHeader>
        <CardContent><BaseChart :option="bugTrendOption" height="260px" /></CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle class="text-base flex items-center gap-2"><PieChart class="w-4 h-4 text-blue-500" /> 接口方法分布</CardTitle></CardHeader>
        <CardContent><BaseChart :option="methodDistOption" height="260px" /></CardContent>
      </Card>
    </div>

    <!-- Plan Progress -->
    <Card v-if="planProgressList.length > 0">
      <CardHeader><CardTitle class="text-base flex items-center gap-2"><ClipboardCheck class="w-4 h-4 text-indigo-500" /> 计划执行进度</CardTitle></CardHeader>
      <CardContent>
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <div v-for="p in planProgressList" :key="p.id" class="p-4 rounded-lg border space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium truncate">{{ p.name }}</span>
            </div>
            <div class="h-2 bg-muted rounded-full overflow-hidden flex">
              <div v-if="p.passed > 0" class="bg-emerald-500 transition-all duration-300" :style="{ width: (p.passed / p.total * 100) + '%' }" />
              <div v-if="p.failed > 0" class="bg-red-500 transition-all duration-300" :style="{ width: (p.failed / p.total * 100) + '%' }" />
              <div v-if="p.blocked > 0" class="bg-amber-500 transition-all duration-300" :style="{ width: (p.blocked / p.total * 100) + '%' }" />
            </div>
            <div class="flex items-center justify-between text-xs text-muted-foreground">
              <div class="flex items-center gap-2">
                <span class="flex items-center gap-1"><CheckCircle2 class="w-3 h-3 text-emerald-500" />{{ p.passed }}</span>
                <span v-if="p.failed > 0" class="flex items-center gap-1"><XCircle class="w-3 h-3 text-red-500" />{{ p.failed }}</span>
                <span v-if="p.blocked > 0" class="flex items-center gap-1"><AlertTriangle class="w-3 h-3 text-amber-500" />{{ p.blocked }}</span>
              </div>
              <span>{{ p.done }}/{{ p.total }} ({{ p.pct }}%)</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
