<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  BookOpen, Plus, Trash2, Edit3, Search, ChevronDown, ChevronRight,
  FolderOpen, Tag, Save, Send, Upload,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import * as apidefsApi from '@/api/apidefs'
import { safeParseJSON, formatDate } from '@/utils'
import type { APIDefinition, APIDefModule, HttpMethod } from '@/types'

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  PUT: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
  PATCH: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  HEAD: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400',
  OPTIONS: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400',
}

const METHOD_ORDER: HttpMethod[] = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']

interface EditableDef {
  id?: string
  name: string
  method: HttpMethod
  url: string
  moduleId: string
  description: string
  contentType: string
  headers: string // JSON 字符串
  body: string
  responseCode: number
  responseExample: string
}

function blankDef(): EditableDef {
  return {
    name: '', method: 'GET', url: '', moduleId: '',
    description: '', contentType: 'application/json', headers: '{}',
    body: '', responseCode: 200, responseExample: '',
  }
}

/* 后端 APIDefinition 缺少 description/contentType/response 字段，仅在本会话内保留显示 */
interface DefMeta {
  description?: string
  contentType?: string
  responseCode?: number
  responseExample?: string
}
const metaMap = ref<Record<string, DefMeta>>({})

const definitions = ref<APIDefinition[]>([])
const modules = ref<APIDefModule[]>([])
const loading = ref(false)
const searchTerm = ref('')
const selectedDefId = ref<string | null>(null)
const editingDef = ref<EditableDef | null>(null)
const isCreating = ref(false)
const expandedModules = ref<Set<string>>(new Set())
const showImport = ref(false)
const importUrl = ref('')
const importStatus = ref('')

const selectedDef = computed(() => definitions.value.find((d) => d.id === selectedDefId.value) || null)

const filteredDefs = computed(() => {
  const t = searchTerm.value.trim().toLowerCase()
  if (!t) return definitions.value
  return definitions.value.filter((d) =>
    d.name.toLowerCase().includes(t) ||
    d.url.toLowerCase().includes(t) ||
    safeParseJSON<string[]>(d.tags, []).some((tag) => tag.toLowerCase().includes(t)),
  )
})

const groupedDefs = computed<Record<string, APIDefinition[]>>(() => {
  const map: Record<string, APIDefinition[]> = {}
  filteredDefs.value.forEach((d) => {
    const mid = d.moduleId || 'uncategorized'
    if (!map[mid]) map[mid] = []
    map[mid].push(d)
  })
  return map
})

const uncategorizedDefs = computed(() => groupedDefs.value['uncategorized'] || [])

const stats = computed(() => ({
  total: definitions.value.length,
  methods: {
    GET: definitions.value.filter((d) => d.method === 'GET').length,
    POST: definitions.value.filter((d) => d.method === 'POST').length,
    PUT: definitions.value.filter((d) => d.method === 'PUT').length,
    DELETE: definitions.value.filter((d) => d.method === 'DELETE').length,
  },
}))

function toggleModule(id: string): void {
  const next = new Set(expandedModules.value)
  next.has(id) ? next.delete(id) : next.add(id)
  expandedModules.value = next
}

function defTags(d: APIDefinition): string[] {
  return safeParseJSON<string[]>(d.tags, [])
}
function defHeaders(d: APIDefinition): Record<string, string> {
  return safeParseJSON<Record<string, string>>(d.headers, {})
}

async function handleNewFolder(): Promise<void> {
  try {
    const { value } = await ElMessageBox.prompt('请输入模块名称：', '新建模块', {
      inputPattern: /\S+/, inputErrorMessage: '名称不能为空',
    })
    await apidefsApi.createApiDefModule({ name: value.trim(), parentId: '' })
    ElMessage.success('已创建模块')
    await loadAll()
  } catch { /* cancelled */ }
}

async function handleDeleteFolder(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除此模块及其下接口？', '提示', { type: 'warning' })
    const modDefs = definitions.value.filter((d) => d.moduleId === id)
    for (const d of modDefs) await apidefsApi.deleteApiDefinition(d.id)
    await apidefsApi.deleteApiDefModule(id)
    delete metaMap.value[id]
    if (selectedDefId.value && modDefs.some((d) => d.id === selectedDefId.value)) selectedDefId.value = null
    ElMessage.success('已删除')
    await loadAll()
  } catch { /* cancelled */ }
}

function startCreate(moduleId?: string): void {
  const def = blankDef()
  if (moduleId) def.moduleId = moduleId
  editingDef.value = def
  isCreating.value = true
}

function startEdit(def: APIDefinition): void {
  const meta = metaMap.value[def.id] || {}
  editingDef.value = {
    id: def.id,
    name: def.name,
    method: def.method,
    url: def.url,
    moduleId: def.moduleId,
    description: meta.description || '',
    contentType: meta.contentType || 'application/json',
    headers: def.headers || '{}',
    body: def.body || '',
    responseCode: meta.responseCode || 200,
    responseExample: meta.responseExample || '',
  }
  isCreating.value = false
}

async function saveEditing(): Promise<void> {
  const e = editingDef.value
  if (!e || !e.name.trim() || !e.url.trim()) { ElMessage.warning('请填写名称和路径'); return }
  const payload: Partial<APIDefinition> = {
    name: e.name,
    method: e.method,
    url: e.url,
    moduleId: e.moduleId,
    tags: JSON.stringify([]),
    headers: e.headers,
    body: e.body,
  }
  if (isCreating.value) {
    const created = await apidefsApi.createApiDefinition(payload)
    metaMap.value[created.id] = {
      description: e.description,
      contentType: e.contentType,
      responseCode: e.responseCode,
      responseExample: e.responseExample,
    }
    ElMessage.success('已创建')
  } else if (e.id) {
    await apidefsApi.updateApiDefinition(e.id, payload)
    metaMap.value[e.id] = {
      description: e.description,
      contentType: e.contentType,
      responseCode: e.responseCode,
      responseExample: e.responseExample,
    }
    ElMessage.success('已更新')
  }
  editingDef.value = null
  await loadAll()
}

async function handleDelete(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm('确定删除此接口定义？', '提示', { type: 'warning' })
    await apidefsApi.deleteApiDefinition(id)
    delete metaMap.value[id]
    if (selectedDefId.value === id) selectedDefId.value = null
    ElMessage.success('已删除')
    await loadAll()
  } catch { /* cancelled */ }
}

/* OpenAPI / Swagger 解析（本地，最小实现） */
function parseOpenApi(spec: any): { modules: Partial<APIDefModule>[]; defs: Partial<APIDefinition>[] } {
  const mods: Partial<APIDefModule>[] = []
  const defs: Partial<APIDefinition>[] = []
  const tagMap: Record<string, string> = {}
  const tags = spec.tags || []
  tags.forEach((t: any, i: number) => {
    const id = 'dm-imp-' + Date.now() + '-' + i
    tagMap[t.name] = id
    mods.push({ id, name: t.name, parentId: '', sortOrder: i })
  })
  const paths = spec.paths || {}
  Object.entries(paths).forEach(([path, ops]: any) => {
    Object.entries(ops).forEach(([method, op]: any) => {
      if (!['get', 'post', 'put', 'delete', 'patch', 'head', 'options'].includes(method)) return
      const tag = op.tags && op.tags[0]
      defs.push({
        id: 'def-imp-' + Date.now() + '-' + Math.random().toString(36).slice(2),
        name: op.summary || op.operationId || (method.toUpperCase() + ' ' + path),
        method: method.toUpperCase(),
        url: path,
        moduleId: tag ? (tagMap[tag] || '') : '',
        tags: JSON.stringify([]),
        headers: '{}',
        body: '',
      })
    })
  })
  return { modules: mods, defs }
}

async function importSpec(spec: any): Promise<void> {
  const result = parseOpenApi(spec)
  for (const m of result.modules) await apidefsApi.createApiDefModule(m)
  for (const d of result.defs) await apidefsApi.createApiDefinition(d)
  await loadAll()
  importStatus.value = `导入成功：${result.modules.length} 个模块，${result.defs.length} 个接口`
}

async function handleFileImport(e: Event): Promise<void> {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importStatus.value = '正在解析...'
  try {
    const text = await file.text()
    const spec = JSON.parse(text)
    await importSpec(spec)
  } catch {
    importStatus.value = '解析失败，请检查文件格式是否正确'
  }
  input.value = ''
}

async function handleUrlImport(): Promise<void> {
  if (!importUrl.value.trim()) return
  importStatus.value = '正在从 URL 获取...'
  try {
    const res = await fetch(importUrl.value)
    const spec = await res.json()
    await importSpec(spec)
  } catch {
    importStatus.value = '获取或解析失败，请检查 URL 是否正确'
  }
}

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [ds, ms] = await Promise.all([
      apidefsApi.getApiDefinitions(),
      apidefsApi.getApiDefModules(),
    ])
    definitions.value = ds
    modules.value = ms
    expandedModules.value = new Set(ms.map((m) => m.id))
  } catch (err) {
    ElMessage.error((err as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(loadAll)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">接口定义</h1>
        <p class="text-muted-foreground mt-0.5">接口文档管理、Swagger 导入、参数定义</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" class="gap-2" @click="showImport = true"><Upload class="w-4 h-4" /> 导入</Button>
        <Button variant="outline" class="gap-2" @click="handleNewFolder"><FolderOpen class="w-4 h-4" /> 新建模块</Button>
        <Button class="gap-2" @click="startCreate()"><Plus class="w-4 h-4" /> 新建接口</Button>
      </div>
    </div>

    <!-- Stats -->
    <div class="grid gap-4 md:grid-cols-5">
      <Card><CardContent class="p-3 text-center"><p class="text-xl font-bold">{{ stats.total }}</p><p class="text-xs text-muted-foreground">接口总数</p></CardContent></Card>
      <Card><CardContent class="p-3 text-center"><p class="text-xl font-bold text-emerald-600">{{ stats.methods.GET }}</p><p class="text-xs text-muted-foreground">GET</p></CardContent></Card>
      <Card><CardContent class="p-3 text-center"><p class="text-xl font-bold text-blue-600">{{ stats.methods.POST }}</p><p class="text-xs text-muted-foreground">POST</p></CardContent></Card>
      <Card><CardContent class="p-3 text-center"><p class="text-xl font-bold text-amber-600">{{ stats.methods.PUT }}</p><p class="text-xs text-muted-foreground">PUT</p></CardContent></Card>
      <Card><CardContent class="p-3 text-center"><p class="text-xl font-bold text-red-600">{{ stats.methods.DELETE }}</p><p class="text-xs text-muted-foreground">DELETE</p></CardContent></Card>
    </div>

    <div class="flex gap-6">
      <!-- Left: Module Tree -->
      <div class="w-64 shrink-0 space-y-1">
        <div class="relative mb-2">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input v-model="searchTerm" placeholder="搜索接口..." class="pl-9" />
        </div>

        <div v-for="mod in modules" :key="mod.id">
          <div class="flex items-center gap-1 px-2 py-1.5 rounded-lg cursor-pointer hover:bg-accent/50 transition-colors group"
            @click="toggleModule(mod.id)">
            <ChevronDown v-if="expandedModules.has(mod.id)" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <ChevronRight v-else class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <FolderOpen class="w-3.5 h-3.5 text-amber-500 shrink-0" />
            <span class="text-xs font-medium flex-1 truncate">{{ mod.name }}</span>
            <span class="text-xs text-muted-foreground">{{ (groupedDefs[mod.id] || []).length }}</span>
            <button class="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-red-500 ml-1"
              @click.stop="handleDeleteFolder(mod.id)">
              <Trash2 class="w-3 h-3" />
            </button>
          </div>
          <template v-if="expandedModules.has(mod.id)">
            <div v-for="d in (groupedDefs[mod.id] || [])" :key="d.id"
              :class="cn('flex items-center gap-2 ml-5 px-2 py-1.5 rounded-lg cursor-pointer transition-colors text-xs',
                selectedDefId === d.id ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50')"
              @click="selectedDefId = d.id">
              <span :class="cn('px-1 py-0.5 rounded text-xs font-bold shrink-0', METHOD_COLORS[d.method])">{{ d.method }}</span>
              <span class="truncate">{{ d.url }}</span>
              <span class="truncate text-muted-foreground ml-auto">{{ d.name }}</span>
            </div>
            <div v-if="(groupedDefs[mod.id] || []).length === 0" class="ml-8 py-1 text-xs text-muted-foreground">暂无接口</div>
            <button class="ml-8 py-1 text-xs text-muted-foreground hover:text-primary flex items-center gap-1" @click="startCreate(mod.id)">
              <Plus class="w-3 h-3" /> 添加接口
            </button>
          </template>
        </div>

        <div v-if="uncategorizedDefs.length > 0">
          <div class="flex items-center gap-1 px-2 py-1.5 text-xs text-muted-foreground">
            <FolderOpen class="w-3.5 h-3.5" /> 未分类
            <span class="ml-auto text-xs">{{ uncategorizedDefs.length }}</span>
          </div>
          <div v-for="d in uncategorizedDefs" :key="d.id"
            :class="cn('flex items-center gap-2 ml-5 px-2 py-1.5 rounded-lg cursor-pointer transition-colors text-xs',
              selectedDefId === d.id ? 'bg-accent' : 'hover:bg-accent/50')"
            @click="selectedDefId = d.id">
            <span :class="cn('px-1 py-0.5 rounded text-xs font-bold shrink-0', METHOD_COLORS[d.method])">{{ d.method }}</span>
            <span class="truncate">{{ d.url }}</span>
          </div>
        </div>

        <div v-if="modules.length === 0 && definitions.length === 0" class="text-center py-8 text-muted-foreground">
          <BookOpen class="w-8 h-8 mx-auto mb-2 opacity-20" />
          <p class="text-xs">暂无接口定义</p>
        </div>
      </div>

      <!-- Right: Detail / Editor -->
      <div class="flex-1 min-w-0">
        <Card v-if="editingDef">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ isCreating ? '新建接口' : '编辑接口' }}</CardTitle>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-[auto_1fr] gap-3">
              <div>
                <label class="text-xs font-medium mb-1 block">方法</label>
                <select v-model="editingDef.method" class="h-9 rounded-md border border-input bg-background px-3 text-sm font-bold">
                  <option v-for="m in METHOD_ORDER" :key="m" :value="m">{{ m }}</option>
                </select>
              </div>
              <div>
                <label class="text-xs font-medium mb-1 block">接口路径 *</label>
                <Input v-model="editingDef.url" placeholder="/api/example" class="font-mono" />
              </div>
            </div>
            <div>
              <label class="text-xs font-medium mb-1 block">接口名称 *</label>
              <Input v-model="editingDef.name" placeholder="描述接口用途" />
            </div>
            <div>
              <label class="text-xs font-medium mb-1 block">描述</label>
              <textarea v-model="editingDef.description" class="w-full h-16 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none" />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-xs font-medium mb-1 block">所属模块</label>
                <select v-model="editingDef.moduleId" class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm">
                  <option value="">未分类</option>
                  <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.name }}</option>
                </select>
              </div>
              <div>
                <label class="text-xs font-medium mb-1 block">Content-Type</label>
                <Input v-model="editingDef.contentType" />
              </div>
            </div>
            <div>
              <label class="text-xs font-medium mb-1 block">请求头 (JSON)</label>
              <textarea v-model="editingDef.headers" class="w-full h-20 rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-none" placeholder='{"Authorization": "Bearer ..."}' />
            </div>
            <div>
              <label class="text-xs font-medium mb-1 block">请求 Body</label>
              <textarea v-model="editingDef.body" class="w-full h-32 rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-none" placeholder='{"key": "value"}' />
            </div>
            <div>
              <label class="text-xs font-medium mb-1 block">响应示例</label>
              <div class="flex items-center gap-2 mb-2">
                <span class="text-xs text-muted-foreground">状态码：</span>
                <input type="number" v-model.number="editingDef.responseCode" class="h-8 w-20 rounded-md border border-input bg-background px-2 text-sm" />
              </div>
              <textarea v-model="editingDef.responseExample" class="w-full h-40 rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-none" />
            </div>
            <div class="flex gap-2 pt-2 border-t">
              <Button class="flex-1 gap-2" @click="saveEditing"><Save class="w-4 h-4" /> 保存</Button>
              <Button variant="outline" @click="editingDef = null">取消</Button>
            </div>
          </CardContent>
        </Card>

        <Card v-else-if="selectedDef">
          <CardHeader class="pb-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span :class="cn('px-2 py-0.5 rounded text-xs font-bold', METHOD_COLORS[selectedDef.method])">{{ selectedDef.method }}</span>
                <code class="text-sm font-mono">{{ selectedDef.url }}</code>
              </div>
              <div class="flex gap-1">
                <Button variant="outline" size="sm" class="gap-1 h-8 text-xs" @click="() => {}"><Send class="w-3 h-3" /> 测试</Button>
                <Button variant="ghost" size="icon" class="h-8 w-8" @click="startEdit(selectedDef)"><Edit3 class="w-3.5 h-3.5" /></Button>
                <Button variant="ghost" size="icon" class="h-8 w-8" @click="handleDelete(selectedDef.id)"><Trash2 class="w-3.5 h-3.5 text-red-500" /></Button>
              </div>
            </div>
          </CardHeader>
          <CardContent class="space-y-4">
            <div>
              <h3 class="font-semibold">{{ selectedDef.name }}</h3>
              <p class="text-sm text-muted-foreground mt-0.5">{{ (metaMap[selectedDef.id]?.description) || '无描述' }}</p>
            </div>
            <div class="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
              <div><span class="font-medium">模块：</span>{{ modules.find((m) => m.id === selectedDef?.moduleId)?.name || '未分类' }}</div>
              <div><span class="font-medium">Content-Type：</span>{{ metaMap[selectedDef.id]?.contentType || 'application/json' }}</div>
              <div v-if="defTags(selectedDef).length > 0" class="flex items-center gap-1 col-span-2"><Tag class="w-3 h-3" />{{ defTags(selectedDef).join(', ') }}</div>
            </div>

            <div v-if="Object.keys(defHeaders(selectedDef)).length > 0">
              <p class="text-xs font-medium text-muted-foreground mb-1">请求头</p>
              <div class="bg-muted/30 rounded-lg p-3 space-y-1">
                <div v-for="(val, key) in defHeaders(selectedDef)" :key="key" class="text-xs">
                  <code class="text-primary">{{ key }}</code><span class="text-muted-foreground">: {{ val }}</span>
                </div>
              </div>
            </div>

            <div v-if="selectedDef.body">
              <p class="text-xs font-medium text-muted-foreground mb-1">请求 Body</p>
              <pre class="bg-muted/50 p-3 rounded-lg text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap">{{ selectedDef.body }}</pre>
            </div>

            <div v-if="metaMap[selectedDef.id]?.responseExample">
              <p class="text-xs font-medium text-muted-foreground mb-1">响应示例（{{ metaMap[selectedDef.id]?.responseCode || 200 }}）</p>
              <pre class="bg-muted/50 p-3 rounded-lg text-xs font-mono overflow-auto max-h-48 whitespace-pre-wrap">{{ metaMap[selectedDef.id]?.responseExample }}</pre>
            </div>
          </CardContent>
        </Card>

        <div v-else class="flex items-center justify-center h-64 text-muted-foreground">
          <div class="text-center">
            <BookOpen class="w-12 h-12 mx-auto mb-3 opacity-20" />
            <p class="text-sm">从左侧选择接口查看详情，或点击「新建接口」</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Import Dialog -->
    <div v-if="showImport" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" @click="showImport = false">
      <Card class="w-[480px]" @click.stop>
        <CardHeader><CardTitle class="text-lg">导入接口定义</CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <div>
            <label class="text-sm font-medium mb-1 block">从 URL 导入 (OpenAPI/Swagger JSON)</label>
            <div class="flex gap-2">
              <Input v-model="importUrl" placeholder="https://example.com/api-docs" class="flex-1" />
              <Button variant="outline" size="sm" :disabled="!importUrl.trim()" @click="handleUrlImport">获取</Button>
            </div>
          </div>
          <div class="border-t pt-4">
            <label class="text-sm font-medium mb-1 block">上传文件 (JSON)</label>
            <input type="file" accept=".json" @change="handleFileImport" class="block w-full text-sm file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-input file:bg-muted file:text-sm file:font-medium hover:file:bg-accent" />
          </div>
          <div v-if="importStatus" :class="cn('text-sm p-3 rounded-lg', importStatus.includes('成功') ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400' : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400')">
            {{ importStatus }}
          </div>
          <div class="flex justify-end">
            <Button variant="outline" size="sm" @click="showImport = false; importStatus = ''">关闭</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
