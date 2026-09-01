<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const rows = ref([])
const loading = ref(false)

const actionText = a => ({
  diagnose: '诊断',
  create_issue: '创建工单',
  create_memory: '新增记忆',
  delete_memory: '删除记忆',
  create_inspection_task: '创建巡检任务',
  toggle_inspection_task: '切换巡检任务',
  manual_run_inspection: '手动执行巡检',
  inspection: '巡检执行',
  delete_inspection_task: '删除巡检任务',
  create_user: '创建用户',
}[a] || a)

const roleText = r => ({
  admin: '管理员',
  viewer: '只读用户',
  system: '系统',
}[r] || r || '-')

const columns = [
  { title: '操作', key: 'action', width: 160 },
  { title: '操作人', key: 'actor', width: 180 },
  { title: '角色', key: 'role', width: 120 },
  { title: '产品', dataIndex: 'product_id', width: 140 },
  { title: '状态', key: 'status', width: 100 },
  { title: '耗时(ms)', dataIndex: 'duration_ms', width: 120 },
  { title: '时间', key: 't', width: 200 },
]

async function load() {
  loading.value = true
  try { rows.value = (await api('/audits')).items || [] }
  catch { rows.value = [] }
  finally { loading.value = false }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h2>审计日志</h2>
      <p>记录诊断、巡检、记忆和用户管理操作，并保留操作人身份</p>
    </header>
    <div class="page-body">
      <a-space style="margin-bottom:14px">
        <a-button @click="load">刷新</a-button>
      </a-space>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="id" :pagination="{ pageSize: 15 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">{{ actionText(record.action) }}</template>
          <template v-else-if="column.key === 'actor'">{{ record.username || 'system' }}</template>
          <template v-else-if="column.key === 'role'">{{ roleText(record.role) }}</template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="record.status === 'success' ? 'green' : 'red'">{{ record.status }}</a-tag>
          </template>
          <template v-else-if="column.key === 't'">{{ new Date(record.created_at).toLocaleString() }}</template>
        </template>
      </a-table>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; }
.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
</style>
