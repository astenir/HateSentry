import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ClientList from './ClientList.vue'
import type { ClientApplication, ModerationPolicy } from '@/types'

const policies: ModerationPolicy[] = [
  { version: 'default-v1', review_threshold: 0.4, block_threshold: 0.75, default: true },
  { version: 'strict-v1', review_threshold: 0.2, block_threshold: 0.5, default: false },
]

const active: ClientApplication = {
  id: 4, name: 'blog', status: 'active', api_key_prefix: 'hs_blog_',
  policy_version: '', webhook_url: '', created_at: '2026-07-12T08:00:00Z',
}
const inactive: ClientApplication = {
  ...active, id: 5, name: 'support', status: 'inactive', policy_version: 'strict-v1',
  webhook_url: 'https://example.com/hook',
}

describe('ClientList', () => {
  it('shows read-only policy and webhook state with correct status operations', async () => {
    const wrapper = mount(ClientList, {
      props: { items: [active, inactive], policies, loading: false, busyClientIds: new Set<number>(), credentialOpen: false },
    })

    expect(wrapper.text()).toContain('跟随系统默认')
    expect(wrapper.text()).toContain('strict-v1')
    expect(wrapper.text()).toContain('未配置')
    expect(wrapper.text()).toContain('已配置')

    const statusButtons = wrapper.findAll('.row-actions button').filter((button) => ['停用', '启用'].includes(button.text()))
    await statusButtons[0].trigger('click')
    await statusButtons[1].trigger('click')
    expect(wrapper.emitted('setActive')).toEqual([[active, false], [inactive, true]])
  })

  it('requires a second explicit click before rotating a key', async () => {
    const wrapper = mount(ClientList, {
      props: { items: [active], policies, loading: false, busyClientIds: new Set<number>(), credentialOpen: false },
    })
    const rotate = wrapper.get('.rotate-button')

    await rotate.trigger('click')
    expect(wrapper.emitted('rotate')).toBeUndefined()
    expect(rotate.text()).toContain('旧 Key 失效')
    await rotate.trigger('click')
    expect(wrapper.emitted('rotate')).toEqual([[active]])
  })

  it('disables rotation while another one-time credential is open', () => {
    const wrapper = mount(ClientList, {
      props: { items: [active], policies, loading: false, busyClientIds: new Set<number>(), credentialOpen: true },
    })

    expect(wrapper.get('.rotate-button').attributes()).toHaveProperty('disabled')
    expect(wrapper.get('.row-actions button').attributes()).not.toHaveProperty('disabled')
  })

  it('forwards an explicit policy assignment from the row editor', async () => {
    const wrapper = mount(ClientList, {
      props: { items: [active], policies, loading: false, busyClientIds: new Set<number>(), credentialOpen: false },
    })

    await wrapper.get('select').setValue('strict-v1')
    await wrapper.get('.policy-editor').trigger('submit')

    expect(wrapper.emitted('assignPolicy')).toEqual([[active, 'strict-v1']])
  })
})
