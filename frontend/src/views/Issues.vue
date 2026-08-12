<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const rows = ref([])
const loading = ref(false)
const statusColor = s => ({ open: 'blue', in_progress: 'gold', resolved: 'green', dismissed: 'default' }[s] || 'default')
const statusText = s => ({ open: '待处理', in_progress: '处理中', resolved: '已解决', dismissed: '已忽略' }[s] || s)
const columns = [
  { title: '工单号', dataIndex: 'id', width: 220 },
  { title: '产品', dataIndex: 'product_id', width: 120 },
  { title: '标题', dataIndex: 'title' },
  { title: '状态', key: 'status', width: 100 },
  { title: '创建时间', key: 't', width: 190 },
]

async function load() {
  loading.value = true
  try { rows.value = (await api('/issues')).items || [] }
  catch { rows.value = [] }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head"><h2>问题工单</h2><p>诊断沉淀的可追踪工单（写入 MySQL）</p></header>
    <div class="page-body">
      <a-space style="margin-bottom:14px"><a-button @click="load">刷新</a-button></a-space>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
          <template v-else-if="column.key === 't'">{{ new Date(record.created_at).toLocaleString() }}</template>
        </template>
      </a-table>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; } .page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
</style>
