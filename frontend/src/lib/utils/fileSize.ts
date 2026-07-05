export function formatFileSize(size: number): string {
  if (!Number.isFinite(size) || size < 0) return ''
  if (size === 0) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB']
  let value = size
  let unit = 0

  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }

  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}
