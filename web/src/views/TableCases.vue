<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Search, Plus, Trash2, FileSpreadsheet, AlertCircle, FolderPlus, X, PenLine, ListChecks,
  ClipboardPaste, Keyboard,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import CaseEditorDrawer from '@/components/CaseEditorDrawer.vue'
import * as tableApi from '@/api/table'
import type { TableCase, TableModule } from '@/types'

const TYPE_LABELS: Record<string, string> = {
  functional: '功能测试',
  performance: '性能测试',
  security: '安全测试',
  compatibility: '兼容性测试',
  smoke: '冒烟测试',
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

const cases = ref<TableCase[]>([])
const modules = ref<TableModule[]>([])
const search = ref('')
const selectedModule = ref<string | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const showNewModule = ref(false)
const newModuleName = ref('')

// 抽屉编辑器状态（用于编辑步骤等详情）
const showDrawer = ref(false)
const editingCase = ref<TableCase | null>(null)

// ===== WPS 风格交互状态 =====
// 列定义（含电子表格列头，方便键盘导航）
const COLUMNS = [
  { key: 'name', label: '名称' },
  { key: 'moduleId', label: '模块' },
  { key: 'priority', label: '优先级' },
  { key: 'type', label: '类型' },
  { key: 'assignee', label: '负责人' },
  { key: 'steps', label: '步骤' },
  { key: 'updatedAt', label: '更新时间' },
] as const
type ColKey = (typeof COLUMNS)[number]['key']

// 当前激活单元格（行下标 + 列 key）；-1 表示无
const activeRow = ref(-1)
const activeCol = ref<ColKey | null>(null)
// 新增行草稿（WPS 式：底部一行直接输入即新建）
const draft = ref({ name: '', moduleId: '', priority: 'P1' as string, assignee: '', type: 'functional' as string })
const savingDraft = ref(false)

// 所有可聚焦单元格的引用（键盘导航需要）
// Input 组件实例（已 expose focus/select）或原生 select 元素
const cellEls = ref<Record<string, { focus: () => void; select?: () => void } | null>>({})
function cellKey(i: number, col: ColKey): string { return `${i}:${col}` }

function moduleName(id: string): string {
  return modules.value.find((m) => m.id === id)?.name || '未分类'
}

/** 兼容 JSON 结构化步骤与旧的纯文本换行步骤，返回步骤条数 */
function stepCount(s: string | undefined): number {
  if (!s || !s.trim()) return 0
  const t = s.trim()
  if (t.startsWith('[')) {
    try {
      const arr = JSON.parse(t)
      if (Array.isArray(arr)) return arr.filter((x) => x && (x.action || x.expected)).length
    } catch {
      /* fallthrough */
    }
  }
  return t.split('\n').filter((l) => l.trim().length > 0).length
}

const filtered = computed(() =>
  cases.value.filter((c) => {
    const matchSearch = !search.value || c.name.includes(search.value) || moduleName(c.moduleId).includes(search.value)
    const matchModule = !selectedModule.value || c.moduleId === selectedModule.value
    return matchSearch && matchModule
  }),
)

async function loadAll(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const [casesData, modulesData] = await Promise.all([
      tableApi.getTableCases(),
      tableApi.getTableModules(),
    ])
    cases.value = casesData
    modules.value = modulesData
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(loadAll)

async function handleAddModule(): Promise<void> {
  if (!newModuleName.value.trim()) return
  try {
    const m = await tableApi.createTableModule({ name: newModuleName.value.trim() })
    modules.value = [...modules.value, m]
    newModuleName.value = ''
    showNewModule.value = false
    ElMessage.success('模块已创建')
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  }
}

async function handleDeleteModule(id: string): Promise<void> {
  try {
    await tableApi.deleteTableModule(id)
    modules.value = modules.value.filter((m) => m.id !== id)
    if (selectedModule.value === id) selectedModule.value = null
    ElMessage.success('模块已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

// ---- 抽屉：新建 / 编辑（详情：步骤、前置条件等）----
function openCreate(): void {
  editingCase.value = null
  showDrawer.value = true
}
function openEdit(c: TableCase): void {
  editingCase.value = c
  showDrawer.value = true
}

/** 供抽屉调用的保存逻辑 */
async function submitCase(payload: Record<string, unknown>, isEdit: boolean): Promise<void> {
  if (isEdit && editingCase.value) {
    await tableApi.updateTableCase(editingCase.value.id, payload as Partial<TableCase>)
  } else {
    await tableApi.createTableCase(payload as Partial<TableCase>)
  }
  await loadAll()
}

/** 简单字段的行内编辑（名称 / 模块 / 优先级 / 负责人）即时保存 */
async function saveInline(c: TableCase): Promise<void> {
  try {
    await tableApi.updateTableCase(c.id, {
      name: c.name,
      moduleId: c.moduleId,
      priority: c.priority,
      type: c.type,
      precondition: c.precondition,
      steps: c.steps,
      assignee: c.assignee,
      status: c.status,
      tags: c.tags,
    })
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
    loadAll()
  }
}

async function handleDeleteCase(id: string): Promise<void> {
  try {
    await tableApi.deleteTableCase(id)
    cases.value = cases.value.filter((c) => c.id !== id)
    ElMessage.success('用例已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

// ==================== WPS 式交互逻辑 ====================

/** 点击单元格：选中并聚焦 */
function selectCell(i: number, col: ColKey): void {
  activeRow.value = i
  activeCol.value = col
  const el = cellEls.value[cellKey(i, col)]
  if (el) {
    el.focus()
    el.select?.()
  }
}

/** 键盘导航：方向键移动，Enter 下移，Tab 右移（仿电子表格） */
function onCellKeydown(e: KeyboardEvent, i: number, col: ColKey): void {
  const cols = COLUMNS
  let row = i
  let c = cols.findIndex((x) => x.key === col)
  const total = filtered.value.length

  const move = (r: number, cc: number): boolean => {
    if (cc < 0) { c = 0; return false }
    if (cc >= cols.length) { c = cols.length - 1; return false }
    if (r < 0) { row = 0; return false }
    if (r >= total) { row = total - 1; return false }
    row = r
    c = cc
    return true
  }

  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      move(row + 1, c) || move(row, c)
      break
    case 'ArrowUp':
      e.preventDefault()
      move(row - 1, c) || move(row, c)
      break
    case 'ArrowLeft':
      e.preventDefault()
      if (c > 0) c--
      else if (row > 0) { row--; c = cols.length - 1 }
      break
    case 'ArrowRight':
      e.preventDefault()
      if (c < cols.length - 1) c++
      else if (row < total - 1) { row++; c = 0 }
      break
    case 'Enter':
      e.preventDefault()
      move(row + 1, c) || move(row, c)
      break
    case 'Tab':
      e.preventDefault()
      if (e.shiftKey) {
        if (c > 0) c--
        else if (row > 0) { row--; c = cols.length - 1 }
      } else {
        if (c < cols.length - 1) c++
        else if (row < total - 1) { row++; c = 0 }
      }
      break
    default:
      return // 不消费其他按键（保留输入）
  }
  activeRow.value = row
  activeCol.value = cols[c].key
  nextTick(() => {
    const el = cellEls.value[cellKey(row, cols[c].key)]
    el?.focus()
    el?.select?.()
  })
}

/** 底部新增行：Enter 或点击「添加行」保存草稿（WPS 式，输入即新建） */
async function saveDraft(): Promise<void> {
  if (savingDraft.value) return
  const name = draft.value.name.trim()
  if (!name) { ElMessage.warning('请输入用例名称'); return }
  savingDraft.value = true
  try {
    await tableApi.createTableCase({
      name,
      moduleId: draft.value.moduleId || selectedModule.value || '',
      priority: draft.value.priority as TableCase['priority'],
      type: draft.value.type,
      assignee: draft.value.assignee,
      precondition: '',
      steps: '[]',
      status: 'draft',
      tags: '[]',
    })
    draft.value = { name: '', moduleId: '', priority: 'P1', assignee: '', type: 'functional' }
    ElMessage.success('用例已添加')
    await loadAll()
    // 聚焦到新行首列（最后一行）
    await nextTick()
    if (filtered.value.length > 0) selectCell(filtered.value.length - 1, 'name')
  } catch (e: any) {
    ElMessage.error(e.message || '添加失败')
  } finally {
    savingDraft.value = false
  }
}

/** 从 Excel / WPS 剪贴板粘贴多行：按 Tab 分列、按换行分行，批量新建用例 */
async function handleDraftPaste(e: ClipboardEvent): Promise<void> {
  const text = (e.clipboardData?.getData('text') || '').trim()
  if (!text) return
  // 仅当粘贴内容含多行或多列时走批量逻辑；单段文本走普通输入
  if (!text.includes('\n') && !text.includes('\t')) return
  e.preventDefault()
  const rows = text.split(/\r?\n/).filter((r) => r.trim() !== '')
  if (rows.length === 0) return
  savingDraft.value = true
  let ok = 0
  try {
    for (const row of rows) {
      const cells = row.split('\t')
      const name = (cells[0] || '').trim()
      if (!name) continue
      await tableApi.createTableCase({
        name,
        moduleId: draft.value.moduleId || selectedModule.value || '',
        priority: (cells[2] as TableCase['priority']) || draft.value.priority,
        type: draft.value.type,
        assignee: (cells[1] || draft.value.assignee).trim(),
        precondition: '',
        steps: '[]',
        status: 'draft',
        tags: '[]',
      })
      ok++
    }
    draft.value = { name: '', moduleId: '', priority: 'P1', assignee: '', type: 'functional' }
    ElMessage.success(`已批量新建 ${ok} 条用例（支持从 Excel/WPS 直接粘贴多行）`)
    await loadAll()
  } catch (err: any) {
    ElMessage.error(err.message || '批量新建失败')
  } finally {
    savingDraft.value = false
  }
}
</script>

<template>
  <div class="flex h-full gap-6">
    <!-- Left - Modules -->
    <div class="w-56 shrink-0">
      <Card class="h-full">
        <CardContent class="p-3 flex flex-col h-full">
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-sm font-semibold">模块</h3>
            <Button variant="ghost" size="icon" class="h-6 w-6 rounded-lg" @click="showNewModule = true">
              <FolderPlus class="w-3.5 h-3.5" />
            </Button>
          </div>
          <div class="flex-1 overflow-y-auto space-y-0.5">
            <button
              @click="selectedModule = null"
              :class="`w-full text-left px-2 py-1.5 rounded-lg text-xs transition-colors ${
                !selectedModule ? 'bg-primary/10 text-primary font-medium' : 'hover:bg-accent'
              }`"
            >
              全部
            </button>
            <div v-for="m in modules" :key="m.id" class="group flex items-center">
              <button
                @click="selectedModule = m.id"
                :class="`flex-1 text-left px-2 py-1.5 rounded-lg text-xs transition-colors truncate ${
                  selectedModule === m.id ? 'bg-primary/10 text-primary font-medium' : 'hover:bg-accent'
                }`"
              >
                {{ m.name }}
              </button>
              <button
                @click="handleDeleteModule(m.id)"
                class="opacity-0 group-hover:opacity-100 shrink-0 px-1 text-muted-foreground hover:text-destructive"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Right - Spreadsheet -->
    <div class="flex-1 flex flex-col gap-4">
      <Card class="flex-1">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <FileSpreadsheet class="w-4 h-4 text-emerald-500" />
              <h3 class="text-sm font-semibold">
                表格用例
                <span class="text-muted-foreground font-normal ml-1">({{ filtered.length }})</span>
              </h3>
              <span class="inline-flex items-center gap-1 ml-1 px-2 py-0.5 rounded-full bg-muted/60 text-[10px] text-muted-foreground">
                <Keyboard class="w-3 h-3" /> 方向键/Enter/Tab 导航
              </span>
            </div>
            <div class="flex items-center gap-2">
              <div class="relative">
                <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input v-model="search" placeholder="搜索用例..." class="h-8 pl-8 text-xs w-48" />
              </div>
              <Button size="sm" class="h-8 rounded-lg text-xs gap-1.5" @click="openCreate">
                <Plus class="w-3 h-3" /> 新建用例
              </Button>
            </div>
          </div>

          <div v-if="error" class="flex items-center gap-2 p-3 mb-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-xs">
            <AlertCircle class="w-4 h-4" /> {{ error }}
          </div>

          <div class="flex-1 overflow-auto border rounded-lg">
            <table class="w-full text-sm border-collapse">
              <thead>
                <tr class="bg-muted/50 text-left text-xs text-muted-foreground sticky top-0 z-10">
                  <th class="w-9 px-0 py-2 font-medium border-b border-r text-center select-none">#</th>
                  <th class="px-2 py-2 font-medium border-b border-r">名称</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-32">模块</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-24">优先级</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-28">类型</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-28">负责人</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-24 text-center">步骤</th>
                  <th class="px-2 py-2 font-medium border-b border-r w-28">更新时间</th>
                  <th class="px-2 py-2 font-medium border-b w-20 text-center">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(c, i) in filtered"
                  :key="c.id"
                  class="transition-colors"
                  :class="i === activeRow ? 'bg-primary/5' : 'hover:bg-accent/30'"
                >
                  <!-- 行号列（WPS 风格） -->
                  <td class="px-0 py-1 border-b border-r text-center align-middle select-none bg-muted/30 text-[10px] text-muted-foreground">
                    {{ i + 1 }}
                  </td>
                  <td class="px-1 py-1 border-b border-r align-middle">
                    <Input
                      :ref="(el) => (cellEls[cellKey(i, 'name')] = el as never)"
                      v-model="c.name"
                      class="h-8 text-xs border-transparent bg-transparent focus:bg-background"
                      @blur="saveInline(c)"
                      @click.stop="selectCell(i, 'name')"
                      @keydown="onCellKeydown($event, i, 'name')"
                    />
                  </td>
                  <td class="px-1 py-1 border-b border-r align-middle">
                    <select
                      :ref="(el) => (cellEls[cellKey(i, 'moduleId')] = el as never)"
                      :value="c.moduleId"
                      @change="c.moduleId = ($event.target as HTMLSelectElement).value; saveInline(c)"
                      class="w-full h-8 rounded-md border border-transparent bg-transparent px-1 text-xs outline-none focus:bg-background focus:border-input"
                      @click.stop="selectCell(i, 'moduleId')"
                      @keydown="onCellKeydown($event, i, 'moduleId')"
                    >
                      <option value="">未分类</option>
                      <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.name }}</option>
                    </select>
                  </td>
                  <td class="px-1 py-1 border-b border-r align-middle">
                    <select
                      :ref="(el) => (cellEls[cellKey(i, 'priority')] = el as never)"
                      v-model="c.priority"
                      @change="saveInline(c)"
                      class="w-full h-8 rounded-md border border-transparent bg-transparent px-1 text-xs outline-none focus:bg-background focus:border-input"
                      @click.stop="selectCell(i, 'priority')"
                      @keydown="onCellKeydown($event, i, 'priority')"
                    >
                      <option value="P0">P0</option>
                      <option value="P1">P1</option>
                      <option value="P2">P2</option>
                      <option value="P3">P3</option>
                    </select>
                  </td>
                  <td class="px-2 py-1 border-b border-r align-middle text-xs text-muted-foreground">{{ TYPE_LABELS[c.type] || c.type || '-' }}</td>
                  <td class="px-1 py-1 border-b border-r align-middle">
                    <Input
                      :ref="(el) => (cellEls[cellKey(i, 'assignee')] = el as never)"
                      v-model="c.assignee"
                      class="h-8 text-xs border-transparent bg-transparent focus:bg-background"
                      @blur="saveInline(c)"
                      @click.stop="selectCell(i, 'assignee')"
                      @keydown="onCellKeydown($event, i, 'assignee')"
                    />
                  </td>
                  <td class="px-2 py-1 border-b border-r text-center align-middle">
                    <button
                      class="inline-flex items-center gap-1 px-2 h-7 rounded-md text-xs hover:bg-accent text-muted-foreground hover:text-foreground"
                      title="查看/编辑操作步骤"
                      @click="openEdit(c)"
                    >
                      <ListChecks class="w-3.5 h-3.5" />
                      {{ stepCount(c.steps) }} 步
                    </button>
                  </td>
                  <td class="px-2 py-1 border-b border-r text-xs text-muted-foreground align-middle">{{ formatDate(c.updatedAt) }}</td>
                  <td class="px-1 py-1 border-b text-center align-middle">
                    <div class="flex items-center justify-center gap-0.5">
                      <Button variant="ghost" size="icon" class="h-8 w-8 rounded-lg text-muted-foreground hover:text-primary" title="编辑" @click="openEdit(c)">
                        <PenLine class="w-3.5 h-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon" class="h-8 w-8 rounded-lg text-muted-foreground hover:text-destructive" title="删除" @click="handleDeleteCase(c.id)">
                        <Trash2 class="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-if="!loading && filtered.length === 0" class="text-center py-12 text-sm text-muted-foreground">
              {{ search ? '无匹配用例' : '暂无数据，可在下方直接输入新建' }}
            </div>
          </div>

          <!-- 底部新增行（WPS 风格：输入即新建） -->
          <div class="mt-3 border rounded-lg bg-muted/20 px-3 py-2 flex items-end gap-2">
            <div class="flex items-center gap-1 text-xs text-muted-foreground shrink-0 pb-1.5">
              <Plus class="w-3.5 h-3.5" />
              <span>新行</span>
            </div>
            <Input
              v-model="draft.name"
              placeholder="用例名称，Enter 保存（支持从 Excel/WPS 整行粘贴批量新建）"
              class="flex-1 h-8 text-xs"
              @keydown.enter="saveDraft"
              @paste="handleDraftPaste"
            />
            <select v-model="draft.moduleId" class="h-8 rounded-md border bg-background px-1 text-xs w-32 outline-none focus:border-input">
              <option value="">默认模块</option>
              <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select>
            <select v-model="draft.priority" class="h-8 rounded-md border bg-background px-1 text-xs w-20 outline-none focus:border-input">
              <option value="P0">P0</option>
              <option value="P1">P1</option>
              <option value="P2">P2</option>
              <option value="P3">P3</option>
            </select>
            <Input v-model="draft.assignee" placeholder="负责人" class="h-8 text-xs w-24" @keydown.enter="saveDraft" />
            <Button size="sm" class="h-8 rounded-lg text-xs gap-1.5 shrink-0" :disabled="savingDraft" @click="saveDraft">
              <Plus class="w-3 h-3" /> 添加
            </Button>
          </div>
          <div class="flex items-center gap-1 mt-1.5 text-[10px] text-muted-foreground">
            <ClipboardPaste class="w-3 h-3" />
            提示：从 Excel / WPS 复制多行（每行一列用 Tab 分隔）粘贴到「新行」输入框，可一次新建多条用例
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 抽屉式用例编辑器 -->
    <CaseEditorDrawer
      v-model:visible="showDrawer"
      :modules="modules"
      :editing="editingCase"
      :default-module-id="selectedModule || ''"
      title="表格用例"
      :submit="submitCase"
    />

    <!-- New Module Modal -->
    <div v-if="showNewModule" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" @click="showNewModule = false">
      <div class="bg-popover border rounded-2xl shadow-2xl p-6 w-[420px] animate-scale-in" @click.stop>
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-base font-semibold">新建模块</h3>
          <Button variant="ghost" size="icon" @click="showNewModule = false"><X class="w-4 h-4" /></Button>
        </div>
        <Input
          v-model="newModuleName"
          placeholder="模块名称"
          class="mb-4"
          autofocus
          @keydown.enter="handleAddModule"
        />
        <div class="flex justify-end gap-2">
          <Button variant="outline" size="sm" @click="showNewModule = false">取消</Button>
          <Button size="sm" @click="handleAddModule">创建</Button>
        </div>
      </div>
    </div>
  </div>
</template>
