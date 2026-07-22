<script lang="ts">
  import Icon from '@iconify/svelte'
  import AccountSection from './AccountSection.svelte'
  import {
    buildFolderNavigationList,
    nextSidebarAction,
    nextSidebarNavigationIndex,
    type FolderNavItem,
    type SidebarAction,
  } from './folderNavigation'
  import AccountDialog from '$lib/components/settings/AccountDialog.svelte'
  import { Button } from '$lib/components/ui/button'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { isAccountExpanded, setAccountExpanded, setFolderCollapsed, getUIState, getUIStateVersion, saveUIState } from '$lib/stores/uiState.svelte'
  import { setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import { account, folder } from '../../../../wailsjs/go/models'

  // Track focused account header for keyboard navigation
  let focusedAccountId = $state<string | null>(null)
  // Up/Down enters or leaves this horizontal group; Left/Right moves inside it.
  let sidebarActionsFocused = $state(false)
  let selectedSidebarAction = $state<SidebarAction>('compose')
  // Directory-only IMAP nodes remain keyboard-focusable so users can expand
  // groups such as “Other folders” without changing the selected mailbox.
  let focusedFolderGroupAccountId = $state<string | null>(null)
  let focusedFolderGroupId = $state<string | null>(null)

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
    showBackButton = false,
    onBack,
  }: Props = $props()

  // Dialog state
  let showAccountDialog = $state(false)
  let editingAccount = $state<account.Account | null>(null)

  // Handle folder selection
  function handleFolderSelect(accountId: string, folderId: string, folderPath: string, folderName: string, folderType: string) {
    sidebarActionsFocused = false
    focusedAccountId = null
    focusedFolderGroupAccountId = null
    focusedFolderGroupId = null
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

  // Build the visible keyboard order, including top actions, account headers,
  // and directory-only groups that can be expanded but not selected as mailboxes.
  function buildFolderNavList(): FolderNavItem[] {
    return buildFolderNavigationList(accountStore.accounts, expandedAccounts, collapsedFolders)
  }

  // Get current folder index in navigation list
  function getCurrentFolderIndex(): number {
    const navList = buildFolderNavList()

    if (sidebarActionsFocused) {
      return navList.findIndex(item => item.type === 'sidebar-actions')
    }

    // Check if an account header is focused
    if (focusedAccountId) {
      return navList.findIndex(item =>
        item.type === 'account-header' && item.accountId === focusedAccountId
      )
    }

    if (focusedFolderGroupAccountId && focusedFolderGroupId) {
      return navList.findIndex(item =>
        item.type === 'folder-group'
        && item.accountId === focusedFolderGroupAccountId
        && item.folderId === focusedFolderGroupId
      )
    }

    // Find the selected folder item
    return navList.findIndex(item =>
      item.type === 'folder' && item.accountId === selectedAccountId && item.folderId === selectedFolderId
    )
  }

  // Navigate to the previous sidebar item and wrap at the top.
  export function selectPreviousFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = nextSidebarNavigationIndex(currentIndex, navList.length, -1)

    selectFolderByIndex(navList, newIndex)
  }

  // Navigate to the next sidebar item and wrap at the bottom.
  export function selectNextFolder() {
    const navList = buildFolderNavList()
    if (navList.length === 0) return

    const currentIndex = getCurrentFolderIndex()
    const newIndex = nextSidebarNavigationIndex(currentIndex, navList.length, 1)

    selectFolderByIndex(navList, newIndex)
  }

  // Scroll an item into view
  function scrollItemIntoView(item: FolderNavItem) {
    if (!scrollContainer) return

    // Build selector based on item type
    let selector: string | null = null
    if (item.type === 'account-header' && item.accountId) {
      selector = `[data-sidebar-item="account-header"][data-account-id="${item.accountId}"]`
    } else if ((item.type === 'folder' || item.type === 'folder-group') && item.accountId && item.folderId) {
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

    // Move the logical keyboard cursor; only selectable folders change mail.
    if (item.type === 'sidebar-actions') {
      sidebarActionsFocused = true
      focusedAccountId = null
      focusedFolderGroupAccountId = null
      focusedFolderGroupId = null
    } else if (item.type === 'account-header' && item.accountId) {
      // Focus on account header (Enter/Space will toggle expand)
      sidebarActionsFocused = false
      focusedAccountId = item.accountId
      focusedFolderGroupAccountId = null
      focusedFolderGroupId = null
    } else if (item.type === 'folder-group' && item.accountId && item.folderId) {
      // Keep the current mailbox selected while focusing a directory-only row.
      sidebarActionsFocused = false
      focusedAccountId = null
      focusedFolderGroupAccountId = item.accountId
      focusedFolderGroupId = item.folderId
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

  export function hasFocusedSidebarAction(): boolean {
    return sidebarActionsFocused
  }

  export function activateFocusedSidebarAction(): void {
    if (selectedSidebarAction === 'compose') {
      onCompose?.()
    } else if (selectedSidebarAction === 'sync') {
      void toggleSync()
    }
  }

  export function moveFocusedSidebarAction(direction: 1 | -1): void {
    if (!sidebarActionsFocused) return
    selectedSidebarAction = nextSidebarAction(selectedSidebarAction, direction)
  }

  function activateSidebarAction(action: SidebarAction): void {
    sidebarActionsFocused = true
    selectedSidebarAction = action
    focusedAccountId = null
    focusedFolderGroupAccountId = null
    focusedFolderGroupId = null
    activateFocusedSidebarAction()
  }

  // Check if an account header is focused
  export function hasFocusedAccount(): boolean {
    return focusedAccountId !== null
  }

  export function toggleFocusedFolderGroup(): void {
    if (
      focusedFolderGroupAccountId
      && focusedFolderGroupId
      && folderHasChildren(focusedFolderGroupAccountId, focusedFolderGroupId)
    ) {
      toggleFolderCollapsed(focusedFolderGroupId)
    }
  }

  export function hasFocusedFolderGroup(): boolean {
    return focusedFolderGroupId !== null
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
    sidebarActionsFocused = false
    focusedAccountId = null
    focusedFolderGroupAccountId = null
    focusedFolderGroupId = null
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

<div class="flex flex-col h-full">
  <!-- Header with Compose Button -->
  <div class="px-4 py-3 border-b border-border">
    <div class="flex items-center gap-2">
      <button
        class="flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-md text-sm font-medium transition-colors {!sidebarActionsFocused || selectedSidebarAction === 'compose' ? 'bg-primary text-primary-foreground hover:bg-primary/90' : 'bg-primary/10 text-primary hover:bg-primary/20'}"
        type="button"
        tabindex="-1"
        data-sidebar-item="sidebar-action"
        data-sidebar-action="compose"
        data-keyboard-selected={sidebarActionsFocused && selectedSidebarAction === 'compose'}
        onclick={() => activateSidebarAction('compose')}
      >
        <Icon icon="mdi:email-edit-outline" class="w-4 h-4" />
        <span>{$_('sidebar.compose')}</span>
      </button>
      <button
        class="h-9 w-9 flex items-center justify-center rounded-md transition-colors flex-shrink-0 {sidebarActionsFocused && selectedSidebarAction === 'sync' ? 'bg-primary text-primary-foreground hover:bg-primary/90' : 'text-muted-foreground hover:text-foreground hover:bg-muted'}"
        type="button"
        tabindex="-1"
        data-sidebar-item="sidebar-action"
        data-sidebar-action="sync"
        data-keyboard-selected={sidebarActionsFocused && selectedSidebarAction === 'sync'}
        title={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
        aria-label={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
        onclick={() => activateSidebarAction('sync')}
      >
        <Icon
          icon="mdi:refresh"
          class="w-5 h-5 {accountStore.isAnySyncing ? `animate-spin ${sidebarActionsFocused && selectedSidebarAction === 'sync' ? 'text-primary-foreground' : 'text-primary'}` : ''}"
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
          showFolderSelection={!sidebarActionsFocused && focusedAccountId === null && focusedFolderGroupId === null}
          {focusedFolderGroupAccountId}
          {focusedFolderGroupId}
          isExpanded={expandedAccounts[accWithFolders.account.id] ?? true}
          {collapsedFolders}
          {onMessagesMoved}
          onFolderSelect={handleFolderSelect}
          onToggleExpanded={() => {
            sidebarActionsFocused = false
            focusedAccountId = accWithFolders.account.id
            focusedFolderGroupAccountId = null
            focusedFolderGroupId = null
            toggleAccountExpanded(accWithFolders.account.id)
          }}
          onToggleFolderCollapse={(folderId, directoryOnly) => {
            sidebarActionsFocused = false
            if (directoryOnly) {
              focusedAccountId = null
              focusedFolderGroupAccountId = accWithFolders.account.id
              focusedFolderGroupId = folderId
            } else {
              focusedFolderGroupAccountId = null
              focusedFolderGroupId = null
            }
            toggleFolderCollapsed(folderId)
          }}
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
