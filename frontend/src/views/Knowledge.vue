<script setup>
import { ref } from 'vue'
import { api } from '../api'

const q = ref('')
const rows = ref([])
const loading = ref(false)
const searched = ref(false)

async function search() {
  if (!q.value.trim()) return
  loading.value = true
  searched.value = true
  try { rows.value = (await api(`/knowledge/search?query=${encodeURIComponent(q.value)}&limit=8`)).items || [] }
  catch { rows.value = [] }
  finally { loading.value = false }
}
</script>

<template>
  <div class="page">
    <header class="page-head"><h2>知识库</h2><p>运维知识检索（当前 BM25 关键词，后续接向量 + RRF）</p></header>
    <div class="page-body">
      <a-input-search v-model:value="q" placeholder="输入检索关键词，如 告警检索、连接池、慢查询" enter-button="检索" :loading="loading" @search="search" style="max-width:580px" />
      <a-list style="margin-top:18px" :data-source="rows" :loading="loading" :locale="{ emptyText: searched ? '没有命中的知识片段' : '输入关键词开始检索' }">
        <template #renderItem="{ item }">
          <a-list-item>
            <a-card size="small" style="width:100%">
              <div style="display:flex;justify-content:space-between;align-items:center"><strong>{{ item.title }}</strong><a-tag color="blue">{{ Math.round(item.score * 100) }}%</a-tag></div>
              <p style="color:#667085;margin:8px 0">{{ item.content }}</p>
              <code style="color:#8c98a8;font-size:12px">{{ item.source }}</code>
            </a-card>
          </a-list-item>
        </template>
      </a-list>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; } .page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
</style>
