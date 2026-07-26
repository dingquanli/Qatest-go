<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ClipboardCheck, Plus, Trash2, Edit3, Play, CheckCircle2,
  XCircle, AlertCircle, Minus, SkipForward, Search, Calendar,
  ArrowLeft, Save, Clock,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import * as plansApi from '@/api/plans'
import * as casesApi from '@/api/cases'
import type { CaseExecution } from '@/types'
import { safeParseJSON, formatDate } from '@/utils'
import { useUserStore } from '@/stores/user'
import type { TestPlan, TestCase, PlanExecution, TestPlanStatus, ExecResult, PlanCaseDetail } from '@/types'

const userStore = useUserStore()

type StepResult = ExecResult

const stepResultCfg: Record<StepResult, { label: string; icon: any; color: string }> = {
  passed: { label: '通过', icon: CheckCircle2, color: 'text-emerald-500' },
  failed: { label: '失败', icon: XCircle, color: 'text-red-500' },
  blocked: { label: '阻塞', icon: AlertCircle, color: 'text-amber-500' },
  skipped: { label: '跳过', icon: SkipForward, color: 'text-gray-400' },
  pending: { label: '待测', icon: Minus, color: 'text-gray-300' },
}

const planStatusCfg: Record<TestPlanStatus, { label: string; cls: string }> = {
  draft: { label: '草稿', cls: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400' },
  in_progress: { label: '进行中', cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  completed: { label: '已完成', cls: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' },
  cancelled: { label: '已取消', cls: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400' },
}

const today = () => new Date().toISOString().slice(0, 10)

const RESULT_ORDER: StepResult[] = ['passed', 'failed', 'blocked', 'skipped']

interface EditablePlan {
  id?: string
  name: string
  description: string
  assignee: string
  status: TestPlanStatus
  startDate: string
  endDate: string
  caseIds: string[]
}

function blankPlan(): EditablePlan {
  return {
    name: '', description: '', assignee: '', status: 'draft',
    startDate: today(), endDate: today(), caseIds: [],
  }
}

const plans = ref<TestPlan[]>([])
const cases = ref<TestCase[]>([])
const execs = ref<PlanExecution[]>([])
const loading = ref(false)

const searchTerm = ref('')
const editingPlan = ref<EditablePlan | null>(null)
const isCreating = ref(false)
const isExecuting = ref(false)
const execPlanId = ref<string | null>(null)
const execResults = ref<Record<string, StepResult>>({})
const execRemarks = ref<Record<string, string>>({})
const execCaseExecId = ref<Record<string, string>>({}) // caseId -> case_executions 记录 id（回写用）
const runningExecution = ref(false)

/* 后端 TestPlan 无 assignee 字段，仅在本会话内保留显示 */
const assigneeMap = ref<Record<string, string>>({})

const allCases = computed(() => cases.value)

function planCaseIds(p: TestPlan): string[] {
  return safeParseJSON<string[]>(p.caseIds, [])
}
function caseName(id: string): string {
  return allCases.value.find((c) => c.id === id)?.name || id
}
function caseSteps(id: string): number {
  const c = allCases.value.find((x) => x.id === id)
  return c ? safeParseJSON<any[]>(c.steps, []).length : 0
}

const planExecsMap = computed<Record<string, PlanExecution[]>>(() => {
  const map: Record<string, PlanExecution[]> = {}
  execs.value.forEach((e) => {
    if (!map[e.planId]) map[e.planId] = []
    map[e.planId].push(e)
  })
  return map
})

const filteredPlans = computed(() =>
  plans.value.filter((p) => {
    const kw = searchTerm.value.trim()
    if (!kw) return true
    return p.name.includes(kw) || (assigneeMap.value[p.id] || '').includes(kw)
  }),
)

const stats = computed(() => {
  const es = execs.value
  const passed = es.reduce((a, e) => a + (e.casesPassed || 0), 0)
  const failed = es.reduce((a, e) => a + (e.casesFailed || 0), 0)
  const blocked = 0
  return {
    total: plans.value.length,
    inProgress: plans.value.filter((p) => p.status === 'in_progress').length,
    completed: plans.value.filter((p) => p.status === 'completed').length,
    totalCases: plans.value.reduce((a, p) => a + planCaseIds(p).length, 0),
    totalExecs: es.length,
    passed, failed, blocked,
  }
})

function computeProgress(plan: TestPlan) {
  const pe = planExecsMap.value[plan.id] || []
  const total = planCaseIds(plan).length
  const latest = pe[pe.length - 1]
  const done = latest ? latest.casesTotal || 0 : 0
  const passed = latest ? latest.casesPassed || 0 : 0
  const failed = latest ? latest.casesFailed || 0 : 0
  const blocked = 0
  const pct = total > 0 ? Math.round((done / total) * 100) : 0
  return { total, done, passed, failed, blocked, pct }
}

/* 编辑 / 新建 */
function startCreate(): void {
  editingPlan.value = blankPlan()
  isCreating.value = true
}
function startEdit(plan: TestPlan): void {
  editingPlan.value = {
    id: plan.id,
    name: plan.name,
    description: plan.description,
    assignee: assigneeMap.value[plan.id] || '',
    status: plan.status,
    startDate: plan.startDate,
    endDate: plan.endDate,
    caseIds: planCaseIds(plan),
  }
  isCreating.value = false
  isExecuting.value = false
}

async function saveEditing(): Promise<void> {
  const e = editingPlan.value
  if (!e || !e.name.trim()) { ElMessage.warning('请输入计划名称'); return }
  const payload: Partial<TestPlan> = {
    name: e.name,
    description: e.description,
    status: e.status,
    startDate: e.startDate ? e.startDate.slice(0, 10) : today(),
    endDate: e.endDate ? e.endDate.slice(0, 10) : today(),
    caseIds: JSON.stringify(e.caseIds),
  }
  if (isCreating.value) {
    const created = await plansApi.createTestPlan(payload)
    assigneeMap.value[created.id] = e.assignee
    ElMessage.success('已创建')
  } else if (e.id) {
    await plansApi.updateTestPlan(e.id, payload)
    assigneeMap.value[e.id] = e.assignee
    ElMessage.success('已更新')
  }
  editingPlan.value = null
  await loadAll()
}

async function handleDelete(id: string): Promise<void> {
  const plan = plans.value.find((p) => p.id === id)
  try {
    await ElMessageBox.confirm(`确定删除计划「${plan?.name || ''}」？`, '提示', { type: 'warning' })
    await plansApi.deleteTestPlan(id)
    delete assigneeMap.value[id]
    ElMessage.success('已删除')
    await loadAll()
  } catch { /* cancelled */ }
}

function toggleCase(caseId: string): void {
  const e = editingPlan.value
  if (!e) return
  e.caseIds = e.caseIds.includes(caseId)
    ? e.caseIds.filter((id) => id !== caseId)
    : [...e.caseIds, caseId]
}

/* 执行 */
const execPlan = computed(() => plans.value.find((p) => p.id === execPlanId.value) || null)

function startExecution(plan: TestPlan): void {
  isCreating.value = false
  editingPlan.value = null
  execPlanId.value = plan.id
  isExecuting.value = true
  const results: Record<string, StepResult> = {}
  const remarks: Record<string, string> = {}
  planCaseIds(plan).forEach((cid) => { results[cid] = 'pending' })
  execResults.value = results
  execRemarks.value = remarks
  execCaseExecId.value = {}
}

// 调用后端执行引擎，创建计划执行记录与逐用例执行记录（manual/auto）
async function runExecution(mode: 'manual' | 'auto' = 'manual', deviceSerial = '', planArg?: TestPlan): Promise<void> {
  const plan = planArg || execPlan.value
  if (!plan) return
  if (planArg) {
    // 从列表直接进入执行
    execPlanId.value = plan.id
    isExecuting.value = false
    const results: Record<string, StepResult> = {}
    planCaseIds(plan).forEach((cid) => { results[cid] = 'pending' })
    execResults.value = results
    execRemarks.value = {}
    execCaseExecId.value = {}
  }
  runningExecution.value = true
  try {
    const res = await plansApi.executeTestPlan(plan.id, { mode, deviceSerial })
    const map: Record<string, string> = {}
    res.caseExecutions.forEach((ce: CaseExecution) => { map[ce.caseId] = ce.id })
    execCaseExecId.value = map
    ElMessage.success(mode === 'auto' ? '已启动自动执行' : '已创建执行记录')
    await loadAll()
  } catch (err) {
    ElMessage.error((err as Error).message || '启动执行失败')
  } finally {
    runningExecution.value = false
  }
}

function markResult(caseId: string, result: StepResult): void {
  execResults.value = { ...execResults.value, [caseId]: result }
}
function setRemark(caseId: string, val: string): void {
  execRemarks.value = { ...execRemarks.value, [caseId]: val }
}

function countResults(cids: string[]) {
  const r = execResults.value
  let passed = 0, failed = 0, blocked = 0, skipped = 0
  cids.forEach((cid) => {
    const x = r[cid] || 'pending'
    if (x === 'passed') passed++
    else if (x === 'failed') failed++
    else if (x === 'blocked') blocked++
    else if (x === 'skipped') skipped++
  })
  return { passed, failed, blocked, skipped, done: passed + failed + blocked + skipped }
}

async function submitExecution(): Promise<void> {
  const plan = execPlan.value
  if (!plan) return
  const cids = planCaseIds(plan)
  const c = countResults(cids)
  // 逐用例回写结果到真实的 case_executions 记录（由后端执行引擎聚合计划结果）
  for (const cid of cids) {
    const ceID = execCaseExecId.value[cid]
    if (!ceID) continue
    try {
      await casesApi.updateCaseExecution(ceID, {
        result: (execResults.value[cid] || 'pending') as ExecResult,
        remark: execRemarks.value[cid] || '',
      })
    } catch {
      /* 单条失败不影响其它 */
    }
  }
  // 同步计划状态
  const allDone = cids.length > 0 && cids.every((cid) => (execResults.value[cid] || 'pending') !== 'pending')
  await plansApi.updateTestPlan(plan.id, { status: allDone ? 'completed' : 'in_progress' })
  await loadAll()
  isExecuting.value = false
  execPlanId.value = null
  execCaseExecId.value = {}
}

async function finishPlan(plan: TestPlan): Promise<void> {
  const cids = planCaseIds(plan)
  const c = countResults(cids)
  const allDone = cids.every((cid) => {
    const x = execResults.value[cid] || 'pending'
    return x !== 'pending'
  })
  await plansApi.updateTestPlan(plan.id, { status: allDone ? 'completed' : 'in_progress' })
  await loadAll()
  isExecuting.value = false
  execPlanId.value = null
}

function resultBtnClass(r: StepResult, current: StepResult): string {
  if (current === r) {
    return r === 'passed' ? 'bg-emerald-100 border-emerald-300 text-emerald-700'
      : r === 'failed' ? 'bg-red-100 border-red-300 text-red-700'
      : r === 'blocked' ? 'bg-amber-100 border-amber-300 text-amber-700'
      : 'bg-gray-100 border-gray-300 text-gray-700'
  }
  return 'bg-muted/50 border-transparent text-muted-foreground hover:bg-accent'
}

const execDone = computed(() => Object.values(execResults.value).filter((r) => r !== 'pending').length)
const execPlanCases = computed(() => {
  const plan = execPlan.value
  if (!plan) return []
  const ids = planCaseIds(plan)
  return allCases.value.filter((c) => ids.includes(c.id))
})

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [ps, cs, es] = await Promise.all([
      plansApi.getTestPlans(),
      casesApi.getCases(),
      plansApi.getPlanExecutions(),
    ])
    plans.value = ps
    cases.value = cs
    execs.value = es
  } catch (err) {
    ElMessage.error((err as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(loadAll)
</script>

<template>
  <!-- 执行视图 -->
  <div v-if="isExecuting && execPlan" class="max-w-3xl mx-auto space-y-4">
    <div class="flex items-center gap-3">
      <Button variant="ghost" size="icon" @click="isExecuting = false; execPlanId = null">
        <ArrowLeft class="w-4 h-4" />
      </Button>
      <div>
        <h2 class="text-lg font-bold">{{ execPlan.name }} · 登记执行结果</h2>
        <p class="text-sm text-muted-foreground">逐条勾选用例的执行结果并备注 · {{ execDone }}/{{ execPlanCases.length }} 已登记</p>
      </div>
    </div>
    <div class="h-2 bg-muted rounded-full overflow-hidden">
      <div class="h-full bg-primary rounded-full transition-all" :style="{ width: (execPlanCases.length > 0 ? (execDone / execPlanCases.length) * 100 : 0) + '%' }" />
    </div>
    <div class="space-y-2">
      <Card v-for="tc in execPlanCases" :key="tc.id" :class="cn('transition-all', execResults[tc.id] !== 'pending' ? 'opacity-80' : '')">
        <CardContent class="p-4">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium">{{ tc.name }}</span>
              <span :class="cn('flex items-center gap-1 text-xs', stepResultCfg[execResults[tc.id] || 'pending'].color)">
                <component :is="stepResultCfg[execResults[tc.id] || 'pending'].icon" class="w-3.5 h-3.5" />
                {{ stepResultCfg[execResults[tc.id] || 'pending'].label }}
              </span>
            </div>
          </div>
          <div class="flex gap-2 flex-wrap">
            <button
              v-for="r in RESULT_ORDER"
              :key="r"
              @click="markResult(tc.id, r)"
              :class="cn('flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium transition-all border', resultBtnClass(r, execResults[tc.id] || 'pending'))"
            >
              <component :is="stepResultCfg[r].icon" class="w-3.5 h-3.5" /> {{ stepResultCfg[r].label }}
            </button>
          </div>
          <textarea
            v-if="execResults[tc.id] === 'failed' || execResults[tc.id] === 'blocked'"
            class="w-full mt-2 h-12 rounded border border-input bg-background px-2 py-1 text-xs resize-none"
            :value="execRemarks[tc.id] || ''"
            @input="setRemark(tc.id, ($event.target as HTMLTextAreaElement).value)"
            placeholder="备注说明..."
          />
        </CardContent>
      </Card>
    </div>
    <div class="flex gap-2">
      <Button class="flex-1 gap-2" :disabled="execDone === 0 || runningExecution" @click="submitExecution"><Save class="w-4 h-4" /> 提交执行结果</Button>
      <Button variant="outline" :disabled="runningExecution" @click="runExecution('auto')"><Play class="w-4 h-4" /> 自动执行</Button>
      <Button variant="outline" @click="finishPlan(execPlan)">完成计划</Button>
    </div>
  </div>

  <!-- 编辑视图 -->
  <div v-else-if="editingPlan" class="max-w-2xl mx-auto space-y-4">
    <div class="flex items-center gap-3">
      <Button variant="ghost" size="icon" @click="editingPlan = null"><ArrowLeft class="w-4 h-4" /></Button>
      <h2 class="text-lg font-bold">{{ isCreating ? '新建测试计划' : '编辑测试计划' }}</h2>
    </div>
    <Card>
      <CardContent class="p-6 space-y-4">
        <div>
          <label class="text-sm font-medium mb-1 block">计划名称 *</label>
          <Input v-model="editingPlan.name" placeholder="输入计划名称" />
        </div>
        <div>
          <label class="text-sm font-medium mb-1 block">描述</label>
          <textarea v-model="editingPlan.description" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-sm font-medium mb-1 block">负责人</label>
            <Input v-model="editingPlan.assignee" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">状态</label>
            <select v-model="editingPlan.status" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
              <option v-for="(v, k) in planStatusCfg" :key="k" :value="k">{{ v.label }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-sm font-medium mb-1 block">开始日期</label>
            <input type="date" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
              :value="editingPlan.startDate ? editingPlan.startDate.slice(0, 10) : ''"
              @change="editingPlan.startDate = ($event.target as HTMLInputElement).value || today()" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">结束日期</label>
            <input type="date" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
              :value="editingPlan.endDate ? editingPlan.endDate.slice(0, 10) : ''"
              @change="editingPlan.endDate = ($event.target as HTMLInputElement).value || today()" />
          </div>
        </div>
        <div>
          <label class="text-sm font-medium mb-2 block">关联用例 ({{ editingPlan.caseIds.length }} 个)</label>
          <div class="max-h-64 overflow-auto space-y-1 border rounded-lg p-2">
            <p v-if="allCases.length === 0" class="text-sm text-muted-foreground text-center py-4">暂无用例</p>
            <label v-for="tc in allCases" :key="tc.id" class="flex items-center gap-2 p-2 rounded hover:bg-accent/30 cursor-pointer">
              <input type="checkbox" :checked="editingPlan.caseIds.includes(tc.id)" @change="toggleCase(tc.id)" class="accent-primary" />
              <span class="text-sm truncate">{{ tc.name }}</span>
              <span class="text-xs text-muted-foreground ml-auto">{{ caseSteps(tc.id) }}步</span>
            </label>
          </div>
        </div>
        <div class="flex gap-2 pt-2 border-t">
          <Button class="flex-1 gap-2" @click="saveEditing"><Save class="w-4 h-4" /> 保存</Button>
          <Button variant="outline" @click="editingPlan = null">取消</Button>
        </div>
      </CardContent>
    </Card>
  </div>

  <!-- 列表视图 -->
  <div v-else class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">测试计划</h1>
        <p class="text-muted-foreground mt-0.5">制定计划、分配执行、跟踪质量</p>
      </div>
      <Button class="gap-2" @click="startCreate"><Plus class="w-4 h-4" /> 新建计划</Button>
    </div>

    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20"><ClipboardCheck class="w-5 h-5 text-blue-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.total }}</p><p class="text-xs text-muted-foreground">计划总数</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20"><Play class="w-5 h-5 text-blue-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.inProgress }}</p><p class="text-xs text-muted-foreground">进行中</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-900/20"><CheckCircle2 class="w-5 h-5 text-emerald-600" /></div>
        <div><p class="text-2xl font-bold text-emerald-600">{{ stats.passed }}</p><p class="text-xs text-muted-foreground">通过</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-red-50 dark:bg-red-900/20"><XCircle class="w-5 h-5 text-red-600" /></div>
        <div><p class="text-2xl font-bold text-red-600">{{ stats.failed }}</p><p class="text-xs text-muted-foreground">失败</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-amber-50 dark:bg-amber-900/20"><AlertCircle class="w-5 h-5 text-amber-600" /></div>
        <div><p class="text-2xl font-bold text-amber-600">{{ stats.blocked }}</p><p class="text-xs text-muted-foreground">阻塞</p></div>
      </CardContent></Card>
    </div>

    <div class="relative max-w-sm">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <Input v-model="searchTerm" placeholder="搜索计划..." class="pl-9" />
    </div>

    <div v-if="filteredPlans.length === 0" class="text-center py-12 text-muted-foreground">
      <ClipboardCheck class="w-12 h-12 mx-auto mb-3 opacity-20" />
      <p class="text-sm">暂无测试计划</p>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <Card v-for="plan in filteredPlans" :key="plan.id" class="card-hover">
        <CardContent class="p-5">
          <div class="flex items-start justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <h3 class="font-semibold text-sm">{{ plan.name }}</h3>
                <span :class="cn('px-1.5 py-0.5 rounded text-xs font-medium', planStatusCfg[plan.status].cls)">{{ planStatusCfg[plan.status].label }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-1">{{ plan.description || '无描述' }}</p>
            </div>
            <div class="flex items-center gap-1 ml-3 shrink-0">
              <Button variant="default" size="sm" class="gap-1 h-8 text-xs" :disabled="planCaseIds(plan).length === 0" @click="runExecution('manual', '', plan)">
                <Play class="w-3 h-3" /> 执行
              </Button>
              <Button variant="ghost" size="sm" class="gap-1 h-8 text-xs" :disabled="planCaseIds(plan).length === 0" @click="startExecution(plan)">
                <ClipboardCheck class="w-3 h-3" /> 登记结果
              </Button>
              <Button variant="ghost" size="icon" class="h-7 w-7" @click="startEdit(plan)"><Edit3 class="w-3.5 h-3.5" /></Button>
              <Button variant="ghost" size="icon" class="h-7 w-7" @click="handleDelete(plan.id)"><Trash2 class="w-3.5 h-3.5 text-red-500" /></Button>
            </div>
          </div>
          <div class="mt-3 space-y-2">
            <div class="flex items-center gap-4 text-xs text-muted-foreground">
              <span class="flex items-center gap-1"><Calendar class="w-3 h-3" />{{ formatDate(plan.startDate).slice(0, 10) }} ~ {{ formatDate(plan.endDate).slice(0, 10) }}</span>
              <span class="flex items-center gap-1"><Clock class="w-3 h-3" />{{ planCaseIds(plan).length }} 用例</span>
            </div>
            <div v-if="computeProgress(plan).total > 0" class="h-2 bg-muted rounded-full overflow-hidden flex">
              <div v-if="computeProgress(plan).passed > 0" class="bg-emerald-500 transition-all duration-300" :style="{ width: (computeProgress(plan).passed / computeProgress(plan).total * 100) + '%' }" />
              <div v-if="computeProgress(plan).failed > 0" class="bg-red-500 transition-all duration-300" :style="{ width: (computeProgress(plan).failed / computeProgress(plan).total * 100) + '%' }" />
              <div v-if="computeProgress(plan).blocked > 0" class="bg-amber-500 transition-all duration-300" :style="{ width: (computeProgress(plan).blocked / computeProgress(plan).total * 100) + '%' }" />
            </div>
            <div v-else class="h-2 bg-muted rounded-full" />
            <div class="flex items-center gap-3 text-xs">
              <span class="text-emerald-600 flex items-center gap-1"><CheckCircle2 class="w-3 h-3" />{{ computeProgress(plan).passed }} 通过</span>
              <span v-if="computeProgress(plan).failed > 0" class="text-red-600 flex items-center gap-1"><XCircle class="w-3 h-3" />{{ computeProgress(plan).failed }} 失败</span>
              <span v-if="computeProgress(plan).blocked > 0" class="text-amber-600 flex items-center gap-1"><AlertCircle class="w-3 h-3" />{{ computeProgress(plan).blocked }} 阻塞</span>
              <span class="text-muted-foreground">{{ computeProgress(plan).done }}/{{ computeProgress(plan).total }} 已完成</span>
            </div>
          </div>
          <div class="mt-2 flex flex-wrap gap-1">
            <span v-for="cid in planCaseIds(plan).slice(0, 6)" :key="cid" class="text-xs px-1.5 py-0.5 rounded bg-muted/50 text-muted-foreground truncate max-w-[120px]">{{ caseName(cid) }}</span>
            <span v-if="planCaseIds(plan).length > 6" class="text-xs text-muted-foreground">+{{ planCaseIds(plan).length - 6 }}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
