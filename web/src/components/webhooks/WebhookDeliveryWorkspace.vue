<script setup lang="ts">
import { onMounted } from 'vue'
import WebhookDeliveryFilters from './WebhookDeliveryFilters.vue'
import WebhookDeliveryList from './WebhookDeliveryList.vue'
import { useWebhookDeliveries } from '@/composables/useWebhookDeliveries'
import type { WebhookDeliveryFilter } from '@/types'
const props = defineProps<{ token: string }>()
const emit = defineEmits<{ unauthorized: [] }>()
const deliveries = useWebhookDeliveries({ token: props.token, onUnauthorized: () => emit('unauthorized') })
onMounted(deliveries.load)
function apply(filter: WebhookDeliveryFilter) { void deliveries.load(filter) }
</script>
<template><div class="workspace"><div v-if="deliveries.error.value" class="error" role="alert">{{ deliveries.error.value }}</div><div v-if="deliveries.notice.value" class="notice" role="status">{{ deliveries.notice.value }}</div><WebhookDeliveryFilters :model-value="deliveries.filter.value" :busy="deliveries.isLoading.value || deliveries.retryingIds.value.size > 0" @apply="apply"/><WebhookDeliveryList :items="deliveries.items.value" :loading="deliveries.isLoading.value" :retrying-ids="deliveries.retryingIds.value" @retry="deliveries.retry"/></div></template>
<style scoped>@reference "../../styles.css";.workspace { @apply min-h-0 flex-1 overflow-y-auto bg-[#f4f1e8]; }.error { @apply bg-[#f7e8e5] px-5 py-3 text-sm text-[#8b392f]; }.notice { @apply bg-[#e8f2e5] px-5 py-3 text-sm text-[#37633f]; }</style>
