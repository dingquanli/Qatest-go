<script setup lang="ts">
import { ref } from 'vue'
const show = ref(false)
</script>

<template>
  <span class="relative inline-flex" @mouseenter="show = true" @mouseleave="show = false">
    <slot />
    <span
      v-if="show"
      class="absolute bottom-full left-1/2 z-50 mb-1 -translate-x-1/2 whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-xs text-background shadow animate-fade-in"
    >
      <slot name="content" />
    </span>
  </span>
</template>
