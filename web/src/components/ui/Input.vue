<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ modelValue?: string | number }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const attrs = useAttrs()
const cls = computed(() =>
  cn(
    'flex h-9 w-full rounded-xl border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 transition-shadow',
    (attrs.class as string) || '',
  ),
)

function onInput(e: Event): void {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}
</script>

<template>
  <input
    :class="cls"
    :value="props.modelValue"
    v-bind="{ ...attrs, class: undefined }"
    @input="onInput"
  >
</template>
