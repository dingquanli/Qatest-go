<script setup lang="ts">
import { Delete, Plus } from '@element-plus/icons-vue'

export interface KeyValue {
  key: string
  value: string
}

const props = defineProps<{ modelValue: KeyValue[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: KeyValue[]): void }>()

function emitNext(next: KeyValue[]): void {
  emit('update:modelValue', next)
}

function add(): void {
  emitNext([...props.modelValue, { key: '', value: '' }])
}

function removeAt(index: number): void {
  const next = props.modelValue.slice()
  next.splice(index, 1)
  emitNext(next)
}

function setField(index: number, field: keyof KeyValue, value: string): void {
  const next = props.modelValue.slice()
  next[index] = { ...next[index], [field]: value }
  emitNext(next)
}
</script>

<template>
  <div class="kv-editor">
    <div v-for="(row, i) in modelValue" :key="i" class="kv-row">
      <el-input
        :model-value="row.key"
        placeholder="Key"
        @update:model-value="(v: string) => setField(i, 'key', v)"
      />
      <el-input
        :model-value="row.value"
        placeholder="Value"
        @update:model-value="(v: string) => setField(i, 'value', v)"
      />
      <el-button :icon="Delete" circle plain type="danger" @click="removeAt(i)" />
    </div>
    <el-button text type="primary" :icon="Plus" @click="add">添加一行</el-button>
  </div>
</template>

<style scoped>
.kv-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kv-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.kv-row .el-input {
  flex: 1;
}
</style>
