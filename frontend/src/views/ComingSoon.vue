<script setup>
defineProps({ title: { type: String, default: '功能' }, desc: { type: String, default: '' } })
</script>

<template>
  <div class="page">
    <header class="page-head"><h2>{{ title }}</h2><p>{{ desc }}</p></header>
    <div class="page-body center">
      <a-result :title="`${title} · 即将推出`" :sub-title="desc">
        <template #icon><span style="font-size:56px">🚧</span></template>
      </a-result>
    </div>
  </div>
</template>

<style scoped>
.page { height: 100%; width: 100%; overflow-y: auto; }
.page-head { padding: 20px 26px; background: #fff; border-bottom: 1px solid #edeff2; }
.page-head h2 { margin: 0; font-size: 18px; } .page-head p { margin: 4px 0 0; color: #98a2b3; font-size: 13px; }
.page-body { padding: 22px 26px; }
.center { display: grid; place-items: center; min-height: 60vh; }
</style>
