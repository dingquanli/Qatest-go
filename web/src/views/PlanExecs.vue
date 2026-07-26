<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import {
  Search, AlertCircle, RefreshCw, CheckCircle2, XCircle,
  AlertTriangle, Clock, ClipboardCheck, Bot, ChevronRight, ChevronDown, User,
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import Input from '@/components/ui/Input.vue'
import { cn } from '@/lib/utils'
import { safeParseJSON } from '@/utils'
import * as plansApi from '@/api/plans'
import type { PlanExecution, AutoTaskExecution, PlanCaseDetail } from '@/types'

const CASE_RESULT_LABEL: Record<string, string> = {
  passed: '通过', failed: '失败', blocked: '阻塞', skipped: '跳过', pending: '待测',
}

const RESULT_COLORS: Record<string, string> = {
  passed: 'text-emerald-500', success: 'text-emerald-500', completed: 'text-emerald-500',
  failed: 'text-red-500', blocked: 'text-amber-500', skipped: 'text-gray-400',
  pending: 'text-blue-400', running: 'text-blue-400', partial: 'text-amber-500',
}
const RESULT_BG: Record<string, string> = {
  passed: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  completed: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
  blocked: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  skipped: 'bg-gray-100 text-gray-600 dark:bg-gray-900/30 dark:text-gray-400',
  pending: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  running: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  partial: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
}

function formatDate(iso?: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '-'
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
function formatDuration(sec: number): string {
  const ms = sec * 1000
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
}

const planExecs = ref<PlanExecution[]>([])
const taskExecs = ref<AutoTaskExecution[]>([])
const activeTab = ref<'plan' | 'task'>('plan')
const search = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

const filteredPlans = computed(() =>
  planExecs.value.filter((e) => !search.value || (e.planName || '').includes(search.value)),
)

// 展开的执行记录 id 集合
const expanded = ref<Set<string>>(new Set())
function toggleExpand(id: string): void {
  const s = new Set(expanded.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  expanded.value = s
}
function parseDetail(e: PlanExecution): PlanCaseDetail[] {
  return safeParseJSON<PlanCaseDetail[]>(e.casesDetail, [])
}
const filteredTasks = computed(() =>
  taskExecs.value.filter((e) => !search.value || (e.taskName || '').includes(search.value)),
)

async function loadData(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    if (activeTab.value === 'plan') {
      planExecs.value = await plansApi.getPlanExecutions()
    } else {
      taskExecs.value = await plansApi.getAutoTaskExecutions()
    }
  } catch (e) {
    error.value = (e as Error).message || '加载失败'
  } finally {
    loading.value = false
  }
}
watch(activeTab, loadData)
onMounted(loadData)
</script>

<template>
  <div class="flex h-full gap-6">
    <div class="flex-1 flex flex-col gap-4">
      <!-- Tab Switcher -->
      <div class="flex items-center gap-1 bg-muted/50 rounded-xl p-1 w-fit">
        <button
          :class="cn(
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all',
            activeTab === 'plan' ? 'bg-white dark:bg-gray-800 shadow-sm' : 'text-muted-foreground hover:text-foreground',
          )"
          @click="activeTab = 'plan'"
        >
          <ClipboardCheck class="w-3.5 h-3.5" /> 计划执行
        </button>
        <button
          :class="cn(
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all',
            activeTab === 'task' ? 'bg-white dark:bg-gray-800 shadow-sm' : 'text-muted-foreground hover:text-foreground',
          )"
          @click="activeTab = 'task'"
        >
          <Bot class="w-3.5 h-3.5" /> 自动化执行
        </button>
      </div>

      <Card class="flex-1">
        <CardContent class="p-4 flex flex-col h-full">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-semibold">
              {{ activeTab === 'plan' ? '计划执行记录' : '自动化执行记录' }}
              <span class="text-muted-foreground font-normal ml-1">
                ({{ activeTab === 'plan' ? filteredPlans.length : filteredTasks.length }})
              </span>
            </h3>
            <div class="relative">
              <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
              <Input v-model="search" placeholder="搜索..." class="h-8 pl-8 text-xs w-48" />
            </div>
          </div>

          <div v-if="error" class="flex items-center gap-2 p-3 mb-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-xs">
            <AlertCircle class="w-4 h-4" /> {{ error }}
          </div>

          <div class="flex-1 overflow-auto">
            <table v-if="activeTab === 'plan'" class="w-full text-sm">
              <thead>
                <tr class="border-b text-left text-xs text-muted-foreground">
                  <th class="pb-2 font-medium w-6"></th>
                  <th class="pb-2 font-medium">计划名称</th>
                  <th class="pb-2 font-medium">状态</th>
                  <th class="pb-2 font-medium">通过/失败/总数</th>
                  <th class="pb-2 font-medium">执行人</th>
                  <th class="pb-2 font-medium">开始时间</th>
                  <th class="pb-2 font-medium">完成时间</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="e in filteredPlans" :key="e.id">
                  <tr
                    class="border-b border-border/40 hover:bg-accent/50 transition-colors cursor-pointer"
                    @click="toggleExpand(e.id)"
                  >
                    <td class="py-2.5 text-muted-foreground">
                      <component :is="expanded.has(e.id) ? ChevronDown : ChevronRight" class="w-4 h-4" />
                    </td>
                    <td class="py-2.5 font-medium text-sm">{{ e.planName || '-' }}</td>
                    <td class="py-2.5">
                      <Badge :class="cn('text-[10px] font-bold px-1.5 py-0', RESULT_BG[e.status] || '')">{{ e.status }}</Badge>
                    </td>
                    <td class="py-2.5 text-xs">
                      <span class="text-emerald-500 font-medium">{{ e.casesPassed }}</span>
                      <span class="text-muted-foreground"> / </span>
                      <span class="text-red-500 font-medium">{{ e.casesFailed }}</span>
                      <span class="text-muted-foreground"> / {{ e.casesTotal }}</span>
                    </td>
                    <td class="py-2.5 text-xs text-muted-foreground">
                      <span v-if="e.executedBy" class="inline-flex items-center gap-1"><User class="w-3 h-3" />{{ e.executedBy }}</span>
                      <span v-else>-</span>
                    </td>
                    <td class="py-2.5 text-xs text-muted-foreground">{{ formatDate(e.startedAt || e.createdAt) }}</td>
                    <td class="py-2.5 text-xs text-muted-foreground">{{ formatDate(e.finishedAt) }}</td>
                  </tr>
                  <!-- 逐用例明细展开行 -->
                  <tr v-if="expanded.has(e.id)" class="bg-muted/30">
                    <td></td>
                    <td colspan="6" class="py-2 pr-4">
                      <div v-if="parseDetail(e).length === 0" class="text-xs text-muted-foreground py-2">
                        本次执行未记录逐用例明细（可能为旧数据）。
                      </div>
                      <div v-else class="rounded-lg border bg-background overflow-hidden">
                        <div class="grid grid-cols-[36px_1fr_80px_1.2fr] bg-muted/50 text-[11px] text-muted-foreground font-medium">
                          <div class="px-2 py-1.5 text-center border-r">#</div>
                          <div class="px-2 py-1.5 border-r">用例名称</div>
                          <div class="px-2 py-1.5 border-r text-center">结果</div>
                          <div class="px-2 py-1.5">备注</div>
                        </div>
                        <div
                          v-for="(d, i) in parseDetail(e)"
                          :key="d.caseId || i"
                          class="grid grid-cols-[36px_1fr_80px_1.2fr] border-t text-xs items-center"
                        >
                          <div class="px-2 py-1.5 text-center text-muted-foreground border-r">{{ i + 1 }}</div>
                          <div class="px-2 py-1.5 border-r truncate">{{ d.caseName || d.caseId }}</div>
                          <div class="px-2 py-1.5 border-r text-center">
                            <span :class="cn('inline-block px-1.5 py-0.5 rounded text-[10px] font-medium', RESULT_BG[d.result] || 'bg-muted text-muted-foreground')">
                              {{ CASE_RESULT_LABEL[d.result] || d.result }}
                            </span>
                          </div>
                          <div class="px-2 py-1.5 text-muted-foreground truncate">{{ d.remark || '-' }}</div>
                        </div>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>

            <table v-else class="w-full text-sm">
              <thead>
                <tr class="border-b text-left text-xs text-muted-foreground">
                  <th class="pb-2 font-medium">任务名称</th>
                  <th class="pb-2 font-medium">状态</th>
                  <th class="pb-2 font-medium">耗时</th>
                  <th class="pb-2 font-medium">结果</th>
                  <th class="pb-2 font-medium">创建时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="e in filteredTasks" :key="e.id" class="border-b border-border/40 hover:bg-accent/50 transition-colors">
                  <td class="py-2.5 font-medium text-sm">{{ e.taskName }}</td>
                  <td class="py-2.5">
                    <span :class="cn('inline-flex items-center gap-1 text-xs font-medium', RESULT_COLORS[e.status] || '')">
                      <CheckCircle2 v-if="e.status === 'success' || e.status === 'completed'" class="w-3 h-3" />
                      <XCircle v-else-if="e.status === 'failed'" class="w-3 h-3" />
                      <RefreshCw v-else-if="e.status === 'running'" class="w-3 h-3 animate-spin" />
                      <AlertTriangle v-else class="w-3 h-3" />
                      {{ e.status }}
                    </span>
                  </td>
                  <td class="py-2.5 text-xs text-muted-foreground">{{ e.duration ? formatDuration(e.duration) : '-' }}</td>
                  <td class="py-2.5 text-xs text-muted-foreground max-w-40 truncate">
                    {{ typeof e.result === 'string' ? e.result : JSON.stringify(e.result) }}
                  </td>
                  <td class="py-2.5 text-xs text-muted-foreground">
                    <span class="inline-flex items-center gap-1"><Clock class="w-3 h-3" />{{ formatDate(e.createdAt) }}</span>
                  </td>
                </tr>
              </tbody>
            </table>

            <div
              v-if="!loading && ((activeTab === 'plan' && filteredPlans.length === 0) || (activeTab === 'task' && filteredTasks.length === 0))"
              class="text-center py-12 text-sm text-muted-foreground"
            >
              {{ search ? '无匹配记录' : '暂无执行记录' }}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
