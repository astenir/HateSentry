import { readonly, ref, shallowRef } from 'vue'

import { ApiError, finalizeReview, getReview, listPendingReviews } from '@/api'
import type { ReviewActionInput, ReviewCase } from '@/types'

interface UseReviewsOptions {
  token: string
  onUnauthorized: () => void
}

function messageFor(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '操作失败，请稍后重试'
}

export function useReviews(options: UseReviewsOptions) {
  const items = ref<ReviewCase[]>([])
  const selected = shallowRef<ReviewCase | null>(null)
  const isLoading = shallowRef(false)
  const isLoadingDetail = shallowRef(false)
  const isSubmitting = shallowRef(false)
  const error = shallowRef('')
  const notice = shallowRef('')
  let detailRequestSequence = 0

  function handleError(cause: unknown): void {
    if (cause instanceof ApiError && cause.status === 401) {
      options.onUnauthorized()
      return
    }
    error.value = messageFor(cause)
  }

  async function loadQueue(): Promise<boolean> {
    isLoading.value = true
    error.value = ''
    try {
      items.value = await listPendingReviews(options.token)
      if (selected.value && !items.value.some((item) => item.id === selected.value?.id)) {
        selected.value = null
      }
      return true
    } catch (cause) {
      handleError(cause)
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function selectReview(id: number): Promise<void> {
    const requestSequence = ++detailRequestSequence
    isLoadingDetail.value = true
    error.value = ''
    notice.value = ''
    try {
      const detail = await getReview(options.token, id)
      if (requestSequence === detailRequestSequence) {
        selected.value = detail
      }
    } catch (cause) {
      if (requestSequence === detailRequestSequence) {
        handleError(cause)
      }
    } finally {
      if (requestSequence === detailRequestSequence) {
        isLoadingDetail.value = false
      }
    }
  }

  async function submitAction(input: ReviewActionInput): Promise<void> {
    if (!selected.value) return

    const submittedCaseID = selected.value.id
    isSubmitting.value = true
    error.value = ''
    notice.value = ''
    try {
      const completed = await finalizeReview(
        options.token,
        submittedCaseID,
        input,
      )
      items.value = items.value.filter((item) => item.id !== completed.id)
      if (selected.value?.id === submittedCaseID) {
        selected.value = completed
      }
      notice.value = '复核结果已保存，待处理队列已更新。'
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        detailRequestSequence++
        selected.value = null
        items.value = items.value.filter((item) => item.id !== submittedCaseID)
        const refreshed = await loadQueue()
        if (refreshed) {
          error.value = '该案件已被其他操作员处理，待处理队列已刷新。'
        }
      } else {
        handleError(cause)
      }
    } finally {
      isSubmitting.value = false
    }
  }

  return {
    items: readonly(items),
    selected: readonly(selected),
    isLoading: readonly(isLoading),
    isLoadingDetail: readonly(isLoadingDetail),
    isSubmitting: readonly(isSubmitting),
    error: readonly(error),
    notice: readonly(notice),
    loadQueue,
    selectReview,
    submitAction,
  }
}
