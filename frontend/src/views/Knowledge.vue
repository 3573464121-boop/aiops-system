<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { CloudUploadOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue'
import { api, isAdmin } from '../api'

const admin = isAdmin()
const q = ref('')
const results = ref([])
const documents = ref([])
const status = ref({ document_count: 0, enabled_count: 0, chunk_count: 0, mode: 'empty', version: 'empty' })
const loading = ref(false)
const searching = ref(false)
const importing = ref(false)
const rebuilding = ref(false)
const searched = ref(false)
const fileInput = ref(null)

const modeText = computed(() => status.value.mode === 'bm25+vector(RRF)' ? '混合检索' : status.value.mode === 'empty' ? '未建索引' : 'BM25')
const columns = [
  { title: '文档', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '版本', dataIndex: 'version', width: 170 },
  { title: '分块', dataIndex: 'chunk_count', width: 80 },
  { title: '命中', dataIndex: 'hit_count', width: 80 },
  { title: '来源', key: 'source', width: 100 },
  { title: '状态', key: 'enabled', width: 90 },
  { title: '更新时间', key: 'updated', width: 180 },
  { title: '操作', key: 'operations', width: 130, fixed: 'right' },
]

async function load() {
  loading.value = true
  try {
    const [docs, state] = await Promise.all([api('/knowledge/documents'), api('/knowledge/status')])
    documents.value = docs.items || []
    status.value = state
  } catch (error) {
    message.error(error.message)
  } finally {
    loading.value = false
  }
}

async function search() {
  if (!q.value.trim()) return
  searching.value = true
  searched.value = true
  try {
    results.value = (await api(`/knowledge/search?query=${encodeURIComponent(q.value)}&limit=8`)).items || []
    await load()
  } catch (error) {
    results.value = []
    message.error(error.message)
  } finally {
    searching.value = false
  }
}

function chooseFile() {
  fileInput.value?.click()
}

async function importFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.md')) {
    message.warning('只允许导入 Markdown (.md) 文档')
    return
  }
  importing.value = true
  try {
    const form = new FormData()
    form.append('file', file)
    await api('/knowledge/documents', { method: 'POST', body: form })
    message.success('文档已导入并完成索引重建')
    await load()
  } catch (error) {
    message.error(error.message)
  } finally {
    importing.value = false
  }
}

async function toggle(record, enabled) {
  try {
    await api(`/knowledge/documents/${record.id}/toggle`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    })
    message.success(enabled ? '文档已启用' : '文档已停用')
    await load()
  } catch (error) {
    message.error(error.message)
    await load()
  }
}

async function remove(record) {
  try {
    await api(`/knowledge/documents/${record.id}`, { method: 'DELETE' })
    message.success('文档已删除')
    await load()
  } catch (error) {
    message.error(error.message)
  }
}

async function reindex() {
  rebuilding.value = true
  try {
    status.value = await api('/knowledge/reindex', { method: 'POST' })
    message.success(status.value.warning || '索引重建完成')
    await load()
  } catch (error) {
    message.error(error.message)
  } finally {
    rebuilding.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h2>知识库</h2>
        <p>管理诊断依据，追踪文档版本、索引状态和检索命中。</p>
      </div>
      <div v-if="admin" class="head-actions">
        <input ref="fileInput" type="file" accept=".md,text/markdown" hidden @change="importFile">
        <a-button :loading="importing" @click="chooseFile"><CloudUploadOutlined />导入 Markdown</a-button>
        <a-button :loading="rebuilding" @click="reindex"><ReloadOutlined />重建索引</a-button>
      </div>
    </header>

    <div class="page-body">
      <section class="metrics">
        <div class="metric"><span>文档总数</span><strong>{{ status.document_count }}</strong></div>
        <div class="metric"><span>启用文档</span><strong>{{ status.enabled_count }}</strong></div>
        <div class="metric"><span>知识分块</span><strong>{{ status.chunk_count }}</strong></div>
        <div class="metric"><span>检索模式</span><strong class="mode">{{ modeText }}</strong></div>
        <div class="metric version"><span>索引版本</span><code>{{ status.version }}</code></div>
      </section>

      <a-alert v-if="status.warning" :message="status.warning" type="warning" show-icon class="notice" />

      <section class="search-band">
        <div class="section-copy">
          <h3>检索验证</h3>
          <span>直接验证当前启用文档是否能支撑故障诊断。</span>
        </div>
        <a-input-search v-model:value="q" placeholder="输入故障现象、组件或处置关键词" enter-button="检索" :loading="searching" @search="search">
          <template #prefix><SearchOutlined /></template>
        </a-input-search>
      </section>

      <a-list v-if="searched || results.length" class="results" :data-source="results" :loading="searching" :locale="{ emptyText: '当前索引没有命中相关知识' }">
        <template #renderItem="{ item }">
          <a-list-item class="result-row">
            <div class="result-head"><strong>{{ item.title }}</strong><a-tag color="blue">{{ Math.round(item.score * 100) }}%</a-tag></div>
            <p>{{ item.content }}</p>
            <code>{{ item.source }}</code>
          </a-list-item>
        </template>
      </a-list>

      <div class="section-title">
        <div><h3>文档清单</h3><span>内置文档可停用，导入文档可停用或删除。</span></div>
        <a-button size="small" :loading="loading" @click="load"><ReloadOutlined />刷新</a-button>
      </div>
      <a-table :columns="columns" :data-source="documents" :loading="loading" row-key="id" size="small" :pagination="{ pageSize: 10 }" :scroll="{ x: 1050 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <div class="doc-name"><strong>{{ record.name }}</strong><small>{{ record.created_by || 'system' }}</small></div>
          </template>
          <template v-else-if="column.key === 'source'">
            <a-tag :color="record.managed ? 'cyan' : 'default'">{{ record.managed ? '导入' : '内置' }}</a-tag>
          </template>
          <template v-else-if="column.key === 'enabled'">
            <a-switch v-if="admin" :checked="record.enabled" size="small" @change="value => toggle(record, value)" />
            <a-tag v-else :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</a-tag>
          </template>
          <template v-else-if="column.key === 'updated'">
            {{ new Date(record.updated_at).toLocaleString() }}
          </template>
          <template v-else-if="column.key === 'operations'">
            <a-popconfirm v-if="admin && record.managed" title="删除文档并从索引中移除？" @confirm="remove(record)">
              <a-button type="link" size="small" danger>删除</a-button>
            </a-popconfirm>
            <span v-else class="muted">{{ admin ? '不可删除' : '只读' }}</span>
          </template>
        </template>
      </a-table>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; container: knowledge-page / inline-size; }
.page-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2, .section-title h3, .section-copy h3 { margin: 0; }
.page-head h2 { font-size: 18px; line-height: 1.35; }
.page-head p, .section-title span, .section-copy span { margin: 3px 0 0; color: #7c8798; font-size: 12px; line-height: 1.55; }
.head-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.page-body { padding: 22px 26px 32px; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(104px, 1fr)) minmax(205px, 1.6fr); background: #fff; border: 1px solid #e5e9ef; border-radius: 6px; margin-bottom: 18px; overflow: hidden; }
.metrics > div { display: flex; min-height: 72px; flex-direction: column; justify-content: center; gap: 5px; padding: 13px 17px; border-right: 1px solid #edf0f3; }
.metrics > div:last-child { border-right: 0; }
.metrics span { color: #7c8798; font-size: 11px; line-height: 1.2; }
.metrics strong { color: #202938; font-size: 21px; line-height: 1.15; font-variant-numeric: tabular-nums; }
.metrics .mode { font-size: 14px; line-height: 1.35; }
.metrics code { color: #475467; font-size: 11px; line-height: 1.4; overflow-wrap: anywhere; }
.notice { margin-bottom: 18px; }
.search-band { display: grid; grid-template-columns: minmax(210px, .7fr) minmax(320px, 1.3fr); align-items: center; gap: 28px; padding: 18px 0; border-top: 1px solid #eaecf0; border-bottom: 1px solid #eaecf0; }
.section-copy h3, .section-title h3 { color: #344054; font-size: 14px; line-height: 1.4; }
.results { margin: 0 0 22px; }
.result-row { display: block; padding: 15px 2px !important; }
.result-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.result-row p { margin: 7px 0; color: #667085; line-height: 1.65; }
.result-row code { color: #8c98a8; font-size: 12px; overflow-wrap: anywhere; }
.section-title { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin: 24px 0 10px; }
.doc-name { display: flex; flex-direction: column; align-items: flex-start; gap: 3px; min-width: 0; line-height: 1.35; }
.doc-name strong { display: block; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
.doc-name small { display: block; color: #98a2b3; font-size: 11px; line-height: 1.3; }
.muted { color: #98a2b3; font-size: 12px; }
:deep(.ant-table-cell) { padding-top: 11px !important; padding-bottom: 11px !important; font-size: 13px; line-height: 1.45; }
@container knowledge-page (max-width: 900px) {
  .page-head { align-items: flex-start; flex-direction: column; }
  .metrics { grid-template-columns: repeat(4, 1fr); }
  .metrics .version { grid-column: 1 / -1; border-top: 1px solid #edf0f3; }
  .search-band { grid-template-columns: 1fr; gap: 12px; }
}
@container knowledge-page (max-width: 620px) {
  .page-head, .page-body { padding-left: 16px; padding-right: 16px; }
  .metrics { grid-template-columns: repeat(2, 1fr); }
  .metrics > div:nth-child(2n) { border-right: 0; }
  .metrics .version { grid-column: 1 / -1; }
  .head-actions { width: 100%; }
}
</style>
