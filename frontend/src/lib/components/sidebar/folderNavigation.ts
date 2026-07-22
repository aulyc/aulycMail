interface FolderTreeLike {
  folder?: {
    id: string
    name: string
    path: string
    type: string
    noSelect: boolean
  }
  children?: FolderTreeLike[]
}

interface AccountFoldersLike {
  account?: {
    id: string
    name: string
  }
  folders?: FolderTreeLike[]
}

export type SidebarAction = 'compose' | 'sync'

export interface FolderNavItem {
  type: 'sidebar-actions' | 'account-header' | 'folder-group' | 'folder'
  accountId?: string
  folderId?: string
  folderPath?: string
  folderName: string
  folderType?: string
}

/** Build the keyboard order to match every visible sidebar tree row. */
export function buildFolderNavigationList(
  accounts: readonly AccountFoldersLike[],
  expandedAccounts: Readonly<Record<string, boolean>>,
  collapsedFolders: Readonly<Record<string, boolean>>,
): FolderNavItem[] {
  const items: FolderNavItem[] = [
    { type: 'sidebar-actions', folderName: 'sidebar-actions' },
  ]

  const flattenFolders = (accountId: string, trees: readonly FolderTreeLike[]) => {
    for (const tree of trees) {
      const currentFolder = tree.folder
      if (!currentFolder) continue

      items.push({
        type: currentFolder.noSelect ? 'folder-group' : 'folder',
        accountId,
        folderId: currentFolder.id,
        folderPath: currentFolder.path,
        folderName: currentFolder.name,
        folderType: currentFolder.type,
      })

      if (
        tree.children?.length
        && collapsedFolders[currentFolder.id] === false
      ) {
        flattenFolders(accountId, tree.children)
      }
    }
  }

  for (const accountWithFolders of accounts) {
    const currentAccount = accountWithFolders.account
    if (!currentAccount) continue

    items.push({
      type: 'account-header',
      accountId: currentAccount.id,
      folderName: currentAccount.name,
    })

    if (expandedAccounts[currentAccount.id]) {
      flattenFolders(currentAccount.id, accountWithFolders.folders ?? [])
    }
  }

  return items
}

/** Move one step through the sidebar and wrap at both boundaries. */
export function nextSidebarNavigationIndex(
  currentIndex: number,
  itemCount: number,
  direction: 1 | -1,
): number {
  if (itemCount <= 0) return -1
  if (currentIndex < 0 || currentIndex >= itemCount) {
    return direction === 1 ? 0 : itemCount - 1
  }
  return (currentIndex + direction + itemCount) % itemCount
}

/** Move within the horizontal Compose/Sync action group and wrap. */
export function nextSidebarAction(
  current: SidebarAction,
  direction: 1 | -1,
): SidebarAction {
  const actions: readonly SidebarAction[] = ['compose', 'sync']
  const currentIndex = actions.indexOf(current)
  return actions[(currentIndex + direction + actions.length) % actions.length]
}
