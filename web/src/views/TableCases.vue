<script setup lang="ts">
import { ref, computed, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Trash2, FileSpreadsheet, Keyboard, Search, Bold, Italic, Underline,
  AlignLeft, AlignCenter, AlignRight, WrapText, PaintBucket, Merge, Split, Undo2, Redo2,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import * as sheetApi from '@/api/spreadsheet'
import type { Spreadsheet, CellFormat, MergeRange } from '@/types'

const MIN_ROWS = 24
const MIN_COLS = 8
const DEFAULT_COLS = 8
const DEF_COL_W = 110
const DEF_ROW_H = 32
const MIN_COL_W = 40
const MIN_ROW_H = 24

// ---- 数据 ----
const sheets = ref<Spreadsheet[]>([])
const currentId = ref<string>('')
const name = ref<string>('工作表')
const grid = ref<string[][]>([])
const formats = ref<Record<string, CellFormat>>({}) // "r,c" -> 格式
const colWidths = ref<Record<number, number>>({})
const rowHeights = ref<Record<number, number>>({})
const merges = ref<MergeRange[]>([])
const loading = ref(false)
const saving = ref(false)

// 选中单元格 + 选区（range）
const activeRow = ref(-1)
const activeCol = ref(-1)
const selAnchor = ref<{ r: number; c: number }>({ r: -1, c: -1 })
const cellEls = ref<Record<string, HTMLInputElement | null>>({})
const editDirty = ref(false)
function cellKey(r: number, c: number): string {
  return `${r}:${c}`
}
function fmtAt(r: number, c: number): CellFormat {
  return formats.value[r + ',' + c] || {}
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
const colCount = computed(() => grid.value[0]?.length || 0)

// ---- 撤销 / 重做 ----
interface SheetState {
  cells: string[][]
  formats: Record<string, CellFormat>
  colWidths: Record<number, number>
  rowHeights: Record<number, number>
  merges: MergeRange[]
}
const history = ref<SheetState[]>([])
const future = ref<SheetState[]>([])
const canUndo = computed(() => history.value.length > 0)
const canRedo = computed(() => future.value.length > 0)

function sheetState(): SheetState {
  return {
    cells: grid.value.map((r) => [...r]),
    formats: JSON.parse(JSON.stringify(formats.value)),
    colWidths: { ...colWidths.value },
    rowHeights: { ...rowHeights.value },
    merges: merges.value.map((m) => ({ ...m })),
  }
}
function pushHistory(): void {
  history.value.push(sheetState())
  if (history.value.length > 50) history.value.shift()
  future.value = []
}
function applyState(st: SheetState): void {
  grid.value = st.cells.map((r) => [...r])
  formats.value = JSON.parse(JSON.stringify(st.formats))
  colWidths.value = { ...st.colWidths }
  rowHeights.value = { ...st.rowHeights }
  merges.value = st.merges.map((m) => ({ ...m }))
  scheduleSave()
}
async function undo(): Promise<void> {
  if (!history.value.length) return
  future.value.push(sheetState())
  applyState(history.value.pop()!)
}
async function redo(): Promise<void> {
  if (!future.value.length) return
  history.value.push(sheetState())
  applyState(future.value.pop()!)
}

// ---- 合并布局（预计算 span / covered） ----
const mergeLayout = computed(() => {
  const span = new Map<string, { rs: number; cs: number }>()
  const covered = new Set<string>()
  for (const m of merges.value) {
    for (let i = 0; i < m.rs; i++) {
      for (let j = 0; j < m.cs; j++) {
        const key = m.r + i + ',' + (m.c + j)
        if (i === 0 && j === 0) span.set(key, { rs: m.rs, cs: m.cs })
        else covered.add(key)
      }
    }
  }
  return { span, covered }
})

// ---- 选区 ----
function selRect(): { r1: number; c1: number; r2: number; c2: number } {
  const a = selAnchor.value
  const b = { r: activeRow.value, c: activeCol.value }
  if (a.r < 0 || b.r < 0) return { r1: 0, c1: 0, r2: 0, c2: 0 }
  return {
    r1: Math.min(a.r, b.r),
    c1: Math.min(a.c, b.c),
    r2: Math.max(a.r, b.r),
    c2: Math.max(a.c, b.c),
  }
}
function inSelection(r: number, c: number): boolean {
  const { r1, c1, r2, c2 } = selRect()
  return r >= r1 && r <= r2 && c >= c1 && c <= c2
}

// ---- 加载 ----
function emptyGrid(rows: number, cols: number): string[][] {
  return Array.from({ length: rows }, () => Array.from({ length: cols }, () => ''))
}
function parseJSON<T>(raw: unknown, def: T): T {
  if (raw == null) return def
  if (typeof raw === 'object') return raw as T
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as T
    } catch {
      return def
    }
  }
  return def
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
      const created = await sheetApi.createSpreadsheet({ name: '工作表', cells: JSON.stringify(emptyGrid(MIN_ROWS, DEFAULT_COLS)) })
      sheets.value = [created]
      currentId.value = created.id
      grid.value = emptyGrid(MIN_ROWS, DEFAULT_COLS)
      name.value = created.name
      formats.value = {}
      colWidths.value = {}
      rowHeights.value = {}
      merges.value = []
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
    formats.value = parseJSON(s.formats as unknown, {})
    colWidths.value = parseJSON(s.colWidths as unknown, {})
    rowHeights.value = parseJSON(s.rowHeights as unknown, {})
    merges.value = parseJSON(s.merges as unknown, [])
    activeRow.value = -1
    activeCol.value = -1
    selAnchor.value = { r: -1, c: -1 }
  } catch (e: any) {
    ElMessage.error(e.message || '加载表格失败')
  }
}

async function switchSheet(id: string): Promise<void> {
  if (id === currentId.value) return
  await flushSave()
  await loadSheet(id)
}

// ---- 保存（防抖，整表写入；格式层一并提交） ----
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
      formats: formats.value,
      colWidths: colWidths.value,
      rowHeights: rowHeights.value,
      merges: merges.value,
    })
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// ---- 单元格选择 / 编辑 ----
function focusCell(r: number, c: number, selectAll = true): void {
  const el = cellEls.value[cellKey(r, c)]
  if (el) {
    el.focus()
    if (selectAll) el.select()
  }
}
function selectCell(r: number, c: number, extend = false): void {
  if (r < 0 || c < 0) return
  if (!extend) selAnchor.value = { r, c }
  activeRow.value = r
  activeCol.value = c
  focusCell(r, c, true)
  editDirty.value = false
}

function onCellInput(r: number, c: number, val: string): void {
  if (!grid.value[r]) grid.value[r] = []
  if (grid.value[r][c] !== val) {
    if (!editDirty.value) {
      pushHistory()
      editDirty.value = true
    }
    grid.value[r][c] = val
    scheduleSave()
  }
}

function onCellKeydown(e: KeyboardEvent, r: number, c: number): void {
  const rows = rowCount.value
  const cols = colCount.value
  const ctrl = e.ctrlKey || e.metaKey
  if (ctrl) {
    const k = e.key.toLowerCase()
    if (k === 'b') {
      e.preventDefault()
      toggleBold()
      return
    }
    if (k === 'i') {
      e.preventDefault()
      toggleItalic()
      return
    }
    if (k === 'u') {
      e.preventDefault()
      toggleUnderline()
      return
    }
    if (k === 'm') {
      e.preventDefault()
      mergeSelection()
      return
    }
    if (k === 'f') {
      e.preventDefault()
      openFind()
      return
    }
    if (k === 'c') {
      e.preventDefault()
      copyRange()
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
  if (e.key === 'Delete' || e.key === 'Backspace') {
    e.preventDefault()
    clearSelection()
    return
  }
  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      selectCell(Math.min(rows - 1, r + 1), c, e.shiftKey)
      break
    case 'ArrowUp':
      e.preventDefault()
      selectCell(Math.max(0, r - 1), c, e.shiftKey)
      break
    case 'ArrowLeft':
      if ((e.target as HTMLInputElement).selectionStart === 0) {
        e.preventDefault()
        selectCell(r, Math.max(0, c - 1), e.shiftKey)
      }
      break
    case 'ArrowRight':
      if ((e.target as HTMLInputElement).selectionStart === (e.target as HTMLInputElement).value.length) {
        e.preventDefault()
        selectCell(r, Math.min(cols - 1, c + 1), e.shiftKey)
      }
      break
    case 'Tab': {
      e.preventDefault()
      if (e.shiftKey) {
        if (c > 0) selectCell(r, c - 1, false)
        else if (r > 0) selectCell(r - 1, cols - 1, false)
      } else {
        if (c < cols - 1) selectCell(r, c + 1, false)
        else if (r < rows - 1) selectCell(r + 1, 0, false)
      }
      break
    }
    case 'Enter':
      e.preventDefault()
      if (e.shiftKey) {
        if (r > 0) selectCell(r - 1, c, false)
      } else if (r < rows - 1) selectCell(r + 1, c, false)
      break
  }
}

// 粘贴（从 Excel/WPS 复制的多行多列 TSV）
function onCellPaste(e: ClipboardEvent, r: number, c: number): void {
  const text = e.clipboardData?.getData('text') || ''
  if (!text.includes('\n') && !text.includes('\t')) return
  e.preventDefault()
  pushHistory()
  const lines = text.replace(/\r/g, '').split('\n')
  let rr = r
  for (const line of lines) {
    if (rr >= rowCount.value) addRowData()
    const cellsData = line.split('\t')
    let cc = c
    for (const val of cellsData) {
      if (cc >= colCount.value) addColData()
      grid.value[rr][cc] = val
      cc++
    }
    rr++
  }
  scheduleSave()
  nextTick(() => selectCell(Math.min(r + lines.length - 1, rowCount.value - 1), c))
}

// ---- 格式应用 ----
function applyFormat(patch: Partial<CellFormat>): void {
  if (activeRow.value < 0 || activeCol.value < 0) return
  pushHistory()
  const { r1, c1, r2, c2 } = selRect()
  for (let r = r1; r <= r2; r++) {
    for (let c = c1; c <= c2; c++) {
      if (mergeLayout.value.covered.has(r + ',' + c)) continue
      const key = r + ',' + c
      formats.value[key] = { ...(formats.value[key] || {}), ...patch }
    }
  }
  scheduleSave()
}
function toggleProp(prop: 'bold' | 'italic' | 'underline' | 'wrap' | 'border'): void {
  const cur = fmtAt(activeRow.value, activeCol.value)
  const on = !!cur[prop]
  applyFormat({ [prop]: !on } as Partial<CellFormat>)
}
function toggleBold(): void {
  toggleProp('bold')
}
function toggleItalic(): void {
  toggleProp('italic')
}
function toggleUnderline(): void {
  toggleProp('underline')
}
function toggleWrap(): void {
  toggleProp('wrap')
}
function toggleBorder(): void {
  toggleProp('border')
}
function setAlign(align: 'left' | 'center' | 'right'): void {
  applyFormat({ align })
}
function setFill(color: string): void {
  applyFormat({ fill: color })
}
function clearFormat(): void {
  if (activeRow.value < 0) return
  pushHistory()
  const { r1, c1, r2, c2 } = selRect()
  for (let r = r1; r <= r2; r++) {
    for (let c = c1; c <= c2; c++) {
      const key = r + ',' + c
      if (formats.value[key]) delete formats.value[key]
    }
  }
  scheduleSave()
}

// ---- 合并 / 取消合并 ----
function mergeSelection(): void {
  if (activeRow.value < 0) return
  const { r1, c1, r2, c2 } = selRect()
  if (r1 === r2 && c1 === c2) {
    ElMessage.warning('请选择多个单元格再合并')
    return
  }
  pushHistory()
  const rs = r2 - r1 + 1
  const cs = c2 - c1 + 1
  merges.value = merges.value.filter(
    (m) => !(m.r < r1 + rs && m.r + m.rs > r1 && m.c < c1 + cs && m.c + m.cs > c1),
  )
  merges.value.push({ r: r1, c: c1, rs, cs })
  scheduleSave()
}
function unmergeSelection(): void {
  if (activeRow.value < 0) return
  const { r1, c1, r2, c2 } = selRect()
  const before = merges.value.length
  merges.value = merges.value.filter((m) => !(m.r >= r1 && m.r <= r2 && m.c >= c1 && m.c <= c2))
  if (merges.value.length !== before) {
    pushHistory()
    scheduleSave()
  }
}

// ---- 清空选区内容 ----
function clearSelection(): void {
  if (activeRow.value < 0) return
  const { r1, c1, r2, c2 } = selRect()
  pushHistory()
  for (let r = r1; r <= r2; r++) {
    for (let c = c1; c <= c2; c++) {
      if (mergeLayout.value.covered.has(r + ',' + c)) continue
      if (!grid.value[r]) grid.value[r] = []
      grid.value[r][c] = ''
    }
  }
  scheduleSave()
}

// ---- 复制选区为 TSV ----
function copyText(text: string): void {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).catch(() => fallbackCopy(text))
  } else {
    fallbackCopy(text)
  }
}
function fallbackCopy(text: string): void {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
  } catch {
    /* ignore */
  }
  document.body.removeChild(ta)
}
function copyRange(): void {
  if (activeRow.value < 0) return
  const { r1, c1, r2, c2 } = selRect()
  const lines: string[] = []
  for (let r = r1; r <= r2; r++) {
    const row: string[] = []
    for (let c = c1; c <= c2; c++) row.push(grid.value[r]?.[c] ?? '')
    lines.push(row.join('\t'))
  }
  copyText(lines.join('\n'))
  ElMessage.success('已复制选区')
}

// ---- 增删行列（保持格式层一致） ----
function addRowData(): void {
  const cols = colCount.value || MIN_COLS
  grid.value.push(Array.from({ length: cols }, () => ''))
}
function addColData(): void {
  grid.value.forEach((row) => row.push(''))
}
function addRow(focus = true): void {
  addRowData()
  scheduleSave()
  if (focus) nextTick(() => selectCell(rowCount.value - 1, Math.max(0, activeCol.value)))
}
function addCol(focus = true): void {
  addColData()
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
  pushHistory()
  const r = activeRow.value
  grid.value.splice(r, 1)
  const nf: Record<string, CellFormat> = {}
  for (const k of Object.keys(formats.value)) {
    const [rr, cc] = k.split(',').map(Number)
    if (rr < r) nf[k] = formats.value[k]
    else if (rr > r) nf[rr - 1 + ',' + cc] = formats.value[k]
  }
  formats.value = nf
  const nm: MergeRange[] = []
  for (const m of merges.value) {
    if (m.r <= r && r < m.r + m.rs) continue
    nm.push({ ...m, r: m.r > r ? m.r - 1 : m.r })
  }
  merges.value = nm
  activeRow.value = Math.min(r, rowCount.value - 1)
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
  pushHistory()
  const c = activeCol.value
  grid.value.forEach((row) => row.splice(c, 1))
  const nf: Record<string, CellFormat> = {}
  for (const k of Object.keys(formats.value)) {
    const [rr, cc] = k.split(',').map(Number)
    if (cc < c) nf[k] = formats.value[k]
    else if (cc > c) nf[rr + ',' + (cc - 1)] = formats.value[k]
  }
  formats.value = nf
  const nm: MergeRange[] = []
  for (const m of merges.value) {
    if (m.c <= c && c < m.c + m.cs) continue
    nm.push({ ...m, c: m.c > c ? m.c - 1 : m.c })
  }
  merges.value = nm
  activeCol.value = Math.min(c, colCount.value - 1)
  scheduleSave()
}

// ---- 列宽 / 行高 拖拽调整 ----
const resizing = ref<{ type: 'col' | 'row'; index: number; start: number; startSize: number } | null>(null)
function onResizeMove(e: MouseEvent): void {
  const rz = resizing.value
  if (!rz) return
  if (rz.type === 'col') {
    colWidths.value[rz.index] = Math.max(MIN_COL_W, Math.round(rz.startSize + (e.clientX - rz.start)))
  } else {
    rowHeights.value[rz.index] = Math.max(MIN_ROW_H, Math.round(rz.startSize + (e.clientY - rz.start)))
  }
}
function onResizeEnd(): void {
  if (!resizing.value) return
  resizing.value = null
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
  scheduleSave()
}
function startColResize(c: number, e: MouseEvent): void {
  e.preventDefault()
  e.stopPropagation()
  pushHistory()
  resizing.value = { type: 'col', index: c, start: e.clientX, startSize: colWidths.value[c] || DEF_COL_W }
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
}
function startRowResize(r: number, e: MouseEvent): void {
  e.preventDefault()
  e.stopPropagation()
  pushHistory()
  resizing.value = { type: 'row', index: r, start: e.clientY, startSize: rowHeights.value[r] || DEF_ROW_H }
  document.addEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
}
function autofitCol(c: number): void {
  let max = 0
  grid.value.forEach((row) => {
    const v = (row[c] || '').toString()
    if (v.length > max) max = v.length
  })
  const w = Math.max(MIN_COL_W, Math.min(400, max * 8 + 24))
  pushHistory()
  colWidths.value[c] = w
  scheduleSave()
}
function colStyle(c: number): Record<string, string> {
  return { width: (colWidths.value[c] || DEF_COL_W) + 'px', minWidth: (colWidths.value[c] || DEF_COL_W) + 'px' }
}
function rowStyle(r: number): Record<string, string> {
  return { height: (rowHeights.value[r] || DEF_ROW_H) + 'px' }
}

// ---- 单元格样式（格式 + 选中） ----
function cellClass(r: number, c: number): string[] {
  const cls: string[] = []
  const f = fmtAt(r, c)
  if (f.bold) cls.push('font-bold')
  if (f.italic) cls.push('italic')
  if (f.underline) cls.push('underline')
  const align = f.align || 'left'
  if (align === 'center') cls.push('text-center')
  else if (align === 'right') cls.push('text-right')
  else cls.push('text-left')
  if (f.wrap) cls.push('whitespace-normal break-words align-top')
  else cls.push('whitespace-nowrap overflow-hidden')
  if (activeRow.value === r && activeCol.value === c) cls.push('ring-2 ring-inset ring-primary z-10')
  else if (inSelection(r, c)) cls.push('bg-primary/10')
  return cls
}
function cellStyle(r: number, c: number): Record<string, string> {
  const f = fmtAt(r, c)
  const st: Record<string, string> = { ...rowStyle(r) }
  if (f.fill) st.backgroundColor = f.fill
  if (f.color) st.color = f.color
  if (f.border) st.boxShadow = 'inset 0 0 0 1px hsl(var(--border))'
  return st
}

// ---- 查找 ----
const findOpen = ref(false)
const findText = ref('')
const findIndex = ref(0)
const findInput = ref<HTMLInputElement | null>(null)
const findMatches = computed(() => {
  const q = findText.value.toLowerCase().trim()
  if (!q) return []
  const res: { r: number; c: number }[] = []
  for (let r = 0; r < grid.value.length; r++) {
    for (let c = 0; c < (grid.value[r]?.length || 0); c++) {
      if ((grid.value[r][c] || '').toLowerCase().includes(q)) res.push({ r, c })
    }
  }
  return res
})
function openFind(): void {
  if (activeRow.value < 0) selectCell(0, 0)
  findOpen.value = true
  nextTick(() => findInput.value?.focus())
}
function closeFind(): void {
  findOpen.value = false
}
function onFindInput(): void {
  findIndex.value = 0
  if (findMatches.value.length > 0) {
    const m = findMatches.value[0]
    selectCell(m.r, m.c)
  }
}
function gotoFind(delta: number): void {
  if (findMatches.value.length === 0) return
  findIndex.value = (findIndex.value + delta + findMatches.value.length) % findMatches.value.length
  const m = findMatches.value[findIndex.value]
  selectCell(m.r, m.c)
}
function isFindMatch(r: number, c: number): boolean {
  return findMatches.value.some((m) => m.r === r && m.c === c)
}
function isFindActive(r: number, c: number): boolean {
  const m = findMatches.value[findIndex.value]
  return !!m && m.r === r && m.c === c
}

// ---- 表管理 ----
async function newSheet(): Promise<void> {
  await flushSave()
  try {
    const created = await sheetApi.createSpreadsheet({ name: `工作表${sheets.value.length + 1}`, cells: JSON.stringify(emptyGrid(MIN_ROWS, DEFAULT_COLS)) })
    sheets.value = [...sheets.value, created]
    await loadSheet(created.id)
    ElMessage.success('已新建工作表')
  } catch (e: any) {
    ElMessage.error(e.message || '新建失败')
  }
}

// 测试用例模板：预填行业标准列头 + 表头加粗居中浅底格式
const TESTCASE_HEADERS = ['用例编号', '用例标题', '所属模块', '前置条件', '输入数据', '测试步骤', '预期结果', '优先级', '类型', '负责人', '状态', '备注']
const TESTCASE_COL_W = [110, 180, 120, 160, 140, 220, 200, 80, 100, 100, 90, 160]
async function newTestCaseSheet(): Promise<void> {
  await flushSave()
  try {
    const cols = TESTCASE_HEADERS.length
    const g: string[][] = Array.from({ length: MIN_ROWS }, () => Array.from({ length: cols }, () => ''))
    TESTCASE_HEADERS.forEach((h, c) => {
      g[0][c] = h
    })
    const fm: Record<string, CellFormat> = {}
    const cw: Record<number, number> = {}
    TESTCASE_HEADERS.forEach((_, c) => {
      fm['0,' + c] = { bold: true, align: 'center', fill: '#f1f5f9' }
      cw[c] = TESTCASE_COL_W[c]
    })
    const created = await sheetApi.createSpreadsheet({
      name: `测试用例表${sheets.value.length + 1}`,
      cells: JSON.stringify(g),
      formats: fm,
      colWidths: cw,
      rowHeights: {},
      merges: [],
    })
    sheets.value = [...sheets.value, created]
    await loadSheet(created.id)
    ElMessage.success('已新建测试用例表（含标准列头）')
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
onBeforeUnmount(() => {
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
})
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
              <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5" @click="newTestCaseSheet">
                <FileSpreadsheet class="w-3 h-3" /> 测试用例模板
              </Button>
              <Button size="sm" variant="outline" class="h-8 rounded-lg text-xs gap-1.5 text-destructive border-destructive/40" @click="deleteSheet">
                <Trash2 class="w-3 h-3" /> 删除表
              </Button>
            </div>
          </div>

          <!-- 格式工具栏 -->
          <div class="flex items-center gap-1 mb-2 flex-wrap">
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).bold ? 'bg-accent text-foreground' : ''" title="加粗 (Ctrl+B)" :disabled="activeRow < 0" @click="toggleBold">
              <Bold class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).italic ? 'bg-accent text-foreground' : ''" title="斜体 (Ctrl+I)" :disabled="activeRow < 0" @click="toggleItalic">
              <Italic class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).underline ? 'bg-accent text-foreground' : ''" title="下划线 (Ctrl+U)" :disabled="activeRow < 0" @click="toggleUnderline">
              <Underline class="w-3.5 h-3.5" />
            </button>
            <span class="w-px h-5 bg-border mx-1" />
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).align === 'left' ? 'bg-accent text-foreground' : ''" title="左对齐" :disabled="activeRow < 0" @click="setAlign('left')">
              <AlignLeft class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).align === 'center' ? 'bg-accent text-foreground' : ''" title="居中" :disabled="activeRow < 0" @click="setAlign('center')">
              <AlignCenter class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).align === 'right' ? 'bg-accent text-foreground' : ''" title="右对齐" :disabled="activeRow < 0" @click="setAlign('right')">
              <AlignRight class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).wrap ? 'bg-accent text-foreground' : ''" title="自动换行" :disabled="activeRow < 0" @click="toggleWrap">
              <WrapText class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).border ? 'bg-accent text-foreground' : ''" title="单元格边框" :disabled="activeRow < 0" @click="toggleBorder">
              <span class="block w-3.5 h-3.5 border border-current" />
            </button>
            <span class="w-px h-5 bg-border mx-1" />
            <label class="flex items-center gap-1 px-1.5 h-7 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :class="fmtAt(activeRow, activeCol).fill ? 'bg-accent text-foreground' : ''" title="填充颜色">
              <PaintBucket class="w-3.5 h-3.5" />
              <input type="color" class="w-4 h-4 border-0 bg-transparent p-0 cursor-pointer" :disabled="activeRow < 0" @input="setFill(($event.target as HTMLInputElement).value)" />
            </label>
            <button class="px-2 h-7 rounded hover:bg-accent text-muted-foreground text-[10px]" title="清除格式" :disabled="activeRow < 0" @click="clearFormat">
              清除
            </button>
            <span class="w-px h-5 bg-border mx-1" />
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" title="合并单元格 (Ctrl+M)" :disabled="activeRow < 0" @click="mergeSelection">
              <Merge class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" title="取消合并" :disabled="activeRow < 0" @click="unmergeSelection">
              <Split class="w-3.5 h-3.5" />
            </button>
            <span class="w-px h-5 bg-border mx-1" />
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :disabled="!canUndo" title="撤销 (Ctrl+Z)" @click="undo">
              <Undo2 class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground disabled:opacity-30" :disabled="!canRedo" title="重做 (Ctrl+Y)" @click="redo">
              <Redo2 class="w-3.5 h-3.5" />
            </button>
            <button class="p-1.5 rounded hover:bg-accent text-muted-foreground" title="查找 (Ctrl+F)" @click="openFind">
              <Search class="w-3.5 h-3.5" />
            </button>
          </div>

          <!-- 查找条 -->
          <div v-if="findOpen" class="flex items-center gap-2 mb-2 px-2 py-1.5 rounded-lg border bg-muted/40">
            <Search class="w-3.5 h-3.5 text-muted-foreground" />
            <input
              ref="findInput"
              v-model="findText"
              placeholder="查找内容..."
              class="h-7 flex-1 text-xs bg-background border rounded px-2 outline-none focus:border-input"
              @input="onFindInput"
              @keydown.enter.prevent="gotoFind(1)"
            />
            <span class="text-[10px] text-muted-foreground">{{ findMatches.length ? findIndex + 1 : 0 }}/{{ findMatches.length }}</span>
            <button class="px-2 h-7 rounded text-xs hover:bg-accent" @click="gotoFind(-1)">上一个</button>
            <button class="px-2 h-7 rounded text-xs hover:bg-accent" @click="gotoFind(1)">下一个</button>
            <button class="px-2 h-7 rounded text-xs hover:bg-accent" @click="closeFind">关闭</button>
          </div>

          <div v-if="loading" class="flex-1 flex items-center justify-center text-sm text-muted-foreground">
            加载中...
          </div>

          <div v-else class="flex-1 overflow-auto border rounded-lg">
            <table class="border-collapse" style="table-layout: fixed">
              <thead>
                <tr class="bg-muted/50 text-xs text-muted-foreground sticky top-0 z-10">
                  <th class="w-10 px-0 py-1.5 font-medium border-b border-r text-center select-none relative" style="min-width: 40px">#</th>
                  <th
                    v-for="c in colCount"
                    :key="c - 1"
                    class="px-2 py-1.5 font-medium border-b border-r text-center select-none relative"
                    :class="activeCol === c - 1 ? 'bg-primary/10 text-primary' : ''"
                    :style="colStyle(c - 1)"
                  >
                    {{ colLabel(c - 1) }}
                    <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-primary/40" @mousedown.stop="startColResize(c - 1, $event)" @dblclick.stop="autofitCol(c - 1)" />
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, r) in grid" :key="r" class="transition-colors">
                  <td
                    class="px-0 py-0 border-b border-r text-center align-middle select-none bg-muted/30 text-[10px] text-muted-foreground relative"
                    :class="activeRow === r ? 'bg-primary/10 text-primary' : ''"
                    :style="rowStyle(r)"
                  >
                    {{ r + 1 }}
                    <div class="absolute bottom-0 left-0 w-full h-1 cursor-row-resize hover:bg-primary/40" @mousedown.stop="startRowResize(r, $event)" />
                  </td>
                  <template v-for="(val, c) in row" :key="c">
                    <td
                      v-if="!mergeLayout.covered.has(r + ',' + c)"
                      class="p-0 border-b border-r align-middle relative"
                      :class="cellClass(r, c)"
                      :style="cellStyle(r, c)"
                      :rowspan="mergeLayout.span.get(r + ',' + c)?.rs"
                      :colspan="mergeLayout.span.get(r + ',' + c)?.cs"
                      @click="selectCell(r, c, ($event.shiftKey || $event.metaKey || $event.ctrlKey))"
                      @dblclick="focusCell(r, c, false)"
                    >
                      <input
                        :ref="(el) => (cellEls[cellKey(r, c)] = el as HTMLInputElement | null)"
                        :value="val"
                        class="w-full h-full px-2 text-xs outline-none bg-transparent focus:bg-background"
                        :class="isFindMatch(r, c) ? (isFindActive(r, c) ? 'bg-amber-300/70' : 'bg-amber-200/40') : ''"
                        @input="onCellInput(r, c, ($event.target as HTMLInputElement).value)"
                        @keydown="onCellKeydown($event, r, c)"
                        @paste="onCellPaste($event, r, c)"
                      />
                    </td>
                  </template>
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

          <div class="mt-1 text-[10px] text-muted-foreground">
            拖拽列/行表头分隔线可调整宽高（双击列标自动适应内容）· Shift+点击/方向键 框选 · Ctrl+C 复制选区 · Ctrl+F 查找 · Ctrl+Z/Y 撤销重做
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
