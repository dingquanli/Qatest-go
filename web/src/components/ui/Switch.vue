<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{ modelValue?: boolean; disabled?: boolean }>(), { modelValue: false })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const attrs = useAttrs()
function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
const cls = computed(() =>
  cn(
    'peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=unchecked]:bg-input',
    (attrs.class as string) || '',
  ),
)
</script>

<template>
  <button
    type="button"
    role="switch"
    :data-state="props.modelValue ? 'checked' : 'unchecked'"
    :disabled="props.disabled"
    :class="cls"
    @click="toggle"
    v-bind="{ ...attrs, class: undefined }"
  >
    <span
      class="pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0"
      :data-state="props.modelValue ? 'checked' : 'unchecked'"
    ></span>
  </button>
</template>
