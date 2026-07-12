import { describe, expect, it, vi } from 'vitest'

import { ApiError, getReview, listReviewHistory } from '@/api'
import type { ReviewCase, ReviewHistoryPage } from '@/types'
import { useReviewHistory } from './useReviewHistory'

vi.mock('@/api', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api')>()
  return {
    ...original,
    getReview: vi.fn(),
    listReviewHistory: vi.fn(),
  }
})

const mockedList = vi.mocked(listReviewHistory)
const mockedGet = vi.mocked(getReview)

const approvedCase: ReviewCase = {
  id: 8,
  request_id: 'request-approved',
  user_id: 7,
  content: '已经人工通过的内容',
  source: 'comment',
  status: 'approved',
  policy_decision: 'review',
  final_decision: 'allow',
  risk_score: 0.6,
  labels: ['harassment'],
  reason: '风险位于人工复核区间',
  policy_version: 'default-v1',
  reviewer_id: 1,
  reviewed_at: '2026-07-12T08:00:00Z',
  created_at: '2026-07-12T06:00:00Z',
}

describe('useReviewHistory', () => {
  it('loads completed cases, selects a detail, and clears it when the filter changes', async () => {
    mockedList
      .mockResolvedValueOnce({ items: [approvedCase] })
      .mockResolvedValueOnce({ items: [] })
    mockedGet.mockResolvedValue(approvedCase)
    const history = useReviewHistory({ token: 'jwt', onUnauthorized: vi.fn() })

    await history.loadHistory()
    await history.selectReview(approvedCase.id)
    await history.setFilter('rejected')

    expect(mockedList).toHaveBeenNthCalledWith(1, 'jwt', 'all')
    expect(mockedList).toHaveBeenNthCalledWith(2, 'jwt', 'rejected')
    expect(history.filter.value).toBe('rejected')
    expect(history.items.value).toEqual([])
    expect(history.selected.value).toBeNull()
  })

  it('ignores a stale list response after a newer filter resolves', async () => {
    let resolveFirst: (value: ReviewHistoryPage) => void = () => undefined
    mockedList
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce({ items: [] })
    const history = useReviewHistory({ token: 'jwt', onUnauthorized: vi.fn() })

    const first = history.loadHistory()
    await history.setFilter('mistake')
    resolveFirst({ items: [approvedCase] })
    await first

    expect(history.filter.value).toBe('mistake')
    expect(history.items.value).toEqual([])
    expect(history.isLoading.value).toBe(false)
  })

  it('does not resurrect a stale detail after a history refresh', async () => {
    let resolveDetail: (value: ReviewCase) => void = () => undefined
    mockedList
      .mockResolvedValueOnce({ items: [approvedCase] })
      .mockResolvedValueOnce({ items: [] })
    mockedGet.mockReturnValueOnce(new Promise((resolve) => { resolveDetail = resolve }))
    const history = useReviewHistory({ token: 'jwt', onUnauthorized: vi.fn() })

    await history.loadHistory()
    const detail = history.selectReview(approvedCase.id)
    await history.loadHistory()
    resolveDetail(approvedCase)
    await detail

    expect(history.items.value).toEqual([])
    expect(history.selected.value).toBeNull()
    expect(history.isLoadingDetail.value).toBe(false)
  })

  it('appends the next page once and clears the cursor on the final page', async () => {
    const secondCase = { ...approvedCase, id: 7, request_id: 'request-second' }
    mockedList
      .mockResolvedValueOnce({ items: [approvedCase], next_cursor: 'cursor-1' })
      .mockResolvedValueOnce({ items: [approvedCase, secondCase] })
    const history = useReviewHistory({ token: 'jwt', onUnauthorized: vi.fn() })

    await history.loadHistory()
    await history.loadMore()

    expect(mockedList).toHaveBeenNthCalledWith(2, 'jwt', 'all', 'cursor-1')
    expect(history.items.value.map((item) => item.id)).toEqual([8, 7])
    expect(history.hasMore.value).toBe(false)
    expect(history.isLoadingMore.value).toBe(false)
  })

  it('ends the session when loading history returns unauthorized', async () => {
    mockedList.mockRejectedValue(new ApiError('Token expired', 401, 'UNAUTHORIZED'))
    const onUnauthorized = vi.fn()
    const history = useReviewHistory({ token: 'jwt', onUnauthorized })

    await history.loadHistory()

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('does not keep another filter items or cursor when the new filter fails', async () => {
    mockedList
      .mockResolvedValueOnce({ items: [approvedCase], next_cursor: 'approved-cursor' })
      .mockRejectedValueOnce(new Error('筛选查询失败'))
    const history = useReviewHistory({ token: 'jwt', onUnauthorized: vi.fn() })

    await history.loadHistory()
    await history.setFilter('rejected')

    expect(history.filter.value).toBe('rejected')
    expect(history.items.value).toEqual([])
    expect(history.hasMore.value).toBe(false)
    expect(history.error.value).toBe('筛选查询失败')
  })
})
