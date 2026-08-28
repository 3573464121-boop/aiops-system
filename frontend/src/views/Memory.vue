<script setup>
import { ref, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import { api } from '../api'

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const form = ref({ scope: 'global', product_id: '', kind: 'fact', content: '' })

// 提取草稿
const extractText = ref('')
const extractPid = ref('')
const extracting = ref(false)

const kindText = k => ({ fact: '事实', fix: '处置', preference: '偏好' }[k] || k)
const kindColor = k => ({ fact: 'blue', fix: 'green', preference: 'purple' }[k] || 'default')
const scopeText = r => (r.scope === 'product' ? r.product_id : '全局')

const columns = [
  { title: '作用域', key: 'scope', width: 110 },
  { title: '类型', key: 'kind', width: 80 },
  { title: '内容', dataIndex: 'content' },
  { title: '来源', dataIndex: 'source', width: 90 },
  { title: '时间', key: 't', width: 170 },
  { title: '', key: 'ops', width: 70 },
]

const total = computed(() => rows.value.length)

async function load() {
  loading.value = true
  try { rows.value = (await api('/memories')).items || [] }
  catch { rows.value = [] }
  finally { loading.value = false }
}

async function save() {
  if (!form.value.content.trim()) { message.warning('请填写记忆内容'); return }
  if (form.value.scope === 'product' && !form.value.product_id.trim()) { message.warning('产品作用域需填产品标识'); return }
  saving.value = true
  try {
    await api('/memories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    message.success('已记住')
    form.value = { scope: 'global', product_id: '', kind: 'fact', content: '' }
    load()
  } catch (e) { message.error(e.message) }
  finally { saving.value = false }
}

async function remove(rec) {
  try {
    await api(`/memories/${rec.id}`, { method: 'DELETE' })
    load()
  } catch (e) { message.error(e.message) }
}

async function extract() {
  if (!extractText.value.trim()) { message.warning('请粘贴一段诊断内容'); return }
  extracting.value = true
  try {
    const r = await api('/memories/extract', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product_id: extractPid.value, text: extractText.value }),
    })
    // 把提炼结果填进上方表单，交由用户确认后保存
    form.value.content = r.draft || ''
    form.value.source = 'extracted'
    if (extractPid.value.trim()) { form.value.scope = 'product'; form.value.product_id = extractPid.value }
    message.success('已提炼，请在上方确认后保存')
  } catch (e) { message.error(e.message) }
  finally { extracting.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h2>记忆空间</h2>
      <p>沉淀跨对话的长期经验与环境事实，诊断时按相关度自动召回并作为背景注入（全局 / 产品两级作用域）</p>
    </header>
    <div class="page-body">
      <a-row :gutter="18">
        <a-col :span="14">
          <a-card size="small" title="新增记忆" style="margin-bottom:18px">
            <a-space direction="vertical" style="width:100%">
              <a-space>
                <a-select v-model:value="form.scope" style="width:120px">
                  <a-select-option value="global">全局</a-select-option>
                  <a-select-option value="product">限定产品</a-select-option>
                </a-select>
                <a-input v-if="form.scope === 'product'" v-model:value="form.product_id" placeholder="产品标识，如 payment" style="width:160px" />
                <a-select v-model:value="form.kind" style="width:110px">
                  <a-select-option value="fact">事实</a-select-option>
                  <a-select-option value="fix">处置</a-select-option>
                  <a-select-option value="preference">偏好</a-select-option>
                </a-select>
              </a-space>
              <a-textarea v-model:value="form.content" :rows="3" placeholder="一句话经验，如：payment 数据库连接池上限 50，历史上被 inventory 超时拖垮打满过" />
              <a-button type="primary" :loading="saving" @click="save">记住</a-button>
            </a-space>
          </a-card>
        </a-col>
        <a-col :span="10">
          <a-card size="small" title="从诊断内容提炼" style="margin-bottom:18px">
            <a-space direction="vertical" style="width:100%">
              <a-input v-model:value="extractPid" placeholder="产品标识（可选）" />
              <a-textarea v-model:value="extractText" :rows="3" placeholder="粘贴一段诊断结论，交给模型提炼成可复用的一句话记忆" />
              <a-button :loading="extracting" @click="extract">提炼草稿</a-button>
            </a-space>
          </a-card>
        </a-col>
      </a-row>

      <div class="sec-title">
        <span>已有记忆 · {{ total }}</span>
        <a-button size="small" @click="load">刷新</a-button>
      </div>
      <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="id" :pagination="{ pageSize: 10 }" size="small">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'scope'">
            <a-tag :color="record.scope === 'product' ? 'geekblue' : 'default'">{{ scopeText(record) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'kind'"><a-tag :color="kindColor(record.kind)">{{ kindText(record.kind) }}</a-tag></template>
          <template v-else-if="column.key === 't'">{{ new Date(record.created_at).toLocaleString() }}</template>
          <template v-else-if="column.key === 'ops'">
            <a-popconfirm title="删除该记忆？" @confirm="remove(record)">
              <a-button type="link" size="small" danger>删除</a-button>
            </a-popconfirm>
          </template>
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
