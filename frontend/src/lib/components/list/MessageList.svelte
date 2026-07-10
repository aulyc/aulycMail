<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import ConversationRow from './ConversationRow.svelte'
  import { DropdownMenu } from 'bits-ui'
  import { cn } from '$lib/utils'
  import { Button } from '$lib/components/ui/button'
  // @ts-ignore - wailsjs bindings
  import { GetConversations, GetConversationCount, SyncFolder, ForceSyncFolder, CancelFolderSync, SetMessageListSortOrder, GetUnifiedInboxConversations, GetUnifiedInboxCount, GetFTSIndexStatus, IsFTSIndexing, Trash, DeletePermanently, EmptyTrash, Undo, FetchServerMessage } from '../../../../wailsjs/go/app/App'
  import { toasts } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  import { ConfirmDialog } from '$lib/components/ui/confirm-dialog'
  import MessageContextMenu from '$lib/components/common/MessageContextMenu.svelte'
  // @ts-ignore - wailsjs path
  import { message } from '../../../../wailsjs/go/models'
  // @ts-ignore - wailsjs runtime
  import { EventsOn } from '../../../../wailsjs/runtime/runtime'
  import { getMessageListDensity, getMessageListSortOrder, setMessageListSortOrder } from '$lib/stores/settings.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getLayoutMode, hideViewer } from '$lib/stores/layout.svelte'
  import { isDialogGuardActive, onDialogGuardChange } from '$lib/stores/dialogGuard'
  import { createDebouncer } from '$lib/utils/debounce'
  import {
    loadMoreLocalMessageListSearch,
    searchLocalMessageList,
    searchServerMessageList,
  } from './messageListSearch'
  import {
    getMessageListRowHeight,
    getVirtualWindow as calculateVirtualWindow,
    type VirtualWindow,
  } from './messageListVirtual'
  import {
    createMultiRowContextMenu,
    createSingleRowContextMenu,
    getSelectedMessageIds as collectSelectedMessageIds,
    hasSelectedUnread,
    hasSelectedUnstarred,
    toggleSetEntry,
    type RowContextMenuState,
  } from './messageListSelection'

  interface Props {
    accountId?: string | null
    folderId?: string | null
    folderName?: string
    folderType?: string
    onConversationSelect?: (threadId: string, folderId: string, accountId: string) => void
    /** Called when the loaded folder has no conversations (so the parent can clear the viewer). */
    onEmptyFolder?: () => void
    onReply?: (mode: 'reply' | 'reply-all' | 'forward', messageId: string) => void
    onRowActionComplete?: () => void
    /** Double-click a draft row → open it in the composer (drafts folder only). */
    onOpenDraft?: (messageId: string) => void
    onSearch?: () => void
    isFocused?: boolean
    isFlashing?: boolean
    showFolderToggle?: boolean
    onToggleSidebar?: () => void
  }

  let {
    accountId = null,
    folderId = null,
    folderName = 'Inbox',
    folderType = 'inbox',
    onConversationSelect,
    onEmptyFolder,
    onReply,
    onRowActionComplete,
    onOpenDraft,
    onSearch,
    isFocused: _isFocused = false,
    isFlashing = false,
    showFolderToggle = false,
    onToggleSidebar,
  }: Props = $props()

  // State
  let conversations = $state<message.Conversation[]>([])
  let totalCount = $state(0)
  let loading = $state(false)
  let error = $state<string | null>(null)
  let selectedThreadId = $state<string | null>(null)
  let lastLoadedFolderId = $state<string | null>(null) // Track folder changes
  let loadGeneration = $state(0) // Invalidates stale async results when folder changes mid-load (#200)

  // Derived: check if this folder is currently syncing (from account store's progress tracking)
  const syncing = $derived(
    !!(accountId && folderId && accountStore.syncProgress[accountId]?.[folderId] !== undefined)
  )

  // Derived: get sync progress for this folder (if syncing)
  const syncProgress = $derived(
    accountId && folderId
      ? accountStore.syncProgress[accountId]?.[folderId]
      : null
  )

  // Multi-select state
  let checkedThreadIds = $state<Set<string>>(new Set())
  let lastClickedIndex = $state<number | null>(null)

  // Pagination
  const PAGE_SIZE = 50
  let offset = $state(0)

  // Debounce timer for reloading after flag changes
  let reloadTimer: ReturnType<typeof setTimeout> | null = null

  // Debounce timer for coalescing sync event reloads (fixes event flooding with 3+ accounts)
  let syncReloadTimer: ReturnType<typeof setTimeout> | null = null

  // Deferred reload: when a dialog (e.g. folder picker) is open, defer the reload
  // so the component tree isn't destroyed mid-interaction
  let pendingReload = false
  let eventUnsubscribers: Array<() => void> = []

  // Buffer for flag changes that arrive while loadConversations() is in-flight.
  // On notification click, loadConversations (folder change) and MarkAsRead race —
  // the flagsChanged event may fire before the new conversations array is ready.
  let pendingFlagChanges: Array<{messageIds: string[], isRead: boolean}> = []

  // Search state
  let showSearch = $state(false)
  let searchQuery = $state('')
  let searchResults = $state<any[]>([])  // ConversationSearchResult from backend
  let searchTotalCount = $state(0)
  let searchOffset = $state(0)
  let isSearching = $state(false)
  const searchDebouncer = createDebouncer(300)

  // Filter state
  let filterMode = $state<string>('')  // '' | 'unread' | 'starred' | 'attachments'

  const filterLabel = $derived((() => {
    switch (filterMode) {
      case 'unread': return $_('messageList.filterUnread')
      case 'starred': return $_('messageList.filterStarred')
      case 'attachments': return $_('messageList.filterAttachments')
      default: return ''
    }
  })())

  const filterOptions = $derived([
    { value: '', label: $_('messageList.filterAll') },
    { value: 'unread', label: $_('messageList.filterUnread'), separator: true },
    { value: 'starred', label: $_('messageList.filterStarred') },
    { value: 'attachments', label: $_('messageList.filterAttachments') },
  ])

  // Server search state
  let serverSearchMode = $state(false)
  let serverSearchResults = $state<any[]>([])
  let serverSearchCount = $state(0)
  let serverSearchTotalCount = $state(0)  // Total matching UIDs on server (may exceed serverSearchCount when limited)
  let isServerSearching = $state(false)
  let lastServerQuery = $state('')
  const SERVER_SEARCH_LIMIT = 200

  // FTS indexing state
  let indexProgress = $state(0)
  let indexComplete = $state(true)
  let isIndexing = $state(false)
  let searchInputRef = $state<HTMLInputElement | null>(null)

  // Check if a folder is an inbox by looking it up in the account store
  function isInboxFolder(acctId: string, fldId: string): boolean {
    const acct = accountStore.accounts.find(a => a.account.id === acctId)
    if (!acct) return false
    for (const tree of acct.folders) {
      if (tree.folder?.id === fldId) return tree.folder.type === 'inbox'
      for (const child of tree.children || []) {
        if (child.folder?.id === fldId) return child.folder.type === 'inbox'
      }
    }
    return false
  }

  // Schedule a debounced reload — coalesces rapid sync events from multiple accounts
  // into a single loadConversations() call after they settle (300ms).
  // Defers if a dialog guard is active (e.g. folder picker open).
  function scheduleReload() {
    if (isDialogGuardActive()) {
      pendingReload = true
      return
    }
    if (syncReloadTimer) clearTimeout(syncReloadTimer)
    syncReloadTimer = setTimeout(() => {
      syncReloadTimer = null
      if (loading) {
        pendingReload = true
        return
      }
      offset = 0
      loadConversations()
    }, 300)
  }

  // Listen for folder sync events from backend
  onMount(() => {
    eventUnsubscribers = [
      EventsOn('folder:synced', (data: { accountId: string; folderId: string }) => {
        // Reload if this is the current folder, or unified inbox when an inbox folder synced
        if ((isUnifiedView && isInboxFolder(data.accountId, data.folderId)) || (!isUnifiedView && accountId && folderId && data.accountId === accountId && data.folderId === folderId)) {
          scheduleReload()
        }
      }),

      // Listen for messages:updated events (e.g., from IDLE push notifications)
      EventsOn('messages:updated', (data: { accountId: string; folderId: string }) => {
        // Reload if this is the current folder, or unified inbox when an inbox folder updated
        if ((isUnifiedView && isInboxFolder(data.accountId, data.folderId)) || (!isUnifiedView && accountId && folderId && data.accountId === accountId && data.folderId === folderId)) {
          scheduleReload()
        }
      }),

      // Listen for message read-state changes.
      EventsOn('messages:readChanged', (data: { messageIds: string[], isRead: boolean }) => {
        // Update conversations locally instead of reloading from DB
        let anyUpdated = false
        for (const c of conversations) {
          const affectedCount = (c.messageIds || []).filter(id => data.messageIds.includes(id)).length
          if (affectedCount > 0) {
            anyUpdated = true
            const delta = data.isRead ? -affectedCount : affectedCount
            c.unreadCount = Math.max(0, (c.unreadCount || 0) + delta)
          }
        }
        if (anyUpdated) {
          conversations = conversations
          return
        }
        // loadConversations() is in-flight — the new array isn't ready yet.
        // Buffer this change so we can apply it after the load completes.
        if (loading) {
          pendingFlagChanges.push({ messageIds: data.messageIds, isRead: data.isRead })
        }
      }),

      // Listen for FTS indexing progress
      EventsOn('fts:progress', (data: { folderId: string; indexed: number; total: number; percentage: number }) => {
        if (folderId && data.folderId === folderId) {
          indexProgress = data.percentage
          indexComplete = false
          isIndexing = true
        }
      }),

      // Listen for FTS indexing completion
      EventsOn('fts:complete', (data: { folderId: string }) => {
        if (folderId && data.folderId === folderId) {
          indexComplete = true
          isIndexing = false
          indexProgress = 100
        }
      }),

      // Listen for FTS indexing status changes
      EventsOn('fts:indexing', (data: { status: string }) => {
        switch (data.status) {
          case 'completed':
            indexComplete = true
            isIndexing = false
            break
          case 'started':
            isIndexing = true
            break
        }
      }),
    ]

    // Check initial FTS index status for current folder
    checkFTSIndexStatus()

    eventUnsubscribers.push(
      onDialogGuardChange((active) => {
        if (pendingReload && !active) {
          pendingReload = false
          scheduleReload()
        }
      })
    )
  })

  onDestroy(() => {
    eventUnsubscribers.forEach(unsubscribe => unsubscribe())
    eventUnsubscribers = []
    if (reloadTimer) clearTimeout(reloadTimer)
    if (syncReloadTimer) clearTimeout(syncReloadTimer)
    searchDebouncer.cancel()
  })
  // Check FTS index status for current folder
  async function checkFTSIndexStatus() {
    if (!folderId) return
    try {
      const status = await GetFTSIndexStatus(folderId)
      if (status) {
        indexComplete = status.isComplete
        if (status.totalCount > 0) {
          indexProgress = Math.round((status.indexedCount / status.totalCount) * 100)
        }
      }
      isIndexing = await IsFTSIndexing()
    } catch (err) {
      console.error('Failed to check FTS index status:', err)
    }
  }

  // Track previous folder to detect actual changes
  let prevAccountId: string | null = null
  let prevFolderId: string | null = null

  // True only for a load that was triggered by genuine folder navigation (the
  // folder-change $effect below). Background/deferred reloads (sync events, the
  // dialog-guard flush after a dialog closes, sort/filter) leave it false so
  // they never clear or steal the user's current selection. (#selection-loss)
  let folderNavLoad = false

  // Clear selection and search when folder changes
  $effect(() => {
    const currentAccount = isUnifiedView ? 'unified' : accountId
    const currentFolder = isUnifiedView ? 'inbox' : folderId

    if (!isUnifiedView && (!accountId || !folderId)) {
      prevAccountId = null
      prevFolderId = null
      conversations = []
      totalCount = 0
      checkedThreadIds = new Set()
      return
    }

    // Only reset and reload if folder actually changed
    if (currentAccount === prevAccountId && currentFolder === prevFolderId) return

    prevAccountId = currentAccount
    prevFolderId = currentFolder
    loadGeneration++ // Invalidate any in-flight loads from the previous folder (#200)
    offset = 0
    listContainerRef?.scrollTo({ top: 0 })
    listScrollTop = 0
    checkedThreadIds = new Set()
    lastClickedIndex = null
    // Clear search state when folder changes
    showSearch = false
    searchQuery = ''
    searchResults = []
    searchTotalCount = 0
    searchOffset = 0
    serverSearchMode = false
    serverSearchResults = []
    serverSearchCount = 0
    serverSearchTotalCount = 0
    lastServerQuery = ''
    folderNavLoad = true // this load is a real folder switch → may auto-select first
    loadConversations()
    checkFTSIndexStatus()
  })

  // Compute selected message IDs from all checked conversations (for multi-select context menu)
  // Check both conversations and searchResults since selections can span both
  // Use Set to deduplicate in case same conversation appears in both arrays
  const selectedMessageIds = $derived(collectSelectedMessageIds([...conversations, ...searchResults], checkedThreadIds))

  // Aggregated star/read state for multi-select context menu
  // Show "Star" if any selected is unstarred, show "Mark as Read" if any selected is unread
  const selectedHasUnstarred = $derived(hasSelectedUnstarred([...conversations, ...searchResults], checkedThreadIds))
  const selectedHasUnread = $derived(hasSelectedUnread([...conversations, ...searchResults], checkedThreadIds))

  // Clear multi-select (called when right-clicking on unchecked row)
  function clearSelection() {
    checkedThreadIds = new Set()
    lastClickedIndex = null
  }

  // Check if viewing unified inbox
  const isUnifiedView = $derived(accountId === 'unified' && folderId === 'inbox')

  async function loadConversations(customLimit?: number) {
    // For unified view, we don't need accountId/folderId
    if (!isUnifiedView && (!accountId || !folderId)) return

    // Prevent concurrent loads — defer instead of dropping
    if (loading) {
      pendingReload = true
      return
    }

    loading = true
    error = null

    // Capture offset and generation at start — both may change during async operations
    const currentOffset = offset
    const limit = customLimit ?? PAGE_SIZE
    const generation = loadGeneration

    try {
      const [convList, count] = isUnifiedView
        ? await Promise.all([
          GetUnifiedInboxConversations(currentOffset, limit, getMessageListSortOrder(), filterMode),
          GetUnifiedInboxCount(filterMode),
        ])
        : await Promise.all([
          GetConversations(accountId!, folderId!, currentOffset, limit, getMessageListSortOrder(), filterMode),
          GetConversationCount(accountId!, folderId!, filterMode),
        ])

      // Discard stale result — folder was switched while this load was in-flight (#200)
      if (generation !== loadGeneration) return

      if (currentOffset !== 0) {
        conversations = [...conversations, ...(convList || [])]
        totalCount = count
        return
      }

      conversations = convList || []

      // Apply any flag changes that arrived while we were loading.
      // This fixes the race where MarkAsRead fires before the new array is ready.
      if (pendingFlagChanges.length > 0) {
        for (const change of pendingFlagChanges) {
          for (const c of conversations) {
            const affectedCount = (c.messageIds || []).filter(
              (id: string) => change.messageIds.includes(id)
            ).length
            if (affectedCount > 0) {
              const delta = change.isRead ? -affectedCount : affectedCount
              c.unreadCount = Math.max(0, (c.unreadCount || 0) + delta)
            }
          }
        }
        pendingFlagChanges = []
      }

      // Was this load triggered by genuine folder navigation? (Consume the flag.)
      // A background/deferred reload of the SAME folder must NOT be treated as a
      // folder switch — otherwise it clears or steals the current selection
      // (e.g. opening then closing Settings replays a deferred reload).
      const folderChanged = folderNavLoad
      folderNavLoad = false
      lastLoadedFolderId = folderId // still used by the pagination-exhausted check

      // Auto-select first message on folder navigation or initial load
      if (conversations.length === 0) {
        totalCount = count
        // Only clear the selection / viewer when navigating to an empty folder.
        // A transient empty result from a background reload leaves the current
        // selection untouched.
        if (folderChanged) {
          selectedThreadId = null
          if (getLayoutMode() !== 'narrow') onEmptyFolder?.()
        }
        return
      }

      if (folderChanged || !selectedThreadId) {
        selectedThreadId = conversations[0].threadId
        // Also open it in the viewer so the right pane reflects the highlighted
        // first row. In narrow layout the viewer is an overlay, so don't
        // force it open on a folder switch.
        if (getLayoutMode() !== 'narrow') {
          const first = conversations[0] as any
          const realFolderId = isUnifiedView && first.folderId ? first.folderId : folderId!
          const realAccountId = isUnifiedView && first.accountId ? first.accountId : accountId!
          onConversationSelect?.(first.threadId, realFolderId, realAccountId)
        }
      }
      totalCount = count
    } catch (err) {
      // Discard stale error — folder was switched while this load was in-flight (#200)
      if (generation !== loadGeneration) return
      console.error('Failed to load messages:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      loading = false
      // Flush any deferred reload (from sync event during load or dialog guard).
      // Only the latest-generation load should drive the flush — otherwise a
      // stale completion could fire scheduleReload redundantly.
      if (generation === loadGeneration && pendingReload && !isDialogGuardActive()) {
        pendingReload = false
        scheduleReload()
      }
    }
  }

  export async function syncFolder() {
    // Can't sync unified inbox directly - individual folders must be synced
    if (isUnifiedView || !accountId || !folderId) return

    error = null

    try {
      // SyncFolder returns after headers sync, but body fetch continues in background
      // The account store tracks sync:progress and folder:synced events to manage syncing state
      await SyncFolder(accountId, folderId)
      offset = 0
      await loadConversations()
    } catch (err) {
      console.error('Failed to sync folder:', err)
      error = $_('viewer.failedToLoadMessages')
    }
    // No need to manage syncing state - account store handles it via events
  }

  // Cancel folder sync
  export async function cancelFolderSync() {
    if (isUnifiedView || !accountId || !folderId) return

    try {
      await CancelFolderSync(accountId, folderId)
    } catch (err) {
      console.error('Failed to cancel folder sync:', err)
    }
  }

  // Toggle folder sync (start if not running, cancel if running) - for keyboard shortcut and UI
  export async function toggleFolderSync() {
    if (syncing) {
      await cancelFolderSync()
      return
    }
    await syncFolder()
  }

  // Force re-sync folder (clears bodies & attachments, then re-fetches)
  async function forceSyncFolder() {
    if (isUnifiedView || !accountId || !folderId) return

    error = null

    try {
      await ForceSyncFolder(accountId, folderId)
      offset = 0
      await loadConversations()
    } catch (err) {
      console.error('Failed to force re-sync folder:', err)
      error = $_('viewer.failedToLoadMessages')
    }
  }

  // Handle search input with debounce
  function handleSearchInput() {
    if (!searchQuery.trim()) {
      // Clear search immediately if query is empty
      searchResults = []
      searchTotalCount = 0
      serverSearchResults = []
      serverSearchCount = 0
      serverSearchTotalCount = 0
      serverSearchMode = false
      searchDebouncer.cancel()
      return
    }

    // In server mode, don't auto-search locally — user will press Shift+Enter
    if (serverSearchMode) return

    searchDebouncer.schedule(performSearch)
  }

  // Perform the actual search
  async function performSearch() {
    const query = searchQuery.trim()
    if (!query) {
      searchResults = []
      searchTotalCount = 0
      searchOffset = 0
      return
    }

    // Don't start a new search if one is already in progress
    if (isSearching) return

    isSearching = true
    error = null
    searchOffset = 0  // Reset offset for new search

    try {
      const { results, count } = await searchLocalMessageList({
        isUnifiedView,
        accountId,
        folderId,
        query,
        offset: 0,
        limit: PAGE_SIZE,
        filterMode,
      })
      searchResults = results || []
      searchTotalCount = count
      // Auto-select first search result for keyboard navigation
      if (searchResults.length > 0) {
        selectedThreadId = searchResults[0].threadId
      }
    } catch (err) {
      console.error('Search failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isSearching = false
    }
  }

  // Load more search results (pagination)
  async function loadMoreSearchResults() {
    const query = searchQuery.trim()
    if (!query || isSearching) return

    searchDebouncer.cancel()

    isSearching = true
    const newOffset = searchOffset + PAGE_SIZE

    try {
      const results = await loadMoreLocalMessageListSearch({
        isUnifiedView,
        accountId,
        folderId,
        query,
        offset: newOffset,
        limit: PAGE_SIZE,
        filterMode,
      })

      if (results && results.length > 0) {
        searchResults = [...searchResults, ...results]
        searchOffset = newOffset
      }
    } catch (err) {
      console.error('Load more search results failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isSearching = false
    }
  }

  // Clear search and return to normal view
  function clearSearch() {
    searchQuery = ''
    searchResults = []
    searchTotalCount = 0
    searchOffset = 0
    showSearch = false
    serverSearchMode = false
    serverSearchResults = []
    serverSearchCount = 0
    serverSearchTotalCount = 0
    lastServerQuery = ''
    isServerSearching = false
    searchDebouncer.cancel()
  }

  // Handle keyboard events in search input
  function handleSearchKeydown(event: KeyboardEvent) {
    switch (true) {
      case event.key === 'Enter' && event.shiftKey:
        event.preventDefault()
        if (isUnifiedView) return
        handleShiftEnter()
        break
      case event.key === 'Enter':
        // Move focus from search input to message list so user can navigate with arrow keys
        event.preventDefault()
        searchInputRef?.blur()
        listContainerRef?.focus()
        break
    }
  }

  // Smart toggle/re-search for server search (Shift+Enter)
  function handleShiftEnter() {
    const query = searchQuery.trim()
    if (!query) return

    if (!serverSearchMode) {
      // Local → server
      serverSearchMode = true
      lastServerQuery = query
      performServerSearch()
      return
    }

    if (query !== lastServerQuery) {
      // Server mode, query changed → re-search
      lastServerQuery = query
      performServerSearch()
      return
    }

    // Server mode, same query → toggle back to local
    serverSearchMode = false
  }

  // Perform IMAP server-side search. limit=0 means no limit (show all).
  async function performServerSearch(limit: number = SERVER_SEARCH_LIMIT) {
    const query = searchQuery.trim()
    if (!query || !accountId || !folderId || isUnifiedView) return

    isServerSearching = true
    error = null
    try {
      const response = await searchServerMessageList({ accountId, folderId, query, limit })
      const items = response.results
      serverSearchResults = items
      serverSearchCount = items.length
      serverSearchTotalCount = response.totalCount
      if (items.length > 0) {
        selectedThreadId = items[0].threadId
      }
    } catch (err) {
      console.error('Server search failed:', err)
      error = $_('viewer.failedToLoadMessages')
    } finally {
      isServerSearching = false
    }
  }

  // Check if we're in search mode with results
  const isSearchMode = $derived(showSearch && searchQuery.trim().length > 0)

  // Active list - either conversations, local search results, or server search results
  const activeList = $derived(
    isSearchMode
      ? (serverSearchMode ? serverSearchResults : searchResults)
      : conversations
  )
  const activeCount = $derived(
    isSearchMode
      ? (serverSearchMode ? serverSearchTotalCount : searchTotalCount)
      : totalCount
  )

  // A row is shown selected (highlighted) if it's in the multi-selection set;
  // when nothing is multi-selected, the single open conversation is highlighted.
  function isRowSelected(threadId: string): boolean {
    return checkedThreadIds.size > 0 ? checkedThreadIds.has(threadId) : selectedThreadId === threadId
  }

  function selectConversation(threadId: string, index: number, event?: MouseEvent) {
    // Shift+click: range-select from the anchor to here, seeding with the
    // currently-open conversation so the first click is included in the range.
    if (event?.shiftKey) {
      const newChecked = new Set(checkedThreadIds)
      if (newChecked.size === 0 && selectedThreadId) newChecked.add(selectedThreadId)
      const anchor = lastClickedIndex !== null ? lastClickedIndex : index
      const start = Math.min(anchor, index)
      const end = Math.max(anchor, index)
      for (let i = start; i <= end; i++) {
        newChecked.add(activeList[i].threadId)
      }
      checkedThreadIds = newChecked
      return
    }

    // Update anchor for non-shift clicks
    lastClickedIndex = index

    // Cmd/Ctrl+click: toggle this row in the multi-selection, seeding with the
    // currently-open conversation so it joins the set on the first Cmd+click.
    if (event?.ctrlKey || event?.metaKey) {
      const newChecked = new Set(checkedThreadIds)
      if (newChecked.size === 0 && selectedThreadId) newChecked.add(selectedThreadId)
      toggleSetEntry(newChecked, threadId)
      checkedThreadIds = newChecked
      return
    }

    // Normal click - select for viewing, clear multi-selection
    checkedThreadIds = new Set()
    selectedThreadId = threadId
    if (isSearchMode) {
      scrollToIndex(index, 'start')
    }

    // For unified view or search, use real folderId and accountId from conversation data
    const conversation = activeList[index] as any
    const realFolderId = (isUnifiedView || isSearchMode) && conversation.folderId ? conversation.folderId : folderId!
    const realAccountId = (isUnifiedView || isSearchMode) && conversation.accountId ? conversation.accountId : accountId!

    // If this is a non-local server result, fetch it first
    if (serverSearchMode && conversation._isLocal === false && conversation._uid) {
      fetchAndSelectServerResult(conversation, realFolderId, realAccountId)
      return
    }
    onConversationSelect?.(threadId, realFolderId, realAccountId)
  }

  // Fetch a non-local server result, save locally, update the result, then select
  async function fetchAndSelectServerResult(conversation: any, realFolderId: string, realAccountId: string) {
    try {
      const msg = await FetchServerMessage(realAccountId, realFolderId, conversation._uid)
      if (msg) {
        // Update the server result to be local
        const idx = serverSearchResults.findIndex(r => r._uid === conversation._uid)
        if (idx >= 0) {
          serverSearchResults[idx] = {
            ...serverSearchResults[idx],
            threadId: msg.threadId || msg.id,
            messageIds: [msg.id],
            snippet: msg.snippet || '',
            _isLocal: true,
            _uid: conversation._uid,
          }
          serverSearchResults = serverSearchResults
          selectedThreadId = serverSearchResults[idx].threadId
        }
        onConversationSelect?.(msg.threadId || msg.id, realFolderId, realAccountId)
      }
    } catch (err) {
      console.error('Failed to fetch server message:', err)
      error = $_('viewer.failedToLoadMessages')
    }
  }

  export function handleActionComplete(autoSelectNext: boolean = false) {
    onRowActionComplete?.()
    // Get target index BEFORE reload (for auto-select after delete/archive/spam)
    // Uses earliest checked item's index so bulk delete doesn't overshoot
    const currentIndex = getEarliestCheckedIndex()
    const scrollTop = listContainerRef?.scrollTop ?? 0

    // If in search mode, refresh search results instead of conversations
    if (isSearchMode) {
      performSearch().then(() => {
        // Restore scroll position
        if (listContainerRef) {
          requestAnimationFrame(() => {
            listContainerRef!.scrollTop = scrollTop
          })
        }

        // Auto-select next message if requested
        if (autoSelectNext) {
          const isNarrow = getLayoutMode() === 'narrow'
          if (isNarrow) {
            hideViewer()
          }
          if (currentIndex >= 0 && searchResults.length > 0) {
            const newIndex = Math.min(currentIndex, searchResults.length - 1)
            const conv = searchResults[newIndex]
            if (conv) {
              if (isNarrow) {
                selectedThreadId = conv.threadId
              }
              if (!isNarrow) {
                selectConversation(conv.threadId, newIndex)
              }
            }
          }
        }
      })
      return
    }

    // Preserve loaded messages: reload all messages that were loaded
    // Use conversations.length to track actual loaded count (offset gets reset after first action)
    const totalLoaded = Math.max(conversations.length, PAGE_SIZE)
    offset = 0

    loadConversations(totalLoaded).then(() => {
      // Restore scroll position
      if (listContainerRef) {
        requestAnimationFrame(() => {
          listContainerRef!.scrollTop = scrollTop
        })
      }

      // Auto-select next message if requested (for delete/archive/spam actions)
      // After reload, the same index now points to what was the "next" message
      if (autoSelectNext) {
        const isNarrow = getLayoutMode() === 'narrow'
        if (isNarrow) {
          hideViewer()
        }
        if (currentIndex >= 0 && conversations.length > 0) {
          const newIndex = Math.min(currentIndex, conversations.length - 1)
          const conv = conversations[newIndex]
          if (conv) {
            if (isNarrow) {
              selectedThreadId = conv.threadId
            }
            if (!isNarrow) {
              selectConversation(conv.threadId, newIndex)
            }
          }
        }
      }

    })
  }

  // Toggle sort order and persist to backend
  async function toggleSortOrder() {
    const newOrder = getMessageListSortOrder() === 'newest' ? 'oldest' : 'newest'
    try {
      await SetMessageListSortOrder(newOrder)
      setMessageListSortOrder(newOrder)
      offset = 0
      loadConversations()
    } catch (err) {
      console.error('Failed to save sort order:', err)
    }
  }

  // Set filter mode and reload
  function setFilter(mode: string) {
    filterMode = mode
    offset = 0
    if (isSearchMode) {
      searchOffset = 0
      performSearch()
      return
    }
    loadConversations()
  }

  // Total unread for the header. Use the folder's authoritative unread count
  // (the same live value the sidebar badge shows) so the header always agrees
  // with the badge — summing only loaded conversations under-counts once the
  // list is paginated (unread threads past the first page aren't loaded). The
  // unified view has no single folder, so it keeps the loaded-sum fallback.
  const unreadCount = $derived(
    isUnifiedView
      ? conversations.reduce((sum, c) => sum + (c.unreadCount || 0), 0)
      : accountStore.getFolderUnreadCount(folderId)
  )

  // Reference to the list container for scrolling
  let listContainerRef = $state<HTMLDivElement | null>(null)
  let listViewportHeight = $state(0)
  let listScrollTop = $state(0)

  const rowHeight = $derived(getMessageListRowHeight(getMessageListDensity()))

  // Reference to the "Load more" button for keyboard navigation
  let loadMoreButtonRef = $state<HTMLButtonElement | null>(null)

  let rowContextMenu = $state<RowContextMenuState>({
    messageIds: [],
    accountId: '',
    folderId: '',
    folderType: '',
    isStarred: false,
    isRead: true,
    allowReply: false,
  })

  function handleListScroll() {
    listScrollTop = listContainerRef?.scrollTop ?? 0
  }

  function getVirtualWindow<T>(items: T[]): VirtualWindow<T> {
    return calculateVirtualWindow(items, listViewportHeight, listScrollTop, rowHeight)
  }

  function prepareRowContextMenu(conversation: any, rowAccountId: string, rowFolderId: string, useMultiSelect: boolean) {
    if (useMultiSelect) {
      rowContextMenu = createMultiRowContextMenu({
        messageIds: selectedMessageIds,
        accountId: rowAccountId,
        folderId: rowFolderId,
        folderType,
        hasUnstarred: selectedHasUnstarred,
        hasUnread: selectedHasUnread,
      })
      return
    }

    clearSelection()
    rowContextMenu = createSingleRowContextMenu({
      conversation,
      accountId: rowAccountId,
      folderId: rowFolderId,
      folderType,
    })
  }

  type RowRenderOptions = {
    useItemLocation?: boolean
    showAccountIndicator?: boolean
    showSearchFields?: boolean
    showNonLocal?: boolean
    allowDraftOpen?: boolean
  }

  // Get current selected index
  function getSelectedIndex(): number {
    if (!selectedThreadId) return -1
    return activeList.findIndex(c => c.threadId === selectedThreadId)
  }

  // Select previous message (exposed for keyboard navigation)
  // Moves the single selection; collapses any multi-selection back to single.
  export function selectPrevious() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    const newIndex = currentIndex <= 0 ? 0 : currentIndex - 1

    const conv = activeList[newIndex]
    if (conv) {
      if (checkedThreadIds.size > 0) checkedThreadIds = new Set()
      selectedThreadId = conv.threadId
      lastClickedIndex = newIndex
      scrollToIndex(newIndex)
      // Blur any focused element so Enter key triggers openSelected() instead of the button
      ;(document.activeElement as HTMLElement)?.blur?.()
    }
  }

  // Select next message (exposed for keyboard navigation)
  // Moves the single selection; collapses any multi-selection back to single.
  export function selectNext() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()

    // If at last message and more are available, focus the "Load more" button
    if (currentIndex >= activeList.length - 1 && activeList.length < activeCount) {
      loadMoreButtonRef?.focus()
      return
    }

    const newIndex = currentIndex >= activeList.length - 1 ? activeList.length - 1 : currentIndex + 1

    const conv = activeList[newIndex]
    if (conv) {
      if (checkedThreadIds.size > 0) checkedThreadIds = new Set()
      selectedThreadId = conv.threadId
      lastClickedIndex = newIndex
      scrollToIndex(newIndex)
      // Blur any focused element so Enter key triggers openSelected() instead of the button
      ;(document.activeElement as HTMLElement)?.blur?.()
    }
  }

  // Open the currently selected conversation (exposed for keyboard navigation)
  export function openSelected() {
    if (!selectedThreadId) return

    const index = getSelectedIndex()
    if (index >= 0) {
      const conv = activeList[index] as any
      const realFolderId = (isUnifiedView || isSearchMode) && conv.folderId ? conv.folderId : folderId!
      const realAccountId = (isUnifiedView || isSearchMode) && conv.accountId ? conv.accountId : accountId!
      onConversationSelect?.(selectedThreadId, realFolderId, realAccountId)
    }
  }

  // Select a specific thread by ID (exposed for notification clicks and the
  // Contacts related-mail list). The target may be an old conversation that
  // isn't in the first loaded page, so page more conversations in (bounded)
  // until it appears, then scroll it to the TOP. selectedThreadId is (re)set
  // after loading so the folder-change auto-select-first doesn't override it.
  export async function selectThread(threadId: string) {
    selectedThreadId = threadId
    if (isSearchMode) {
      clearSearch()
      await tick()
    }

    let iterations = 0
    let loads = 0
    while (iterations++ < 100) {
      const index = activeList.findIndex(c => c.threadId === threadId)
      if (index >= 0) {
        selectedThreadId = threadId
        scrollToIndex(index, 'start')
        return
      }
      // Wait for any in-flight load before deciding to page in more.
      if (loading) {
        await new Promise((r) => setTimeout(r, 40))
        continue
      }
      // Stop if everything is loaded, or we've paged in enough (~12 pages).
      const exhausted = lastLoadedFolderId === folderId && conversations.length >= totalCount
      if (exhausted || loads >= 12) {
        selectedThreadId = threadId
        return
      }
      loads++
      offset = conversations.length
      await loadConversations()
    }
    selectedThreadId = threadId
  }

  // Toggle search focus (exposed for keyboard navigation via Ctrl+S)
  // Three-state: closed → open, open but unfocused → focus, open and focused → close
  export function toggleSearchFocus() {
    switch (true) {
      case !showSearch:
        showSearch = true
        setTimeout(() => searchInputRef?.focus(), 50)
        break
      case document.activeElement !== searchInputRef:
        searchInputRef?.focus()
        break
      default:
        clearSearch()
    }
  }

  // Get the currently selected thread ID (exposed for parent access)
  export function getSelectedThreadId(): string | null {
    return selectedThreadId
  }

  // Get message IDs for the keyboard-focused thread (for delete without checking)
  export function getSelectedMessageIds(): string[] {
    if (!selectedThreadId) return []
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    if (!conv) return []
    return conv.messageIds || conv.messages?.map((m: any) => m.id) || []
  }

  // Get account and folder info for the keyboard-focused thread (for unified inbox)
  export function getSelectedConversationInfo(): { accountId: string; folderId: string } | null {
    if (!selectedThreadId) return null
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    if (!conv) return null

    const realAccountId = (isUnifiedView || isSearchMode) && conv.accountId ? conv.accountId : accountId
    const realFolderId = (isUnifiedView || isSearchMode) && conv.folderId ? conv.folderId : folderId

    if (!realAccountId || !realFolderId) return null
    return { accountId: realAccountId, folderId: realFolderId }
  }

  // Check if the keyboard-focused thread is starred
  export function isSelectedStarred(): boolean {
    if (!selectedThreadId) return false
    const conv = activeList.find(c => c.threadId === selectedThreadId) as any
    return conv?.isStarred ?? false
  }

  // Toggle checkbox for focused message (Space key)
  export function toggleCheck() {
    if (!selectedThreadId) return
    const newChecked = new Set(checkedThreadIds)
    toggleSetEntry(newChecked, selectedThreadId)
    checkedThreadIds = newChecked
    lastClickedIndex = getSelectedIndex()
  }

  // Select previous message AND check both current and previous (Shift+Up/k)
  export function selectPreviousWithCheck() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    if (currentIndex <= 0) return  // Already at top or no selection

    const newIndex = currentIndex - 1
    const conv = activeList[newIndex]
    if (!conv) return

    // Check both current and new message
    const newChecked = new Set(checkedThreadIds)
    newChecked.add(activeList[currentIndex].threadId)
    newChecked.add(conv.threadId)
    checkedThreadIds = newChecked

    // Move focus (but don't open in viewer)
    selectedThreadId = conv.threadId
    scrollToIndex(newIndex)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Select next message AND check both current and next (Shift+Down/j)
  export function selectNextWithCheck() {
    if (activeList.length === 0) return

    const currentIndex = getSelectedIndex()
    if (currentIndex < 0 || currentIndex >= activeList.length - 1) return  // No selection or already at bottom

    const newIndex = currentIndex + 1
    const conv = activeList[newIndex]
    if (!conv) return

    // Check both current and new message
    const newChecked = new Set(checkedThreadIds)
    newChecked.add(activeList[currentIndex].threadId)
    newChecked.add(conv.threadId)
    checkedThreadIds = newChecked

    // Move focus (but don't open in viewer)
    selectedThreadId = conv.threadId
    scrollToIndex(newIndex)
    // Blur any focused element so Enter key triggers openSelected() instead of the button
    ;(document.activeElement as HTMLElement)?.blur?.()
  }

  // Get all checked message IDs for bulk operations
  export function getCheckedMessageIds(): string[] {
    return selectedMessageIds
  }

  // Check if any messages are checked
  export function hasCheckedMessages(): boolean {
    return checkedThreadIds.size > 0
  }

  // Get aggregated star state (true if any unstarred)
  export function getCheckedHasUnstarred(): boolean {
    return selectedHasUnstarred
  }

  // Get aggregated read state (true if any unread)
  export function getCheckedHasUnread(): boolean {
    return selectedHasUnread
  }

  // Get index of earliest checked item (for post-delete focus)
  function getEarliestCheckedIndex(): number {
    if (checkedThreadIds.size === 0) return getSelectedIndex()
    for (let i = 0; i < activeList.length; i++) {
      if (checkedThreadIds.has(activeList[i].threadId)) return i
    }
    return getSelectedIndex()
  }

  // Clear all checkboxes
  export function clearChecked() {
    checkedThreadIds = new Set()
    lastClickedIndex = null
  }

  export function selectAll() {
    checkedThreadIds = new Set(activeList.map(c => c.threadId))
  }

  // Open context menu for the currently selected conversation row
  export async function openContextMenu() {
    if (!selectedThreadId || !listContainerRef) return
    const index = activeList.findIndex(c => c.threadId === selectedThreadId)
    if (index < 0) return
    let row = listContainerRef.querySelector(`[data-conversation-row][data-row-index="${index}"]`) as HTMLElement | null
    if (!row) {
      listContainerRef.scrollTop = Math.max(0, index * rowHeight)
      listScrollTop = listContainerRef.scrollTop
      await tick()
      row = listContainerRef.querySelector(`[data-conversation-row][data-row-index="${index}"]`) as HTMLElement | null
    }
    if (!row) return
    const rect = row.getBoundingClientRect()
    row.dispatchEvent(new MouseEvent('contextmenu', {
      bubbles: true,
      clientX: rect.right,
      clientY: rect.top + rect.height / 2,
    }))
  }

  // Permanent delete confirmation state
  let showDeleteConfirm = $state(false)
  let pendingDeleteIds = $state<string[]>([])

  // Empty trash confirmation state
  let showEmptyTrashConfirm = $state(false)

  async function handleUndo() {
    try {
      const description = await Undo()
      toasts.success($_('toast.undone', { values: { description } }))
    } catch (err) {
      console.error('Undo failed:', err)
      toasts.error($_('toast.undoFailed'))
    }
  }

  async function handleConfirmPermanentDelete() {
    try {
      await DeletePermanently(pendingDeleteIds)
      toasts.success($_('toast.permanentlyDeleted'))
      handleActionComplete(true)
      clearChecked()
    } catch (err) {
      console.error('Permanent delete failed:', err)
      toasts.error($_('toast.failedToDelete'))
    }
    showDeleteConfirm = false
    pendingDeleteIds = []
  }

  async function handleEmptyTrash() {
    if (!accountId || !folderId) return
    try {
      await EmptyTrash(accountId, folderId)
      toasts.success($_('toast.trashEmptied'))
      handleActionComplete(true)
      clearChecked()
    } catch (err) {
      console.error('Empty trash failed:', err)
      toasts.error($_('toast.failedToEmptyTrash'))
    }
    showEmptyTrashConfirm = false
  }

  // Shared delete handler — same flow as context menu "Delete" action
  // Set permanent=true to force permanent delete (e.g. Shift+Delete)
  export function requestDelete(messageIds: string[], permanent: boolean = false) {
    if (permanent || folderType === 'trash') {
      pendingDeleteIds = messageIds
      showDeleteConfirm = true
      return
    }
    Trash(messageIds)
      .then((movedToTrash) => {
        const toastMsg = movedToTrash ? $_('toast.movedToTrash') : $_('toast.deletedFromFolder')
        const actions = movedToTrash ? [{ label: $_('common.undo'), onClick: handleUndo }] : []
        toasts.success(toastMsg, actions)
        handleActionComplete(true)
        clearChecked()
      })
      .catch((err) => {
        console.error('Delete failed:', err)
        toasts.error($_('toast.failedToDelete'))
      })
  }

  // Scroll the list back to the very top (one-click "to top" button).
  function scrollToTop() {
    listContainerRef?.scrollTo({ top: 0, behavior: 'smooth' })
  }

  // Scroll to a specific index in the list
  function scrollToIndex(index: number, block: 'start' | 'center' | 'end' | 'nearest' = 'nearest') {
    if (!listContainerRef) return

    const targetTop = index * rowHeight
    const targetBottom = targetTop + rowHeight
    const viewportTop = listContainerRef.scrollTop
    const viewportBottom = viewportTop + listContainerRef.clientHeight
    let scrollTop = viewportTop

    switch (block) {
      case 'start':
        scrollTop = targetTop
        break
      case 'center':
        scrollTop = targetTop - (listContainerRef.clientHeight - rowHeight) / 2
        break
      case 'end':
        scrollTop = targetBottom - listContainerRef.clientHeight
        break
      case 'nearest':
        if (targetTop < viewportTop) {
          scrollTop = targetTop
        } else if (targetBottom > viewportBottom) {
          scrollTop = targetBottom - listContainerRef.clientHeight
        }
        break
    }

    if (scrollTop !== viewportTop) {
      listContainerRef.scrollTo({ top: Math.max(0, scrollTop), behavior: 'smooth' })
    }
  }
</script>

<div class="flex flex-col h-full {isFlashing ? 'pane-focus-flash' : ''}">
  <!-- Header -->
  <div class="flex items-center gap-2 px-4 py-3 border-b border-border">
    {#if showFolderToggle}
      <button
        class="p-1.5 -ml-1 rounded-md hover:bg-muted transition-colors flex-shrink-0"
        title={$_('responsive.folders')}
        aria-label={$_('aria.toggleSidebar')}
        onclick={onToggleSidebar}
      >
        <Icon icon="mdi:dock-left" class="w-5 h-5 text-muted-foreground" />
      </button>
    {/if}
    <!-- Title (always shown). Search opens to its right via the `/` shortcut. -->
    <div class="flex items-center gap-2 min-w-0">
      <h2 class="font-semibold text-foreground truncate">{folderName}</h2>
      <span class="text-sm text-muted-foreground whitespace-nowrap flex-shrink-0">
        {$_('messageList.unread', { values: { count: unreadCount } })}
      </span>
    </div>
    {#if showSearch}
      <!-- Search input — sits where the search button used to be -->
      <div class="flex items-center gap-1 bg-muted rounded-md px-2 flex-1 min-w-0">
        <Icon icon="mdi:magnify" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
        <input
          bind:this={searchInputRef}
          type="text"
          placeholder={$_('messageList.searchMessages')}
          class="bg-transparent border-none outline-none text-sm py-1.5 w-full min-w-0"
          bind:value={searchQuery}
          oninput={handleSearchInput}
          onkeydown={handleSearchKeydown}
        />
        {#if serverSearchMode}
          <button
            onclick={() => { serverSearchMode = false }}
            class="px-1.5 py-0.5 text-[10px] font-medium bg-primary/20 text-primary rounded-full flex-shrink-0 hover:bg-primary/30 transition-colors"
            title={$_('search.localSearch')}
          >
            {$_('search.server')}
          </button>
        {/if}
        {#if searchQuery || isSearching || isServerSearching}
          <button
            onclick={clearSearch}
            class="p-0.5 hover:bg-muted-foreground/20 rounded flex-shrink-0"
            title={$_('messageList.clearSearch')}
          >
            {#if isSearching || isServerSearching}
              <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground" />
            {:else}
              <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
            {/if}
          </button>
        {/if}
      </div>
    {:else}
      <div class="flex-1"></div>
    {/if}
    <div class="flex items-center gap-1 flex-shrink-0">
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors"
        title={$_('common.search')}
        aria-label={$_('common.search')}
        onclick={onSearch}
      >
        <Icon icon="mdi:magnify" class="w-5 h-5 text-muted-foreground" />
      </button>
      <!-- Scroll list to top -->
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors"
        title={$_('messageList.scrollToTop')}
        aria-label={$_('messageList.scrollToTop')}
        onclick={scrollToTop}
      >
        <Icon icon="mdi:arrow-collapse-up" class="w-5 h-5 text-muted-foreground" />
      </button>
      {#if syncing}
        <!-- While syncing, show spinning icon that cancels on click -->
        <button
          class="p-2 rounded-md hover:bg-muted transition-colors"
          title={syncProgress ? `${$_('sidebar.syncing')} ${syncProgress.phase}: ${syncProgress.percentage}% - ${$_('sidebar.clickToCancel')}` : `${$_('sidebar.syncing')} ${$_('sidebar.clickToCancel')}`}
          onclick={cancelFolderSync}
        >
          <Icon
            icon="mdi:refresh"
            class="w-5 h-5 text-muted-foreground animate-spin"
          />
        </button>
      {:else}
        <!-- Dropdown menu for sync options -->
        <DropdownMenu.Root>
          <DropdownMenu.Trigger
            class="p-2 rounded-md hover:bg-muted transition-colors disabled:opacity-50"
            disabled={loading || isUnifiedView}
          >
            <Icon
              icon="mdi:refresh"
              class="w-5 h-5 text-muted-foreground"
            />
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              side="bottom"
              align="end"
              sideOffset={4}
              class={cn(
                'z-50 min-w-[180px] rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
                'data-[state=open]:animate-in data-[state=closed]:animate-out',
                'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
                'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
                'data-[side=bottom]:slide-in-from-top-2'
              )}
            >
              <DropdownMenu.Item
                onSelect={syncFolder}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:refresh" class="w-4 h-4 mr-2" />
                {$_('messageList.syncFolder')}
              </DropdownMenu.Item>
              <DropdownMenu.Separator class="-mx-1 my-1 h-px bg-border" />
              <DropdownMenu.Item
                onSelect={forceSyncFolder}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:refresh-auto" class="w-4 h-4 mr-2" />
                {$_('messageList.forceResync')}
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      {/if}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          class="p-2 rounded-md hover:bg-muted transition-colors {filterMode ? 'bg-muted' : ''}"
          title={$_('messageList.filter')}
        >
          <Icon
            icon={filterMode ? 'mdi:filter' : 'mdi:filter-outline'}
            class="w-5 h-5 {filterMode ? 'text-primary' : 'text-muted-foreground'}"
          />
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            side="bottom"
            align="end"
            sideOffset={4}
            class={cn(
              'z-50 min-w-[180px] rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
              'data-[state=open]:animate-in data-[state=closed]:animate-out',
              'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
              'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
              'data-[side=bottom]:slide-in-from-top-2'
            )}
          >
            {#each filterOptions as opt (opt.value ?? opt.label)}
              {#if opt.separator}
                <DropdownMenu.Separator class="-mx-1 my-1 h-px bg-border" />
              {/if}
              <DropdownMenu.Item
                onSelect={() => setFilter(opt.value)}
                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground"
              >
                <Icon icon="mdi:check" class="w-4 h-4 mr-2 {filterMode === opt.value ? '' : 'invisible'}" />
                {opt.label}
              </DropdownMenu.Item>
            {/each}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors"
        title={getMessageListSortOrder() === 'newest' ? $_('messageList.showingNewest') : $_('messageList.showingOldest')}
        onclick={toggleSortOrder}
      >
        <Icon
          icon={getMessageListSortOrder() === 'newest' ? 'mdi:sort-descending' : 'mdi:sort-ascending'}
          class="w-5 h-5 text-muted-foreground"
        />
      </button>
    </div>
  </div>

  <!-- Active filter chip -->
  {#if filterMode}
    <div class="flex items-center gap-2 px-4 py-1.5 border-b border-border bg-muted/30">
      <span class="text-xs text-muted-foreground">{$_('messageList.filterLabel')}:</span>
      <button
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-primary/10 text-primary hover:bg-primary/20 transition-colors"
        onclick={() => setFilter('')}
      >
        {filterLabel}
        <Icon icon="mdi:close" class="w-3 h-3" />
      </button>
    </div>
  {/if}

  <!-- Empty Trash bar (only shown when viewing trash folder with messages, not in search mode) -->
  {#if folderType === 'trash' && totalCount > 0 && !isSearchMode}
    <div class="flex items-center justify-end px-4 py-2 bg-muted/50 border-b border-border">
      <Button
        size="sm"
        variant="outline"
        class="text-destructive hover:text-destructive hover:bg-destructive/10 border-destructive/50 bg-muted/50"
        onclick={() => { showEmptyTrashConfirm = true }}
      >
        <Icon icon="mdi:delete-sweep-outline" class="w-4 h-4 mr-1.5" />
        {$_('messageList.emptyTrash')}
      </Button>
    </div>
  {/if}

  <!-- FTS Indexing indicator (only shown when searching and index is incomplete) -->
  {#if showSearch && !indexComplete && isIndexing}
    <div class="px-4 py-2 bg-muted/50 border-b border-border">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Icon icon="mdi:database-sync" class="w-4 h-4 animate-pulse" />
        <span>{$_('messageList.ftsBuilding', { values: { percentage: indexProgress } })}</span>
      </div>
      <div class="h-1 bg-muted rounded-full mt-1.5 overflow-hidden">
        <div
          class="h-full bg-primary transition-all duration-300"
          style="width: {indexProgress}%"
        ></div>
      </div>
    </div>
  {/if}

  {#snippet conversationRows(window: VirtualWindow<any>, options: RowRenderOptions)}
    <MessageContextMenu
      messageIds={rowContextMenu.messageIds}
      accountId={rowContextMenu.accountId}
      currentFolderId={rowContextMenu.folderId}
      folderType={rowContextMenu.folderType}
      isStarred={rowContextMenu.isStarred}
      isRead={rowContextMenu.isRead}
      onActionComplete={handleActionComplete}
      onReply={rowContextMenu.allowReply ? onReply : undefined}
    >
      <div>
        <div aria-hidden="true" style="height: {window.topHeight}px"></div>
        {#each window.rows as row (row.item.threadId + '-' + row.index)}
          {@const conversation = row.item}
          {@const index = row.index}
          {@const rowAccountId = conversation.accountId || accountId || ''}
          {@const rowFolderId = conversation.folderId || folderId || ''}
          {@const resolvedAccountId = options.useItemLocation || isUnifiedView ? rowAccountId : accountId!}
          {@const resolvedFolderId = options.useItemLocation || isUnifiedView ? rowFolderId : folderId!}
          <ConversationRow
            {conversation}
            density={getMessageListDensity()}
            selected={isRowSelected(conversation.threadId)}
            checked={checkedThreadIds.has(conversation.threadId)}
            accountId={resolvedAccountId}
            folderId={resolvedFolderId}
            rowIndex={index}
            {selectedMessageIds}
            showAccountIndicator={!!options.showAccountIndicator && isUnifiedView}
            accountColor={conversation.accountColor || ''}
            accountName={conversation.accountName || ''}
            highlightedSubject={options.showSearchFields ? conversation.highlightedSubject : ''}
            highlightedSnippet={options.showSearchFields ? conversation.highlightedSnippet : ''}
            highlightedFromName={options.showSearchFields ? conversation.highlightedFromName : ''}
            searchFolderName={options.showSearchFields ? conversation.folderName : ''}
            searchFolderType={options.showSearchFields ? conversation.folderType : ''}
            isNonLocal={!!options.showNonLocal && conversation._isLocal === false}
            onSelect={(e?: MouseEvent) => selectConversation(conversation.threadId, index, e)}
            onContextMenu={() => prepareRowContextMenu(conversation, resolvedAccountId, resolvedFolderId, checkedThreadIds.has(conversation.threadId))}
            onActionComplete={handleActionComplete}
            onOpenDraft={options.allowDraftOpen && folderType === 'drafts' && conversation.messageIds?.[0]
              ? () => onOpenDraft?.(conversation.messageIds[0])
              : undefined}
          />
        {/each}
        <div aria-hidden="true" style="height: {window.bottomHeight}px"></div>
      </div>
    </MessageContextMenu>
  {/snippet}

  <!-- Conversation List -->
  <div
    bind:this={listContainerRef}
    bind:clientHeight={listViewportHeight}
    class="flex-1 overflow-y-auto scrollbar-thin"
    onscroll={handleListScroll}
  >
    {#if loading && conversations.length === 0 && !isSearchMode}
      <div class="flex items-center justify-center h-32">
        <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    {:else if error}
      <div class="flex flex-col items-center justify-center h-32 text-center px-4">
        <Icon icon="mdi:alert-circle-outline" class="w-8 h-8 text-destructive mb-2" />
        <p class="text-sm text-destructive">{error}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={() => isSearchMode ? performSearch() : loadConversations()}
        >
          {$_('messageList.tryAgain')}
        </button>
      </div>
    {:else if !isUnifiedView && (!accountId || !folderId)}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:email-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.selectFolder')}</p>
      </div>
    {:else if isSearchMode}
      <!-- Search Results -->
      {#if isSearching || isServerSearching}
        <div class="flex flex-col items-center justify-center h-32 gap-2">
          <Icon icon="mdi:loading" class="w-6 h-6 animate-spin text-muted-foreground" />
          {#if isServerSearching}
            <span class="text-xs text-muted-foreground">{$_('search.serverSearching')}</span>
          {/if}
        </div>
      {:else if serverSearchMode}
        <!-- Server search results -->
        {#if serverSearchResults.length === 0}
          <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
            <Icon icon="mdi:magnify" class="w-12 h-12 mb-2" />
            <p>{$_('messageList.noResults', { values: { query: searchQuery } })}</p>
          </div>
        {:else}
          <!-- Server results header -->
          <div class="flex items-center justify-between px-4 py-2 bg-muted/30 border-b border-border text-sm text-muted-foreground">
            <span>
              {#if serverSearchCount < serverSearchTotalCount}
                {$_('search.serverResultsCapped', { values: { shown: serverSearchCount, total: serverSearchTotalCount, query: searchQuery } })}
              {:else}
                {$_('search.serverResults', { values: { count: serverSearchCount, query: searchQuery } })}
              {/if}
            </span>
            <button
              class="text-xs text-primary hover:underline"
              onclick={() => { serverSearchMode = false }}
            >
              {$_('search.localSearch')}
            </button>
          </div>
          {@const serverWindow = getVirtualWindow(serverSearchResults)}
          {@render conversationRows(serverWindow, { useItemLocation: true, showNonLocal: true })}

          <!-- Show all results button (when results are capped) -->
          {#if serverSearchCount < serverSearchTotalCount}
            <div class="flex justify-center py-4">
              <button
                bind:this={loadMoreButtonRef}
                class="text-sm text-primary hover:underline focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded px-2 py-1"
                onclick={() => performServerSearch(0)}
                disabled={isServerSearching}
              >
                {isServerSearching ? $_('common.loading') : $_('search.showAllResults', { values: { total: serverSearchTotalCount } })}
              </button>
            </div>
          {/if}
        {/if}
      {:else if searchResults.length === 0}
        <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
          <Icon icon="mdi:magnify" class="w-12 h-12 mb-2" />
          <p>{$_('messageList.noResults', { values: { query: searchQuery } })}</p>
          {#if !indexComplete}
            <p class="text-xs mt-1">{$_('messageList.indexBuilding')}</p>
          {/if}
          {#if !isUnifiedView && accountId && folderId}
            <button
              class="mt-2 text-sm text-primary hover:underline"
              onclick={() => { serverSearchMode = true; lastServerQuery = searchQuery.trim(); performServerSearch() }}
            >
              {$_('search.searchOnServer')}
            </button>
          {/if}
        </div>
      {:else}
        <!-- Local search results header -->
        <div class="flex items-center justify-between px-4 py-2 bg-muted/30 border-b border-border text-sm text-muted-foreground">
          <span>{$_('messageList.foundResults', { values: { count: searchTotalCount, query: searchQuery } })}</span>
          {#if !isUnifiedView && accountId && folderId}
            <button
              class="text-xs text-primary hover:underline"
              onclick={() => { serverSearchMode = true; lastServerQuery = searchQuery.trim(); performServerSearch() }}
            >
              {$_('search.serverSearch')}
            </button>
          {/if}
        </div>
        {@const searchWindow = getVirtualWindow(searchResults)}
        {@render conversationRows(searchWindow, { showAccountIndicator: true, showSearchFields: true })}

        <!-- Load more search results -->
        {#if searchResults.length < searchTotalCount}
          <div class="flex justify-center py-4">
            <button
              bind:this={loadMoreButtonRef}
              class="text-sm text-primary hover:underline focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded px-2 py-1"
              onclick={() => loadMoreSearchResults()}
              disabled={isSearching}
            >
              {isSearching ? $_('common.loading') : $_('messageList.loadMore', { values: { remaining: searchTotalCount - searchResults.length } })}
            </button>
          </div>
        {/if}
      {/if}
    {:else if conversations.length === 0 && filterMode}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:filter-off-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.noFilteredMessages')}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={() => setFilter('')}
        >
          {$_('messageList.filterAll')}
        </button>
      </div>
    {:else if conversations.length === 0}
      <div class="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Icon icon="mdi:inbox-outline" class="w-12 h-12 mb-2" />
        <p>{$_('messageList.noMessages')}</p>
        <button
          class="mt-2 text-sm text-primary hover:underline"
          onclick={syncFolder}
          disabled={syncing}
        >
          {$_('messageList.syncNow')}
        </button>
      </div>
    {:else}
      {@const conversationWindow = getVirtualWindow(conversations)}
      {@render conversationRows(conversationWindow, { showAccountIndicator: true, allowDraftOpen: true })}

      <!-- Load more button for pagination -->
      {#if conversations.length < totalCount}
        <div class="flex justify-center py-4">
          <button
            bind:this={loadMoreButtonRef}
            class="text-sm text-primary hover:underline focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded px-2 py-1"
            onclick={() => {
              offset += PAGE_SIZE
              loadConversations()
            }}
            disabled={loading}
          >
            {loading ? $_('common.loading') : $_('messageList.loadMore', { values: { remaining: totalCount - conversations.length } })}
          </button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<!-- Permanent Delete Confirmation Dialog -->
<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('dialog.deletePermanently')}
  description={$_('dialog.deleteDescription')}
  confirmLabel={$_('dialog.confirmDeletePermanently')}
  variant="destructive"
  onConfirm={handleConfirmPermanentDelete}
  onCancel={() => { showDeleteConfirm = false; pendingDeleteIds = [] }}
/>

<!-- Empty Trash Confirmation Dialog -->
<ConfirmDialog
  bind:open={showEmptyTrashConfirm}
  title={$_('dialog.emptyTrash')}
  description={$_('dialog.emptyTrashDescription')}
  confirmLabel={$_('dialog.confirmEmptyTrash')}
  variant="destructive"
  onConfirm={handleEmptyTrash}
  onCancel={() => { showEmptyTrashConfirm = false }}
/>
