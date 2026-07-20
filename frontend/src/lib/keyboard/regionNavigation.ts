export type MainKeyboardRegion = 'featureNav' | 'sidebar' | 'messageList' | 'viewer'

export const MAIN_KEYBOARD_REGION_ORDER: readonly MainKeyboardRegion[] = [
  'featureNav',
  'sidebar',
  'messageList',
  'viewer',
]

export function nextVisibleRegion(
  current: MainKeyboardRegion,
  direction: 1 | -1,
  visibleRegions: readonly MainKeyboardRegion[],
): MainKeyboardRegion {
  const visible = new Set(visibleRegions)
  if (visible.size === 0) return current

  const currentIndex = MAIN_KEYBOARD_REGION_ORDER.indexOf(current)
  const startIndex = currentIndex >= 0 ? currentIndex : 0
  for (let offset = 1; offset <= MAIN_KEYBOARD_REGION_ORDER.length; offset++) {
    const index = (
      startIndex + direction * offset + MAIN_KEYBOARD_REGION_ORDER.length
    ) % MAIN_KEYBOARD_REGION_ORDER.length
    const candidate = MAIN_KEYBOARD_REGION_ORDER[index]
    if (visible.has(candidate)) return candidate
  }

  return current
}

export type RovingNavigationKey = 'ArrowUp' | 'ArrowDown' | 'Home' | 'End'

export function nextRovingIndex(
  key: RovingNavigationKey,
  currentIndex: number,
  itemCount: number,
  wrap = false,
): number {
  if (itemCount <= 0) return -1
  if (key === 'Home') return 0
  if (key === 'End') return itemCount - 1

  const normalized = currentIndex >= 0 && currentIndex < itemCount
    ? currentIndex
    : (key === 'ArrowUp' ? itemCount : -1)
  if (key === 'ArrowDown') {
    if (wrap) return (normalized + 1 + itemCount) % itemCount
    return Math.min(normalized + 1, itemCount - 1)
  }
  if (wrap) return (normalized - 1 + itemCount) % itemCount
  return Math.max(normalized - 1, 0)
}
