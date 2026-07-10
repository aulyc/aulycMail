// Runtime state for "Refresh contacts from mail". The backend scans stored
// messages and emits contacts:refreshProgress so the global status bar can show
// the user that a long refresh is still moving.

// @ts-ignore - wailsjs runtime
import { EventsOn } from '$wailsjs/runtime/runtime'

type ContactRefreshProgress = {
  phase?: string
  scanned?: number
  total?: number
}

let active = $state(false)
let scanned = $state(0)
let total = $state(0)
let eventsStarted = false

function normalizeCount(value: number | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : 0
}

function applyProgress(data: ContactRefreshProgress | null | undefined): void {
  const phase = data?.phase ?? 'scanning'
  scanned = normalizeCount(data?.scanned)
  total = normalizeCount(data?.total)

  if (phase === 'complete' || phase === 'error' || (total > 0 && scanned >= total)) {
    active = false
    return
  }

  active = true
}

export const contactRefresh = {
  get active(): boolean {
    return active
  },
  get scanned(): number {
    return scanned
  },
  get total(): number {
    return total
  },
  get percentage(): number | null {
    if (!active || total <= 0) return null
    return Math.min(100, Math.round((scanned / total) * 100))
  },
}

export function initContactRefreshEvents(): void {
  if (eventsStarted) return
  eventsStarted = true
  EventsOn('contacts:refreshProgress', applyProgress)
}

export function beginContactRefresh(): void {
  active = true
  scanned = 0
  total = 0
}

export function completeContactRefresh(scannedCount?: number, totalCount?: number): void {
  scanned = normalizeCount(scannedCount)
  total = normalizeCount(totalCount)
  active = false
}

export function failContactRefresh(): void {
  active = false
}
