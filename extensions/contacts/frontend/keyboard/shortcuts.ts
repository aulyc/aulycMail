// Contacts pane keyboard shortcut predicates.
//
// Lives beside the Contacts pane so host-side keyboard files stay focused on
// global and Mail-domain shortcuts. Mail and the kit consume their shared
// predicates from `$lib/keyboard/shortcuts.ts`; Contacts owns these.
//
// Composable helpers (noMods, ctrlOrMeta, altOnly) are imported from the host
// shortcuts file so the modifier-checking conventions match mail's exactly.
// Predicates here get registered via registerExtensionShortcut at component
// mount; the host's global key handler dispatches them via
// dispatchExtensionShortcut when Contacts is the active rail pane.

import { noMods, ctrlOrMeta } from '$lib/keyboard/shortcuts'

/** `e` — edit the currently-focused contact. */
export const CONTACT_EDIT = (e: KeyboardEvent): boolean =>
  e.key === 'e' && noMods(e)

/** `Ctrl/Cmd+N` — open the local new-contact dialog. Routed by the rail-pane
 *  shortcut registry before App.svelte's mail-domain switch — only fires when
 *  contacts is the active rail. */
export const CONTACT_NEW = (e: KeyboardEvent): boolean =>
  e.key.toLowerCase() === 'n' && ctrlOrMeta(e) && !e.shiftKey && !e.altKey

export const KEY = {
  CONTACT_EDIT,
  CONTACT_NEW,
}
