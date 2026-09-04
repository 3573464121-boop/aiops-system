<script setup>
import { onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { api } from '../api'

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const form = ref({ username: '', password: '', role: 'viewer', team_id: 'operations' })

const columns = [
  { title: '用户名', dataIndex: 'username' },
  { title: '角色', key: 'role', width: 120 },
  { title: '团队', key: 'team', width: 180 },
  { title: '创建时间', key: 'created', width: 190 },
]

async function load() {
  loading.value = true
  try {
    rows.value = (await api('/users')).items || []
  } catch (error) {
    rows.value = []
    message.error(error.message)
  } finally {
    loading.value = false
  }
}

async function createUser() {
  if (!form.value.username.trim() || !form.value.password) {
    message.warning('请填写用户名和初始密码')
    return
  }
  saving.value = true
  try {
    await api('/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    message.success('用户已创建')
    form.value = { username: '', password: '', role: 'viewer', team_id: 'operations' }
    await load()
  } catch (error) {
    message.error(error.message)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <h2>用户与团队</h2>
      <p>创建系统账号并设置角色和团队归属，团队信息用于共享记忆隔离</p>
    </header>

    <div class="page-body">
      <section class="create-bar">
        <a-input v-model:value="form.username" placeholder="用户名" style="width:180px" />
        <a-input-password v-model:value="form.password" placeholder="初始密码" style="width:200px" />
        <a-select v-model:value="form.role" style="width:130px">
          <a-select-option value="viewer">只读用户</a-select-option>
          <a-select-option value="admin">管理员</a-select-option>
        </a-select>
        <a-input v-model:value="form.team_id" placeholder="团队标识" style="width:180px" />
        <a-button type="primary" :loading="saving" @click="createUser">创建用户</a-button>
      </section>

      <div class="section-title">
        <span>系统用户 · {{ rows.length }}</span>
        <a-button size="small" @click="load">刷新</a-button>
      </div>
      <a-table
        :columns="columns"
        :data-source="rows"
        :loading="loading"
        row-key="id"
        :pagination="{ pageSize: 10 }"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'role'">
            <a-tag :color="record.role === 'admin' ? 'red' : 'blue'">
              {{ record.role === 'admin' ? '管理员' : '只读用户' }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'team'">
            <a-tag color="cyan">{{ record.team_id || '未分配' }}</a-tag>
          </template>
          <template v-else-if="column.key === 'created'">
            {{ new Date(record.created_at).toLocaleString() }}
          </template>
        </template>
      </a-table>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; }
.page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
.create-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 22px; padding: 16px; background: #fff; border: 1px solid #eaecf0; border-radius: 6px; }
.section-title { display: flex; align-items: center; justify-content: space-between; margin: 0 0 10px; font-size: 14px; font-weight: 600; color: #344054; }
</style>
