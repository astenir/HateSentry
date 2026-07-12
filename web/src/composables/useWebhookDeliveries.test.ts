import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, listWebhookDeliveries, retryWebhookDelivery } from '@/api'
import { useWebhookDeliveries } from './useWebhookDeliveries'
import type { WebhookDelivery } from '@/types'
vi.mock('@/api', async (original) => ({ ...await original<typeof import('@/api')>(), listWebhookDeliveries: vi.fn(), retryWebhookDelivery: vi.fn() }))
const item: WebhookDelivery = { id: 7, delivery_id: 'd7', request_id: 'r7', client_id: 4, event: 'moderation.finalized', status: 'failed', attempt_count: 1, last_attempt_at: '2026-07-12T08:00:00Z', created_at: '2026-07-12T08:00:00Z', updated_at: '2026-07-12T08:00:00Z' }
beforeEach(() => vi.clearAllMocks())
describe('useWebhookDeliveries', () => {
  it('loads filtered records and replaces a retried row', async () => {
    vi.mocked(listWebhookDeliveries).mockResolvedValueOnce([item]).mockResolvedValueOnce([])
    vi.mocked(retryWebhookDelivery).mockResolvedValue({ ...item, status: 'succeeded', attempt_count: 2 })
    const state = useWebhookDeliveries({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.load({ status: 'failed', clientId: '4', requestId: '' })
    await state.retry(item)
    expect(state.items.value).toHaveLength(0)
    expect(state.notice.value).toContain('重试成功')
  })
  it('refreshes after a concurrent retry conflict', async () => {
    vi.mocked(listWebhookDeliveries).mockResolvedValue([item])
    vi.mocked(retryWebhookDelivery).mockRejectedValue(new ApiError('conflict', 409))
    const state = useWebhookDeliveries({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.load()
    await state.retry(item)
    expect(listWebhookDeliveries).toHaveBeenCalledTimes(2)
    expect(state.error.value).toContain('列表已刷新')
  })
  it('ends the session on unauthorized', async () => {
    vi.mocked(listWebhookDeliveries).mockRejectedValue(new ApiError('expired', 401))
    const onUnauthorized = vi.fn(); const state = useWebhookDeliveries({ token: 'jwt', onUnauthorized })
    await state.load(); expect(onUnauthorized).toHaveBeenCalledOnce()
  })
})
