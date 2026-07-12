import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, getReview, listPendingReviews, listReviewHistory } from '@/api'
import type { ReviewCase, Session } from '@/types'
import App from './App.vue'

vi.mock('@/api', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api')>()
  return {
    ...original,
    finalizeReview: vi.fn(),
    getReview: vi.fn(),
    listPendingReviews: vi.fn(),
    listReviewHistory: vi.fn(),
    login: vi.fn(),
  }
})

const mockedGet = vi.mocked(getReview)
const mockedList = vi.mocked(listPendingReviews)
const mockedHistory = vi.mocked(listReviewHistory)

const session: Session = {
  token: 'jwt-token',
  user: { id: 1, username: 'admin', email: 'admin@example.com', role: 'admin' },
}

const pendingCase: ReviewCase = {
  id: 3,
  request_id: 'request-123',
  user_id: 7,
  content: '只应对有权限的管理员显示',
  source: 'comment',
  status: 'pending',
  policy_decision: 'review',
  risk_score: 0.6,
  labels: ['harassment'],
  reason: '风险位于人工复核区间',
  policy_version: 'default-v1',
  created_at: '2026-07-12T06:00:00Z',
}

const approvedCase: ReviewCase = {
  ...pendingCase,
  id: 8,
  content: '已经人工通过的内容',
  status: 'approved',
  final_decision: 'allow',
  reviewer_id: 1,
  reviewed_at: '2026-07-12T08:00:00Z',
}

describe('App authentication boundary', () => {
  beforeEach(() => {
    sessionStorage.clear()
    sessionStorage.setItem('hatesentry-operator-session', JSON.stringify(session))
    mockedList.mockResolvedValue([pendingCase])
    mockedHistory.mockResolvedValue([approvedCase])
  })

  it('clears the session and returns to login after a detail 401', async () => {
    mockedGet.mockRejectedValue(new ApiError('Token expired', 401, 'UNAUTHORIZED'))
    const wrapper = mount(App)
    await flushPromises()

    await wrapper.get('.queue-item').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('进入复核队列')
    expect(sessionStorage.getItem('hatesentry-operator-session')).toBeNull()
  })

  it('keeps the session but does not reveal detail after a 403', async () => {
    mockedGet.mockRejectedValue(new ApiError('Forbidden', 403, 'FORBIDDEN'))
    const wrapper = mount(App)
    await flushPromises()

    await wrapper.get('.queue-item').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Forbidden')
    expect(wrapper.find('.detail-placeholder').exists()).toBe(true)
    expect(sessionStorage.getItem('hatesentry-operator-session')).not.toBeNull()
  })

  it('switches from the pending queue to completed review history', async () => {
    const wrapper = mount(App)
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()

    expect(mockedHistory).toHaveBeenCalledWith('jwt-token', 'all')
    expect(wrapper.text()).toContain('审核历史')
    expect(wrapper.text()).toContain('已经人工通过的内容')
    expect(wrapper.text()).toContain('策略 review → 人工 allow')
  })

  it('clears the session after a history detail 401', async () => {
    mockedGet.mockRejectedValue(new ApiError('Token expired', 401, 'UNAUTHORIZED'))
    const wrapper = mount(App)
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()
    await wrapper.get('.history-item').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('进入复核队列')
    expect(sessionStorage.getItem('hatesentry-operator-session')).toBeNull()
  })
})
