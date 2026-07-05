const BACKUP_DIRECTORY_HISTORY_KEY = 'aulycmail.backupViewer.directoryHistory'
const BACKUP_DIRECTORY_HISTORY_EVENT = 'aulycmail:backup-directory-history-changed'
const MAX_BACKUP_DIRECTORY_HISTORY = 50

function canUseStorage(): boolean {
  return typeof window !== 'undefined' && typeof localStorage !== 'undefined'
}

function normalizeBackupDirectoryPath(path: string): string {
  return path.trim()
}

function uniqueBackupDirectoryHistory(paths: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const raw of paths) {
    const path = normalizeBackupDirectoryPath(raw)
    if (!path || seen.has(path)) continue
    seen.add(path)
    result.push(path)
    if (result.length >= MAX_BACKUP_DIRECTORY_HISTORY) break
  }
  return result
}

export function loadBackupDirectoryHistory(): string[] {
  if (!canUseStorage()) return []
  try {
    const raw = localStorage.getItem(BACKUP_DIRECTORY_HISTORY_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed)
      ? uniqueBackupDirectoryHistory(parsed.filter((value): value is string => typeof value === 'string'))
      : []
  } catch {
    return []
  }
}

function saveBackupDirectoryHistory(paths: string[]): string[] {
  const normalized = uniqueBackupDirectoryHistory(paths)
  if (!canUseStorage()) return normalized
  try {
    localStorage.setItem(BACKUP_DIRECTORY_HISTORY_KEY, JSON.stringify(normalized))
    window.dispatchEvent(new CustomEvent(BACKUP_DIRECTORY_HISTORY_EVENT, { detail: normalized }))
  } catch (err) {
    console.warn('Failed to persist backup directory history:', err)
  }
  return normalized
}

export function rememberBackupDirectory(path: string): string[] {
  const normalized = normalizeBackupDirectoryPath(path)
  if (!normalized) return loadBackupDirectoryHistory()
  const current = loadBackupDirectoryHistory()
  return saveBackupDirectoryHistory([normalized, ...current.filter((item) => item !== normalized)])
}

export function removeBackupDirectory(path: string): string[] {
  const normalized = normalizeBackupDirectoryPath(path)
  if (!normalized) return loadBackupDirectoryHistory()
  return saveBackupDirectoryHistory(loadBackupDirectoryHistory().filter((item) => item !== normalized))
}

export function subscribeBackupDirectoryHistory(listener: (paths: string[]) => void): () => void {
  if (typeof window === 'undefined') return () => {}

  const handleHistoryChange = (event: Event) => {
    listener(event instanceof CustomEvent && Array.isArray(event.detail) ? event.detail : loadBackupDirectoryHistory())
  }

  const handleStorage = (event: StorageEvent) => {
    if (event.key === BACKUP_DIRECTORY_HISTORY_KEY) {
      listener(loadBackupDirectoryHistory())
    }
  }

  window.addEventListener(BACKUP_DIRECTORY_HISTORY_EVENT, handleHistoryChange)
  window.addEventListener('storage', handleStorage)
  return () => {
    window.removeEventListener(BACKUP_DIRECTORY_HISTORY_EVENT, handleHistoryChange)
    window.removeEventListener('storage', handleStorage)
  }
}
