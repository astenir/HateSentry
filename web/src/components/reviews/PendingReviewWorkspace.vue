<script setup lang="ts">
import { onMounted } from 'vue'

import ReviewDetail from './ReviewDetail.vue'
import ReviewQueue from './ReviewQueue.vue'
import { useReviews } from '@/composables/useReviews'
import type { ReviewActionInput } from '@/types'

const props = defineProps<{
  token: string
}>()

const emit = defineEmits<{
  unauthorized: []
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
  token: props.token,
  onUnauthorized: () => emit('unauthorized'),
})

onMounted(loadQueue)

function handleAction(input: ReviewActionInput): void {
  void submitAction(input)
}
</script>

<template>
  <div class="workspace-content">
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

.workspace-content {
  @apply flex min-h-0 flex-1 flex-col;
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
}
</style>
