<script setup lang="ts">
import type { ReviewHistoryFilter } from '@/types'

defineProps<{
  modelValue: ReviewHistoryFilter
  loading: boolean
}>()

const emit = defineEmits<{
  change: [filter: ReviewHistoryFilter]
  refresh: []
}>()

function handleChange(event: Event): void {
  emit('change', (event.target as HTMLSelectElement).value as ReviewHistoryFilter)
}
</script>

<template>
  <div class="history-toolbar">
    <div>
      <p class="section-kicker">AUDIT TRAIL</p>
      <h2 id="history-title">审核历史</h2>
    </div>
    <div class="filter-actions">
      <label for="history-status">人工状态</label>
      <select
        id="history-status"
        :value="modelValue"
        :disabled="loading"
        @change="handleChange"
      >
        <option value="all">全部已处理</option>
        <option value="approved">已通过</option>
        <option value="rejected">已拒绝</option>
        <option value="mistake">已标记误判</option>
      </select>
      <button type="button" :disabled="loading" @click="emit('refresh')">
        刷新历史
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.history-toolbar {
  @apply flex flex-col gap-4 border-b border-[#d8d8cd] bg-[#f4f1e8] px-5 py-5 sm:flex-row sm:items-end sm:justify-between md:px-7;
}

.section-kicker {
  @apply text-[0.65rem] font-bold tracking-[0.22em] text-[#758079];
}

.history-toolbar h2 {
  @apply mt-1 text-2xl font-semibold tracking-[-0.03em] text-[#18221a];
}

.filter-actions {
  @apply flex flex-wrap items-center gap-2;
}

.filter-actions label {
  @apply text-xs font-semibold text-[#5d685f];
}

.filter-actions select,
.filter-actions button {
  @apply min-h-11 rounded-xl border border-[#c7cabf] bg-white px-3 text-sm text-[#334037] outline-none transition focus:border-[#456b4d] focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-50;
}

.filter-actions button {
  @apply font-semibold hover:bg-[#f8f7f1];
}
</style>
