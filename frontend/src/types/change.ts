import type { ChangeOrderStatus } from './enums'

export interface ChangeOrder {
  id: number
  project_id: number
  node_id: number
  amount: number
  schedule_impact: number
  reason: string
  status: ChangeOrderStatus
  applicant_id: number
  reviewer_id: number
  review_comment: string
  reviewed_at: string | null
  created_at: string
  updated_at: string
}

export interface CreateChangeOrderRequest {
  project_id: number
  node_id: number
  amount: number
  schedule_impact: number
  reason: string
}

export interface ReviewChangeOrderRequest {
  approved: boolean
  comment?: string
}
