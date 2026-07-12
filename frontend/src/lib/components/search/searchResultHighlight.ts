export type SearchResultInputMode = 'idle' | 'keyboard' | 'pointer'

export const SEARCH_RESULT_VISIBLE_ROWS = 8
export const SEARCH_RESULT_ROW_HEIGHT_PX = 52
export const SEARCH_RESULT_VIEWPORT_HEIGHT_PX = SEARCH_RESULT_VISIBLE_ROWS * SEARCH_RESULT_ROW_HEIGHT_PX

export function searchResultScrollTopForIndex(
  index: number,
  currentScrollTop: number,
  viewportHeight: number,
): number {
  if (index < 0 || viewportHeight <= 0) return currentScrollTop

  const rowTop = index * SEARCH_RESULT_ROW_HEIGHT_PX
  const rowBottom = rowTop + SEARCH_RESULT_ROW_HEIGHT_PX
  if (rowTop < currentScrollTop) return rowTop
  if (rowBottom > currentScrollTop + viewportHeight) {
    return Math.max(0, rowBottom - viewportHeight)
  }
  return currentScrollTop
}

export function shouldActivatePointerResult(
  movementX: number,
  movementY: number,
  now: number,
  suppressedUntil: number,
): boolean {
  if (now < suppressedUntil) return false
  return movementX !== 0 || movementY !== 0
}

export function resultRowHighlightClass(
  index: number,
  activeIndex: number,
  inputMode: SearchResultInputMode,
): string {
  if (inputMode === 'keyboard') {
    return index === activeIndex ? 'bg-muted' : ''
  }
  if (inputMode === 'pointer') {
    return 'hover:bg-muted/50'
  }
  return ''
}
