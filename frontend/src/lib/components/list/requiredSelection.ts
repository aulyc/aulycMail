export function resolveRequiredSelectionIndex(
  itemIds: readonly string[],
  selectedId: string | null,
  preferredIndex = 0,
): number {
  if (itemIds.length === 0) return -1
  if (selectedId) {
    const selectedIndex = itemIds.indexOf(selectedId)
    if (selectedIndex >= 0) return selectedIndex
  }
  return Math.max(0, Math.min(preferredIndex, itemIds.length - 1))
}
