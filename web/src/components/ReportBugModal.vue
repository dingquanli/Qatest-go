<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import * as bugsApi from '@/api/bugs'
import type { Bug, BugSeverity, BugStatus, Priority } from '@/types'
import {
  BUG_SEVERITY_OPTIONS,
  BUG_STATUS_OPTIONS,
  PRIORITY_OPTIONS,
  SEVERITY_LABEL,
  BUG_STATUS_LABEL,
} from '@/types'

const props = withDefaults(
  defineProps<{ modelValue: boolean; initial?: Partial<Bug> }>(),
  { initial: () => ({}) },
)
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'submitted'): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

interface BugForm {
  title: string
  severity: BugSeverity
  priority: Priority
  status: BugStatus
  module: string
  env: string
  assignee: string
  reporter: string
  description: string
  steps: string
  expected: string
  actual: string
  tags: string
}

function emptyForm(): BugForm {
  return {
    title: '',
    severity: 'major',
    priority: 'P2',
    status: 'open',
    module: '',
    env: '',
    assignee: '',
    reporter: '',
    description: '',
    steps: '',
    expected: '',
    actual: '',
    tags: '',
  }
}

const form = ref<BugForm>(emptyForm())
const submitting = ref(false)

watch(visible, (open) => {
  if (open) {
    form.value = { ...emptyForm(), ...props.initial }
  }
})

async function submit(): Promise<void> {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写缺陷标题')
    return
  }
  submitting.value = true
  try {
    const payload: Partial<Bug> = {
      title: form.value.title,
      severity: form.value.severity,
      priority: form.value.priority,
      status: form.value.status,
      module: form.value.module,
      env: form.value.env,
      assignee: form.value.assignee,
      reporter: form.value.reporter,
      description: form.value.description,
      steps: form.value.steps,
      expected: form.value.expected,
      actual: form.value.actual,
      tags: JSON.stringify(
        form.value.tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      ),
    }
    await bugsApi.createBug(payload)
    ElMessage.success('缺陷已创建')
    visible.value = false
    emit('submitted')
  } catch (err) {
    ElMessage.error((err as Error).message || '创建失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" title="提交缺陷" width="640px" destroy-on-close>
    <el-form :model="form" label-width="92px" label-position="right">
      <el-form-item label="标题" required>
        <el-input v-model="form.title" placeholder="缺陷标题" />
      </el-form-item>
      <el-row :gutter="12">
        <el-col :span="8">
          <el-form-item label="严重程度">
            <el-select v-model="form.severity" style="width: 100%">
              <el-option
                v-for="s in BUG_SEVERITY_OPTIONS"
                :key="s"
                :label="SEVERITY_LABEL[s]"
                :value="s"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="优先级">
            <el-select v-model="form.priority" style="width: 100%">
              <el-option v-for="p in PRIORITY_OPTIONS" :key="p" :label="p" :value="p" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="状态">
            <el-select v-model="form.status" style="width: 100%">
              <el-option
                v-for="s in BUG_STATUS_OPTIONS"
                :key="s"
                :label="BUG_STATUS_LABEL[s]"
                :value="s"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="模块">
            <el-input v-model="form.module" placeholder="所属模块" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="环境">
            <el-input v-model="form.env" placeholder="如 Test/Prod" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="指派给">
            <el-input v-model="form.assignee" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="报告人">
            <el-input v-model="form.reporter" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" />
      </el-form-item>
      <el-form-item label="复现步骤">
        <el-input v-model="form.steps" type="textarea" :rows="2" />
      </el-form-item>
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="期望结果">
            <el-input v-model="form.expected" type="textarea" :rows="2" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="实际结果">
            <el-input v-model="form.actual" type="textarea" :rows="2" />
          </el-form-item>
        </el-col>
      </el-row>
      <el-form-item label="标签">
        <el-input v-model="form.tags" placeholder="逗号分隔，如 login,crash" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">提交</el-button>
    </template>
  </el-dialog>
</template>
