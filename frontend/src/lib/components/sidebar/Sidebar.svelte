<script lang="ts">
  import Icon from '@iconify/svelte'
  import { onMount } from 'svelte'
  import AccountSection from './AccountSection.svelte'
  import AccountDialog from '$lib/components/settings/AccountDialog.svelte'
  import { Button } from '$lib/components/ui/button'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { isAccountExpanded, setAccountExpanded, setFolderCollapsed, getUIState, getUIStateVersion, saveUIState } from '$lib/stores/uiState.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account, folder } from '../../../../wailsjs/go/models'

  // Folder item type for flat navigation list
  interface FolderNavItem {
    type: 'account-header' | 'folder'
    accountId?: string
    folderId?: string
    folderPath?: string
    folderName: string
    folderType?: string
  }

  // Track focused account header for keyboard navigation
  let focusedAccountId = $state<string | null>(null)

  // Ref to scrollable container for auto-scroll
  let scrollContainer: HTMLDivElement | null = null

  // Track expanded state for each account (reactive, synced with persisted state)
  let expandedAccounts = $state<Record<string, boolean>>({})

  // Initialize expanded state from persisted storage
  // Depends on both accounts list AND UI state version (so it re-runs when persisted state loads)
  $effect(() => {
    // Read version to create dependency - effect re-runs when UI state finishes loading
    const _version = getUIStateVersion()

    const newExpanded: Record<string, boolean> = {}
    for (const acc of accountStore.accounts) {
      newExpanded[acc.account.id] = isAccountExpanded(acc.account.id)
    }
    expandedAccounts = newExpanded
  })

  // Toggle account expansion
  function toggleAccountExpanded(accountId: string) {
    const newValue = !expandedAccounts[accountId]
    expandedAccounts[accountId] = newValue
    setAccountExpanded(accountId, newValue)
  }

  // Track collapsed state for folders with children (reactive, synced with persisted state)
  let collapsedFolders = $state<Record<string, boolean>>({})

  // Initialize collapsed state from persisted storage and prune stale entries
  $effect(() => {
    const _version = getUIStateVersion()

    // Collect all folder IDs that exist in current accounts
    const allFolderIds = new Set<string>()
    const collectIds = (trees: folder.FolderTree[]) => {
      for (const tree of trees) {
        if (tree.folder) allFolderIds.add(tree.folder.id)
        if (tree.children) collectIds(tree.children)
      }
    }
    for (const acc of accountStore.accounts) {
      collectIds(acc.folders || [])
    }

    // Read persisted collapsed state, keep only entries for existing folders
    const persisted = getUIState().collapsedFolders
    const newCollapsed: Record<string, boolean> = {}
    let hasStale = false
    for (const folderId of Object.keys(persisted)) {
      if (allFolderIds.has(folderId)) {
        newCollapsed[folderId] = persisted[folderId]
      } else {
        hasStale = true
      }
    }
    collapsedFolders = newCollapsed

    // Persist cleaned state if stale entries were pruned
    if (hasStale) {
      saveUIState({ collapsedFolders: newCollapsed })
    }
  })

  // Toggle folder collapse
  function toggleFolderCollapsed(folderId: string) {
    const isCurrentlyCollapsed = collapsedFolders[folderId] !== false
    const newValue = !isCurrentlyCollapsed
    collapsedFolders = { ...collapsedFolders, [folderId]: newValue }
    setFolderCollapsed(folderId, newValue)
  }

  interface Props {
    onFolderSelect?: (accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) => void
    onCompose?: () => void
    onMessagesMoved?: () => void
    selectedAccountId?: string | null
    selectedFolderId?: string | null
    selectionSource?: 'account' | null
    isFocused?: boolean
    isFlashing?: boolean
    showBackButton?: boolean
    onBack?: () => void
  }

  let {
    onFolderSelect,
    onCompose,
    onMessagesMoved,
    selectedAccountId = null,
    selectedFolderId = null,
    selectionSource = null,
    isFocused: _isFocused = false,
    isFlashing = false,
    showBackButton = false,
    onBack,
  }: Props = $props()

  // Dialog state
  let showAccountDialog = $state(false)
  let editingAccount = $state<account.Account | null>(null)

  // Load accounts on mount
  onMount(() => {
    // Load accounts, then trigger comprehensive sync on launch
    accountStore.load().then(async () => {
      try {
        await accountStore.syncAllComplete()
      } catch (err) {
        console.error('Failed to sync on launch:', err)
      }
    })
  })

  // Handle folder selection
  function handleFolderSelect(accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) {
    accountStore.selectFolder(accountId, folderId, folderPath, folderName)
    onFolderSelect?.(accountId, folderId, folderPath, folderName, folderType)
  }

  // Open add account dialog
  function openAddAccount() {
    editingAccount = null
    showAccountDialog = true
  }

  // Sync all accounts (comprehensive sync)
  export async function syncAllAccounts() {
    try {
      await accountStore.syncAllComplete()
    } catch (err) {
      console.error('Sync failed:', err)
      // Error is already stored in account store
    }
  }

  // Cancel all running syncs
  export async function cancelSync() {
    try {
      await accountStore.cancelAllSyncs()
    } catch (err) {
      console.error('Failed to cancel sync:', err)
    }
  }

  // Toggle sync (start if not running, cancel if running) - for keyboard shortcut
  export async function toggleSync() {
    if (accountStore.isAnySyncing) {
      await cancelSync()
    } else {
      await syncAllAccounts()
    }
  }

  // Build flat list of all navigable folders including Unified Inbox
  // The list matches the exact visual order in the sidebar, respecting expanded/collapsed state
  function buildFolderNavList(): FolderNavItem[] {
    const items: FolderNavItem[] = []

    // Add account headers and their folders
    for (const accWithFolders of accountStore.accounts) {
      // Skip if account is not fully loaded yet (can happen during reauth)
      if (!accWithFolders.account) continue

      // Always add the account header (so user can navigate to it and expand)
      items.push({
        type: 'account-header',
        accountId: accWithFolders.account.id,
        folderName: accWithFolders.account.name,
      })

      // Only add folders if the account is expanded
      if (expandedAccounts[accWithFolders.account.id]) {
        const flattenFolders = (trees: folder.FolderTree[]) => {
          for (const tree of trees) {
            if (tree.folder) {
              items.push({
                type: 'folder',
                accountId: accWithFolders.account.id,
                folderId: tree.folder.id,
                folderPath: tree.folder.path,
                folderName: tree.folder.name,
                folderType: tree.folder.type,
              })
            }
            // Skip children of collapsed folders
            if (tree.children && tree.children.length > 0 && tree.folder && collapsedFolders[tree.folder.id] === false) {
              flattenFolders(tree.children)
            }
          }
        }
        flattenFolders(accWithFolders.folders || [])
      }
    }

    return items
  }

  // Get current folder index in navigation list
  function getCurrentFolderIndex(): number {
    const navList = buildFolderNavList()

    // Check if an account header is focused
    if (focusedAccountId) {
      return navList.findIndex(item =>
        item.type === 'account-header' && item.accountId === focusedAccountId
      )
    }

    // Find the selected folder item
    return navList.findIndex(item =>
      item.type === 'folder' && item.accountId === selectedAccountId && item.folderId === selectedFolderId
    )
  }

  // Navigate to previous folder (exposed for keyboard navigation)
  export function selectPreviousFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex <= 0 ? 0 : currentIndex - 1

    selectFolderByIndex(navList, newIndex)
  }

  // Navigate to next folder (exposed for keyboard navigation)
  export function selectNextFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = currentIndex >= navList.length - 1 ? navList.length - 1 : currentIndex + 1

    selectFolderByIndex(navList, newIndex)
  }

  // Scroll an item into view
  function scrollItemIntoView(item: FolderNavItem) {
    if (!scrollContainer) return

    // Build selector based on item type
    let selector: string | null = null
    if (item.type === 'account-header' && item.accountId) {
      selector = `[data-sidebar-item="account-header"][data-account-id="${item.accountId}"]`
    } else if (item.type === 'folder' && item.accountId && item.folderId) {
      selector = `[data-sidebar-item="folder"][data-account-id="${item.accountId}"][data-folder-id="${item.folderId}"]`
    }

    if (selector) {
      const element = scrollContainer.querySelector(selector)
      element?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }

  // Select folder by index in nav list
  function selectFolderByIndex(navList: FolderNavItem[], index: number) {
    const item = navList[index]
    if (!item) return

    // Clear account header focus when selecting a folder
    if (item.type !== 'account-header') {
      focusedAccountId = null
    }

    if (item.type === 'account-header' && item.accountId) {
      // Focus on account header (Enter/Space will toggle expand)
      focusedAccountId = item.accountId
    } else if (item.type === 'folder' && item.accountId && item.folderId && item.folderPath) {
      // Select from account tree - uses handleFolderSelect
      handleFolderSelect(item.accountId, item.folderId, item.folderPath, item.folderName, item.folderType || 'folder')
    }

    // Scroll the selected item into view
    scrollItemIntoView(item)
  }

  // Toggle expand/collapse for the focused account (called on Enter/Space/Alt+Enter)
  export function toggleFocusedAccount() {
    if (focusedAccountId) {
      toggleAccountExpanded(focusedAccountId)
    }
  }

  // Check if an account header is focused
  export function hasFocusedAccount(): boolean {
    return focusedAccountId !== null
  }

  // Check if the currently selected folder has children
  export function hasSelectedFolderWithChildren(): boolean {
    if (!selectedAccountId || !selectedFolderId || selectionSource !== 'account') return false
    return folderHasChildren(selectedAccountId, selectedFolderId)
  }

  // Toggle collapse for the currently selected folder
  export function toggleSelectedFolderCollapse(): void {
    if (!selectedAccountId || !selectedFolderId || selectionSource !== 'account') return
    if (!folderHasChildren(selectedAccountId, selectedFolderId)) return
    toggleFolderCollapsed(selectedFolderId)
  }

  // Check if a folder has children by searching the account folder trees
  function folderHasChildren(accountId: string, folderId: string): boolean {
    const acc = accountStore.accounts.find((item) => item.account.id === accountId)
    const found = acc ? findTreeNode(acc.folders || [], folderId) : null
    return (found?.children?.length ?? 0) > 0
  }

  // Find a FolderTree node by folder ID
  function findTreeNode(trees: folder.FolderTree[], folderId: string): folder.FolderTree | null {
    for (const tree of trees) {
      if (tree.folder?.id === folderId) return tree
      if (tree.children) {
        const found = findTreeNode(tree.children, folderId)
        if (found) return found
      }
    }
    return null
  }

  function findAncestorFolderIds(trees: folder.FolderTree[], folderId: string, ancestors: string[] = []): string[] | null {
    for (const tree of trees) {
      if (!tree.folder) continue
      if (tree.folder.id === folderId) return ancestors
      const found = findAncestorFolderIds(tree.children || [], folderId, [...ancestors, tree.folder.id])
      if (found) return found
    }
    return null
  }

  export function revealFolder(accountId: string, folderId: string) {
    if (!accountId || !folderId) return
    focusedAccountId = null
    expandedAccounts = { ...expandedAccounts, [accountId]: true }
    setAccountExpanded(accountId, true)

    const acc = accountStore.accounts.find((item) => item.account.id === accountId)
    const ancestors = acc ? findAncestorFolderIds(acc.folders || [], folderId) : null
    if (ancestors?.length) {
      const nextCollapsed = { ...collapsedFolders }
      for (const ancestorId of ancestors) {
        nextCollapsed[ancestorId] = false
        setFolderCollapsed(ancestorId, false)
      }
      collapsedFolders = nextCollapsed
    }

    requestAnimationFrame(() => scrollItemIntoView({
      type: 'folder',
      accountId,
      folderId,
      folderName: '',
    }))
  }
</script>

<div class="flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <!-- Header with Compose Button -->
  <div class="px-4 py-3 border-b border-border">
    <div class="flex items-center gap-2">
      <button
        class="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 transition-colors"
        type="button"
        onclick={onCompose}
      >
        <Icon icon="mdi:email-edit-outline" class="w-4 h-4" />
        <span>{$_('sidebar.compose')}</span>
      </button>
      <button
        class="h-9 w-9 flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors flex-shrink-0 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
        type="button"
        title={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
        aria-label={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
        onclick={toggleSync}
      >
        <Icon
          icon="mdi:sync"
          class="w-5 h-5 {accountStore.isAnySyncing ? 'animate-spin text-primary' : ''}"
        />
      </button>
      {#if showBackButton}
        <button
          class="p-2 rounded-md hover:bg-muted transition-colors flex-shrink-0"
          title={$_('responsive.back')}
          aria-label={$_('aria.closeSidebar')}
          onclick={onBack}
        >
          <Icon icon="mdi:close" class="w-5 h-5 text-muted-foreground" />
        </button>
      {/if}
    </div>
  </div>

  <!-- Account List -->
  <div class="flex-1 overflow-y-auto scrollbar-thin py-2" bind:this={scrollContainer}>
    {#if accountStore.loading}
      <div class="flex items-center justify-center py-8">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else if accountStore.accounts.length === 0}
      <!-- Empty State -->
      <div class="flex flex-col items-center justify-center py-8 px-4 text-center">
        <Icon icon="mdi:email-plus-outline" class="w-12 h-12 text-muted-foreground mb-3" />
        <h3 class="text-sm font-medium mb-1">{$_('sidebar.noAccountsYet')}</h3>
        <p class="text-xs text-muted-foreground mb-4">
          {$_('sidebar.addFirstAccount')}
        </p>
        <Button size="sm" onclick={openAddAccount}>
          <Icon icon="mdi:plus" class="w-4 h-4 mr-1" />
          {$_('sidebar.addAccount')}
        </Button>
      </div>
    {:else}
      {#each accountStore.accounts as accWithFolders (accWithFolders.account.id)}
        <AccountSection
          account={accWithFolders.account}
          folders={accWithFolders.folders}
          loading={accWithFolders.loading}
          syncing={accWithFolders.syncing}
          error={accWithFolders.error}
          selectedAccountId={accountStore.selectedFolder?.accountId ?? selectedAccountId ?? ''}
          selectedFolderId={accountStore.selectedFolder?.folderId ?? selectedFolderId ?? ''}
          {selectionSource}
          isHeaderFocused={focusedAccountId === accWithFolders.account.id}
          isExpanded={expandedAccounts[accWithFolders.account.id] ?? true}
          {collapsedFolders}
          {onMessagesMoved}
          onFolderSelect={handleFolderSelect}
          onToggleExpanded={() => toggleAccountExpanded(accWithFolders.account.id)}
          onToggleFolderCollapse={toggleFolderCollapsed}
        />
      {/each}
    {/if}
  </div>

</div>

<!-- Account Dialog -->
<AccountDialog
  bind:open={showAccountDialog}
  editAccount={editingAccount}
  onClose={() => {
    showAccountDialog = false
    editingAccount = null
    setFocusedPane('messageList')
  }}
/>
