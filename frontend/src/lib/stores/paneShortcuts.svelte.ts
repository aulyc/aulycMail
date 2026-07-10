// Rail-pane shortcut registry.
//
// Non-mail panes register pane-local keyboard shortcuts here at component
// mount; the host's global key handler dispatches through this registry when
// the active pane is not Mail.

import { getActivePane } from './uiState.svelte'

type ShortcutPredicate = (e: KeyboardEvent) => boolean
type ShortcutHandler = (e: KeyboardEvent) => void
type Unregister = () => void

interface Registration {
  predicate: ShortcutPredicate
  handler: ShortcutHandler
}

// Indexed by rail pane id -> ordered list of registrations.
const registry = new Map<string, Registration[]>()

export function registerPaneShortcut(
  paneId: string,
  predicate: ShortcutPredicate,
  handler: ShortcutHandler,
): Unregister {
  const reg: Registration = { predicate, handler }
  const existing = registry.get(paneId)
  if (existing) {
    existing.push(reg)
  } else {
    registry.set(paneId, [reg])
  }
  return () => {
    const list = registry.get(paneId)
    if (!list) return
    const idx = list.indexOf(reg)
    if (idx >= 0) list.splice(idx, 1)
    if (list.length === 0) registry.delete(paneId)
  }
}

export function dispatchPaneShortcut(e: KeyboardEvent): boolean {
  const pane = getActivePane()
  if (!pane || pane === 'mail') return false
  const list = registry.get(pane)
  if (!list) return false
  for (const reg of list) {
    if (reg.predicate(e)) {
      reg.handler(e)
      return true
    }
  }
  return false
}
