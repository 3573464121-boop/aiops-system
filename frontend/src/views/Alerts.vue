<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { ApartmentOutlined, CheckCircleOutlined, ReloadOutlined, RobotOutlined, RollbackOutlined, SyncOutlined } from '@ant-design/icons-vue'
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
const viewMode = ref('events')
const incidents = ref([])
const correlation = ref({ open_event_count: 0, incident_count: 0, correlated_event_count: 0, singleton_count: 0, pair_comparisons: 0, linked_pairs: 0, compression_rate: 0, algorithm_version: '', window_minutes: 0, threshold: 0 })
const admin = isAdmin()
const compact = ref(window.innerWidth <= 720)
const updateCompact = () => { compact.value = window.innerWidth <= 720 }

const statusOptions = [
  { label: '全部', value: '' },
  { label: '处理中', value: 'open' },
  { label: '已恢复', value: 'resolved' },
]
const viewOptions = [
  { label: '事件视图', value: 'events' },
  { label: '故障簇', value: 'incidents' },
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
const incidentColumns = [
  { title: '级别', key: 'severity', width: 82 },
  { title: '关联故障簇', key: 'incident', width: 285 },
  { title: '规模', key: 'scale', width: 120 },
  { title: '产品 / 目标', key: 'scope', width: 220 },
  { title: '关联依据', key: 'reasons', width: 310 },
  { title: '时间范围', key: 'time', width: 180 },
  { title: '操作', key: 'ops', width: 110, fixed: 'right' },
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
    const incidentQuery = new URLSearchParams()
    if (product.value.trim()) incidentQuery.set('product_id', product.value.trim())
    const [data, correlated] = await Promise.all([
      api(`/alert-events?${query}`),
      api(`/alert-incidents?${incidentQuery}`),
    ])
    rows.value = data.items || []
    metrics.value = data.metrics || metrics.value
    incidents.value = correlated.items || []
    correlation.value = correlated.metrics || correlation.value
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

async function diagnoseIncident(record) {
  diagnosingID.value = record.id
  try {
    const query = record.product_id ? `?product_id=${encodeURIComponent(record.product_id)}` : ''
    const result = await api(`/alert-incidents/${record.id}/diagnose${query}`, { method: 'POST' })
    selectedEvent.value = {
      severity: result.incident.severity,
      rule: result.incident.title,
      product_id: result.incident.product_id,
      target: result.incident.targets.join('、'),
    }
    diagnosis.value = result.diagnosis
    diagnosisOpen.value = true
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

      <section class="correlation-bar">
        <div class="correlation-title"><ApartmentOutlined /><div><strong>关联分析</strong><span>{{ correlation.algorithm_version }} · {{ correlation.window_minutes }} 分钟窗口 · 阈值 {{ correlation.threshold }}</span></div></div>
        <div class="flow"><b>{{ correlation.open_event_count }}</b><span>开放事件</span><i></i><b>{{ correlation.incident_count }}</b><span>故障簇</span></div>
        <div class="correlation-stat"><strong>{{ percent(correlation.compression_rate) }}</strong><span>事件压缩率</span></div>
        <div class="correlation-stat"><strong>{{ correlation.correlated_event_count }}</strong><span>已关联事件</span></div>
        <div class="correlation-stat"><strong>{{ correlation.singleton_count }}</strong><span>独立事件</span></div>
      </section>

      <section class="event-panel">
        <div class="toolbar">
          <a-segmented v-model:value="viewMode" :options="viewOptions" />
          <span class="toolbar-separator"></span>
          <a-segmented v-if="viewMode === 'events'" v-model:value="status" :options="statusOptions" @change="load" />
          <a-input v-model:value="product" allow-clear placeholder="按产品标识筛选" class="product-filter" @pressEnter="load" @change="!product && load()" />
          <a-button @click="load">查询</a-button>
          <span class="result-count">当前显示 {{ viewMode === 'events' ? `${rows.length} 个事件` : `${incidents.length} 个故障簇` }}</span>
        </div>

        <a-table
          v-if="viewMode === 'events' && !compact"
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
        <div v-else-if="viewMode === 'events'" class="mobile-events">
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

        <a-table
          v-else-if="!compact"
          :columns="incidentColumns"
          :data-source="incidents"
          :loading="loading"
          :scroll="{ x: 1230 }"
          :pagination="{ pageSize: 10, showSizeChanger: false }"
          row-key="id"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'severity'"><a-tag :color="sevColor(record.severity)">{{ sevText(record.severity) }}</a-tag></template>
            <template v-else-if="column.key === 'incident'"><strong class="rule-name">{{ record.title }}</strong><span class="source">{{ record.id }}</span></template>
            <template v-else-if="column.key === 'scale'"><strong class="scale-number">{{ record.event_count }}</strong> 事件<span class="source">{{ record.signal_count }} 条原始信号</span></template>
            <template v-else-if="column.key === 'scope'"><a-tag>{{ record.product_id }}</a-tag><div class="tag-line"><a-tag v-for="target in record.targets" :key="target" color="blue">{{ target }}</a-tag></div></template>
            <template v-else-if="column.key === 'reasons'"><span v-for="reason in record.reasons" :key="reason" class="reason">{{ reason }}</span></template>
            <template v-else-if="column.key === 'time'"><span class="time">{{ formatTime(record.first_seen_at) }}</span><span class="time muted">{{ formatTime(record.last_seen_at) }}</span></template>
            <template v-else-if="column.key === 'ops'"><a-button type="link" size="small" :loading="diagnosingID === record.id" @click="diagnoseIncident(record)"><RobotOutlined />整簇诊断</a-button></template>
          </template>
          <template #emptyText><div class="empty-state">暂无开放事件可供关联</div></template>
        </a-table>

        <div v-else class="mobile-events">
          <article v-for="record in incidents" :key="record.id" class="mobile-event incident-card">
            <div class="mobile-event-head"><a-tag :color="sevColor(record.severity)">{{ sevText(record.severity) }}</a-tag><a-tag color="blue">{{ record.product_id }}</a-tag><span class="incident-size">{{ record.event_count }} 事件 / {{ record.signal_count }} 信号</span></div>
            <strong class="rule-name">{{ record.title }}</strong>
            <div class="tag-line"><a-tag v-for="target in record.targets" :key="target">{{ target }}</a-tag></div>
            <div class="reason-list"><span v-for="reason in record.reasons" :key="reason" class="reason">{{ reason }}</span></div>
            <dl><div><dt>首次出现</dt><dd>{{ formatTime(record.first_seen_at) }}</dd></div><div><dt>最近出现</dt><dd>{{ formatTime(record.last_seen_at) }}</dd></div></dl>
            <div class="mobile-actions"><a-button type="link" size="small" :loading="diagnosingID === record.id" @click="diagnoseIncident(record)"><RobotOutlined />整簇诊断</a-button></div>
          </article>
          <div v-if="!incidents.length && !loading" class="empty-state">暂无开放事件可供关联</div>
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
.page-head { min-height: 82px; padding: 18px 26px; background: #fff; border-bottom: 1px solid #e9edf2; display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.page-head h2 { margin: 0; color: #162338; font-size: 20px; font-weight: 650; line-height: 1.35; }
.page-head p { margin: 5px 0 0; color: #758196; font-size: 13px; line-height: 1.65; }
.page-body { padding: 20px 26px 30px; container-type: inline-size; }
.metrics { display: grid; grid-template-columns: repeat(5, minmax(128px, 1fr)); background: #fff; border: 1px solid #dfe5ec; border-radius: 7px; margin-bottom: 16px; overflow: hidden; }
.metric { min-height: 110px; padding: 18px 20px; border-right: 1px solid #e8edf2; display: flex; flex-direction: column; justify-content: center; }
.metric:last-child { border-right: 0; }
.metric span { color: #58677b; font-size: 13px; line-height: 1.45; }
.metric strong { margin: 6px 0 3px; color: #17263b; font-size: 29px; line-height: 1.05; font-variant-numeric: tabular-nums; }
.metric small { color: #8995a6; font-size: 12px; line-height: 1.45; }
.metric .danger { color: #cf3f45; }
.metric .success { color: #27875c; }
.metric.emphasized { background: #f2f8fd; box-shadow: inset 3px 0 #1677ff; }
.metric.emphasized strong { color: #1766aa; }
.correlation-bar { display: flex; align-items: stretch; gap: 0; min-height: 88px; margin-bottom: 16px; padding: 0 18px; background: #fff; color: #2b3748; border: 1px solid #dfe6ed; border-left: 4px solid #278fbd; border-radius: 7px; overflow: hidden; }
.correlation-title { display: flex; align-items: center; gap: 11px; }
.correlation-title { flex: 1 1 260px; min-width: 230px; padding-right: 18px; }
.correlation-title > :deep(.anticon) { display: grid; place-items: center; width: 34px; height: 34px; flex: none; color: #1c7daf; font-size: 19px; background: #eaf5fa; border-radius: 5px; }
.correlation-title div { display: flex; flex-direction: column; }
.correlation-title strong { color: #1f2f45; font-size: 15px; line-height: 1.5; }
.correlation-title span { margin-top: 3px; color: #748297; font-size: 12px; line-height: 1.55; white-space: nowrap; }
.flow { display: grid; grid-template-columns: auto auto 34px auto auto; align-items: center; gap: 6px; flex: 0 1 245px; min-width: 220px; padding: 0 20px; border-left: 1px solid #edf0f3; border-right: 1px solid #edf0f3; }
.flow b { color: #1d2d43; font-size: 22px; line-height: 1; font-variant-numeric: tabular-nums; }
.flow span { color: #748196; font-size: 12px; line-height: 1.45; white-space: nowrap; }
.flow i { position: relative; height: 1px; margin: 0 3px; background: #a9bac7; }
.flow i::after { position: absolute; right: -1px; top: -3px; width: 0; height: 0; border-top: 3px solid transparent; border-bottom: 3px solid transparent; border-left: 5px solid #8ca4b6; content: ''; }
.correlation-stat { display: flex; flex: 0 1 118px; min-width: 92px; flex-direction: column; align-items: center; justify-content: center; border-right: 1px solid #edf0f3; }
.correlation-stat:last-child { border-right: 0; }
.correlation-stat strong { color: #176d9b; font-size: 21px; line-height: 1.2; font-variant-numeric: tabular-nums; }
.correlation-stat span { margin-top: 4px; color: #778498; font-size: 12px; line-height: 1.45; white-space: nowrap; }
.event-panel { background: #fff; border: 1px solid #e4e9ef; border-radius: 7px; overflow: hidden; }
.toolbar { min-height: 62px; padding: 12px 14px; border-bottom: 1px solid #e8edf2; display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.toolbar-separator { width: 1px; height: 24px; background: #e1e5ea; }
.product-filter { width: 220px; }
.result-count { margin-left: auto; color: #758196; font-size: 12px; line-height: 1.5; }
.rule-name, .source, .target-name, .time { display: block; }
.rule-name { color: #203047; font-size: 14px; line-height: 1.55; }
.source { margin-top: 4px; color: #8490a2; font-size: 12px; line-height: 1.5; }
.target-name { margin-top: 5px; color: #425168; font-size: 13px; line-height: 1.5; }
.count { display: inline-flex; min-width: 28px; height: 24px; padding: 0 7px; align-items: center; justify-content: center; border-radius: 12px; background: #f0f2f5; color: #596579; font-weight: 650; font-variant-numeric: tabular-nums; }
.count.repeated { color: #b65316; background: #fff1e8; }
.time { color: #3f4e64; font-size: 12px; line-height: 1.65; white-space: nowrap; }
.time.muted { color: #8f99a8; }
.diagnosis-summary { display: -webkit-box; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; color: #425168; font-size: 13px; line-height: 1.65; }
.empty { color: #8f9aa9; font-size: 12px; }
.empty-state { padding: 24px; color: #8993a2; }
.scale-number { margin-right: 4px; color: #176ba5; font-size: 18px; line-height: 1.3; }
.tag-line { display: flex; flex-wrap: wrap; gap: 3px; margin-top: 6px; }
.tag-line :deep(.ant-tag) { margin-inline-end: 0; }
.reason { display: inline-block; margin: 3px 5px 3px 0; padding: 3px 8px; background: #eef4f8; color: #486176; border-radius: 3px; font-size: 12px; line-height: 1.5; }
.event-panel :deep(.ant-table-thead > tr > th) { padding-top: 13px; padding-bottom: 13px; color: #25344a; font-size: 13px; font-weight: 650; line-height: 1.5; }
.event-panel :deep(.ant-table-tbody > tr > td) { padding-top: 14px; padding-bottom: 14px; color: #344157; font-size: 13px; line-height: 1.6; vertical-align: middle; }
.mobile-events { padding: 10px; background: #f5f7f9; }
.mobile-event { margin-bottom: 10px; padding: 15px; background: #fff; border: 1px solid #dfe5ec; border-radius: 6px; }
.mobile-event:last-child { margin-bottom: 0; }
.mobile-event-head { display: flex; align-items: center; gap: 8px; margin-bottom: 9px; }
.mobile-event-head .count { margin-left: auto; }
.mobile-target { display: flex; align-items: center; gap: 6px; margin-top: 9px; color: #4e5d72; font-size: 13px; line-height: 1.5; }
.mobile-event dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 12px 0 0; }
.mobile-event dl div { min-width: 0; padding: 8px 9px; background: #f7f9fb; }
.mobile-event dt { color: #788599; font-size: 11px; line-height: 1.45; }
.mobile-event dd { margin: 4px 0 0; overflow: hidden; color: #3f4f65; font-size: 12px; line-height: 1.5; text-overflow: ellipsis; white-space: nowrap; }
.mobile-event .diagnosis-summary { margin-top: 10px; }
.incident-size { margin-left: auto; color: #718094; font-size: 10px; }
.reason-list { margin-top: 9px; }
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
@container (max-width: 980px) {
  .metrics { grid-template-columns: repeat(3, 1fr); }
  .metric { border-bottom: 1px solid #edf0f4; }
  .metric.emphasized { grid-column: span 2; }
  .correlation-bar { flex-wrap: wrap; padding: 0 14px; }
  .correlation-title { flex: 1 1 50%; min-height: 70px; }
  .flow { flex: 1 1 42%; min-height: 70px; border-right: 0; }
  .correlation-stat { flex: 1 1 33.33%; min-height: 58px; border-top: 1px solid #edf0f3; }
}
@media (max-width: 680px) {
  .page-head { align-items: flex-start; padding: 16px; flex-direction: column; }
  .page-head h2 { font-size: 19px; }
  .page-head p { font-size: 13px; line-height: 1.6; }
  .page-body { padding: 14px; }
  .metrics { grid-template-columns: repeat(2, 1fr); }
  .metric { min-height: 94px; padding: 14px; }
  .metric.emphasized { grid-column: span 2; }
  .correlation-bar { padding: 0 12px; }
  .correlation-title { flex: 1 1 100%; min-width: 0; min-height: 78px; padding: 0; }
  .correlation-title span { white-space: normal; }
  .flow { flex: 1 1 100%; min-width: 0; min-height: 58px; justify-content: center; padding: 0; border: 0; border-top: 1px solid #edf0f3; }
  .correlation-stat { min-width: 0; min-height: 60px; }
  .toolbar { align-items: stretch; padding: 13px; }
  .toolbar-separator { display: none; }
  .product-filter { width: 100%; }
  .result-count { width: 100%; margin-left: 0; }
}
</style>
