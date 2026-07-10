export interface Debouncer {
  schedule(callback: () => void, delayMs?: number): void
  cancel(): void
}

export function createDebouncer(defaultDelayMs: number): Debouncer {
  let timer: ReturnType<typeof setTimeout> | null = null

  return {
    schedule(callback: () => void, delayMs = defaultDelayMs) {
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => {
        timer = null
        callback()
      }, delayMs)
    },
    cancel() {
      if (!timer) return
      clearTimeout(timer)
      timer = null
    },
  }
}
