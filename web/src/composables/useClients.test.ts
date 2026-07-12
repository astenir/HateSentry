import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  activateClient,
  ApiError,
  createClient,
  deactivateClient,
  listClients,
  listModerationPolicies,
  rotateClientAPIKey,
  updateClientPolicy,
  updateClientWebhook,
} from '@/api'
import type { ClientApplication } from '@/types'
import { useClients } from './useClients'

vi.mock('@/api', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/api')>(),
  activateClient: vi.fn(), createClient: vi.fn(), deactivateClient: vi.fn(),
  listClients: vi.fn(), listModerationPolicies: vi.fn(), rotateClientAPIKey: vi.fn(),
  updateClientPolicy: vi.fn(),
  updateClientWebhook: vi.fn(),
}))

const client: ClientApplication = {
  id: 4, name: 'blog', status: 'active', api_key_prefix: 'hs_old_',
  policy_version: '', created_at: '2026-07-12T08:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useClients', () => {
  it('loads clients and creates one with an in-memory one-time key', async () => {
    vi.mocked(listClients).mockResolvedValue([client])
    vi.mocked(createClient).mockResolvedValue({
      ...client, id: 5, name: 'forum', api_key: 'hs_created_secret', api_key_prefix: 'hs_created_',
    })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    await state.load()
    await state.create('forum')

    expect(state.items.value.map((item) => item.name)).toEqual(['forum', 'blog'])
    expect(state.items.value[0]).not.toHaveProperty('api_key')
    expect(state.credential.value).toMatchObject({ apiKey: 'hs_created_secret' })
    state.clearCredential()
    expect(state.credential.value).toBeNull()
  })

  it('updates status and keeps only the new prefix after rotation', async () => {
    vi.mocked(listClients).mockResolvedValue([client])
    vi.mocked(deactivateClient).mockResolvedValue({ ...client, status: 'inactive' })
    vi.mocked(activateClient).mockResolvedValue(client)
    vi.mocked(rotateClientAPIKey).mockResolvedValue({
      ...client, api_key: 'hs_new_secret', api_key_prefix: 'hs_new_',
      updated_at: '2026-07-12T09:00:00Z',
    })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.load()

    await state.setActive(client, false)
    expect(state.items.value[0].status).toBe('inactive')
    await state.setActive(state.items.value[0], true)
    expect(state.items.value[0].status).toBe('active')
    await state.rotate(client)

    expect(state.items.value[0].api_key_prefix).toBe('hs_new_')
    expect(state.items.value[0].created_at).toBe(client.created_at)
    expect(state.items.value[0]).not.toHaveProperty('api_key')
    expect(state.credential.value).toMatchObject({ apiKey: 'hs_new_secret' })
  })

  it('ends the session when a client operation returns unauthorized', async () => {
    vi.mocked(listClients).mockRejectedValue(new ApiError('expired', 401))
    const onUnauthorized = vi.fn()
    const state = useClients({ token: 'jwt', onUnauthorized })

    await state.load()

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('ignores a duplicate operation while the client is busy', async () => {
    let resolve: (value: ClientApplication) => void = () => undefined
    vi.mocked(deactivateClient).mockReturnValue(new Promise((done) => { resolve = done }))
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    const first = state.setActive(client, false)
    await state.setActive(client, false)
    resolve({ ...client, status: 'inactive' })
    await first

    expect(deactivateClient).toHaveBeenCalledOnce()
  })

  it('does not let a late list response erase a newly created client', async () => {
    let resolveList: (value: ClientApplication[]) => void = () => undefined
    vi.mocked(listClients).mockReturnValue(new Promise((done) => { resolveList = done }))
    vi.mocked(createClient).mockResolvedValue({
      ...client, id: 7, name: 'new-client', api_key: 'hs_new_client',
    })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    const loading = state.load()
    await state.create('new-client')
    resolveList([client])
    await loading

    expect(state.items.value.map((item) => item.name)).toEqual(['new-client'])
    expect(state.isLoading.value).toBe(false)
  })

  it('does not replace an unacknowledged one-time credential', async () => {
    vi.mocked(createClient).mockResolvedValue({
      ...client, api_key: 'hs_first_secret',
    })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    await state.create('first')
    await state.create('second')
    await state.rotate(client)

    expect(createClient).toHaveBeenCalledOnce()
    expect(rotateClientAPIKey).not.toHaveBeenCalled()
    expect(state.credential.value).toMatchObject({ apiKey: 'hs_first_secret' })
    expect(state.error.value).toContain('先保存并关闭')
  })

  it('ends the session when a key rotation returns unauthorized', async () => {
    vi.mocked(rotateClientAPIKey).mockRejectedValue(new ApiError('expired', 401))
    const onUnauthorized = vi.fn()
    const state = useClients({ token: 'jwt', onUnauthorized })

    await state.rotate(client)

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('does not start a list refresh while client creation is in flight', async () => {
    let resolveCreate: (value: Awaited<ReturnType<typeof createClient>>) => void = () => undefined
    vi.mocked(createClient).mockReturnValue(new Promise((done) => { resolveCreate = done }))
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    const creating = state.create('new-client')
    await state.load()
    resolveCreate({ ...client, id: 8, name: 'new-client', api_key: 'hs_create_inflight' })
    await creating

    expect(listClients).not.toHaveBeenCalled()
    expect(state.items.value[0].name).toBe('new-client')
  })

  it('does not start a list refresh while a status mutation is in flight', async () => {
    let resolveStatus: (value: ClientApplication) => void = () => undefined
    vi.mocked(deactivateClient).mockReturnValue(new Promise((done) => { resolveStatus = done }))
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    const deactivating = state.setActive(client, false)
    await state.load()
    resolveStatus({ ...client, status: 'inactive' })
    await deactivating

    expect(listClients).not.toHaveBeenCalled()
  })

  it('loads policy thresholds and assigns then resets a client policy', async () => {
    vi.mocked(listClients).mockResolvedValue([client])
    vi.mocked(listModerationPolicies).mockResolvedValue([
      { version: 'default-v1', review_threshold: 0.4, block_threshold: 0.75, default: true },
      { version: 'strict-v1', review_threshold: 0.2, block_threshold: 0.5, default: false },
    ])
    vi.mocked(updateClientPolicy)
      .mockResolvedValueOnce({ ...client, policy_version: 'strict-v1' })
      .mockResolvedValueOnce((({ policy_version: _policyVersion, ...withoutPolicy }) => withoutPolicy)({
        ...client, policy_version: 'strict-v1',
      }))
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    await state.load()
    await state.loadPolicies()
    await state.assignPolicy(client, 'strict-v1')
    expect(state.items.value[0].policy_version).toBe('strict-v1')
    expect(state.notice.value).toContain('strict-v1')
    await state.assignPolicy(state.items.value[0], '')

    expect(updateClientPolicy).toHaveBeenNthCalledWith(1, 'jwt', 4, 'strict-v1')
    expect(updateClientPolicy).toHaveBeenNthCalledWith(2, 'jwt', 4, '')
    expect(state.items.value[0].policy_version).toBe('')
    expect(state.policies.value).toHaveLength(2)
  })

  it('ends the session when policy assignment returns unauthorized', async () => {
    vi.mocked(updateClientPolicy).mockRejectedValue(new ApiError('expired', 401))
    const onUnauthorized = vi.fn()
    const state = useClients({ token: 'jwt', onUnauthorized })

    await state.assignPolicy(client, 'strict-v1')

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('configures a Webhook without leaking its one-time secret into the client list', async () => {
    vi.mocked(listClients).mockResolvedValue([client])
    vi.mocked(updateClientWebhook).mockResolvedValue({
      ...client,
      webhook_url: 'https://example.com/hook',
      webhook_secret: 'whsec_secret',
    })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.load()

    await state.configureWebhook(client, 'https://example.com/hook')

    expect(state.items.value[0].webhook_url).toBe('https://example.com/hook')
    expect(state.items.value[0]).not.toHaveProperty('webhook_secret')
    expect(state.credential.value).toMatchObject({
      kind: 'webhook', webhookSecret: 'whsec_secret',
    })
    expect(state.notice.value).toContain('旧签名 secret 已失效')
  })

  it('clears an omitted Webhook URL deterministically after closing the credential', async () => {
    const configured = { ...client, webhook_url: 'https://example.com/hook' }
    vi.mocked(listClients).mockResolvedValue([configured])
    vi.mocked(updateClientWebhook)
      .mockResolvedValueOnce({ ...configured, webhook_secret: 'whsec_secret' })
      .mockResolvedValueOnce((({ webhook_url: _url, ...withoutURL }) => withoutURL)(configured))
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.load()
    await state.configureWebhook(configured, 'https://example.com/new-hook')
    state.clearCredential()

    await state.configureWebhook(state.items.value[0], '')

    expect(updateClientWebhook).toHaveBeenLastCalledWith('jwt', 4, '')
    expect(state.items.value[0].webhook_url).toBe('')
    expect(state.credential.value).toBeNull()
    expect(state.notice.value).toContain('回调已停止')
  })

  it('does not overwrite an open one-time credential with a Webhook secret', async () => {
    vi.mocked(createClient).mockResolvedValue({ ...client, api_key: 'hs_secret' })
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })
    await state.create('blog')

    await state.configureWebhook(client, 'https://example.com/hook')

    expect(updateClientWebhook).not.toHaveBeenCalled()
    expect(state.credential.value).toMatchObject({ kind: 'created', apiKey: 'hs_secret' })
    expect(state.error.value).toContain('先保存并关闭')
  })

  it('ends the session when Webhook configuration returns unauthorized', async () => {
    vi.mocked(updateClientWebhook).mockRejectedValue(new ApiError('expired', 401))
    const onUnauthorized = vi.fn()
    const state = useClients({ token: 'jwt', onUnauthorized })

    await state.configureWebhook(client, 'https://example.com/hook')

    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('serializes all operations that can return a one-time credential', async () => {
    let resolveWebhook: (value: Awaited<ReturnType<typeof updateClientWebhook>>) => void = () => undefined
    vi.mocked(updateClientWebhook).mockReturnValue(new Promise((done) => { resolveWebhook = done }))
    const otherClient = { ...client, id: 9, name: 'support' }
    const state = useClients({ token: 'jwt', onUnauthorized: vi.fn() })

    const configuring = state.configureWebhook(client, 'https://example.com/hook')
    await state.create('new-client')
    await state.rotate(otherClient)
    await state.configureWebhook(otherClient, 'https://example.com/support-hook')

    expect(state.isGeneratingCredential.value).toBe(true)
    expect(createClient).not.toHaveBeenCalled()
    expect(rotateClientAPIKey).not.toHaveBeenCalled()
    expect(updateClientWebhook).toHaveBeenCalledOnce()

    resolveWebhook({
      ...client, webhook_url: 'https://example.com/hook', webhook_secret: 'whsec_first',
    })
    await configuring

    expect(state.isGeneratingCredential.value).toBe(false)
    expect(state.credential.value).toMatchObject({ webhookSecret: 'whsec_first' })
  })
})
