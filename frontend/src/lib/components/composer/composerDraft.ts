export type DraftSaveStatus = 'idle' | 'saving' | 'saved' | 'error'
export type DraftSyncStatus = 'pending' | 'synced' | 'failed'

type DraftStatusMeta = {
  icon: string
  color: string
  labelKey: string
}

export function getDraftStatusMeta(
  saveStatus: DraftSaveStatus,
  syncStatus: DraftSyncStatus,
  hasSavedAt: boolean,
): DraftStatusMeta {
  if (saveStatus === 'saving') {
    return { icon: 'mdi:loading', color: '', labelKey: 'composer.saving' }
  }
  if (saveStatus === 'error') {
    return { icon: 'mdi:alert-circle', color: 'text-red-500', labelKey: 'composer.saveFailed' }
  }
  if (saveStatus !== 'saved' || !hasSavedAt) {
    return { icon: '', color: '', labelKey: '' }
  }

  switch (syncStatus) {
    case 'synced':
      return { icon: 'mdi:cloud-check', color: 'text-green-500', labelKey: 'composer.synced' }
    case 'pending':
      return { icon: 'mdi:cloud-upload', color: 'text-blue-500', labelKey: 'composer.savedLocally' }
    case 'failed':
      return { icon: 'mdi:cloud-off-outline', color: 'text-yellow-500', labelKey: 'composer.savedLocallyOffline' }
  }
}

export function buildDraftContentHash(input: {
  toCount: number
  ccCount: number
  bccCount: number
  subject: string
  bodyContent: string
  attachmentNames: string
  isPlainTextMode: boolean
}): string {
  return [
    input.toCount,
    input.ccCount,
    input.bccCount,
    input.subject,
    input.bodyContent,
    input.attachmentNames,
    input.isPlainTextMode,
  ].join('|')
}
