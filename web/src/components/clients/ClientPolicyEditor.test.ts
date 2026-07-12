import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ClientPolicyEditor from './ClientPolicyEditor.vue'
import type { ClientApplication, ModerationPolicy } from '@/types'

const client: ClientApplication = {
  id: 4, name: 'blog', status: 'active', api_key_prefix: 'hs_blog_',
  policy_version: '', created_at: '2026-07-12T08:00:00Z',
}
const policies: ModerationPolicy[] = [
  { version: 'default-v1', review_threshold: 0.4, block_threshold: 0.75, default: true },
  { version: 'strict-v1', review_threshold: 0.2, block_threshold: 0.5, default: false },
]

describe('ClientPolicyEditor', () => {
  it('requires an explicit save after selecting a policy', async () => {
    const wrapper = mount(ClientPolicyEditor, { props: { client, policies, busy: false } })
    const select = wrapper.get('select')

    expect(wrapper.get('button').attributes()).toHaveProperty('disabled')
    await select.setValue('strict-v1')
    expect(wrapper.emitted('assign')).toBeUndefined()
    expect(wrapper.get('button').attributes('aria-label')).toBe('为 blog 应用策略')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('assign')).toEqual([[client, 'strict-v1']])
  })

  it('offers an explicit reset to following the system default', async () => {
    const strictClient = { ...client, policy_version: 'strict-v1' }
    const wrapper = mount(ClientPolicyEditor, {
      props: { client: strictClient, policies, busy: false },
    })

    expect(wrapper.text()).toContain('跟随系统默认（default-v1）')
    await wrapper.get('select').setValue('')
    expect(wrapper.get('button').text()).toBe('恢复默认')
    expect(wrapper.get('button').attributes('aria-label')).toBe('将 blog 恢复为跟随默认策略')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('assign')).toEqual([[strictClient, '']])
  })
})
