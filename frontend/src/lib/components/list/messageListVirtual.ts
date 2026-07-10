type MessageListDensity = 'micro' | 'compact' | 'standard' | 'large'

type VirtualRow<T> = { item: T; index: number }

export type VirtualWindow<T> = {
  rows: VirtualRow<T>[]
  topHeight: number
  bottomHeight: number
}

const ROW_HEIGHT_BY_DENSITY: Record<MessageListDensity, number> = {
  micro: 66,
  compact: 80,
  standard: 94,
  large: 120,
}

const VIRTUAL_OVERSCAN = 8

export function getMessageListRowHeight(density: string): number {
  return ROW_HEIGHT_BY_DENSITY[density as MessageListDensity] ?? ROW_HEIGHT_BY_DENSITY.standard
}

export function getVirtualWindow<T>(
  items: T[],
  viewportHeight: number,
  scrollTop: number,
  rowHeight: number,
): VirtualWindow<T> {
  if (items.length === 0) {
    return { rows: [], topHeight: 0, bottomHeight: 0 }
  }

  const viewport = viewportHeight || 600
  const visibleCount = Math.ceil(viewport / rowHeight) + VIRTUAL_OVERSCAN * 2
  const maxStart = Math.max(0, items.length - visibleCount)
  const start = Math.min(maxStart, Math.max(0, Math.floor(scrollTop / rowHeight) - VIRTUAL_OVERSCAN))
  const end = Math.min(items.length, start + visibleCount)

  return {
    rows: items.slice(start, end).map((item, offset) => ({
      item,
      index: start + offset,
    })),
    topHeight: start * rowHeight,
    bottomHeight: (items.length - end) * rowHeight,
  }
}
