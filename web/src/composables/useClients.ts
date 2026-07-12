import { readonly, ref, shallowRef } from 'vue'

import {
  activateClient,
  ApiError,
  createClient,
  deactivateClient,
  listClients,
  rotateClientAPIKey,
} from '@/api'
import type { ClientApplication, CreatedClientCredential, RotatedClientCredential } from '@/types'

interface UseClientsOptions {
  token: string
  onUnauthorized: () => void
}

export interface OneTimeCredential {
  clientId: number
  clientName: string
  apiKey: string
  kind: 'created' | 'rotated'
}

function messageFor(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '操作失败，请稍后重试'
}

export function useClients(options: UseClientsOptions) {
  const items = ref<ClientApplication[]>([])
  const credential = shallowRef<OneTimeCredential | null>(null)
  const isLoading = shallowRef(false)
  const isCreating = shallowRef(false)
  const busyClientIds = ref<ReadonlySet<number>>(new Set())
  const error = shallowRef('')
  const notice = shallowRef('')
  let loadSequence = 0
  const operationSequences = new Map<number, number>()

  function handleError(cause: unknown): void {
    if (cause instanceof ApiError && cause.status === 401) {
      options.onUnauthorized()
      return
    }
    error.value = messageFor(cause)
  }

  function publicClient(client: CreatedClientCredential): ClientApplication {
    const { api_key: _apiKey, ...safeClient } = client
    return safeClient
  }

  function replaceClient(client: ClientApplication | RotatedClientCredential): void {
    const index = items.value.findIndex((item) => item.id === client.id)
    if (index < 0) return
    const { api_key: _apiKey, ...safeUpdate } = client as RotatedClientCredential
    items.value[index] = { ...items.value[index], ...safeUpdate }
  }

  function setBusy(id: number, busy: boolean): void {
    const next = new Set(busyClientIds.value)
    if (busy) next.add(id)
    else next.delete(id)
    busyClientIds.value = next
  }

  function invalidatePendingLoad(): void {
    loadSequence++
    isLoading.value = false
  }

  async function load(): Promise<void> {
    if (isCreating.value || busyClientIds.value.size > 0) return
    const sequence = ++loadSequence
    isLoading.value = true
    error.value = ''
    try {
      const clients = await listClients(options.token)
      if (sequence === loadSequence) items.value = clients
    } catch (cause) {
      if (sequence === loadSequence) handleError(cause)
    } finally {
      if (sequence === loadSequence) isLoading.value = false
    }
  }

  async function create(name: string): Promise<void> {
    if (isCreating.value) return
    if (credential.value) {
      error.value = '请先保存并关闭当前一次性 API Key，再创建其他客户端。'
      return
    }
    invalidatePendingLoad()
    isCreating.value = true
    error.value = ''
    notice.value = ''
    try {
      const created = await createClient(options.token, name)
      items.value = [publicClient(created), ...items.value]
      credential.value = {
        clientId: created.id,
        clientName: created.name,
        apiKey: created.api_key,
        kind: 'created',
      }
      notice.value = '客户端已创建。请立即保存一次性 API Key。'
    } catch (cause) {
      handleError(cause)
    } finally {
      isCreating.value = false
    }
  }

  async function setActive(client: ClientApplication, active: boolean): Promise<void> {
    await runClientOperation(client.id, async () => {
      const updated = active
        ? await activateClient(options.token, client.id)
        : await deactivateClient(options.token, client.id)
      replaceClient(updated)
      notice.value = active ? '客户端已启用。' : '客户端已停用，当前 API Key 已无法认证。'
    })
  }

  async function rotate(client: ClientApplication): Promise<void> {
    if (credential.value) {
      error.value = '请先保存并关闭当前一次性 API Key，再轮换其他密钥。'
      return
    }
    await runClientOperation(client.id, async () => {
      const rotated = await rotateClientAPIKey(options.token, client.id)
      replaceClient(rotated)
      credential.value = {
        clientId: rotated.id,
        clientName: rotated.name,
        apiKey: rotated.api_key,
        kind: 'rotated',
      }
      notice.value = 'API Key 已轮换，旧 Key 已立即失效。请保存新 Key。'
    })
  }

  async function runClientOperation(id: number, operation: () => Promise<void>): Promise<void> {
    if (busyClientIds.value.has(id)) return
    invalidatePendingLoad()
    const sequence = (operationSequences.get(id) ?? 0) + 1
    operationSequences.set(id, sequence)
    setBusy(id, true)
    error.value = ''
    notice.value = ''
    try {
      await operation()
    } catch (cause) {
      if (operationSequences.get(id) === sequence) handleError(cause)
    } finally {
      if (operationSequences.get(id) === sequence) setBusy(id, false)
    }
  }

  function clearCredential(): void {
    credential.value = null
  }

  return {
    items: readonly(items),
    credential: readonly(credential),
    isLoading: readonly(isLoading),
    isCreating: readonly(isCreating),
    busyClientIds: readonly(busyClientIds),
    error: readonly(error),
    notice: readonly(notice),
    load,
    create,
    setActive,
    rotate,
    clearCredential,
  }
}
