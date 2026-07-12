import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ClientWebhookEditor from './ClientWebhookEditor.vue'
import type { ClientApplication } from '@/types'

const client: ClientApplication = {
  id: 4, name: 'blog', status: 'active', api_key_prefix: 'hs_blog_',
  created_at: '2026-07-12T08:00:00Z',
}

describe('ClientWebhookEditor', () => {
  it('trims and explicitly saves a public HTTPS URL', async () => {
    const wrapper = mount(ClientWebhookEditor, {
      props: { client, busy: false, credentialOpen: false },
    })

    await wrapper.get('input').setValue('  https://example.com/moderation/webhook  ')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([[client, 'https://example.com/moderation/webhook']])
  })

  it('shows a visible validation error for non-HTTPS URLs', async () => {
    const wrapper = mount(ClientWebhookEditor, {
      props: { client, busy: false, credentialOpen: false },
    })

    await wrapper.get('input').setValue('http://example.com/hook')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('必须使用 HTTPS')
  })

  it('requires a second explicit click before clearing the Webhook', async () => {
    const configured = { ...client, webhook_url: 'https://example.com/hook' }
    const wrapper = mount(ClientWebhookEditor, {
      props: { client: configured, busy: false, credentialOpen: false },
    })

    await wrapper.get('.clear-button').trigger('click')
    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.get('.clear-button').text()).toContain('确认清除')
    await wrapper.get('.clear-button').trigger('click')

    expect(wrapper.emitted('save')).toEqual([[configured, '']])
  })

  it('allows the same URL to be submitted again to rotate a lost secret', async () => {
    const configured = { ...client, webhook_url: 'https://example.com/hook' }
    const wrapper = mount(ClientWebhookEditor, {
      props: { client: configured, busy: false, credentialOpen: false },
    })

    expect(wrapper.get('.save-button').attributes()).not.toHaveProperty('disabled')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([[configured, configured.webhook_url]])
  })

  it('locks mutation controls while another one-time credential is open', () => {
    const wrapper = mount(ClientWebhookEditor, {
      props: { client, busy: false, credentialOpen: true },
    })

    expect(wrapper.get('input').attributes()).toHaveProperty('disabled')
    expect(wrapper.get('.save-button').attributes()).toHaveProperty('disabled')
  })
})
