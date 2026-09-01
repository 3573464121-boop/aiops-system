<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { ApiOutlined, CheckCircleOutlined, DatabaseOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { api, isAdmin } from '../api'

const rows = ref([])
const loading = ref(false)
const testing = ref({})
const admin = isAdmin()
const readyCount = computed(() => rows.value.filter(v => v.status === 'ready').length)
const demoCount = computed(() => rows.value.filter(v => v.status === 'demo').length)
const errorCount = computed(() => rows.value.filter(v => v.status === 'error').length)

const columns = [
  { title: '数据源', key: 'name', width: 180 },
  { title: '类型', key: 'kind', width: 110 },
  { title: '当前模式', dataIndex: 'mode', width: 170 },
  { title: '连接状态', key: 'status', width: 120 },
  { title: '服务地址', key: 'endpoint' },
  { title: '延迟', key: 'latency', width: 100 },
  { title: '操作', key: 'ops', width: 110 },
]
const nameText = v => ({ alerts: '告警平台', logs: '日志平台', knowledge: '知识索引', assets: '资产数据', llm: '大模型', storage: '持久化存储' }[v] || v)
const kindText = v => ({ alert: '告警', log: '日志', knowledge: '知识', asset: '资产', model: '模型', storage: '存储' }[v] || v)
const statusText = v => ({ ready: '正常', demo: '演示数据', unknown: '待检测', error: '异常' }[v] || v)
const statusColor = v => ({ ready: 'green', demo: 'blue', unknown: 'gold', error: 'red' }[v] || 'default')

async function load() {
  loading.value = true
  try { rows.value = (await api('/data-sources')).items || [] }
  catch (e) { message.error(e.message) }
  finally { loading.value = false }
}
async function test(record) {
  testing.value[record.name] = true
  try {
    const result = await api(`/data-sources/${record.name}/test`, { method: 'POST' })
    Object.assign(record, result)
    if (result.status === 'demo') message.info(`${nameText(record.name)}当前使用演示数据`)
    else message.success(`${nameText(record.name)}连接正常`)
  } catch (e) {
    record.status = 'error'
    record.message = e.message
    message.error(e.message)
  } finally { testing.value[record.name] = false }
}
async function testAll() {
  for (const row of rows.value) await test(row)
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div><h2>数据源</h2><p>统一查看诊断链路依赖的数据平台与当前连接状态</p></div>
      <a-button v-if="admin" type="primary" :disabled="loading" @click="testAll"><template #icon><SyncOutlined /></template>全部检测</a-button>
    </header>
    <div class="page-body">
      <section class="summary">
        <div><CheckCircleOutlined class="green" /><strong>{{ readyCount }}</strong><span>连接正常</span></div>
        <div><ApiOutlined class="blue" /><strong>{{ demoCount }}</strong><span>演示模式</span></div>
        <div><DatabaseOutlined class="red" /><strong>{{ errorCount }}</strong><span>连接异常</span></div>
        <p>共 {{ rows.length }} 个数据源，真实告警与日志接入后会自动替换演示 Provider。</p>
      </section>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="name" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'"><strong>{{ nameText(record.name) }}</strong><small>{{ record.message }}</small></template>
          <template v-else-if="column.key === 'kind'">{{ kindText(record.kind) }}</template>
          <template v-else-if="column.key === 'status'"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
          <template v-else-if="column.key === 'endpoint'"><code>{{ record.endpoint || '-' }}</code></template>
          <template v-else-if="column.key === 'latency'">{{ record.latency_ms ? `${record.latency_ms} ms` : '-' }}</template>
          <template v-else-if="column.key === 'ops'"><a-button v-if="admin" type="link" size="small" :loading="testing[record.name]" @click="test(record)">检测</a-button></template>
        </template>
      </a-table>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { min-height: 78px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #edeff2; display: flex; justify-content: space-between; align-items: center; gap: 20px; }
.page-head h2 { margin: 0; font-size: 18px; }.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
.summary { display: grid; grid-template-columns: repeat(3, 170px) 1fr; align-items: center; min-height: 88px; background: #fff; border: 1px solid #e8ebef; border-radius: 8px; margin-bottom: 18px; }
.summary > div { display: grid; grid-template-columns: 34px 1fr; grid-template-rows: 1fr 1fr; align-items: center; padding: 14px 20px; border-right: 1px solid #edf0f3; }
.summary :deep(.anticon) { grid-row: 1 / 3; font-size: 23px; }.summary strong { font-size: 22px; line-height: 1; }.summary span { color: #667085; font-size: 12px; }
.summary p { color: #667085; padding: 0 22px; margin: 0; font-size: 13px; }.green { color: #389e0d; }.blue { color: #1677ff; }.red { color: #cf1322; }
strong + small { display: block; margin-top: 3px; color: #98a2b3; font-weight: 400; max-width: 260px; white-space: normal; } code { color: #475467; }
@media (max-width: 900px) { .summary { grid-template-columns: 1fr; }.summary > div { border-right: 0; border-bottom: 1px solid #edf0f3; }.summary p { padding: 16px 20px; }.page-head { align-items: flex-start; flex-direction: column; }.page-body { padding: 16px; } }
</style>
