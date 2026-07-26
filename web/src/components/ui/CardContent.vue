<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const attrs = useAttrs()
const cls = computed(() => cn('p-5 pt-0', (attrs.class as string) || ''))
</script>

<template>
  <div :class="cls" v-bind="{ ...attrs, class: undefined }">
    <slot />
  </div>
</template>
