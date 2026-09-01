<script setup>
import { computed, onMounted, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { CheckCircleOutlined, ClockCircleOutlined, PlusOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { api, getUser, isAdmin } from '../api'

const rows = ref([])
const loading = ref(false)
const statusFilter = ref('')
const createOpen = ref(false)
const creating = ref(false)
const reviewOpen = ref(false)
const reviewing = ref(false)
const detail = ref(null)
const reviewTarget = ref(null)
const reviewForm = ref({ decision: 'approved', comment: '' })
const form = ref({ product_id: '', action: '', risk: 'high', reason: '', source: 'manual' })
const currentUser = getUser()
const admin = isAdmin()

const visibleRows = computed(() => {
  if (statusFilter.value === 'closed') return rows.value.filter(v => v.status === 'rejected' || v.status === 'cancelled')
  return statusFilter.value ? rows.value.filter(v => v.status === statusFilter.value) : rows.value
})
const count = status => rows.value.filter(v => v.status === status).length
const metrics = computed(() => [
  { label: '待审批', value: count('pending'), tone: 'pending', icon: ClockCircleOutlined },
  { label: '已批准', value: count('approved'), tone: 'approved', icon: SafetyCertificateOutlined },
  { label: '已执行', value: count('executed'), tone: 'executed', icon: CheckCircleOutlined },
  { label: '已驳回/撤销', value: count('rejected') + count('cancelled'), tone: 'closed', icon: SafetyCertificateOutlined },
])

const columns = [
  { title: '编号', dataIndex: 'id', width: 210 },
  { title: '产品', dataIndex: 'product_id', width: 120 },
  { title: '处置动作', dataIndex: 'action', ellipsis: true },
  { title: '风险', key: 'risk', width: 90 },
  { title: '申请人', dataIndex: 'requester_name', width: 110 },
  { title: '状态', key: 'status', width: 105 },
  { title: '申请时间', key: 'created', width: 175 },
  { title: '操作', key: 'ops', width: 210 },
]

const statusText = v => ({ pending: '待审批', approved: '已批准', rejected: '已驳回', executed: '已执行', cancelled: '已撤销' }[v] || v)
const statusColor = v => ({ pending: 'gold', approved: 'blue', rejected: 'red', executed: 'green', cancelled: 'default' }[v] || 'default')
const riskText = v => ({ low: '低风险', medium: '中风险', high: '高风险' }[v] || v)
const riskColor = v => ({ low: 'green', medium: 'gold', high: 'red' }[v] || 'default')
const sourceText = v => ({ diagnosis: '智能诊断', inspection: '主动巡检', manual: '手工申请' }[v] || v)
const canCancel = v => v.status === 'pending' && (admin || (currentUser?.id && currentUser.id === v.requester_id))

async function load() {
  loading.value = true
  try { rows.value = (await api('/approvals')).items || [] }
  catch (e) { message.error(e.message) }
  finally { loading.value = false }
}

async function create() {
  if (!form.value.product_id.trim() || !form.value.action.trim() || !form.value.reason.trim()) {
    message.warning('请完整填写产品、处置动作和申请理由')
    return
  }
  creating.value = true
  try {
    await api('/approvals', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form.value) })
    message.success('审批申请已提交')
    createOpen.value = false
    form.value = { product_id: '', action: '', risk: 'high', reason: '', source: 'manual' }
    await load()
  } catch (e) { message.error(e.message) }
  finally { creating.value = false }
}

function openReview(record, decision) {
  reviewTarget.value = record
  reviewForm.value = { decision, comment: '' }
  reviewOpen.value = true
}

async function review() {
  if (reviewForm.value.decision === 'rejected' && !reviewForm.value.comment.trim()) {
    message.warning('驳回时请填写原因')
    return
  }
  reviewing.value = true
  try {
    await api(`/approvals/${reviewTarget.value.id}/review`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(reviewForm.value) })
    message.success(reviewForm.value.decision === 'approved' ? '审批已通过' : '申请已驳回')
    reviewOpen.value = false
    await load()
  } catch (e) { message.error(e.message) }
  finally { reviewing.value = false }
}

function execute(record) {
  Modal.confirm({
    title: '确认处置已经执行？',
    content: '系统只记录执行结果，不会自动对生产环境执行重启、回滚或扩容操作。',
    okText: '确认已执行', cancelText: '取消',
    async onOk() {
      try {
        await api(`/approvals/${record.id}/execute`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ note: '管理员确认已人工执行' }) })
        message.success('执行结果已记录')
        await load()
      } catch (e) { message.error(e.message); throw e }
    },
  })
}

function cancel(record) {
  Modal.confirm({
    title: '撤销这条审批申请？', okText: '撤销', okType: 'danger', cancelText: '返回',
    async onOk() {
      try {
        await api(`/approvals/${record.id}/cancel`, { method: 'POST' })
        message.success('申请已撤销')
        await load()
      } catch (e) { message.error(e.message); throw e }
    },
  })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div><h2>审批中心</h2><p>高风险处置必须经过人工审核；批准后仍由管理员线下执行并回填结果</p></div>
      <a-button type="primary" @click="createOpen = true"><template #icon><PlusOutlined /></template>新建申请</a-button>
    </header>

    <div class="page-body">
      <section class="metrics">
        <div v-for="m in metrics" :key="m.label" :class="['metric', m.tone]">
          <component :is="m.icon" class="metric-icon" />
          <div><strong>{{ m.value }}</strong><span>{{ m.label }}</span></div>
        </div>
      </section>

      <div class="toolbar">
        <a-segmented v-model:value="statusFilter" :options="[
          { label: '全部', value: '' }, { label: '待审批', value: 'pending' }, { label: '已批准', value: 'approved' },
          { label: '已执行', value: 'executed' }, { label: '已关闭', value: 'closed' },
        ]" />
        <a-button @click="load">刷新</a-button>
      </div>

      <a-table :columns="columns" :data-source="visibleRows" :loading="loading" row-key="id" :pagination="{ pageSize: 12 }" size="middle">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'risk'"><a-tag :color="riskColor(record.risk)">{{ riskText(record.risk) }}</a-tag></template>
          <template v-else-if="column.key === 'status'"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
          <template v-else-if="column.key === 'created'">{{ new Date(record.created_at).toLocaleString() }}</template>
          <template v-else-if="column.key === 'ops'">
            <a-button type="link" size="small" @click="detail = record">详情</a-button>
            <template v-if="admin && record.status === 'pending'">
              <a-button type="link" size="small" @click="openReview(record, 'approved')">批准</a-button>
              <a-button type="link" size="small" danger @click="openReview(record, 'rejected')">驳回</a-button>
            </template>
            <a-button v-if="admin && record.status === 'approved'" type="link" size="small" @click="execute(record)">确认执行</a-button>
            <a-button v-if="canCancel(record)" type="link" size="small" danger @click="cancel(record)">撤销</a-button>
          </template>
        </template>
      </a-table>
    </div>

    <a-modal v-model:open="createOpen" title="新建审批申请" :confirm-loading="creating" ok-text="提交审批" cancel-text="取消" @ok="create">
      <a-form layout="vertical">
        <a-form-item label="产品标识" required><a-input v-model:value="form.product_id" placeholder="例如 payment" /></a-form-item>
        <a-form-item label="处置动作" required><a-input v-model:value="form.action" placeholder="例如回滚最近一次发布" /></a-form-item>
        <a-form-item label="风险级别"><a-segmented v-model:value="form.risk" :options="[{ label: '低', value: 'low' }, { label: '中', value: 'medium' }, { label: '高', value: 'high' }]" /></a-form-item>
        <a-form-item label="申请理由" required><a-textarea v-model:value="form.reason" :rows="4" placeholder="说明故障影响、证据和预期结果" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="reviewOpen" :title="reviewForm.decision === 'approved' ? '批准申请' : '驳回申请'" :confirm-loading="reviewing" :ok-text="reviewForm.decision === 'approved' ? '确认批准' : '确认驳回'" cancel-text="取消" @ok="review">
      <a-alert :type="reviewForm.decision === 'approved' ? 'info' : 'warning'" :message="reviewTarget?.action" show-icon style="margin-bottom:16px" />
      <a-textarea v-model:value="reviewForm.comment" :rows="4" :placeholder="reviewForm.decision === 'rejected' ? '请填写驳回原因（必填）' : '填写审批意见（选填）'" />
    </a-modal>

    <a-drawer :open="!!detail" title="审批详情" width="min(520px, 100vw)" @close="detail = null">
      <a-descriptions v-if="detail" :column="1" bordered size="small">
        <a-descriptions-item label="审批编号">{{ detail.id }}</a-descriptions-item>
        <a-descriptions-item label="产品">{{ detail.product_id }}</a-descriptions-item>
        <a-descriptions-item label="处置动作">{{ detail.action }}</a-descriptions-item>
        <a-descriptions-item label="风险"><a-tag :color="riskColor(detail.risk)">{{ riskText(detail.risk) }}</a-tag></a-descriptions-item>
        <a-descriptions-item label="状态"><a-tag :color="statusColor(detail.status)">{{ statusText(detail.status) }}</a-tag></a-descriptions-item>
        <a-descriptions-item label="来源">{{ sourceText(detail.source) }}</a-descriptions-item>
        <a-descriptions-item label="申请人">{{ detail.requester_name || '-' }}</a-descriptions-item>
        <a-descriptions-item label="申请理由">{{ detail.reason }}</a-descriptions-item>
        <a-descriptions-item label="审批人">{{ detail.reviewer_name || '-' }}</a-descriptions-item>
        <a-descriptions-item label="审批意见">{{ detail.review_comment || '-' }}</a-descriptions-item>
        <a-descriptions-item label="执行人">{{ detail.executor_name || '-' }}</a-descriptions-item>
        <a-descriptions-item label="执行记录">{{ detail.execution_note || '-' }}</a-descriptions-item>
      </a-descriptions>
    </a-drawer>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { min-height: 78px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #edeff2; display: flex; justify-content: space-between; align-items: center; gap: 20px; }
.page-head h2 { margin: 0; font-size: 18px; }
.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(150px, 1fr)); background: #fff; border: 1px solid #e8ebef; border-radius: 8px; margin-bottom: 18px; }
.metric { min-height: 92px; padding: 18px 20px; display: flex; align-items: center; gap: 14px; border-right: 1px solid #edf0f3; }
.metric:last-child { border-right: 0; }
.metric-icon { font-size: 24px; color: #1677ff; }
.metric.pending .metric-icon { color: #d48806; }
.metric.executed .metric-icon { color: #389e0d; }
.metric.closed .metric-icon { color: #8c8c8c; }
.metric div { display: flex; flex-direction: column; }
.metric strong { font-size: 24px; color: #1f2733; line-height: 1.1; }
.metric span { margin-top: 5px; color: #667085; font-size: 12px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; gap: 12px; }
@media (max-width: 900px) {
  .metrics { grid-template-columns: repeat(2, 1fr); }
  .metric:nth-child(2) { border-right: 0; }
  .metric:nth-child(-n+2) { border-bottom: 1px solid #edf0f3; }
  .page-head, .toolbar { align-items: flex-start; flex-direction: column; }
  .page-body { padding: 16px; }
}
</style>
