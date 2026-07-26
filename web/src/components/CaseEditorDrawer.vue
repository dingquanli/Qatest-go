<script setup lang="ts">
/**
 * CaseEditorDrawer —— 表格 / 思维导图用例通用「抽屉式」编辑器。
 * 参考 TestRail / 禅道：右侧滑出抽屉 + 结构化「操作步骤」子表（序号 / 操作步骤 / 预期结果）。
 * 步骤以 JSON 字符串写入 case.steps（后端字段为 string），兼容旧的纯文本换行格式。
 */
import { ref, computed, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Trash2, Save } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'

interface ModuleLite { id: string; name: string }
interface StepRow { action: string; expected: string }
interface CaseLike {
  id?: string
  name?: string
  moduleId?: string
  priority?: string
  type?: string
  precondition?: string
  steps?: string
  assignee?: string
  status?: string
  tags?: string
}

const props = defineProps<{
  visible: boolean
  modules: ModuleLite[]
  editing: CaseLike | null
  defaultModuleId?: string
  title?: string
  /** 父组件提供的保存逻辑：返回 Promise，抛错即视为失败 */
  submit: (payload: Record<string, unknown>, isEdit: boolean) => Promise<void>
}>()

const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'saved'): void
}>()

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

const PRIORITY_OPTIONS = [
  { value: 'P0', label: 'P0 - 紧急' },
  { value: 'P1', label: 'P1 - 高' },
  { value: 'P2', label: 'P2 - 中' },
  { value: 'P3', label: 'P3 - 低' },
]

const form = ref({
  name: '',
  moduleId: '',
  priority: 'P2',
  type: 'functional',
  precondition: '',
  assignee: '',
  status: 'draft',
  tags: '',
})
const steps = ref<StepRow[]>([{ action: '', expected: '' }])
const saving = ref(false)
const nameInput = ref<InstanceType<typeof Input> | null>(null)

const isEdit = computed(() => !!props.editing?.id)
const drawerTitle = computed(() => {
  const base = props.title || '用例'
  return isEdit.value ? `编辑${base}` : `新建${base}`
})

/** 解析 steps 字符串：优先按 JSON 数组解析，回退到纯文本按行拆分 */
function parseSteps(raw: string | undefined): StepRow[] {
  if (!raw || !raw.trim()) return [{ action: '', expected: '' }]
  const s = raw.trim()
  if (s.startsWith('[')) {
    try {
      const arr = JSON.parse(s)
      if (Array.isArray(arr) && arr.length) {
        return arr.map((x: Record<string, string>) => ({
          action: x.action ?? x.step ?? '',
          expected: x.expected ?? '',
        }))
      }
    } catch {
      /* 落到纯文本分支 */
    }
  }
  const rows = s.split('\n').map((l) => l.trim()).filter(Boolean).map((l) => ({ action: l, expected: '' }))
  return rows.length ? rows : [{ action: '', expected: '' }]
}

/** 解析 tags：JSON 数组 -> 逗号串；否则原样 */
function parseTags(raw: string | undefined): string {
  if (!raw || !raw.trim()) return ''
  const s = raw.trim()
  if (s.startsWith('[')) {
    try {
      const arr = JSON.parse(s)
      if (Array.isArray(arr)) return arr.join(', ')
    } catch {
      /* ignore */
    }
  }
  return s
}

function resetForm(keepModule = false): void {
  const mod = keepModule ? form.value.moduleId : (props.defaultModuleId || '')
  form.value = {
    name: '',
    moduleId: mod,
    priority: 'P2',
    type: 'functional',
    precondition: '',
    assignee: '',
    status: 'draft',
    tags: '',
  }
  steps.value = [{ action: '', expected: '' }]
}

function initFromEditing(): void {
  const c = props.editing
  if (c && c.id) {
    form.value = {
      name: c.name || '',
      moduleId: c.moduleId || '',
      priority: c.priority || 'P2',
      type: c.type || 'functional',
      precondition: c.precondition || '',
      assignee: c.assignee || '',
      status: c.status || 'draft',
      tags: parseTags(c.tags),
    }
    steps.value = parseSteps(c.steps)
  } else {
    resetForm()
  }
}

watch(
  () => props.visible,
  (open) => {
    if (open) {
      initFromEditing()
      nextTick(() => nameInput.value?.$el?.querySelector?.('input')?.focus())
    }
  },
)

function addStep(): void {
  steps.value.push({ action: '', expected: '' })
}
function removeStep(i: number): void {
  steps.value.splice(i, 1)
  if (steps.value.length === 0) steps.value.push({ action: '', expected: '' })
}
function moveStep(i: number, dir: -1 | 1): void {
  const j = i + dir
  if (j < 0 || j >= steps.value.length) return
  const arr = steps.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

function buildPayload(): Record<string, unknown> | null {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入用例名称')
    return null
  }
  const cleanSteps = steps.value
    .map((s) => ({ action: s.action.trim(), expected: s.expected.trim() }))
    .filter((s) => s.action || s.expected)
  if (cleanSteps.length === 0) {
    ElMessage.warning('请至少填写一条操作步骤')
    return null
  }
  const tagsArr = form.value.tags
    .split(/[,，\s]+/)
    .map((t) => t.trim())
    .filter(Boolean)
  return {
    name: form.value.name.trim(),
    moduleId: form.value.moduleId || '',
    priority: form.value.priority,
    type: form.value.type,
    precondition: form.value.precondition,
    steps: JSON.stringify(cleanSteps),
    assignee: form.value.assignee,
    status: form.value.status,
    tags: JSON.stringify(tagsArr),
  }
}

async function handleSave(continueCreate = false): Promise<void> {
  const payload = buildPayload()
  if (!payload) return
  saving.value = true
  try {
    await props.submit(payload, isEdit.value)
    emit('saved')
    ElMessage.success(isEdit.value ? '用例已更新' : '用例已创建')
    if (continueCreate && !isEdit.value) {
      resetForm(true)
      nextTick(() => nameInput.value?.$el?.querySelector?.('input')?.focus())
    } else {
      emit('update:visible', false)
    }
  } catch (e: unknown) {
    ElMessage.error((e as Error)?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function close(): void {
  emit('update:visible', false)
}
</script>

<template>
  <el-drawer
    :model-value="visible"
    :title="drawerTitle"
    direction="rtl"
    size="560px"
    :append-to-body="true"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
  >
    <div class="flex flex-col gap-4 pb-24">
      <!-- 基本信息 -->
      <div>
        <label class="text-xs font-medium mb-1 block text-muted-foreground">用例名称 <span class="text-destructive">*</span></label>
        <Input ref="nameInput" v-model="form.name" placeholder="请输入用例名称，如：登录成功跳转首页" />
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">所属模块</label>
          <select v-model="form.moduleId" class="w-full h-9 rounded-xl border border-input bg-transparent px-3 text-sm outline-none focus:ring-2 focus:ring-ring">
            <option value="">未分类</option>
            <option v-for="m in modules" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">优先级</label>
          <select v-model="form.priority" class="w-full h-9 rounded-xl border border-input bg-transparent px-3 text-sm outline-none focus:ring-2 focus:ring-ring">
            <option v-for="p in PRIORITY_OPTIONS" :key="p.value" :value="p.value">{{ p.label }}</option>
          </select>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">用例类型</label>
          <select v-model="form.type" class="w-full h-9 rounded-xl border border-input bg-transparent px-3 text-sm outline-none focus:ring-2 focus:ring-ring">
            <option v-for="t in TYPE_OPTIONS" :key="t.value" :value="t.value">{{ t.label }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">状态</label>
          <select v-model="form.status" class="w-full h-9 rounded-xl border border-input bg-transparent px-3 text-sm outline-none focus:ring-2 focus:ring-ring">
            <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">负责人</label>
          <Input v-model="form.assignee" placeholder="选填" />
        </div>
        <div>
          <label class="text-xs font-medium mb-1 block text-muted-foreground">标签</label>
          <Input v-model="form.tags" placeholder="逗号分隔，如：核心,回归" />
        </div>
      </div>

      <div>
        <label class="text-xs font-medium mb-1 block text-muted-foreground">前置条件</label>
        <textarea v-model="form.precondition" rows="2" placeholder="执行本用例前需要满足的条件" class="w-full rounded-xl border border-input bg-transparent px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring resize-y" />
      </div>

      <!-- 结构化操作步骤 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <label class="text-xs font-medium text-muted-foreground">操作步骤 <span class="text-destructive">*</span></label>
          <Button size="sm" variant="outline" class="h-7 rounded-lg text-xs gap-1" @click="addStep">
            <Plus class="w-3 h-3" /> 添加步骤
          </Button>
        </div>
        <div class="border rounded-xl overflow-hidden">
          <div class="grid grid-cols-[36px_1fr_1fr_32px] bg-muted/50 text-[11px] text-muted-foreground font-medium">
            <div class="px-2 py-1.5 text-center border-r">#</div>
            <div class="px-2 py-1.5 border-r">操作步骤</div>
            <div class="px-2 py-1.5 border-r">预期结果</div>
            <div class="px-1 py-1.5"></div>
          </div>
          <div
            v-for="(s, i) in steps"
            :key="i"
            class="grid grid-cols-[36px_1fr_1fr_32px] border-t items-stretch group"
          >
            <div class="flex flex-col items-center justify-center gap-0.5 border-r bg-muted/20 text-xs text-muted-foreground select-none">
              <button class="opacity-0 group-hover:opacity-100 leading-none hover:text-foreground" title="上移" @click="moveStep(i, -1)">▲</button>
              <span class="font-medium">{{ i + 1 }}</span>
              <button class="opacity-0 group-hover:opacity-100 leading-none hover:text-foreground" title="下移" @click="moveStep(i, 1)">▼</button>
            </div>
            <div class="border-r p-1">
              <textarea v-model="s.action" rows="2" placeholder="描述操作..." class="w-full h-full min-h-[42px] bg-transparent px-1.5 py-1 text-xs outline-none resize-none" />
            </div>
            <div class="border-r p-1">
              <textarea v-model="s.expected" rows="2" placeholder="描述预期..." class="w-full h-full min-h-[42px] bg-transparent px-1.5 py-1 text-xs outline-none resize-none" />
            </div>
            <div class="flex items-center justify-center">
              <button class="text-muted-foreground hover:text-destructive p-1" title="删除步骤" @click="removeStep(i)">
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
        <p class="text-[11px] text-muted-foreground mt-1.5">共 {{ steps.length }} 步 · 可拖拽调整顺序（悬停行显示 ▲▼）</p>
      </div>
    </div>

    <!-- 底部操作栏 -->
    <template #footer>
      <div class="flex items-center justify-end gap-2 w-full">
        <Button variant="outline" size="sm" :disabled="saving" @click="close">取消</Button>
        <Button v-if="!isEdit" variant="outline" size="sm" :disabled="saving" class="gap-1" @click="handleSave(true)">
          <Plus class="w-3.5 h-3.5" /> 保存并继续新建
        </Button>
        <Button size="sm" :disabled="saving" class="gap-1" @click="handleSave(false)">
          <Save class="w-3.5 h-3.5" /> {{ saving ? '保存中...' : '保存' }}
        </Button>
      </div>
    </template>
  </el-drawer>
</template>
