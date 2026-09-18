import { create } from 'zustand'
import type { ChangeOrder } from '@/types'
import { listChangeOrders } from '@/api/change'

interface ChangeOrderState {
  orders: ChangeOrder[]
  loading: boolean
  fetchOrders: (projectId?: number) => Promise<void>
}

export const useChangeOrderStore = create<ChangeOrderState>((set) => ({
  orders: [],
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
}))
