import type { ChangeOrderStatus } from './enums'

export interface ChangeOrder {
  id: number
  project_id: number
  node_id: number
  title: string
  amount: number
  schedule_impact_days: number
  status: ChangeOrderStatus
  submitted_by: number
  reviewed_by: number
  review_note: string
  reviewed_at: string | null
  created_at: string
  updated_at: string
}

export interface CreateChangeOrderRequest {
  project_id: number
  node_id: number
  title: string
  amount: number
  schedule_impact_days?: number
}

export interface ReviewChangeOrderRequest {
  approved: boolean
  note?: string
}

export interface ChangeOrderSummary {
  project_id: number
  contract_amount: number
  used_budget: number
  pending_amount: number
  approved_amount: number
  pending_count: number
  approved_count: number
  remaining_amount: number
}
