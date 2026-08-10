<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Trash2, FileSpreadsheet, Keyboard,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import * as sheetApi from '@/api/spreadsheet'
import type { Spreadsheet } from '@/types'

const MIN_ROWS = 24
const MIN_COLS = 8
const DEFAULT_COLS = 8

// ---- 数据 ----
const sheets = ref<Spreadsheet[]>([])
const currentId = ref<string>('')
const name = ref<string>('工作表')
const grid = ref<string[][]>([])
const loading = ref(false)
const saving = ref(false)

// 选中单元格
const activeRow = ref(-1)
const activeCol = ref(-1)
const cellEls = ref<Record<string, HTMLInputElement | null>>({})
function cellKey(r: number, c: number): string {
  return `${r}:${c}`
}

// 列标 A/B/C...
function colLabel(i: number): string {
  let s = ''
  let n = i
  do {
    s = String.fromCharCode(65 + (n % 26)) + s
    n = Math.floor(n / 26) - 1
  } while (n >= 0)
  return s
}

const rowCount = computed(() => grid.value.length)
const colCount = computed(() => (grid.value[0]?.length) || 0)

// ---- 加载 ----
function emptyGrid(rows: number, cols: number): string[][] {
  return Array.from({ length: rows }, () => Array.from({ length: cols }, () => ''))
}

function normalize(g: string[][]): string[][] {
  let maxCols = MIN_COLS
  g.forEach((r) => {
    if (r.length > maxCols) maxCols = r.length
  })
  const rows = Math.max(g.length, MIN_ROWS)
  const out: string[][] = []
  for (let r = 0; r < rows; r++) {
    const src = g[r] || []
    const row: string[] = []
    for (let c = 0; c < maxCols; c++) row.push(src[c] ?? '')
    out.push(row)
  }
  return out
}

async function loadSheets(): Promise<void> {
  loading.value = true
  try {
    const list = await sheetApi.getSpreadsheets()
    sheets.value = list
    if (list.length === 0) {
      const created = await sheetApi.createSpreadsheet({ name: '工作表', cells: emptyGrid(MIN_ROWS, DEFAULT_COLS) })
      sheets.value = [created]
      currentId.value = created.id
      grid.value = emptyGrid(MIN_ROWS, DEFAULT_COLS)
      name.value = created.name
    } else {
      currentId.value = list[0].id
      await loadSheet(list[0].id)
    }
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function loadSheet(id: string): Promise<void> {
  try {
    const s = await sheetApi.getSpreadsheet(id)
    currentId.value = id
    name.value = s.name
    let parsed: string[][] = []
    try {
      parsed = typeof s.cells === 'string' ? JSON.parse(s.cells) : (s.cells as unknown as string[][])
    } catch {
      parsed = []
    }
    if (!Array.isArray(parsed) || parsed.length === 0) parsed = emptyGrid(MIN_ROWS, DEFAULT_COLS)
    grid.value = normalize(parsed)
    activeRow.value = -1
    activeCol.value = -1
  } catch (e: any) {
    ElMessage.error(e.message || '加载表格失败')
  }
}

async function switchSheet(id: string): Promise<void> {
  if (id === currentId.value) return
  await flushSave()
  await loadSheet(id)
}

// ---- 保存（防抖，整表写入） ----
let saveTimer: ReturnType<typeof setTimeout> | null = null
function scheduleSave(): void {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => flushSave(), 500)
}
async function flushSave(): Promise<void> {
  if (!currentId.value || saving.value) return
  saving.value = true
  try {
    await sheetApi.updateSpreadsheet(currentId.value, {
      name: name.value,
      cells: JSON.stringify(grid.value),
    })
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// ---- 单元格编辑 ----
function onCellInput(r: number, c: number, val: string): void {
  if (!grid.value[r]) grid.value[r] = []
  grid.value[r][c] = val
  scheduleSave()
}
function selectCell(r: number, c: number): void {
  activeRow.value = r
  activeCol.value = c
  const el = cellEls.value[cellKey(r, c)]
  if (el) {
    el.focus()
    el.select()
  }
}

function onCellKeydown(e: KeyboardEvent, r: number, c: number): void {
  const rows = rowCount.value
  const cols = colCount.value
  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      if (r < rows - 1) selectCell(r + 1, c)
      break
    case 'ArrowUp':
      e.preventDefault()
      if (r > 0) selectCell(r - 1, c)
      break
    case 'ArrowLeft':
      if ((e.target as HTMLInputElement).selectionStart === 0) {
        e.preventDefault()
        if (c > 0) selectCell(r, c - 1)
      }
      break
    case 'ArrowRight':
      if ((e.target as HTMLInputElement).selectionStart === (e.target as HTMLInputElement).value.length) {
        e.preventDefault()
        if (c < cols - 1) selectCell(r, c + 1)
      }
      break
    case 'Tab': {
      e.preventDefault()
      if (e.shiftKey) {
        if (c > 0) selectCell(r, c - 1)
        else if (r > 0) selectCell(r - 1, cols - 1)
      } else {
        if (c < cols - 1) selectCell(r, c + 1)
        else if (r < rows - 1) selectCell(r + 1, 0)
      }
      break
    }
    case 'Enter':
      e.preventDefault()
      if (r < rows - 1) selectCell(r + 1, c)
      break
  }
}

// 粘贴（从 Excel/WPS 复制的多行多列 TSV）
function onCellPaste(e: ClipboardEvent, r: number, c: number): void {
  const text = e.clipboardData?.getData('text') || ''
  if (!text.includes('\n') && !text.includes('\t')) return
  e.preventDefault()
  const rowsData = text.replace(/\r/g, '').split('\n').filter((x) => x !== '' || true)
  let rr = r
  for (const line of rowsData) {
    if (rr >= rowCount.value) addRow(false)
    const cellsData = line.split('\t')
    let cc = c
    for (const val of cellsData) {
      if (cc >= colCount.value) addCol(false)
      grid.value[rr][cc] = val
      cc++
    }
    rr++
  }
  scheduleSave()
  nextTick(() => selectCell(Math.min(r + rowsData.length - 1, rowCount.value - 1), c))
}

// ---- 增删行列 ----
function addRow(focus = true): void {
  const cols = colCount.value || MIN_COLS
  grid.value.push(Array.from({ length: cols }, () => ''))
  scheduleSave()
  if (focus) nextTick(() => selectCell(rowCount.value - 1, Math.max(0, activeCol.value)))
}
function addCol(focus = true): void {
  grid.value.forEach((row) => row.push(''))
  scheduleSave()
  if (focus) nextTick(() => selectCell(Math.max(0, activeRow.value), colCount.value - 1))
}
async function deleteRow(): Promise<void> {
  if (activeRow.value < 0) {
    ElMessage.warning('请先选中要删除的行')
    return
  }
  if (rowCount.value <= 1) {
    ElMessage.warning('至少保留一行')
    return
  }
  grid.value.splice(activeRow.value, 1)
  activeRow.value = Math.min(activeRow.value, rowCount.value - 1)
  scheduleSave()
}
async function deleteCol(): Promise<void> {
  if (activeCol.value < 0) {
    ElMessage.warning('请先选中要删除的列')
    return
  }
  if (colCount.value <= 1) {
    ElMessage.warning('至少保留一列')
    return
  }
  grid.value.forEach((row) => row.splice(activeCol.value, 1))
  activeCol.value = Math.min(activeCol.value, colCount.value - 1)
  scheduleSave()
}

// ---- 表管理 ----
async function newSheet(): Promise<void> {
  await flushSave()
  try {
    const created = await sheetApi.createSpreadsheet({ name: `工作表${sheets.value.length + 1}`, cells: emptyGrid(MIN_ROWS, DEFAULT_COLS) })
    sheets.value = [...sheets.value, created]
    await loadSheet(created.id)
    ElMessage.success('已新建工作表')
  } catch (e: any) {
    ElMessage.error(e.message || '新建失败')
  }
}
async function deleteSheet(): Promise<void> {
  if (sheets.value.length <= 1) {
    ElMessage.warning('至少保留一张工作表')
    return
  }
  try {
    await ElMessageBox.confirm('确定删除当前工作表？', '提示', { type: 'warning' })
    await sheetApi.deleteSpreadsheet(currentId.value)
    sheets.value = sheets.value.filter((s) => s.id !== currentId.value)
    await loadSheet(sheets.value[0].id)
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return
    ElMessage.error(e.message || '删除失败')
  }
}
function onNameInput(): void {
  scheduleSave()
}

onMounted(loadSheets)
</script>

<template>
  <div class="flex h-full gap-6">
    <div class="flex-1 flex flex-col gap-4">
      <Card class="flex-1">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <FileSpreadsheet class="w-4 h-4 text-emerald-500" />
              <h3 class="text-sm font-semibold">表格</h3>
              <span class="inline-flex items-center gap-1 ml-1 px-2 py-0.5 rounded-full bg-muted/60 text-[10px] text-muted-foreground">
                <Keyboard class="w-3 h-3" /> 方向键/Tab/Enter 导航 · 支持粘贴
              </span>
            </div>
            <div class="flex items-center gap-2">
              <Input v-model="name" placeholder="工作表名称" class="h-8 text-xs w-32" @input="onNameInput" />
              <select
                :value="currentId"
                class="h-8 rounded-md border bg-background px-1 text-xs outline-none focus:border-input"
                @change="switchSheet(($event.target as HTMLSelectElement).value)"
              >
                <option v-for="s in sheets" :key="s.id" :value="s.id">{{ s.name }}</option>
              </select>
              <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5" @click="newSheet">
                <Plus class="w-3 h-3" /> 新建表
              </Button>
              <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5 text-destructive border-destructive/40" @click="deleteSheet">
                <Trash2 class="w-3 h-3" /> 删除表
              </Button>
            </div>
          </div>

          <div v-if="loading" class="flex-1 flex items-center justify-center text-sm text-muted-foreground">
            加载中...
          </div>

          <div v-else class="flex-1 overflow-auto border rounded-lg">
            <table class="border-collapse" style="table-layout: fixed">
              <thead>
                <tr class="bg-muted/50 text-xs text-muted-foreground sticky top-0 z-10">
                  <th class="w-10 px-0 py-1.5 font-medium border-b border-r text-center select-none">#</th>
                  <th
                    v-for="c in colCount"
                    :key="c - 1"
                    class="px-2 py-1.5 font-medium border-b border-r text-center select-none"
                    :class="activeCol === c - 1 ? 'bg-primary/10 text-primary' : ''"
                    style="min-width: 110px"
                  >
                    {{ colLabel(c - 1) }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, r) in grid" :key="r" class="transition-colors">
                  <td
                    class="px-0 py-0 border-b border-r text-center align-middle select-none bg-muted/30 text-[10px] text-muted-foreground"
                    :class="activeRow === r ? 'bg-primary/10 text-primary' : ''"
                  >
                    {{ r + 1 }}
                  </td>
                  <td
                    v-for="(val, c) in row"
                    :key="c"
                    class="p-0 border-b border-r align-middle"
                    :class="activeRow === r && activeCol === c ? 'ring-2 ring-inset ring-primary' : ''"
                  >
                    <input
                      :ref="(el) => (cellEls[cellKey(r, c)] = el as HTMLInputElement | null)"
                      :value="val"
                      class="w-full h-8 px-2 text-xs outline-none bg-transparent focus:bg-background"
                      @focus="selectCell(r, c)"
                      @click="selectCell(r, c)"
                      @input="onCellInput(r, c, ($event.target as HTMLInputElement).value)"
                      @keydown="onCellKeydown($event, r, c)"
                      @paste="onCellPaste($event, r, c)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 工具栏：增删行列 -->
          <div class="mt-3 flex items-center gap-2">
            <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5" @click="addRow()">
              <Plus class="w-3 h-3" /> 添加行
            </Button>
            <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5" @click="addCol()">
              <Plus class="w-3 h-3" /> 添加列
            </Button>
            <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5 text-destructive border-destructive/40" @click="deleteRow">
              <Trash2 class="w-3 h-3" /> 删除行
            </Button>
            <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5 text-destructive border-destructive/40" @click="deleteCol">
              <Trash2 class="w-3 h-3" /> 删除列
            </Button>
            <span class="ml-auto text-[10px] text-muted-foreground">
              {{ saving ? '保存中...' : `${rowCount} 行 × ${colCount} 列` }}
            </span>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
