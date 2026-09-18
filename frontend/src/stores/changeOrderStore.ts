import { create } from 'zustand'
import type { ChangeOrder, ChangeOrderSummary } from '@/types'
import { getChangeOrderSummary, listChangeOrders } from '@/api/changeOrder'

interface ChangeOrderState {
  orders: ChangeOrder[]
  summary: ChangeOrderSummary | null
  loading: boolean
  fetchOrders: (projectId?: number) => Promise<void>
  fetchSummary: (projectId?: number) => Promise<void>
}

export const useChangeOrderStore = create<ChangeOrderState>((set) => ({
  orders: [],
  summary: null,
  loading: false,
  fetchOrders: async (projectId) => {
    set({ loading: true })
    try {
      const result = await listChangeOrders(projectId ? { project_id: projectId } : { page: 1, page_size: 100 })
      if (Array.isArray(result)) {
        set({ orders: result, loading: false })
      } else {
        set({ orders: result.list, loading: false })
      }
    } catch {
      set({ loading: false })
    }
  },
  fetchSummary: async (projectId) => {
    if (!projectId) {
      set({ summary: null })
      return
    }
    try {
      const summary = await getChangeOrderSummary(projectId)
      set({ summary })
    } catch {
      set({ summary: null })
    }
  },
}))
