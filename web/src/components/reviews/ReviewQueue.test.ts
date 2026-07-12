import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ReviewQueue from './ReviewQueue.vue'
import type { ReviewCase } from '@/types'

const item: ReviewCase = {
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

describe('ReviewQueue', () => {
  it('exposes the selected case to assistive technology', () => {
    const wrapper = mount(ReviewQueue, {
      props: {
        items: [item],
        selectedId: item.id,
        loading: false,
        busy: false,
      },
    })

    expect(wrapper.get('.queue-item').attributes('aria-current')).toBe('true')
  })

  it('locks case selection while a review action is submitting', () => {
    const wrapper = mount(ReviewQueue, {
      props: {
        items: [item],
        loading: false,
        busy: true,
      },
    })

    expect(wrapper.get('.queue-item').attributes()).toHaveProperty('disabled')
  })
})
