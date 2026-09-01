<script setup>
import { computed, onMounted, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { DeleteOutlined, FileAddOutlined, PlayCircleOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons-vue'
import { api } from '../api'

const cases = ref([])
const results = ref([])
const selected = ref(null)
const loading = ref(false)
const running = ref(false)
const importOpen = ref(false)
const importing = ref(false)
const configs = ref(['full', 'bm25', 'no-agent'])
const jsonText = ref('')
const fileInput = ref(null)

const columns = [
  { title: '案例', key: 'name' }, { title: '产品', dataIndex: 'product_id', width: 110 },
  { title: '版本', dataIndex: 'version', width: 80 }, { title: '来源', key: 'source', width: 100 },
  { title: '证据', key: 'evidence', width: 110 }, { title: '最近导入', key: 'created', width: 175 },
  { title: '操作', key: 'ops', width: 155 },
]
const resultColumns = [
  { title: '配置', key: 'config', width: 120 }, { title: '根因正确', key: 'cause', width: 105 },
  { title: '忠实度', key: 'faith', width: 95 }, { title: '幻觉', key: 'hallucination', width: 85 },
  { title: '证据数', key: 'evidence', width: 80 }, { title: '工具失败', dataIndex: 'tool_failures', width: 95 },
  { title: '耗时', key: 'duration', width: 95 }, { title: '诊断结论', key: 'summary' },
]

const selectedResults = computed(() => selected.value ? results.value.filter(v => v.case_id === selected.value.id) : [])
const latestResults = computed(() => {
  const seen = new Set()
  return selectedResults.value.filter(v => seen.has(v.config) ? false : (seen.add(v.config), true))
})
const judged = computed(() => results.value.filter(v => v.judged))
const correctRate = computed(() => judged.value.length ? judged.value.filter(v => v.cause_correct).length / judged.value.length : 0)
const sourceText = v => ({ real: '真实脱敏', synthetic: '合成', imported: '导入' }[v] || v)
const configText = v => ({ full: '完整系统', bm25: '仅 BM25', 'no-agent': '无 Agent' }[v] || v)
const pct = v => `${Math.round((v || 0) * 100)}%`

async function load() {
  loading.value = true
  try {
    const [a, b] = await Promise.all([api('/fault-cases'), api('/replay-results?limit=2000')])
    cases.value = a.items || []
    results.value = b.items || []
    if (selected.value) selected.value = cases.value.find(v => v.id === selected.value.id) || null
  } catch (e) { message.error(e.message) }
  finally { loading.value = false }
}

function openImport() {
  jsonText.value = JSON.stringify({
    name: '支付服务连接池耗尽', product_id: 'payment', question: '分析支付服务持续超时的根因',
    gold_cause: '数据库连接池耗尽导致请求排队并超时', source: 'synthetic', version: 'v1', tags: ['database', 'timeout'],
    alerts: [{ id: 'ALERT-001', product_id: 'payment', rule: '接口错误率过高', severity: 1, target: 'payment-api-01', value: 'error_rate=12.4%' }],
    logs: [{ type: 'log', title: 'payment-api-01', content: 'connection pool exhausted, wait timeout after 3000ms', score: 1, source: 'replay/payment-api-01' }],
    assets: [{ id: 'srv-p01', product_id: 'payment', kind: 'server', name: 'payment-api-01', ip: '10.0.1.11', detail: '支付接口服务', env: 'prod', status: 'online' }],
  }, null, 2)
  importOpen.value = true
}
async function readFile(event) {
  const file = event.target.files?.[0]
  if (file) jsonText.value = await file.text()
  event.target.value = ''
}
async function importCase() {
  let payload
  try { payload = JSON.parse(jsonText.value) } catch { message.warning('JSON 格式不正确'); return }
  importing.value = true
  try {
    const created = await api('/fault-cases', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    message.success('故障案例已导入')
    importOpen.value = false
    await load()
    selected.value = created
  } catch (e) { message.error(e.message) }
  finally { importing.value = false }
}
async function replay(record = selected.value) {
  if (!record) return
  if (!configs.value.length) { message.warning('至少选择一种实验配置'); return }
  selected.value = record
  running.value = true
  try {
    const data = await api(`/fault-cases/${record.id}/replay`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ configs: configs.value }) })
    message.success(`回放完成，生成 ${data.total} 组结果`)
    await load()
  } catch (e) { message.error(e.message) }
  finally { running.value = false }
}
function removeCase(record) {
  Modal.confirm({
    title: '删除这个故障案例？', content: '历史回放结果仍会保留，案例本身将从列表移除。', okText: '删除', okType: 'danger', cancelText: '取消',
    async onOk() { await api(`/fault-cases/${record.id}`, { method: 'DELETE' }); selected.value = null; await load() },
  })
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div><h2>故障回放</h2><p>用固定案例对照 Agent、检索策略与无证据基线，为实验复现保留完整记录</p></div>
      <a-space><a-button @click="load"><ReloadOutlined />刷新</a-button><a-button type="primary" @click="openImport"><FileAddOutlined />导入案例</a-button></a-space>
    </header>
    <div class="page-body">
      <section class="overview">
        <div><strong>{{ cases.length }}</strong><span>故障案例</span></div><div><strong>{{ results.length }}</strong><span>回放结果</span></div>
        <div><strong>{{ pct(correctRate) }}</strong><span>已评审结果准确率</span></div><div class="protocol"><b>对照配置</b><span>完整系统 / 仅 BM25 / 无 Agent</span></div>
      </section>
      <section class="panel">
        <div class="section-head"><div><h3>案例库</h3><span>导入前应完成脱敏，并固定案例版本与标准根因</span></div></div>
        <a-table :columns="columns" :data-source="cases" :loading="loading" row-key="id" :pagination="{ pageSize: 8 }" size="middle">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'"><button class="case-name" @click="selected = record">{{ record.name }}</button><small>{{ record.question }}</small></template>
            <template v-else-if="column.key === 'source'"><a-tag>{{ sourceText(record.source) }}</a-tag></template>
            <template v-else-if="column.key === 'evidence'">{{ record.alerts.length }} / {{ record.logs.length }} / {{ record.assets.length }}</template>
            <template v-else-if="column.key === 'created'">{{ new Date(record.created_at).toLocaleString() }}</template>
            <template v-else-if="column.key === 'ops'"><a-button type="link" size="small" :loading="running && selected?.id === record.id" @click="replay(record)"><PlayCircleOutlined />回放</a-button><a-button type="text" size="small" danger title="删除案例" @click="removeCase(record)"><DeleteOutlined /></a-button></template>
          </template>
          <template #emptyText>尚未导入故障案例</template>
        </a-table>
      </section>
      <section v-if="selected" class="panel">
        <div class="section-head replay-head">
          <div><h3>{{ selected.name }}</h3><span>{{ selected.version }} · 标准根因：{{ selected.gold_cause }}</span></div>
          <div class="run-controls"><a-checkbox-group v-model:value="configs" :options="[{ label: '完整系统', value: 'full' }, { label: '仅 BM25', value: 'bm25' }, { label: '无 Agent', value: 'no-agent' }]" /><a-button type="primary" :loading="running" @click="replay()"><PlayCircleOutlined />运行对照实验</a-button></div>
        </div>
        <a-table :columns="resultColumns" :data-source="latestResults" row-key="id" :pagination="false" size="middle">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'config'"><a-tag color="blue">{{ configText(record.config) }}</a-tag></template>
            <template v-else-if="column.key === 'cause'"><a-tag :color="record.judged && record.cause_correct ? 'green' : 'red'">{{ record.judged ? (record.cause_correct ? '正确' : '错误') : '未评审' }}</a-tag></template>
            <template v-else-if="column.key === 'faith'">{{ record.judged ? pct(record.faithfulness) : '-' }}</template>
            <template v-else-if="column.key === 'hallucination'"><a-tag :color="record.hallucination ? 'red' : 'green'">{{ record.judged ? (record.hallucination ? '有' : '无') : '-' }}</a-tag></template>
            <template v-else-if="column.key === 'evidence'">{{ record.diagnosis.evidence.length }}</template>
            <template v-else-if="column.key === 'duration'">{{ (record.duration_ms / 1000).toFixed(1) }}s</template>
            <template v-else-if="column.key === 'summary'"><span class="summary-text">{{ record.diagnosis.summary }}</span></template>
          </template>
          <template #emptyText>选择配置并运行后显示对照结果</template>
        </a-table>
      </section>
    </div>
    <a-modal v-model:open="importOpen" title="导入故障案例" width="min(760px, 96vw)" :confirm-loading="importing" ok-text="校验并导入" cancel-text="取消" @ok="importCase">
      <div class="import-tools"><span>粘贴案例 JSON，或选择 UTF-8 JSON 文件。</span><a-button @click="fileInput?.click()"><UploadOutlined />选择文件</a-button><input ref="fileInput" type="file" accept="application/json,.json" hidden @change="readFile" /></div>
      <a-textarea v-model:value="jsonText" :rows="20" class="json-editor" spellcheck="false" />
      <a-alert type="warning" show-icon message="真实案例必须移除账号、客户名称、公网地址和密钥等敏感信息。" />
    </a-modal>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; color: #202938; }.page-head { min-height: 78px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #e7eaf0; display: flex; justify-content: space-between; align-items: center; gap: 20px; }.page-head h2,.section-head h3 { margin: 0; letter-spacing: 0; }.page-head h2 { font-size: 18px; }.page-head p { margin: 4px 0 0; color: #7c8798; font-size: 13px; }.page-body { padding: 22px 26px 40px; }
.overview { display: grid; grid-template-columns: repeat(3,minmax(130px,170px)) 1fr; min-height: 86px; background: #fff; border: 1px solid #e2e7ee; border-radius: 8px; margin-bottom: 18px; }.overview>div { padding: 16px 20px; display: flex; flex-direction: column; justify-content: center; border-right: 1px solid #edf0f4; }.overview strong { font-size: 23px; line-height: 1.15; }.overview span { margin-top: 4px; color: #667085; font-size: 12px; }.overview .protocol { border-right: 0; }.protocol b { font-size: 13px; }
.panel { background: #fff; border: 1px solid #e2e7ee; border-radius: 8px; margin-bottom: 18px; overflow: hidden; }.section-head { padding: 15px 18px; border-bottom: 1px solid #edf0f4; display: flex; justify-content: space-between; align-items: center; gap: 18px; }.section-head h3 { font-size: 15px; }.section-head span { display: block; color: #7c8798; margin-top: 3px; font-size: 12px; }.case-name { display: block; border: 0; background: transparent; padding: 0; color: #1677ff; cursor: pointer; font-weight: 600; text-align: left; }.case-name+small { display: block; color: #7c8798; margin-top: 3px; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.replay-head { align-items: flex-start; }.run-controls { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; justify-content: flex-end; }.summary-text { display: block; min-width: 230px; white-space: normal; line-height: 1.5; }.import-tools { display: flex; justify-content: space-between; align-items: center; gap: 16px; margin-bottom: 10px; color: #667085; font-size: 13px; }.json-editor { font-family: Consolas, monospace; font-size: 12px; margin-bottom: 12px; }
@media(max-width:900px){.page-head,.section-head{align-items:flex-start;flex-direction:column}.page-body{padding:16px}.overview{grid-template-columns:repeat(2,1fr)}.overview>div:nth-child(2){border-right:0}.overview .protocol{grid-column:1/3;border-top:1px solid #edf0f4}.run-controls{justify-content:flex-start}.import-tools{align-items:flex-start;flex-direction:column}}
</style>
