import { reactive } from 'vue'

export type ToastType = 'ok' | 'error' | 'warn' | 'info'

export interface Toast {
  id: number
  type: ToastType
  text: string
}

let nextId = 1

export const toasts = reactive<Toast[]>([])

export function showToast(type: ToastType, text: string, duration = 2200) {
  const t: Toast = { id: nextId++, type, text }
  toasts.push(t)
  window.setTimeout(() => {
    const i = toasts.findIndex((x) => x.id === t.id)
    if (i >= 0) toasts.splice(i, 1)
  }, duration)
}
