import { computed, readonly, ref, shallowRef } from 'vue'

import { ApiError, getReview, listReviewHistory } from '@/api'
import type { ReviewCase, ReviewHistoryFilter } from '@/types'

interface UseReviewHistoryOptions {
  token: string
  onUnauthorized: () => void
}

function messageFor(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '操作失败，请稍后重试'
}

export function useReviewHistory(options: UseReviewHistoryOptions) {
  const items = ref<ReviewCase[]>([])
  const selected = shallowRef<ReviewCase | null>(null)
  const filter = shallowRef<ReviewHistoryFilter>('all')
  const isLoading = shallowRef(false)
  const isLoadingMore = shallowRef(false)
  const isLoadingDetail = shallowRef(false)
  const error = shallowRef('')
  const nextCursor = shallowRef('')
  const hasMore = computed(() => nextCursor.value !== '')
  let listRequestSequence = 0
  let detailRequestSequence = 0

  function handleError(cause: unknown): void {
    if (cause instanceof ApiError && cause.status === 401) {
      options.onUnauthorized()
      return
    }
    error.value = messageFor(cause)
  }

  async function loadHistory(): Promise<void> {
    const requestSequence = ++listRequestSequence
    const requestedFilter = filter.value
    detailRequestSequence++
    isLoading.value = true
    isLoadingMore.value = false
    isLoadingDetail.value = false
    error.value = ''
    try {
      const loaded = await listReviewHistory(options.token, requestedFilter)
      if (requestSequence !== listRequestSequence) return

      items.value = loaded.items
      nextCursor.value = loaded.next_cursor || ''
      if (selected.value && !loaded.items.some((item) => item.id === selected.value?.id)) {
        selected.value = null
      }
    } catch (cause) {
      if (requestSequence === listRequestSequence) handleError(cause)
    } finally {
      if (requestSequence === listRequestSequence) isLoading.value = false
    }
  }

  async function loadMore(): Promise<void> {
    if (!nextCursor.value || isLoading.value || isLoadingMore.value) return

    const requestSequence = listRequestSequence
    const requestedFilter = filter.value
    const requestedCursor = nextCursor.value
    isLoadingMore.value = true
    error.value = ''
    try {
      const loaded = await listReviewHistory(
        options.token,
        requestedFilter,
        requestedCursor,
      )
      if (requestSequence !== listRequestSequence || requestedFilter !== filter.value) return

      const existingIDs = new Set(items.value.map((item) => item.id))
      items.value = [
        ...items.value,
        ...loaded.items.filter((item) => !existingIDs.has(item.id)),
      ]
      nextCursor.value = loaded.next_cursor || ''
    } catch (cause) {
      if (requestSequence === listRequestSequence) handleError(cause)
    } finally {
      if (requestSequence === listRequestSequence) isLoadingMore.value = false
    }
  }

  async function setFilter(nextFilter: ReviewHistoryFilter): Promise<void> {
    if (filter.value !== nextFilter) {
      filter.value = nextFilter
      detailRequestSequence++
      items.value = []
      nextCursor.value = ''
      selected.value = null
    }
    await loadHistory()
  }

  async function selectReview(id: number): Promise<void> {
    const requestSequence = ++detailRequestSequence
    isLoadingDetail.value = true
    error.value = ''
    try {
      const detail = await getReview(options.token, id)
      if (requestSequence === detailRequestSequence) selected.value = detail
    } catch (cause) {
      if (requestSequence === detailRequestSequence) handleError(cause)
    } finally {
      if (requestSequence === detailRequestSequence) isLoadingDetail.value = false
    }
  }

  return {
    items: readonly(items),
    selected: readonly(selected),
    filter: readonly(filter),
    isLoading: readonly(isLoading),
    isLoadingMore: readonly(isLoadingMore),
    isLoadingDetail: readonly(isLoadingDetail),
    hasMore,
    error: readonly(error),
    loadHistory,
    loadMore,
    setFilter,
    selectReview,
  }
}
