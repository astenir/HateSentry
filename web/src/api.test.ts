import { afterEach, describe, expect, it, vi } from 'vitest'

import { finalizeReview, listPendingReviews, login } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('review console API client', () => {
  it('logs in through the stable v1 auth endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          token: 'jwt-token',
          user: { id: 1, username: 'admin', email: 'admin@example.com', role: 'admin' },
        }),
        { status: 200 },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await login({ email: 'admin@example.com', password: 'password123' })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'admin@example.com', password: 'password123' }),
      }),
    )
  })

  it('uses the bearer token when loading the pending queue', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listPendingReviews('jwt-token')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/reviews?status=pending',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer jwt-token' }),
      }),
    )
  })

  it('preserves conflict status and code for concurrent review handling', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            code: 'CONFLICT',
            message: 'Review case is already finalized',
          }),
          { status: 409 },
        ),
      ),
    )

    await expect(
      finalizeReview('jwt-token', 3, { action: 'reject', notes: '' }),
    ).rejects.toMatchObject({
      status: 409,
      code: 'CONFLICT',
    })
  })
})
