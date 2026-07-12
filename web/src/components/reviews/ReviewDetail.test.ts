import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ReviewDetail from './ReviewDetail.vue'
import type { ReviewCase } from '@/types'

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

describe('ReviewDetail', () => {
  it('emits an approve action with trimmed notes', async () => {
    const wrapper = mount(ReviewDetail, {
      props: { item: pendingCase, loading: false, busy: false },
    })

    await wrapper.get('#review-notes').setValue('  内容可发布  ')
    await wrapper.get('.action-allow').trigger('click')

    expect(wrapper.emitted('action')).toEqual([
      [{ action: 'approve', notes: '内容可发布', finalDecision: undefined }],
    ])
  })

  it('requires a final decision before marking a mistake', async () => {
    const wrapper = mount(ReviewDetail, {
      props: { item: pendingCase, loading: false, busy: false },
    })

    await wrapper.get('.action-mistake').trigger('click')
    expect(wrapper.get('[role="alert"]').text()).toContain('请选择人工最终决定')
    expect(wrapper.emitted('action')).toBeUndefined()

    await wrapper.get('input[value="allow"]').setValue(true)
    await wrapper.get('.action-mistake').trigger('click')

    expect(wrapper.emitted('action')).toEqual([
      [{ action: 'mark-mistake', notes: '', finalDecision: 'allow' }],
    ])
  })

  it('shows the operator and review time for a completed case', () => {
    const wrapper = mount(ReviewDetail, {
      props: {
        context: 'history',
        item: {
          ...pendingCase,
          status: 'approved',
          final_decision: 'allow',
          reviewer_id: 9,
          review_notes: '人工确认语境无害',
          reviewed_at: '2026-07-12T08:00:00Z',
        },
        loading: false,
        busy: false,
      },
    })

    expect(wrapper.text()).toContain('人工最终决定：allow')
    expect(wrapper.text()).toContain('复核人：操作员 #9')
    expect(wrapper.text()).toContain('AI 建议依据')
    expect(wrapper.text()).toContain('风险位于人工复核区间')
    expect(wrapper.text()).toContain('60')
    expect(wrapper.text()).toContain('harassment')
    expect(wrapper.text()).toContain('策略决定')
    expect(wrapper.text()).toContain('default-v1')
    expect(wrapper.text()).toContain('备注：人工确认语境无害')
    expect(wrapper.get('time').attributes('datetime')).toBe('2026-07-12T08:00:00Z')
  })
})
