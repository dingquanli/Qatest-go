<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'
const attrs = useAttrs()
const cls = computed(() =>
  cn('text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70', (attrs.class as string) || ''),
)
</script>
<template>
  <label :class="cls" v-bind="{ ...attrs, class: undefined }"><slot /></label>
</template>
