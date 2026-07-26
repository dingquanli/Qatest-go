<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Plus, Trash2, AlertCircle, FolderPlus, X, Network, FolderOpen,
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

// ---- Layout constants ----
const ROOT_X = 24
const ROOT_W = 134
const ROOT_H = 46
const MODULE_X = 212
const MODULE_W = 180
const MODULE_H = 40
const CASE_X = 456
const CASE_W = 214
const CASE_H = 38
const CASE_GAP = 54
const TOP = 28
const PAD_BOTTOM = 28

interface MNode { id: string; name: string; count: number; x: number; y: number }
interface CNode { id: string; name: string; priority: string; x: number; y: number; data: XmindCase }
interface MindView { moduleNodes: MNode[]; caseNodes: CNode[]; links: string[]; totalHeight: number; width: number; rootY: number }

function elbow(x1: number, y1: number, x2: number, y2: number): string {
  const mx = x1 + (x2 - x1) / 2
  return `M ${x1} ${y1} H ${mx} V ${y2} H ${x2}`
}

const cases = ref<XmindCase[]>([])
const modules = ref<XmindModule[]>([])
const search = ref('')
const selectedModule = ref<string | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

// Module modal
const showModuleModal = ref(false)
const moduleFormName = ref('')
const editingModuleId = ref<string | null>(null)

// Case drawer（抽屉式结构化编辑器，与表格用例统一体验）
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

// 当前选中模块（selectedModule 为空表示「全部」）
const activeModules = computed(() =>
  selectedModule.value ? modules.value.filter((m) => m.id === selectedModule.value) : modules.value,
)

const visibleCount = computed(() =>
  activeModules.value.reduce((s, m) => s + (grouped.value[m.id]?.length || 0), 0),
)

const view = computed<MindView>(() => {
  const branches = activeModules.value.map((m) => {
    const cs = grouped.value[m.id] || []
    const height = Math.max(1, cs.length) * CASE_GAP
    return { module: m, cases: cs, height }
  })
  const totalHeight = TOP + PAD_BOTTOM + branches.reduce((s, b) => s + b.height, 0)
  let y = TOP
  const moduleNodes: MNode[] = []
  const caseNodes: CNode[] = []
  const links: string[] = []
  branches.forEach((b) => {
    const branchTop = y
    const moduleY = branchTop + b.height / 2 - MODULE_H / 2
    moduleNodes.push({ id: b.module.id, name: b.module.name, count: b.cases.length, x: MODULE_X, y: moduleY })
    links.push(elbow(ROOT_X + ROOT_W, totalHeight / 2, MODULE_X, moduleY + MODULE_H / 2))
    b.cases.forEach((c, j) => {
      const caseY = branchTop + j * CASE_GAP + (CASE_GAP - CASE_H) / 2
      caseNodes.push({ id: c.id, name: c.name, priority: c.priority, x: CASE_X, y: caseY, data: c })
      links.push(elbow(MODULE_X + MODULE_W, moduleY + MODULE_H / 2, CASE_X, caseY + CASE_H / 2))
    })
    y += b.height
  })
  const rootY = totalHeight / 2 - ROOT_H / 2
  const width = CASE_X + CASE_W + 32
  return { moduleNodes, caseNodes, links, totalHeight, width, rootY }
})

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
onMounted(loadAll)

// ---- Module CRUD ----
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
      await xmindApi.updateXmindModule(editingModuleId.value, { name: moduleFormName.value.trim() })
    } else {
      const m = await xmindApi.createXmindModule({ name: moduleFormName.value.trim() })
      modules.value = [...modules.value, m]
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

// ---- Case CRUD（抽屉式）----
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
async function deleteCase(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除此用例？', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await xmindApi.deleteXmindCase(id)
    cases.value = cases.value.filter((c) => c.id !== id)
    await loadAll()
    ElMessage.success('用例已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
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
            </div>
            <div class="flex items-center gap-2">
              <div class="relative">
                <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <Input v-model="search" placeholder="搜索用例..." class="h-8 pl-8 text-xs w-48" />
              </div>
              <Button size="sm" class="h-8 rounded-lg text-xs gap-1.5" @click="openCreate()">
                <Plus class="w-3 h-3" /> 新建用例
              </Button>
            </div>
          </div>

          <div v-if="error" class="flex items-center gap-2 p-3 mb-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-xs">
            <AlertCircle class="w-4 h-4" /> {{ error }}
          </div>

          <div v-if="activeModules.length === 0" class="flex-1 flex items-center justify-center text-sm text-muted-foreground">
            <div class="text-center">
              <Network class="w-12 h-12 mx-auto mb-3 opacity-20" />
              <p>{{ selectedModule ? '该模块暂无用例' : '暂无模块，点击左侧「+」新建模块开始绘制思维导图' }}</p>
            </div>
          </div>
          <div v-else class="flex-1 overflow-auto">
            <div class="relative" :style="{ width: view.width + 'px', height: view.totalHeight + 'px' }">
              <!-- Connectors -->
              <svg :width="view.width" :height="view.totalHeight" class="absolute left-0 top-0 pointer-events-none" style="overflow:visible">
                <path v-for="(d, i) in view.links" :key="i" :d="d" fill="none" stroke="hsl(var(--border))" stroke-width="1.5" />
              </svg>

              <!-- Root node -->
              <div
                class="absolute flex items-center justify-center gap-2 px-3 rounded-xl border-2 border-primary/40 bg-primary/10 text-primary font-semibold shadow-sm"
                :style="{ left: ROOT_X + 'px', top: view.rootY + 'px', width: ROOT_W + 'px', height: ROOT_H + 'px' }"
              >
                <Network class="w-4 h-4" />
                <span class="text-sm">测试用例</span>
                <button class="ml-1 p-0.5 rounded hover:bg-primary/20" title="新建模块" @click="openModuleModal(null)">
                  <Plus class="w-3.5 h-3.5" />
                </button>
              </div>

              <!-- Module nodes -->
              <div
                v-for="m in view.moduleNodes"
                :key="m.id"
                class="absolute flex items-center gap-2 px-3 rounded-xl border bg-card shadow-sm cursor-pointer hover:border-primary/50 transition-colors group"
                :style="{ left: m.x + 'px', top: m.y + 'px', width: MODULE_W + 'px', height: MODULE_H + 'px' }"
                @click="openModuleModal(m.id)"
              >
                <FolderOpen class="w-4 h-4 text-amber-500 shrink-0" />
                <span class="flex-1 truncate text-sm font-medium">{{ m.name }}</span>
                <Badge class="text-[10px] px-1.5 py-0 bg-muted text-muted-foreground">{{ m.count }}</Badge>
                <button class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-accent" title="在此模块下新建用例" @click.stop="openCreate(m.id)">
                  <Plus class="w-3.5 h-3.5" />
                </button>
              </div>

              <!-- Case nodes -->
              <div
                v-for="c in view.caseNodes"
                :key="c.id"
                class="absolute flex items-center gap-2 px-3 rounded-lg border bg-card shadow-sm cursor-pointer hover:border-primary/50 transition-colors group"
                :style="{ left: c.x + 'px', top: c.y + 'px', width: CASE_W + 'px', height: CASE_H + 'px' }"
                @click="openEdit(c.data)"
              >
                <Badge :class="`text-[10px] font-bold px-1.5 py-0 shrink-0 ${PRIORITY_COLORS[c.priority] || ''}`">{{ c.priority }}</Badge>
                <span class="flex-1 truncate text-sm">{{ c.name }}</span>
                <button class="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-destructive/10 hover:text-destructive" title="删除用例" @click.stop="deleteCase(c.id)">
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
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

    <!-- 抽屉式用例编辑器（与表格用例统一体验） -->
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
