export type Decision = 'allow' | 'review' | 'block'
export type ReviewStatus = 'pending' | 'approved' | 'rejected' | 'mistake'
export type CompletedReviewStatus = Exclude<ReviewStatus, 'pending'>
export type ReviewHistoryFilter = 'all' | CompletedReviewStatus
export type ReviewAction = 'approve' | 'reject' | 'mark-mistake'

export interface UserInfo {
  id: number
  username: string
  email: string
  role: string
}

export interface Session {
  token: string
  user: UserInfo
}

export interface LoginCredentials {
  email: string
  password: string
}

export interface ReviewCase {
  id: number
  request_id: string
  user_id: number
  client_id?: number
  content: string
  source: string
  external_id?: string
  actor_id?: string
  status: ReviewStatus
  policy_decision: Decision
  final_decision?: Decision
  risk_score: number
  labels: readonly string[]
  reason: string
  policy_version: string
  reviewer_id?: number
  review_notes?: string
  reviewed_at?: string
  created_at: string
}

export interface ReviewHistoryPage {
  items: ReviewCase[]
  next_cursor?: string
}

export interface ReviewActionInput {
  action: ReviewAction
  notes: string
  finalDecision?: 'allow' | 'block'
}

export type ClientStatus = 'active' | 'inactive'

export interface ClientApplication {
  id: number
  name: string
  status: ClientStatus
  api_key_prefix: string
  webhook_url?: string
  policy_version?: string
  created_at: string
  updated_at?: string
}

export interface CreatedClientCredential extends ClientApplication {
  api_key: string
}

export interface RotatedClientCredential {
  id: number
  name: string
  status: ClientStatus
  api_key: string
  api_key_prefix: string
  webhook_url?: string
  policy_version?: string
  updated_at: string
}

export interface ModerationPolicy {
  version: string
  review_threshold: number
  block_threshold: number
  default: boolean
}

export interface WebhookUpdateCredential extends ClientApplication {
  webhook_secret?: string
}

export interface ErrorResponse {
  code?: string
  error?: string
  message?: string
  details?: string
  trace_id?: string
}
