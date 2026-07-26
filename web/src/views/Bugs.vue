<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Bug, Plus, Trash2, Edit3, Search, CheckCircle2,
  XCircle, AlertTriangle, Clock, User, Tag, ArrowUpDown, Download, Upload,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import * as bugsApi from '@/api/bugs'
import { safeParseJSON, formatDate, downloadText } from '@/utils'
import type { Bug as BugType, BugSeverity, BugStatus } from '@/types'

type BugSource = 'manual' | 'api-test' | 'case' | 'automation' | 'sync'

const severityCfg: Record<BugSeverity, { label: string; cls: string }> = {
  critical: { label: '致命', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
  major: { label: '严重', cls: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  minor: { label: '一般', cls: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  trivial: { label: '轻微', cls: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400' },
}

const statusCfg: Record<BugStatus, { label: string; variant: 'success' | 'secondary' | 'destructive' | 'outline' }> = {
  open: { label: '待处理', variant: 'secondary' },
  in_progress: { label: '处理中', variant: 'success' },
  resolved: { label: '已解决', variant: 'outline' },
  closed: { label: '已关闭', variant: 'success' },
  reopened: { label: '重新打开', variant: 'destructive' },
}

const statusFlow: BugStatus[] = ['open', 'in_progress', 'resolved', 'closed']

const sourceLabelMap: Record<BugSource, { label: string; cls: string }> = {
  manual: { label: '手动录入', cls: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400' },
  'api-test': { label: '接口缺陷', cls: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400' },
  case: { label: '功能缺陷', cls: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400' },
  automation: { label: '自动化缺陷', cls: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400' },
  sync: { label: '外部同步', cls: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' },
}

function bugSource(b: BugType): BugSource {
  if (b.externalId) return 'sync'
  if (b.relatedCaseId) return 'case'
  return 'manual'
}
function getSourceLabel(source: BugSource) {
  return sourceLabelMap[source] || sourceLabelMap.manual
}
function bugTags(b: BugType): string[] {
  return safeParseJSON<string[]>(b.tags, [])
}

const bugs = ref<BugType[]>([])
const loading = ref(false)
const searchTerm = ref('')
const filterStatus = ref('all')
const filterSeverity = ref('all')
const filterSource = ref('all')
const selectedBugId = ref<string | null>(null)
const selectedBugIds = ref<Set<string>>(new Set())

// Jira 配置状态（控制同步按钮可用性）
const jiraConfigured = ref(false)
const jiraBaseUrl = ref('')
async function loadJiraStatus(): Promise<void> {
  try {
    const s = await bugsApi.getJiraStatus()
    jiraConfigured.value = !!s.configured
    jiraBaseUrl.value = s.baseUrl || ''
  } catch {
    jiraConfigured.value = false
  }
}

const selectedBug = computed(() => bugs.value.find((b) => b.id === selectedBugId.value) || null)

const filteredBugs = computed(() =>
  bugs.value.filter((b) => {
    const kw = searchTerm.value.trim().toLowerCase()
    if (kw && !b.title.toLowerCase().includes(kw) && !b.assignee.includes(searchTerm.value)) return false
    if (filterStatus.value !== 'all' && b.status !== filterStatus.value) return false
    if (filterSeverity.value !== 'all' && b.severity !== filterSeverity.value) return false
    if (filterSource.value !== 'all' && bugSource(b) !== filterSource.value) return false
    return true
  }),
)

const stats = computed(() => ({
  total: bugs.value.length,
  critical: bugs.value.filter((b) => b.severity === 'critical' || b.severity === 'major').length,
  unResolved: bugs.value.filter((b) => !['resolved', 'closed'].includes(b.status)).length,
  mine: bugs.value.filter((b) => b.assignee === '当前用户' || b.assignee === '张三').length,
}))

/* 编辑 / 新建 */
const editingBug = ref<Partial<BugType> | null>(null)
const isCreating = ref(false)

function blankBug(): Partial<BugType> {
  return {
    title: '', description: '', steps: '', expected: '', actual: '',
    severity: 'major', priority: 'P2', status: 'open', assignee: '', reporter: '当前用户',
    module: '', relatedCaseId: '', env: '', tags: '[]',
  }
}
function startCreate(): void {
  editingBug.value = blankBug()
  isCreating.value = true
}
function startEdit(bug: BugType): void {
  editingBug.value = { ...bug }
  isCreating.value = false
}

async function saveEditing(): Promise<void> {
  const e = editingBug.value
  if (!e || !e.title?.trim()) { ElMessage.warning('请输入缺陷标题'); return }
  const payload: Partial<BugType> = {
    title: e.title, description: e.description, steps: e.steps, expected: e.expected, actual: e.actual,
    severity: e.severity, priority: e.priority, status: e.status, assignee: e.assignee,
    reporter: e.reporter, module: e.module, env: e.env, tags: e.tags,
  }
  if (isCreating.value) {
    await bugsApi.createBug(payload)
    ElMessage.success('已创建')
  } else if (e.id) {
    await bugsApi.updateBug(e.id, payload)
    ElMessage.success('已更新')
  }
  editingBug.value = null
  await loadAll()
}

async function handleDelete(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除此缺陷？', '提示', { type: 'warning' })
    await bugsApi.deleteBug(id)
    if (selectedBugId.value === id) selectedBugId.value = null
    ElMessage.success('已删除')
    await loadAll()
  } catch { /* cancelled */ }
}

async function advanceStatus(bug: BugType): Promise<void> {
  const idx = statusFlow.indexOf(bug.status)
  if (idx < statusFlow.length - 1) {
    await bugsApi.updateBug(bug.id, { status: statusFlow[idx + 1] })
    await loadAll()
  }
}
async function rejectBug(bug: BugType): Promise<void> {
  await bugsApi.updateBug(bug.id, { status: 'closed' })
  await loadAll()
}

async function syncToExternal(bug: BugType, platform: 'jira' | 'feishu' | 'wecom'): Promise<void> {
  if (platform !== 'jira') {
    ElMessage.warning(`${platform === 'feishu' ? '飞书' : '企微'}同步暂未支持`)
    return
  }
  try {
    await bugsApi.syncBug(bug.id)
    ElMessage.success('已同步到 Jira')
    await loadAll()
  } catch (e) {
    ElMessage.error((e as Error)?.message || '同步失败')
  }
}
async function syncSelected(platform: 'jira' | 'feishu' | 'wecom'): Promise<void> {
  const ids = Array.from(selectedBugIds.value)
  if (ids.length === 0) { ElMessage.warning('请先勾选要同步的缺陷'); return }
  for (const id of ids) {
    const b = bugs.value.find((x) => x.id === id)
    if (b) await syncToExternal(b, platform)
  }
  selectedBugIds.value = new Set()
}
function toggleSelectBug(id: string): void {
  const next = new Set(selectedBugIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedBugIds.value = next
}

function exportJson(): void {
  downloadText(`bugs-${Date.now()}.json`, JSON.stringify(bugs.value, null, 2), 'application/json')
  ElMessage.success('已导出')
}

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    bugs.value = await bugsApi.getBugs()
  } catch (err) {
    ElMessage.error((err as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(() => {
  loadAll()
  loadJiraStatus()
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">缺陷管理</h1>
        <p class="text-muted-foreground mt-0.5">跟踪和管理缺陷全生命周期</p>
      </div>
      <div class="flex gap-2">
        <template v-if="!jiraConfigured">
          <span class="self-center text-xs text-muted-foreground px-2 py-1 rounded bg-muted">Jira 未配置，请在「系统设置 → Jira 同步」填写</span>
        </template>
        <template v-if="selectedBugIds.size > 0">
          <Button variant="outline" size="sm" class="gap-1" :disabled="!jiraConfigured" :title="jiraConfigured ? '' : 'Jira 未配置'" @click="syncSelected('jira')">
            <Upload class="w-3.5 h-3.5" /> 同步 Jira ({{ selectedBugIds.size }})
          </Button>
          <Button variant="outline" size="sm" class="gap-1" disabled title="暂未支持" @click="syncSelected('feishu')">
            <Upload class="w-3.5 h-3.5" /> 同步飞书 ({{ selectedBugIds.size }})
          </Button>
          <Button variant="outline" size="sm" class="gap-1" disabled title="暂未支持" @click="syncSelected('wecom')">
            <Upload class="w-3.5 h-3.5" /> 同步企微 ({{ selectedBugIds.size }})
          </Button>
        </template>
        <Button variant="outline" class="gap-2" @click="exportJson"><Download class="w-4 h-4" /> 导出</Button>
        <Button class="gap-2" @click="startCreate"><Plus class="w-4 h-4" /> 新建缺陷</Button>
      </div>
    </div>

    <!-- Stats -->
    <div class="grid gap-4 md:grid-cols-4">
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20"><Bug class="w-5 h-5 text-blue-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.total }}</p><p class="text-xs text-muted-foreground">缺陷总数</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-red-50 dark:bg-red-900/20"><AlertTriangle class="w-5 h-5 text-red-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.critical }}</p><p class="text-xs text-muted-foreground">致命/严重</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-amber-50 dark:bg-amber-900/20"><XCircle class="w-5 h-5 text-amber-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.unResolved }}</p><p class="text-xs text-muted-foreground">未解决</p></div>
      </CardContent></Card>
      <Card><CardContent class="p-4 flex items-center gap-3">
        <div class="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-900/20"><User class="w-5 h-5 text-emerald-600" /></div>
        <div><p class="text-2xl font-bold">{{ stats.mine }}</p><p class="text-xs text-muted-foreground">我的缺陷</p></div>
      </CardContent></Card>
    </div>

    <div class="flex gap-6">
      <!-- Left: Bug List -->
      <div class="flex-1 space-y-3">
        <div class="flex items-center gap-3">
          <div class="relative flex-1">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input v-model="searchTerm" placeholder="搜索缺陷标题、负责人..." class="pl-9" />
          </div>
          <select v-model="filterStatus" class="h-10 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部状态</option>
            <option v-for="(v, k) in statusCfg" :key="k" :value="k">{{ v.label }}</option>
          </select>
          <select v-model="filterSeverity" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部级别</option>
            <option v-for="(v, k) in severityCfg" :key="k" :value="k">{{ v.label }}</option>
          </select>
          <select v-model="filterSource" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
            <option value="all">全部来源</option>
            <option v-for="(v, k) in sourceLabelMap" :key="k" :value="k">{{ v.label }}</option>
          </select>
        </div>

        <div v-if="filteredBugs.length === 0" class="text-center py-12 text-muted-foreground">
          <Bug class="w-12 h-12 mx-auto mb-3 opacity-20" />
          <p class="text-sm">暂无缺陷</p>
        </div>

        <div class="space-y-2">
          <div
            v-for="bug in filteredBugs"
            :key="bug.id"
            :class="cn(
              'p-4 rounded-lg border cursor-pointer transition-all hover:shadow-sm',
              selectedBugId === bug.id ? 'border-primary ring-1 ring-primary bg-primary/5' : 'border-border',
            )"
            @click="selectedBugId = bug.id"
          >
            <div class="flex items-start gap-3">
              <input
                type="checkbox"
                :checked="selectedBugIds.has(bug.id)"
                class="mt-1 w-3.5 h-3.5 accent-primary shrink-0"
                @click.stop
                @change="toggleSelectBug(bug.id)"
              />
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold shrink-0', severityCfg[bug.severity].cls)">
                    {{ severityCfg[bug.severity].label }}
                  </span>
                  <span class="text-sm font-medium truncate">{{ bug.title }}</span>
                  <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold shrink-0', getSourceLabel(bugSource(bug)).cls)">
                    {{ getSourceLabel(bugSource(bug)).label }}
                  </span>
                </div>
                <div class="flex items-center gap-3 mt-1.5 text-xs text-muted-foreground">
                  <Badge :variant="statusCfg[bug.status].variant" class="text-xs">{{ statusCfg[bug.status].label }}</Badge>
                  <span class="flex items-center gap-1"><User class="w-3 h-3" />{{ bug.assignee || '未指派' }}</span>
                  <span class="flex items-center gap-1"><Clock class="w-3 h-3" />{{ formatDate(bug.createdAt).slice(0, 10) }}</span>
                  <span v-for="t in bugTags(bug)" :key="t" class="text-xs px-1.5 py-0.5 rounded bg-muted">{{ t }}</span>
                </div>
              </div>
              <div class="flex items-center gap-1 ml-3 shrink-0">
                <Button
                  v-if="bug.status !== 'resolved' && bug.status !== 'closed'"
                  variant="ghost" size="icon" class="h-7 w-7" title="流转到下一个状态"
                  @click.stop="advanceStatus(bug)"
                >
                  <ArrowUpDown class="w-3.5 h-3.5" />
                </Button>
                <Button variant="ghost" size="icon" class="h-7 w-7" @click.stop="startEdit(bug)">
                  <Edit3 class="w-3.5 h-3.5" />
                </Button>
                <Button variant="ghost" size="icon" class="h-7 w-7" @click.stop="handleDelete(bug.id)">
                  <Trash2 class="w-3.5 h-3.5 text-red-500" />
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: Detail / Editor -->
      <div class="w-[500px] shrink-0">
        <Card v-if="editingBug">
          <CardHeader><CardTitle class="text-base">{{ isCreating ? '新建缺陷' : '编辑缺陷' }}</CardTitle></CardHeader>
          <CardContent class="space-y-3">
            <div>
              <label class="text-sm font-medium mb-1 block">标题 *</label>
              <Input v-model="editingBug.title" placeholder="简要描述缺陷" />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-sm font-medium mb-1 block">严重程度</label>
                <select v-model="editingBug.severity" class="w-full h-10 rounded-md border border-input bg-background px-3 text-sm">
                  <option v-for="(v, k) in severityCfg" :key="k" :value="k">{{ v.label }}</option>
                </select>
              </div>
              <div>
                <label class="text-sm font-medium mb-1 block">优先级</label>
                <select v-model="editingBug.priority" class="w-full h-10 rounded-md border border-input bg-background px-3 text-sm">
                  <option value="P0">P0 紧急</option><option value="P1">P1 高</option><option value="P2">P2 中</option><option value="P3">P3 低</option>
                </select>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-sm font-medium mb-1 block">负责人</label>
                <Input v-model="editingBug.assignee" placeholder="指派给谁" />
              </div>
              <div>
                <label class="text-sm font-medium mb-1 block">模块</label>
                <Input v-model="editingBug.module" placeholder="所属模块" />
              </div>
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">状态</label>
              <select v-model="editingBug.status" class="w-full h-10 rounded-md border border-input bg-background px-3 text-sm">
                <option v-for="(v, k) in statusCfg" :key="k" :value="k">{{ v.label }}</option>
              </select>
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">描述</label>
              <textarea v-model="editingBug.description" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" placeholder="详细描述缺陷" />
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">复现步骤</label>
              <textarea v-model="editingBug.steps" class="w-full h-20 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none font-mono" placeholder="1. ...&#10;2. ...&#10;3. ..." />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-sm font-medium mb-1 block">预期结果</label>
                <textarea v-model="editingBug.expected" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" />
              </div>
              <div>
                <label class="text-sm font-medium mb-1 block">实际结果</label>
                <textarea v-model="editingBug.actual" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" />
              </div>
            </div>
            <div>
              <label class="text-sm font-medium mb-1 block">环境</label>
              <Input v-model="editingBug.env" placeholder="例如 Dev / Staging / Production" />
            </div>
            <div class="flex gap-2 pt-2 border-t">
              <Button class="flex-1 gap-2" @click="saveEditing">保存</Button>
              <Button variant="outline" @click="editingBug = null">取消</Button>
            </div>
          </CardContent>
        </Card>

        <Card v-else-if="selectedBug">
          <CardHeader class="pb-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Badge :variant="statusCfg[selectedBug.status].variant" class="text-xs">{{ statusCfg[selectedBug.status].label }}</Badge>
                <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', severityCfg[selectedBug.severity].cls)">{{ severityCfg[selectedBug.severity].label }}</span>
              </div>
              <div class="flex gap-1">
                <Button variant="ghost" size="icon" class="h-7 w-7" @click="startEdit(selectedBug)"><Edit3 class="w-3.5 h-3.5" /></Button>
                <Button variant="ghost" size="icon" class="h-7 w-7" @click="selectedBug.status !== 'resolved' && selectedBug.status !== 'closed' && advanceStatus(selectedBug)"><ArrowUpDown class="w-3.5 h-3.5" /></Button>
              </div>
            </div>
          </CardHeader>
          <CardContent class="space-y-3 text-sm">
            <h3 class="font-semibold text-base">{{ selectedBug.title }}</h3>
            <p class="text-muted-foreground">{{ selectedBug.description || '无描述' }}</p>
            <div class="grid grid-cols-2 gap-2 text-xs">
              <div><span class="text-muted-foreground">负责人：</span>{{ selectedBug.assignee || '未指派' }}</div>
              <div><span class="text-muted-foreground">模块：</span>{{ selectedBug.module || '—' }}</div>
              <div><span class="text-muted-foreground">优先级：</span>{{ selectedBug.priority }}</div>
              <div><span class="text-muted-foreground">环境：</span>{{ selectedBug.env || '—' }}</div>
              <div><span class="text-muted-foreground">创建时间：</span>{{ formatDate(selectedBug.createdAt) }}</div>
              <div><span class="text-muted-foreground">更新时间：</span>{{ formatDate(selectedBug.updatedAt) }}</div>
            </div>
            <div v-if="selectedBug.steps">
              <p class="text-sm font-medium mb-1">复现步骤</p>
              <pre class="text-xs bg-muted/50 p-3 rounded-lg whitespace-pre-wrap">{{ selectedBug.steps }}</pre>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div v-if="selectedBug.expected"><p class="text-xs font-medium text-muted-foreground mb-1">预期结果</p><p class="text-sm bg-green-50 dark:bg-green-900/10 p-2 rounded">{{ selectedBug.expected }}</p></div>
              <div v-if="selectedBug.actual"><p class="text-xs font-medium text-muted-foreground mb-1">实际结果</p><p class="text-sm bg-red-50 dark:bg-red-900/10 p-2 rounded">{{ selectedBug.actual }}</p></div>
            </div>
            <div v-if="bugTags(selectedBug).length > 0" class="flex items-center gap-2">
              <Tag class="w-3.5 h-3.5 text-muted-foreground" />
              <span v-for="t in bugTags(selectedBug)" :key="t" class="text-xs px-2 py-0.5 rounded bg-muted">{{ t }}</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span :class="cn('px-1.5 py-0.5 rounded text-xs font-bold', getSourceLabel(bugSource(selectedBug)).cls)">{{ getSourceLabel(bugSource(selectedBug)).label }}</span>
              <Badge v-if="selectedBug.externalId" variant="success" class="text-xs gap-1">已同步 ✓</Badge>
            </div>
            <div class="flex gap-2 pt-2">
              <template v-if="selectedBug.status !== 'resolved' && selectedBug.status !== 'closed'">
                <Button size="sm" class="gap-1" @click="advanceStatus(selectedBug)">
                  <CheckCircle2 class="w-3.5 h-3.5" /> 流转下一步
                </Button>
                <Button variant="outline" size="sm" class="gap-1 text-red-500" @click="rejectBug(selectedBug)">
                  <XCircle class="w-3.5 h-3.5" /> 关闭
                </Button>
              </template>
              <div class="ml-auto flex gap-1">
                <Button variant="outline" size="sm" class="gap-1" :disabled="!jiraConfigured" :title="jiraConfigured ? '同步到 Jira' : 'Jira 未配置'" @click="syncToExternal(selectedBug, 'jira')">
                  <Upload class="w-3.5 h-3.5" /> Jira
                </Button>
                <Button variant="outline" size="sm" class="gap-1" disabled title="暂未支持" @click="syncToExternal(selectedBug, 'feishu')">
                  <Upload class="w-3.5 h-3.5" /> 飞书
                </Button>
                <Button variant="outline" size="sm" class="gap-1" disabled title="暂未支持" @click="syncToExternal(selectedBug, 'wecom')">
                  <Upload class="w-3.5 h-3.5" /> 企微
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <div v-else class="flex items-center justify-center h-64 text-muted-foreground">
          <div class="text-center">
            <Bug class="w-12 h-12 mx-auto mb-3 opacity-20" />
            <p class="text-sm">选择缺陷查看详情，或点击「新建缺陷」</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
