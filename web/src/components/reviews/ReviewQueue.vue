<script setup lang="ts">
import type { ReviewCase } from '@/types'

defineProps<{
  items: readonly ReviewCase[]
  selectedId?: number
  loading: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  select: [id: number]
  refresh: []
}>()

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

function formatDate(value: string): string {
  return dateFormatter.format(new Date(value))
}
</script>

<template>
  <section class="queue-panel" aria-labelledby="queue-title">
    <header class="queue-header">
      <div>
        <p class="section-kicker">PENDING</p>
        <div class="title-row">
          <h2 id="queue-title" class="section-title">待复核</h2>
          <span class="count-badge">{{ items.length }}</span>
        </div>
      </div>
      <button class="icon-button" type="button" :disabled="loading || busy" @click="emit('refresh')">
        <span aria-hidden="true">↻</span>
        <span class="sr-only">刷新待复核队列</span>
      </button>
    </header>

    <div v-if="loading" class="queue-state" role="status">正在加载待复核内容…</div>
    <div v-else-if="items.length === 0" class="queue-empty">
      <span class="empty-check" aria-hidden="true">✓</span>
      <strong>队列已经清空</strong>
      <p>新的 review 决策会自动出现在这里。</p>
    </div>
    <ol v-else class="queue-list">
      <li v-for="item in items" :key="item.id">
        <button
          class="queue-item"
          :class="{ 'queue-item-selected': selectedId === item.id }"
          :aria-current="selectedId === item.id ? 'true' : undefined"
          :disabled="busy"
          type="button"
          @click="emit('select', item.id)"
        >
          <span class="queue-item-topline">
            <span class="source-pill">{{ item.source || 'unknown' }}</span>
            <time :datetime="item.created_at">{{ formatDate(item.created_at) }}</time>
          </span>
          <span class="content-preview">{{ item.content }}</span>
          <span class="queue-item-meta">
            <span>风险 {{ Math.round(item.risk_score * 100) }}</span>
            <span>{{ item.labels.slice(0, 2).join(' · ') || '未标记' }}</span>
          </span>
        </button>
      </li>
    </ol>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.queue-panel {
  @apply flex min-h-0 flex-col border-b border-[#d8d8cd] bg-[#f4f1e8] lg:border-r lg:border-b-0;
}

.queue-header {
  @apply flex items-start justify-between border-b border-[#d8d8cd] px-5 py-5 md:px-6;
}

.section-kicker {
  @apply text-[0.65rem] font-bold tracking-[0.22em] text-[#758079];
}

.title-row {
  @apply mt-1 flex items-center gap-2;
}

.section-title {
  @apply text-xl font-semibold tracking-[-0.025em] text-[#18221a];
}

.count-badge {
  @apply rounded-full bg-[#dde5d9] px-2 py-0.5 text-xs font-bold text-[#38533d];
}

.icon-button {
  @apply grid h-11 w-11 place-items-center rounded-full border border-[#c7cabf] text-lg text-[#445148] transition hover:bg-white focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-40;
}

.queue-state {
  @apply p-6 text-sm text-[#687169];
}

.queue-empty {
  @apply m-5 flex min-h-48 flex-col items-center justify-center rounded-2xl border border-dashed border-[#bdc3b7] px-6 text-center text-[#59645b];
}

.queue-empty strong {
  @apply mt-3 text-sm text-[#263329];
}

.queue-empty p {
  @apply mt-1 text-xs leading-5;
}

.empty-check {
  @apply grid h-10 w-10 place-items-center rounded-full bg-[#dce8d9] text-lg font-bold text-[#3f6948];
}

.queue-list {
  @apply max-h-96 min-h-0 overflow-y-auto p-3 md:p-4 lg:max-h-none;
}

.queue-item {
  @apply mb-2 flex w-full flex-col rounded-2xl border border-transparent px-4 py-4 text-left transition hover:border-[#c6cabe] hover:bg-white/70 focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:cursor-wait disabled:opacity-60;
}

.queue-item-selected {
  @apply border-[#9cad99] bg-white shadow-sm;
}

.queue-item-topline {
  @apply flex w-full items-center justify-between text-[0.68rem] text-[#7c847d];
}

.source-pill {
  @apply rounded-full bg-[#e7e4da] px-2 py-1 font-bold uppercase tracking-wider text-[#566057];
}

.content-preview {
  @apply mt-3 line-clamp-2 text-sm font-medium leading-5 text-[#263129];
}

.queue-item-meta {
  @apply mt-3 flex w-full items-center justify-between gap-3 text-xs text-[#758078];
}
</style>
