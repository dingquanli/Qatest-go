<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Smartphone, Monitor, Plus, Play, Trash2, Edit3, Save,
  XCircle, CheckCircle2, AlertTriangle, Terminal, RotateCcw,
  ArrowLeft, Camera, Code, PlayCircle, Square,
  HardDrive, Bug, Search, Usb,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Textarea from '@/components/ui/Textarea.vue'
import Dialog from '@/components/ui/Dialog.vue'
import DialogHeader from '@/components/ui/DialogHeader.vue'
import DialogTitle from '@/components/ui/DialogTitle.vue'
import DialogDescription from '@/components/ui/DialogDescription.vue'
import DialogFooter from '@/components/ui/DialogFooter.vue'
import { cn } from '@/lib/utils'
import { safeParseJSON, formatDate } from '@/utils'
import * as devicesApi from '@/api/devices'
import * as scriptsApi from '@/api/scripts'
import * as execApi from '@/api/executions'
import * as bugsApi from '@/api/bugs'
import { useWebSocket } from '@/composables/useWebSocket'
import { useUserStore } from '@/stores/user'
import type { DeviceInfo, Script, Execution, ScriptLanguage } from '@/types'

type LogEntry = { timestamp: string; level: string; message: string }
type View = 'devices' | 'scripts' | 'editor' | 'running'

const userStore = useUserStore()
const { messages: wsMessages, connect: connectWs, close: closeWs } = useWebSocket(() =>
  userStore.getToken ? userStore.getToken() : '',
)

const view = ref<View>('devices')
const devices = ref<DeviceInfo[]>([])
const scripts = ref<Script[]>([])
const executions = ref<Execution[]>([])
const editingScript = ref<Script | null>(null)
const isCreating = ref(false)
const runningExecId = ref<string | null>(null)
const liveLogs = ref<LogEntry[]>([])
const selectedDevice = ref<string | null>(null)
const screenshot = ref<string | null>(null)
const shellCmd = ref('')
const shellOutput = ref('')
const bugModalScript = ref<Script | null>(null)
const bugModalOpen = ref(false)
const deviceSearch = ref('')
const deviceStatusFilter = ref('all')
const logsEndRef = ref<HTMLElement | null>(null)

const typeColors: Record<string, string> = {
  available: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  busy: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  disconnected: 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400',
  error: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
  android: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  ios: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
}

const langLabels: Record<ScriptLanguage, string> = { python: 'Python', javascript: 'JavaScript', shell: 'Shell' }

const DEFAULT_PYTHON_SCRIPT = `import subprocess, os

serial = os.environ.get('ADB_SERIAL', '')
def adb(cmd):
    return subprocess.run(
        f'adb -s {serial} {cmd}' if serial else f'adb {cmd}',
        shell=True, capture_output=True, text=True
)

print("=== 开始执行 ===")
# 在此编写你的自动化脚本
# print(adb('shell input tap 500 1000').stdout.strip())
print("=== 执行完成 ===")
`

const DEFAULT_JS_SCRIPT = `// 可用全局函数:
// adb(args)    - 执行 ADB 命令
// log(msg)     - 输出日志
// sleep(ms)    - 等待

log('=== 开始执行 ===');
// 在此编写你的自动化脚本
// adb('shell input tap 500 1000');
log('=== 执行完成 ===');
`

const DEFAULT_SHELL_SCRIPT = `# ADB Shell 脚本
# 每行命令会在设备上依次执行

echo "=== 开始执行 ==="
# 在此编写你的 ADB 命令
# input tap 500 1000
echo "=== 执行完成 ==="
`

function getDefaultCode(lang: ScriptLanguage): string {
  if (lang === 'python') return DEFAULT_PYTHON_SCRIPT
  if (lang === 'javascript') return DEFAULT_JS_SCRIPT
  return DEFAULT_SHELL_SCRIPT
}

const availableDevices = computed(() => devices.value.filter((d) => d.state === 'available'))
const busyDevices = computed(() => devices.value.filter((d) => d.state === 'busy'))
const runningExec = computed(() => executions.value.find((e) => e.id === runningExecId.value) || null)
const filteredDevices = computed(() =>
  devices.value.filter((d) => {
    if (deviceStatusFilter.value !== 'all' && d.state !== deviceStatusFilter.value) return false
    if (deviceSearch.value) {
      const s = deviceSearch.value.toLowerCase()
      return (
        (d.model || '').toLowerCase().includes(s) ||
        (d.serial || '').toLowerCase().includes(s) ||
        (d.manufacturer || '').toLowerCase().includes(s)
      )
    }
    return true
  }),
)

function deviceStatusLabel(s: string): string {
  if (s === 'available') return '可用'
  if (s === 'busy') return '忙碌'
  if (s === 'disconnected') return '离线'
  if (s === 'error') return '异常'
  return s || '未知'
}
function deviceStatusClass(s: string): string {
  return typeColors[s] || 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'
}

// ==================== Data Loading ====================
const refreshDevices = async () => {
  try {
    devices.value = await devicesApi.scanDevices()
  } catch {
    devices.value = await devicesApi.getDevices().catch(() => [])
  }
}
const refreshScripts = async () => {
  scripts.value = await scriptsApi.getScripts()
}
const refreshExecs = async () => {
  executions.value = await execApi.getExecutions()
}

onMounted(() => {
  refreshDevices().catch(() => {})
  refreshScripts().catch(() => {})
  refreshExecs().catch(() => {})
  connectWs()
})

watch(liveLogs, () => {
  nextTick(() => logsEndRef.value?.scrollIntoView({ behavior: 'smooth' }))
})

// Map incoming WebSocket messages into live logs (best-effort, both structured & raw)
let lastWsLen = 0
watch(
  wsMessages,
  (arr) => {
    for (let i = lastWsLen; i < arr.length; i++) {
      const raw = arr[i]
      try {
        const m = JSON.parse(raw)
        if (m.entry) liveLogs.value.push(m.entry)
        else if (m.message)
          liveLogs.value.push({ timestamp: m.timestamp || new Date().toISOString(), level: m.level || 'info', message: m.message })
        else liveLogs.value.push({ timestamp: new Date().toISOString(), level: 'info', message: raw })
      } catch {
        liveLogs.value.push({ timestamp: new Date().toISOString(), level: 'info', message: raw })
      }
    }
    lastWsLen = arr.length
  },
)

// ==================== Device Panel ====================
const takeScreenshot = async (serial: string) => {
  try {
    const res = await devicesApi.takeScreenshot(serial)
    if (res.path) screenshot.value = res.path
  } catch (err) {
    ElMessage.error((err as Error).message || '截图失败')
  }
}

const execShell = async () => {
  if (!selectedDevice.value || !shellCmd.value.trim()) return
  try {
    const res = await devicesApi.execDeviceCommand(selectedDevice.value, shellCmd.value.trim())
    const line = `$ ${shellCmd.value}\n${res.output || '(无输出)'}`
    shellOutput.value = shellOutput.value ? shellOutput.value + '\n' + line : line
    shellCmd.value = ''
  } catch (err) {
    ElMessage.error((err as Error).message || '执行失败')
  }
}

const copySerial = async (serial: string) => {
  try {
    await navigator.clipboard.writeText(serial)
    ElMessage.success('已复制设备序列号')
  } catch {
    ElMessage.success('已复制设备序列号')
  }
}

// ==================== Script Editor ====================
const startCreate = () => {
  editingScript.value = {
    id: '', name: '', description: '', language: 'python', code: getDefaultCode('python'),
    createdAt: '', updatedAt: '',
  }
  isCreating.value = true
  view.value = 'editor'
}

const startEdit = (s: Script) => {
  editingScript.value = { ...s }
  isCreating.value = false
  view.value = 'editor'
}

const saveScript = async () => {
  if (!editingScript.value || !editingScript.value.name.trim()) {
    ElMessage.warning('请输入脚本名称')
    return
  }
  try {
    if (isCreating.value) {
      await scriptsApi.createScript(editingScript.value)
    } else {
      await scriptsApi.updateScript(editingScript.value.id, editingScript.value)
    }
    await refreshScripts()
    view.value = 'scripts'
  } catch (err) {
    ElMessage.error((err as Error).message || '保存失败')
  }
}

const deleteScript = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定删除此脚本？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await scriptsApi.deleteScript(id)
  await refreshScripts()
}

const changeLang = (e: Event) => {
  if (!editingScript.value) return
  const newLang = (e.target as HTMLSelectElement).value as ScriptLanguage
  if (editingScript.value.code.trim() && editingScript.value.language !== newLang) {
    ElMessageBox.confirm('切换语言将覆盖当前代码，确定继续？', '提示', { type: 'warning' })
      .then(() => {
        editingScript.value = { ...editingScript.value!, language: newLang, code: getDefaultCode(newLang) }
      })
      .catch(() => {})
  } else {
    editingScript.value = { ...editingScript.value, language: newLang, code: getDefaultCode(newLang) }
  }
}

// ==================== Execution ====================
const runScript = async (script: Script, deviceSerial: string) => {
  try {
    const res = await execApi.createExecution({ scriptId: script.id, deviceSerial, taskName: script.name })
    liveLogs.value = []
    runningExecId.value = res.id
    view.value = 'running'
    await refreshExecs()
  } catch (err) {
    ElMessage.error('执行创建失败：' + ((err as Error).message || '未知错误'))
  }
}

const cancelExecution = async (id: string) => {
  await execApi.cancelExecution(id)
  await refreshExecs()
}

const selectExecution = (ex: Execution) => {
  runningExecId.value = ex.id
  liveLogs.value = safeParseJSON<LogEntry[]>(ex.logs, [])
}

// ==================== Bug Modal ====================
const bugForm = ref({ title: '', description: '', steps: '', expected: '' })
const openBug = (s: Script) => {
  bugModalScript.value = s
  bugForm.value = {
    title: `[自动化] ${s.name}`,
    description: s.description || `自动化脚本: ${s.name}`,
    steps: '',
    expected: '',
  }
  bugModalOpen.value = true
}
const submitBug = async () => {
  if (!bugForm.value.title.trim()) {
    ElMessage.warning('请输入缺陷标题')
    return
  }
  await bugsApi.createBug({
    title: bugForm.value.title,
    description: bugForm.value.description,
    steps: bugForm.value.steps,
    expected: bugForm.value.expected,
    severity: 'major',
    priority: 'P2',
    status: 'open',
    reporter: '当前用户',
    tags: '[]',
  })
  ElMessage.success('已提交缺陷')
  bugModalOpen.value = false
  bugModalScript.value = null
}

onUnmounted(closeWs)
</script>

<template>
  <div class="space-y-4">
    <!-- Page Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">自动化平台</h1>
        <p class="text-muted-foreground mt-0.5">设备管理、脚本编写、远程执行</p>
      </div>
      <div class="flex gap-2">
        <Button :variant="view === 'devices' ? 'default' : 'outline'" size="sm" class="gap-2" @click="view = 'devices'">
          <Smartphone class="w-4 h-4" /> 设备
        </Button>
        <Button :variant="view === 'scripts' ? 'default' : 'outline'" size="sm" class="gap-2" @click="view = 'scripts'; refreshScripts()">
          <Code class="w-4 h-4" /> 脚本
        </Button>
        <Button :variant="view === 'running' ? 'default' : 'outline'" size="sm" class="gap-2" @click="view = 'running'; refreshExecs()">
          <PlayCircle class="w-4 h-4" /> 执行
        </Button>
      </div>
    </div>

    <!-- ==================== DEVICES VIEW ==================== -->
    <div v-if="view === 'devices'" class="grid gap-6 lg:grid-cols-3">
      <!-- Device List -->
      <div class="lg:col-span-1 space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-semibold flex items-center gap-2">
            <Smartphone class="w-4 h-4" /> 设备列表 ({{ filteredDevices.length }}/{{ devices.length }})
          </h2>
          <Button variant="ghost" size="sm" class="h-7 text-xs" @click="refreshDevices">
            <RotateCcw class="w-3 h-3 mr-1" /> 刷新
          </Button>
        </div>

        <!-- Search -->
        <div class="relative">
          <Search class="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
          <Input v-model="deviceSearch" placeholder="搜索设备型号/序列号/品牌..." class="h-8 pl-7 text-xs" />
        </div>

        <!-- Status filter -->
        <div class="flex gap-1">
          <button
            v-for="f in [{ key: 'all', label: '全部' }, { key: 'available', label: '可用' }, { key: 'busy', label: '忙碌' }, { key: 'disconnected', label: '离线' }]"
            :key="f.key"
            :class="cn(
              'px-2.5 py-1 rounded text-xs font-medium transition-colors',
              deviceStatusFilter === f.key ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-accent',
            )"
            @click="deviceStatusFilter = f.key"
          >
            {{ f.label }}
          </button>
        </div>

        <Card v-if="devices.length === 0">
          <CardContent class="p-8 text-center text-muted-foreground">
            <Smartphone class="w-10 h-10 mx-auto mb-2 opacity-30" />
            <p class="text-sm">未检测到设备</p>
            <p class="text-xs mt-1">请通过 USB 连接手机并开启 ADB 调试</p>
          </CardContent>
        </Card>
        <Card
          v-for="d in filteredDevices"
          :key="d.serial"
          :class="cn(
            'cursor-pointer transition-all hover:shadow-sm',
            selectedDevice === d.serial ? 'ring-2 ring-primary border-primary' : '',
          )"
          @click="selectedDevice = d.serial; screenshot = null; shellOutput = ''"
        >
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div class="flex items-center gap-3">
                <div :class="cn('p-2 rounded-lg', d.state === 'available' ? 'bg-emerald-50 dark:bg-emerald-900/20' : 'bg-gray-50 dark:bg-gray-900/20')">
                  <Monitor :class="cn('w-5 h-5', d.state === 'available' ? 'text-emerald-600' : 'text-gray-400')" />
                </div>
                <div>
                  <p class="text-sm font-medium">{{ d.model }}</p>
                  <p class="text-xs text-muted-foreground">{{ d.serial }}</p>
                </div>
              </div>
              <Badge :class="cn('text-xs', deviceStatusClass(d.state))">
                {{ deviceStatusLabel(d.state) }}
              </Badge>
            </div>
            <div class="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
              <span>Android {{ d.androidVer }}</span>
              <span v-if="d.resolution">{{ d.resolution }}</span>
              <span v-if="Number(d.battery) > 0">🔋 {{ d.battery }}%</span>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Device Detail -->
      <div class="lg:col-span-2">
        <template v-if="selectedDevice">
          <div v-if="!devices.find((x) => x.serial === selectedDevice)" class="text-center text-muted-foreground py-8">设备未找到</div>
          <div v-else class="space-y-4">
            <Card v-for="d in devices.filter((x) => x.serial === selectedDevice)" :key="d.serial">
              <CardHeader class="pb-2">
                <div class="flex items-center justify-between">
                  <CardTitle class="text-base">{{ d.model }}</CardTitle>
                  <div class="flex gap-2">
                    <Button variant="outline" size="sm" class="gap-1 h-7 text-xs" @click="takeScreenshot(d.serial)">
                      <Camera class="w-3 h-3" /> 截图
                    </Button>
                    <Button variant="outline" size="sm" class="gap-1 h-7 text-xs" @click="copySerial(d.serial)">
                      <HardDrive class="w-3 h-3" /> 复制 SN
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div class="grid grid-cols-2 gap-3 text-xs text-muted-foreground">
                  <div>序列号: <span class="text-foreground font-mono">{{ d.serial }}</span></div>
                  <div>型号: <span class="text-foreground">{{ d.model }}</span></div>
                  <div>制造商: <span class="text-foreground">{{ d.manufacturer }}</span></div>
                  <div>系统版本: <span class="text-foreground">Android {{ d.androidVer }} (SDK {{ d.sdkVersion }})</span></div>
                  <div>分辨率: <span class="text-foreground">{{ d.resolution || '未知' }}</span></div>
                  <div>电量: <span class="text-foreground">{{ Number(d.battery) > 0 ? d.battery + '%' : '未知' }}</span></div>
                  <div>状态: <Badge :class="cn('text-xs', deviceStatusClass(d.state))">{{ d.state }}</Badge></div>
                  <div>连接方式: <Badge variant="outline" class="text-xs">未识别</Badge></div>
                </div>
              </CardContent>
            </Card>

            <!-- Screenshot -->
            <Card v-if="screenshot">
              <CardHeader class="pb-2">
                <CardTitle class="text-sm flex items-center gap-2">
                  <Camera class="w-3.5 h-3.5" /> 设备截图
                </CardTitle>
              </CardHeader>
              <CardContent>
                <img :src="screenshot" alt="设备截图" class="w-full max-w-sm rounded-lg border" />
              </CardContent>
            </Card>

            <!-- Shell Console -->
            <Card>
              <CardHeader class="pb-2">
                <div class="flex items-center justify-between">
                  <CardTitle class="text-sm flex items-center gap-2">
                    <Terminal class="w-3.5 h-3.5" /> ADB Shell
                  </CardTitle>
                  <Button v-if="shellOutput" variant="ghost" size="sm" class="h-6 text-xs" @click="shellOutput = ''">
                    清空
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <div class="flex gap-2 mb-2">
                  <Input
                    v-model="shellCmd"
                    placeholder="输入 ADB shell 命令，如 input tap 500 1000"
                    class="font-mono text-sm h-9"
                    @keyup.enter="execShell"
                  />
                  <Button size="sm" class="h-9 gap-1" @click="execShell">
                    <Play class="w-3 h-3" /> 执行
                  </Button>
                </div>
                <div class="bg-muted/80 text-foreground rounded-lg p-3 font-mono text-xs max-h-64 overflow-auto whitespace-pre-wrap border">
                  <span v-if="!shellOutput" class="text-muted-foreground">输入命令后点击执行</span>
                  <template v-else>{{ shellOutput }}</template>
                </div>
              </CardContent>
            </Card>
          </div>
        </template>
        <div v-else class="flex items-center justify-center h-96 text-muted-foreground">
          <div class="text-center">
            <Smartphone class="w-16 h-16 mx-auto mb-3 opacity-20" />
            <p class="text-sm">从左侧选择设备</p>
            <p class="text-xs mt-1">支持 USB / 远程设备柜连接</p>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== SCRIPTS VIEW ==================== -->
    <div v-if="view === 'scripts'" class="grid gap-6 lg:grid-cols-3">
      <!-- Script List -->
      <div class="lg:col-span-1 space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-semibold flex items-center gap-2">
            <Code class="w-4 h-4" /> 脚本列表 ({{ scripts.length }})
          </h2>
          <Button variant="default" size="sm" class="h-7 text-xs gap-1" @click="startCreate">
            <Plus class="w-3 h-3" /> 新建
          </Button>
        </div>
        <Card v-if="scripts.length === 0">
          <CardContent class="p-8 text-center text-muted-foreground">
            <Code class="w-10 h-10 mx-auto mb-2 opacity-30" />
            <p class="text-sm">暂无脚本</p>
            <Button variant="link" size="sm" @click="startCreate">创建第一个脚本</Button>
          </CardContent>
        </Card>
        <Card
          v-for="s in scripts"
          :key="s.id"
          class="cursor-pointer hover:shadow-sm transition-all"
          @click="startEdit(s)"
        >
          <CardContent class="p-4">
            <div class="flex items-start justify-between">
              <div>
                <p class="text-sm font-medium">{{ s.name || '未命名脚本' }}</p>
                <p class="text-xs text-muted-foreground mt-0.5">{{ s.description || '无描述' }}</p>
              </div>
              <Badge variant="outline" class="text-xs">{{ langLabels[s.language] || '未知' }}</Badge>
            </div>
            <div class="flex items-center gap-2 mt-2 text-xs text-muted-foreground">
              <span>{{ (s.code || '').length }} 字符</span>
              <span>·</span>
              <span>{{ s.updatedAt ? formatDate(s.updatedAt, 'YYYY-MM-DD') : '' }}</span>
            </div>
            <div class="flex gap-1 mt-2">
              <Button
                v-for="d in availableDevices"
                :key="d.serial"
                variant="ghost"
                size="sm"
                class="h-6 text-xs gap-1"
                @click.stop="runScript(s, d.serial)"
              >
                <Play class="w-2.5 h-2.5" /> {{ d.model.split(' ')[0] }}
              </Button>
              <span v-if="availableDevices.length === 0" class="text-xs text-muted-foreground">无可用设备</span>
            </div>
            <div class="flex gap-1 mt-2 pt-2 border-t">
              <Button variant="ghost" size="icon" class="h-6 w-6 text-destructive/70 hover:text-destructive" @click.stop="openBug(s)">
                <Bug class="w-3 h-3" />
              </Button>
              <Button variant="ghost" size="icon" class="h-7 w-7" @click.stop="startEdit(s)">
                <Edit3 class="w-3 h-3" />
              </Button>
              <Button variant="ghost" size="icon" class="h-7 w-7" @click.stop="deleteScript(s.id)">
                <Trash2 class="w-3 h-3 text-red-500" />
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Quick Start Guide -->
      <div class="lg:col-span-2">
        <Card>
          <CardHeader>
            <CardTitle class="text-base">快速开始</CardTitle>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid gap-4 md:grid-cols-3">
              <div class="p-4 rounded-lg border bg-muted/30 text-center">
                <div class="p-2 rounded-full bg-emerald-50 dark:bg-emerald-900/20 w-10 h-10 flex items-center justify-center mx-auto mb-2">
                  <Usb class="w-5 h-5 text-emerald-600" />
                </div>
                <h3 class="text-sm font-medium">1. 连接设备</h3>
                <p class="text-xs text-muted-foreground mt-1">USB 连接手机或接入设备柜</p>
              </div>
              <div class="p-4 rounded-lg border bg-muted/30 text-center">
                <div class="p-2 rounded-full bg-blue-50 dark:bg-blue-900/20 w-10 h-10 flex items-center justify-center mx-auto mb-2">
                  <Code class="w-5 h-5 text-blue-600" />
                </div>
                <h3 class="text-sm font-medium">2. 编写脚本</h3>
                <p class="text-xs text-muted-foreground mt-1">支持 Python / JavaScript / Shell</p>
              </div>
              <div class="p-4 rounded-lg border bg-muted/30 text-center">
                <div class="p-2 rounded-full bg-violet-50 dark:bg-violet-900/20 w-10 h-10 flex items-center justify-center mx-auto mb-2">
                  <PlayCircle class="w-5 h-5 text-violet-600" />
                </div>
                <h3 class="text-sm font-medium">3. 远程执行</h3>
                <p class="text-xs text-muted-foreground mt-1">选择设备执行，实时查看日志</p>
              </div>
            </div>
            <div class="p-4 rounded-lg bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-900/30">
              <p class="text-sm font-medium text-amber-700 dark:text-amber-400">前置条件</p>
              <ul class="text-xs text-amber-600 dark:text-amber-300 mt-1 space-y-1">
                <li>• 电脑安装 ADB (Android Debug Bridge) 并配置到系统 PATH</li>
                <li>• 手机开启「开发者选项」和「USB 调试」</li>
                <li>• USB 连接手机后允许调试授权</li>
                <li>• 远程设备柜需确保 ADB 可连接 (adb connect IP:PORT)</li>
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- ==================== EDITOR VIEW ==================== -->
    <div v-if="view === 'editor' && editingScript" class="space-y-4">
      <div class="flex items-center gap-3">
        <Button variant="ghost" size="icon" @click="view = 'scripts'">
          <ArrowLeft class="w-4 h-4" />
        </Button>
        <div class="flex-1">
          <Input
            v-model="editingScript.name"
            placeholder="脚本名称"
            class="h-9 text-base font-bold border-dashed max-w-md"
          />
        </div>
        <select
          :value="editingScript.language"
          class="h-9 rounded-md border border-input bg-background px-3 text-sm"
          @change="changeLang"
        >
          <option v-for="(v, k) in langLabels" :key="k" :value="k">{{ v }}</option>
        </select>
        <Button class="gap-2" @click="saveScript">
          <Save class="w-4 h-4" /> 保存脚本
        </Button>
      </div>
      <div class="flex gap-4">
        <div class="flex-1 space-y-2">
          <div class="text-xs text-muted-foreground">代码编辑器</div>
          <div class="rounded-lg border overflow-hidden" style="height: calc(100vh - 280px)">
            <Textarea
              v-model="editingScript.code"
              placeholder="在此编写自动化脚本..."
              class="h-full w-full font-mono text-sm resize-none rounded-none border-0"
              style="height: 100%"
            />
          </div>
        </div>
        <div class="w-72 shrink-0 space-y-2">
          <div class="text-xs text-muted-foreground">描述</div>
          <Textarea v-model="editingScript.description" placeholder="脚本用途说明..." class="h-24 resize-none" />
          <div class="text-xs text-muted-foreground mt-4">可用设备</div>
          <div class="space-y-1.5">
            <button
              v-for="d in availableDevices"
              :key="d.serial"
              class="flex items-center gap-2 w-full p-2 rounded-lg border hover:bg-accent/50 transition-colors text-left"
              @click="runScript(editingScript, d.serial)"
            >
              <Monitor class="w-4 h-4 text-emerald-500" />
              <div class="flex-1 min-w-0">
                <p class="text-xs font-medium truncate">{{ d.model }}</p>
                <p class="text-xs text-muted-foreground truncate">{{ d.serial }}</p>
              </div>
              <Play class="w-3 h-3 text-muted-foreground shrink-0" />
            </button>
            <p v-if="availableDevices.length === 0" class="text-xs text-muted-foreground text-center py-4">无可用设备，请先连接</p>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== EXECUTION VIEW ==================== -->
    <div v-if="view === 'running'" class="grid gap-6 lg:grid-cols-3">
      <!-- Execution List -->
      <div class="lg:col-span-1 space-y-3">
        <h2 class="text-sm font-semibold flex items-center gap-2">
          <PlayCircle class="w-4 h-4" /> 执行记录
        </h2>
        <Card v-if="executions.length === 0">
          <CardContent class="p-8 text-center text-muted-foreground">
            <PlayCircle class="w-10 h-10 mx-auto mb-2 opacity-30" />
            <p class="text-sm">暂无执行记录</p>
          </CardContent>
        </Card>
        <Card
          v-for="ex in executions.slice(0, 30)"
          :key="ex.id"
          :class="cn(
            'cursor-pointer transition-all',
            runningExecId === ex.id ? 'ring-2 ring-primary' : 'hover:shadow-sm',
          )"
          @click="selectExecution(ex)"
        >
          <CardContent class="p-3">
            <div class="flex items-center gap-2">
              <RotateCcw v-if="ex.status === 'running'" class="w-4 h-4 text-blue-500 animate-spin" />
              <CheckCircle2 v-else-if="ex.status === 'success'" class="w-4 h-4 text-emerald-500" />
              <XCircle v-else-if="ex.status === 'failed'" class="w-4 h-4 text-red-500" />
              <AlertTriangle v-else class="w-4 h-4 text-amber-500" />
              <div class="flex-1 min-w-0">
                <p class="text-xs font-medium truncate">{{ ex.taskName }}</p>
                <p class="text-xs text-muted-foreground truncate">{{ ex.deviceSerial }}</p>
              </div>
              <div class="text-right shrink-0">
                <Badge
                  :variant="ex.status === 'success' ? 'success' : ex.status === 'failed' ? 'destructive' : 'secondary'"
                  class="text-xs"
                >
                  {{ ex.status === 'running' ? '运行中' : ex.status === 'success' ? '成功' : ex.status === 'failed' ? '失败' : '已取消' }}
                </Badge>
                <p v-if="ex.duration > 0" class="text-xs text-muted-foreground mt-0.5">{{ ex.duration }}s</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Running Detail -->
      <div class="lg:col-span-2 space-y-4">
        <template v-if="runningExec">
          <Card>
            <CardHeader class="pb-2">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <RotateCcw v-if="runningExec.status === 'running'" class="w-5 h-5 text-blue-500 animate-spin" />
                  <CheckCircle2 v-else-if="runningExec.status === 'success'" class="w-5 h-5 text-emerald-500" />
                  <XCircle v-else class="w-5 h-5 text-red-500" />
                  <div>
                    <CardTitle class="text-base">{{ runningExec.taskName }}</CardTitle>
                    <p class="text-xs text-muted-foreground">设备: {{ runningExec.deviceSerial }}</p>
                  </div>
                </div>
                <Button v-if="runningExec.status === 'running'" variant="destructive" size="sm" class="gap-1" @click="cancelExecution(runningExec.id)">
                  <Square class="w-3 h-3" /> 停止
                </Button>
              </div>
            </CardHeader>
          </Card>

          <!-- Live Logs -->
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm flex items-center gap-2">
                <Terminal class="w-3.5 h-3.5" /> 执行日志
                <span v-if="runningExec.status === 'running'" class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div class="bg-muted/80 text-foreground rounded-lg p-4 font-mono text-xs space-y-1 max-h-[500px] overflow-auto border">
                <div v-if="liveLogs.length === 0" class="text-muted-foreground">等待日志...</div>
                <div
                  v-for="(log, i) in liveLogs"
                  :key="i"
                  :class="cn(
                    log.level === 'error' ? 'text-red-400' :
                    log.level === 'warn' ? 'text-amber-400' :
                    log.level === 'debug' ? 'text-gray-500' : 'text-gray-200',
                  )"
                >
                  <span class="text-gray-500">[{{ formatDate(log.timestamp, 'HH:mm:ss') }}]</span> {{ log.message }}
                </div>
                <div ref="logsEndRef" />
              </div>
            </CardContent>
          </Card>
        </template>
        <div v-else class="flex items-center justify-center h-96 text-muted-foreground">
          <div class="text-center">
            <PlayCircle class="w-16 h-16 mx-auto mb-3 opacity-20" />
            <p class="text-sm">选择执行记录查看详情</p>
            <p class="text-xs mt-1">或在脚本列表中点击执行按钮启动新任务</p>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== Bug Report Modal ==================== -->
    <Dialog v-model="bugModalOpen">
      <template #default="{ close }">
        <DialogHeader>
          <DialogTitle>报告缺陷</DialogTitle>
          <DialogDescription>从自动化平台提交一个缺陷，便于跟踪修复。</DialogDescription>
        </DialogHeader>
        <div class="space-y-3 py-2">
          <div>
            <label class="text-sm font-medium mb-1 block">标题 *</label>
            <Input v-model="bugForm.title" placeholder="简要描述缺陷" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">描述</label>
            <Textarea v-model="bugForm.description" placeholder="详细描述" class="h-20 resize-none" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">复现步骤</label>
            <Textarea v-model="bugForm.steps" placeholder="1. ...&#10;2. ..." class="h-20 resize-none font-mono" />
          </div>
          <div>
            <label class="text-sm font-medium mb-1 block">预期结果</label>
            <Textarea v-model="bugForm.expected" placeholder="预期结果" class="h-16 resize-none" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="close">取消</Button>
          <Button @click="submitBug">提交</Button>
        </DialogFooter>
      </template>
    </Dialog>
  </div>
</template>
