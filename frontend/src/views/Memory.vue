<script setup>
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { api, getUser, isAdmin } from '../api'

const user = getUser() || {}
const admin = isAdmin()
const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const initialScope = admin ? 'global' : 'personal'
const form = ref({ scope: initialScope, product_id: '', kind: 'fact', content: '' })

const extractText = ref('')
const extractPid = ref('')
const extracting = ref(false)

const scopeOptions = computed(() => {
  const options = [{ value: 'personal', label: '个人' }]
  if (user.team_id) options.push({ value: 'team', label: `团队 · ${user.team_id}` })
  if (admin) options.unshift(
    { value: 'global', label: '全局' },
    { value: 'product', label: '指定产品' },
  )
  return options
})
const kindText = kind => ({ fact: '事实', fix: '处置', preference: '偏好' }[kind] || kind)
const kindColor = kind => ({ fact: 'blue', fix: 'green', preference: 'purple' }[kind] || 'default')
const scopeText = record => ({
  global: '全局',
  product: `产品 · ${record.product_id}`,
  team: `团队 · ${record.team_id}`,
  personal: `个人 · ${record.owner_name || '本人'}`,
}[record.scope] || record.scope)
const scopeColor = scope => ({ global: 'default', product: 'geekblue', team: 'cyan', personal: 'green' }[scope] || 'default')
const canDelete = record => {
  if (record.scope === 'personal') return record.owner_id === user.id
  if (record.scope === 'team') return record.owner_id === user.id || (admin && record.team_id === user.team_id)
  return admin
}

const columns = [
  { title: '作用域', key: 'scope', width: 160 },
  { title: '类型', key: 'kind', width: 80 },
  { title: '内容', dataIndex: 'content' },
  { title: '创建者', dataIndex: 'owner_name', width: 100 },
  { title: '来源', dataIndex: 'source', width: 90 },
  { title: '时间', key: 'time', width: 170 },
  { title: '', key: 'operations', width: 70 },
]

const total = computed(() => rows.value.length)

async function load() {
  loading.value = true
  try {
    rows.value = (await api('/memories')).items || []
  } catch (error) {
    rows.value = []
    message.error(error.message)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.value = { scope: initialScope, product_id: '', kind: 'fact', content: '' }
}

async function save() {
  if (!form.value.content.trim()) {
    message.warning('请填写记忆内容')
    return
  }
  if (form.value.scope === 'product' && !form.value.product_id.trim()) {
    message.warning('产品记忆必须填写产品标识')
    return
  }
  saving.value = true
  try {
    await api('/memories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ...form.value,
        team_id: form.value.scope === 'team' ? user.team_id : '',
      }),
    })
    message.success('记忆已保存')
    resetForm()
    await load()
  } catch (error) {
    message.error(error.message)
  } finally {
    saving.value = false
  }
}

async function remove(record) {
  try {
    await api(`/memories/${record.id}`, { method: 'DELETE' })
    message.success('记忆已删除')
    await load()
  } catch (error) {
    message.error(error.message)
  }
}

async function extract() {
  if (!extractText.value.trim()) {
    message.warning('请粘贴一段诊断内容')
    return
  }
  extracting.value = true
  try {
    const result = await api('/memories/extract', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ product_id: extractPid.value, text: extractText.value }),
    })
    form.value.content = result.draft || ''
    if (extractPid.value.trim() && admin) {
      form.value.scope = 'product'
      form.value.product_id = extractPid.value
    }
    message.success('已生成草稿，请确认后保存')
  } catch (error) {
    message.error(error.message)
  } finally {
    extracting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h2>记忆空间</h2>
        <p>按全局、产品、团队和个人范围管理可复用的运维经验</p>
      </div>
      <div class="identity">
        <span>{{ user.username }}</span>
        <a-tag color="cyan">{{ user.team_id || '未分配团队' }}</a-tag>
      </div>
    </header>

    <div class="page-body">
      <a-row :gutter="18">
        <a-col :xs="24" :xl="14">
          <a-card size="small" title="新增记忆" class="editor">
            <a-space direction="vertical" style="width:100%">
              <a-space wrap>
                <a-select v-model:value="form.scope" style="width:170px" :options="scopeOptions" />
                <a-input
                  v-if="form.scope === 'product'"
                  v-model:value="form.product_id"
                  placeholder="产品标识，例如 payment"
                  style="width:190px"
                />
                <a-select v-model:value="form.kind" style="width:110px">
                  <a-select-option value="fact">事实</a-select-option>
                  <a-select-option value="fix">处置</a-select-option>
                  <a-select-option value="preference">偏好</a-select-option>
                </a-select>
              </a-space>
              <a-alert
                v-if="form.scope === 'personal'"
                message="仅你本人可以查看和召回"
                type="info"
                show-icon
              />
              <a-alert
                v-else-if="form.scope === 'team'"
                :message="`团队 ${user.team_id} 的成员可以查看和召回`"
                type="info"
                show-icon
              />
              <a-textarea
                v-model:value="form.content"
                :rows="3"
                placeholder="例如：payment 数据库连接池上限为 50，历史上曾因连接耗尽触发超时"
              />
              <a-button type="primary" :loading="saving" @click="save">保存记忆</a-button>
            </a-space>
          </a-card>
        </a-col>

        <a-col :xs="24" :xl="10">
          <a-card size="small" title="从诊断内容提炼" class="editor">
            <a-space direction="vertical" style="width:100%">
              <a-input v-model:value="extractPid" placeholder="产品标识（可选）" />
              <a-textarea
                v-model:value="extractText"
                :rows="3"
                placeholder="粘贴诊断结论，让模型提炼为可复用的一句话经验"
              />
              <a-button :loading="extracting" @click="extract">生成草稿</a-button>
            </a-space>
          </a-card>
        </a-col>
      </a-row>

      <div class="section-title">
        <span>可访问记忆 · {{ total }}</span>
        <a-button size="small" @click="load">刷新</a-button>
      </div>
      <a-table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        row-key="id"
        :pagination="{ pageSize: 10 }"
        size="small"
        :scroll="{ x: 980 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'scope'">
            <a-tag :color="scopeColor(record.scope)">{{ scopeText(record) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'kind'">
            <a-tag :color="kindColor(record.kind)">{{ kindText(record.kind) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'time'">
            {{ new Date(record.created_at).toLocaleString() }}
          </template>
          <template v-else-if="column.key === 'operations'">
            <a-popconfirm v-if="canDelete(record)" title="确认删除这条记忆？" @confirm="remove(record)">
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
.page-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; }
.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.identity { display: flex; align-items: center; gap: 10px; color: #475467; font-size: 13px; }
.page-body { padding: 22px 26px; }
.editor { margin-bottom: 18px; min-height: 250px; }
.section-title { display: flex; align-items: center; justify-content: space-between; margin: 0 0 10px; font-size: 14px; font-weight: 600; color: #344054; }
@media (max-width: 1199px) {
  .editor { min-height: 0; }
  .page-head { align-items: flex-start; }
}
</style>
