# Qatest-go 前端架构与技术选型报告

> 更新时间：2026-07-24
> 适用版本：Vue 3 重构版（已完成 React → Vue 迁移，本文以当前代码为准）

## 技术栈

| 维度 | 选型 | 说明 |
|---|---|---|
| 框架 | Vue 3（Composition API + `<script setup>`） | 组合式 API，逻辑复用清晰 |
| 构建 | Vite 5 | 快启动、按需编译；产物输出至 `../static` 由后端同源托管 |
| 状态管理 | Pinia | 轻量 store，替代 Vuex |
| 路由 | Vue Router 4 | 视图懒加载（`() => import(...)`） |
| UI 组件 | Element Plus（全局注册）+ shadcn 风格 `components/ui/*` | 业务表单用 Element Plus，基础控件用 shadcn 风格封装 |
| 样式 | Tailwind CSS + PostCSS | 原子化样式 |
| 图表 | ECharts 5 | 报表/趋势可视化 |
| 图标 | lucide-vue-next | 线性图标 |
| 请求 | axios + 统一拦截器 | 拦截 `success=false` 自动报错 |

## 目录结构

```
web/src/
├── api/            # 后端接口封装（request.ts 统一拦截）
├── components/     # 业务组件 + ui/（shadcn 风格基础控件）
├── composables/    # 组合式逻辑复用
├── layouts/        # AppLayout 等布局（侧边导航）
├── router/         # 路由定义（懒加载视图）
├── store/          # Pinia store
├── types/          # TypeScript 类型定义
├── views/          # 页面级视图（15+）
├── env.d.ts
├── main.ts         # 入口：注册 Element Plus / Pinia / 路由
└── App.vue
```

## 关键设计

- **同源托管**：Vite 构建输出到 `../static`，后端 Gin 直接托管，无需独立前端部署与跨域配置（CORS 仍可按 `ALLOWED_ORIGINS` 配置）。
- **统一响应信封**：后端返回 `{ success, data, error }`，`request.ts` 在拦截器中判断 `success`，失败自动 `ElMessage` 报错并 reject。
- **导航结构**：`AppLayout.vue` 维护 `api` 分组导航，含「SDK 上报」等入口；新增视图需在 `router/index.ts` 与 `AppLayout` 导航同步注册。
- **SDK 上报页**：`SdkReports.vue` 调用 `GET /api/qa/reports` 拉取上报记录，支持事件过滤、详情抽屉（headers/request/response 美化 + 复制）。
- **上报令牌**：协议录制页展示 `reportToken`（来自 `GET /api/config/sdk/list`），供 SDK 接入鉴权。

## 构建与运行

```bash
cd web
npm install
npm run build      # 产物至 ../static
# 开发预览：npm run dev
```

修改前端后必须重新 `npm run build`，后端才会提供更新后的 UI。
