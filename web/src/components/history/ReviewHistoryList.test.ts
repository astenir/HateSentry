import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ReviewHistoryList from './ReviewHistoryList.vue'
import type { ReviewCase } from '@/types'

const item: ReviewCase = {
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

describe('ReviewHistoryList', () => {
  it('shows decision trace and emits the selected case', async () => {
    const wrapper = mount(ReviewHistoryList, {
      props: { items: [item], selectedId: item.id, loading: false },
    })

    expect(wrapper.text()).toContain('策略 review → 人工 allow')
    expect(wrapper.text()).toContain('操作员 #1')
    expect(wrapper.get('.history-item').attributes('aria-current')).toBe('true')

    await wrapper.get('.history-item').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[item.id]])
  })

  it('distinguishes rejected and mistake outcomes', () => {
    const wrapper = mount(ReviewHistoryList, {
      props: {
        items: [
          { ...item, id: 9, status: 'rejected', final_decision: 'block' },
          { ...item, id: 10, status: 'mistake', final_decision: 'allow' },
        ],
        loading: false,
      },
    })

    expect(wrapper.text()).toContain('已拒绝')
    expect(wrapper.text()).toContain('人工 block')
    expect(wrapper.text()).toContain('误判')
    expect(wrapper.text()).toContain('人工 allow')
  })
})
