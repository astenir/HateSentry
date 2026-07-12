<script setup lang="ts">
import { onMounted } from 'vue'

import ReviewDetail from './ReviewDetail.vue'
import ReviewQueue from './ReviewQueue.vue'
import { useReviews } from '@/composables/useReviews'
import type { ReviewActionInput, Session } from '@/types'

const props = defineProps<{
  session: Session
}>()

const emit = defineEmits<{
  logout: []
}>()

const {
  items,
  selected,
  isLoading,
  isLoadingDetail,
  isSubmitting,
  error,
  notice,
  loadQueue,
  selectReview,
  submitAction,
} = useReviews({
  token: props.session.token,
  onUnauthorized: () => emit('logout'),
})

onMounted(loadQueue)

function handleAction(input: ReviewActionInput): void {
  void submitAction(input)
}
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

    <div class="feedback-region" aria-live="polite" aria-atomic="true">
      <p v-if="error" class="feedback feedback-error">{{ error }}</p>
      <p v-else-if="notice" class="feedback feedback-success">{{ notice }}</p>
    </div>

    <div class="workspace-grid">
      <ReviewQueue
        :items="items"
        :selected-id="selected?.id"
        :loading="isLoading"
        :busy="isSubmitting"
        @select="selectReview"
        @refresh="loadQueue"
      />
      <ReviewDetail
        :item="selected"
        :loading="isLoadingDetail"
        :busy="isSubmitting"
        @action="handleAction"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.workspace-shell {
  @apply flex min-h-screen flex-col bg-[#fbfaf5] text-[#172119];
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

.feedback-region {
  @apply bg-[#ebe9df];
}

.feedback {
  @apply border-b px-5 py-3 text-sm md:px-7;
}

.feedback-error {
  @apply border-[#e0b7b0] bg-[#f7e8e5] text-[#8b392f];
}

.feedback-success {
  @apply border-[#bfd2ba] bg-[#eaf4e7] text-[#365e3e];
}

.workspace-grid {
  @apply grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[22rem_minmax(0,1fr)];
  height: calc(100vh - 4.5rem);
}

@media (max-width: 1023px) {
  .workspace-grid {
    height: auto;
  }
}
</style>
