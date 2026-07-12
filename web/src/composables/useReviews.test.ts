import { describe, expect, it, vi } from 'vitest'

import {
  ApiError,
  finalizeReview,
  getReview,
  listPendingReviews,
} from '@/api'
import type { ReviewCase } from '@/types'
import { useReviews } from './useReviews'

vi.mock('@/api', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api')>()
  return {
    ...original,
    finalizeReview: vi.fn(),
    getReview: vi.fn(),
    listPendingReviews: vi.fn(),
  }
})

const mockedList = vi.mocked(listPendingReviews)
const mockedGet = vi.mocked(getReview)
const mockedFinalize = vi.mocked(finalizeReview)

const pendingCase: ReviewCase = {
  id: 3,
  request_id: 'request-123',
  user_id: 7,
  content: '需要人工判断的内容',
  source: 'comment',
  status: 'pending',
  policy_decision: 'review',
  risk_score: 0.6,
  labels: ['harassment'],
  reason: '风险位于人工复核区间',
  policy_version: 'default-v1',
  created_at: '2026-07-12T06:00:00Z',
}

describe('useReviews', () => {
  it('loads, selects, and removes a finalized case from the pending queue', async () => {
    mockedList.mockResolvedValue([pendingCase])
    mockedGet.mockResolvedValue(pendingCase)
    mockedFinalize.mockResolvedValue({
      ...pendingCase,
      status: 'approved',
      final_decision: 'allow',
    })
    const reviews = useReviews({ token: 'jwt', onUnauthorized: vi.fn() })

    await reviews.loadQueue()
    await reviews.selectReview(3)
    await reviews.submitAction({ action: 'approve', notes: '内容可发布' })

    expect(reviews.items.value).toHaveLength(0)
    expect(reviews.selected.value?.status).toBe('approved')
    expect(reviews.notice.value).toContain('队列已更新')
  })

  it('turns a concurrent finalization conflict into operator guidance', async () => {
    mockedList.mockResolvedValue([])
    mockedGet.mockResolvedValue(pendingCase)
    mockedFinalize.mockRejectedValue(
      new ApiError('Review case is already finalized', 409, 'CONFLICT'),
    )
    const reviews = useReviews({ token: 'jwt', onUnauthorized: vi.fn() })

    await reviews.selectReview(3)
    await reviews.submitAction({ action: 'reject', notes: '' })

    expect(reviews.error.value).toContain('已被其他操作员处理')
    expect(reviews.selected.value).toBeNull()
    expect(reviews.items.value).toHaveLength(0)
  })

  it('keeps the refresh error and removes the stale case when 409 recovery fails', async () => {
    mockedList
      .mockResolvedValueOnce([pendingCase])
      .mockRejectedValueOnce(new Error('队列刷新失败'))
    mockedGet.mockResolvedValue(pendingCase)
    mockedFinalize.mockRejectedValue(
      new ApiError('Review case is already finalized', 409, 'CONFLICT'),
    )
    const reviews = useReviews({ token: 'jwt', onUnauthorized: vi.fn() })

    await reviews.loadQueue()
    await reviews.selectReview(3)
    await reviews.submitAction({ action: 'approve', notes: '' })

    expect(reviews.items.value).toHaveLength(0)
    expect(reviews.selected.value).toBeNull()
    expect(reviews.error.value).toBe('队列刷新失败')
  })

  it('ignores a stale detail response after a newer selection resolves', async () => {
    let resolveFirst: (value: ReviewCase) => void = () => undefined
    mockedGet
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce({ ...pendingCase, id: 4, content: '较新的案件' })
    const reviews = useReviews({ token: 'jwt', onUnauthorized: vi.fn() })

    const first = reviews.selectReview(3)
    await reviews.selectReview(4)
    resolveFirst(pendingCase)
    await first

    expect(reviews.selected.value?.id).toBe(4)
    expect(reviews.isLoadingDetail.value).toBe(false)
  })

  it('does not replace a newer selection with an older submit response', async () => {
    let resolveFinalize: (value: ReviewCase) => void = () => undefined
    mockedGet
      .mockResolvedValueOnce(pendingCase)
      .mockResolvedValueOnce({ ...pendingCase, id: 4, content: '较新的案件' })
    mockedFinalize.mockReturnValueOnce(
      new Promise((resolve) => { resolveFinalize = resolve }),
    )
    const reviews = useReviews({ token: 'jwt', onUnauthorized: vi.fn() })

    await reviews.selectReview(3)
    const submitting = reviews.submitAction({ action: 'approve', notes: '' })
    await reviews.selectReview(4)
    resolveFinalize({ ...pendingCase, status: 'approved', final_decision: 'allow' })
    await submitting

    expect(reviews.selected.value?.id).toBe(4)
  })

  it('ends the session when the API returns unauthorized', async () => {
    mockedList.mockRejectedValue(new ApiError('Token expired', 401, 'UNAUTHORIZED'))
    const onUnauthorized = vi.fn()
    const reviews = useReviews({ token: 'jwt', onUnauthorized })

    await reviews.loadQueue()

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })
})
