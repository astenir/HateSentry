import { readonly, ref, shallowRef } from 'vue'

import { ApiError, listWebhookDeliveries, retryWebhookDelivery } from '@/api'
import type { WebhookDelivery, WebhookDeliveryFilter } from '@/types'

interface Options { token: string, onUnauthorized: () => void }

export function useWebhookDeliveries(options: Options) {
  const items = ref<WebhookDelivery[]>([])
  const filter = ref<WebhookDeliveryFilter>({ status: 'all', clientId: '', requestId: '' })
  const isLoading = shallowRef(false)
  const retryingIds = ref<ReadonlySet<number>>(new Set())
  const error = shallowRef('')
  const notice = shallowRef('')
  let loadSequence = 0

  function handleError(cause: unknown): void {
    if (cause instanceof ApiError && cause.status === 401) {
      options.onUnauthorized()
      return
    }
    error.value = cause instanceof Error ? cause.message : '操作失败，请稍后重试'
  }

  async function load(nextFilter: WebhookDeliveryFilter = filter.value, force = false): Promise<boolean> {
    if (!force && retryingIds.value.size > 0) return false
    const sequence = ++loadSequence
    filter.value = { ...nextFilter }
    isLoading.value = true
    error.value = ''
    try {
      const result = await listWebhookDeliveries(options.token, filter.value)
      if (sequence === loadSequence) items.value = result
      return true
    } catch (cause) {
      if (sequence === loadSequence) handleError(cause)
      return false
    } finally {
      if (sequence === loadSequence) isLoading.value = false
    }
  }

  async function retry(delivery: WebhookDelivery): Promise<void> {
    if (isLoading.value || retryingIds.value.has(delivery.id)) return
    retryingIds.value = new Set(retryingIds.value).add(delivery.id)
    error.value = ''
    notice.value = ''
    try {
      const updated = await retryWebhookDelivery(options.token, delivery.id)
      items.value = items.value.flatMap((item) => item.id !== updated.id
        ? [item]
        : matchesFilter(updated, filter.value) ? [updated] : [])
      const refreshed = await load(filter.value, true)
      if (refreshed) notice.value = updated.status === 'succeeded'
        ? 'Webhook 重试成功。'
        : 'Webhook 重试已完成，请查看最新状态。'
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        const refreshed = await load(filter.value, true)
        if (refreshed) error.value = '该投递已被其他操作员或后台任务处理，列表已刷新。'
      } else {
        handleError(cause)
      }
    } finally {
      const next = new Set(retryingIds.value)
      next.delete(delivery.id)
      retryingIds.value = next
    }
  }

  function matchesFilter(delivery: WebhookDelivery, activeFilter: WebhookDeliveryFilter): boolean {
    if (activeFilter.status !== 'all' && delivery.status !== activeFilter.status) return false
    if (activeFilter.clientId.trim() && String(delivery.client_id) !== activeFilter.clientId.trim()) return false
    return !activeFilter.requestId.trim() || delivery.request_id === activeFilter.requestId.trim()
  }

  return {
    items: readonly(items), filter: readonly(filter), isLoading: readonly(isLoading),
    retryingIds: readonly(retryingIds), error: readonly(error), notice: readonly(notice),
    load, retry,
  }
}
