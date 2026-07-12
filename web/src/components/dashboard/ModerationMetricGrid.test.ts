import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ModerationMetricGrid from './ModerationMetricGrid.vue'
import WebhookMetricSummary from './WebhookMetricSummary.vue'
import type { OperationsStats } from '@/types'

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

describe('operations metric presentation', () => {
  it('shows persisted moderation totals and accessible decision distribution', () => {
    const wrapper = mount(ModerationMetricGrid, { props: { stats } })

    expect(wrapper.get('[data-testid="total-moderated"]').text()).toBe('12')
    expect(wrapper.text()).toContain('20%')
    expect(wrapper.get('[role="progressbar"][aria-label="最终允许"]').attributes('aria-valuenow')).toBe('58.3')
    expect(wrapper.text()).toContain('7 · 58.3%')
    expect(wrapper.text()).toContain('2 · 16.7%')
    expect(wrapper.text()).toContain('3 · 25%')
  })

  it('uses a valid percentage range when no moderation records exist', () => {
    const wrapper = mount(ModerationMetricGrid, {
      props: {
        stats: Object.fromEntries(Object.keys(stats).map((key) => [key, 0])) as unknown as OperationsStats,
      },
    })

    const progress = wrapper.get('[role="progressbar"][aria-label="最终允许"]')
    expect(progress.attributes('aria-valuemin')).toBe('0')
    expect(progress.attributes('aria-valuemax')).toBe('100')
    expect(progress.attributes('aria-valuenow')).toBe('0')
  })

  it('states the latest-delivery status scope for Webhook metrics', () => {
    const wrapper = mount(WebhookMetricSummary, { props: { stats } })

    expect(wrapper.get('[data-testid="webhook-total"]').text()).toBe('9')
    expect(wrapper.text()).toContain('成功')
    expect(wrapper.text()).toContain('失败')
    expect(wrapper.text()).toContain('重试中')
    expect(wrapper.text()).toContain('不代表逐次投递尝试次数')
  })
})
