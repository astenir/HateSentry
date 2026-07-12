<script setup lang="ts">
import { onMounted } from 'vue'

import ReviewHistoryFilters from './ReviewHistoryFilters.vue'
import ReviewHistoryList from './ReviewHistoryList.vue'
import ReviewDetail from '@/components/reviews/ReviewDetail.vue'
import { useReviewHistory } from '@/composables/useReviewHistory'
import type { ReviewHistoryFilter } from '@/types'

const props = defineProps<{
  token: string
}>()

const emit = defineEmits<{
  unauthorized: []
}>()

const {
  items,
  selected,
  filter,
  isLoading,
  isLoadingMore,
  isLoadingDetail,
  hasMore,
  error,
  loadHistory,
  loadMore,
  setFilter,
  selectReview,
} = useReviewHistory({
  token: props.token,
  onUnauthorized: () => emit('unauthorized'),
})

onMounted(loadHistory)

function handleFilter(filterValue: ReviewHistoryFilter): void {
  void setFilter(filterValue)
}
</script>

<template>
  <div class="history-workspace">
    <div v-if="error" class="history-error" role="alert">{{ error }}</div>
    <ReviewHistoryFilters
      :model-value="filter"
      :loading="isLoading"
      @change="handleFilter"
      @refresh="loadHistory"
    />
    <div class="history-grid">
      <ReviewHistoryList
        :items="items"
        :selected-id="selected?.id"
        :loading="isLoading"
        :loading-more="isLoadingMore"
        :has-more="hasMore"
        @select="selectReview"
        @load-more="loadMore"
      />
      <ReviewDetail
        context="history"
        :item="selected"
        :loading="isLoadingDetail"
        :busy="false"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../styles.css";

.history-workspace {
  @apply flex min-h-0 flex-1 flex-col;
}

.history-error {
  @apply border-b border-[#e0b7b0] bg-[#f7e8e5] px-5 py-3 text-sm text-[#8b392f] md:px-7;
}

.history-grid {
  @apply grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[24rem_minmax(0,1fr)];
}
</style>
