import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { login as loginRequest } from '@/api'
import { useSession } from './useSession'

vi.mock('@/api', () => ({ login: vi.fn() }))

const mockedLogin = vi.mocked(loginRequest)

describe('useSession', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('stores an administrator session for the current tab', async () => {
    mockedLogin.mockResolvedValue({
      token: 'jwt-token',
      user: { id: 1, username: 'admin', email: 'admin@example.com', role: 'admin' },
    })
    const session = useSession()

    await session.login({ email: 'admin@example.com', password: 'password123' })
    await flushPromises()

    expect(session.session.value?.token).toBe('jwt-token')
    expect(sessionStorage.getItem('hatesentry-operator-session')).toContain('jwt-token')
  })

  it('rejects a non-admin account without storing it', async () => {
    mockedLogin.mockResolvedValue({
      token: 'user-token',
      user: { id: 2, username: 'user', email: 'user@example.com', role: 'user' },
    })
    const session = useSession()

    await expect(
      session.login({ email: 'user@example.com', password: 'password123' }),
    ).rejects.toThrow('没有人工复核权限')
    expect(session.session.value).toBeNull()
    expect(sessionStorage.getItem('hatesentry-operator-session')).toBeNull()
  })
})
