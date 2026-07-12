<script setup lang="ts">
import { shallowRef } from 'vue'

import ReviewHistoryWorkspace from '@/components/history/ReviewHistoryWorkspace.vue'
import ClientManagementWorkspace from '@/components/clients/ClientManagementWorkspace.vue'
import WebhookDeliveryWorkspace from '@/components/webhooks/WebhookDeliveryWorkspace.vue'
import OperationsDashboardWorkspace from '@/components/dashboard/OperationsDashboardWorkspace.vue'
import PendingReviewWorkspace from './PendingReviewWorkspace.vue'
import type { Session } from '@/types'

const props = defineProps<{
  session: Session
}>()

const emit = defineEmits<{
  logout: []
}>()

const activeView = shallowRef<'pending' | 'history' | 'clients' | 'deliveries' | 'dashboard'>('pending')
</script>

<template>
  <div class="workspace-shell">
    <header class="app-header">
      <div class="brand-lockup">
        <span class="brand-dot" aria-hidden="true"></span>
        <div>
          <strong>HateSentry</strong>
          <span>人工复核控制台</span>
        </div>
      </div>

      <div class="operator-menu">
        <div class="operator-copy">
          <strong>{{ session.user.username }}</strong>
          <span>{{ session.user.email }}</span>
        </div>
        <button class="logout-button" type="button" @click="emit('logout')">退出</button>
      </div>
    </header>

    <nav class="console-nav" aria-label="复核控制台视图">
      <button
        type="button"
        :aria-pressed="activeView === 'pending'"
        :class="{ 'nav-active': activeView === 'pending' }"
        @click="activeView = 'pending'"
      >
        待处理队列
      </button>
      <button
        type="button"
        :aria-pressed="activeView === 'history'"
        :class="{ 'nav-active': activeView === 'history' }"
        @click="activeView = 'history'"
      >
        审核历史
      </button>
      <button
        type="button"
        :aria-pressed="activeView === 'clients'"
        :class="{ 'nav-active': activeView === 'clients' }"
        @click="activeView = 'clients'"
      >
        客户端管理
      </button>
      <button type="button" :aria-pressed="activeView === 'deliveries'" :class="{ 'nav-active': activeView === 'deliveries' }" @click="activeView = 'deliveries'">Webhook 投递</button>
      <button
        type="button"
        :aria-pressed="activeView === 'dashboard'"
        :class="{ 'nav-active': activeView === 'dashboard' }"
        @click="activeView = 'dashboard'"
      >
        运营概览
      </button>
    </nav>

    <PendingReviewWorkspace
      v-if="activeView === 'pending'"
      :token="session.token"
      @unauthorized="emit('logout')"
    />
    <ReviewHistoryWorkspace
      v-else-if="activeView === 'history'"
      :token="session.token"
      @unauthorized="emit('logout')"
    />
    <ClientManagementWorkspace
      v-else-if="activeView === 'clients'"
      :token="session.token"
      @unauthorized="emit('logout')"
    />
    <WebhookDeliveryWorkspace
      v-else-if="activeView === 'deliveries'"
      :token="session.token"
      @unauthorized="emit('logout')"
    />
    <OperationsDashboardWorkspace v-else :token="session.token" @unauthorized="emit('logout')" />
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.workspace-shell {
  @apply flex min-h-screen flex-col bg-[#fbfaf5] text-[#172119] lg:h-screen;
}

.app-header {
  @apply flex min-h-18 items-center justify-between gap-4 border-b border-[#d8d8cd] bg-[#172019] px-4 py-3 text-[#f7f1df] md:px-7;
}

.brand-lockup {
  @apply flex items-center gap-3;
}

.brand-dot {
  @apply h-3 w-3 rounded-full bg-[#f1b65b] shadow-[0_0_0_6px_rgba(241,182,91,0.12)];
}

.brand-lockup div {
  @apply flex flex-col;
}

.brand-lockup strong {
  @apply text-sm font-bold tracking-wide;
}

.brand-lockup span:last-child {
  @apply text-[0.68rem] text-[#aeb9af];
}

.operator-menu {
  @apply flex items-center gap-3;
}

.operator-copy {
  @apply hidden flex-col text-right sm:flex;
}

.operator-copy strong {
  @apply text-xs;
}

.operator-copy span {
  @apply text-[0.68rem] text-[#aeb9af];
}

.logout-button {
  @apply min-h-11 rounded-lg border border-white/15 px-3 py-2 text-xs font-semibold transition hover:bg-white/10 focus:outline-none focus:ring-4 focus:ring-white/10;
}

.console-nav {
  @apply flex gap-1 overflow-x-auto border-b border-[#d8d8cd] bg-[#ebe9df] px-4 py-2 md:px-7;
}

.console-nav button {
  @apply min-h-11 shrink-0 rounded-lg px-4 text-sm font-semibold text-[#5f6961] transition hover:bg-white/60 focus:outline-none focus:ring-4 focus:ring-[#456b4d]/15;
}

.console-nav .nav-active {
  @apply bg-white text-[#23402a] shadow-sm;
}
</style>
