export interface BackupCounterInput {
  total?: number
  exported?: number
  skipped?: number
  missing?: number
  unavailable?: number
  failed?: number
  completed?: number
}

export interface BackupStatistics {
  checked: number
  backedUp: number
  notBackedUp: number
  newlyBackedUp: number
  previouslyBackedUp: number
  serverNotReturned: number
  noReadableSource: number
  processingFailed: number
}

export function backupStatistics(input: BackupCounterInput): BackupStatistics {
  const newlyBackedUp = input.exported ?? 0
  const previouslyBackedUp = input.skipped ?? 0
  const serverNotReturned = input.missing ?? 0
  const noReadableSource = input.unavailable ?? 0
  const processingFailed = input.failed ?? 0
  const backedUp = input.completed ?? newlyBackedUp + previouslyBackedUp
  const notBackedUp = serverNotReturned + noReadableSource + processingFailed

  return {
    checked: input.total ?? backedUp + notBackedUp,
    backedUp,
    notBackedUp,
    newlyBackedUp,
    previouslyBackedUp,
    serverNotReturned,
    noReadableSource,
    processingFailed,
  }
}

export function backupProgressPercent(current: number, total: number): number | null {
  if (!Number.isFinite(total) || total <= 0) return null
  if (!Number.isFinite(current)) return 0
  return Math.min(100, Math.max(0, Math.round(current / total * 100)))
}
