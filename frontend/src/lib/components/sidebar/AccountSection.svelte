<script lang="ts">
  import Icon from '@iconify/svelte'
  // @ts-ignore - wailsjs path
  import { account, folder } from '../../../../wailsjs/go/models'
  import FolderTreeItem from './FolderTreeItem.svelte'
  import { _ } from '$lib/i18n'
  import { getAccountColor } from '$lib/utils/accountColor'

  interface Props {
    account: account.Account
    folders: folder.FolderTree[]
    loading: boolean
    syncing: boolean
    error: string | null
    selectedAccountId: string
    selectedFolderId: string
    selectionSource: 'unified' | 'account' | null
    isHeaderFocused?: boolean
    showFolderSelection?: boolean
    focusedFolderGroupAccountId?: string | null
    focusedFolderGroupId?: string | null
    isExpanded?: boolean
    onFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onToggleExpanded?: () => void
    collapsedFolders?: Record<string, boolean>
    onToggleFolderCollapse?: (folderId: string, directoryOnly: boolean) => void
    onMessagesMoved?: () => void
  }

  let {
    account: acc,
    folders,
    loading,
    syncing,
    error,
    selectedAccountId,
    selectedFolderId,
    selectionSource,
    isHeaderFocused = false,
    showFolderSelection = true,
    focusedFolderGroupAccountId = null,
    focusedFolderGroupId = null,
    isExpanded = true,
    onFolderSelect,
    onToggleExpanded,
    collapsedFolders = {},
    onToggleFolderCollapse,
    onMessagesMoved,
  }: Props = $props()

  // Toggle expand/collapse via callback
  function toggleExpanded() {
    onToggleExpanded?.()
  }

  function selectFolder(f: folder.Folder) {
    onFolderSelect?.(acc.id, f.id, f.path, f.name, f.type)
  }

  function sumAccountBadgeCounts(trees: folder.FolderTree[]): { unread: number; drafts: number } {
    const totals = { unread: 0, drafts: 0 }
    for (const tree of trees) {
      if (tree.folder?.type === 'drafts') {
        totals.drafts += tree.folder.totalCount || 0
      } else {
        totals.unread += tree.folder?.unreadCount || 0
      }
      if (tree.children?.length) {
        const childTotals = sumAccountBadgeCounts(tree.children)
        totals.unread += childTotals.unread
        totals.drafts += childTotals.drafts
      }
    }
    return totals
  }

  let accountBadgeCounts = $derived(sumAccountBadgeCounts(folders))
  let accountBadgeTotal = $derived(accountBadgeCounts.unread + accountBadgeCounts.drafts)
  let accountColor = $derived(getAccountColor(acc))
</script>

<div class="mb-1">
  <!-- Account Header. Edit / delete / sync live in Settings → Accounts, so the
       sidebar header is just an expand/collapse toggle. -->
  <div>
    <button
      class="w-full flex items-center gap-2 px-3 py-2 text-sm font-medium transition-colors focus:outline-none {isHeaderFocused ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-muted/50'}"
      tabindex="-1"
      data-sidebar-item="account-header"
      data-account-id={acc.id}
      type="button"
      aria-expanded={isExpanded}
      aria-label={isExpanded ? $_('sidebar.collapseAccount', { values: { account: acc.name || acc.email } }) : $_('sidebar.expandAccount', { values: { account: acc.name || acc.email } })}
      onclick={toggleExpanded}
    >
      <span
        class="w-2 h-2 rounded-full flex-shrink-0"
        style="background-color: {accountColor}"
        aria-hidden="true"
      ></span>
      <span class="truncate flex-1 text-left">{acc.name || acc.email}</span>

      {#if syncing}
        <Icon icon="mdi:sync" class="w-4 h-4 animate-spin text-muted-foreground" />
      {:else if error}
        <span title={error}>
          <Icon icon="mdi:alert-circle" class="w-4 h-4 text-destructive" />
        </span>
      {:else if accountBadgeTotal > 0}
        <span
          class="px-1.5 py-0.5 text-xs font-medium rounded-full bg-primary text-primary-foreground"
        >
          {accountBadgeTotal}
        </span>
      {/if}
    </button>
  </div>

  <!-- Sync errors are recorded in Settings → Activity Log. The header spinner
       and bottom status still convey live sync state. -->

  <!-- Folder List -->
  {#if isExpanded}
    <div class="ml-4">
      {#if loading}
        <div class="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground">
          <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          <span>{$_('sidebar.loadingFolders')}</span>
        </div>
      {:else if folders.length === 0}
        <div class="px-3 py-2 text-sm text-muted-foreground">
          {$_('sidebar.noFoldersSynced')}
        </div>
      {:else}
        {#each folders as tree (tree.folder?.id ?? 'unknown')}
          <FolderTreeItem
            {tree}
            accountId={acc.id}
            {selectedAccountId}
            {selectedFolderId}
            {selectionSource}
            {showFolderSelection}
            {focusedFolderGroupAccountId}
            {focusedFolderGroupId}
            {collapsedFolders}
            {onMessagesMoved}
            onFolderSelect={(f) => selectFolder(f)}
            onToggleCollapse={(folderId, directoryOnly) => onToggleFolderCollapse?.(folderId, directoryOnly)}
          />
        {/each}
      {/if}
    </div>
  {/if}
</div>
