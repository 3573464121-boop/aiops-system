<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { CheckCircleOutlined, ReloadOutlined, RobotOutlined, RollbackOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { api, isAdmin, sevColor, sevText } from '../api'

const product = ref('')
const status = ref('')
const rows = ref([])
const metrics = ref({ raw_signals: 0, event_count: 0, open_count: 0, resolved_count: 0, reduction_rate: 0 })
const loading = ref(false)
const syncing = ref(false)
const diagnosingID = ref('')
const diagnosisOpen = ref(false)
const diagnosis = ref(null)
const selectedEvent = ref(null)
const admin = isAdmin()
const compact = ref(window.innerWidth <= 720)
const updateCompact = () => { compact.value = window.innerWidth <= 720 }

const statusOptions = [
  { label: '全部', value: '' },
  { label: '处理中', value: 'open' },
  { label: '已恢复', value: 'resolved' },
]
const columns = [
  { title: '级别', key: 'severity', width: 82 },
  { title: '告警事件', key: 'event', width: 260 },
  { title: '产品 / 对象', key: 'target', width: 190 },
  { title: '当前值', dataIndex: 'value', width: 145 },
  { title: '信号数', key: 'occurrences', width: 90, align: 'center' },
  { title: '状态', key: 'status', width: 94 },
  { title: '首次 / 最近出现', key: 'time', width: 180 },
  { title: '诊断摘要', key: 'diagnosis', width: 270 },
  { title: '操作', key: 'ops', width: 150, fixed: 'right' },
]

const formatTime = value => {
  if (!value || value.startsWith?.('0001')) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}
const percent = value => `${Math.round((value || 0) * 100)}%`
const statusText = value => value === 'resolved' ? '已恢复' : '处理中'
const statusColor = value => value === 'resolved' ? 'success' : 'error'

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({ limit: '200' })
    if (product.value.trim()) query.set('product_id', product.value.trim())
    if (status.value) query.set('status', status.value)
    const data = await api(`/alert-events?${query}`)
    rows.value = data.items || []
    metrics.value = data.metrics || metrics.value
  } catch (e) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function syncProvider() {
  syncing.value = true
  try {
    const result = await api('/alert-events/sync', { method: 'POST' })
    message.success(`已接收 ${result.received} 条信号，新建 ${result.created} 个事件，聚合 ${result.merged} 条`)
    await load()
  } catch (e) {
    message.error(e.message)
  } finally {
    syncing.value = false
  }
}

async function diagnoseEvent(record) {
  diagnosingID.value = record.id
  try {
    const result = await api(`/alert-events/${record.id}/diagnose`, { method: 'POST' })
    selectedEvent.value = result.event
    diagnosis.value = result.diagnosis
    diagnosisOpen.value = true
    await load()
  } catch (e) {
    message.error(e.message)
  } finally {
    diagnosingID.value = ''
  }
}

async function resolveEvent(record) {
  try {
    await api(`/alert-events/${record.id}/resolve`, { method: 'POST' })
    message.success('事件已标记为恢复')
    await load()
  } catch (e) {
    message.error(e.message)
  }
}

async function reopenEvent(record) {
  try {
    await api(`/alert-events/${record.id}/reopen`, { method: 'POST' })
    message.success('事件已重新打开')
    await load()
  } catch (e) {
    message.error(e.message)
  }
}

onMounted(() => {
  window.addEventListener('resize', updateCompact)
  load()
})
onUnmounted(() => window.removeEventListener('resize', updateCompact))
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h2>告警事件中心</h2>
        <p>将重复告警归并为可跟踪事件，保留原始信号数量与诊断记录</p>
      </div>
      <a-space>
        <a-button @click="load"><ReloadOutlined />刷新</a-button>
        <a-button v-if="admin" type="primary" :loading="syncing" @click="syncProvider"><SyncOutlined />同步当前告警</a-button>
      </a-space>
    </header>

    <div class="page-body">
      <section class="metrics">
        <div class="metric"><span>原始信号</span><strong>{{ metrics.raw_signals }}</strong><small>接收总次数</small></div>
        <div class="metric"><span>聚合事件</span><strong>{{ metrics.event_count }}</strong><small>唯一指纹数</small></div>
        <div class="metric"><span>处理中</span><strong class="danger">{{ metrics.open_count }}</strong><small>尚未恢复</small></div>
        <div class="metric"><span>已恢复</span><strong class="success">{{ metrics.resolved_count }}</strong><small>已关闭事件</small></div>
        <div class="metric emphasized"><span>降噪率</span><strong>{{ percent(metrics.reduction_rate) }}</strong><small>重复信号压缩比例</small></div>
      </section>

      <section class="event-panel">
        <div class="toolbar">
          <a-segmented v-model:value="status" :options="statusOptions" @change="load" />
          <a-input v-model:value="product" allow-clear placeholder="按产品标识筛选" class="product-filter" @pressEnter="load" @change="!product && load()" />
          <a-button @click="load">查询</a-button>
          <span class="result-count">当前显示 {{ rows.length }} 个事件</span>
        </div>

        <a-table
          v-if="!compact"
          :columns="columns"
          :data-source="rows"
          :loading="loading"
          :scroll="{ x: 1460 }"
          :pagination="{ pageSize: 12, showSizeChanger: false }"
          row-key="id"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'severity'">
              <a-tag :color="sevColor(record.severity)">{{ sevText(record.severity) }}</a-tag>
            </template>
            <template v-else-if="column.key === 'event'">
              <strong class="rule-name">{{ record.rule }}</strong>
              <span class="source">{{ record.source }}<template v-if="record.external_id"> · {{ record.external_id }}</template></span>
            </template>
            <template v-else-if="column.key === 'target'">
              <a-tag>{{ record.product_id }}</a-tag>
              <span class="target-name">{{ record.target }}</span>
            </template>
            <template v-else-if="column.key === 'occurrences'">
              <span :class="['count', { repeated: record.occurrences > 1 }]">{{ record.occurrences }}</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-badge :status="statusColor(record.status)" :text="statusText(record.status)" />
            </template>
            <template v-else-if="column.key === 'time'">
              <span class="time">{{ formatTime(record.first_seen_at) }}</span>
              <span class="time muted">{{ formatTime(record.last_seen_at) }}</span>
            </template>
            <template v-else-if="column.key === 'diagnosis'">
              <span v-if="record.diagnosis_summary" class="diagnosis-summary">{{ record.diagnosis_summary }}</span>
              <span v-else class="empty">尚未诊断</span>
            </template>
            <template v-else-if="column.key === 'ops'">
              <a-button type="link" size="small" :loading="diagnosingID === record.id" @click="diagnoseEvent(record)"><RobotOutlined />诊断</a-button>
              <a-popconfirm v-if="admin && record.status === 'open'" title="确认该告警事件已经恢复？" ok-text="确认恢复" cancel-text="取消" @confirm="resolveEvent(record)">
                <a-button type="link" size="small"><CheckCircleOutlined />恢复</a-button>
              </a-popconfirm>
              <a-button v-else-if="admin" type="link" size="small" @click="reopenEvent(record)"><RollbackOutlined />重新打开</a-button>
            </template>
          </template>
          <template #emptyText>
            <div class="empty-state">暂无事件，管理员可同步当前告警 Provider</div>
          </template>
        </a-table>
        <div v-else class="mobile-events">
          <article v-for="record in rows" :key="record.id" class="mobile-event">
            <div class="mobile-event-head">
              <a-tag :color="sevColor(record.severity)">{{ sevText(record.severity) }}</a-tag>
              <a-badge :status="statusColor(record.status)" :text="statusText(record.status)" />
              <span :class="['count', { repeated: record.occurrences > 1 }]">{{ record.occurrences }} 次</span>
            </div>
            <strong class="rule-name">{{ record.rule }}</strong>
            <div class="mobile-target"><a-tag>{{ record.product_id }}</a-tag><span>{{ record.target }}</span></div>
            <dl>
              <div><dt>当前值</dt><dd>{{ record.value || '-' }}</dd></div>
              <div><dt>最近出现</dt><dd>{{ formatTime(record.last_seen_at) }}</dd></div>
            </dl>
            <p v-if="record.diagnosis_summary" class="diagnosis-summary">{{ record.diagnosis_summary }}</p>
            <div class="mobile-actions">
              <a-button type="link" size="small" :loading="diagnosingID === record.id" @click="diagnoseEvent(record)"><RobotOutlined />诊断</a-button>
              <a-popconfirm v-if="admin && record.status === 'open'" title="确认该告警事件已经恢复？" ok-text="确认恢复" cancel-text="取消" @confirm="resolveEvent(record)">
                <a-button type="link" size="small"><CheckCircleOutlined />恢复</a-button>
              </a-popconfirm>
              <a-button v-else-if="admin" type="link" size="small" @click="reopenEvent(record)"><RollbackOutlined />重新打开</a-button>
            </div>
          </article>
          <div v-if="!rows.length && !loading" class="empty-state">暂无事件，管理员可同步当前告警 Provider</div>
        </div>
      </section>
    </div>

    <a-drawer v-model:open="diagnosisOpen" title="事件诊断" width="min(680px, 92vw)">
      <div v-if="diagnosis" class="diagnosis-detail">
        <div class="drawer-context">
          <a-tag :color="sevColor(selectedEvent?.severity)">{{ sevText(selectedEvent?.severity) }}</a-tag>
          <strong>{{ selectedEvent?.rule }}</strong>
          <span>{{ selectedEvent?.product_id }} / {{ selectedEvent?.target }}</span>
        </div>
        <section><h3>诊断结论</h3><p>{{ diagnosis.summary }}</p><a-progress :percent="Math.round((diagnosis.confidence || 0) * 100)" size="small" /></section>
        <section><h3>根因假设</h3><ol><li v-for="item in diagnosis.hypotheses" :key="item.rank"><span>{{ item.cause }}</span><b>{{ percent(item.confidence) }}</b></li></ol></section>
        <section><h3>处置建议</h3><ul><li v-for="item in diagnosis.actions" :key="item.name"><span>{{ item.name }}</span><a-tag :color="item.risk === 'high' ? 'red' : item.risk === 'medium' ? 'orange' : 'green'">{{ item.risk }}</a-tag></li></ul></section>
        <section><h3>证据链</h3><div v-for="item in diagnosis.evidence" :key="item.source" class="evidence"><strong>{{ item.title }}</strong><p>{{ item.content }}</p><small>{{ item.source }}</small></div></section>
      </div>
    </a-drawer>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; color: #273142; }
.page-head { min-height: 78px; padding: 17px 26px; background: #fff; border-bottom: 1px solid #e9edf2; display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.page-head h2 { margin: 0; font-size: 18px; font-weight: 650; }
.page-head p { margin: 4px 0 0; color: #7f8a9a; font-size: 13px; line-height: 1.5; }
.page-body { padding: 20px 26px 28px; container-type: inline-size; }
.metrics { display: grid; grid-template-columns: repeat(5, minmax(128px, 1fr)); background: #fff; border: 1px solid #e4e9ef; border-radius: 7px; margin-bottom: 16px; overflow: hidden; }
.metric { min-height: 108px; padding: 17px 20px; border-right: 1px solid #edf0f4; display: flex; flex-direction: column; }
.metric:last-child { border-right: 0; }
.metric span { color: #657184; font-size: 12px; }
.metric strong { margin: 6px 0 2px; color: #1d2a3b; font-size: 27px; line-height: 1.1; font-variant-numeric: tabular-nums; }
.metric small { color: #a0a8b4; font-size: 11px; }
.metric .danger { color: #cf3f45; }
.metric .success { color: #27875c; }
.metric.emphasized { background: #f2f8fd; box-shadow: inset 3px 0 #1677ff; }
.metric.emphasized strong { color: #1766aa; }
.event-panel { background: #fff; border: 1px solid #e4e9ef; border-radius: 7px; overflow: hidden; }
.toolbar { min-height: 58px; padding: 11px 14px; border-bottom: 1px solid #edf0f4; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.product-filter { width: 220px; }
.result-count { margin-left: auto; color: #8b95a4; font-size: 12px; }
.rule-name, .source, .target-name, .time { display: block; }
.rule-name { color: #263348; font-size: 13px; line-height: 1.45; }
.source { margin-top: 4px; color: #98a2b1; font-size: 11px; }
.target-name { margin-top: 5px; color: #4d5a6d; font-size: 12px; }
.count { display: inline-flex; min-width: 28px; height: 24px; padding: 0 7px; align-items: center; justify-content: center; border-radius: 12px; background: #f0f2f5; color: #596579; font-weight: 650; font-variant-numeric: tabular-nums; }
.count.repeated { color: #b65316; background: #fff1e8; }
.time { color: #445064; font-size: 11px; line-height: 1.6; white-space: nowrap; }
.time.muted { color: #8f99a8; }
.diagnosis-summary { display: -webkit-box; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; color: #4b586b; font-size: 12px; line-height: 1.55; }
.empty { color: #a1a9b5; font-size: 12px; }
.empty-state { padding: 24px; color: #8993a2; }
.mobile-events { padding: 10px; background: #f5f7f9; }
.mobile-event { margin-bottom: 10px; padding: 13px; background: #fff; border: 1px solid #e4e9ef; border-radius: 6px; }
.mobile-event:last-child { margin-bottom: 0; }
.mobile-event-head { display: flex; align-items: center; gap: 8px; margin-bottom: 9px; }
.mobile-event-head .count { margin-left: auto; }
.mobile-target { display: flex; align-items: center; gap: 5px; margin-top: 8px; color: #596679; font-size: 12px; }
.mobile-event dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 12px 0 0; }
.mobile-event dl div { min-width: 0; padding: 8px 9px; background: #f7f9fb; }
.mobile-event dt { color: #8b95a4; font-size: 10px; }
.mobile-event dd { margin: 3px 0 0; overflow: hidden; color: #465366; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.mobile-event .diagnosis-summary { margin-top: 10px; }
.mobile-actions { display: flex; justify-content: flex-end; margin-top: 8px; padding-top: 7px; border-top: 1px solid #edf0f4; }
.drawer-context { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding-bottom: 16px; border-bottom: 1px solid #edf0f4; }
.drawer-context span { color: #788496; font-size: 12px; }
.diagnosis-detail section { padding: 18px 0; border-bottom: 1px solid #edf0f4; }
.diagnosis-detail h3 { margin: 0 0 10px; font-size: 14px; }
.diagnosis-detail p { margin: 0; color: #4f5c6d; line-height: 1.7; }
.diagnosis-detail ol, .diagnosis-detail ul { margin: 0; padding-left: 20px; }
.diagnosis-detail li { margin: 8px 0; padding-left: 3px; color: #455266; }
.diagnosis-detail li span { margin-right: 10px; }
.diagnosis-detail li b { color: #1677ff; font-size: 12px; }
.evidence { margin-top: 9px; padding: 11px 13px; background: #f7f9fb; border-left: 3px solid #8bb8dd; }
.evidence strong { font-size: 12px; }
.evidence p { margin: 4px 0; font-size: 12px; }
.evidence small { color: #929dab; overflow-wrap: anywhere; }
@container (max-width: 880px) {
  .metrics { grid-template-columns: repeat(3, 1fr); }
  .metric { border-bottom: 1px solid #edf0f4; }
  .metric.emphasized { grid-column: span 2; }
}
@media (max-width: 680px) {
  .page-head { align-items: flex-start; padding: 15px 16px; flex-direction: column; }
  .page-body { padding: 14px; }
  .metrics { grid-template-columns: repeat(2, 1fr); }
  .metric { min-height: 94px; padding: 14px; }
  .metric.emphasized { grid-column: span 2; }
  .product-filter { width: 100%; }
  .result-count { width: 100%; margin-left: 0; }
}
</style>
