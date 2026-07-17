export const FOLDER_OPEN_SYNC_RETRY_COOLDOWN_MS = 60_000

const DEFAULT_BACKGROUND_FOLDER_TYPES = new Set(['inbox', 'drafts', 'sent'])

export interface FolderOpenSyncInput {
  accountId: string | null | undefined
  folderId: string | null | undefined
  folderType: string | null | undefined
  folderSubscribed: boolean
  syncAllFolders: boolean
  syncFoldersEnabled: boolean
  isUnifiedView: boolean
  isOnline: boolean
  isSyncing: boolean
  lastAttemptAt: number | null | undefined
  now?: number
}

interface FolderSyncDescriptor {
  type: string
  subscribed: boolean
}

interface FolderTreeLike {
  folder?: {
    id: string
    type: string
    subscribed: boolean
  }
  children?: FolderTreeLike[]
}

export function findFolderSyncDescriptor(
  trees: FolderTreeLike[],
  folderId: string,
): FolderSyncDescriptor | null {
  for (const tree of trees) {
    if (tree.folder?.id === folderId) {
      return {
        type: tree.folder.type,
        subscribed: tree.folder.subscribed,
      }
    }
    if (tree.children) {
      const found = findFolderSyncDescriptor(tree.children, folderId)
      if (found) return found
    }
  }
  return null
}

export function shouldAutoSyncFolderOnOpen(input: FolderOpenSyncInput): boolean {
  if (
    !input.accountId ||
    !input.folderId ||
    !input.folderType ||
    input.isUnifiedView ||
    !input.isOnline ||
    input.isSyncing
  ) {
    return false
  }

  const now = input.now ?? Date.now()
  if (
    input.lastAttemptAt != null &&
    now - input.lastAttemptAt < FOLDER_OPEN_SYNC_RETRY_COOLDOWN_MS
  ) {
    return false
  }

  // Inbox is always synchronized by the account scheduler before any optional
  // subscribed/all-folder work, even if its IMAP subscription flag is absent.
  if (input.folderType === 'inbox') return false
  if (input.syncAllFolders) return false
  if (input.syncFoldersEnabled) return !input.folderSubscribed
  return !DEFAULT_BACKGROUND_FOLDER_TYPES.has(input.folderType)
}
