import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import LoginForm from './LoginForm.vue'

describe('LoginForm', () => {
  it('submits trimmed administrator credentials', async () => {
    const wrapper = mount(LoginForm, {
      props: { busy: false, error: '' },
    })

    await wrapper.get('#email').setValue('  admin@example.com  ')
    await wrapper.get('#password').setValue('password123')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('submit')).toEqual([
      [{ email: 'admin@example.com', password: 'password123' }],
    ])
  })

  it('shows an accessible login error', () => {
    const wrapper = mount(LoginForm, {
      props: { busy: false, error: '账号或密码错误' },
    })

    expect(wrapper.get('[role="alert"]').text()).toBe('账号或密码错误')
  })
})
