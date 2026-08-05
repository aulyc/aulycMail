import type { DraftSaveStatus } from './composerDraft'

interface DraftSaveControllerOptions {
  delayMs: number
  hasContent: () => boolean
  getContentHash: () => string
  hasPersistedDraft: () => boolean
  getStatus: () => DraftSaveStatus
  setStatus: (status: DraftSaveStatus) => void
  save: () => Promise<void>
  onError?: (error: unknown) => void
}

export interface DraftSaveController {
  schedule(): void
  saveNow(): Promise<boolean>
  seed(contentHash: string): void
  cancelPending(): void
  setDiscarding(value: boolean): void
  waitForIdle(): Promise<void>
  destroy(): void
}

export function createDraftSaveController(options: DraftSaveControllerOptions): DraftSaveController {
  let timeout: ReturnType<typeof setTimeout> | null = null
  let lastContent = ''
  let saving = false
  let discarding = false
  let savingComplete: Promise<void> = Promise.resolve()

  const cancelPending = () => {
    if (timeout) clearTimeout(timeout)
    timeout = null
  }

  const saveNow = async (): Promise<boolean> => {
    if (discarding || saving || !options.hasContent()) return false

    const contentHash = options.getContentHash()
    if (contentHash === lastContent && options.hasPersistedDraft()) return false

    let resolveSaving!: () => void
    savingComplete = new Promise<void>((resolve) => { resolveSaving = resolve })
    saving = true
    options.setStatus('saving')
    try {
      await options.save()
      lastContent = contentHash
      options.setStatus('saved')
      return true
    } catch (error) {
      options.setStatus('error')
      options.onError?.(error)
      return false
    } finally {
      saving = false
      resolveSaving()
    }
  }

  return {
    schedule() {
      cancelPending()
      if (options.getStatus() === 'saved') options.setStatus('idle')
      timeout = setTimeout(async () => {
        timeout = null
        if (!options.hasContent() || options.getContentHash() === lastContent) return
        await saveNow()
      }, options.delayMs)
    },
    saveNow,
    seed(contentHash) {
      lastContent = contentHash
    },
    cancelPending,
    setDiscarding(value) {
      discarding = value
      if (value) cancelPending()
    },
    waitForIdle() {
      return savingComplete
    },
    destroy() {
      cancelPending()
    },
  }
}
