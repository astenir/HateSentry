import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ReviewHistoryFilters from './ReviewHistoryFilters.vue'

describe('ReviewHistoryFilters', () => {
  it('emits status changes and refresh requests through labelled controls', async () => {
    const wrapper = mount(ReviewHistoryFilters, {
      props: { modelValue: 'all', loading: false },
    })

    expect(wrapper.get('label[for="history-status"]').text()).toBe('人工状态')
    await wrapper.get('#history-status').setValue('rejected')
    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('change')).toEqual([['rejected']])
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })
})
