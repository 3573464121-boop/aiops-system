import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Assistant from './views/Assistant.vue'
import Alerts from './views/Alerts.vue'
import Knowledge from './views/Knowledge.vue'
import Issues from './views/Issues.vue'
import Audits from './views/Audits.vue'
import ComingSoon from './views/ComingSoon.vue'

const routes = [
  { path: '/', component: Dashboard },
  { path: '/assistant', component: Assistant },
  { path: '/alerts', component: Alerts },
  { path: '/knowledge', component: Knowledge },
  { path: '/issues', component: Issues },
  { path: '/audits', component: Audits },
  { path: '/memory', component: ComingSoon, props: { title: '记忆空间', desc: '跨对话长期记忆：提取 → 反射 → 召回，个人 / 组 / 全局三级作用域。' } },
  { path: '/inspection', component: ComingSoon, props: { title: '巡检管理', desc: '定时自动巡检、告警治理与效能报告。' } },
  { path: '/assets', component: ComingSoon, props: { title: '资产管理', desc: 'CMDB 服务器 / 数据库实例 / IP 归属 / 数据地图。' } },
  { path: '/workflow', component: ComingSoon, props: { title: '工作流', desc: '可视化编排诊断与处置流程、规则路由与规则演化。' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export default createRouter({ history: createWebHashHistory(), routes })
