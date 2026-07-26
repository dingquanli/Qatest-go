<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const attrs = useAttrs()
const cls = computed(() => cn('text-sm text-muted-foreground', (attrs.class as string) || ''))
</script>

<template>
  <p :class="cls" v-bind="{ ...attrs, class: undefined }">
    <slot />
  </p>
</template>
