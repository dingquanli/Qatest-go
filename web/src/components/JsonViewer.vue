<script setup lang="ts">
import { computed } from 'vue'
import { prettyJSON } from '@/utils'

const props = withDefaults(
  defineProps<{
    value: unknown
    maxHeight?: string
  }>(),
  { maxHeight: '360px' },
)

const text = computed(() => prettyJSON(props.value))
</script>

<template>
  <pre class="qt-json qt-mono" :style="{ maxHeight }">{{ text }}</pre>
</template>

<style scoped>
.qt-json {
  margin: 0;
  overflow: auto;
  background: #0b1220;
  border: 1px solid var(--qt-border);
  border-radius: 8px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  color: #cbd5e1;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
