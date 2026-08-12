<script setup>
import { ref, onMounted } from 'vue'
import { api, sevText, sevColor } from '../api'

const product = ref('')
const rows = ref([])
const loading = ref(false)
const columns = [
  { title: '级别', key: 'severity', width: 90 },
  { title: '规则', dataIndex: 'rule' },
  { title: '对象', dataIndex: 'target' },
  { title: '值', dataIndex: 'value' },
  { title: '触发时间', key: 't', width: 190 },
]

async function load() {
  loading.value = true
  try { rows.value = (await api(`/alerts/active?product_id=${encodeURIComponent(product.value)}`)).items || [] }
  catch { rows.value = [] }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head"><h2>告警管理</h2><p>当前活跃告警（安全只读）</p></header>
    <div class="page-body">
      <a-space style="margin-bottom:14px">
        <a-input v-model:value="product" placeholder="产品标识，空=全部" style="width:220px" @pressEnter="load" />
        <a-button type="primary" @click="load">查询</a-button>
      </a-space>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="id" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'severity'"><a-tag :color="sevColor(record.severity)">{{ sevText(record.severity) }}</a-tag></template>
          <template v-else-if="column.key === 't'">{{ new Date(record.triggered_at).toLocaleString() }}</template>
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
