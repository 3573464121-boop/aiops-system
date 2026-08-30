<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { UserOutlined, LockOutlined, ThunderboltFilled } from '@ant-design/icons-vue'
import { API, setSession } from '../api'

const router = useRouter()
const form = ref({ username: '', password: '' })
const loading = ref(false)

async function submit() {
  if (!form.value.username.trim() || !form.value.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const resp = await fetch(`${API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) throw new Error(data.error || `登录失败（${resp.status}）`)
    setSession(data.token, data.user)
    message.success(`欢迎回来，${data.user.username}`)
    router.replace('/')
  } catch (e) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <div class="login-card">
      <div class="brand">
        <ThunderboltFilled class="brand-ic" />
        <div>
          <h1>智能运维</h1>
          <p>AIOps 故障诊断平台</p>
        </div>
      </div>
      <a-input v-model:value="form.username" size="large" placeholder="用户名" class="fld" @keyup.enter="submit">
        <template #prefix><UserOutlined /></template>
      </a-input>
      <a-input-password v-model:value="form.password" size="large" placeholder="密码" class="fld" @keyup.enter="submit">
        <template #prefix><LockOutlined /></template>
      </a-input-password>
      <a-button type="primary" size="large" block :loading="loading" @click="submit">登录</a-button>
      <p class="hint">首次使用默认管理员账号 admin，口令见后端启动日志</p>
    </div>
  </div>
</template>

<style scoped>
.login-wrap { height: 100vh; width: 100%; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #0f1b2d 0%, #1a2b45 100%); }
.login-card { width: 360px; background: #fff; border-radius: 16px; padding: 36px 32px;
  box-shadow: 0 12px 40px rgba(0,0,0,.25); display: flex; flex-direction: column; gap: 16px; }
.brand { display: flex; align-items: center; gap: 14px; margin-bottom: 8px; }
.brand-ic { font-size: 38px; color: #1677ff; }
.brand h1 { margin: 0; font-size: 20px; color: #1f2733; }
.brand p { margin: 2px 0 0; font-size: 12px; color: #98a2b3; }
.fld { border-radius: 10px; }
.hint { margin: 4px 0 0; text-align: center; font-size: 12px; color: #b0b8c4; }
</style>
