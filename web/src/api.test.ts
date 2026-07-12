import { afterEach, describe, expect, it, vi } from 'vitest'

import { finalizeReview, listPendingReviews, listReviewHistory, login } from './api'

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

  it('loads all completed reviews through one cursor-paginated request', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [], next_cursor: 'cursor-3' }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const history = await listReviewHistory('jwt-token', 'all', 'cursor-2')

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/reviews?status=completed&limit=50&cursor=cursor-2',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer jwt-token' }),
      }),
    )
    expect(history.next_cursor).toBe('cursor-3')
  })

  it('uses the selected completed status without client-side aggregation', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listReviewHistory('jwt-token', 'mistake')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/reviews?status=mistake&limit=50',
      expect.any(Object),
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
