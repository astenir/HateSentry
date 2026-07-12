import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import OneTimeCredentialPanel from './OneTimeCredentialPanel.vue'

afterEach(() => vi.unstubAllGlobals())

const credential = { clientId: 4, clientName: 'blog', apiKey: 'hs_secret', kind: 'rotated' as const }

describe('OneTimeCredentialPanel', () => {
  it('warns that the key is one-time and emits close', async () => {
    const wrapper = mount(OneTimeCredentialPanel, { props: { credential } })
    expect(wrapper.text().replace(/\s+/g, ' ')).toContain('完整 API Key 仅在此处显示一次')
    expect(wrapper.text()).toContain('关闭后无法再次查看')
    await wrapper.get('[aria-label="关闭一次性 API Key"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('copies through the clipboard and reports failures visibly', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const wrapper = mount(OneTimeCredentialPanel, { props: { credential } })

    await wrapper.get('.copy-button').trigger('click')
    await Promise.resolve()
    expect(writeText).toHaveBeenCalledWith('hs_secret')
    expect(wrapper.text()).toContain('已复制到剪贴板')

    writeText.mockRejectedValueOnce(new Error('denied'))
    await wrapper.get('.copy-button').trigger('click')
    await Promise.resolve()
    expect(wrapper.text()).toContain('复制失败')
  })

  it('renders and copies a one-time Webhook signing secret', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const wrapper = mount(OneTimeCredentialPanel, {
      props: {
        credential: {
          clientId: 4, clientName: 'blog', webhookSecret: 'whsec_secret', kind: 'webhook',
        },
      },
    })

    expect(wrapper.text()).toContain('Webhook 签名 secret')
    expect(wrapper.get('[data-testid="one-time-webhook-secret"]').text()).toBe('whsec_secret')
    await wrapper.get('.copy-button').trigger('click')
    await Promise.resolve()

    expect(writeText).toHaveBeenCalledWith('whsec_secret')
    expect(wrapper.get('[aria-label="关闭一次性 Webhook secret"]').text()).toBe('关闭')
  })
})
