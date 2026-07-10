import {
  Archive,
  CopyToFolder,
  DeletePermanently,
  MarkAsNotSpam,
  MarkAsRead,
  MarkAsSpam,
  MarkAsUnread,
  MoveToFolder,
  Star,
  Trash,
  Undo,
  Unstar,
} from '../../wailsjs/go/app/App'
import { _ } from '$lib/i18n'
import { toasts } from '$lib/stores/toast'
import { get } from 'svelte/store'

type CompleteCallback = (autoSelectNext?: boolean) => void | Promise<void>

interface MailActionOptions {
  onUndo?: () => void | Promise<void>
  onSuccess?: CompleteCallback
  onError?: (err: unknown) => void | Promise<void>
  autoSelectNext?: boolean
}

function t(key: string, options?: Record<string, unknown>): string {
  return get(_)(key, options)
}

function undoActions(onUndo?: () => void | Promise<void>) {
  if (!onUndo) return []
  return [{ label: t('common.undo'), onClick: () => { void onUndo() } }]
}

async function notifySuccess(options?: MailActionOptions) {
  await options?.onSuccess?.(options.autoSelectNext)
}

async function notifyError(options: MailActionOptions | undefined, err: unknown) {
  await options?.onError?.(err)
}

export async function undoLastMailAction(options?: Pick<MailActionOptions, 'onSuccess' | 'onError'>) {
  try {
    const description = await Undo()
    toasts.success(t('toast.undone', { values: { description } }))
    await notifySuccess(options)
  } catch (err) {
    console.error('Undo failed:', err)
    toasts.error(t('toast.undoFailed'))
    await notifyError(options, err)
  }
}

export async function archiveMessages(messageIds: string[], options?: MailActionOptions & { successKey?: string }) {
  try {
    await Archive(messageIds)
    toasts.success(t(options?.successKey ?? 'toast.archived'), undoActions(options?.onUndo))
    await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
  } catch (err) {
    console.error('Archive failed:', err)
    toasts.error(t('toast.failedToArchive'))
    await notifyError(options, err)
  }
}

export async function trashMessages(messageIds: string[], options?: MailActionOptions) {
  try {
    const movedToTrash = await Trash(messageIds)
    const toastMsg = movedToTrash ? t('toast.movedToTrash') : t('toast.deletedFromFolder')
    toasts.success(toastMsg, movedToTrash ? undoActions(options?.onUndo) : [])
    await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
  } catch (err) {
    console.error('Delete failed:', err)
    toasts.error(t('toast.failedToDelete'))
    await notifyError(options, err)
  }
}

export async function deleteMessagesPermanently(messageIds: string[], options?: MailActionOptions) {
  try {
    await DeletePermanently(messageIds)
    toasts.success(t('toast.permanentlyDeleted'))
    await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
  } catch (err) {
    console.error('Permanent delete failed:', err)
    toasts.error(t('toast.failedToDelete'))
    await notifyError(options, err)
  }
}

export async function toggleSpamMessages(
  messageIds: string[],
  isSpamFolder: boolean,
  options?: MailActionOptions & { spamSuccessMode?: 'dynamic' | 'alwaysMarked' },
) {
  try {
    if (isSpamFolder) {
      await MarkAsNotSpam(messageIds)
      toasts.success(t('toast.markedAsNotSpam'), undoActions(options?.onUndo))
      await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
      return
    }

    const movedToSpam = await MarkAsSpam(messageIds)
    if (options?.spamSuccessMode === 'alwaysMarked') {
      toasts.success(t('toast.markedAsSpam'), undoActions(options?.onUndo))
      await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
      return
    }

    const toastMsg = movedToSpam ? t('toast.markedAsSpam') : t('toast.deletedFromFolder')
    toasts.success(toastMsg, movedToSpam ? undoActions(options?.onUndo) : [])
    await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
  } catch (err) {
    console.error('Spam toggle failed:', err)
    toasts.error(t(isSpamFolder ? 'toast.failedToMarkAsNotSpam' : 'toast.failedToMarkAsSpam'))
    await notifyError(options, err)
  }
}

export async function toggleStarMessages(
  messageIds: string[],
  shouldStar: boolean,
  options?: MailActionOptions & { unstarSuccessKey?: string },
) {
  try {
    if (shouldStar) {
      await Star(messageIds)
      toasts.success(t('toast.starred'))
    } else {
      await Unstar(messageIds)
      toasts.success(t(options?.unstarSuccessKey ?? 'toast.starRemoved'))
    }
    await notifySuccess(options)
  } catch (err) {
    console.error('Star toggle failed:', err)
    toasts.error(t('toast.failedToUpdateStar'))
    await notifyError(options, err)
  }
}

export async function setReadStateMessages(
  messageIds: string[],
  shouldRead: boolean,
  options?: MailActionOptions & { errorKey?: string },
) {
  try {
    if (shouldRead) {
      await MarkAsRead(messageIds)
      toasts.success(t('toast.markedAsRead'))
    } else {
      await MarkAsUnread(messageIds)
      toasts.success(t('toast.markedAsUnread'))
    }
    await notifySuccess(options)
  } catch (err) {
    console.error('Read status update failed:', err)
    toasts.error(t(options?.errorKey ?? 'toast.failedToUpdateReadStatus'))
    await notifyError(options, err)
  }
}

export async function moveMessagesToFolder(messageIds: string[], destFolderId: string, folderName: string, options?: MailActionOptions) {
  try {
    await MoveToFolder(messageIds, destFolderId)
    toasts.success(t('toast.movedTo', { values: { folder: folderName } }), undoActions(options?.onUndo))
    await notifySuccess({ ...options, autoSelectNext: options?.autoSelectNext ?? true })
  } catch (err) {
    console.error('Move failed:', err)
    toasts.error(t('toast.failedToMove'))
    await notifyError(options, err)
  }
}

export async function copyMessagesToFolder(messageIds: string[], destFolderId: string, folderName: string, options?: MailActionOptions) {
  try {
    await CopyToFolder(messageIds, destFolderId)
    toasts.success(t('toast.copyingTo', { values: { folder: folderName } }))
    await notifySuccess(options)
  } catch (err) {
    console.error('Copy failed:', err)
    toasts.error(t('toast.failedToCopy'))
    await notifyError(options, err)
  }
}
