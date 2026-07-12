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
} from '@/api'
import type { ClientApplication } from '@/types'
import { useClients } from './useClients'

vi.mock('@/api', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/api')>(),
  activateClient: vi.fn(), createClient: vi.fn(), deactivateClient: vi.fn(),
  listClients: vi.fn(), listModerationPolicies: vi.fn(), rotateClientAPIKey: vi.fn(),
  updateClientPolicy: vi.fn(),
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
    expect(state.credential.value?.apiKey).toBe('hs_created_secret')
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
    expect(state.credential.value?.apiKey).toBe('hs_new_secret')
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
    expect(state.credential.value?.apiKey).toBe('hs_first_secret')
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
})
