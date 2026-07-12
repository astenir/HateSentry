<script setup lang="ts">
import { computed } from 'vue'

import type { OperationsStats } from '@/types'

const props = defineProps<{
  stats: OperationsStats
}>()

const rows = computed(() => [
  { key: 'succeeded', label: '成功', value: props.stats.webhook_succeeded, dot: 'bg-[#4f7c58]' },
  { key: 'failed', label: '失败', value: props.stats.webhook_failed, dot: 'bg-[#a34d42]' },
  { key: 'retrying', label: '重试中', value: props.stats.webhook_retrying, dot: 'bg-[#d69a43]' },
])
</script>

<template>
  <section class="webhook-panel" aria-labelledby="webhook-metrics-heading">
    <div>
      <p class="eyebrow">Webhook</p>
      <h2 id="webhook-metrics-heading">投递健康度</h2>
      <p class="scope-note">按每个 delivery 的最新状态计数，不代表逐次投递尝试次数。</p>
    </div>
    <dl class="webhook-total">
      <dt>投递记录</dt>
      <dd data-testid="webhook-total">{{ stats.webhook_total }}</dd>
    </dl>
    <ul class="status-list">
      <li v-for="row in rows" :key="row.key">
        <span class="status-name"><i :class="row.dot" aria-hidden="true"></i>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </li>
    </ul>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.webhook-panel {
  @apply grid gap-5 rounded-xl border border-[#cfd3c9] bg-[#202d24] p-5 text-[#f7f1df] shadow-sm md:grid-cols-[minmax(0,1fr)_9rem_15rem] md:items-center;
}

.eyebrow {
  @apply mb-1 text-[0.65rem] font-bold uppercase tracking-[0.18em] text-[#b8c9ad];
}

.webhook-panel h2 {
  @apply text-xl font-bold;
}

.scope-note {
  @apply mt-2 max-w-xl text-xs leading-5 text-[#b8c0b9];
}

.webhook-total {
  @apply border-l-0 border-white/10 md:border-l md:pl-5;
}

.webhook-total dt {
  @apply text-xs text-[#b8c0b9];
}

.webhook-total dd {
  @apply mt-1 text-3xl font-bold tabular-nums;
}

.status-list {
  @apply divide-y divide-white/10 rounded-lg bg-white/5 px-3;
}

.status-list li {
  @apply flex items-center justify-between py-2 text-xs;
}

.status-name {
  @apply flex items-center gap-2 text-[#d6ddd5];
}

.status-name i {
  @apply h-2 w-2 rounded-full;
}

.status-list strong {
  @apply tabular-nums;
}
</style>
