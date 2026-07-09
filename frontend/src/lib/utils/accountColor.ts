type AccountColorSource = {
  color?: string
  orderIndex?: number
}

const defaultAccountColors = [
  '#3B82F6',
  '#10B981',
  '#F59E0B',
  '#EF4444',
  '#8B5CF6',
  '#EC4899',
  '#06B6D4',
  '#F97316',
]

export function getAccountColor(source: AccountColorSource | null | undefined): string {
  if (source?.color) return source.color
  const index = Math.max(0, source?.orderIndex || 0)
  return defaultAccountColors[index % defaultAccountColors.length]
}
