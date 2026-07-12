<script setup lang="ts">
import { shallowRef } from 'vue'

import ClientPolicyEditor from './ClientPolicyEditor.vue'
import type { ClientApplication, ModerationPolicy } from '@/types'

defineProps<{
  items: readonly ClientApplication[]
  loading: boolean
  busyClientIds: ReadonlySet<number>
  credentialOpen: boolean
  policies: readonly ModerationPolicy[]
}>()

const emit = defineEmits<{
  setActive: [client: ClientApplication, active: boolean]
  rotate: [client: ClientApplication]
  assignPolicy: [client: ClientApplication, policyVersion: string]
  editWebhook: [clientId: number]
}>()

const confirmingRotation = shallowRef<number | null>(null)
const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
})

function formatDate(value: string): string {
  return dateFormatter.format(new Date(value))
}

function requestRotation(client: ClientApplication): void {
  if (confirmingRotation.value === client.id) {
    confirmingRotation.value = null
    emit('rotate', client)
    return
  }
  confirmingRotation.value = client.id
}

function requestStatus(client: ClientApplication): void {
  confirmingRotation.value = null
  emit('setActive', client, client.status !== 'active')
}

function assignPolicy(client: ClientApplication, policyVersion: string): void {
  confirmingRotation.value = null
  emit('assignPolicy', client, policyVersion)
}
</script>

<template>
  <section class="client-list-panel" aria-labelledby="client-list-title">
    <div class="list-heading">
      <div>
        <h3 id="client-list-title">外部客户端</h3>
        <p>可分配策略，并配置客户端 Webhook 回调。</p>
      </div>
      <span>{{ items.length }} 个客户端</span>
    </div>

    <div v-if="loading" class="list-state" role="status">正在加载客户端…</div>
    <div v-else-if="items.length === 0" class="list-state">尚未创建外部客户端。</div>
    <div v-else class="client-table-wrap">
      <table>
        <colgroup>
          <col class="col-client"><col class="col-status"><col class="col-prefix">
          <col class="col-policy"><col class="col-webhook"><col class="col-created"><col class="col-actions">
        </colgroup>
        <thead>
          <tr>
            <th>客户端</th><th>状态</th><th>API Key 前缀</th><th>策略</th><th>Webhook</th><th>创建时间</th><th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="client in items" :key="client.id">
            <td data-label="客户端"><strong :title="client.name">{{ client.name }}</strong><small>#{{ client.id }}</small></td>
            <td data-label="状态"><span class="status" :class="`status-${client.status}`">{{ client.status === 'active' ? '已启用' : '已停用' }}</span></td>
            <td data-label="API Key 前缀"><code :title="client.api_key_prefix">{{ client.api_key_prefix }}</code></td>
            <td data-label="策略">
              <ClientPolicyEditor
                :client="client"
                :policies="policies"
                :busy="busyClientIds.has(client.id)"
                @assign="assignPolicy"
              />
            </td>
            <td data-label="Webhook">
              <span
                :class="client.webhook_url ? 'configured' : 'muted'"
                :title="client.webhook_url || '未配置'"
              >{{ client.webhook_url || '未配置' }}</span>
              <button
                type="button"
                class="webhook-button"
                :aria-label="`配置 ${client.name} 的 Webhook`"
                :disabled="busyClientIds.has(client.id) || credentialOpen"
                @click="emit('editWebhook', client.id)"
              >{{ client.webhook_url ? '修改 Webhook' : '配置 Webhook' }}</button>
            </td>
            <td data-label="创建时间"><time :datetime="client.created_at">{{ formatDate(client.created_at) }}</time></td>
            <td data-label="操作">
              <div class="row-actions">
                <button
                  type="button"
                  :disabled="busyClientIds.has(client.id)"
                  @click="requestStatus(client)"
                >{{ busyClientIds.has(client.id) ? '处理中…' : client.status === 'active' ? '停用' : '启用' }}</button>
                <button
                  type="button"
                  class="rotate-button"
                  :class="{ confirming: confirmingRotation === client.id }"
                  :disabled="busyClientIds.has(client.id) || credentialOpen"
                  @click="requestRotation(client)"
                >{{ confirmingRotation === client.id ? '确认轮换并使旧 Key 失效' : '轮换 API Key' }}</button>
                <button
                  v-if="confirmingRotation === client.id"
                  type="button"
                  class="cancel-button"
                  @click="confirmingRotation = null"
                >取消</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.client-list-panel { @apply overflow-hidden rounded-2xl border border-[#d8d8cd] bg-white shadow-sm; }
.list-heading { @apply flex items-center justify-between gap-4 border-b border-[#d8d8cd] px-4 py-4 md:px-5; }
.list-heading h3 { @apply text-sm font-bold text-[#263129]; }
.list-heading p { @apply mt-1 text-xs text-[#758078]; }
.list-heading > span { @apply shrink-0 text-xs text-[#758078]; }
.list-state { @apply p-8 text-center text-sm text-[#687169]; }
.client-table-wrap { @apply min-w-0; }
table { @apply w-full table-fixed border-collapse text-left; }
.col-client { width: 13%; }
.col-status { width: 8%; }
.col-prefix { width: 10%; }
.col-policy { width: 20%; }
.col-webhook { width: 16%; }
.col-created { width: 13%; }
.col-actions { width: 20%; }
th { @apply bg-[#f4f1e8] px-3 py-3 text-[0.68rem] font-bold uppercase tracking-wide text-[#69736b]; }
td { @apply border-t border-[#e4e3da] px-3 py-4 align-top text-xs text-[#4e5c51]; }
td strong, td span, td code { @apply block max-w-full truncate; }
td small { @apply mt-1 block text-[#8a928b]; }
td code { @apply font-mono text-[#334037]; }
.status { @apply inline-block w-fit rounded-full px-2 py-1 font-bold; }
.status-active { @apply bg-[#deedda] text-[#37633f]; }
.status-inactive { @apply bg-[#ecebe6] text-[#666d67]; }
.configured { @apply font-semibold text-[#37633f]; }
.muted { @apply text-[#8a928b]; }
.webhook-button { @apply mt-2 min-h-11 w-full rounded-lg border border-[#c7cabf] bg-white px-2 text-xs font-bold text-[#334037] hover:bg-[#f4f1e8] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-50; }
.row-actions { @apply flex flex-col items-stretch gap-2; }
.row-actions button { @apply min-h-11 rounded-lg border border-[#c7cabf] bg-white px-2 text-xs font-bold text-[#334037] transition hover:bg-[#f4f1e8] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:cursor-wait disabled:opacity-50; }
.row-actions .confirming { @apply border-[#bf7348] bg-[#fff0e8] text-[#874122]; }
.row-actions .cancel-button { @apply border-transparent text-[#687169]; }

@media (max-width: 1100px) {
  colgroup { @apply hidden; }
  thead { @apply sr-only; }
  table, tbody, tr, td { @apply block w-full; }
  tbody { @apply grid gap-3 p-3; }
  tr { @apply rounded-xl border border-[#deded4] p-3; }
  td { @apply grid grid-cols-[7rem_minmax(0,1fr)] gap-3 border-0 px-0 py-2; }
  td::before { content: attr(data-label); @apply text-[0.68rem] font-bold uppercase tracking-wide text-[#7a837c]; }
  .row-actions { @apply flex-row flex-wrap; }
}
</style>
