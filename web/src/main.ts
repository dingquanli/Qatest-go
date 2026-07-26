import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import App from './App.vue'
import router from './router'
import './styles/index.css'
// 在应用启动时就应用主题：确保真实主题（深色/浅色）从首屏即生效，
// 而不是等进入“系统设置”页面才因 useTheme 模块被加载而误翻转为浅色。
import { useTheme } from './composables/useTheme'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus)

// 启动即应用主题（在挂载前执行，避免首屏闪烁与“进入设置才翻转”的问题）
useTheme()

// 全量注册 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component as never)
}

app.mount('#app')
