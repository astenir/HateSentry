<script setup lang="ts">
import { onMounted } from 'vue'

import ClientCreateForm from './ClientCreateForm.vue'
import ClientList from './ClientList.vue'
import OneTimeCredentialPanel from './OneTimeCredentialPanel.vue'
import PolicyCatalog from './PolicyCatalog.vue'
import { useClients } from '@/composables/useClients'
import type { ClientApplication } from '@/types'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ unauthorized: [] }>()

const clients = useClients({
  token: props.token,
  onUnauthorized: () => emit('unauthorized'),
})

onMounted(() => {
  void clients.load()
  void clients.loadPolicies()
})

function setActive(client: ClientApplication, active: boolean): void {
  void clients.setActive(client, active)
}
</script>

<template>
  <div class="clients-workspace">
    <header class="page-heading">
      <div>
        <span>接入管理</span>
        <h2>客户端管理</h2>
        <p>创建外部应用凭证、控制访问状态，并安全轮换 API Key。</p>
      </div>
      <button
        type="button"
        :disabled="clients.isLoading.value || clients.isCreating.value || clients.busyClientIds.value.size > 0"
        @click="clients.load"
      >刷新列表</button>
    </header>

    <div class="clients-content">
      <div v-if="clients.error.value" class="message error-message" role="alert">{{ clients.error.value }}</div>
      <div v-if="clients.notice.value" class="message notice-message" role="status">{{ clients.notice.value }}</div>
      <OneTimeCredentialPanel
        v-if="clients.credential.value"
        :credential="clients.credential.value"
        @close="clients.clearCredential"
      />
      <ClientCreateForm
        :busy="clients.isCreating.value"
        :locked="Boolean(clients.credential.value)"
        @submit="clients.create"
      />
      <PolicyCatalog
        :policies="clients.policies.value"
        :loading="clients.isLoadingPolicies.value"
      />
      <ClientList
        :items="clients.items.value"
        :loading="clients.isLoading.value"
        :busy-client-ids="clients.busyClientIds.value"
        :credential-open="Boolean(clients.credential.value)"
        :policies="clients.policies.value"
        @set-active="setActive"
        @rotate="clients.rotate"
        @assign-policy="clients.assignPolicy"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.clients-workspace { @apply min-h-0 flex-1 overflow-y-auto bg-[#f4f1e8]; }
.page-heading { @apply flex items-end justify-between gap-5 border-b border-[#d8d8cd] bg-[#fbfaf5] px-4 py-6 md:px-7; }
.page-heading span { @apply text-[0.68rem] font-bold uppercase tracking-[0.18em] text-[#758078]; }
.page-heading h2 { @apply mt-1 text-2xl font-bold tracking-tight text-[#213127]; }
.page-heading p { @apply mt-2 text-sm leading-6 text-[#687169]; }
.page-heading button { @apply min-h-11 shrink-0 rounded-xl border border-[#c7cabf] bg-white px-4 text-sm font-bold text-[#334037] hover:bg-[#f4f1e8] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15 disabled:opacity-50; }
.clients-content { @apply mx-auto grid w-full max-w-[90rem] gap-4 p-4 md:gap-5 md:p-7; }
.message { @apply rounded-xl border px-4 py-3 text-sm; }
.error-message { @apply border-[#e0b7b0] bg-[#f7e8e5] text-[#8b392f]; }
.notice-message { @apply border-[#bad0b9] bg-[#e8f2e5] text-[#37633f]; }
</style>
