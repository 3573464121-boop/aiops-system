<script setup>
import { ref, reactive, computed, onMounted, nextTick, watch } from 'vue'
import { PlusOutlined, SendOutlined, RobotOutlined, ExportOutlined, MenuFoldOutlined, MenuUnfoldOutlined, CheckCircleFilled } from '@ant-design/icons-vue'
import { api, API } from '../api'

const productId = ref('payment')
const model = ref('deepseek-chat')
const draft = ref('')
const busy = ref(false)
const status = ref(null)
const sideOpen = ref(true)
const scroller = ref(null)

const uid = () => Math.random().toString(36).slice(2, 9)
const sessions = ref([{ id: uid(), title: '新对话', messages: [] }])
const activeId = ref(sessions.value[0].id)
const active = computed(() => sessions.value.find(s => s.id === activeId.value) || sessions.value[0])

// 会话持久化到 localStorage（刷新 / 重开浏览器都保留）
const STORE_KEY = 'aiops_sessions_v1'
function persist() {
  try {
    // 只持久化“已完成”的回合：进行中的诊断不落盘，避免刷新后残留半截结果
    const clean = sessions.value.map(s => ({ ...s, messages: (s.messages || []).filter(m => m.role !== 'assistant' || (!m.loading && m.result)) }))
    localStorage.setItem(STORE_KEY, JSON.stringify({ sessions: clean, activeId: activeId.value }))
  } catch { /* ignore */ }
}
watch([sessions, activeId], persist, { deep: true })

const confColor = p => (p >= 70 ? 'green' : p >= 40 ? 'gold' : 'red')
const riskColor = r => ({ low: 'green', medium: 'gold', high: 'red' }[r] || 'default')
const riskText = r => ({ low: '低风险', medium: '中风险', high: '高风险' }[r] || r)
const pct = r => Math.round((r?.confidence || 0) * 100)

async function scrollToBottom() { await nextTick(); if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight }
function newChat() { if (busy.value) return; const s = { id: uid(), title: '新对话', messages: [] }; sessions.value.unshift(s); activeId.value = s.id }
function switchTo(id) { if (!busy.value) activeId.value = id }
function onEnter(e) { if (e.isComposing || e.shiftKey) return; e.preventDefault(); send() }

async function send() {
  const q = draft.value.trim()
  if (!q || busy.value || !productId.value.trim()) return
  draft.value = ''
  const sess = active.value
  if (sess.messages.length === 0) sess.title = q.length > 18 ? q.slice(0, 18) + '…' : q
  sess.messages.push({ role: 'user', text: q })
  const bot = reactive({ role: 'assistant', question: q, events: [], result: null, error: '', loading: true, elapsed: '0.0', issueMsg: '', reasonKey: ['r'] })
  sess.messages.push(bot)
  busy.value = true
  const t0 = performance.now()
  scrollToBottom()
  try {
    const resp = await fetch(`${API}/diagnoses/stream`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product_id: productId.value, question: q, window_minutes: 30 })
    })
    if (!resp.ok || !resp.body) { const d = await resp.json().catch(() => ({})); throw new Error(d.error || `请求失败（${resp.status}）`) }
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      let idx
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const frame = buf.slice(0, idx)
        buf = buf.slice(idx + 2)
        const line = frame.split('\n').find(l => l.startsWith('data:'))
        if (!line) continue
        try {
          const ev = JSON.parse(line.slice(5).trim())
          if (ev.type === 'result') bot.result = ev.result
          else if (ev.type === 'done') { /* 结束 */ }
          else bot.events.push(ev)
          scrollToBottom()
        } catch { /* 半帧忽略 */ }
      }
    }
  } catch (e) {
    bot.error = `${e.message}（请确认后端在 8080 端口运行）`
  } finally {
    bot.loading = false
    bot.elapsed = ((performance.now() - t0) / 1000).toFixed(1)
    bot.reasonKey = []
    busy.value = false
    scrollToBottom()
  }
}

async function createIssue(bot) {
  if (!bot.result) return
  try {
    const issue = await api('/issues', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product_id: bot.result.product_id || productId.value, title: bot.question, diagnosis: bot.result.summary })
    })
    bot.issueMsg = `已创建工单 ${issue.id}`
  } catch (e) { bot.issueMsg = `创建失败：${e.message}` }
}

function exportChat() {
  const lines = []
  for (const m of active.value.messages) {
    if (m.role === 'user') lines.push(`【提问】${m.text}`)
    else if (m.result) {
      lines.push(`【诊断结论 ${pct(m.result)}%】${m.result.summary}`)
      m.result.hypotheses.forEach(h => lines.push(`  · 假设：${h.cause}（${Math.round(h.confidence * 100)}%）`))
      m.result.actions.forEach(a => lines.push(`  · 建议：${a.name} [${riskText(a.risk)}${a.requires_approval ? '·需审批' : ''}]`))
    }
    lines.push('')
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${active.value.title || '诊断会话'}.txt`
  a.click()
  URL.revokeObjectURL(a.href)
}

onMounted(async () => {
  // 恢复历史会话
  try {
    const raw = localStorage.getItem(STORE_KEY)
    const d = raw && JSON.parse(raw)
    if (d && Array.isArray(d.sessions) && d.sessions.length) {
      // 只恢复“已完成”的回合，清理任何半截/未完成的诊断
      d.sessions.forEach(s => { s.messages = (s.messages || []).filter(m => m.role === 'user' || (m.role === 'assistant' && m.result)) })
      sessions.value = d.sessions
      activeId.value = d.sessions.some(s => s.id === d.activeId) ? d.activeId : d.sessions[0].id
    }
  } catch { /* ignore */ }
  try { status.value = await api('/system/status') } catch { /* ignore */ }
})
</script>

<template>
  <div class="assistant">
    <!-- 会话列表（可折叠） -->
    <aside :class="['conv', { hide: !sideOpen }]">
      <a-button type="primary" block class="new-btn" @click="newChat"><template #icon><PlusOutlined /></template>新对话</a-button>
      <div class="sess-group">最近对话</div>
      <div class="sess-list">
        <div v-for="s in sessions" :key="s.id" :class="['sess', { active: s.id === activeId }]" @click="switchTo(s.id)">
          <RobotOutlined />
          <span class="sess-title">{{ s.title }}</span>
        </div>
      </div>
      <div class="conv-foot" v-if="status"><a-badge status="success" />后端在线 · 只读 · {{ status.storage_provider }}</div>
    </aside>

    <!-- 对话主区 -->
    <section class="chat">
      <header class="chat-head">
        <div class="head-left">
          <a-tooltip :title="sideOpen ? '收起会话栏' : '展开会话栏'"><a-button type="text" class="fold" @click="sideOpen = !sideOpen"><component :is="sideOpen ? MenuFoldOutlined : MenuUnfoldOutlined" /></a-button></a-tooltip>
          <span class="chat-title">{{ active.title }}</span>
        </div>
        <div class="head-right">
          <span class="prod">产品 <a-input v-model:value="productId" size="small" style="width:120px" /></span>
          <a-button size="small" :disabled="!active.messages.length" @click="exportChat"><template #icon><ExportOutlined /></template>导出</a-button>
        </div>
      </header>

      <div class="messages" ref="scroller">
        <div v-if="!active.messages.length" class="welcome">
          <a-avatar :size="60" style="background:#1677ff"><template #icon><RobotOutlined /></template></a-avatar>
          <h3>描述你遇到的告警或故障，我来带证据地诊断</h3>
          <a-space direction="vertical" style="width:100%;max-width:460px">
            <a-button block @click="draft = '分析支付服务最近30分钟的高错误率告警'">分析支付服务最近30分钟的高错误率告警</a-button>
            <a-button block @click="draft = '库存服务实例存活异常，帮我排查'">库存服务实例存活异常，帮我排查</a-button>
          </a-space>
        </div>

        <template v-for="(m, mi) in active.messages" :key="mi">
          <div v-if="m.role === 'user'" class="row user"><div class="bubble">{{ m.text }}</div></div>

          <div v-else class="row bot">
            <a-avatar class="bot-ava" style="background:#1677ff"><template #icon><RobotOutlined /></template></a-avatar>
            <a-card class="bot-card" size="small">
              <div class="bot-head">
                <CheckCircleFilled v-if="!m.loading && !m.error" style="color:#52c41a" />
                <span class="bot-title">AI 综合分析</span>
                <a-spin v-if="m.loading" size="small" class="bh-right" />
                <span v-else class="bh-right elapsed">耗时 {{ m.elapsed }}s</span>
              </div>
              <div class="bot-div"></div>

              <a-collapse v-if="m.events.length || m.loading" v-model:activeKey="m.reasonKey" :bordered="false" class="reason">
                <a-collapse-panel key="r">
                  <template #header><span class="rh">查看推理过程<span v-if="m.events.length" class="rn">（{{ m.events.filter(e => e.type === 'tool_call').length }} 次工具调用）</span></span></template>
                  <div class="feed">
                    <div v-for="(ev, i) in m.events" :key="i" :class="['feed-item', ev.type]">
                      <span class="dot"></span>
                      <span v-if="ev.tool" class="ftool">{{ ev.tool }}</span>
                      <span class="fmsg">{{ ev.message }}</span>
                      <a-tag v-if="ev.status" :color="ev.status === 'success' ? 'green' : 'red'" class="ftag">{{ ev.status }}</a-tag>
                    </div>
                  </div>
                </a-collapse-panel>
              </a-collapse>

              <a-alert v-if="m.error" type="error" :message="m.error" show-icon style="margin-top:10px" />

              <div v-if="m.result" class="diag">
                <div class="summary">
                  <a-tag :color="confColor(pct(m.result))" class="conf">置信度 {{ pct(m.result) }}%</a-tag>
                  <span class="sum-text">{{ m.result.summary }}</span>
                </div>

                <div class="block" v-if="m.result.hypotheses.length">
                  <div class="block-h">根因假设</div>
                  <div v-for="h in m.result.hypotheses" :key="h.rank" class="hyp"><span>{{ h.rank }}. {{ h.cause }}</span><a-tag color="blue">{{ Math.round(h.confidence * 100) }}%</a-tag></div>
                </div>

                <div class="block" v-if="m.result.actions.length">
                  <div class="block-h">处置建议</div>
                  <div v-for="a in m.result.actions" :key="a.name" class="act"><span>{{ a.name }}</span><span class="act-tags"><a-tag v-if="a.requires_approval" color="volcano">需审批</a-tag><a-tag :color="riskColor(a.risk)">{{ riskText(a.risk) }}</a-tag></span></div>
                </div>

                <a-collapse :bordered="false" class="reason" v-if="m.result.evidence.length">
                  <a-collapse-panel key="e">
                    <template #header><span class="rh">证据链（{{ m.result.evidence.length }}）</span></template>
                    <div v-for="(e, i) in m.result.evidence" :key="i" class="ev"><a-tag color="cyan">{{ e.type }}</a-tag><div class="ev-body"><strong>{{ e.title }}</strong><p>{{ e.content }}</p><code>{{ e.source }}</code></div></div>
                  </a-collapse-panel>
                </a-collapse>

                <div class="card-foot"><a-button type="primary" ghost size="small" @click="createIssue(m)">创建工单</a-button><span class="gen">生成自 {{ model }}</span></div>
                <a-alert v-if="m.issueMsg" type="success" :message="m.issueMsg" show-icon banner style="margin-top:8px" />
              </div>
            </a-card>
          </div>
        </template>
      </div>

      <footer class="composer">
        <div class="composer-box">
          <a-textarea v-model:value="draft" :bordered="false" :auto-size="{ minRows: 1, maxRows: 6 }" placeholder="描述告警或故障…（Enter 发送，Shift+Enter 换行）" @keydown.enter="onEnter" />
          <div class="composer-tools">
            <a-select v-model:value="model" size="small" style="width:150px" :options="[{ value: 'deepseek-chat', label: 'DeepSeek Chat' }]" />
            <a-button type="primary" shape="round" :loading="busy" :disabled="!draft.trim()" @click="send"><template #icon><SendOutlined /></template>发送</a-button>
          </div>
        </div>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.assistant { display: flex; height: 100%; width: 100%; background: #f5f6f8; }
.conv { width: 248px; flex: none; background: #fff; border-right: 1px solid #edeff2; padding: 14px 12px; display: flex; flex-direction: column; gap: 10px; overflow: hidden; transition: width .22s ease, padding .22s ease; }
.conv.hide { width: 0; padding-left: 0; padding-right: 0; border-right: 0; }
.new-btn { height: 38px; }
.sess-group { color: #98a2b3; font-size: 12px; padding: 4px 6px 0; }
.sess-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.sess { display: flex; align-items: center; gap: 8px; padding: 9px 10px; border-radius: 8px; cursor: pointer; color: #3d4757; font-size: 13px; white-space: nowrap; }
.sess:hover { background: #f2f6ff; } .sess.active { background: #e8f1ff; color: #1677ff; font-weight: 600; }
.sess-title { overflow: hidden; text-overflow: ellipsis; }
.conv-foot { font-size: 12px; color: #98a2b3; border-top: 1px solid #edeff2; padding-top: 10px; }

.chat { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.chat-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 18px; background: #fff; border-bottom: 1px solid #edeff2; }
.head-left { display: flex; align-items: center; gap: 6px; } .chat-title { font-weight: 600; font-size: 15px; }
.head-right { display: flex; align-items: center; gap: 12px; } .prod { color: #667085; font-size: 12px; display: flex; align-items: center; gap: 6px; }

.messages { flex: 1; overflow-y: auto; padding: 24px; display: flex; flex-direction: column; gap: 20px; }
.messages::-webkit-scrollbar { width: 8px; } .messages::-webkit-scrollbar-thumb { background: #d8dde4; border-radius: 8px; }
.welcome { margin: auto; text-align: center; color: #667085; display: flex; flex-direction: column; align-items: center; gap: 14px; }
.welcome h3 { font-weight: 500; color: #3d4757; margin: 0; }

.row { display: flex; animation: fade .28s ease; }
@keyframes fade { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
.row.user { justify-content: flex-end; }
.row.user .bubble { max-width: 78%; background: #1677ff; color: #fff; padding: 10px 14px; border-radius: 14px 14px 4px 14px; line-height: 1.6; }
.row.bot { gap: 12px; align-items: flex-start; }
.bot-ava { flex: none; }
.bot-card { flex: 1; min-width: 0; border-radius: 14px; border: 1px solid #edeff2; box-shadow: 0 1px 4px rgba(0,0,0,.04); }
.bot-head { display: flex; align-items: center; gap: 8px; }
.bot-title { font-weight: 700; color: #1f2733; } .bh-right { margin-left: auto; } .elapsed { color: #98a2b3; font-size: 12px; }
.bot-div { height: 1px; background: #f0f0f0; margin: 10px 0; }

.reason :deep(.ant-collapse-header) { padding: 4px 0 !important; }
.reason :deep(.ant-collapse-content-box) { padding: 4px 0 8px !important; }
.rh { color: #1677ff; font-size: 13px; } .rn { color: #98a2b3; }
.feed { display: grid; gap: 6px; }
.feed-item { display: flex; align-items: center; gap: 9px; padding: 7px 10px; border-radius: 8px; background: #f6f8fb; font-size: 12.5px; color: #3d4757; animation: fade .2s ease; }
.feed-item .dot { width: 7px; height: 7px; border-radius: 50%; background: #1677ff; flex: none; }
.feed-item.tool_call .dot { background: #faad14; } .feed-item.tool_result .dot { background: #52c41a; }
.ftool { font-weight: 700; color: #1677ff; } .fmsg { color: #667085; } .ftag { margin-left: auto; }

.diag { display: grid; gap: 14px; margin-top: 6px; }
.summary { background: #f6f8fb; border-radius: 10px; padding: 14px; }
.summary .conf { float: right; margin-left: 10px; } .sum-text { line-height: 1.7; color: #1f2733; }
.block-h { font-size: 12px; font-weight: 700; color: #667085; margin-bottom: 8px; }
.hyp, .act { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 9px 12px; border-radius: 8px; background: #f6f8fb; margin-bottom: 6px; font-size: 13px; }
.act-tags { flex: none; }
.ev { display: flex; gap: 10px; padding: 9px 0; border-bottom: 1px solid #f0f0f0; }
.ev-body strong { font-size: 13px; } .ev-body p { margin: 3px 0; color: #667085; font-size: 12px; line-height: 1.5; } .ev-body code { color: #8c98a8; font-size: 11px; }
.card-foot { display: flex; align-items: center; gap: 12px; } .gen { color: #b0b8c4; font-size: 11px; }

.composer { padding: 12px 24px 18px; background: #fff; border-top: 1px solid #edeff2; }
.composer-box { border: 1px solid #d9dde3; border-radius: 16px; padding: 8px 12px; transition: border-color .2s, box-shadow .2s; }
.composer-box:focus-within { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.composer-tools { display: flex; justify-content: space-between; align-items: center; margin-top: 6px; }

@media (max-width: 820px) { .conv { position: absolute; z-index: 10; height: 100%; } }
</style>
