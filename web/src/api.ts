import type {
  ErrorResponse,
  ClientApplication,
  CreatedClientCredential,
  RotatedClientCredential,
  LoginCredentials,
  ModerationPolicy,
  ReviewActionInput,
  ReviewCase,
  ReviewHistoryFilter,
  ReviewHistoryPage,
  ReviewStatus,
  Session,
  WebhookUpdateCredential,
  WebhookDelivery,
  WebhookDeliveryFilter,
  OperationsStats,
} from './types'

const API_PREFIX = '/api/v1'

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code = '',
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_PREFIX}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  })

  const payload = await readJson(response)
  if (!response.ok) {
    const error = payload as ErrorResponse | null
    throw new ApiError(
      error?.message || '请求失败，请稍后重试',
      response.status,
      error?.code || error?.error || '',
    )
  }

  return payload as T
}

async function readJson(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) return null

  try {
    return JSON.parse(text)
  } catch {
    throw new ApiError('服务返回了无法解析的响应', response.status)
  }
}

function authorized(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` }
}

export function login(credentials: LoginCredentials): Promise<Session> {
  return request<Session>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(credentials),
  })
}

export async function listReviews(
  token: string,
  status: ReviewStatus,
): Promise<ReviewCase[]> {
  const response = await request<{ items: ReviewCase[] }>(`/reviews?status=${status}`, {
    headers: authorized(token),
  })
  return response.items
}

export function listPendingReviews(token: string): Promise<ReviewCase[]> {
  return listReviews(token, 'pending')
}

export async function listReviewHistory(
  token: string,
  filter: ReviewHistoryFilter,
  cursor = '',
): Promise<ReviewHistoryPage> {
  const query = new URLSearchParams({
    status: filter === 'all' ? 'completed' : filter,
    limit: '50',
  })
  if (cursor) query.set('cursor', cursor)

  return request<ReviewHistoryPage>(`/reviews?${query.toString()}`, {
    headers: authorized(token),
  })
}

export function getReview(token: string, id: number): Promise<ReviewCase> {
  return request<ReviewCase>(`/reviews/${id}`, {
    headers: authorized(token),
  })
}

export function finalizeReview(
  token: string,
  id: number,
  input: ReviewActionInput,
): Promise<ReviewCase> {
  const body: Record<string, string> = { notes: input.notes }
  if (input.action === 'mark-mistake' && input.finalDecision) {
    body.final_decision = input.finalDecision
  }

  return request<ReviewCase>(`/reviews/${id}/${input.action}`, {
    method: 'POST',
    headers: authorized(token),
    body: JSON.stringify(body),
  })
}

export async function listClients(token: string): Promise<ClientApplication[]> {
  const response = await request<{ items: ClientApplication[] }>('/admin/clients', {
    headers: authorized(token),
  })
  return response.items
}

export function createClient(token: string, name: string): Promise<CreatedClientCredential> {
  return request<CreatedClientCredential>('/admin/clients', {
    method: 'POST',
    headers: authorized(token),
    body: JSON.stringify({ name }),
  })
}

export function activateClient(token: string, id: number): Promise<ClientApplication> {
  return updateClient(token, id, 'activate')
}

export function deactivateClient(token: string, id: number): Promise<ClientApplication> {
  return updateClient(token, id, 'deactivate')
}

export function rotateClientAPIKey(token: string, id: number): Promise<RotatedClientCredential> {
  return request<RotatedClientCredential>(`/admin/clients/${id}/api-key/rotate`, {
    method: 'POST',
    headers: authorized(token),
  })
}

export async function listModerationPolicies(token: string): Promise<ModerationPolicy[]> {
  const response = await request<{ items: ModerationPolicy[] }>('/admin/moderation/policies', {
    headers: authorized(token),
  })
  return response.items
}

export function updateClientPolicy(
  token: string,
  id: number,
  policyVersion: string,
): Promise<ClientApplication> {
  return request<ClientApplication>(`/admin/clients/${id}/policy`, {
    method: 'POST',
    headers: authorized(token),
    body: JSON.stringify({ policy_version: policyVersion }),
  })
}

export function updateClientWebhook(
  token: string,
  id: number,
  webhookURL: string,
): Promise<WebhookUpdateCredential> {
  return request<WebhookUpdateCredential>(`/admin/clients/${id}/webhook`, {
    method: 'POST',
    headers: authorized(token),
    body: JSON.stringify({ webhook_url: webhookURL }),
  })
}

export async function listWebhookDeliveries(
  token: string,
  filter: WebhookDeliveryFilter,
): Promise<WebhookDelivery[]> {
  const query = new URLSearchParams({ limit: '50' })
  if (filter.status !== 'all') query.set('status', filter.status)
  if (filter.clientId.trim()) query.set('client_id', filter.clientId.trim())
  if (filter.requestId.trim()) query.set('request_id', filter.requestId.trim())
  const response = await request<{ items: WebhookDelivery[] }>(`/admin/webhook-deliveries?${query}`, {
    headers: authorized(token),
  })
  return response.items
}

export function retryWebhookDelivery(token: string, id: number): Promise<WebhookDelivery> {
  return request<WebhookDelivery>(`/admin/webhook-deliveries/${id}/retry`, {
    method: 'POST',
    headers: authorized(token),
  })
}

export function getOperationsStats(token: string): Promise<OperationsStats> {
  return request<OperationsStats>('/reviews/stats', {
    headers: authorized(token),
  })
}

function updateClient(
  token: string,
  id: number,
  action: 'activate' | 'deactivate',
): Promise<ClientApplication> {
  return request<ClientApplication>(`/admin/clients/${id}/${action}`, {
    method: 'POST',
    headers: authorized(token),
  })
}
