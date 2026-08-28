<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import { api } from '../api'

const tasks = ref([])
const reports = ref([])
const loadingTasks = ref(false)
const loadingReports = ref(false)
const creating = ref(false)
const form = ref({ product_id: '', question: '', interval_sec: 300 })

const riskColor = r => ({ ok: 'green', warn: 'gold', high: 'red', error: 'red' }[r] || 'default')
const riskText = r => ({ ok: '正常', warn: '关注', high: '高风险', error: '异常' }[r] || (r || '未运行'))

const taskColumns = [
  { title: '产品', dataIndex: 'product_id', width: 110 },
  { title: '巡检问题', dataIndex: 'question', ellipsis: true },
  { title: '周期', key: 'interval', width: 90 },
  { title: '启用', key: 'enabled', width: 80 },
  { title: '上次结果', key: 'last', width: 100 },
  { title: '上次运行', key: 'lastRun', width: 170 },
  { title: '操作', key: 'ops', width: 150 },
]
const reportColumns = [
  { title: '时间', key: 't', width: 170 },
  { title: '产品', dataIndex: 'product_id', width: 110 },
  { title: '风险', key: 'risk', width: 90 },
  { title: '结论', dataIndex: 'summary', ellipsis: true },
  { title: '置信度', key: 'conf', width: 90 },
]

async function loadTasks() {
  loadingTasks.value = true
  try { tasks.value = (await api('/inspections')).items || [] }
  catch { tasks.value = [] }
  finally { loadingTasks.value = false }
}
async function loadReports() {
  loadingReports.value = true
  try { reports.value = (await api('/inspection-reports?limit=50')).items || [] }
  catch { reports.value = [] }
  finally { loadingReports.value = false }
}
function refresh() { loadTasks(); loadReports() }

async function create() {
  if (!form.value.product_id.trim()) { message.warning('请填写产品标识'); return }
  creating.value = true
  try {
    await api('/inspections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    message.success('巡检任务已创建，将在下一轮调度运行')
    form.value = { product_id: '', question: '', interval_sec: 300 }
    loadTasks()
  } catch (e) { message.error(e.message) }
  finally { creating.value = false }
}
async function toggle(rec, checked) {
  try {
    await api(`/inspections/${rec.id}/toggle`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: checked }),
    })
    rec.enabled = checked
  } catch (e) { message.error(e.message) }
}
async function runNow(rec) {
  message.loading({ content: '正在巡检…', key: rec.id })
  try {
    const r = await api(`/inspections/${rec.id}/run`, { method: 'POST' })
    message.success({ content: `巡检完成：${riskText(r.risk)}`, key: rec.id })
    refresh()
  } catch (e) { message.error({ content: e.message, key: rec.id }) }
}
async function remove(rec) {
  try {
    await api(`/inspections/${rec.id}`, { method: 'DELETE' })
    message.success('已删除')
    loadTasks()
  } catch (e) { message.error(e.message) }
}
function fmtInterval(s) {
  if (s % 3600 === 0) return `${s / 3600} 小时`
  if (s % 60 === 0) return `${s / 60} 分钟`
  return `${s} 秒`
}

let timer = null
onMounted(() => { refresh(); timer = setInterval(loadReports, 20000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h2>主动巡检</h2>
      <p>定时对产品自动跑一轮诊断，发现问题主动沉淀报告（进程内调度，只读安全）</p>
    </header>
    <div class="page-body">
      <a-card size="small" title="新建巡检任务" style="margin-bottom:18px">
        <a-form layout="inline" @submit.prevent="create">
          <a-form-item label="产品">
            <a-input v-model:value="form.product_id" placeholder="如 payment" style="width:150px" />
          </a-form-item>
          <a-form-item label="巡检问题">
            <a-input v-model:value="form.question" placeholder="留空=通用健康巡检" style="width:280px" />
          </a-form-item>
          <a-form-item label="周期">
            <a-select v-model:value="form.interval_sec" style="width:120px">
              <a-select-option :value="60">1 分钟</a-select-option>
              <a-select-option :value="300">5 分钟</a-select-option>
              <a-select-option :value="1800">30 分钟</a-select-option>
              <a-select-option :value="3600">1 小时</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item>
            <a-button type="primary" :loading="creating" @click="create">创建</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <div class="sec-title">
        <span>巡检任务</span>
        <a-button size="small" @click="loadTasks">刷新</a-button>
      </div>
      <a-table :columns="taskColumns" :data-source="tasks" :loading="loadingTasks" row-key="id" :pagination="false" size="small" style="margin-bottom:22px">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'interval'">{{ fmtInterval(record.interval_sec) }}</template>
          <template v-else-if="column.key === 'enabled'">
            <a-switch :checked="record.enabled" size="small" @change="c => toggle(record, c)" />
          </template>
          <template v-else-if="column.key === 'last'">
            <a-tag :color="riskColor(record.last_status)">{{ riskText(record.last_status) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'lastRun'">
            {{ record.last_run_at && !record.last_run_at.startsWith('0001') ? new Date(record.last_run_at).toLocaleString() : '—' }}
          </template>
          <template v-else-if="column.key === 'ops'">
            <a-button type="link" size="small" @click="runNow(record)">立即巡检</a-button>
            <a-popconfirm title="删除该任务？" @confirm="remove(record)">
              <a-button type="link" size="small" danger>删除</a-button>
            </a-popconfirm>
          </template>
        </template>
      </a-table>

      <div class="sec-title">
        <span>巡检报告</span>
        <a-button size="small" @click="loadReports">刷新</a-button>
      </div>
      <a-table :columns="reportColumns" :data-source="reports" :loading="loadingReports" row-key="id" :pagination="{ pageSize: 10 }" size="small">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 't'">{{ new Date(record.created_at).toLocaleString() }}</template>
          <template v-else-if="column.key === 'risk'"><a-tag :color="riskColor(record.risk)">{{ riskText(record.risk) }}</a-tag></template>
          <template v-else-if="column.key === 'conf'">{{ (record.confidence * 100).toFixed(0) }}%</template>
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
.sec-title { display: flex; align-items: center; justify-content: space-between; margin: 0 0 10px; font-size: 14px; font-weight: 600; color: #344054; }
</style>
