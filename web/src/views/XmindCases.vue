<script setup lang="ts">
import { ref, computed, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Plus, Trash2, AlertCircle, Network, ZoomIn, ZoomOut, Maximize, RotateCcw,
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

function visibleNodes(): XmindCase[] {
  const q = search.value.trim().toLowerCase()
  if (!q) return nodes.value
  return nodes.value.filter((n) => n.name.toLowerCase().includes(q))
}

/** 逻辑图布局：递归计算节点坐标 */
const view = computed<{ nodes: MindNode[]; edges: Edge[]; width: number; height: number }>(() => {
  const list = visibleNodes()
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
  if (!name) {
    cancelEdit()
    return
  }
  try {
    await xmindApi.updateXmindCase(id, { name } as Partial<XmindCase>)
    await loadAll()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    editing.value = null
  }
}

/** 新建节点：先建再进入编辑（XMind 风格） */
async function createNode(parentId: string): Promise<void> {
  try {
    const created = await xmindApi.createXmindCase({ name: '新节点', parentId } as Partial<XmindCase>)
    await loadAll()
    await nextTick()
    selected.value = created.id
    await startEdit(created.id)
  } catch (e: any) {
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
    await xmindApi.deleteXmindCase(id)
    if (selected.value === id) selected.value = null
    await loadAll()
    ElMessage.success('已删除')
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return
    ElMessage.error(e.message || '删除失败')
  }
}

// ---- 键盘快捷键（仿 XMind：Tab 子节点 / Enter 同级 / Delete 删除 / 方向键导航 / F2 编辑） ----
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
    if (e.key === 'Enter') { e.preventDefault(); commitEdit() }
    else if (e.key === 'Escape') cancelEdit()
    return
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
  try {
    await xmindApi.updateXmindCase(id, { parentId } as Partial<XmindCase>)
    await loadAll()
  } catch (e: any) {
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
            <span>拖动空白处平移 · Ctrl+滚轮缩放 · 拖拽节点到其他节点上可改变父级</span>
            <span>共 {{ nodes.length }} 个节点</span>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
