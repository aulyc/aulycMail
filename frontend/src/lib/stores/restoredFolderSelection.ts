export interface RestoredFolderInfo {
  noSelect: boolean
}

export function shouldClearRestoredFolderSelection(
  accountExists: boolean,
  foldersLoaded: boolean,
  folder: RestoredFolderInfo | null,
): boolean {
  if (!foldersLoaded) return false
  return !accountExists || folder === null || folder.noSelect
}
