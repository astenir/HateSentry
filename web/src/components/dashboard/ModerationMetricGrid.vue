<script setup lang="ts">
import { computed } from 'vue'

import type { OperationsStats } from '@/types'

const props = defineProps<{
  stats: OperationsStats
}>()

const decisionRows = computed(() => [
  { key: 'allow', label: '最终允许', value: props.stats.allowed, color: 'bg-[#4f7c58]' },
  { key: 'review', label: '待人工复核', value: props.stats.pending_review, color: 'bg-[#d69a43]' },
  { key: 'block', label: '最终阻断', value: props.stats.blocked, color: 'bg-[#a34d42]' },
])

function percentage(value: number, total: number): number {
  if (total === 0) return 0
  return Math.min(100, Math.round((value / total) * 1000) / 10)
}

function formatRate(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'percent',
    maximumFractionDigits: 1,
  }).format(value)
}
</script>

<template>
  <section aria-labelledby="moderation-metrics-heading">
    <div class="section-heading">
      <div>
        <p class="eyebrow">Moderation</p>
        <h2 id="moderation-metrics-heading">审核运营</h2>
      </div>
      <p>当前持久化记录的累计快照</p>
    </div>

    <dl class="metric-grid">
      <div class="metric-card metric-primary">
        <dt>审核总量</dt>
        <dd data-testid="total-moderated">{{ stats.total_moderated }}</dd>
        <p>已生成审核结果的请求</p>
      </div>
      <div class="metric-card">
        <dt>已完成人工复核</dt>
        <dd>{{ stats.reviewed }}</dd>
        <p>通过、拒绝及标记误判</p>
      </div>
      <div class="metric-card metric-warning">
        <dt>待人工复核</dt>
        <dd>{{ stats.pending_review }}</dd>
        <p>尚未形成最终人工决定</p>
      </div>
      <div class="metric-card">
        <dt>误判率</dt>
        <dd>{{ formatRate(stats.mistake_rate) }}</dd>
        <p>{{ stats.mistakes }} 条误判 / {{ stats.reviewed }} 条已复核</p>
      </div>
    </dl>

    <div class="decision-panel">
      <div class="decision-copy">
        <h3>当前最终决策分布</h3>
        <p>人工已处理的 review 记录按最终决定归入允许或阻断。</p>
      </div>
      <ul class="decision-list">
        <li v-for="row in decisionRows" :key="row.key">
          <div class="decision-label">
            <span>{{ row.label }}</span>
            <strong>{{ row.value }} · {{ percentage(row.value, stats.total_moderated) }}%</strong>
          </div>
          <div
            class="decision-track"
            role="progressbar"
            :aria-label="row.label"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuenow="percentage(row.value, stats.total_moderated)"
          >
            <span :class="row.color" :style="{ width: `${percentage(row.value, stats.total_moderated)}%` }"></span>
          </div>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.section-heading {
  @apply mb-4 flex flex-col justify-between gap-2 sm:flex-row sm:items-end;
}

.section-heading h2 {
  @apply text-xl font-bold text-[#1d3222];
}

.section-heading > p,
.decision-copy p {
  @apply text-xs text-[#68726a];
}

.eyebrow {
  @apply mb-1 text-[0.65rem] font-bold uppercase tracking-[0.18em] text-[#6b795e];
}

.metric-grid {
  @apply grid gap-3 sm:grid-cols-2 xl:grid-cols-4;
}

.metric-card {
  @apply rounded-xl border border-[#d8d8cd] bg-white p-4 shadow-sm;
}

.metric-card dt {
  @apply text-xs font-semibold text-[#68726a];
}

.metric-card dd {
  @apply mt-2 text-3xl font-bold tabular-nums text-[#223b29];
}

.metric-card p {
  @apply mt-2 text-xs leading-5 text-[#778078];
}

.metric-primary {
  @apply border-[#b8c8b5] bg-[#edf4e9];
}

.metric-warning {
  @apply border-[#e5cfaa] bg-[#fff8e9];
}

.decision-panel {
  @apply mt-3 grid gap-5 rounded-xl border border-[#d8d8cd] bg-white p-4 shadow-sm lg:grid-cols-[16rem_minmax(0,1fr)];
}

.decision-copy h3 {
  @apply mb-1 text-sm font-bold text-[#26392b];
}

.decision-list {
  @apply space-y-3;
}

.decision-label {
  @apply mb-1 flex items-center justify-between gap-4 text-xs text-[#526057];
}

.decision-label strong {
  @apply tabular-nums text-[#26392b];
}

.decision-track {
  @apply h-2 overflow-hidden rounded-full bg-[#ecece5];
}

.decision-track span {
  @apply block h-full rounded-full;
}
</style>
