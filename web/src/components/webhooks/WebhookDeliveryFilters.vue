<script setup lang="ts">
import { ref } from 'vue'
import type { WebhookDeliveryFilter } from '@/types'

const props = defineProps<{ modelValue: WebhookDeliveryFilter, busy: boolean }>()
const emit = defineEmits<{ apply: [filter: WebhookDeliveryFilter] }>()
const draft = ref({ ...props.modelValue })
</script>

<template>
  <form class="filters" @submit.prevent="emit('apply', { ...draft })">
    <label>状态<select v-model="draft.status" :disabled="busy"><option value="all">全部</option><option value="failed">失败</option><option value="retrying">重试中</option><option value="succeeded">成功</option></select></label>
    <label>客户端 ID<input v-model="draft.clientId" inputmode="numeric" placeholder="例如：12" :disabled="busy" /></label>
    <label>请求 ID<input v-model="draft.requestId" placeholder="request UUID" :disabled="busy" /></label>
    <button type="submit" :disabled="busy">{{ busy ? '查询中…' : '应用筛选' }}</button>
  </form>
</template>

<style scoped>
@reference "../../styles.css";
.filters { @apply grid gap-3 border-b border-[#d8d8cd] bg-[#fbfaf5] p-4 md:grid-cols-[12rem_12rem_minmax(16rem,1fr)_auto] md:items-end md:px-7; }
.filters label { @apply flex flex-col gap-1 text-xs font-bold text-[#4c584f]; }
.filters input, .filters select { @apply min-h-11 rounded-xl border border-[#c7cabf] bg-white px-3 text-sm font-normal outline-none focus:border-[#62806a] focus:ring-4 focus:ring-[#456b4d]/15; }
.filters button { @apply min-h-11 rounded-xl bg-[#31593a] px-5 text-sm font-bold text-white disabled:opacity-50; }
</style>
