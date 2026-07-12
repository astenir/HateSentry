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

  it('combines completed review statuses and sorts them by review time', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      const status = new URL(url, 'http://console.local').searchParams.get('status')
      const items = status === 'approved'
        ? [{ id: 1, reviewed_at: '2026-07-12T09:00:00Z', created_at: '2026-07-12T07:00:00Z' }]
        : status === 'rejected'
          ? [{ id: 2, reviewed_at: '2026-07-12T09:00:00Z', created_at: '2026-07-12T07:30:00Z' }]
          : []
      return Promise.resolve(new Response(JSON.stringify({ items }), { status: 200 }))
    })
    vi.stubGlobal('fetch', fetchMock)

    const history = await listReviewHistory('jwt-token', 'all')

    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/reviews?status=approved',
      '/api/v1/reviews?status=rejected',
      '/api/v1/reviews?status=mistake',
    ])
    expect(history.map((item) => item.id)).toEqual([2, 1])
  })

  it('does not return partial all-history results when one status request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      const failed = url.includes('status=rejected')
      return Promise.resolve(new Response(
        JSON.stringify(failed ? { message: '历史查询失败' } : { items: [] }),
        { status: failed ? 503 : 200 },
      ))
    }))

    await expect(listReviewHistory('jwt-token', 'all')).rejects.toMatchObject({
      message: '历史查询失败',
      status: 503,
    })
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
