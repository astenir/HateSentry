import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import WebhookDeliveryList from './WebhookDeliveryList.vue'
import type { WebhookDelivery } from '@/types'
const item: WebhookDelivery = { id: 7, delivery_id: 'd7', request_id: 'request-7', client_id: 4, event: 'moderation.finalized', status: 'failed', attempt_count: 1, last_attempt_at: '2026-07-12T08:00:00Z', http_status: 503, error_message: 'upstream unavailable', created_at: '2026-07-12T08:00:00Z', updated_at: '2026-07-12T08:00:00Z' }
describe('WebhookDeliveryList', () => {
  it('shows operational fields and requires retry confirmation', async () => {
    const wrapper = mount(WebhookDeliveryList, { props: { items: [item], loading: false, retryingIds: new Set<number>() } })
    expect(wrapper.text()).toContain('request-7'); expect(wrapper.text()).toContain('503'); expect(wrapper.text()).toContain('upstream unavailable')
    const button = wrapper.get('.actions button'); await button.trigger('click'); expect(wrapper.emitted('retry')).toBeUndefined(); await button.trigger('click')
    expect(wrapper.emitted('retry')).toEqual([[item]])
  })
})
