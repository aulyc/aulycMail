export type SearchResultInputMode = 'idle' | 'keyboard' | 'pointer'

export function resultRowHighlightClass(
  index: number,
  activeIndex: number,
  inputMode: SearchResultInputMode,
): string {
  if (inputMode === 'keyboard') {
    return index === activeIndex ? 'bg-muted' : ''
  }
  return 'hover:bg-muted/50'
}
