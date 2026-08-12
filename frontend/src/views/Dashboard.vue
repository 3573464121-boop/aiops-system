<script setup>
import { ref, onMounted } from 'vue'
import { api, sevText, sevColor } from '../api'

const status = ref(null)
const alerts = ref([])
const issues = ref([])
const audits = ref([])

onMounted(async () => {
  try { status.value = await api('/system/status') } catch { /* ignore */ }
  try { alerts.value = (await api('/alerts/active')).items || [] } catch { /* ignore */ }
  try { issues.value = (await api('/issues')).items || [] } catch { /* ignore */ }
  try { audits.value = (await api('/audits')).items || [] } catch { /* ignore */ }
})
</script>

<template>
  <div class="page">
    <header class="page-head"><h2>仪表盘</h2><p>系统总览与实时信号</p></header>
    <div class="page-body">
      <a-row :gutter="16">
        <a-col :span="6"><a-card><a-statistic title="活跃告警" :value="alerts.length" /></a-card></a-col>
        <a-col :span="6"><a-card><a-statistic title="问题工单" :value="issues.length" /></a-card></a-col>
        <a-col :span="6"><a-card><a-statistic title="知识库分块" :value="status?.knowledge_chunks || 0" /></a-card></a-col>
        <a-col :span="6"><a-card><a-statistic title="审计事件" :value="audits.length" /></a-card></a-col>
      </a-row>

      <a-row :gutter="16" style="margin-top:16px">
        <a-col :span="14">
          <a-card title="活跃告警">
            <a-list :data-source="alerts" size="small" :locale="{ emptyText: '暂无告警' }">
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-tag :color="sevColor(item.severity)">{{ sevText(item.severity) }}</a-tag>
                  <span style="flex:1">{{ item.rule }} · {{ item.target }}</span>
                  <span style="color:#98a2b3">{{ item.value }}</span>
                </a-list-item>
              </template>
            </a-list>
          </a-card>
        </a-col>
        <a-col :span="10">
          <a-card title="运行状态">
            <p>后端：<a-tag color="green">在线</a-tag></p>
            <p>安全模式：<a-tag color="blue">只读</a-tag></p>
            <p>大模型：<a-tag>{{ status?.llm_provider || '-' }}</a-tag></p>
            <p>存储：<a-tag>{{ status?.storage_provider || '-' }}</a-tag></p>
            <p>知识库：<a-tag>{{ status?.knowledge_provider || '-' }}</a-tag></p>
          </a-card>
        </a-col>
      </a-row>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; } .page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
</style>
