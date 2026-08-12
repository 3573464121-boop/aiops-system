<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  DashboardOutlined, RobotOutlined, AlertOutlined, ReadOutlined, ProfileOutlined, AuditOutlined,
  BulbOutlined, ScheduleOutlined, DatabaseOutlined, ApartmentOutlined,
  MenuFoldOutlined, MenuUnfoldOutlined, ThunderboltFilled,
} from '@ant-design/icons-vue'

const collapsed = ref(false)
const route = useRoute()
const nav = [
  { to: '/', label: '仪表盘', icon: DashboardOutlined },
  { to: '/assistant', label: '运维助手', icon: RobotOutlined },
  { to: '/alerts', label: '告警管理', icon: AlertOutlined },
  { to: '/knowledge', label: '知识库', icon: ReadOutlined },
  { to: '/issues', label: '问题工单', icon: ProfileOutlined },
  { to: '/audits', label: '审计日志', icon: AuditOutlined },
  { divider: true },
  { to: '/memory', label: '记忆空间', icon: BulbOutlined },
  { to: '/inspection', label: '巡检管理', icon: ScheduleOutlined },
  { to: '/assets', label: '资产管理', icon: DatabaseOutlined },
  { to: '/workflow', label: '工作流', icon: ApartmentOutlined },
]
</script>

<template>
  <div class="layout">
    <nav :class="['rail', { collapsed }]">
      <div class="logo"><ThunderboltFilled /><span v-if="!collapsed">智能运维</span></div>
      <div class="nav">
        <template v-for="(item, i) in nav" :key="i">
          <div v-if="item.divider" class="nav-div"></div>
          <router-link v-else :to="item.to" class="nav-item" :class="{ active: route.path === item.to }" :title="item.label">
            <component :is="item.icon" class="nav-ic" />
            <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
          </router-link>
        </template>
      </div>
      <button class="collapse" @click="collapsed = !collapsed">
        <component :is="collapsed ? MenuUnfoldOutlined : MenuFoldOutlined" />
        <span v-if="!collapsed">收起</span>
      </button>
    </nav>
    <div class="content"><router-view /></div>
  </div>
</template>

<style scoped>
.layout { display: flex; height: 100vh; }
.rail { width: 210px; flex: none; background: #0f1b2d; color: #c7d2de; display: flex; flex-direction: column; padding: 14px 10px; transition: width .2s ease; }
.rail.collapsed { width: 66px; }
.logo { display: flex; align-items: center; gap: 10px; color: #fff; font-weight: 700; font-size: 16px; padding: 6px 10px 16px; white-space: nowrap; }
.logo :deep(.anticon) { color: #4db8ff; font-size: 20px; }
.nav { flex: 1; overflow-y: auto; display: flex; flex-direction: column; }
.nav-item { display: flex; align-items: center; gap: 12px; padding: 11px 13px; border-radius: 10px; color: #aebccb; text-decoration: none; margin-bottom: 3px; white-space: nowrap; transition: background .15s, color .15s; }
.nav-ic { font-size: 17px; }
.nav-item:hover { background: #182740; color: #fff; }
.nav-item.active { background: #1677ff; color: #fff; }
.nav-div { height: 1px; background: #1c2c44; margin: 10px 8px; }
.collapse { margin-top: auto; display: flex; align-items: center; gap: 10px; width: 100%; background: transparent; border: 0; color: #8fa3b8; padding: 11px 13px; cursor: pointer; border-radius: 10px; font-size: 14px; }
.collapse:hover { background: #182740; color: #fff; }
.content { flex: 1; min-width: 0; background: #f5f6f8; overflow: hidden; display: flex; }
</style>
