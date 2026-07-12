export type Decision = 'allow' | 'review' | 'block'
export type ReviewStatus = 'pending' | 'approved' | 'rejected' | 'mistake'
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

export interface ReviewActionInput {
  action: ReviewAction
  notes: string
  finalDecision?: 'allow' | 'block'
}

export interface ErrorResponse {
  code?: string
  error?: string
  message?: string
  details?: string
  trace_id?: string
}
