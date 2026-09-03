<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { DeleteOutlined, DownloadOutlined, ExperimentOutlined, FileAddOutlined, PlayCircleOutlined, ReloadOutlined, UploadOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { api } from '../api'

const cases = ref([])
const results = ref([])
const batches = ref([])
const selected = ref(null)
const selectedBatchID = ref('')
const loading = ref(false)
const running = ref(false)
const importOpen = ref(false)
const importing = ref(false)
const reviewOpen = ref(false)
const reviewing = ref(false)
const reviewTarget = ref(null)
const reviewForm = ref({ accepted: true, cause_ok: true, note: '' })
const configs = ref(['full', 'bm25', 'no-agent'])
const jsonText = ref('')
const fileInput = ref(null)
const batchOpen = ref(false)
const batchCreating = ref(false)
const batchForm = ref({ name: '', case_ids: [], configs: ['full', 'bm25', 'no-agent'], repeats: 1 })
let pollTimer

const columns = [
  { title: '案例', key: 'name' }, { title: '产品', dataIndex: 'product_id', width: 110 },
  { title: '版本', dataIndex: 'version', width: 80 }, { title: '来源', key: 'source', width: 100 },
  { title: '证据', key: 'evidence', width: 110 }, { title: '最近导入', key: 'created', width: 175 },
  { title: '操作', key: 'ops', width: 155 },
]
const resultColumns = [
	{ title: '复核', key: 'review', width: 95 },
  { title: '质量', key: 'quality', width: 95 },
  { title: '轮次', key: 'trial', width: 70 },
  { title: '配置', key: 'config', width: 120 }, { title: '根因正确', key: 'cause', width: 105 },
  { title: '忠实度', key: 'faith', width: 95 }, { title: '幻觉', key: 'hallucination', width: 85 },
  { title: '证据数', key: 'evidence', width: 80 }, { title: '工具失败', dataIndex: 'tool_failures', width: 95 },
  { title: '耗时', key: 'duration', width: 95 }, { title: '判官', key: 'judge', width: 100 }, { title: '诊断结论', key: 'summary' },
]
const batchColumns = [
  { title: '批次', key: 'name' }, { title: '状态', key: 'status', width: 105 },
  { title: '进度', key: 'progress', width: 190 }, { title: '案例 / 配置 / 重复', key: 'scope', width: 170 },
  { title: '模型 / 判官', key: 'models', width: 210 }, { title: '创建时间', key: 'created', width: 175 },
]

const filteredResults = computed(() => selectedBatchID.value ? results.value.filter(v => v.batch_id === selectedBatchID.value) : results.value)
const selectedResults = computed(() => selected.value ? filteredResults.value.filter(v => v.case_id === selected.value.id) : [])
const latestResults = computed(() => {
  const seen = new Set()
  return selectedResults.value.filter(v => {
    const key = `${v.config}:${v.trial || 1}`
    return seen.has(key) ? false : (seen.add(key), true)
  })
})
const judged = computed(() => filteredResults.value.filter(v => v.judged))
const correctRate = computed(() => judged.value.length ? judged.value.filter(v => v.cause_correct).length / judged.value.length : 0)
const acceptedRows = computed(() => filteredResults.value.filter(v => v.review_status === 'accepted'))
const warningCount = computed(() => filteredResults.value.filter(v => v.quality_status === 'warning').length)
const sourceText = v => ({ real: '真实脱敏', synthetic: '合成', imported: '导入' }[v] || v)
const configText = v => ({ full: '完整系统', bm25: '仅 BM25', 'no-agent': '无 Agent' }[v] || v)
const pct = v => `${Math.round((v || 0) * 100)}%`
const qualityIssueText = v => ({
  judge_unavailable: '判官未完成评估', faithfulness_without_evidence: '无证据但忠实度大于零',
  zero_faithfulness_without_hallucination: '忠实度为零但未判幻觉', correct_cause_without_evidence: '无证据但根因判定正确',
}[v] || v)
const batchStatusText = v => ({ pending: '等待中', running: '运行中', completed: '已完成', failed: '失败' }[v] || v)
const batchStatusColor = v => ({ pending: 'default', running: 'processing', completed: 'success', failed: 'error' }[v] || 'default')

async function load() {
  loading.value = true
  try {
    const [a, b, c] = await Promise.all([api('/fault-cases'), api('/replay-results?limit=5000'), api('/experiment-batches?limit=200')])
    cases.value = a.items || []
    results.value = b.items || []
    batches.value = c.items || []
    if (selected.value) selected.value = cases.value.find(v => v.id === selected.value.id) || null
  } catch (e) { message.error(e.message) }
  finally { loading.value = false }
}

function openBatch() {
  batchForm.value = { name: `对照实验 ${new Date().toLocaleDateString()}`, case_ids: cases.value.map(v => v.id), configs: ['full', 'bm25', 'no-agent'], repeats: 1 }
  batchOpen.value = true
}
async function createBatch() {
  if (!batchForm.value.name.trim() || !batchForm.value.case_ids.length || !batchForm.value.configs.length) {
    message.warning('请填写批次名称并选择案例和配置')
    return
  }
  batchCreating.value = true
  try {
    const created = await api('/experiment-batches', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(batchForm.value) })
    batchOpen.value = false
    selectedBatchID.value = created.id
    message.success(`实验批次已创建，共 ${created.total_runs} 次运行`)
    await load()
  } catch (e) { message.error(e.message) }
  finally { batchCreating.value = false }
}
function selectBatch(record) {
  selectedBatchID.value = selectedBatchID.value === record.id ? '' : record.id
  selected.value = null
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
    const bulk = Array.isArray(payload)
    const created = await api(bulk ? '/fault-cases/bulk' : '/fault-cases', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    message.success(`已导入 ${created.total || 1} 条故障案例`)
    importOpen.value = false
    await load()
    selected.value = bulk ? null : created
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
function openReview(record) {
  reviewTarget.value = record
  reviewForm.value = { accepted: record.review_status === 'accepted', cause_ok: record.review_cause ?? record.cause_correct, note: record.review_note || '' }
  reviewOpen.value = true
}
async function review() {
  if (!reviewForm.value.note.trim()) { message.warning('请填写人工复核说明'); return }
  reviewing.value = true
  try {
    const updated = await api(`/replay-results/${reviewTarget.value.id}/review`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(reviewForm.value) })
    const index = results.value.findIndex(v => v.id === updated.id)
    if (index >= 0) results.value[index] = updated
    reviewOpen.value = false
    message.success('人工复核已保存')
  } catch (e) { message.error(e.message) }
  finally { reviewing.value = false }
}
function exportRows() {
  return acceptedRows.value.map(v => {
    const c = cases.value.find(item => item.id === v.case_id) || {}
    return {
      result_id: v.id, case_id: v.case_id, case_name: c.name || '', case_version: c.version || '',
      case_source: c.source || '', product_id: c.product_id || v.diagnosis.product_id || '', question: c.question || v.diagnosis.question || '',
      gold_cause: c.gold_cause || '', config: v.config, model: v.model, judge_model: v.judge_model,
      judge_source: v.judge_source, cause_correct: v.cause_correct, faithfulness: v.faithfulness,
      hallucination: v.hallucination, quality_status: v.quality_status, quality_issues: v.quality_issues || [],
      evidence_count: v.diagnosis.evidence.length, evidence_sources: v.diagnosis.evidence.map(e => e.source).filter(Boolean),
      tool_failures: v.tool_failures, duration_ms: v.duration_ms, summary: v.diagnosis.summary,
      review_cause: v.review_cause, review_note: v.review_note, reviewed_by: v.reviewed_by,
      reviewed_at: v.reviewed_at, created_at: v.created_at, diagnosis: v.diagnosis,
    }
  })
}
function download(content, type, extension) {
  const blob = new Blob([content], { type })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `aiops-replay-reviewed-${new Date().toISOString().slice(0, 10)}.${extension}`
  link.click()
  URL.revokeObjectURL(link.href)
}
function exportJSON() {
  if (!acceptedRows.value.length) { message.warning('没有已采纳的人工复核结果'); return }
  download(JSON.stringify({ exported_at: new Date().toISOString(), total: acceptedRows.value.length, items: exportRows() }, null, 2), 'application/json;charset=utf-8', 'json')
}
function csvCell(value) {
  const text = Array.isArray(value) ? value.join(';') : String(value ?? '')
  return `"${text.replaceAll('"', '""')}"`
}
function exportCSV() {
  if (!acceptedRows.value.length) { message.warning('没有已采纳的人工复核结果'); return }
  const rows = exportRows().map(({ diagnosis, ...v }) => v)
  const headers = Object.keys(rows[0])
  const csv = '\ufeff' + [headers.map(csvCell).join(','), ...rows.map(v => headers.map(k => csvCell(v[k])).join(','))].join('\r\n')
  download(csv, 'text/csv;charset=utf-8', 'csv')
}
onMounted(() => {
  load()
  pollTimer = window.setInterval(() => {
    if (batches.value.some(v => v.status === 'pending' || v.status === 'running')) load()
  }, 5000)
})
onUnmounted(() => window.clearInterval(pollTimer))
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div><h2>故障回放</h2><p>用固定案例对照 Agent、检索策略与无证据基线，为实验复现保留完整记录</p></div>
      <a-space wrap><a-button :disabled="!acceptedRows.length" @click="exportJSON"><DownloadOutlined />JSON</a-button><a-button :disabled="!acceptedRows.length" @click="exportCSV"><DownloadOutlined />CSV</a-button><a-button @click="load"><ReloadOutlined />刷新</a-button><a-button @click="openImport"><FileAddOutlined />导入案例</a-button><a-button type="primary" @click="openBatch"><ExperimentOutlined />新建实验批次</a-button></a-space>
    </header>
    <div class="page-body">
      <section class="overview">
        <div><strong>{{ cases.length }}</strong><span>故障案例</span></div><div><strong>{{ batches.length }}</strong><span>实验批次</span></div><div><strong>{{ filteredResults.length }}</strong><span>回放结果</span></div>
        <div><strong>{{ acceptedRows.length }}</strong><span>人工复核采纳</span></div><div><strong>{{ warningCount }}</strong><span>质量告警</span></div>
        <div><strong>{{ pct(correctRate) }}</strong><span>机器判官准确率</span></div><div class="protocol"><b>对照配置</b><span>完整系统 / 仅 BM25 / 无 Agent</span></div>
      </section>
      <section class="panel">
        <div class="section-head"><div><h3>实验批次</h3><span>后台串行运行；点击批次可筛选案例和结果，再次点击取消筛选</span></div><a-tag v-if="selectedBatchID" closable @close="selectedBatchID = ''">已筛选批次</a-tag></div>
        <a-table :columns="batchColumns" :data-source="batches" row-key="id" :pagination="{ pageSize: 6 }" size="middle" :custom-row="record => ({ onClick: () => selectBatch(record) })" :row-class-name="record => selectedBatchID === record.id ? 'selected-row' : 'batch-row'">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'"><strong>{{ record.name }}</strong><small class="batch-id">{{ record.id }}</small></template>
            <template v-else-if="column.key === 'status'"><a-tag :color="batchStatusColor(record.status)">{{ batchStatusText(record.status) }}</a-tag><small v-if="record.failed_runs" class="failed">{{ record.failed_runs }} 次失败</small></template>
            <template v-else-if="column.key === 'progress'"><a-progress :percent="record.total_runs ? Math.round(record.completed_runs / record.total_runs * 100) : 0" :status="record.status === 'failed' ? 'exception' : record.status === 'completed' ? 'success' : 'active'" size="small" /><small>{{ record.completed_runs }} / {{ record.total_runs }}</small></template>
            <template v-else-if="column.key === 'scope'">{{ record.case_ids.length }} / {{ record.configs.length }} / {{ record.repeats }}</template>
            <template v-else-if="column.key === 'models'"><span>{{ record.model || '-' }}</span><small>{{ record.judge_source === 'independent' ? '独立判官' : '自评' }}：{{ record.judge_model || '-' }}</small></template>
            <template v-else-if="column.key === 'created'">{{ new Date(record.created_at).toLocaleString() }}</template>
          </template>
          <template #emptyText>尚未创建实验批次</template>
        </a-table>
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
            <template v-else-if="column.key === 'trial'">{{ record.trial || 1 }}</template>
            <template v-else-if="column.key === 'cause'"><a-tag :color="record.judged && record.cause_correct ? 'green' : 'red'">{{ record.judged ? (record.cause_correct ? '正确' : '错误') : '未评审' }}</a-tag></template>
            <template v-else-if="column.key === 'faith'">{{ record.judged ? pct(record.faithfulness) : '-' }}</template>
            <template v-else-if="column.key === 'hallucination'"><a-tag :color="record.hallucination ? 'red' : 'green'">{{ record.judged ? (record.hallucination ? '有' : '无') : '-' }}</a-tag></template>
            <template v-else-if="column.key === 'evidence'">{{ record.diagnosis.evidence.length }}</template>
            <template v-else-if="column.key === 'duration'">{{ (record.duration_ms / 1000).toFixed(1) }}s</template>
            <template v-else-if="column.key === 'judge'"><a-tag :color="record.judge_source === 'independent' ? 'green' : 'gold'">{{ record.judge_source === 'independent' ? '独立' : '自评' }}</a-tag></template>
            <template v-else-if="column.key === 'review'"><a-button type="link" size="small" @click="openReview(record)">{{ record.review_status === 'accepted' ? '已采纳' : record.review_status === 'rejected' ? '已驳回' : '复核' }}</a-button></template>
            <template v-else-if="column.key === 'quality'"><a-tooltip :title="(record.quality_issues || []).map(qualityIssueText).join('；') || '未发现指标冲突'"><a-tag :color="record.quality_status === 'warning' ? 'orange' : 'green'"><WarningOutlined v-if="record.quality_status === 'warning'" />{{ record.quality_status === 'warning' ? '需检查' : '通过' }}</a-tag></a-tooltip></template>
            <template v-else-if="column.key === 'summary'"><span class="summary-text">{{ record.diagnosis.summary }}</span></template>
          </template>
          <template #emptyText>选择配置并运行后显示对照结果</template>
        </a-table>
      </section>
    </div>
    <a-modal v-model:open="batchOpen" title="新建实验批次" width="min(680px, 96vw)" :confirm-loading="batchCreating" ok-text="创建并开始" cancel-text="取消" @ok="createBatch">
      <a-alert type="info" show-icon message="批次将在后台串行执行，关闭页面不会中断；每条结果仍需后续人工复核。" style="margin-bottom: 16px" />
      <a-form layout="vertical">
        <a-form-item label="批次名称" required><a-input v-model:value="batchForm.name" maxlength="120" /></a-form-item>
        <a-form-item label="故障案例" required><a-select v-model:value="batchForm.case_ids" mode="multiple" show-search :max-tag-count="4" placeholder="选择 1 到 100 个案例" :options="cases.map(v => ({ label: `${v.name} (${v.version})`, value: v.id }))" /></a-form-item>
        <a-form-item label="对照配置" required><a-checkbox-group v-model:value="batchForm.configs" :options="[{ label: '完整系统', value: 'full' }, { label: '仅 BM25', value: 'bm25' }, { label: '无 Agent', value: 'no-agent' }]" /></a-form-item>
        <a-form-item label="重复次数"><a-input-number v-model:value="batchForm.repeats" :min="1" :max="10" /><span class="form-note">总运行次数：{{ batchForm.case_ids.length * batchForm.configs.length * batchForm.repeats }}</span></a-form-item>
      </a-form>
    </a-modal>
    <a-modal v-model:open="importOpen" title="导入故障案例" width="min(760px, 96vw)" :confirm-loading="importing" ok-text="校验并导入" cancel-text="取消" @ok="importCase">
      <div class="import-tools"><span>粘贴案例 JSON，或选择 UTF-8 JSON 文件。</span><a-button @click="fileInput?.click()"><UploadOutlined />选择文件</a-button><input ref="fileInput" type="file" accept="application/json,.json" hidden @change="readFile" /></div>
      <a-textarea v-model:value="jsonText" :rows="20" class="json-editor" spellcheck="false" />
      <a-alert type="warning" show-icon message="真实案例必须移除账号、客户名称、公网地址和密钥等敏感信息。" />
    </a-modal>
    <a-modal v-model:open="reviewOpen" title="人工复核回放结果" :confirm-loading="reviewing" ok-text="保存复核" cancel-text="取消" @ok="review">
      <a-alert type="info" show-icon :message="reviewTarget?.diagnosis?.summary" style="margin-bottom: 16px" />
      <a-form layout="vertical">
        <a-form-item label="是否纳入论文数据集"><a-switch v-model:checked="reviewForm.accepted" checked-children="纳入" un-checked-children="不纳入" /></a-form-item>
        <a-form-item label="人工确认根因是否正确"><a-radio-group v-model:value="reviewForm.cause_ok"><a-radio :value="true">正确</a-radio><a-radio :value="false">不正确</a-radio></a-radio-group></a-form-item>
        <a-form-item label="复核说明" required><a-textarea v-model:value="reviewForm.note" :rows="4" placeholder="说明判断依据、与标准根因的关系以及是否存在证据不足" /></a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; color: #202938; }.page-head { min-height: 78px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #e7eaf0; display: flex; justify-content: space-between; align-items: center; gap: 20px; }.page-head h2,.section-head h3 { margin: 0; letter-spacing: 0; }.page-head h2 { font-size: 18px; }.page-head p { margin: 4px 0 0; color: #7c8798; font-size: 13px; }.page-body { padding: 22px 26px 40px; }
.overview { display: grid; grid-template-columns: repeat(6,minmax(110px,145px)) 1fr; min-height: 86px; background: #fff; border: 1px solid #e2e7ee; border-radius: 8px; margin-bottom: 18px; }.overview>div { padding: 16px 18px; display: flex; flex-direction: column; justify-content: center; border-right: 1px solid #edf0f4; }.overview strong { font-size: 23px; line-height: 1.15; }.overview span { margin-top: 4px; color: #667085; font-size: 12px; }.overview .protocol { border-right: 0; }.protocol b { font-size: 13px; }
.panel { background: #fff; border: 1px solid #e2e7ee; border-radius: 8px; margin-bottom: 18px; overflow: hidden; }.section-head { padding: 15px 18px; border-bottom: 1px solid #edf0f4; display: flex; justify-content: space-between; align-items: center; gap: 18px; }.section-head h3 { font-size: 15px; }.section-head span { display: block; color: #7c8798; margin-top: 3px; font-size: 12px; }.case-name { display: block; border: 0; background: transparent; padding: 0; color: #1677ff; cursor: pointer; font-weight: 600; text-align: left; }.case-name+small { display: block; color: #7c8798; margin-top: 3px; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.replay-head { align-items: flex-start; }.run-controls { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; justify-content: flex-end; }.summary-text { display: block; min-width: 230px; white-space: normal; line-height: 1.5; }.import-tools { display: flex; justify-content: space-between; align-items: center; gap: 16px; margin-bottom: 10px; color: #667085; font-size: 13px; }.json-editor { font-family: Consolas, monospace; font-size: 12px; margin-bottom: 12px; }.batch-row { cursor: pointer; }.batch-id,.batch-row small { display: block; color: #8a94a3; margin-top: 3px; }.failed { color: #d4380d !important; }.form-note { margin-left: 12px; color: #7c8798; font-size: 12px; }
:deep(.selected-row) td { background: #eaf3ff !important; cursor: pointer; }
@media(max-width:1100px){.overview{grid-template-columns:repeat(3,1fr)}.overview .protocol{grid-column:1/4;border-top:1px solid #edf0f4}}@media(max-width:900px){.page-head,.section-head{align-items:flex-start;flex-direction:column}.page-body{padding:16px}.overview{grid-template-columns:repeat(2,1fr)}.overview .protocol{grid-column:1/3;border-top:1px solid #edf0f4}.run-controls{justify-content:flex-start}.import-tools{align-items:flex-start;flex-direction:column}}
</style>
