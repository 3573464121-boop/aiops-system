import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Assistant from './views/Assistant.vue'
import Alerts from './views/Alerts.vue'
import Knowledge from './views/Knowledge.vue'
import Issues from './views/Issues.vue'
import Audits from './views/Audits.vue'
import Assets from './views/Assets.vue'
import Inspection from './views/Inspection.vue'
import Memory from './views/Memory.vue'
import Login from './views/Login.vue'
import ComingSoon from './views/ComingSoon.vue'
import { isLoggedIn } from './api'

const routes = [
  { path: '/login', component: Login, meta: { public: true } },
  { path: '/', component: Dashboard },
  { path: '/assistant', component: Assistant },
  { path: '/alerts', component: Alerts },
  { path: '/knowledge', component: Knowledge },
  { path: '/issues', component: Issues },
  { path: '/audits', component: Audits },
  { path: '/memory', component: Memory },
  { path: '/inspection', component: Inspection },
  { path: '/assets', component: Assets },
  { path: '/workflow', component: ComingSoon, props: { title: '工作流', desc: '可视化编排诊断与处置流程、规则路由与规则演化。' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({ history: createWebHashHistory(), routes })

// 路由守卫：未登录只能访问登录页；已登录再访问登录页则回到首页。
router.beforeEach((to) => {
  const authed = isLoggedIn()
  if (!to.meta.public && !authed) return '/login'
  if (to.path === '/login' && authed) return '/'
  return true
})

export default router
