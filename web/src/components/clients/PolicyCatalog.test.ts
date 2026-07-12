import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import PolicyCatalog from './PolicyCatalog.vue'

describe('PolicyCatalog', () => {
  it('shows policy thresholds and marks the server default', () => {
    const wrapper = mount(PolicyCatalog, {
      props: {
        loading: false,
        policies: [
          { version: 'default-v1', review_threshold: 0.4, block_threshold: 0.75, default: true },
          { version: 'strict-v1', review_threshold: 0.2, block_threshold: 0.5, default: false },
        ],
      },
    })

    expect(wrapper.text()).toContain('default-v1')
    expect(wrapper.text()).toContain('系统默认')
    expect(wrapper.text()).toContain('≥ 40%')
    expect(wrapper.text()).toContain('≥ 75%')
    expect(wrapper.text()).toContain('strict-v1')
  })
})
