<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'
const attrs = useAttrs()
const cls = computed(() => cn('text-lg font-semibold leading-none tracking-tight', (attrs.class as string) || ''))
</script>
<template>
  <h2 :class="cls" v-bind="{ ...attrs, class: undefined }"><slot /></h2>
</template>
