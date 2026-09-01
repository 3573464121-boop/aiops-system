<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { DownloadOutlined, ExperimentOutlined } from '@ant-design/icons-vue'
import { api } from '../api'

const rows = ref([])
const loading = ref(false)
const filter = ref('all')
const detail = ref(null)
const reviewOpen = ref(false)
const reviewing = ref(false)
const reviewTarget = ref(null)
const reviewForm = ref({ included: true, gold_cause: '', note: '' })

const visibleRows = computed(() => filter.value === 'included' ? rows.value.filter(v => v.included) : filter.value === 'pending' ? rows.value.filter(v => !v.reviewed_at || v.reviewed_at.startsWith('0001')) : rows.value)
const includedRows = computed(() => rows.value.filter(v => v.included))
const avgConfidence = computed(() => rows.value.length ? rows.value.reduce((n, v) => n + v.confidence, 0) / rows.value.length : 0)
const avgDuration = computed(() => rows.value.length ? rows.value.reduce((n, v) => n + v.duration_ms, 0) / rows.value.length : 0)
const knowledgeRate = computed(() => rows.value.length ? rows.value.filter(v => v.knowledge_hit).length / rows.value.length : 0)

const columns = [
  { title: '时间', key: 'created', width: 175 },
  { title: '产品', dataIndex: 'product_id', width: 110 },
  { title: '诊断问题', dataIndex: 'question', ellipsis: true },
  { title: '模式', dataIndex: 'mode', width: 105 },
  { title: '置信度', key: 'confidence', width: 90 },
  { title: '证据', dataIndex: 'evidence_count', width: 70 },
  { title: '工具', dataIndex: 'tool_call_count', width: 70 },
  { title: '耗时', key: 'duration', width: 95 },
  { title: '数据集', key: 'included', width: 90 },
  { title: '操作', key: 'ops', width: 125 },
]

async function load() {
  loading.value = true
  try { rows.value = (await api('/diagnosis-runs?limit=1000')).items || [] }
  catch (e) { message.error(e.message) }
  finally { loading.value = false }
}
function openReview(record) {
  reviewTarget.value = record
  reviewForm.value = { included: record.included, gold_cause: record.gold_cause || '', note: record.reviewer_note || '' }
  reviewOpen.value = true
}
async function review() {
  if (reviewForm.value.included && !reviewForm.value.gold_cause.trim()) {
    message.warning('纳入数据集时必须填写标准根因')
    return
  }
  reviewing.value = true
  try {
    const result = await api(`/diagnosis-runs/${reviewTarget.value.id}/review`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(reviewForm.value) })
    Object.assign(reviewTarget.value, result)
    message.success(result.included ? '已纳入论文数据集' : '已保存复核结果')
    reviewOpen.value = false
  } catch (e) { message.error(e.message) }
  finally { reviewing.value = false }
}
function exportDataset() {
  if (!includedRows.value.length) { message.warning('还没有已复核并纳入的数据'); return }
  const dataset = includedRows.value.map(v => ({
    id: v.id, product: v.product_id, question: v.question, gold_cause: v.gold_cause,
    diagnosis: v.summary, confidence: v.confidence, mode: v.mode, model: v.model,
    evidence_sources: v.evidence_sources, tools: v.tools, evidence_count: v.evidence_count,
    tool_call_count: v.tool_call_count, failed_tool_count: v.failed_tool_count,
    knowledge_hit: v.knowledge_hit, memory_hit: v.memory_hit, asset_hit: v.asset_hit,
    duration_ms: v.duration_ms, alert_provider: v.alert_provider, log_provider: v.log_provider,
    knowledge_mode: v.knowledge_mode, reviewed_by: v.reviewed_by,
  }))
  const blob = new Blob([JSON.stringify(dataset, null, 2)], { type: 'application/json;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `aiops-reviewed-dataset-${new Date().toISOString().slice(0, 10)}.json`
  link.click()
  URL.revokeObjectURL(link.href)
}
const fmtPercent = v => `${Math.round(v * 100)}%`
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div><h2>实验记录</h2><p>自动沉淀诊断运行指标，并由人工复核后形成可导出的论文数据集</p></div>
      <a-button type="primary" :disabled="!includedRows.length" @click="exportDataset"><template #icon><DownloadOutlined /></template>导出已复核数据</a-button>
    </header>
    <div class="page-body">
      <section class="metrics">
        <div><ExperimentOutlined /><strong>{{ rows.length }}</strong><span>诊断运行</span></div>
        <div><strong>{{ includedRows.length }}</strong><span>已纳入数据集</span></div>
        <div><strong>{{ fmtPercent(avgConfidence) }}</strong><span>平均置信度</span></div>
        <div><strong>{{ (avgDuration / 1000).toFixed(1) }}s</strong><span>平均耗时</span></div>
        <div><strong>{{ fmtPercent(knowledgeRate) }}</strong><span>知识命中率</span></div>
      </section>
      <div class="toolbar">
        <a-segmented v-model:value="filter" :options="[{ label: '全部', value: 'all' }, { label: '待复核', value: 'pending' }, { label: '已纳入', value: 'included' }]" />
        <a-button @click="load">刷新</a-button>
      </div>
      <a-table :columns="columns" :data-source="visibleRows" :loading="loading" row-key="id" :pagination="{ pageSize: 15 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'created'">{{ new Date(record.created_at).toLocaleString() }}</template>
          <template v-else-if="column.key === 'confidence'">{{ fmtPercent(record.confidence) }}</template>
          <template v-else-if="column.key === 'duration'">{{ (record.duration_ms / 1000).toFixed(1) }}s</template>
          <template v-else-if="column.key === 'included'"><a-tag :color="record.included ? 'green' : 'default'">{{ record.included ? '已纳入' : '未纳入' }}</a-tag></template>
          <template v-else-if="column.key === 'ops'"><a-button type="link" size="small" @click="detail = record">详情</a-button><a-button type="link" size="small" @click="openReview(record)">复核</a-button></template>
        </template>
      </a-table>
    </div>

    <a-modal v-model:open="reviewOpen" title="复核诊断记录" :confirm-loading="reviewing" ok-text="保存复核" cancel-text="取消" @ok="review">
      <a-form layout="vertical">
        <a-form-item><a-checkbox v-model:checked="reviewForm.included">纳入论文实验数据集</a-checkbox></a-form-item>
        <a-form-item label="标准根因" :required="reviewForm.included"><a-textarea v-model:value="reviewForm.gold_cause" :rows="3" placeholder="填写人工确认的核心故障机理" /></a-form-item>
        <a-form-item label="复核备注"><a-textarea v-model:value="reviewForm.note" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <a-drawer :open="!!detail" title="诊断实验详情" width="min(600px, 100vw)" @close="detail = null">
      <a-descriptions v-if="detail" :column="1" bordered size="small">
        <a-descriptions-item label="运行编号">{{ detail.id }}</a-descriptions-item>
        <a-descriptions-item label="问题">{{ detail.question }}</a-descriptions-item>
        <a-descriptions-item label="诊断结论">{{ detail.summary }}</a-descriptions-item>
        <a-descriptions-item label="模型/模式">{{ detail.model || '-' }} / {{ detail.mode }}</a-descriptions-item>
        <a-descriptions-item label="数据配置">{{ detail.alert_provider }} · {{ detail.log_provider }} · {{ detail.knowledge_mode }}</a-descriptions-item>
        <a-descriptions-item label="工具调用">{{ detail.tools.join('、') || '-' }}</a-descriptions-item>
        <a-descriptions-item label="证据来源"><div v-for="s in detail.evidence_sources" :key="s"><code>{{ s }}</code></div></a-descriptions-item>
        <a-descriptions-item label="命中能力"><a-space><a-tag :color="detail.knowledge_hit ? 'green' : 'default'">知识</a-tag><a-tag :color="detail.memory_hit ? 'green' : 'default'">记忆</a-tag><a-tag :color="detail.asset_hit ? 'green' : 'default'">资产</a-tag></a-space></a-descriptions-item>
        <a-descriptions-item label="标准根因">{{ detail.gold_cause || '尚未复核' }}</a-descriptions-item>
      </a-descriptions>
    </a-drawer>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }.page-head { min-height: 78px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #edeff2; display: flex; justify-content: space-between; align-items: center; gap: 20px; }.page-head h2 { margin: 0; font-size: 18px; }.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }.page-body { padding: 22px 26px; }
.metrics { display: grid; grid-template-columns: repeat(5, 1fr); background: #fff; border: 1px solid #e8ebef; border-radius: 8px; margin-bottom: 18px; }.metrics > div { min-height: 90px; padding: 17px 20px; display: flex; flex-direction: column; justify-content: center; border-right: 1px solid #edf0f3; }.metrics > div:last-child { border-right: 0; }.metrics :deep(.anticon) { color: #1677ff; font-size: 20px; margin-bottom: 3px; }.metrics strong { color: #1f2733; font-size: 23px; line-height: 1.2; }.metrics span { color: #667085; font-size: 12px; margin-top: 4px; }.toolbar { display: flex; justify-content: space-between; margin-bottom: 12px; }
code { color: #475467; font-size: 12px; overflow-wrap: anywhere; } @media (max-width: 900px) { .metrics { grid-template-columns: repeat(2, 1fr); }.metrics > div { border-bottom: 1px solid #edf0f3; }.page-head { align-items: flex-start; flex-direction: column; }.page-body { padding: 16px; } }
</style>
