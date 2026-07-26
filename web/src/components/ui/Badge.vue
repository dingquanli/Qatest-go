<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'
  }>(),
  { variant: 'default' },
)

const attrs = useAttrs()
const variants: Record<string, string> = {
  default: 'border-transparent bg-primary text-primary-foreground',
  secondary: 'border-transparent bg-secondary text-secondary-foreground',
  destructive: 'border-transparent bg-destructive text-destructive-foreground',
  outline: 'text-foreground border-border',
  success: 'border-transparent bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400',
  warning: 'border-transparent bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400',
}
const cls = computed(() =>
  cn(
    'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
    variants[props.variant],
    (attrs.class as string) || '',
  ),
)
</script>

<template>
  <div :class="cls" v-bind="{ ...attrs, class: undefined }">
    <slot />
  </div>
</template>
