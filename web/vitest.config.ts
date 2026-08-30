import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// 前端单元测试配置：happy-dom 提供 localStorage/URL/WebSocket 等浏览器 API。
// 只跑 src 下的 *.test.ts；组件暂不挂载（无 @vue/test-utils），聚焦核心交互逻辑。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.ts'],
  },
})
