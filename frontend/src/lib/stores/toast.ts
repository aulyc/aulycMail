import { writable } from 'svelte/store'

interface ToastAction {
  label: string
  onClick: () => void
}

export interface Toast {
  id: string
  message: string
  type: 'success' | 'error' | 'info' | 'warning'
  duration?: number
  actions?: ToastAction[]
}

function createToastStore() {
  const { subscribe, update } = writable<Toast[]>([])

  function add(toast: Omit<Toast, 'id'>) {
    const id = crypto.randomUUID()
    const newToast: Toast = { ...toast, id }

    update(() => [newToast])

    // Auto-remove after duration (default 5 seconds)
    const duration = toast.duration ?? 5000
    setTimeout(() => {
      remove(id)
    }, duration)

    return id
  }

  function remove(id: string) {
    update(toasts => toasts.filter(t => t.id !== id))
  }

  function success(message: string, actions?: ToastAction[]) {
    return add({ message, type: 'success', actions })
  }

  function error(message: string, actions?: ToastAction[]) {
    return add({ message, type: 'error', actions })
  }

  function info(message: string, actions?: ToastAction[]) {
    return add({ message, type: 'info', actions })
  }

  function warning(message: string, actions?: ToastAction[]) {
    return add({ message, type: 'warning', actions })
  }

  return {
    subscribe,
    add,
    remove,
    success,
    error,
    info,
    warning
  }
}

export const toasts = createToastStore()

// Helper function for easy toast creation
export function addToast(toast: Omit<Toast, 'id'>) {
  return toasts.add(toast)
}
