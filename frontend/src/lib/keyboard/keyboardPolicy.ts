export interface KeyboardPolicyEvent {
  key: string
  code?: string
  altKey: boolean
  ctrlKey: boolean
  metaKey: boolean
}

export type AppKeyboardPolicy = 'search' | 'enhanced' | 'native'

/**
 * Resolve the application-level keyboard policy for an event.
 *
 * Command/Ctrl+F is deliberately independent of the enhanced-keyboard setting.
 * "native" means aulycMail must not prevent the event or dispatch a custom
 * action; WebKit and macOS retain their ordinary behavior.
 */
export function resolveAppKeyboardPolicy(
  event: KeyboardPolicyEvent,
  enhancedKeyboardNavigation: boolean,
): AppKeyboardPolicy {
  if (
    (event.metaKey || event.ctrlKey)
    && !event.altKey
    && event.key.toLowerCase() === 'f'
  ) {
    return 'search'
  }
  return enhancedKeyboardNavigation ? 'enhanced' : 'native'
}

/**
 * Match Option-letter shortcuts by physical key code. On macOS, Option changes
 * KeyboardEvent.key to the produced symbol (for example Option+T becomes †),
 * while KeyboardEvent.code remains KeyT.
 */
export function isOptionCodeShortcut(
  event: KeyboardPolicyEvent,
  code: `Key${string}`,
): boolean {
  return event.altKey && !event.ctrlKey && !event.metaKey && event.code === code
}
