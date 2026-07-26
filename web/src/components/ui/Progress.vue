<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{ modelValue?: number; value?: number; class?: string }>(), {
  modelValue: 0,
  value: 0,
})
const attrs = useAttrs()
const pct = computed(() => Math.min(100, Math.max(0, props.value ?? props.modelValue ?? 0)))
const cls = computed(() => cn('relative h-2 w-full overflow-hidden rounded-full bg-secondary', (attrs.class as string) || ''))
</script>

<template>
  <div :class="cls" v-bind="{ ...attrs, class: undefined }">
    <div class="h-full bg-primary transition-all" :style="{ width: pct + '%' }"></div>
  </div>
</template>
