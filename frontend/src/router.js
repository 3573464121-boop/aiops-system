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
import Workflow from './views/Workflow.vue'
import DataSources from './views/DataSources.vue'
import Experiments from './views/Experiments.vue'
import Replay from './views/Replay.vue'
import Users from './views/Users.vue'
import { isAdmin, isLoggedIn } from './api'

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
  { path: '/workflow', component: Workflow },
  { path: '/data-sources', component: DataSources },
  { path: '/experiments', component: Experiments },
  { path: '/replay', component: Replay },
  { path: '/users', component: Users, meta: { adminOnly: true } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({ history: createWebHashHistory(), routes })

// 路由守卫：未登录只能访问登录页；已登录再访问登录页则回到首页。
router.beforeEach((to) => {
  const authed = isLoggedIn()
  if (!to.meta.public && !authed) return '/login'
  if (to.path === '/login' && authed) return '/'
  if (to.meta.adminOnly && !isAdmin()) return '/'
  return true
})

export default router
