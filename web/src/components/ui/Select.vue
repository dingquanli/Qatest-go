<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { cn } from '@/lib/utils'

interface Opt {
  value: string
  label: string
}
const props = withDefaults(
  defineProps<{ modelValue?: string; options?: Opt[]; placeholder?: string; class?: string }>(),
  { options: () => [], placeholder: '请选择' },
)
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
const open = ref(false)
const root = ref<HTMLElement | null>(null)
const label = computed(() => props.options.find((o) => o.value === props.modelValue)?.label ?? props.placeholder)
function choose(v: string) {
  emit('update:modelValue', v)
  open.value = false
}
function onDoc(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('mousedown', onDoc))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      @click="open = !open"
      :class="
        cn(
          'flex h-9 w-full items-center justify-between rounded-xl border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50',
          props.class,
        )
      "
    >
      <span :class="modelValue ? '' : 'text-muted-foreground'">{{ label }}</span>
      <svg class="h-4 w-4 opacity-50" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="m6 9 6 6 6-6" />
      </svg>
    </button>
    <div
      v-if="open"
      class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-xl border bg-popover p-1 text-popover-foreground shadow-md animate-scale-in"
    >
      <button
        v-for="o in options"
        :key="o.value"
        type="button"
        @click="choose(o.value)"
        :class="
          cn(
            'relative flex w-full cursor-pointer select-none items-center rounded-lg py-1.5 pl-8 pr-2 text-sm outline-none hover:bg-accent hover:text-accent-foreground',
            o.value === modelValue && 'bg-accent text-accent-foreground',
          )
        "
      >
        <span v-if="o.value === modelValue" class="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M20 6 9 17l-5-5" />
          </svg>
        </span>
        {{ o.label }}
      </button>
    </div>
  </div>
</template>
