<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link'
    size?: 'default' | 'sm' | 'lg' | 'icon'
  }>(),
  { variant: 'default', size: 'default' },
)

const attrs = useAttrs()
const variants: Record<string, string> = {
  default: 'bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm',
  destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-sm',
  outline: 'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
  secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
  ghost: 'hover:bg-accent hover:text-accent-foreground',
  link: 'text-primary underline-offset-4 hover:underline',
}
const sizes: Record<string, string> = {
  default: 'h-9 px-5 py-2',
  sm: 'h-8 rounded-xl px-3 text-xs',
  lg: 'h-10 rounded-xl px-8',
  icon: 'h-9 w-9',
}
const cls = computed(() =>
  cn(
    'inline-flex items-center justify-center whitespace-nowrap text-sm font-medium ring-offset-background transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 rounded-xl',
    variants[props.variant],
    sizes[props.size],
    (attrs.class as string) || '',
  ),
)
</script>

<template>
  <button :class="cls" v-bind="{ ...attrs, class: undefined }">
    <slot />
  </button>
</template>
