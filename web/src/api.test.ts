import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  activateClient,
  createClient,
  deactivateClient,
  finalizeReview,
  listClients,
  listModerationPolicies,
  listWebhookDeliveries,
  listPendingReviews,
  listReviewHistory,
  login,
  rotateClientAPIKey,
  retryWebhookDelivery,
  updateClientPolicy,
  updateClientWebhook,
} from './api'

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

  it('lists clients with the administrator bearer token', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listClients('jwt-token')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/clients', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer jwt-token' }),
    }))
  })

  it('creates a named client and preserves its one-time API key response', async () => {
    const created = {
      id: 11, name: 'blog', status: 'active', api_key: 'hs_live_secret',
      api_key_prefix: 'hs_live_', created_at: '2026-07-12T08:00:00Z',
    }
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(created), { status: 201 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const result = await createClient('jwt-token', 'blog')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/clients', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'blog' }),
    }))
    expect(result.api_key).toBe('hs_live_secret')
  })

  it('uses the explicit status and API key rotation paths', async () => {
    const payload = {
      id: 11, name: 'blog', status: 'active', api_key: 'hs_new_secret',
      api_key_prefix: 'hs_new_', created_at: '2026-07-12T08:00:00Z',
    }
    const fetchMock = vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify(payload), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await activateClient('jwt-token', 11)
    await deactivateClient('jwt-token', 11)
    const rotated = await rotateClientAPIKey('jwt-token', 11)

    expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([
      '/api/v1/admin/clients/11/activate',
      '/api/v1/admin/clients/11/deactivate',
      '/api/v1/admin/clients/11/api-key/rotate',
    ])
    expect(rotated.api_key).toBe('hs_new_secret')
  })

  it('loads configured policies and updates a client policy assignment', async () => {
    const policyResponse = { items: [{
      version: 'strict-v1', review_threshold: 0.2, block_threshold: 0.5, default: false,
    }] }
    const clientResponse = {
      id: 11, name: 'blog', status: 'active', api_key_prefix: 'hs_blog_',
      policy_version: 'strict-v1', created_at: '2026-07-12T08:00:00Z',
    }
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(policyResponse), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(clientResponse), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify((({ policy_version: _policyVersion, ...withoutPolicy }) => withoutPolicy)(clientResponse)), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const policies = await listModerationPolicies('jwt-token')
    const updated = await updateClientPolicy('jwt-token', 11, 'strict-v1')
    const reset = await updateClientPolicy('jwt-token', 11, '')

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/admin/moderation/policies', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer jwt-token' }),
    }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/admin/clients/11/policy', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ policy_version: 'strict-v1' }),
    }))
    expect(policies[0].review_threshold).toBe(0.2)
    expect(updated.policy_version).toBe('strict-v1')
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/admin/clients/11/policy', expect.objectContaining({
      body: JSON.stringify({ policy_version: '' }),
    }))
    expect(reset.policy_version).toBeUndefined()
  })

  it('configures and clears a client Webhook with the exact request body', async () => {
    const configured = {
      id: 11, name: 'blog', status: 'active', api_key_prefix: 'hs_blog_',
      webhook_url: 'https://example.com/hook', webhook_secret: 'whsec_secret',
      created_at: '2026-07-12T08:00:00Z',
    }
    const cleared = (({ webhook_url: _url, webhook_secret: _secret, ...rest }) => rest)(configured)
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(configured), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(cleared), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const result = await updateClientWebhook('jwt-token', 11, configured.webhook_url)
    const reset = await updateClientWebhook('jwt-token', 11, '')

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/admin/clients/11/webhook', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ webhook_url: configured.webhook_url }),
    }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/admin/clients/11/webhook', expect.objectContaining({
      body: JSON.stringify({ webhook_url: '' }),
    }))
    expect(result.webhook_secret).toBe('whsec_secret')
    expect(reset.webhook_url).toBeUndefined()
  })

  it('filters and retries Webhook deliveries with administrator authorization', async () => {
    const item = { id: 7, delivery_id: 'delivery-7', request_id: 'request-7', client_id: 4, event: 'moderation.finalized', status: 'failed', attempt_count: 1, last_attempt_at: '2026-07-12T08:00:00Z', created_at: '2026-07-12T08:00:00Z', updated_at: '2026-07-12T08:00:00Z' }
    const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ items: [item], ...item }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    await listWebhookDeliveries('jwt-token', { status: 'failed', clientId: ' 4 ', requestId: ' request-7 ' })
    await retryWebhookDelivery('jwt-token', 7)
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/admin/webhook-deliveries?limit=50&status=failed&client_id=4&request_id=request-7', expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer jwt-token' }) }))
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/admin/webhook-deliveries/7/retry', expect.objectContaining({ method: 'POST' }))
  })
})
