<script setup lang="ts">
import { ref, computed, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Plus, Trash2, AlertCircle, FolderPlus, X, Network, FolderOpen,
  ZoomIn, ZoomOut, Maximize, RotateCcw, PenLine,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import CaseEditorDrawer from '@/components/CaseEditorDrawer.vue'
import * as xmindApi from '@/api/xmind'
import type { XmindCase, XmindModule } from '@/types'

const PRIORITY_COLORS: Record<string, string> = {
  P0: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
  P1: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  P2: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  P3: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400',
}

// ---- Layout constants（逻辑图：根 → 模块 → 用例 三级右发） ----
const ROOT_W = 148
const ROOT_H = 48
const MODULE_X = 236
const MODULE_W = 196
const MODULE_H = 42
const CASE_X = 516
const CASE_W = 232
const CASE_H = 38
const CASE_GAP = 56
const MODULE_GAP = 22 // 模块分支间距
const TOP = 32
const PAD_BOTTOM = 32

type NodeKind = 'root' | 'module' | 'case'

interface MindNode {
  kind: NodeKind
  id: string
  name: string
  x: number
  y: number
  w: number
  h: number
  priority?: string
  stepCount?: number
  // 父节点引用（键盘 ← 返回父层用）
  parentKind?: NodeKind
  parentId?: string
}

interface MindView {
  nodes: MindNode[]
  links: string[]
  width: number
  height: number
  rootY: number
}

/** 平滑贝塞尔曲线（XMind 逻辑图风格连线） */
function curve(x1: number, y1: number, x2: number, y2: number): string {
  const dx = Math.max(24, (x2 - x1) * 0.55)
  return `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`
}

// ---- 数据 ----
const cases = ref<XmindCase[]>([])
const modules = ref<XmindModule[]>([])
const search = ref('')
const selectedModule = ref<string | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

// ---- 交互状态 ----
const selected = ref<{ kind: NodeKind; id: string } | null>(null)
const editing = ref<{ kind: NodeKind; id: string; isNew: boolean; moduleId?: string } | null>(null)
const editName = ref('')
// Input 组件实例（已 defineExpose focus/select）
const editInput = ref<{ focus: () => void; select: () => void } | null>(null)

// 缩放 / 平移（仿 XMind）
const zoom = ref(1)
const panX = ref(0)
const panY = ref(0)
const canvasRef = ref<HTMLDivElement | null>(null)
// 拖拽平移
const dragging = ref(false)
const dragStart = ref({ x: 0, y: 0, px: 0, py: 0 })

// 模块 / 用例抽屉（编辑步骤等详情）
const showDrawer = ref(false)
const editingCase = ref<XmindCase | null>(null)

const grouped = computed(() => {
  const map: Record<string, XmindCase[]> = {}
  const q = search.value.trim().toLowerCase()
  cases.value.forEach((c) => {
    if (q && !c.name.toLowerCase().includes(q)) return
    ;(map[c.moduleId] ||= []).push(c)
  })
  return map
})

const activeModules = computed(() =>
  selectedModule.value ? modules.value.filter((m) => m.id === selectedModule.value) : modules.value,
)

const visibleCount = computed(() =>
  activeModules.value.reduce((s, m) => s + (grouped.value[m.id]?.length || 0), 0),
)

/** 兼容 JSON 结构化步骤与纯文本换行步骤，返回步骤条数 */
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

/** 布局计算：返回全部节点坐标与连线 */
const view = computed<MindView>(() => {
  const branches = activeModules.value.map((m) => {
    const cs = grouped.value[m.id] || []
    return { module: m, cases: cs }
  })

  // 模块分支高度 = 模块节点 + 子用例列表
  const branchHeights = branches.map((b) => {
    const casesH = b.cases.length > 0 ? b.cases.length * CASE_GAP : CASE_GAP
    return Math.max(MODULE_H, casesH)
  })
  const totalHeight = TOP + PAD_BOTTOM + branchHeights.reduce((s, h) => s + h + MODULE_GAP, 0) - MODULE_GAP

  const nodes: MindNode[] = []
  const links: string[] = []
  let y = TOP

  branches.forEach((b, bi) => {
    const branchTop = y
    const casesH = b.cases.length > 0 ? b.cases.length * CASE_GAP : CASE_GAP
    const moduleY = branchTop + (casesH - MODULE_H) / 2

    // 模块节点
    nodes.push({
      kind: 'module', id: b.module.id, name: b.module.name,
      x: MODULE_X, y: moduleY, w: MODULE_W, h: MODULE_H,
      parentKind: 'root', parentId: 'root',
    })
    links.push(curve(ROOT_W, totalHeight / 2, MODULE_X, moduleY + MODULE_H / 2))

    // 用例节点
    b.cases.forEach((c, j) => {
      const caseY = branchTop + j * CASE_GAP + (CASE_GAP - CASE_H) / 2
      nodes.push({
        kind: 'case', id: c.id, name: c.name, priority: c.priority,
        stepCount: stepCount(c.steps),
        x: CASE_X, y: caseY, w: CASE_W, h: CASE_H,
        parentKind: 'module', parentId: b.module.id,
      })
      links.push(curve(MODULE_X + MODULE_W, moduleY + MODULE_H / 2, CASE_X, caseY + CASE_H / 2))
    })
    y += casesH + MODULE_GAP
    void bi
  })

  // 根节点
  const rootY = totalHeight / 2 - ROOT_H / 2
  nodes.push({ kind: 'root', id: 'root', name: '测试用例', x: 0, y: rootY, w: ROOT_W, h: ROOT_H })

  return { nodes, links, width: CASE_X + CASE_W + 40, height: totalHeight, rootY }
})

const MIN_ZOOM = 0.2
const MAX_ZOOM = 2.5

/** 适应画布（XMind 的「适应窗口」） */
function fitView(): void {
  const el = canvasRef.value
  if (!el) return
  const cw = el.clientWidth
  const ch = el.clientHeight
  if (cw <= 0 || ch <= 0) return
  const v = view.value
  const z = Math.min((cw - 60) / v.width, (ch - 60) / v.height, 1.5)
  zoom.value = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, Math.round(z * 100) / 100))
  panX.value = (cw - v.width * zoom.value) / 2
  panY.value = (ch - v.height * zoom.value) / 2
}
function resetView(): void {
  zoom.value = 1
  panX.value = 0
  panY.value = 0
}
function zoomBy(delta: number): void {
  zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round((zoom.value + delta) * 100) / 100))
}

// ---- 加载 ----
async function loadAll(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const [casesData, modulesData] = await Promise.all([
      xmindApi.getXmindCases(),
      xmindApi.getXmindModules(),
    ])
    cases.value = casesData
    modules.value = modulesData
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(async () => {
  await loadAll()
  await nextTick()
  fitView()
})

// ---- 节点选择 ----
function findNode(kind: NodeKind, id: string): MindNode | undefined {
  return view.value.nodes.find((n) => n.kind === kind && n.id === id)
}
function selectNode(kind: NodeKind, id: string): void {
  if (!findNode(kind, id)) return
  selected.value = { kind, id }
}

/** 同层节点 id 有序列表（键盘上下导航用） */
function siblingIds(kind: NodeKind, parentKind?: NodeKind, parentId?: string): string[] {
  if (kind === 'module') return activeModules.value.map((m) => m.id)
  if (kind === 'case') return (grouped.value[parentId || ''] || []).map((c) => c.id)
  return ['root']
}
/** 父节点定位（键盘 ← 返回父层） */
function parentOf(kind: NodeKind, id: string): { kind: NodeKind; id: string } | null {
  if (kind === 'module') return { kind: 'root', id: 'root' }
  if (kind === 'case') {
    const c = cases.value.find((x) => x.id === id)
    return c ? { kind: 'module', id: c.moduleId } : null
  }
  return null
}
/** 第一个子节点（键盘 → 进入子层） */
function firstChildOf(kind: NodeKind, id: string): { kind: NodeKind; id: string } | null {
  if (kind === 'root') {
    const m = activeModules.value[0]
    return m ? { kind: 'module', id: m.id } : null
  }
  if (kind === 'module') {
    const c = (grouped.value[id] || [])[0]
    return c ? { kind: 'case', id: c.id } : null
  }
  return null
}

// ---- 就地编辑（双击 / F2 / 新建） ----
async function startEdit(kind: NodeKind, id: string, opts: { isNew?: boolean; moduleId?: string } = {}): Promise<void> {
  if (editing.value) await cancelEdit()
  let name = ''
  if (!opts.isNew) {
    if (kind === 'root') return
    const n = kind === 'module' ? modules.value.find((m) => m.id === id) : cases.value.find((c) => c.id === id)
    name = n?.name || ''
  }
  editing.value = { kind, id, isNew: !!opts.isNew, moduleId: opts.moduleId }
  editName.value = name
  await nextTick()
  editInput.value?.focus()
  editInput.value?.select()
}

function cancelEdit(): void {
  editing.value = null
  editName.value = ''
}

async function commitEdit(): Promise<void> {
  const e = editing.value
  if (!e) return
  const name = editName.value.trim()
  if (!name) { cancelEdit(); return }
  try {
    if (e.kind === 'module') {
      if (e.isNew) {
        const m = await xmindApi.createXmindModule({ name, parentId: null, sortOrder: modules.value.length })
        await loadAll()
        await nextTick()
        selectNode('module', m.id)
      } else {
        const m = modules.value.find((x) => x.id === e.id)
        if (m) {
          await xmindApi.updateXmindModule(e.id, { name, parentId: m.parentId, sortOrder: m.sortOrder })
          await loadAll()
          selectNode('module', e.id)
        }
      }
    } else if (e.kind === 'case') {
      if (e.isNew) {
        const payload = {
          name,
          moduleId: e.moduleId || selectedModule.value || '',
          priority: 'P1', type: 'functional', precondition: '', steps: '[]',
          assignee: '', status: 'draft', tags: '[]',
        }
        const c = await xmindApi.createXmindCase(payload as Partial<XmindCase>)
        await loadAll()
        await nextTick()
        selectNode('case', c.id)
      } else {
        const c = cases.value.find((x) => x.id === e.id)
        if (c) {
          // 后端全量更新，须携带原有全部字段（含 expected/sortOrder）
          await xmindApi.updateXmindCase(e.id, { ...(c as unknown as Record<string, unknown>), name } as unknown as Partial<XmindCase>)
          await loadAll()
          selectNode('case', e.id)
        }
      }
    }
    editing.value = null
  } catch (err: any) {
    ElMessage.error(err.message || '保存失败')
  }
}

// ---- 新建（快捷键 Tab/Enter） ----
/** 在选中节点下新建子节点（Tab）：根→模块，模块→用例 */
async function createChild(kind: NodeKind, id: string): Promise<void> {
  if (kind === 'root') await startEdit('module', 'new', { isNew: true })
  else if (kind === 'module') await startEdit('case', 'new', { isNew: true, moduleId: id })
}
/** 新建同级节点（Enter）：根→模块，模块→模块，用例→用例 */
async function createSibling(kind: NodeKind, id: string): Promise<void> {
  if (kind === 'root') await startEdit('module', 'new', { isNew: true })
  else if (kind === 'module') await startEdit('module', 'new', { isNew: true })
  else {
    const c = cases.value.find((x) => x.id === id)
    await startEdit('case', 'new', { isNew: true, moduleId: c?.moduleId || '' })
  }
}

// ---- 删除 ----
async function removeNode(kind: NodeKind, id: string): Promise<void> {
  try {
    if (kind === 'module') {
      await ElMessageBox.confirm('删除模块将把其中的用例移至「未分类」，确定删除？', '提示', { type: 'warning' })
      await xmindApi.deleteXmindModule(id)
    } else if (kind === 'case') {
      await ElMessageBox.confirm('确定删除此用例？', '提示', { type: 'warning' })
      await xmindApi.deleteXmindCase(id)
    } else {
      return
    }
    selected.value = null
    await loadAll()
    ElMessage.success('已删除')
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return
    ElMessage.error(e.message || '删除失败')
  }
}

// ---- 键盘快捷键（仿 XMind：Tab 子节点 / Enter 同级 / Delete 删除 / 方向键导航 / F2 编辑） ----
function onCanvasKeydown(e: KeyboardEvent): void {
  // 正在编辑时不劫持快捷键
  if (editing.value) {
    if (e.key === 'Enter') { e.preventDefault(); commitEdit() }
    else if (e.key === 'Escape') cancelEdit()
    return
  }
  const sel = selected.value
  if (!sel) return
  const node = findNode(sel.kind, sel.id)
  if (!node) return

  switch (e.key) {
    case 'Tab': {
      e.preventDefault()
      createChild(sel.kind, sel.id)
      break
    }
    case 'Enter': {
      e.preventDefault()
      createSibling(sel.kind, sel.id)
      break
    }
    case 'Delete':
    case 'Backspace': {
      e.preventDefault()
      removeNode(sel.kind, sel.id)
      break
    }
    case 'F2': {
      e.preventDefault()
      startEdit(sel.kind, sel.id)
      break
    }
    case 'ArrowUp': {
      e.preventDefault()
      const sibs = siblingIds(sel.kind, node.parentKind, node.parentId)
      const idx = sibs.indexOf(sel.id)
      if (idx > 0) selectNode(sel.kind, sibs[idx - 1])
      break
    }
    case 'ArrowDown': {
      e.preventDefault()
      const sibs = siblingIds(sel.kind, node.parentKind, node.parentId)
      const idx = sibs.indexOf(sel.id)
      if (idx >= 0 && idx < sibs.length - 1) selectNode(sel.kind, sibs[idx + 1])
      break
    }
    case 'ArrowRight': {
      e.preventDefault()
      const child = firstChildOf(sel.kind, sel.id)
      if (child) selectNode(child.kind, child.id)
      break
    }
    case 'ArrowLeft': {
      e.preventDefault()
      const p = parentOf(sel.kind, sel.id)
      if (p) selectNode(p.kind, p.id)
      break
    }
  }
}

// ---- 画布：滚轮缩放 + 空白拖拽平移 ----
function onCanvasWheel(e: WheelEvent): void {
  if (!e.ctrlKey && !e.metaKey) return // 无修饰键时交给容器滚动
  e.preventDefault()
  const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1
  zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(zoom.value * factor * 100) / 100))
}
function onCanvasMouseDown(e: MouseEvent): void {
  // 仅在空白处（画布自身，非节点）触发拖拽平移
  if ((e.target as HTMLElement).dataset.canvas !== 'bg') return
  dragging.value = true
  dragStart.value = { x: e.clientX, y: e.clientY, px: panX.value, py: panY.value }
}
function onCanvasMouseMove(e: MouseEvent): void {
  if (!dragging.value) return
  panX.value = dragStart.value.px + (e.clientX - dragStart.value.x)
  panY.value = dragStart.value.py + (e.clientY - dragStart.value.y)
}
function stopDrag(): void {
  dragging.value = false
}
function canvasStyle(): Record<string, string> {
  return {
    transform: `translate(${panX.value}px, ${panY.value}px) scale(${zoom.value})`,
    transformOrigin: '0 0',
  }
}

/** 画布点阵背景（仿 XMind 工作区），使用 CSS 变量适配明暗主题 */
const dotGridStyle = computed(() => ({
  backgroundColor: 'hsl(var(--background))',
  backgroundImage: 'radial-gradient(circle, hsl(var(--border)) 1px, transparent 1px)',
  backgroundSize: '22px 22px',
}))
onBeforeUnmount(() => window.removeEventListener('mouseup', stopDrag))
onMounted(() => window.addEventListener('mouseup', stopDrag))

// ---- 模块 CRUD（左侧栏） ----
const showModuleModal = ref(false)
const moduleFormName = ref('')
const editingModuleId = ref<string | null>(null)

function openModuleModal(id: string | null): void {
  editingModuleId.value = id
  const m = modules.value.find((x) => x.id === id)
  moduleFormName.value = m ? m.name : ''
  showModuleModal.value = true
}
async function saveModule(): Promise<void> {
  if (!moduleFormName.value.trim()) { ElMessage.warning('请输入模块名称'); return }
  try {
    if (editingModuleId.value) {
      const m = modules.value.find((x) => x.id === editingModuleId.value)
      await xmindApi.updateXmindModule(editingModuleId.value, { name: moduleFormName.value.trim(), parentId: m?.parentId ?? null, sortOrder: m?.sortOrder ?? 0 })
    } else {
      await xmindApi.createXmindModule({ name: moduleFormName.value.trim(), parentId: null, sortOrder: modules.value.length })
    }
    showModuleModal.value = false
    await loadAll()
    ElMessage.success('模块已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}
async function deleteModule(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('删除模块将把其中的用例移至「未分类」，确定删除？', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await xmindApi.deleteXmindModule(id)
    modules.value = modules.value.filter((m) => m.id !== id)
    if (selectedModule.value === id) selectedModule.value = null
    await loadAll()
    ElMessage.success('模块已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

// ---- 用例详情抽屉 ----
const drawerDefaultModule = ref('')
function openCreate(moduleId?: string): void {
  editingCase.value = null
  drawerDefaultModule.value = moduleId || selectedModule.value || modules.value[0]?.id || ''
  showDrawer.value = true
}
function openEdit(c: XmindCase): void {
  editingCase.value = c
  drawerDefaultModule.value = c.moduleId || ''
  showDrawer.value = true
}
/** 供抽屉调用的保存逻辑 */
async function submitCase(payload: Record<string, unknown>, isEdit: boolean): Promise<void> {
  if (isEdit && editingCase.value) {
    await xmindApi.updateXmindCase(editingCase.value.id, payload as Partial<XmindCase>)
  } else {
    await xmindApi.createXmindCase(payload as Partial<XmindCase>)
  }
  await loadAll()
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
            <Button variant="ghost" size="icon" class="h-6 w-6 rounded-lg" @click="openModuleModal(null)">
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
                @click="deleteModule(m.id)"
                class="opacity-0 group-hover:opacity-100 shrink-0 px-1 text-muted-foreground hover:text-destructive"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Right - Mindmap canvas -->
    <div class="flex-1 flex flex-col gap-4">
      <Card class="flex-1">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <Network class="w-4 h-4 text-violet-500" />
              <h3 class="text-sm font-semibold">
                思维导图用例
                <span class="text-muted-foreground font-normal ml-1">({{ visibleCount }})</span>
              </h3>
              <span class="hidden lg:inline-flex items-center px-2 py-0.5 rounded-full bg-muted/60 text-[10px] text-muted-foreground">
                双击编辑 · Tab 子节点 · Enter 同级 · Delete 删除 · Ctrl+滚轮缩放
              </span>
            </div>
            <div class="flex items-center gap-2">
              <div class="relative">
                <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input v-model="search" placeholder="搜索用例..." class="h-8 pl-8 text-xs w-44" />
              </div>
              <!-- 缩放工具栏（仿 XMind） -->
              <div class="flex items-center gap-0.5 border rounded-lg px-1 h-8 bg-background">
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="缩小" @click="zoomBy(-0.1)">
                  <ZoomOut class="w-3.5 h-3.5" />
                </button>
                <span class="w-10 text-center text-[10px] text-muted-foreground select-none">{{ Math.round(zoom * 100) }}%</span>
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="放大" @click="zoomBy(0.1)">
                  <ZoomIn class="w-3.5 h-3.5" />
                </button>
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="适应窗口" @click="fitView">
                  <Maximize class="w-3.5 h-3.5" />
                </button>
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="重置 100%" @click="resetView">
                  <RotateCcw class="w-3.5 h-3.5" />
                </button>
              </div>
              <Button size="sm" class="h-8 rounded-lg text-xs gap-1.5" @click="openCreate()">
                <Plus class="w-3 h-3" /> 新建用例
              </Button>
            </div>
          </div>

          <div v-if="error" class="flex items-center gap-2 p-3 mb-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-xs">
            <AlertCircle class="w-4 h-4" /> {{ error }}
          </div>

          <!-- 画布 -->
            <div
              ref="canvasRef"
              tabindex="0"
              data-canvas="bg"
              class="flex-1 overflow-auto rounded-xl border outline-none focus:ring-2 focus:ring-ring/40 select-none"
              :style="dotGridStyle"
              @keydown="onCanvasKeydown"
              @wheel="onCanvasWheel"
              @mousedown="onCanvasMouseDown"
              @mousemove="onCanvasMouseMove"
            >
            <div v-if="activeModules.length === 0" class="h-full flex items-center justify-center text-sm text-muted-foreground">
              <div class="text-center">
                <Network class="w-12 h-12 mx-auto mb-3 opacity-20" />
                <p>{{ selectedModule ? '该模块暂无用例' : '暂无模块，点击左侧「+」或选中中央主题后按 Tab 新建模块' }}</p>
              </div>
            </div>
            <div v-else class="relative" :style="{ width: view.width + 'px', height: view.height + 'px' }">
              <!-- 变换层：缩放 + 平移 -->
              <div class="absolute left-0 top-0" data-canvas="bg" :style="canvasStyle()">
                <!-- 贝塞尔连线 -->
                <svg :width="view.width" :height="view.height" class="absolute left-0 top-0 pointer-events-none" style="overflow: visible">
                  <path v-for="(d, i) in view.links" :key="i" :d="d" fill="none" stroke="hsl(var(--border))" stroke-width="2" />
                </svg>

                <!-- 根节点（中央主题） -->
                <div
                  class="absolute flex items-center justify-center gap-2 px-3 rounded-xl border-2 shadow-sm transition-colors cursor-pointer"
                  :class="selected?.kind === 'root' ? 'border-primary bg-primary/15 text-primary' : 'border-primary/40 bg-primary/10 text-primary hover:border-primary'"
                  :style="{ left: 0 + 'px', top: view.rootY + 'px', width: ROOT_W + 'px', height: ROOT_H + 'px' }"
                  @click="selectNode('root', 'root')"
                  @dblclick="openCreate()"
                >
                  <Network class="w-4 h-4 shrink-0" />
                  <span class="text-sm font-semibold truncate">测试用例</span>
                  <button
                    class="ml-1 p-0.5 rounded hover:bg-primary/20 shrink-0"
                    title="新建模块（Tab）"
                    @click.stop="startEdit('module', 'new', { isNew: true })"
                  >
                    <Plus class="w-3.5 h-3.5" />
                  </button>
                </div>

                <!-- 模块节点 -->
                <div
                  v-for="m in view.nodes.filter((n) => n.kind === 'module')"
                  :key="m.id"
                  class="absolute flex items-center gap-2 px-3 rounded-xl border shadow-sm cursor-pointer transition-all group"
                  :class="selected?.kind === 'module' && selected.id === m.id
                    ? 'border-primary ring-2 ring-primary/30 bg-primary/5'
                    : 'bg-card border-border hover:border-primary/50'"
                  :style="{ left: m.x + 'px', top: m.y + 'px', width: m.w + 'px', height: m.h + 'px' }"
                  @click="selectNode('module', m.id)"
                  @dblclick="startEdit('module', m.id)"
                >
                  <FolderOpen class="w-4 h-4 text-amber-500 shrink-0" />
                  <!-- 就地编辑 -->
                  <template v-if="editing?.kind === 'module' && editing.id === m.id">
                    <Input
                      ref="editInput"
                      v-model="editName"
                      class="h-7 text-xs flex-1"
                      @keydown.enter="commitEdit"
                      @keydown.esc="cancelEdit"
                      @blur="commitEdit"
                      @click.stop
                    />
                  </template>
                  <template v-else>
                    <span class="flex-1 truncate text-sm font-medium">{{ m.name }}</span>
                    <Badge class="text-[10px] px-1.5 py-0 bg-muted text-muted-foreground shrink-0">{{ (grouped[m.id] || []).length }}</Badge>
                    <button
                      class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-accent"
                      title="在此模块下新建用例（Tab）"
                      @click.stop="startEdit('case', 'new', { isNew: true, moduleId: m.id })"
                    >
                      <Plus class="w-3.5 h-3.5" />
                    </button>
                    <button
                      class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-destructive/10 hover:text-destructive"
                      title="删除模块（Delete）"
                      @click.stop="removeNode('module', m.id)"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </template>
                </div>

                <!-- 用例节点 -->
                <div
                  v-for="c in view.nodes.filter((n) => n.kind === 'case')"
                  :key="c.id"
                  class="absolute flex items-center gap-2 px-3 rounded-lg border shadow-sm cursor-pointer transition-all group"
                  :class="selected?.kind === 'case' && selected.id === c.id
                    ? 'border-primary ring-2 ring-primary/30 bg-primary/5'
                    : 'bg-card border-border hover:border-primary/50'"
                  :style="{ left: c.x + 'px', top: c.y + 'px', width: c.w + 'px', height: c.h + 'px' }"
                  @click="selectNode('case', c.id)"
                  @dblclick="startEdit('case', c.id)"
                >
                  <template v-if="editing?.kind === 'case' && editing.id === c.id">
                    <Input
                      ref="editInput"
                      v-model="editName"
                      class="h-7 text-xs flex-1"
                      @keydown.enter="commitEdit"
                      @keydown.esc="cancelEdit"
                      @blur="commitEdit"
                      @click.stop
                    />
                  </template>
                  <template v-else>
                    <Badge :class="`text-[10px] font-bold px-1.5 py-0 shrink-0 ${PRIORITY_COLORS[c.priority || ''] || ''}`">{{ c.priority }}</Badge>
                    <span class="flex-1 truncate text-sm">{{ c.name }}</span>
                    <span v-if="c.stepCount" class="text-[10px] text-muted-foreground shrink-0">{{ c.stepCount }}步</span>
                    <button
                      class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-accent"
                      title="编辑详情"
                      @click.stop="openEdit(cases.find((x) => x.id === c.id)!)"
                    >
                      <PenLine class="w-3.5 h-3.5" />
                    </button>
                    <button
                      class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-destructive/10 hover:text-destructive"
                      title="删除用例（Delete）"
                      @click.stop="removeNode('case', c.id)"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </template>
                </div>

                <!-- 新建中的占位节点（就地输入，置于画布左下角空白区避免遮挡） -->
                <div
                  v-if="editing?.isNew"
                  class="absolute flex items-center gap-2 px-3 rounded-xl border-2 border-dashed border-primary/60 bg-primary/5"
                  :style="{
                    left: 8 + 'px',
                    top: (view.height - 66) + 'px',
                    width: (editing.kind === 'module' ? MODULE_W : CASE_W) + 'px',
                    height: (editing.kind === 'module' ? MODULE_H : CASE_H) + 'px',
                  }"
                >
                  <Input
                    ref="editInput"
                    v-model="editName"
                    :placeholder="editing.kind === 'module' ? '输入模块名称...' : '输入用例名称...'"
                    class="h-7 text-xs flex-1"
                    @keydown.enter="commitEdit"
                    @keydown.esc="cancelEdit"
                    @blur="commitEdit"
                  />
                </div>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between mt-2 text-[10px] text-muted-foreground">
            <span>拖动空白处平移 · Ctrl+滚轮缩放</span>
            <span>共 {{ visibleCount }} 个用例 / {{ activeModules.length }} 个模块</span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Module Modal -->
    <div v-if="showModuleModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" @click="showModuleModal = false">
      <div class="bg-popover border rounded-2xl shadow-2xl p-6 w-[420px] animate-scale-in" @click.stop>
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-base font-semibold">{{ editingModuleId ? '编辑模块' : '新建模块' }}</h3>
          <Button variant="ghost" size="icon" @click="showModuleModal = false"><X class="w-4 h-4" /></Button>
        </div>
        <Input v-model="moduleFormName" placeholder="模块名称" class="mb-4" autofocus @keydown.enter="saveModule" />
        <div class="flex items-center justify-between">
          <Button v-if="editingModuleId" variant="outline" size="sm" class="text-destructive border-destructive/40" @click="deleteModule(editingModuleId)">删除</Button>
          <span v-else></span>
          <div class="flex gap-2">
            <Button variant="outline" size="sm" @click="showModuleModal = false">取消</Button>
            <Button size="sm" @click="saveModule">保存</Button>
          </div>
        </div>
      </div>
    </div>

    <!-- 抽屉式用例编辑器（详情：步骤、前置条件等） -->
    <CaseEditorDrawer
      v-model:visible="showDrawer"
      :modules="modules"
      :editing="editingCase"
      :default-module-id="drawerDefaultModule"
      title="思维导图用例"
      :submit="submitCase"
    />
  </div>
</template>
