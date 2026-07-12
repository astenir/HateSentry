import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ClientCreateForm from './ClientCreateForm.vue'

describe('ClientCreateForm', () => {
  it('trims the name before submitting', async () => {
    const wrapper = mount(ClientCreateForm, { props: { busy: false } })
    await wrapper.get('input').setValue('  blog-comments  ')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('submit')).toEqual([['blog-comments']])
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe('')
  })
})
