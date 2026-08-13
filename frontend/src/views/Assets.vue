<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const product = ref('')
const ip = ref('')
const rows = ref([])
const loading = ref(false)
const mode = ref('')
const columns = [
  { title: '名称', dataIndex: 'name', width: 180 },
  { title: '类型', key: 'kind', width: 90 },
  { title: '产品', dataIndex: 'product_id', width: 120 },
  { title: 'IP', dataIndex: 'ip', width: 150 },
  { title: '说明', dataIndex: 'detail' },
  { title: '环境', dataIndex: 'env', width: 90 },
  { title: '状态', key: 'status', width: 90 },
]

async function load() {
  loading.value = true
  ip.value = ''
  try {
    const r = await api(`/assets?product_id=${encodeURIComponent(product.value)}`)
    rows.value = r.items || []
    mode.value = r.mode || ''
  } catch { rows.value = [] }
  finally { loading.value = false }
}

async function lookup() {
  const v = ip.value.trim()
  if (!v) { return load() }
  loading.value = true
  try {
    const r = await api(`/assets/lookup?ip=${encodeURIComponent(v)}`)
    rows.value = r.items || []
  } catch { rows.value = [] }
  finally { loading.value = false }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h2>资产管理</h2>
      <p>CMDB 服务器 / 数据库实例清单，支持按产品过滤与 IP 反查<span v-if="mode"> · 数据源 {{ mode }}</span></p>
    </header>
    <div class="page-body">
      <a-space style="margin-bottom:14px" wrap>
        <a-input v-model:value="product" placeholder="产品标识，空=全部" style="width:200px" @pressEnter="load" />
        <a-button type="primary" @click="load">查询</a-button>
        <a-divider type="vertical" />
        <a-input v-model:value="ip" placeholder="按 IP 反查，如 10.0.1.11" style="width:200px" @pressEnter="lookup" />
        <a-button @click="lookup">IP 反查</a-button>
      </a-space>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="id" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'kind'">
            <a-tag :color="record.kind === 'db' ? 'purple' : 'blue'">{{ record.kind === 'db' ? '数据库' : '服务器' }}</a-tag>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-badge :status="record.status === 'online' ? 'success' : 'error'" :text="record.status === 'online' ? '在线' : '离线'" />
          </template>
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
