import { get } from 'svelte/store'
import { _ } from '$lib/i18n'
import { formatLocalDateTime } from '$lib/utils/date'
import type { ActivityLog, ActivityLogPayload } from './activityLogTypes'

function payloadOf(log: ActivityLog): ActivityLogPayload {
  if (log.payload) return log.payload
  if (!log.payloadJson) return {}
  try { return JSON.parse(log.payloadJson) as ActivityLogPayload } catch { return {} }
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
  const payload = payloadOf(log)
  if (log.type === 'backup') {
    if (payload.mode !== 'incremental' && payload.mode !== 'full') {
      return log.status === 'cancelled' ? t('activityLog.backupCancelled') : t('activityLog.backupFailed')
    }
    const mode = payload.mode === 'incremental' ? t('settingsBackup.incrementalExport') : t('settingsBackup.fullExport')
    return t('activityLog.backupSummary', {
      values: {
        mode,
        completed: payload.completed ?? payload.success ?? (payload.added ?? 0) + (payload.skipped ?? 0),
        added: payload.added ?? 0,
        skipped: payload.skipped ?? 0,
        missing: payload.missing ?? 0,
        failed: payload.failed ?? 0,
      },
    })
  }
  if (log.type === 'sync') {
    const target = payload.folderName || log.title
    return t('activityLog.syncSummary', { values: { target, added: payload.added ?? 0 } })
  }
  return log.summary
}
