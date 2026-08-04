<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as XLSX from 'xlsx'
import {
  Plus, Play, Trash2, Edit3, CheckCircle2, XCircle, AlertCircle,
  Minus, SkipForward, FileCheck, BarChart3, Clock, ChevronDown,
  ChevronRight, FolderOpen, Search, ArrowLeft, Save, Bug,
  Download, Upload,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Dialog from '@/components/ui/Dialog.vue'
import { cn } from '@/lib/utils'
import { safeParseJSON, formatDate, downloadText } from '@/utils'
import * as casesApi from '@/api/cases'
import * as bugsApi from '@/api/bugs'
import * as scriptsApi from '@/api/scripts'
import type { TestCase, CaseStep, CaseModule, CaseExecution, Script, Priority, ExecResult } from '@/types'

type CaseType = string
type StepResult = ExecResult
type TabKey = 'cases' | 'table' | 'xmind' | 'execHistory' | 'execute'

// ==================== Config Maps ====================

const priorityCfg: Record<Priority, { label: string; cls: string }> = {
  P0: { label: 'P0 致命', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
  P1: { label: 'P1 严重', cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  P2: { label: 'P2 一般', cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  P3: { label: 'P3 轻微', cls: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400' },
}

const typeCfg: Record<string, { label: string; color: string }> = {
  functional: { label: '功能测试', color: 'text-blue-600' },
  smoke: { label: '冒烟测试', color: 'text-emerald-600' },
  performance: { label: '性能测试', color: 'text-amber-600' },
  security: { label: '安全测试', color: 'text-red-600' },
  compatibility: { label: '兼容测试', color: 'text-violet-600' },
}
function typeLabel(t: string): string {
  return (typeCfg[t] || { label: t || '未分类' }).label
}
function typeColor(t: string): string {
  return (typeCfg[t] || { color: 'text-muted-foreground' }).color
}

const stepResultCfg: Record<StepResult, { label: string; icon: unknown; color: string }> = {
  passed: { label: '通过', icon: CheckCircle2, color: 'text-emerald-500' },
  failed: { label: '失败', icon: XCircle, color: 'text-red-500' },
  blocked: { label: '阻塞', icon: AlertCircle, color: 'text-amber-500' },
  skipped: { label: '跳过', icon: SkipForward, color: 'text-gray-400' },
  pending: { label: '待测', icon: Minus, color: 'text-gray-300' },
}

// ==================== State ====================

const tab = ref<TabKey>('cases')
const cases = ref<TestCase[]>([])
const modules = ref<CaseModule[]>([])
const executions = ref<CaseExecution[]>([])
const selectedCaseId = ref<string | null>(null)
const searchTerm = ref('')
const filterPriority = ref('all')
const filterType = ref('all')
const filterStatus = ref('all')
const expandedModules = ref<Set<string>>(new Set())
const editingCase = ref<EditingCase | null>(null)
const isCreating = ref(false)
const importMsg = ref('')
const importFileRef = ref<HTMLInputElement | null>(null)
const importXmindRef = ref<HTMLInputElement | null>(null)

// Execution state
const executingCase = ref<TestCase | null>(null)
const execSteps = ref<{ action: string; expected: string; actual: string; result: StepResult }[]>([])
const execStartTime = ref(0)
const quickExecCaseId = ref<string | null>(null)

// Batch execution state
const batchMode = ref(false)
const batchSelected = ref<Set<string>>(new Set())
const batchResults = ref<Record<string, StepResult>>({})

// Selected execution detail
const selectedExecId = ref<string | null>(null)

// Bug report modal
const bugModalCase = ref<TestCase | null>(null)
const bugForm = ref({ title: '', description: '', steps: '', expected: '', relatedCaseId: '', moduleId: '' })

interface EditingCase {
  id?: string
  name: string
  moduleId: string
  priority: Priority
  type: CaseType
  precondition: string
  steps: { action: string; expected: string }[]
  assignee: string
  status: string
  tags: string
  scriptId?: string
}

// ==================== Derived ====================

const selectedCase = computed(() => cases.value.find((c) => c.id === selectedCaseId.value) || null)
const selectedExec = computed(() => executions.value.find((e) => e.id === selectedExecId.value) || null)

function caseSteps(c: TestCase): CaseStep[] {
  return safeParseJSON<CaseStep[]>(c.steps, [])
}
function caseStepCount(c: TestCase): number {
  return caseSteps(c).length
}
function lastExecOf(c: TestCase): CaseExecution | undefined {
  return executions.value.filter((e) => e.caseId === c.id)[0]
}
function moduleName(id: string): string {
  return modules.value.find((m) => m.id === id)?.name || '未分类'
}
function modulePassedCount(modId: string, modCases: TestCase[]): number {
  return modCases.filter((c) => {
    const ex = executions.value.filter((e) => e.caseId === c.id)
    return ex.length > 0 && ex[0].result === 'passed'
  }).length
}

const filteredCases = computed(() =>
  cases.value.filter((c) => {
    if (searchTerm.value && !c.name.toLowerCase().includes(searchTerm.value.toLowerCase()) && !c.assignee.toLowerCase().includes(searchTerm.value.toLowerCase())) return false
    if (filterPriority.value !== 'all' && c.priority !== filterPriority.value) return false
    if (filterType.value !== 'all' && c.type !== filterType.value) return false
    if (filterStatus.value !== 'all' && c.status !== filterStatus.value) return false
    return true
  }),
)

const groupedCases = computed(() => {
  const map: Record<string, TestCase[]> = {}
  filteredCases.value.forEach((c) => {
    const mid = c.moduleId || 'uncategorized'
    if (!map[mid]) map[mid] = []
    map[mid].push(c)
  })
  return map
})

const dailyStats = computed(() => {
  const days: { date: string; caseRuns: number; passRate: number }[] = []
  const today = new Date()
  for (let i = 13; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    const dateStr = d.toISOString().slice(0, 10)
    const dayExecs = executions.value.filter((e) => (e.executedAt || '').slice(0, 10) === dateStr)
    const total = dayExecs.length
    const passed = dayExecs.filter((e) => e.result === 'passed').length
    const passRate = total === 0 ? 0 : Math.round((passed / total) * 100)
    days.push({ date: dateStr, caseRuns: total, passRate })
  }
  return days
})
const chartMax = computed(() => Math.max(1, ...dailyStats.value.map((d) => d.caseRuns)))

const tabs = computed(() => {
  const base: { key: TabKey; label: string; icon: unknown }[] = [
    { key: 'cases', label: '用例列表', icon: FileCheck },
    { key: 'table', label: '表格用例', icon: Download },
    { key: 'xmind', label: '思维导图', icon: FolderOpen },
    { key: 'execHistory', label: '执行记录', icon: BarChart3 },
  ]
  if (executingCase.value) base.push({ key: 'execute', label: '测试执行', icon: Play })
  return base
})
const stepResults: StepResult[] = ['passed', 'failed', 'blocked', 'skipped']

// 自动化脚本列表（用于把用例关联到脚本，供测试计划「自动执行」模式派发）
const scripts = ref<Script[]>([])

// ==================== Handlers ====================

function toggleModule(id: string): void {
  const next = new Set(expandedModules.value)
  next.has(id) ? next.delete(id) : next.add(id)
  expandedModules.value = next
}

function blankStep(): { action: string; expected: string } {
  return { action: '', expected: '' }
}

function startCreateCase(moduleId?: string): void {
  editingCase.value = {
    id: undefined,
    name: '',
    moduleId: moduleId || modules.value[0]?.id || '',
    priority: 'P1',
    type: 'functional',
    precondition: '',
    steps: [blankStep()],
    assignee: '',
    status: 'draft',
    tags: '[]',
    scriptId: '',
  }
  isCreating.value = true
}

function startEditCase(tc: TestCase): void {
  editingCase.value = {
    id: tc.id,
    name: tc.name,
    moduleId: tc.moduleId,
    priority: tc.priority,
    type: tc.type,
    precondition: tc.precondition,
    steps: safeParseJSON<{ action: string; expected: string }[]>(tc.steps, [blankStep()]),
    assignee: tc.assignee,
    status: tc.status,
    tags: tc.tags,
    scriptId: tc.scriptId || '',
  }
  isCreating.value = false
}

async function saveEditingCase(): Promise<void> {
  const e = editingCase.value
  if (!e) return
  if (!e.name.trim()) { ElMessage.warning('请输入用例名称'); return }
  const payload: Partial<TestCase> = {
    name: e.name,
    moduleId: e.moduleId,
    priority: e.priority,
    type: e.type,
    precondition: e.precondition,
    steps: JSON.stringify(e.steps),
    assignee: e.assignee,
    status: e.status,
    tags: e.tags || '[]',
    scriptId: e.scriptId || '',
  }
  try {
    if (e.id) await casesApi.updateCase(e.id, payload)
    else await casesApi.createCase(payload)
    ElMessage.success('已保存')
    editingCase.value = null
    isCreating.value = false
    await loadCases()
  } catch (err) {
    ElMessage.error((err as Error).message || '保存失败')
  }
}

async function handleDeleteCase(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除此用例？', '提示', { type: 'warning' })
    await casesApi.deleteCase(id)
    if (selectedCaseId.value === id) selectedCaseId.value = null
    ElMessage.success('已删除')
    await loadCases()
  } catch { /* cancelled */ }
}

// ---- Module CRUD ----
async function addModule(): Promise<void> {
  try {
    const { value } = await ElMessageBox.prompt('模块名称', '新建模块', { inputValue: '' })
    if (!value?.trim()) return
    await casesApi.createCaseModule({ name: value.trim(), sortOrder: modules.value.length })
    ElMessage.success('已创建模块')
    await loadCases()
  } catch { /* cancelled */ }
}

async function renameModule(modId: string): Promise<void> {
  const mod = modules.value.find((m) => m.id === modId)
  if (!mod) return
  try {
    const { value } = await ElMessageBox.prompt('模块名称', '重命名模块', { inputValue: mod.name })
    if (!value?.trim()) return
    await casesApi.updateCaseModule(modId, { name: value.trim() })
    await loadCases()
  } catch { /* cancelled */ }
}

async function deleteModule(modId: string): Promise<void> {
  try {
    await ElMessageBox.confirm('删除模块将把其中的用例移至「未分类」，确定删除？', '提示', { type: 'warning' })
    const moved = cases.value.filter((c) => c.moduleId === modId)
    for (const c of moved) await casesApi.updateCase(c.id, { ...c, moduleId: '' })
    await casesApi.deleteCaseModule(modId)
    ElMessage.success('已删除模块')
    await loadCases()
  } catch { /* cancelled */ }
}

// Quick execute
async function quickExecute(tc: TestCase, result: 'passed' | 'failed' | 'blocked'): Promise<void> {
  try {
    await casesApi.createCaseExecution({
      caseId: tc.id,
      caseName: tc.name,
      executor: '当前用户',
      result,
      steps: JSON.stringify(caseSteps(tc).map((s) => ({ ...s, result }))),
      duration: 0,
      remark: '',
    })
    ElMessage.success('已记录执行')
    await loadExecutions()
    quickExecCaseId.value = null
  } catch (err) {
    ElMessage.error((err as Error).message || '操作失败')
  }
}

// Batch execute
function toggleBatchCase(caseId: string): void {
  const next = new Set(batchSelected.value)
  next.has(caseId) ? next.delete(caseId) : next.add(caseId)
  batchSelected.value = next
}
function batchSetResult(result: StepResult): void {
  const updated: Record<string, StepResult> = { ...batchResults.value }
  batchSelected.value.forEach((id) => { updated[id] = result })
  batchResults.value = updated
}
function batchResultOf(id: string): StepResult {
  return batchResults.value[id] || 'pending'
}
async function submitBatchExecution(): Promise<void> {
  for (const caseId of batchSelected.value) {
    const tc = cases.value.find((c) => c.id === caseId)
    if (!tc) continue
    const result = batchResults.value[caseId] || 'pending'
    if (result === 'pending') continue
    await casesApi.createCaseExecution({
      caseId: tc.id,
      caseName: tc.name,
      executor: '当前用户',
      result: result as 'passed' | 'failed' | 'blocked',
      steps: JSON.stringify(caseSteps(tc).map((s) => ({ ...s, result }))),
      duration: 0,
      remark: '',
    })
  }
  ElMessage.success('已提交批量执行')
  await loadExecutions()
  batchSelected.value = new Set()
  batchResults.value = {}
  batchMode.value = false
}

// Editor steps
function addStep(): void {
  if (!editingCase.value) return
  editingCase.value.steps.push(blankStep())
}
function removeStep(idx: number): void {
  if (!editingCase.value || editingCase.value.steps.length <= 1) return
  editingCase.value.steps.splice(idx, 1)
}

// Execution
function startExecution(tc: TestCase): void {
  executingCase.value = tc
  execSteps.value = caseSteps(tc).map((s) => ({ action: s.action, expected: s.expected, actual: s.actual || '', result: 'pending' }))
  execStartTime.value = Date.now()
  tab.value = 'execute'
}
function markStepResult(stepId: number, result: StepResult): void {
  execSteps.value = execSteps.value.map((s, i) => (i === stepId ? { ...s, result } : s))
}
function updateStepActual(stepId: number, actual: string): void {
  execSteps.value = execSteps.value.map((s, i) => (i === stepId ? { ...s, actual } : s))
}
async function submitExecution(): Promise<void> {
  if (!executingCase.value) return
  if (execSteps.value.some((s) => s.result === 'pending')) { ElMessage.warning('还有步骤未标记结果，请全部标记后再提交'); return }
  const caseResult: StepResult = execSteps.value.some((s) => s.result === 'failed') ? 'failed'
    : execSteps.value.some((s) => s.result === 'blocked') ? 'blocked'
    : execSteps.value.every((s) => s.result === 'skipped') ? 'skipped' : 'passed'
  try {
    await casesApi.createCaseExecution({
      caseId: executingCase.value.id,
      caseName: executingCase.value.name,
      executor: '当前用户',
      result: caseResult,
      steps: JSON.stringify(execSteps.value),
      duration: Math.round((Date.now() - execStartTime.value) / 1000),
      remark: '',
    })
    ElMessage.success('执行结果已提交')
    await loadExecutions()
    executingCase.value = null
    execSteps.value = []
    tab.value = 'cases'
  } catch (err) {
    ElMessage.error((err as Error).message || '提交失败')
  }
}

// Bug report
function openBugModal(tc: TestCase): void {
  bugModalCase.value = tc
  bugForm.value = {
    title: `[用例] ${tc.name}`,
    description: `来自用例平台: ${tc.name}`,
    steps: caseSteps(tc).map((s, i) => `${i + 1}. ${s.action}`).join('\n'),
    expected: caseSteps(tc).slice(-1)[0]?.expected || '',
    relatedCaseId: tc.id,
    moduleId: tc.moduleId,
  }
}
async function submitBug(): Promise<void> {
  if (!bugForm.value.title.trim()) { ElMessage.warning('请输入缺陷标题'); return }
  try {
    await bugsApi.createBug({
      title: bugForm.value.title,
      description: bugForm.value.description,
      steps: bugForm.value.steps,
      expected: bugForm.value.expected,
      severity: 'major',
      priority: 'P2',
      status: 'open',
      assignee: '',
      reporter: '当前用户',
      module: moduleName(bugForm.value.moduleId),
      env: '',
      actual: '',
      tags: '[]',
      relatedCaseId: bugForm.value.relatedCaseId,
    })
    ElMessage.success('已记录为缺陷')
    bugModalCase.value = null
  } catch (err) {
    ElMessage.error((err as Error).message || '提交失败')
  }
}

// ==================== Table / XMind ====================

function exportTable(): void {
  const header = ['用例名称', '优先级', '类型', '模块', '负责人', '步骤数', '状态', '最近执行']
  const rows = cases.value.map((tc) => {
    const le = lastExecOf(tc)
    return [
      tc.name, tc.priority, typeLabel(tc.type), moduleName(tc.moduleId), tc.assignee || '未指派',
      caseStepCount(tc), tc.status === 'active' ? '启用' : '草稿',
      le ? (le.result === 'passed' ? '通过' : le.result === 'failed' ? '失败' : '阻塞') : '未执行',
    ]
  })
  const csv = [header, ...rows].map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n')
  downloadText(`cases-${Date.now()}.csv`, csv, 'text/csv')
  ElMessage.success('已导出')
}

function exportXmind(): void {
  downloadText(`cases-${Date.now()}.json`, JSON.stringify(cases.value, null, 2), 'application/json')
  ElMessage.success('已导出')
}

async function onTableImport(e: Event): Promise<void> {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  try {
    const buf = await file.arrayBuffer()
    const wb = XLSX.read(buf, { type: 'array' })
    const sheet = wb.Sheets[wb.SheetNames[0]]
    const rows = XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet, { defval: '' })
    const mapped: Partial<TestCase>[] = rows.map((r) => ({
      name: String(r['名称'] ?? r['name'] ?? ''),
      moduleId: modules.value[0]?.id || '',
      priority: (String(r['优先级'] ?? r['priority'] ?? 'P2') as Priority),
      type: String(r['类型'] ?? r['type'] ?? ''),
      precondition: String(r['前置条件'] ?? r['precondition'] ?? ''),
      assignee: String(r['负责人'] ?? r['assignee'] ?? ''),
      status: String(r['状态'] ?? r['status'] ?? 'draft'),
      tags: JSON.stringify(String(r['标签'] ?? '').split(',').map((s) => s.trim()).filter(Boolean)),
      steps: JSON.stringify([{ action: String(r['步骤'] ?? ''), expected: String(r['预期'] ?? '') }]),
    }))
    const valid = mapped.filter((m) => m.name)
    if (!valid.length) { ElMessage.warning('未解析到有效用例（需含“名称”列）'); return }
    const res = await casesApi.batchImportCases(valid)
    importMsg.value = `导入完成：成功 ${res.count} 条`
    await loadCases()
  } catch (err) {
    importMsg.value = `导入失败：${(err as Error).message}`
  } finally {
    ;(e.target as HTMLInputElement).value = ''
  }
}

function onXmindImport(e: Event): void {
  ;(e.target as HTMLInputElement).value = ''
  ElMessage.warning('思维导图导入暂未实现，请使用导出')
}

// ==================== Load ====================

async function loadCases(): Promise<void> {
  try {
    const [cs, ms, ss] = await Promise.all([
      casesApi.getCases(),
      casesApi.getCaseModules(),
      scriptsApi.getScripts().catch(() => [] as Script[]),
    ])
    cases.value = cs
    modules.value = ms
    scripts.value = ss
    expandedModules.value = new Set(ms.map((m) => m.id))
  } catch (err) {
    ElMessage.error((err as Error).message || '加载失败')
  }
}

async function loadExecutions(): Promise<void> {
  try {
    executions.value = await casesApi.getCaseExecutions()
  } catch (err) {
    ElMessage.error((err as Error).message || '加载执行记录失败')
  }
}

onMounted(async () => {
  await loadCases()
  await loadExecutions()
})
</script>

<template>
  <div class="space-y-4">
    <!-- Page Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">用例平台</h1>
        <p class="text-muted-foreground mt-0.5">功能用例编写、执行与追踪</p>
      </div>
      <div class="flex gap-2">
        <Button :variant="batchMode ? 'default' : 'outline'" class="gap-2" @click="batchMode = !batchMode; batchSelected = new Set(); batchResults = {}">
          <component :is="batchMode ? CheckCircle2 : Minus" class="w-4 h-4" />
          {{ batchMode ? '退出批量' : '批量执行' }}
        </Button>
        <Button variant="outline" class="gap-2" @click="startCreateCase()">
          <Plus class="w-4 h-4" /> 新建用例
        </Button>
      </div>
    </div>
    <div v-if="importMsg" :class="cn('text-sm p-3 rounded-lg', importMsg.includes('失败') ? 'bg-amber-50 text-amber-700' : 'bg-emerald-50 text-emerald-700')">
      {{ importMsg }}
      <button class="ml-2 text-muted-foreground hover:text-foreground" @click="importMsg = ''">✕</button>
    </div>

    <!-- Tab Bar -->
    <div class="flex gap-1 bg-muted/50 p-1 rounded-lg w-fit">
      <button
        v-for="t in tabs"
        :key="t.key"
        @click="tab = t.key"
        :class="cn(
          'flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium transition-all',
          tab === t.key ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground',
        )"
      >
        <component :is="t.icon" class="w-4 h-4" /> {{ t.label }}
      </button>
    </div>

    <!-- ==================== Cases Management Tab ==================== -->
    <div v-if="tab === 'cases'" class="flex gap-6">
      <!-- Left: Case List -->
      <div class="flex-1 space-y-3">
        <!-- Filters -->
        <div class="flex items-center gap-3">
          <div class="relative flex-1">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input v-model="searchTerm" placeholder="搜索用例名称、负责人..." class="pl-9" />
          </div>
          <select v-model="filterPriority" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部优先级</option>
            <option value="P0">P0</option><option value="P1">P1</option><option value="P2">P2</option><option value="P3">P3</option>
          </select>
          <select v-model="filterType" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部类型</option>
            <option v-for="(v, k) in typeCfg" :key="k" :value="k">{{ v.label }}</option>
          </select>
          <select v-model="filterStatus" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部状态</option>
            <option value="draft">草稿</option>
            <option value="active">激活</option>
            <option value="disabled">禁用</option>
          </select>
        </div>

        <!-- Batch Execution Bar -->
        <Card v-if="batchMode && batchSelected.size > 0" class="border-primary/30 bg-primary/5">
          <CardContent class="p-3 flex items-center gap-3">
            <span class="text-sm font-medium">已选 {{ batchSelected.size }} 条用例</span>
            <div class="flex gap-2 ml-auto">
              <Button size="sm" variant="outline" class="gap-1 text-xs" @click="batchSetResult('passed')">
                <CheckCircle2 class="w-3 h-3" /> 全部通过
              </Button>
              <Button size="sm" variant="outline" class="gap-1 text-xs text-red-600" @click="batchSetResult('failed')">
                <XCircle class="w-3 h-3" /> 全部失败
              </Button>
              <Button size="sm" variant="outline" class="gap-1 text-xs text-amber-600" @click="batchSetResult('blocked')">
                <AlertCircle class="w-3 h-3" /> 全部阻塞
              </Button>
              <Button size="sm" class="gap-1 text-xs" @click="submitBatchExecution">
                <Save class="w-3 h-3" /> 提交
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- Module Grouped Cases -->
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm font-medium text-muted-foreground">模块列表</span>
          <Button variant="ghost" size="sm" class="h-8 text-xs gap-1" @click="addModule">
            <Plus class="w-3 h-3" /> 新建模块
          </Button>
        </div>
        <template v-for="mod in modules" :key="mod.id">
          <Card class="group">
            <div class="flex items-center justify-between p-3 cursor-pointer hover:bg-accent/50 transition-colors" @click="toggleModule(mod.id)">
              <div class="flex items-center gap-2">
                <component :is="expandedModules.has(mod.id) ? ChevronDown : ChevronRight" class="w-4 h-4" />
                <FolderOpen class="w-4 h-4 text-amber-500" />
                <span class="font-medium text-sm">{{ mod.name }}</span>
                <Badge variant="secondary" class="text-xs">{{ (groupedCases[mod.id] || []).length }}</Badge>
              </div>
              <div class="flex items-center gap-1">
                <Badge v-if="(groupedCases[mod.id] || []).length > 0" variant="success" class="text-xs">✓{{ modulePassedCount(mod.id, groupedCases[mod.id] || []) }}</Badge>
                <button class="p-1.5 hover:text-primary hover:bg-accent rounded transition-colors" title="新增用例" @click.stop="startCreateCase(mod.id)">
                  <Plus class="w-3.5 h-3.5" />
                </button>
                <button class="p-1.5 hover:text-primary hover:bg-accent rounded transition-colors" title="重命名" @click.stop="renameModule(mod.id)">
                  <Edit3 class="w-3.5 h-3.5" />
                </button>
                <button class="p-1.5 hover:text-destructive hover:bg-destructive/10 rounded transition-colors" title="删除模块" @click.stop="deleteModule(mod.id)">
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
            <div v-if="expandedModules.has(mod.id) && (groupedCases[mod.id] || []).length > 0" class="border-t">
              <div
                v-for="c in groupedCases[mod.id]"
                :key="c.id"
                :class="cn(
                  'flex items-center justify-between p-3 px-5 cursor-pointer hover:bg-accent/30 transition-colors border-b last:border-0',
                  selectedCaseId === c.id ? 'bg-primary/5' : '',
                )"
                @click="!batchMode && (selectedCaseId = c.id)"
              >
                <div class="flex items-center gap-3 min-w-0">
                  <input v-if="batchMode" type="checkbox" :checked="batchSelected.has(c.id)" @change="toggleBatchCase(c.id)" @click.stop class="accent-primary w-4 h-4" />
                  <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[c.priority].cls)">{{ c.priority }}</span>
                  <div class="min-w-0">
                    <p class="text-sm font-medium truncate">{{ c.name }}</p>
                    <div class="flex items-center gap-2 mt-0.5 text-xs text-muted-foreground">
                      <span>{{ typeLabel(c.type) }}</span>
                      <span>·</span>
                      <span>{{ c.assignee || '未指派' }}</span>
                      <span>·</span>
                      <span>{{ caseStepCount(c) }} 步骤</span>
                      <template v-if="lastExecOf(c)">
                        <span>·</span>
                        <span :class="lastExecOf(c)?.result === 'passed' ? 'text-emerald-600' : lastExecOf(c)?.result === 'failed' ? 'text-red-600' : 'text-amber-600'">
                          {{ lastExecOf(c)?.result === 'passed' ? '✓ 通过' : lastExecOf(c)?.result === 'failed' ? '✗ 失败' : '⊘ 阻塞' }}
                        </span>
                      </template>
                    </div>
                  </div>
                </div>
                <div class="flex items-center gap-1 shrink-0">
                  <template v-if="!batchMode">
                    <!-- Quick execute dropdown -->
                    <div class="relative">
                      <Button variant="ghost" size="icon" class="h-8 w-8" @click.stop="quickExecCaseId = quickExecCaseId === c.id ? null : c.id">
                        <Play class="w-3.5 h-3.5" />
                      </Button>
                      <div v-if="quickExecCaseId === c.id" class="absolute right-0 top-full mt-1 z-50 bg-background border rounded-lg shadow-lg py-1 min-w-[140px]">
                        <button class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-emerald-600" @click.stop="quickExecute(c, 'passed')">
                          <CheckCircle2 class="w-4 h-4" /> 快速通过
                        </button>
                        <button class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-red-600" @click.stop="quickExecute(c, 'failed')">
                          <XCircle class="w-4 h-4" /> 快速失败
                        </button>
                        <button class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-amber-600" @click.stop="quickExecute(c, 'blocked')">
                          <AlertCircle class="w-4 h-4" /> 快速阻塞
                        </button>
                        <div class="border-t my-1" />
                        <button class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent" @click.stop="quickExecCaseId = null; startExecution(c)">
                          <Play class="w-4 h-4" /> 详细执行
                        </button>
                      </div>
                    </div>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive/70 hover:text-destructive" @click.stop="openBugModal(c)">
                      <Bug class="w-3.5 h-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click.stop="startEditCase(c)">
                      <Edit3 class="w-3.5 h-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click.stop="handleDeleteCase(c.id)">
                      <Trash2 class="w-3.5 h-3.5 text-red-500" />
                    </Button>
                  </template>
                </div>
              </div>
              <div class="p-3 border-t bg-muted/30">
                <Button size="sm" variant="outline" class="w-full gap-1" @click.stop="startCreateCase(mod.id)">
                  <Plus class="w-4 h-4" /> 新增用例
                </Button>
              </div>
            </div>
            <div v-if="expandedModules.has(mod.id) && (groupedCases[mod.id] || []).length === 0" class="p-6 text-center border-t">
              <p class="text-sm text-muted-foreground mb-3">暂无用例</p>
              <Button size="sm" class="gap-1" @click.stop="startCreateCase(mod.id)">
                <Plus class="w-4 h-4" /> 新增用例
              </Button>
            </div>
          </Card>
        </template>
        <!-- Uncategorized -->
        <Card v-if="(groupedCases['uncategorized'] || []).length > 0" class="group">
          <div class="flex items-center justify-between p-3">
            <div class="flex items-center gap-2">
              <FolderOpen class="w-4 h-4 text-gray-400" />
              <span class="font-medium text-sm">未分类</span>
              <Badge variant="secondary" class="text-xs">{{ groupedCases['uncategorized'].length }}</Badge>
            </div>
          </div>
          <div class="border-t">
            <div
              v-for="c in groupedCases['uncategorized']"
              :key="c.id"
              :class="cn(
                'flex items-center justify-between p-3 px-5 cursor-pointer hover:bg-accent/30 transition-colors border-b last:border-0',
                selectedCaseId === c.id ? 'bg-primary/5' : '',
              )"
              @click="!batchMode && (selectedCaseId = c.id)"
            >
              <div class="flex items-center gap-3 min-w-0">
                <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[c.priority].cls)">{{ c.priority }}</span>
                <div class="min-w-0">
                  <p class="text-sm font-medium truncate">{{ c.name }}</p>
                  <div class="flex items-center gap-2 mt-0.5 text-xs text-muted-foreground">
                    <span>{{ typeLabel(c.type) }}</span><span>·</span><span>{{ c.assignee || '未指派' }}</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <Button variant="ghost" size="icon" class="h-8 w-8" @click.stop="startEditCase(c)"><Edit3 class="w-3.5 h-3.5" /></Button>
                <Button variant="ghost" size="icon" class="h-8 w-8" @click.stop="handleDeleteCase(c.id)"><Trash2 class="w-3.5 h-3.5 text-red-500" /></Button>
              </div>
            </div>
          </div>
        </Card>
      </div>

      <!-- Right: Case Detail / Editor -->
      <div class="w-[480px] shrink-0">
        <Card v-if="editingCase">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ isCreating ? '新建用例' : '编辑用例' }}</CardTitle>
          </CardHeader>
          <CardContent class="space-y-4">
            <div>
              <label class="text-sm font-medium mb-1 block">用例名称 *</label>
              <Input v-model="editingCase.name" placeholder="输入用例名称" />
            </div>
            <div class="grid grid-cols-3 gap-3">
              <div>
                <label class="text-sm font-medium mb-1 block">模块</label>
                <select v-model="editingCase.moduleId" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
                  <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.name }}</option>
                </select>
              </div>
              <div>
                <label class="text-sm font-medium mb-1 block">优先级</label>
                <select v-model="editingCase.priority" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
                  <option v-for="(v, k) in priorityCfg" :key="k" :value="k">{{ v.label }}</option>
                </select>
              </div>
              <div>
                <label class="text-sm font-medium mb-1 block">类型</label>
                <select v-model="editingCase.type" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
                  <option v-for="(v, k) in typeCfg" :key="k" :value="k">{{ v.label }}</option>
                </select>
              </div>
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">负责人</label>
              <Input v-model="editingCase.assignee" placeholder="指派给谁" />
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">关联脚本（自动执行）</label>
              <select v-model="editingCase.scriptId" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
                <option value="">无（手动执行）</option>
                <option v-for="s in scripts" :key="s.id" :value="s.id">{{ s.name }}（{{ s.language }}）</option>
              </select>
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">前置条件</label>
              <textarea v-model="editingCase.precondition" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring" placeholder="执行前需要满足的条件" />
            </div>

            <!-- Steps Table -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <label class="text-sm font-medium">测试步骤 *</label>
                <Button variant="outline" size="sm" @click="addStep" class="gap-1 text-xs">
                  <Plus class="w-3 h-3" /> 添加步骤
                </Button>
              </div>
              <div class="space-y-2">
                <div v-for="(step, idx) in editingCase.steps" :key="idx" class="grid grid-cols-[32px_1fr_1fr_32px] gap-2 items-start p-3 rounded-lg bg-muted/30 border">
                  <div class="flex items-center justify-center h-9 w-8 rounded-full bg-primary/10 text-primary text-sm font-bold">{{ idx + 1 }}</div>
                  <div>
                    <label class="text-xs text-muted-foreground uppercase tracking-wide">测试步骤</label>
                    <textarea v-model="editingCase.steps[idx].action" class="w-full mt-0.5 h-16 rounded border border-input bg-background px-2 py-1 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring" placeholder="描述操作步骤" />
                  </div>
                  <div>
                    <label class="text-xs text-muted-foreground uppercase tracking-wide">预期结果</label>
                    <textarea v-model="editingCase.steps[idx].expected" class="w-full mt-0.5 h-16 rounded border border-input bg-background px-2 py-1 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring" placeholder="描述预期结果" />
                  </div>
                  <Button variant="ghost" size="icon" class="h-8 w-8 mt-5" @click="removeStep(idx)" :disabled="editingCase.steps.length <= 1">
                    <Trash2 class="w-3.5 h-3.5 text-red-500" />
                  </Button>
                </div>
              </div>
            </div>

            <div class="flex gap-2 pt-2 border-t">
              <Button class="flex-1 gap-2" @click="saveEditingCase"><Save class="w-4 h-4" /> 保存用例</Button>
              <Button variant="outline" @click="editingCase = null">取消</Button>
            </div>
          </CardContent>
        </Card>

        <Card v-else-if="selectedCase">
          <CardHeader class="pb-3">
            <div class="flex items-center justify-between">
              <CardTitle class="text-base">用例详情</CardTitle>
              <div class="flex gap-1">
                <Button variant="ghost" size="icon" class="h-8 w-8" @click="startEditCase(selectedCase)"><Edit3 class="w-3.5 h-3.5" /></Button>
                <Button variant="ghost" size="icon" class="h-8 w-8" @click="handleDeleteCase(selectedCase.id)"><Trash2 class="w-3.5 h-3.5 text-red-500" /></Button>
              </div>
            </div>
          </CardHeader>
          <CardContent class="space-y-4">
            <div>
              <h3 class="font-semibold">{{ selectedCase.name }}</h3>
              <div class="flex items-center gap-2 mt-1.5 flex-wrap">
                <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[selectedCase.priority].cls)">{{ priorityCfg[selectedCase.priority].label }}</span>
                <span :class="cn('text-xs', typeColor(selectedCase.type))">{{ typeLabel(selectedCase.type) }}</span>
                <Badge :variant="selectedCase.status === 'active' ? 'success' : 'secondary'" class="text-xs">{{ selectedCase.status === 'active' ? '启用' : selectedCase.status === 'draft' ? '草稿' : '禁用' }}</Badge>
              </div>
            </div>
            <div v-if="selectedCase.precondition" class="p-3 rounded-lg bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-900/30">
              <p class="text-xs font-medium text-blue-600 dark:text-blue-400 mb-1">前置条件</p>
              <p class="text-sm text-blue-700 dark:text-blue-300">{{ selectedCase.precondition }}</p>
            </div>
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div><span class="text-muted-foreground">负责人：</span><span class="font-medium">{{ selectedCase.assignee || '未指派' }}</span></div>
              <div><span class="text-muted-foreground">步骤数：</span><span class="font-medium">{{ caseStepCount(selectedCase) }}</span></div>
            </div>
            <!-- Steps Preview -->
            <div>
              <p class="text-sm font-medium mb-2">测试步骤</p>
              <div class="space-y-2">
                <div v-for="(step, idx) in caseSteps(selectedCase)" :key="idx" class="flex gap-3 p-3 rounded-lg bg-muted/30">
                  <div class="flex items-center justify-center h-6 w-6 rounded-full bg-primary/10 text-primary text-xs font-bold shrink-0">{{ idx + 1 }}</div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm"><span class="text-muted-foreground">步骤：</span>{{ step.action || '—' }}</p>
                    <p class="text-sm mt-0.5"><span class="text-muted-foreground">预期：</span>{{ step.expected || '—' }}</p>
                  </div>
                </div>
              </div>
            </div>
            <!-- Recent Executions -->
            <div v-if="executions.filter(e => e.caseId === selectedCase?.id).slice(0, 5).length > 0">
              <p class="text-sm font-medium mb-2">最近执行</p>
              <div class="space-y-1.5">
                <div v-for="ex in executions.filter(e => e.caseId === selectedCase?.id).slice(0, 5)" :key="ex.id" class="flex items-center justify-between p-2 rounded bg-muted/50 text-xs cursor-pointer hover:bg-accent/50">
                  <div class="flex items-center gap-2">
                    <component :is="stepResultCfg[ex.result].icon" :class="cn('w-4 h-4', stepResultCfg[ex.result].color)" />
                    <span>{{ formatDate(ex.executedAt) }}</span>
                  </div>
                  <span class="text-muted-foreground">{{ ex.executor }} · {{ ex.duration }}s</span>
                </div>
              </div>
            </div>
            <div v-else class="text-sm text-muted-foreground text-center py-3">暂无执行记录</div>
            <Button class="w-full gap-2" @click="startExecution(selectedCase)"><Play class="w-4 h-4" /> 执行用例</Button>
          </CardContent>
        </Card>

        <div v-else class="flex items-center justify-center h-64">
          <div class="text-center text-muted-foreground">
            <FileCheck class="w-12 h-12 mx-auto mb-3 opacity-20" />
            <p class="text-sm">选择用例查看详情，或点击「新建用例」</p>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== Execution Tab ==================== -->
    <div v-if="tab === 'execute' && executingCase" class="max-w-3xl mx-auto space-y-4">
      <div class="flex items-center gap-3">
        <Button variant="ghost" size="icon" @click="tab = 'cases'"><ArrowLeft class="w-4 h-4" /></Button>
        <div>
          <h2 class="text-lg font-bold">{{ executingCase.name }}</h2>
          <p class="text-sm text-muted-foreground">{{ execSteps.length }} 个步骤 · 请逐步标记执行结果</p>
        </div>
      </div>

      <div v-if="executingCase.precondition" class="p-3 rounded-lg bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-900/30">
        <p class="text-xs font-medium text-blue-600 dark:text-blue-400">⚠ 前置条件</p>
        <p class="text-sm text-blue-700 dark:text-blue-300 mt-1">{{ executingCase.precondition }}</p>
      </div>

      <!-- Progress Bar -->
      <div class="flex items-center gap-2">
        <div class="flex-1 h-2 bg-muted rounded-full overflow-hidden">
          <div
            class="h-full bg-emerald-500 rounded-full transition-all duration-300"
            :style="{ width: (execSteps.filter(s => s.result !== 'pending').length / execSteps.length) * 100 + '%' }"
          />
        </div>
        <span class="text-xs text-muted-foreground">{{ execSteps.filter(s => s.result !== 'pending').length }}/{{ execSteps.length }}</span>
      </div>

      <!-- Steps -->
      <div class="space-y-3">
        <Card v-for="(step, idx) in execSteps" :key="idx" :class="cn('transition-all', step.result !== 'pending' ? 'opacity-80' : 'ring-2 ring-primary/30')">
          <CardContent class="p-4 space-y-3">
            <div class="flex items-center gap-3">
              <div class="flex items-center justify-center h-8 w-8 rounded-full bg-primary/10 text-primary text-sm font-bold">{{ idx + 1 }}</div>
              <div class="flex-1">
                <p class="text-sm font-medium">{{ step.action || '未填写步骤' }}</p>
                <p class="text-xs text-muted-foreground mt-0.5">预期：{{ step.expected || '未填写' }}</p>
              </div>
              <div :class="cn('flex items-center gap-1', stepResultCfg[step.result].color)">
                <component :is="stepResultCfg[step.result].icon" class="w-4 h-4" />
                <span class="text-xs font-medium">{{ stepResultCfg[step.result].label }}</span>
              </div>
            </div>
            <!-- Result Buttons -->
            <div class="flex gap-2 pl-11">
              <button
                v-for="r in stepResults"
                :key="r"
                @click="markStepResult(idx, r)"
                :class="cn(
                  'flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium transition-all border',
                  step.result === r
                    ? r === 'passed' ? 'bg-emerald-100 border-emerald-300 text-emerald-700 dark:bg-emerald-900/30 dark:border-emerald-700 dark:text-emerald-400'
                    : r === 'failed' ? 'bg-red-100 border-red-300 text-red-700 dark:bg-red-900/30 dark:border-red-700 dark:text-red-400'
                    : r === 'blocked' ? 'bg-amber-100 border-amber-300 text-amber-700 dark:bg-amber-900/30 dark:border-amber-700 dark:text-amber-400'
                    : 'bg-gray-100 border-gray-300 text-gray-700 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-400'
                    : 'bg-muted/50 border-transparent text-muted-foreground hover:bg-accent',
                )"
              >
                <component :is="stepResultCfg[r].icon" class="w-4 h-4" /> {{ stepResultCfg[r].label }}
              </button>
            </div>
            <!-- Actual result input -->
            <div v-if="step.result === 'failed' || step.result === 'blocked'" class="pl-11">
              <label class="text-xs text-muted-foreground mb-1 block">实际结果</label>
              <textarea
                :value="step.actual"
                @input="updateStepActual(idx, ($event.target as HTMLTextAreaElement).value)"
                class="w-full h-12 rounded border border-input bg-background px-2 py-1 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring"
                placeholder="描述实际结果..."
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <Button class="w-full h-12 text-base gap-2" @click="submitExecution" :disabled="execSteps.some(s => s.result === 'pending')">
        <CheckCircle2 class="w-5 h-5" /> 提交执行结果
      </Button>
    </div>

    <!-- ==================== Table View Tab ==================== -->
    <div v-if="tab === 'table'" class="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle class="text-base flex items-center gap-2">
            <Download class="w-4 h-4" /> 表格测试用例
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex items-center gap-4">
            <label class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium h-10 px-4 py-2 gap-2 border border-input bg-background hover:bg-accent hover:text-accent-foreground cursor-pointer">
              <Upload class="w-4 h-4" /> 导入表格
              <input type="file" accept=".xlsx,.xls" class="hidden" @change="onTableImport" />
            </label>
            <Button variant="outline" class="gap-2" @click="exportTable">
              <Download class="w-4 h-4" /> 导出表格
            </Button>
            <span class="text-sm text-muted-foreground">共 {{ cases.length }} 条用例</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">用例列表</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b text-left text-muted-foreground">
                  <th class="pb-2 pr-4 font-medium">用例名称</th>
                  <th class="pb-2 pr-4 font-medium">优先级</th>
                  <th class="pb-2 pr-4 font-medium">类型</th>
                  <th class="pb-2 pr-4 font-medium">模块</th>
                  <th class="pb-2 pr-4 font-medium">负责人</th>
                  <th class="pb-2 pr-4 font-medium text-center">步骤数</th>
                  <th class="pb-2 pr-4 font-medium">状态</th>
                  <th class="pb-2 font-medium">最近执行</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="tc in cases" :key="tc.id" class="border-b last:border-0 hover:bg-accent/30 cursor-pointer" @click="selectedCaseId = tc.id; tab = 'cases'">
                  <td class="py-2 pr-4 font-medium">{{ tc.name }}</td>
                  <td class="py-2 pr-4"><span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[tc.priority].cls)">{{ tc.priority }}</span></td>
                  <td class="py-2 pr-4"><span :class="typeColor(tc.type)">{{ typeLabel(tc.type) }}</span></td>
                  <td class="py-2 pr-4">{{ moduleName(tc.moduleId) }}</td>
                  <td class="py-2 pr-4">{{ tc.assignee || '未指派' }}</td>
                  <td class="py-2 pr-4 text-center">{{ caseStepCount(tc) }}</td>
                  <td class="py-2 pr-4"><Badge :variant="tc.status === 'active' ? 'success' : 'secondary'" class="text-xs">{{ tc.status === 'active' ? '启用' : '草稿' }}</Badge></td>
                  <td class="py-2">
                    <span v-if="lastExecOf(tc)" :class="lastExecOf(tc)?.result === 'passed' ? 'text-emerald-600' : lastExecOf(tc)?.result === 'failed' ? 'text-red-600' : 'text-amber-600'">
                      {{ lastExecOf(tc)?.result === 'passed' ? '✓ 通过' : lastExecOf(tc)?.result === 'failed' ? '✗ 失败' : '⊘ 阻塞' }}
                    </span>
                    <span v-else class="text-muted-foreground">未执行</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- ==================== Execution History Tab ==================== -->
    <div v-if="tab === 'execHistory'" class="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle class="text-base flex items-center gap-2">
            <BarChart3 class="w-4 h-4" /> 每日执行统计
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex items-end gap-1 h-[220px]">
            <div v-for="d in dailyStats" :key="d.date" class="flex-1 flex flex-col items-center justify-end gap-1">
              <div class="flex items-end gap-0.5 h-full">
                <div :style="{ height: (d.caseRuns / chartMax * 100) + '%' }" class="w-3 bg-blue-500 rounded-t" :title="`执行数 ${d.caseRuns}`" />
                <div :style="{ height: d.passRate + '%' }" class="w-3 bg-emerald-500 rounded-t" :title="`通过率 ${d.passRate}%`" />
              </div>
              <span class="text-[10px] text-muted-foreground">{{ d.date.slice(5) }}</span>
            </div>
          </div>
          <div class="flex items-center gap-4 mt-3 text-xs text-muted-foreground">
            <span class="flex items-center gap-1"><span class="w-3 h-3 rounded-sm bg-blue-500" /> 执行数</span>
            <span class="flex items-center gap-1"><span class="w-3 h-3 rounded-sm bg-emerald-500" /> 通过率%</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">每日执行明细</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b text-left text-muted-foreground">
                  <th class="pb-2 pr-4 font-medium">日期</th>
                  <th class="pb-2 pr-4 font-medium text-center">执行总数</th>
                  <th class="pb-2 pr-4 font-medium text-center">通过</th>
                  <th class="pb-2 pr-4 font-medium text-center">失败</th>
                  <th class="pb-2 pr-4 font-medium text-center">阻塞</th>
                  <th class="pb-2 font-medium text-center">通过率</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="d in [...dailyStats].reverse()" :key="d.date" class="border-b last:border-0 hover:bg-accent/30">
                  <td class="py-2 pr-4 font-medium">{{ d.date }}</td>
                  <td class="py-2 pr-4 text-center">{{ d.caseRuns }}</td>
                  <td class="py-2 pr-4 text-center text-emerald-600">{{ executions.filter(e => e.executedAt.slice(0,10) === d.date).filter(e => e.result === 'passed').length }}</td>
                  <td class="py-2 pr-4 text-center text-red-600">{{ executions.filter(e => e.executedAt.slice(0,10) === d.date).filter(e => e.result === 'failed').length }}</td>
                  <td class="py-2 pr-4 text-center text-amber-600">{{ executions.filter(e => e.executedAt.slice(0,10) === d.date).filter(e => e.result === 'blocked').length }}</td>
                  <td class="py-2 text-center">
                    <span :class="cn(
                      'px-2 py-0.5 rounded text-xs font-medium',
                      d.passRate >= 90 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                      : d.passRate >= 70 ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
                    )">{{ d.passRate }}%</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <div class="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle class="text-base">执行记录列表</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="space-y-2 max-h-[500px] overflow-auto">
              <div
                v-for="ex in executions.slice(0, 50)"
                :key="ex.id"
                :class="cn(
                  'flex items-center justify-between p-3 rounded-lg cursor-pointer transition-colors',
                  selectedExecId === ex.id ? 'bg-primary/10 ring-1 ring-primary/30' : 'bg-muted/30 hover:bg-accent/30',
                )"
                @click="selectedExecId = ex.id"
              >
                <div class="flex items-center gap-2">
                  <component :is="stepResultCfg[ex.result].icon" :class="cn('w-4 h-4', stepResultCfg[ex.result].color)" />
                  <div>
                    <p class="text-sm font-medium truncate max-w-[200px]">{{ ex.caseName }}</p>
                    <p class="text-xs text-muted-foreground">{{ ex.executor }} · {{ formatDate(ex.executedAt) }}</p>
                  </div>
                </div>
                <div class="text-right shrink-0">
                  <Badge :variant="ex.result === 'passed' ? 'success' : ex.result === 'failed' ? 'destructive' : 'secondary'" class="text-xs">
                    {{ stepResultCfg[ex.result].label }}
                  </Badge>
                  <p class="text-xs text-muted-foreground mt-1">{{ ex.duration }}s</p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle class="text-base">执行详情</CardTitle>
          </CardHeader>
          <CardContent>
            <div v-if="selectedExec" class="space-y-3">
              <div>
                <h3 class="font-medium">{{ selectedExec.caseName }}</h3>
                <div class="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                  <span>{{ selectedExec.executor }}</span>
                  <span>{{ formatDate(selectedExec.executedAt) }}</span>
                  <span>耗时 {{ selectedExec.duration }}s</span>
                </div>
                <div class="mt-2">
                  <Badge :variant="selectedExec.result === 'passed' ? 'success' : selectedExec.result === 'failed' ? 'destructive' : 'secondary'" class="text-xs">
                    {{ stepResultCfg[selectedExec.result].label }}
                  </Badge>
                </div>
              </div>
              <div class="space-y-2">
                <div v-for="(step, idx) in safeParseJSON<any[]>(selectedExec.steps, [])" :key="idx" :class="cn('p-3 rounded-lg border',
                  step.result === 'passed' ? 'bg-emerald-50 dark:bg-emerald-900/10 border-emerald-200 dark:border-emerald-900/30'
                  : step.result === 'failed' ? 'bg-red-50 dark:bg-red-900/10 border-red-200 dark:border-red-900/30'
                  : step.result === 'blocked' ? 'bg-amber-50 dark:bg-amber-900/10 border-amber-200 dark:border-amber-900/30'
                  : 'bg-muted/30 border-transparent')">
                  <div class="flex items-center gap-2 mb-1">
                    <span class="flex items-center justify-center h-5 w-5 rounded-full bg-primary/10 text-primary text-xs font-bold">{{ idx + 1 }}</span>
                    <span :class="cn('flex items-center gap-1', stepResultCfg[step.result as keyof typeof stepResultCfg].color)">
                      <component :is="stepResultCfg[step.result as keyof typeof stepResultCfg].icon" class="w-4 h-4" />
                      <span class="text-xs font-medium">{{ stepResultCfg[step.result as keyof typeof stepResultCfg].label }}</span>
                    </span>
                  </div>
                  <p class="text-sm"><span class="text-muted-foreground">步骤：</span>{{ step.action }}</p>
                  <p class="text-sm"><span class="text-muted-foreground">预期：</span>{{ step.expected }}</p>
                  <p v-if="step.actual" class="text-sm"><span class="text-muted-foreground">实际：</span>{{ step.actual }}</p>
                </div>
              </div>
            </div>
            <div v-else class="flex items-center justify-center h-48 text-muted-foreground">
              <div class="text-center">
                <Clock class="w-10 h-10 mx-auto mb-2 opacity-20" />
                <p class="text-sm">点击左侧记录查看详情</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- ==================== XMind Format Cases Tab ==================== -->
    <div v-if="tab === 'xmind'" class="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle class="text-base flex items-center gap-2">
            <FolderOpen class="w-4 h-4" /> 思维导图用例
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex items-center gap-4">
            <label class="inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium h-10 px-4 py-2 gap-2 border border-input bg-background hover:bg-accent hover:text-accent-foreground cursor-pointer">
              <Upload class="w-4 h-4" /> 导入思维导图
              <input type="file" accept=".xmind" class="hidden" @change="onXmindImport" />
            </label>
            <Button variant="outline" class="gap-2" @click="exportXmind">
              <Download class="w-4 h-4" /> 导出思维导图
            </Button>
            <span class="text-sm text-muted-foreground">共 {{ cases.length }} 条用例</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">思维导图预览</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="space-y-2 max-h-[600px] overflow-auto">
            <div v-if="modules.length === 0 && cases.length === 0" class="text-center py-8 text-muted-foreground">
              <FileCheck class="w-12 h-12 mx-auto mb-3 opacity-20" />
              <p class="text-sm">暂无用例，请先导入或创建用例</p>
            </div>
            <template v-else>
              <div v-for="mod in modules" :key="mod.id">
                <div v-if="cases.filter(c => c.moduleId === mod.id).length > 0" class="border rounded-lg overflow-hidden">
                  <div class="flex items-center gap-2 p-3 bg-muted/50 font-medium text-sm">
                    <FolderOpen class="w-4 h-4 text-amber-500" />
                    {{ mod.name }}
                    <Badge variant="secondary" class="text-xs ml-auto">{{ cases.filter(c => c.moduleId === mod.id).length }}</Badge>
                  </div>
                  <div class="pl-6 space-y-1 p-2">
                    <div v-for="tc in cases.filter(c => c.moduleId === mod.id)" :key="tc.id" class="border-l-2 border-primary/30 pl-3 py-2">
                      <div class="flex items-center gap-2">
                        <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[tc.priority].cls)">{{ tc.priority }}</span>
                        <span class="font-medium text-sm">{{ tc.name }}</span>
                        <Badge :variant="tc.status === 'active' ? 'success' : 'secondary'" class="text-xs">{{ tc.status === 'active' ? '启用' : '草稿' }}</Badge>
                      </div>
                      <div class="pl-4 mt-1 space-y-0.5">
                        <div v-for="(step, idx) in caseSteps(tc)" :key="idx" class="flex items-start gap-2 text-xs text-muted-foreground">
                          <span class="text-primary/60">{{ idx + 1 }}.</span>
                          <span>{{ step.action }}</span>
                          <span v-if="step.expected" class="text-muted-foreground/60">→ {{ step.expected }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <!-- Uncategorized cases -->
              <div v-if="cases.filter(c => !c.moduleId || !modules.find(m => m.id === c.moduleId)).length > 0" class="border rounded-lg overflow-hidden">
                <div class="flex items-center gap-2 p-3 bg-muted/50 font-medium text-sm">
                  <FolderOpen class="w-4 h-4 text-gray-400" />
                  未分类
                  <Badge variant="secondary" class="text-xs ml-auto">{{ cases.filter(c => !c.moduleId || !modules.find(m => m.id === c.moduleId)).length }}</Badge>
                </div>
                <div class="pl-6 space-y-1 p-2">
                  <div v-for="tc in cases.filter(c => !c.moduleId || !modules.find(m => m.id === c.moduleId))" :key="tc.id" class="border-l-2 border-gray-300 pl-3 py-2">
                    <div class="flex items-center gap-2">
                      <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', priorityCfg[tc.priority].cls)">{{ tc.priority }}</span>
                      <span class="font-medium text-sm">{{ tc.name }}</span>
                    </div>
                    <div class="pl-4 mt-1 space-y-0.5">
                      <div v-for="(step, idx) in caseSteps(tc)" :key="idx" class="flex items-start gap-2 text-xs text-muted-foreground">
                        <span class="text-primary/60">{{ idx + 1 }}.</span>
                        <span>{{ step.action }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Bug Report Modal -->
    <Dialog :model-value="bugModalCase !== null" @update:model-value="(v: boolean) => { if (!v) bugModalCase = null }">
      <template #default="{ close }">
        <div v-if="bugModalCase" class="space-y-4">
          <div>
            <h3 class="text-lg font-semibold">报告缺陷</h3>
            <p class="text-sm text-muted-foreground">从用例「{{ bugModalCase.name }}」创建缺陷</p>
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">标题 *</label>
            <Input v-model="bugForm.title" placeholder="简要描述缺陷" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">描述</label>
            <textarea v-model="bugForm.description" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">复现步骤</label>
            <textarea v-model="bugForm.steps" class="w-full h-20 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none font-mono" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">预期结果</label>
            <Input v-model="bugForm.expected" />
          </div>
          <div class="flex gap-2 pt-2 border-t">
            <Button class="flex-1 gap-2" @click="submitBug">提交缺陷</Button>
            <Button variant="outline" @click="close()">取消</Button>
          </div>
        </div>
      </template>
    </Dialog>
  </div>
</template>
