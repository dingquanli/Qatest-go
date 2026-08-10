<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Plus, Trash2, AlertCircle, Network, ZoomIn, ZoomOut, Maximize, RotateCcw,
  ChevronRight, ChevronDown, Copy, ClipboardPaste, Undo2, Redo2, FileText,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import * as xmindApi from '@/api/xmind'
import type { XmindCase } from '@/types'

// ---- 布局常量（逻辑图：中心主题 → 分支 → 子主题，向右展开） ----
const COL_W = 230 // 每层列宽
const ROW_H = 44
const V_GAP = 16
const H_PAD = 60
const TOP = 48

interface MindNode {
  id: string
  name: string
  parentId: string
  x: number
  y: number
  depth: number
  isRoot: boolean
}

interface Edge {
  from: { x: number; y: number }
  to: { x: number; y: number }
}

// ---- 数据 ----
const nodes = ref<XmindCase[]>([])
const search = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

const rootNode = computed(() => nodes.value.find((n) => n.parentId === '' || n.parentId == null))

// ---- 交互状态 ----
const selected = ref<string | null>(null)
const editing = ref<string | null>(null)
const editName = ref('')
const editInput = ref<{ focus: () => void; select: () => void } | null>(null)

// 缩放 / 平移
const zoom = ref(1)
const panX = ref(0)
const panY = ref(0)
const canvasRef = ref<HTMLDivElement | null>(null)
const dragging = ref(false)
const dragStart = ref({ x: 0, y: 0, px: 0, py: 0 })
const dragId = ref<string | null>(null)

const MIN_ZOOM = 0.3
const MAX_ZOOM = 2

// ---- 撤销 / 重做（整体快照，经 ReplaceXmindCases 写回） ----
const history = ref<XmindCase[][]>([])
const future = ref<XmindCase[][]>([])
const canUndo = computed(() => history.value.length > 0)
const canRedo = computed(() => future.value.length > 0)

function snapshot(): XmindCase[] {
  return nodes.value.map((n) => ({ ...n }))
}
function pushHistory(): void {
  history.value.push(snapshot())
  if (history.value.length > 50) history.value.shift()
  future.value = []
}
async function restore(list: XmindCase[]): Promise<void> {
  await xmindApi.replaceXmindCases(list)
  await loadAll()
  editing.value = null
  validateSelection()
}
function validateSelection(): void {
  if (selected.value && !nodes.value.some((n) => n.id === selected.value)) selected.value = null
}

// ---- 用例属性面板（完整版：暴露全部测试业务字段） ----
const PRIORITY_OPTIONS = [
  { value: 'P0', label: 'P0 - 紧急' },
  { value: 'P1', label: 'P1 - 高' },
  { value: 'P2', label: 'P2 - 中' },
  { value: 'P3', label: 'P3 - 低' },
]
const TYPE_OPTIONS = [
  { value: 'functional', label: '功能测试' },
  { value: 'performance', label: '性能测试' },
  { value: 'security', label: '安全测试' },
  { value: 'compatibility', label: '兼容性测试' },
  { value: 'smoke', label: '冒烟测试' },
]
const STATUS_OPTIONS = [
  { value: 'draft', label: '草稿' },
  { value: 'ready', label: '待评审' },
  { value: 'active', label: '生效' },
  { value: 'deprecated', label: '废弃' },
]
const selectedNode = computed(() => nodes.value.find((n) => n.id === selected.value) || null)
const propForm = reactive({
  code: '', name: '', priority: 'P2', type: 'functional', precondition: '',
  testData: '', steps: '', expected: '', actualResult: '', assignee: '',
  status: 'draft', defectId: '', env: '', estimate: '', remark: '', tags: '',
})
function syncForm(): void {
  const n = selectedNode.value
  if (!n) return
  propForm.code = n.code || ''
  propForm.name = n.name || ''
  propForm.priority = n.priority || 'P2'
  propForm.type = n.type || 'functional'
  propForm.precondition = n.precondition || ''
  propForm.testData = n.testData || ''
  propForm.steps = n.steps || ''
  propForm.expected = n.expected || ''
  propForm.actualResult = n.actualResult || ''
  propForm.assignee = n.assignee || ''
  propForm.status = n.status || 'draft'
  propForm.defectId = n.defectId || ''
  propForm.env = n.env || ''
  propForm.estimate = n.estimate || ''
  propForm.remark = n.remark || ''
  propForm.tags = n.tags || ''
}
watch([() => selected.value, selectedNode], syncForm)
async function saveProp(field: string, value: string): Promise<void> {
  const id = selected.value
  if (!id) return
  const node = nodes.value.find((n) => n.id === id)
  if (!node) return
  // 发送完整节点对象，避免后端全列 UPDATE 把未传字段清空（name/steps/expected 等）
  const payload = { ...node, [field]: value }
  try {
    await xmindApi.updateXmindCase(id, payload)
    await loadAll()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
    await loadAll()
  }
}

// ---- 剪贴板（复制/粘贴子树） ----
const clipboard = ref<XmindCase[] | null>(null)
function uid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return 'xc' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}
function hasChildren(id: string): boolean {
  return nodes.value.some((n) => (n.parentId || '') === id)
}
function descendantCount(id: string): number {
  return Math.max(0, collectSubtree(id).length - 1)
}
function collectSubtree(id: string): XmindCase[] {
  const result: XmindCase[] = []
  const walk = (nid: string): void => {
    const n = nodes.value.find((x) => x.id === nid)
    if (!n) return
    result.push(n)
    nodes.value.filter((x) => (x.parentId || '') === nid).forEach((c) => walk(c.id))
  }
  walk(id)
  return result
}
function nodeCollapsed(id: string): boolean {
  return !!nodes.value.find((x) => x.id === id)?.collapsed
}

// ---- 折叠 / 展开（视图偏好，持久化但不进撤销栈） ----
async function toggleCollapse(id: string): Promise<void> {
  const n = nodes.value.find((x) => x.id === id)
  if (!n) return
  try {
    // 发送完整节点对象，避免后端全列 UPDATE 覆盖其他字段
    await xmindApi.updateXmindCase(id, { ...n, collapsed: !n.collapsed } as Partial<XmindCase>)
    await loadAll()
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  }
}

// ---- 可见节点（尊重折叠状态；搜索时忽略折叠） ----
function subtreeHidden(): Set<string> {
  const hidden = new Set<string>()
  const childrenMap = new Map<string, XmindCase[]>()
  nodes.value.forEach((n) => {
    const key = n.parentId || ''
    if (!childrenMap.has(key)) childrenMap.set(key, [])
    childrenMap.get(key)!.push(n)
  })
  const walk = (id: string, parentCollapsed: boolean): void => {
    const node = nodes.value.find((n) => n.id === id)
    const collapsed = !!node?.collapsed
    if (parentCollapsed) hidden.add(id)
    ;(childrenMap.get(id) || []).forEach((k) => walk(k.id, parentCollapsed || collapsed))
  }
  const root = nodes.value.find((n) => !n.parentId)
  if (root) walk(root.id, false)
  return hidden
}
function displayList(): XmindCase[] {
  const q = search.value.trim().toLowerCase()
  if (q) return nodes.value.filter((n) => n.name.toLowerCase().includes(q))
  const hidden = subtreeHidden()
  return nodes.value.filter((n) => !hidden.has(n.id))
}

/** 逻辑图布局：递归计算节点坐标 */
const view = computed<{ nodes: MindNode[]; edges: Edge[]; width: number; height: number }>(() => {
  const list = displayList()
  const childrenMap = new Map<string, XmindCase[]>()
  list.forEach((n) => {
    const key = n.parentId || ''
    if (!childrenMap.has(key)) childrenMap.set(key, [])
    childrenMap.get(key)!.push(n)
  })
  const root = list.find((n) => n.parentId === '' || n.parentId == null)
  if (!root) return { nodes: [], edges: [], width: 0, height: 0 }

  const placed: MindNode[] = []
  const edges: Edge[] = []
  let cursorY = TOP

  function layout(n: XmindCase, depth: number): MindNode {
    const kids = childrenMap.get(n.id) || []
    let y: number
    if (kids.length === 0) {
      y = cursorY
      cursorY += ROW_H + V_GAP
    } else {
      const kidNodes = kids.map((k) => layout(k, depth + 1))
      y = (kidNodes[0].y + kidNodes[kidNodes.length - 1].y) / 2
    }
    const node: MindNode = {
      id: n.id,
      name: n.name,
      parentId: n.parentId || '',
      x: H_PAD + depth * COL_W,
      y,
      depth,
      isRoot: depth === 0,
    }
    placed.push(node)
    if (depth > 0) {
      const parent = placed.find((p) => p.id === n.parentId)
      if (parent) {
        edges.push({
          from: { x: parent.x + 180, y: parent.y + ROW_H / 2 },
          to: { x: node.x, y: node.y + ROW_H / 2 },
        })
      }
    }
    return node
  }
  layout(root, 0)

  const width = H_PAD * 2 + (Math.max(0, ...placed.map((p) => p.depth)) + 1) * COL_W
  const height = Math.max(cursorY + 40, TOP + 80)
  return { nodes: placed, edges, width, height }
})

function curve(x1: number, y1: number, x2: number, y2: number): string {
  const dx = Math.max(28, (x2 - x1) * 0.5)
  return `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`
}

// ---- 加载 ----
async function loadAll(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    nodes.value = await xmindApi.getXmindCases()
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

// ---- 缩放 / 平移 ----
function fitView(): void {
  const el = canvasRef.value
  if (!el || view.value.nodes.length === 0) return
  const cw = el.clientWidth
  const ch = el.clientHeight
  if (cw <= 0 || ch <= 0) return
  const v = view.value
  const z = Math.min((cw - 80) / v.width, (ch - 80) / v.height, 1.5)
  zoom.value = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, Math.round(z * 100) / 100))
  panX.value = (cw - v.width * zoom.value) / 2
  panY.value = Math.max(20, (ch - v.height * zoom.value) / 2)
}
function resetView(): void {
  zoom.value = 1
  panX.value = H_PAD
  panY.value = TOP
}
function zoomBy(delta: number): void {
  zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round((zoom.value + delta) * 100) / 100))
}

// ---- 选择 / 编辑 ----
function selectNode(id: string): void {
  selected.value = id
  canvasRef.value?.focus()
}
function isDescendant(id: string, ancestorId: string): boolean {
  let cur = nodes.value.find((n) => n.id === id)
  while (cur && cur.parentId) {
    if (cur.parentId === ancestorId) return true
    cur = nodes.value.find((n) => n.id === cur!.parentId)
  }
  return false
}

async function startEdit(id: string): Promise<void> {
  const n = nodes.value.find((x) => x.id === id)
  if (!n) return
  editing.value = id
  editName.value = n.name
  await nextTick()
  editInput.value?.focus()
  editInput.value?.select()
}
function cancelEdit(): void {
  editing.value = null
  editName.value = ''
}
async function commitEdit(): Promise<void> {
  const id = editing.value
  if (!id) return
  const name = editName.value.trim()
  const node = nodes.value.find((x) => x.id === id)
  if (!node) {
    editing.value = null
    return
  }
  if (!name) {
    cancelEdit()
    return
  }
  if (name === node.name) {
    editing.value = null
    return
  }
  pushHistory()
  try {
    // 发送完整节点对象，避免后端全列 UPDATE 覆盖其他字段
    await xmindApi.updateXmindCase(id, { ...node, name } as Partial<XmindCase>)
    await loadAll()
  } catch (e: any) {
    history.value.pop()
    ElMessage.error(e.message || '保存失败')
  } finally {
    editing.value = null
  }
}

/** 新建节点：先建再进入编辑（XMind 风格） */
async function createNode(parentId: string): Promise<void> {
  pushHistory()
  try {
    const created = await xmindApi.createXmindCase({ name: '新节点', parentId } as Partial<XmindCase>)
    await loadAll()
    await nextTick()
    selected.value = created.id
    await startEdit(created.id)
  } catch (e: any) {
    history.value.pop()
    ElMessage.error(e.message || '创建失败')
  }
}

/** 工具栏「新建主题」：无中心主题则建中心主题，否则在选中节点下建子节点 */
function toolbarNew(): void {
  if (!rootNode.value) {
    createNode('')
  } else if (selected.value) {
    createNode(selected.value)
  } else {
    createNode(rootNode.value.id)
  }
}

async function removeNode(id: string): Promise<void> {
  const n = nodes.value.find((x) => x.id === id)
  const isRoot = !n || !n.parentId
  try {
    await ElMessageBox.confirm(
      isRoot ? '删除中心主题将同时删除整张思维导图，确定？' : '删除该节点及其全部子节点，确定？',
      '提示',
      { type: 'warning' },
    )
    pushHistory()
    await xmindApi.deleteXmindCase(id)
    if (selected.value === id) selected.value = null
    await loadAll()
    ElMessage.success('已删除')
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return
    history.value.pop()
    ElMessage.error(e.message || '删除失败')
  }
}

// ---- 复制 / 粘贴 ----
function copySelection(): void {
  if (!selected.value) {
    ElMessage.warning('请先选中一个节点')
    return
  }
  clipboard.value = collectSubtree(selected.value).map((n) => ({ ...n }))
  ElMessage.success('已复制节点子树')
}
async function pasteClipboard(): Promise<void> {
  if (!clipboard.value || clipboard.value.length === 0) {
    ElMessage.warning('剪贴板为空')
    return
  }
  const targetParent = selected.value
    ? selected.value
    : rootNode.value
      ? rootNode.value.id
      : ''
  pushHistory()
  const topOldId = clipboard.value[0].id
  const idMap = new Map<string, string>()
  const newNodes: XmindCase[] = clipboard.value.map((n) => {
    const nid = uid()
    idMap.set(n.id, nid)
    return { ...n, id: nid, collapsed: false }
  })
  newNodes.forEach((n) => {
    if (n.id === idMap.get(topOldId)) {
      n.parentId = targetParent
    } else {
      n.parentId = idMap.get(n.parentId) || n.parentId
    }
  })
  const merged = [...nodes.value.map((n) => ({ ...n })), ...newNodes]
  try {
    await xmindApi.replaceXmindCases(merged)
    await loadAll()
    selected.value = idMap.get(topOldId) || selected.value
    ElMessage.success('已粘贴')
  } catch (e: any) {
    history.value.pop()
    ElMessage.error(e.message || '粘贴失败')
  }
}

// ---- 撤销 / 重做 ----
async function undo(): Promise<void> {
  if (history.value.length === 0) return
  future.value.push(snapshot())
  const prev = history.value.pop()!
  await restore(prev)
}
async function redo(): Promise<void> {
  if (future.value.length === 0) return
  history.value.push(snapshot())
  const next = future.value.pop()!
  await restore(next)
}

// ---- 键盘快捷键（仿 XMind） ----
function nodeById(id: string): MindNode | undefined {
  return view.value.nodes.find((n) => n.id === id)
}
function siblingsOf(id: string): string[] {
  const n = nodes.value.find((x) => x.id === id)
  const pid = n?.parentId || ''
  return nodes.value.filter((x) => (x.parentId || '') === pid).map((x) => x.id)
}
function firstChildOf(id: string): string | undefined {
  return nodes.value.find((x) => (x.parentId || '') === id)?.id
}

function onCanvasKeydown(e: KeyboardEvent): void {
  if (editing.value) {
    if (e.key === 'Enter') {
      e.preventDefault()
      commitEdit()
    } else if (e.key === 'Escape') cancelEdit()
    return
  }
  // Ctrl/Cmd 组合键：复制 / 粘贴 / 撤销 / 重做
  if (e.ctrlKey || e.metaKey) {
    const k = e.key.toLowerCase()
    if (k === 'c') {
      e.preventDefault()
      copySelection()
      return
    }
    if (k === 'v') {
      e.preventDefault()
      pasteClipboard()
      return
    }
    if (k === 'z') {
      e.preventDefault()
      if (e.shiftKey) redo()
      else undo()
      return
    }
    if (k === 'y') {
      e.preventDefault()
      redo()
      return
    }
  }
  const id = selected.value
  if (!id) return
  switch (e.key) {
    case 'Tab': {
      e.preventDefault()
      createNode(id)
      break
    }
    case 'Enter': {
      e.preventDefault()
      const n = nodes.value.find((x) => x.id === id)
      createNode(n?.parentId || '')
      break
    }
    case 'Delete':
    case 'Backspace': {
      e.preventDefault()
      removeNode(id)
      break
    }
    case 'F2': {
      e.preventDefault()
      startEdit(id)
      break
    }
    case 'ArrowUp': {
      e.preventDefault()
      const sibs = siblingsOf(id)
      const idx = sibs.indexOf(id)
      if (idx > 0) selectNode(sibs[idx - 1])
      break
    }
    case 'ArrowDown': {
      e.preventDefault()
      const sibs = siblingsOf(id)
      const idx = sibs.indexOf(id)
      if (idx >= 0 && idx < sibs.length - 1) selectNode(sibs[idx + 1])
      break
    }
    case 'ArrowRight': {
      e.preventDefault()
      const c = firstChildOf(id)
      if (c) selectNode(c)
      break
    }
    case 'ArrowLeft': {
      e.preventDefault()
      const n = nodes.value.find((x) => x.id === id)
      if (n?.parentId) selectNode(n.parentId)
      break
    }
  }
}

// ---- 画布：滚轮缩放 + 空白拖拽平移 ----
function onCanvasWheel(e: WheelEvent): void {
  if (!e.ctrlKey && !e.metaKey) return
  e.preventDefault()
  const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1
  zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(zoom.value * factor * 100) / 100))
}
function onCanvasMouseDown(e: MouseEvent): void {
  if ((e.target as HTMLElement).dataset.canvas !== 'bg') return
  selected.value = null
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

// ---- 拖拽改父级 ----
function onDragStart(id: string): void {
  const n = nodes.value.find((x) => x.id === id)
  if (!n || !n.parentId) return // 中心主题不可拖拽
  dragId.value = id
}
function onDrop(targetId: string): void {
  const from = dragId.value
  dragId.value = null
  if (!from || from === targetId) return
  const target = nodes.value.find((x) => x.id === targetId)
  if (!target) return
  if (isDescendant(targetId, from)) {
    ElMessage.warning('不能移动到自己的子节点下')
    return
  }
  if (nodes.value.find((x) => x.id === from)?.parentId === targetId) return
  reparent(from, targetId)
}
function onDropCanvas(): void {
  const from = dragId.value
  dragId.value = null
  if (!from) return
  const newParent = rootNode.value ? rootNode.value.id : ''
  if (nodes.value.find((x) => x.id === from)?.parentId === newParent) return
  reparent(from, newParent)
}
async function reparent(id: string, parentId: string): Promise<void> {
  const node = nodes.value.find((n) => n.id === id)
  if (!node) return
  pushHistory()
  try {
    // 发送完整节点对象，避免后端全列 UPDATE 覆盖其他字段
    await xmindApi.updateXmindCase(id, { ...node, parentId } as Partial<XmindCase>)
    await loadAll()
  } catch (e: any) {
    history.value.pop()
    ElMessage.error(e.message || '移动失败')
  }
}

const dotGridStyle = computed(() => ({
  backgroundColor: 'hsl(var(--background))',
  backgroundImage: 'radial-gradient(circle, hsl(var(--border)) 1px, transparent 1px)',
  backgroundSize: '22px 22px',
}))
onBeforeUnmount(() => window.removeEventListener('mouseup', stopDrag))
onMounted(() => window.addEventListener('mouseup', stopDrag))
</script>

<template>
  <div class="flex h-full gap-6">
    <!-- 右侧画布区 -->
    <div class="flex-1 flex flex-col gap-4">
      <Card class="flex-1">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <Network class="w-4 h-4 text-violet-500" />
              <h3 class="text-sm font-semibold">思维导图</h3>
              <span class="hidden lg:inline-flex items-center px-2 py-0.5 rounded-full bg-muted/60 text-[10px] text-muted-foreground">
                双击编辑 · Tab 子节点 · Enter 同级 · Delete 删除 · 拖拽改父级 · Ctrl+滚轮缩放
              </span>
            </div>
            <div class="flex items-center gap-2">
              <div class="relative">
                <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input v-model="search" placeholder="搜索节点..." class="h-8 pl-8 text-xs w-40" />
              </div>
              <div class="flex items-center gap-0.5 border rounded-lg px-1 h-8 bg-background">
                <button
                  class="p-1 rounded hover:bg-accent text-muted-foreground disabled:opacity-30 disabled:cursor-not-allowed"
                  title="撤销 (Ctrl+Z)" :disabled="!canUndo" @click="undo"
                >
                  <Undo2 class="w-3.5 h-3.5" />
                </button>
                <button
                  class="p-1 rounded hover:bg-accent text-muted-foreground disabled:opacity-30 disabled:cursor-not-allowed"
                  title="重做 (Ctrl+Y)" :disabled="!canRedo" @click="redo"
                >
                  <Redo2 class="w-3.5 h-3.5" />
                </button>
                <span class="w-px h-4 bg-border mx-0.5" />
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="复制 (Ctrl+C)" @click="copySelection">
                  <Copy class="w-3.5 h-3.5" />
                </button>
                <button class="p-1 rounded hover:bg-accent text-muted-foreground" title="粘贴 (Ctrl+V)" @click="pasteClipboard">
                  <ClipboardPaste class="w-3.5 h-3.5" />
                </button>
              </div>
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
              <Button size="sm" class="h-8 rounded-lg text-xs gap-1.5" @click="toolbarNew">
                <Plus class="w-3 h-3" /> 新建主题
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
            @dragover.prevent
            @drop="onDropCanvas"
          >
            <!-- 空状态 -->
            <div v-if="view.nodes.length === 0" class="h-full flex items-center justify-center text-sm text-muted-foreground">
              <div class="text-center">
                <Network class="w-12 h-12 mx-auto mb-3 opacity-20" />
                <p>还没有思维导图</p>
                <Button size="sm" class="mt-3" @click="createNode('')">
                  <Plus class="w-3.5 h-3.5" /> 创建中心主题
                </Button>
              </div>
            </div>

            <div v-else class="relative" :style="{ width: view.width + 'px', height: view.height + 'px' }">
              <div class="absolute left-0 top-0" data-canvas="bg" :style="canvasStyle()">
                <!-- 连线 -->
                <svg :width="view.width" :height="view.height" class="absolute left-0 top-0 pointer-events-none" style="overflow: visible">
                  <path
                    v-for="(e, i) in view.edges"
                    :key="i"
                    :d="curve(e.from.x, e.from.y, e.to.x, e.to.y)"
                    fill="none"
                    stroke="hsl(var(--border))"
                    stroke-width="2"
                  />
                </svg>

                <!-- 节点 -->
                <div
                  v-for="n in view.nodes"
                  :key="n.id"
                  class="absolute flex items-center px-3 rounded-xl border shadow-sm cursor-pointer transition-all group"
                  :class="[
                    n.isRoot
                      ? 'border-2 border-primary bg-primary/10 text-primary font-semibold'
                      : (selected === n.id ? 'border-primary ring-2 ring-primary/30 bg-primary/5' : 'bg-card border-border hover:border-primary/50'),
                    editing === n.id ? 'ring-2 ring-primary/40' : ''
                  ]"
                  :style="{ left: n.x + 'px', top: n.y + 'px', minWidth: '120px', maxWidth: '200px', height: ROW_H + 'px' }"
                  :draggable="!n.isRoot && !editing"
                  @click.stop="selectNode(n.id)"
                  @dblclick.stop="startEdit(n.id)"
                  @dragstart="onDragStart(n.id)"
                  @dragover.prevent
                  @drop.stop="onDrop(n.id)"
                >
                  <Input
                    v-if="editing === n.id"
                    ref="editInput"
                    v-model="editName"
                    class="h-7 text-xs flex-1 border-transparent bg-transparent focus:bg-background px-1"
                    @keydown.enter="commitEdit"
                    @keydown.esc="cancelEdit"
                    @blur="commitEdit"
                    @click.stop
                  />
                  <template v-else>
                    <span class="flex-1 truncate text-sm">{{ n.name || '未命名' }}</span>
                    <!-- 折叠/展开 -->
                    <button
                      v-if="hasChildren(n.id)"
                      class="shrink-0 ml-1 p-0.5 rounded hover:bg-accent text-muted-foreground"
                      :title="nodeCollapsed(n.id) ? '展开子树' : '折叠子树'"
                      @click.stop="toggleCollapse(n.id)"
                    >
                      <component :is="nodeCollapsed(n.id) ? ChevronRight : ChevronDown" class="w-3.5 h-3.5" />
                    </button>
                    <span
                      v-if="nodeCollapsed(n.id) && descendantCount(n.id) > 0"
                      class="shrink-0 ml-0.5 px-1 rounded-full bg-muted text-[10px] text-muted-foreground"
                    >+{{ descendantCount(n.id) }}</span>
                    <button
                      class="opacity-0 group-hover:opacity-100 shrink-0 ml-1 p-0.5 rounded hover:bg-destructive/10 hover:text-destructive"
                      title="删除"
                      @click.stop="removeNode(n.id)"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between mt-2 text-[10px] text-muted-foreground">
            <span>拖动空白处平移 · Ctrl+滚轮缩放 · 拖拽节点到其他节点上可改变父级 · 折叠后显示隐藏数量</span>
            <span>共 {{ nodes.length }} 个节点</span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 右侧：用例属性面板（完整版业务字段） -->
    <div class="w-80 shrink-0">
      <Card class="h-full">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center gap-2 mb-3">
            <FileText class="w-4 h-4 text-violet-500" />
            <h3 class="text-sm font-semibold">用例属性</h3>
            <span v-if="selectedNode" class="ml-auto text-[10px] text-muted-foreground truncate max-w-[110px]">
              {{ selectedNode.name || '未命名' }}
            </span>
          </div>

          <div v-if="!selectedNode" class="flex-1 flex items-center justify-center text-xs text-muted-foreground text-center px-4">
            在左侧画布选中一个节点，即可编辑其全部测试业务字段
          </div>

          <div v-else class="flex-1 overflow-auto pr-1 space-y-3">
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">用例编号</label>
              <Input v-model="propForm.code" class="h-8 mt-1 text-xs" placeholder="TC-MOD-001" @change="saveProp('code', propForm.code)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">主题 / 标题</label>
              <Input v-model="propForm.name" class="h-8 mt-1 text-xs" placeholder="用例标题" @change="saveProp('name', propForm.name)" />
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">优先级</label>
                <select v-model="propForm.priority" class="w-full h-8 mt-1 rounded-lg border border-input bg-transparent px-2 text-xs outline-none focus:ring-2 focus:ring-ring" @change="saveProp('priority', propForm.priority)">
                  <option v-for="o in PRIORITY_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
              </div>
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">类型</label>
                <select v-model="propForm.type" class="w-full h-8 mt-1 rounded-lg border border-input bg-transparent px-2 text-xs outline-none focus:ring-2 focus:ring-ring" @change="saveProp('type', propForm.type)">
                  <option v-for="o in TYPE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">状态</label>
                <select v-model="propForm.status" class="w-full h-8 mt-1 rounded-lg border border-input bg-transparent px-2 text-xs outline-none focus:ring-2 focus:ring-ring" @change="saveProp('status', propForm.status)">
                  <option v-for="o in STATUS_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
              </div>
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">负责人</label>
                <Input v-model="propForm.assignee" class="h-8 mt-1 text-xs" placeholder="执行人" @change="saveProp('assignee', propForm.assignee)" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">测试环境</label>
                <Input v-model="propForm.env" class="h-8 mt-1 text-xs" placeholder="如 QA / 预发" @change="saveProp('env', propForm.env)" />
              </div>
              <div>
                <label class="text-[11px] font-medium text-muted-foreground">预计工时</label>
                <Input v-model="propForm.estimate" class="h-8 mt-1 text-xs" placeholder="如 1h / 30m" @change="saveProp('estimate', propForm.estimate)" />
              </div>
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">缺陷编号</label>
              <Input v-model="propForm.defectId" class="h-8 mt-1 text-xs" placeholder="关联缺陷单号" @change="saveProp('defectId', propForm.defectId)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">标签</label>
              <Input v-model="propForm.tags" class="h-8 mt-1 text-xs" placeholder="逗号分隔，如 回归,核心" @change="saveProp('tags', propForm.tags)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">前置条件</label>
              <textarea v-model="propForm.precondition" rows="2" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="执行前需满足的状态" @change="saveProp('precondition', propForm.precondition)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">输入数据</label>
              <textarea v-model="propForm.testData" rows="2" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="步骤所需的输入数据" @change="saveProp('testData', propForm.testData)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">测试步骤</label>
              <textarea v-model="propForm.steps" rows="4" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="每步一条，可编号" @change="saveProp('steps', propForm.steps)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">预期结果</label>
              <textarea v-model="propForm.expected" rows="3" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="可观测的预期结果" @change="saveProp('expected', propForm.expected)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">实际结果</label>
              <textarea v-model="propForm.actualResult" rows="3" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="执行后实际表现（执行阶段填写）" @change="saveProp('actualResult', propForm.actualResult)" />
            </div>
            <div>
              <label class="text-[11px] font-medium text-muted-foreground">备注</label>
              <textarea v-model="propForm.remark" rows="2" class="w-full mt-1 rounded-lg border border-input bg-transparent px-2 py-1 text-xs outline-none focus:ring-2 focus:ring-ring resize-none" placeholder="补充说明" @change="saveProp('remark', propForm.remark)" />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
