import { readonly, shallowRef } from 'vue'

import { ApiError, getOperationsStats } from '@/api'
import type { OperationsStats } from '@/types'

interface UseOperationsStatsOptions {
  token: string
  onUnauthorized: () => void
}

export function useOperationsStats(options: UseOperationsStatsOptions) {
  const stats = shallowRef<OperationsStats | null>(null)
  const isLoading = shallowRef(false)
  const error = shallowRef('')

  async function load(): Promise<void> {
    isLoading.value = true
    error.value = ''
    try {
      stats.value = await getOperationsStats(options.token)
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) {
        options.onUnauthorized()
        return
      }
      error.value = cause instanceof Error ? cause.message : '运营指标加载失败，请稍后重试'
    } finally {
      isLoading.value = false
    }
  }

  return {
    stats: readonly(stats),
    isLoading: readonly(isLoading),
    error: readonly(error),
    load,
  }
}
