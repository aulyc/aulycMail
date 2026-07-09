// Tracks open dialogs that should prevent background reloads from destroying
// the component tree (e.g., folder picker dialog dismissed by sync reload).
let count = 0
const listeners = new Set<(active: boolean) => void>()

export function dialogGuardOpen() {
  const wasActive = isDialogGuardActive()
  count++
  notifyIfChanged(wasActive)
}

export function dialogGuardClose() {
  const wasActive = isDialogGuardActive()
  count = Math.max(0, count - 1)
  notifyIfChanged(wasActive)
}

export function isDialogGuardActive(): boolean {
  return count > 0
}

export function onDialogGuardChange(listener: (active: boolean) => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function notifyIfChanged(wasActive: boolean) {
  const active = isDialogGuardActive()
  if (active === wasActive) return
  listeners.forEach(listener => listener(active))
}
