<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{ modelValue?: boolean; class?: string }>(), { modelValue: false })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const attrs = useAttrs()
function close() {
  emit('update:modelValue', false)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="props.modelValue" class="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div class="fixed inset-0 bg-black/60 animate-fade-in" @click="close"></div>
      <div
        :class="
          cn(
            'relative z-50 grid w-full max-w-lg gap-4 border bg-card p-6 shadow-lg rounded-2xl animate-scale-in',
            (attrs.class as string) || '',
          )
        "
        v-bind="{ ...attrs, class: undefined }"
      >
        <slot :close="close" />
      </div>
    </div>
  </Teleport>
</template>
