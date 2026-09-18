import { apiGet, apiPost, apiPut } from '@/utils/request'
import type { ChangeOrder, ChangeOrderSummary, CreateChangeOrderRequest, PageResult, ReviewChangeOrderRequest } from '@/types'
import { API_PATHS } from '@/constants/apiPaths'

export function listChangeOrders(params?: Record<string, unknown>): Promise<PageResult<ChangeOrder> | ChangeOrder[]> {
  return apiGet<PageResult<ChangeOrder> | ChangeOrder[]>(API_PATHS.changeOrders, params)
}

export function getChangeOrder(id: number): Promise<ChangeOrder> {
  return apiGet<ChangeOrder>(`${API_PATHS.changeOrders}/${id}`)
}

export function createChangeOrder(body: CreateChangeOrderRequest): Promise<ChangeOrder> {
  return apiPost<ChangeOrder>(API_PATHS.changeOrders, body)
}

export function reviewChangeOrder(id: number, body: ReviewChangeOrderRequest): Promise<ChangeOrder> {
  return apiPut<ChangeOrder>(`${API_PATHS.changeOrders}/${id}/review`, body)
}

export function getChangeOrderSummary(projectId: number): Promise<ChangeOrderSummary> {
  return apiGet<ChangeOrderSummary>(`${API_PATHS.changeOrders}/summary`, { project_id: projectId })
}
