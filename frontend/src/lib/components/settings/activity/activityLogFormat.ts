import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { formatLocalDateTime } from '$lib/utils/date'
import { backupStatistics } from '$lib/backup/backupStatistics'
import type { BackupStatistics } from '$lib/backup/backupStatistics'
import type { ActivityLog, ActivityLogPayload } from './activityLogTypes'

export interface BackupActivityDetails {
  mode: string
  statistics: BackupStatistics
  directory: string
}

function activityPayload(log: ActivityLog): ActivityLogPayload {
  if (log.payload) return log.payload
  if (!log.payloadJson) return {}
  try { return JSON.parse(log.payloadJson) as ActivityLogPayload } catch { return {} }
}

export function backupActivityDetails(log: ActivityLog): BackupActivityDetails | null {
  if (log.type !== 'backup') return null
  const payload = activityPayload(log)
  if (payload.mode !== 'incremental' && payload.mode !== 'full') return null
  const t = get(_)
  return {
    mode: payload.mode === 'incremental' ? t('settingsBackup.incrementalExport') : t('settingsBackup.fullExport'),
    statistics: backupStatistics({
      total: payload.total,
      completed: payload.backedUp ?? payload.completed ?? payload.success,
      exported: payload.added,
      skipped: payload.skipped,
      missing: payload.missing,
      unavailable: payload.unavailable,
      failed: payload.failed,
    }),
    directory: payload.directory?.trim() ?? '',
  }
}

export function activityTypeLabel(type: string): string {
  const t = get(_)
  if (type === 'sync') return t('activityLog.sync')
  if (type === 'backup') return t('activityLog.backup')
  return type
}

export function activityStatusLabel(status: string): string {
  const t = get(_)
  if (status === 'success') return t('activityLog.status.success')
  if (status === 'partial') return t('activityLog.status.partial')
  if (status === 'failed') return t('activityLog.status.failed')
  if (status === 'cancelled') return t('activityLog.status.cancelled')
  return t('activityLog.status.unknown')
}

export function activityTime(value: string): string {
  try { return formatLocalDateTime(value) } catch { return value }
}

export function activitySummary(log: ActivityLog): string {
  const t = get(_)
  const payload = activityPayload(log)
  if (log.type === 'backup') {
    const details = backupActivityDetails(log)
    if (!details) {
      return log.status === 'cancelled' ? t('activityLog.backupCancelled') : t('activityLog.backupFailed')
    }
    return t('activityLog.backupCompactSummary', {
      values: {
        mode: details.mode,
        backedUp: details.statistics.backedUp,
        notBackedUp: details.statistics.notBackedUp,
      },
    })
  }
  if (log.type === 'sync') {
    const folder = payload.folderName || log.title
    const target = payload.accountEmail ? `${payload.accountEmail} · ${folder}` : folder
    return t('activityLog.syncSummary', { values: { target, added: payload.added ?? 0 } })
  }
  return log.summary
}
