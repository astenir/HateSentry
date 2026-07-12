<script setup lang="ts">
import type { ReviewCase } from '@/types'

defineProps<{
  items: readonly ReviewCase[]
  selectedId?: number
  loading: boolean
  loadingMore: boolean
  hasMore: boolean
}>()

const emit = defineEmits<{
  select: [id: number]
  loadMore: []
}>()

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

const statusLabels: Record<ReviewCase['status'], string> = {
  pending: '待复核',
  approved: '已通过',
  rejected: '已拒绝',
  mistake: '误判',
}

function formatDate(value: string): string {
  return dateFormatter.format(new Date(value))
}
</script>

<template>
  <section class="history-list-panel" aria-labelledby="history-list-title">
    <div class="list-heading">
      <h3 id="history-list-title">处理记录</h3>
      <span>{{ items.length }} 条</span>
    </div>
    <div v-if="loading" class="list-state" role="status">正在加载审核历史…</div>
    <div v-else-if="items.length === 0" class="list-state">当前筛选下没有已处理案件。</div>
    <ol v-else class="history-list">
      <li v-for="item in items" :key="item.id">
        <button
          class="history-item"
          :class="{ 'history-item-selected': selectedId === item.id }"
          :aria-current="selectedId === item.id ? 'true' : undefined"
          type="button"
          @click="emit('select', item.id)"
        >
          <span class="item-topline">
            <span class="status-badge" :class="`status-${item.status}`">
              {{ statusLabels[item.status] }}
            </span>
            <time :datetime="item.reviewed_at || item.created_at">
              {{ formatDate(item.reviewed_at || item.created_at) }}
            </time>
          </span>
          <strong>{{ item.content }}</strong>
          <span class="item-decisions">
            策略 {{ item.policy_decision }} → 人工 {{ item.final_decision || '—' }}
          </span>
          <span class="item-operator">
            {{ item.reviewer_id ? `操作员 #${item.reviewer_id}` : '操作员未知' }}
            · {{ item.source || 'unknown' }}
          </span>
        </button>
      </li>
    </ol>
    <div v-if="!loading && items.length > 0" class="pagination-footer">
      <button
        v-if="hasMore"
        type="button"
        :disabled="loadingMore"
        @click="emit('loadMore')"
      >
        {{ loadingMore ? '正在加载…' : '加载更多' }}
      </button>
      <span v-else>已加载全部记录</span>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.history-list-panel {
  @apply flex min-h-0 flex-col border-b border-[#d8d8cd] bg-[#f4f1e8] lg:border-r lg:border-b-0;
}

.list-heading {
  @apply flex items-center justify-between border-b border-[#d8d8cd] px-5 py-4;
}

.list-heading h3 {
  @apply text-sm font-semibold text-[#263129];
}

.list-heading span {
  @apply text-xs text-[#758079];
}

.list-state {
  @apply p-6 text-sm leading-6 text-[#687169];
}

.history-list {
  @apply max-h-96 min-h-0 overflow-y-auto p-3 lg:max-h-none;
}

.history-item {
  @apply mb-2 flex w-full flex-col rounded-2xl border border-transparent px-4 py-4 text-left transition hover:border-[#c6cabe] hover:bg-white/70 focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15;
}

.history-item-selected {
  @apply border-[#9cad99] bg-white shadow-sm;
}

.item-topline {
  @apply flex w-full items-center justify-between gap-3 text-[0.68rem] text-[#7c847d];
}

.status-badge {
  @apply rounded-full px-2 py-1 font-bold;
}

.status-approved {
  @apply bg-[#deedda] text-[#37633f];
}

.status-rejected {
  @apply bg-[#f2d9d4] text-[#8b392f];
}

.status-mistake {
  @apply bg-[#e6def1] text-[#5e4478];
}

.status-pending {
  @apply bg-[#fff0cf] text-[#8a5a13];
}

.history-item strong {
  @apply mt-3 line-clamp-2 text-sm font-medium leading-5 text-[#263129];
}

.item-decisions {
  @apply mt-3 text-xs font-semibold text-[#4e5c51];
}

.item-operator {
  @apply mt-1 text-xs text-[#758078];
}

.pagination-footer {
  @apply border-t border-[#d8d8cd] p-3 text-center;
}

.pagination-footer button {
  @apply min-h-11 w-full rounded-xl border border-[#c7cabf] bg-white px-4 text-sm font-semibold text-[#334037] transition hover:bg-[#f8f7f1] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:cursor-wait disabled:opacity-50;
}

.pagination-footer span {
  @apply text-xs text-[#758078];
}
</style>
