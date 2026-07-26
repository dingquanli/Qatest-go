<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, shallowRef } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'

const props = withDefaults(
  defineProps<{
    option: EChartsOption
    height?: string
  }>(),
  { height: '320px' },
)

const el = ref<HTMLDivElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)
let resizeObserver: ResizeObserver | null = null

function render(): void {
  if (!el.value) return
  if (!chart.value) {
    // 使用内置 'dark' 主题适配暗色风格
    chart.value = echarts.init(el.value, 'dark')
  }
  chart.value.setOption(props.option, true)
}

onMounted(() => {
  render()
  if (el.value) {
    resizeObserver = new ResizeObserver(() => chart.value?.resize())
    resizeObserver.observe(el.value)
  }
})

watch(
  () => props.option,
  () => render(),
  { deep: true },
)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart.value?.dispose()
  chart.value = null
})
</script>

<template>
  <div ref="el" :style="{ width: '100%', height }"></div>
</template>
