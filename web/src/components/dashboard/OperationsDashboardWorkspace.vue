<script setup lang="ts">
import { onMounted } from 'vue'

import ModerationMetricGrid from './ModerationMetricGrid.vue'
import WebhookMetricSummary from './WebhookMetricSummary.vue'
import { useOperationsStats } from '@/composables/useOperationsStats'

const props = defineProps<{
  token: string
}>()

const emit = defineEmits<{
  unauthorized: []
}>()

const operations = useOperationsStats({
  token: props.token,
  onUnauthorized: () => emit('unauthorized'),
})

onMounted(operations.load)
</script>

<template>
  <div class="dashboard-workspace">
    <header class="dashboard-header">
      <div>
        <p class="eyebrow">Operations snapshot</p>
        <h1>运营概览</h1>
        <p>审核决策、人工复核与 Webhook 投递的当前累计状态。</p>
      </div>
      <button type="button" :disabled="operations.isLoading.value" @click="operations.load">
        {{ operations.isLoading.value ? '正在刷新…' : '刷新指标' }}
      </button>
    </header>

    <p v-if="operations.error.value" class="feedback-error" role="alert">
      {{ operations.error.value }}
    </p>
    <div v-if="operations.isLoading.value && !operations.stats.value" class="loading-state" role="status">
      正在加载运营指标…
    </div>
    <div v-else-if="operations.stats.value" class="dashboard-content">
      <ModerationMetricGrid :stats="operations.stats.value" />
      <WebhookMetricSummary :stats="operations.stats.value" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.dashboard-workspace {
  @apply min-h-0 flex-1 overflow-y-auto bg-[#f4f1e8] px-4 py-5 md:px-7 md:py-6;
}

.dashboard-header {
  @apply mb-6 flex flex-col justify-between gap-4 border-b border-[#d8d8cd] pb-5 sm:flex-row sm:items-end;
}

.eyebrow {
  @apply mb-1 text-[0.65rem] font-bold uppercase tracking-[0.18em] text-[#6b795e];
}

.dashboard-header h1 {
  @apply text-2xl font-bold text-[#1d3222];
}

.dashboard-header div > p:last-child {
  @apply mt-1 text-sm text-[#68726a];
}

.dashboard-header button {
  @apply min-h-11 rounded-lg bg-[#294b32] px-4 text-sm font-semibold text-white transition hover:bg-[#203d28] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/20 disabled:cursor-not-allowed disabled:opacity-60;
}

.feedback-error {
  @apply mb-4 rounded-lg border border-[#e0b7b0] bg-[#f7e8e5] px-4 py-3 text-sm text-[#8b392f];
}

.loading-state {
  @apply rounded-xl border border-[#d8d8cd] bg-white px-5 py-12 text-center text-sm text-[#68726a];
}

.dashboard-content {
  @apply space-y-5;
}
</style>
