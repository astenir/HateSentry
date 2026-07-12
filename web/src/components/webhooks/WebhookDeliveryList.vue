<script setup lang="ts">
import { shallowRef } from 'vue'
import type { WebhookDelivery } from '@/types'

defineProps<{ items: readonly WebhookDelivery[], loading: boolean, retryingIds: ReadonlySet<number> }>()
const emit = defineEmits<{ retry: [delivery: WebhookDelivery] }>()
const confirming = shallowRef<number | null>(null)
const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' })
function retry(item: WebhookDelivery) {
  if (confirming.value !== item.id) { confirming.value = item.id; return }
  confirming.value = null
  emit('retry', item)
}
</script>

<template>
  <section class="delivery-list" aria-labelledby="delivery-title">
    <div class="heading"><div><h2 id="delivery-title">Webhook 投递记录</h2><p>每条记录展示该 delivery 的最新状态，不是逐次尝试明细。</p></div><span>{{ items.length }} 条</span></div>
    <div v-if="loading" class="empty" role="status">正在加载投递记录…</div>
    <div v-else-if="items.length === 0" class="empty">当前筛选下没有投递记录。</div>
    <ol v-else>
      <li v-for="item in items" :key="item.id">
        <div class="topline"><span class="status" :class="`status-${item.status}`">{{ item.status }}</span><time :datetime="item.updated_at">{{ formatter.format(new Date(item.updated_at)) }}</time></div>
        <div class="ids"><strong :title="item.request_id">{{ item.request_id }}</strong><span>客户端 #{{ item.client_id }} · delivery #{{ item.id }}</span></div>
        <dl><div><dt>事件</dt><dd>{{ item.event }}</dd></div><div><dt>尝试次数</dt><dd>{{ item.attempt_count }}</dd></div><div><dt>HTTP 状态</dt><dd>{{ item.http_status ?? '—' }}</dd></div><div><dt>最后尝试</dt><dd>{{ formatter.format(new Date(item.last_attempt_at)) }}</dd></div></dl>
        <p v-if="item.error_message" class="error-summary" :title="item.error_message">{{ item.error_message }}</p>
        <div v-if="item.status === 'failed'" class="actions">
          <button type="button" :class="{ confirming: confirming === item.id }" :disabled="loading || retryingIds.has(item.id)" @click="retry(item)">{{ retryingIds.has(item.id) ? '重试中…' : confirming === item.id ? '确认立即重试' : '手动重试' }}</button>
          <button v-if="confirming === item.id" type="button" class="cancel" @click="confirming = null">取消</button>
        </div>
      </li>
    </ol>
  </section>
</template>

<style scoped>
@reference "../../styles.css";
.delivery-list { @apply mx-auto my-5 w-[calc(100%-2rem)] max-w-6xl overflow-hidden rounded-2xl border border-[#d8d8cd] bg-white shadow-sm md:w-[calc(100%-3.5rem)]; }
.heading { @apply flex items-end justify-between gap-4 border-b border-[#d8d8cd] p-5; }
.heading h2 { @apply text-xl font-bold text-[#263129]; }.heading p { @apply mt-1 text-xs text-[#758078]; }.heading span { @apply text-xs text-[#758078]; }
.empty { @apply p-8 text-center text-sm text-[#687169]; } ol { @apply grid gap-3 bg-[#f4f1e8] p-3; } li { @apply rounded-xl border border-[#deded4] bg-white p-4; }
.topline { @apply flex items-center justify-between gap-3 text-xs text-[#758078]; }.status { @apply rounded-full px-2 py-1 font-bold; }.status-succeeded { @apply bg-[#deedda] text-[#37633f]; }.status-failed { @apply bg-[#f2d9d4] text-[#8b392f]; }.status-retrying { @apply bg-[#fff0cf] text-[#8a5a13]; }
.ids { @apply mt-3 flex min-w-0 flex-col gap-1; }.ids strong { @apply truncate text-sm text-[#263129]; }.ids span { @apply text-xs text-[#758078]; }
dl { @apply mt-3 grid grid-cols-2 gap-2 lg:grid-cols-4; } dl div { @apply rounded-lg bg-[#f6f5ef] p-2; } dt { @apply text-[0.68rem] text-[#758078]; } dd { @apply mt-1 truncate text-xs font-bold text-[#445047]; }
.error-summary { @apply mt-3 line-clamp-2 rounded-lg bg-[#fff2ef] p-3 text-xs leading-5 text-[#8b392f]; }
.actions { @apply mt-3 flex flex-wrap gap-2; }.actions button { @apply min-h-11 rounded-lg border border-[#d5a99f] bg-[#fff4f1] px-3 text-xs font-bold text-[#8b392f] disabled:opacity-50; }.actions .confirming { @apply border-[#a95747] bg-[#f7ded8]; }.actions .cancel { @apply border-[#d5d0c3] bg-white text-[#5f625d]; }
</style>
