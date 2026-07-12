import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, getOperationsStats } from '@/api'
import { useOperationsStats } from './useOperationsStats'
import type { OperationsStats } from '@/types'

vi.mock('@/api', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/api')>(),
  getOperationsStats: vi.fn(),
}))

const stats: OperationsStats = {
  total_moderated: 12,
  allowed: 7,
  blocked: 3,
  pending_review: 2,
  reviewed: 5,
  mistakes: 1,
  mistake_rate: 0.2,
  webhook_total: 9,
  webhook_succeeded: 6,
  webhook_failed: 2,
  webhook_retrying: 1,
}

describe('useOperationsStats', () => {
  beforeEach(() => vi.clearAllMocks())

  it('loads the current persisted operations snapshot', async () => {
    vi.mocked(getOperationsStats).mockResolvedValue(stats)
    const state = useOperationsStats({ token: 'jwt', onUnauthorized: vi.fn() })

    await state.load()

    expect(getOperationsStats).toHaveBeenCalledWith('jwt')
    expect(state.stats.value).toEqual(stats)
    expect(state.error.value).toBe('')
  })

  it('ends the session when the statistics request is unauthorized', async () => {
    const onUnauthorized = vi.fn()
    vi.mocked(getOperationsStats).mockRejectedValue(new ApiError('expired', 401))
    const state = useOperationsStats({ token: 'jwt', onUnauthorized })

    await state.load()

    expect(onUnauthorized).toHaveBeenCalledOnce()
    expect(state.error.value).toBe('')
  })
})
