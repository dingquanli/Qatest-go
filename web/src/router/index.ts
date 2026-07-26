import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/stores/user'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '工作台' } },
      { path: 'cases', name: 'cases', component: () => import('@/views/Cases.vue'), meta: { title: '测试用例' } },
      { path: 'table-cases', name: 'table-cases', component: () => import('@/views/TableCases.vue'), meta: { title: '表格用例' } },
      { path: 'xmind-cases', name: 'xmind-cases', component: () => import('@/views/XmindCases.vue'), meta: { title: '思维导图用例' } },
      { path: 'testplan', name: 'testplan', component: () => import('@/views/TestPlan.vue'), meta: { title: '测试计划' } },
      { path: 'plan-execs', name: 'plan-execs', component: () => import('@/views/PlanExecs.vue'), meta: { title: '执行记录' } },
      { path: 'api-defs', name: 'api-defs', component: () => import('@/views/ApiDefs.vue'), meta: { title: '接口定义' } },
      { path: 'api-test', name: 'api-test', component: () => import('@/views/ApiTest.vue'), meta: { title: '接口测试' } },
      { path: 'automation', name: 'automation', component: () => import('@/views/Automation.vue'), meta: { title: '自动化平台' } },
      { path: 'proxy-interceptor', name: 'proxy-interceptor', component: () => import('@/views/ProxyInterceptor.vue'), meta: { title: 'gRPC 代理拦截' } },
      { path: 'protocol-recorder', name: 'protocol-recorder', component: () => import('@/views/ProtocolRecorder.vue'), meta: { title: '协议录制' } },
      { path: 'sdk-reports', name: 'sdk-reports', component: () => import('@/views/SdkReports.vue'), meta: { title: 'SDK 上报' } },
      { path: 'bugs', name: 'bugs', component: () => import('@/views/Bugs.vue'), meta: { title: '缺陷管理' } },
      { path: 'settings', name: 'settings', component: () => import('@/views/Settings.vue'), meta: { title: '系统设置' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const user = useUserStore()
  const loggedIn = user.isLoggedIn
  if (!to.meta.public && !loggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && loggedIn) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
