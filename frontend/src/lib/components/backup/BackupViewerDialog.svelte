<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ModalFrame from '$lib/components/ui/ModalFrame.svelte'
  import BackupViewerMessageDetail from '$lib/components/backup/BackupViewerMessageDetail.svelte'
  import BackupViewerSearchOverlay from '$lib/components/backup/BackupViewerSearchOverlay.svelte'
  import BackupViewerToolbar from '$lib/components/backup/BackupViewerToolbar.svelte'
  // @ts-ignore - wailsjs path
  import {
    BuildBackupViewerIndex,
    GetBackupSettings,
    GetBackupViewerCatalog,
    GetBackupViewerMessage,
    ListBackupViewerMessages,
    OpenBackupViewerDirectory,
    SaveBackupViewerAttachmentAs,
    SearchBackupViewerMessages,
  } from '../../../../wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs path
  import type { app } from '../../../../wailsjs/go/models'
  import { toasts } from '$lib/stores/toast'
  import { getDarkMailContent, getThemeMode } from '$lib/stores/settings.svelte'
  import { buildDarkMailFilterStyles } from '$lib/utils/dark-mail'
  import { createDebouncer } from '$lib/utils/debounce'
  import { formatLocalDateTime, formatLocalDateTimeShort, parseFlexibleDate } from '$lib/utils/date'
  import {
    rememberBackupDirectory,
    removeBackupDirectory,
  } from '$lib/utils/backup-directory-history'
  import { dialogGuardClose, dialogGuardOpen } from '$lib/stores/dialogGuard'
  import { nextRovingIndex, type RovingNavigationKey } from '$lib/keyboard/regionNavigation'

  interface Props {
    open?: boolean
    onClose?: () => void
  }

  interface Scope {
    id: string
    label: string
    count?: number
  }

  let { open = $bindable(false), onClose }: Props = $props()

  let directory = $state('')
  let directoryMenuOpen = $state(false)
  let catalog = $state<app.BackupViewerCatalog | null>(null)
  let messages = $state<app.BackupViewerMessageSummary[]>([])
  let messagesTotal = $state(0)
  let selectedAccountEmail = $state('')
  let selectedMessageKey = $state('')
  let detail = $state<app.BackupViewerMessageDetail | null>(null)
  let loadingCatalog = $state(false)
  let loadingMessages = $state(false)
  let loadingMoreMessages = $state(false)
  let buildingIndex = $state(false)
  let loadingDetail = $state(false)
  let errorMessage = $state('')
  let darkFilterEnabled = $state(false)
  let savingAttachmentIndexes = $state<Set<number>>(new Set())
  let attachmentCountsByKey = $state<Record<string, number>>({})
  let attachmentsExpanded = $state(true)
  let messageSortOrder = $state<'newest' | 'oldest'>('newest')
  let messageListEl = $state<HTMLDivElement | null>(null)

  let searchOpen = $state(false)
  let searchQuery = $state('')
  let searchResults = $state<app.BackupViewerMessageSummary[]>([])
  let searchLoading = $state(false)
  let searchActiveIndex = $state(0)
  let searchScopeEmail = $state('')
  let searchInputEl = $state<HTMLInputElement | null>(null)
  const searchDebouncer = createDebouncer(200)
  let searchSeq = 0
  let messageDetailSeq = 0
  let composing = false
  let guardActive = false
  const messagePageSize = 200

  const accountScopes = $derived.by((): Scope[] => [
    { id: '', label: $_('backupViewer.scopeAll'), count: catalog?.messageCount ?? 0 },
    ...(catalog?.accounts ?? []).map((account) => ({
      id: account.accountEmail,
      label: account.accountEmail,
      count: account.messageCount,
    })),
  ])
  const selectedScope = $derived(accountScopes.find((scope) => scope.id === selectedAccountEmail) ?? accountScopes[0])
  const detailHeaderTitle = $derived(detail ? (detail.subject || $_('backupViewer.unknownSubject')) : '')
  const hasMoreMessages = $derived(messages.length < messagesTotal)
  const darkFilterStyle = $derived.by(() => {
    void getThemeMode()
    if (!darkFilterEnabled) return ''
    const styles = buildDarkMailFilterStyles()
    return `background-color: ${styles.surfaceBackground}; --backup-viewer-content-filter: ${styles.contentFilter}; --backup-viewer-media-filter: ${styles.mediaFilter};`
  })
  const visibleMessages = $derived(messages)
  const hasValidMessageSelection = $derived(
    visibleMessages.some((message) => message.key === selectedMessageKey),
  )

  $effect(() => {
    if (open) {
      void initialize()
    } else {
      resetViewerContent()
    }
    setGuardActive(open)
  })

  $effect(() => {
    if (!open) return
    window.addEventListener('keydown', onDialogKeydown, { capture: true })
    return () => window.removeEventListener('keydown', onDialogKeydown, { capture: true })
  })

  onDestroy(() => setGuardActive(false))

  function setGuardActive(active: boolean) {
    if (active === guardActive) return
    guardActive = active
    if (active) {
      dialogGuardOpen()
    } else {
      dialogGuardClose()
    }
  }

  function isEditableTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false
    const tag = target.tagName
    return tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable
  }

  function eventTargetsBackupViewer(event: KeyboardEvent): boolean {
    const target = event.target
    if (!(target instanceof Element)) return true
    if (target.closest('[data-backup-viewer-root], [data-backup-viewer-search]')) return true
    return searchOpen
  }

  function onDialogKeydown(event: KeyboardEvent) {
    if (!eventTargetsBackupViewer(event)) return
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'f') {
      if (!catalog?.messageCount) return
      event.preventDefault()
      event.stopPropagation()
      event.stopImmediatePropagation()
      if (searchOpen) {
        searchInputEl?.focus()
      } else {
        openSearch()
      }
      return
    }
    if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !isEditableTarget(event.target)) {
      if (!catalog?.messageCount) return
      event.preventDefault()
      event.stopPropagation()
      event.stopImmediatePropagation()
      openSearch()
      return
    }
    if (event.key !== 'Escape') return
    event.preventDefault()
    event.stopPropagation()
    event.stopImmediatePropagation()
    if (searchOpen) {
      closeSearch()
      return
    }
    if (directoryMenuOpen) {
      directoryMenuOpen = false
      return
    }
    closeDialog()
  }

  async function initialize() {
    resetViewerContent()
    darkFilterEnabled = getDarkMailContent()
    try {
      const settings = await GetBackupSettings()
      if (settings?.directory) {
        rememberBackupDirectory(settings.directory)
      }
    } catch (err) {
      console.warn('Failed to seed backup viewer directory history:', err)
    }
  }

  function resetViewerContent() {
    closeSearch()
    directoryMenuOpen = false
    directory = ''
    catalog = null
    messages = []
    messagesTotal = 0
    selectedAccountEmail = ''
    selectedMessageKey = ''
    messageDetailSeq += 1
    detail = null
    loadingCatalog = false
    loadingMessages = false
    loadingMoreMessages = false
    buildingIndex = false
    loadingDetail = false
    errorMessage = ''
    savingAttachmentIndexes = new Set()
    attachmentCountsByKey = {}
    attachmentsExpanded = true
    messageSortOrder = 'newest'
  }

  async function loadCatalog(dir: string, options: { fromHistory?: boolean; remember?: boolean } = {}) {
    if (!dir.trim()) return
    loadingCatalog = true
    errorMessage = ''
    catalog = null
    messages = []
    messagesTotal = 0
    detail = null
    selectedMessageKey = ''
    messageDetailSeq += 1
    selectedAccountEmail = ''
    directory = dir.trim()
    try {
      const result = await GetBackupViewerCatalog(dir)
      catalog = result
      directory = result?.directory || dir
      if (options.remember) {
        rememberBackupDirectory(directory)
      }
      const availableScopes = new Set(['', ...((result?.accounts ?? []).map((account) => account.accountEmail))])
      if (!availableScopes.has(selectedAccountEmail)) {
        selectedAccountEmail = ''
      }
      await loadMessagePage({ reset: true, selectFirst: true })
    } catch (err) {
      console.error('Failed to load backup catalog:', err)
      const failedPath = dir.trim()
      resetViewerContent()
      if (options.fromHistory) {
        removeBackupDirectory(failedPath)
        const message = $_('backupViewer.historyPathMissing', { values: { path: failedPath } })
        errorMessage = message
        toasts.error(message)
      } else {
        errorMessage = $_('backupViewer.loadFailed')
      }
    } finally {
      loadingCatalog = false
    }
  }

  function handleChooseDirectoryError() {
    errorMessage = $_('backupViewer.chooseDirectoryFailed')
  }

  function handleRemoveDirectoryHistory(path: string) {
    if (directory === path) {
      clearDirectory()
    }
  }

  async function openDirectory() {
    if (!directory.trim()) return
    try {
      await OpenBackupViewerDirectory(directory)
    } catch (err) {
      console.error('Failed to open backup directory:', err)
      errorMessage = $_('backupViewer.openDirectoryFailed')
    }
  }

  async function refreshCatalog() {
    if (!directory.trim()) return
    await loadCatalog(directory)
  }

  async function buildViewerIndex() {
    if (!directory.trim() || buildingIndex) return
    buildingIndex = true
    errorMessage = ''
    try {
      await BuildBackupViewerIndex(directory)
      await loadCatalog(directory)
    } catch (err) {
      console.error('Failed to build backup viewer index:', err)
      errorMessage = $_('backupViewer.buildIndexFailed')
    } finally {
      buildingIndex = false
    }
  }

  async function selectScope(scopeID: string) {
    if (selectedAccountEmail === scopeID) return
    selectedAccountEmail = scopeID
    await loadMessagePage({ reset: true, selectFirst: true })
  }

  function clearDirectory() {
    resetViewerContent()
  }

  async function loadMessagePage(options: { reset?: boolean; selectFirst?: boolean } = {}) {
    if (!directory.trim() || !catalog?.messageCount) {
      messages = []
      messagesTotal = 0
      return
    }

    const reset = options.reset ?? false
    const offset = reset ? 0 : messages.length
    if (reset) {
      loadingMessages = true
      messages = []
      messagesTotal = 0
      selectedMessageKey = ''
      messageDetailSeq += 1
      detail = null
    } else {
      loadingMoreMessages = true
    }
    errorMessage = ''

    try {
      const page = await ListBackupViewerMessages(directory, selectedAccountEmail, messageSortOrder, offset, messagePageSize)
      const nextMessages = page?.messages ?? []
      messages = reset ? nextMessages : [...messages, ...nextMessages]
      messagesTotal = page?.total ?? messages.length
      if (options.selectFirst) {
        await selectFirstVisibleMessage()
      }
    } catch (err) {
      console.error('Failed to load backup message page:', err)
      errorMessage = $_('backupViewer.loadFailed')
    } finally {
      loadingMessages = false
      loadingMoreMessages = false
    }
  }

  async function selectFirstVisibleMessage() {
    await tick()
    const current = visibleMessages.find((message) => message.key === selectedMessageKey)
    if (current) return
    const first = visibleMessages[0]
    if (!first) {
      selectedMessageKey = ''
      messageDetailSeq += 1
      detail = null
      return
    }
    await selectMessage(first.key)
  }

  async function selectMessage(key: string) {
    if (!directory.trim() || !key) return
    selectedMessageKey = key
    const seq = ++messageDetailSeq
    loadingDetail = true
    errorMessage = ''
    try {
      const loadedDetail = await GetBackupViewerMessage(directory, key)
      if (seq !== messageDetailSeq || selectedMessageKey !== key) return
      detail = loadedDetail
      errorMessage = ''
      attachmentsExpanded = true
      updateMessageAttachmentCount(key, loadedDetail?.attachments?.length ?? 0)
    } catch (err) {
      if (seq !== messageDetailSeq || selectedMessageKey !== key) return
      console.error('Failed to load backup message:', err)
      detail = null
      const reason = describeError(err)
      errorMessage = reason ? `${$_('backupViewer.messageLoadFailed')}: ${reason}` : $_('backupViewer.messageLoadFailed')
    } finally {
      if (seq === messageDetailSeq) loadingDetail = false
    }
  }

  function updateMessageAttachmentCount(key: string, count: number) {
    if (!key) return
    const attachmentCount = Math.max(0, count)
    attachmentCountsByKey = { ...attachmentCountsByKey, [key]: attachmentCount }
  }

  function messageAttachmentCount(message: app.BackupViewerMessageSummary): number {
    return attachmentCountsByKey[message.key] ?? message.attachmentCount ?? 0
  }

  function scrollMessageListToTop() {
    messageListEl?.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function scrollMessageIntoView(
    key: string,
    block: 'start' | 'center' | 'end' | 'nearest' = 'nearest',
  ) {
    if (!messageListEl || !key) return
    const row = [...messageListEl.querySelectorAll<HTMLElement>('[data-backup-message-key]')]
      .find((element) => element.dataset.backupMessageKey === key)
    row?.scrollIntoView({ block, behavior: 'smooth' })
  }

  function focusMessageRow(key: string) {
    if (!messageListEl || !key) return
    const row = [...messageListEl.querySelectorAll<HTMLElement>('[data-backup-message-key]')]
      .find((element) => element.dataset.backupMessageKey === key)
    row?.focus({ preventScroll: true })
  }

  async function moveMessageSelection(key: RovingNavigationKey) {
    if (visibleMessages.length === 0) return
    const currentIndex = visibleMessages.findIndex((message) => message.key === selectedMessageKey)
    const nextIndex = nextRovingIndex(key, currentIndex, visibleMessages.length)
    const next = visibleMessages[nextIndex]
    if (!next) return
    void selectMessage(next.key)
    await tick()
    scrollMessageIntoView(next.key)
    focusMessageRow(next.key)
  }

  function handleMessageListFocus() {
    if (hasValidMessageSelection || visibleMessages.length === 0) return
    const first = visibleMessages[0]
    if (!first) return
    void selectMessage(first.key)
    void tick().then(() => focusMessageRow(first.key))
  }

  function handleMessageListKeydown(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) return
    if (['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) {
      event.preventDefault()
      event.stopPropagation()
      void moveMessageSelection(event.key as RovingNavigationKey)
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      event.stopPropagation()
      if (selectedMessageKey) void selectMessage(selectedMessageKey)
    }
  }

  function toggleMessageSortOrder() {
    messageSortOrder = messageSortOrder === 'newest' ? 'oldest' : 'newest'
    scrollMessageListToTop()
    void loadMessagePage({ reset: true, selectFirst: true })
  }

  function describeError(err: unknown): string {
    if (!err) return ''
    if (typeof err === 'string') return err
    if (err instanceof Error) return err.message
    if (typeof err === 'object' && 'message' in err) {
      const message = (err as { message?: unknown }).message
      return typeof message === 'string' ? message : ''
    }
    return String(err)
  }

  function closeDialog() {
    resetViewerContent()
    open = false
    setGuardActive(false)
    onClose?.()
  }

  async function saveBackupAttachment(attachment: app.BackupViewerAttachment, fallbackIndex: number) {
    if (!directory.trim() || !detail?.key) return
    const index = typeof attachment.index === 'number' ? attachment.index : fallbackIndex
    savingAttachmentIndexes = new Set([...savingAttachmentIndexes, index])
    try {
      const path = await SaveBackupViewerAttachmentAs(directory, detail.key, index)
      if (path) {
        toasts.success($_('toast.attachmentSaved', { values: { filename: attachment.filename } }))
      }
    } catch (err) {
      console.error('Failed to save backup attachment:', err)
      errorMessage = $_('backupViewer.attachmentSaveFailed')
      toasts.error($_('toast.failedToSaveAttachment', { values: { filename: attachment.filename } }))
    } finally {
      savingAttachmentIndexes = new Set([...savingAttachmentIndexes].filter((value) => value !== index))
    }
  }

  function openSearch() {
    searchOpen = true
    searchQuery = ''
    searchResults = []
    searchActiveIndex = 0
    searchScopeEmail = selectedAccountEmail
    setTimeout(() => searchInputEl?.focus(), 30)
  }

  function closeSearch() {
    searchOpen = false
    searchQuery = ''
    searchResults = []
    searchLoading = false
    composing = false
    searchDebouncer.cancel()
  }

  function runSearch() {
    const query = searchQuery.trim()
    if (!directory.trim() || !query) {
      searchResults = []
      searchLoading = false
      return
    }
    searchLoading = true
    const seq = ++searchSeq
    SearchBackupViewerMessages(directory, searchScopeEmail, query, 0, 50)
      .then((page: app.BackupViewerMessagePage) => {
        if (seq !== searchSeq) return
        searchResults = page?.messages || []
        searchActiveIndex = 0
      })
      .catch((err: unknown) => {
        if (seq !== searchSeq) return
        console.error('Backup viewer search failed:', err)
        searchResults = []
      })
      .finally(() => {
        if (seq === searchSeq) searchLoading = false
      })
  }

  function onSearchInput() {
    if (composing) return
    searchDebouncer.schedule(runSearch)
  }

  function onCompositionStart() {
    composing = true
  }

  function onCompositionEnd() {
    composing = false
    searchDebouncer.schedule(runSearch)
  }

  function selectSearchScope(scopeID: string) {
    searchScopeEmail = scopeID
    searchActiveIndex = 0
    searchDebouncer.cancel()
    runSearch()
    setTimeout(() => searchInputEl?.focus(), 0)
  }

  function moveSearchScope(delta: number) {
    if (accountScopes.length === 0) return
    const currentIndex = Math.max(0, accountScopes.findIndex((scope) => scope.id === searchScopeEmail))
    const nextIndex = (currentIndex + delta + accountScopes.length) % accountScopes.length
    selectSearchScope(accountScopes[nextIndex].id)
  }

	  async function selectSearchResult(index: number) {
	    const message = searchResults[index]
	    if (!message) return
	    const accountChanged = selectedAccountEmail !== message.accountEmail
	    selectedAccountEmail = message.accountEmail
	    closeSearch()
	    if (accountChanged) {
	      await loadMessagePage({ reset: true })
	    }
	    if (!messages.some((item) => item.key === message.key)) {
	      messages = [message, ...messages]
	      messagesTotal = Math.max(messagesTotal, messages.length)
	    }
	    await tick()
	    await selectMessage(message.key)
	    await tick()
	    scrollMessageIntoView(message.key, 'start')
	  }

  function onSearchKeydown(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) return
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      closeSearch()
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      event.stopPropagation()
      searchActiveIndex = Math.min(searchActiveIndex + 1, Math.max(0, searchResults.length - 1))
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      event.stopPropagation()
      searchActiveIndex = Math.max(searchActiveIndex - 1, 0)
    } else if (event.key === 'Tab') {
      event.preventDefault()
      event.stopPropagation()
      moveSearchScope(event.shiftKey ? -1 : 1)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      event.stopPropagation()
      void selectSearchResult(searchActiveIndex)
    } else if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'a') {
      event.preventDefault()
      event.stopPropagation()
      searchInputEl?.select()
    }
  }

  function formatDate(value: string): string {
    if (!value) return ''
    const date = parseFlexibleDate(value)
    if (!date) return value
    return formatLocalDateTime(date)
  }

  function formatShortDate(value: string): string {
    if (!value) return ''
    const date = parseFlexibleDate(value)
    if (!date) return value
    return formatLocalDateTimeShort(date)
  }

  function scopeLabel(scope: Scope | undefined): string {
    return scope?.label ?? $_('backupViewer.scopeAll')
  }
</script>

{#if open}
  <ModalFrame
    {open}
    onClose={closeDialog}
    containerClass="z-[90] flex items-center justify-center p-5"
    panelClass="flex h-[min(86vh,760px)] w-[min(94vw,1180px)] flex-col overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-2xl"
  >
      <div data-backup-viewer-root class="contents">
      <BackupViewerToolbar
        {directory}
        bind:directoryMenuOpen
        {catalog}
        {selectedAccountEmail}
        {selectedScope}
        {accountScopes}
        {loadingCatalog}
        {buildingIndex}
        {errorMessage}
        bind:darkFilterEnabled
        {messageSortOrder}
        onLoadCatalog={loadCatalog}
        onChooseDirectoryError={handleChooseDirectoryError}
        onRemoveDirectoryHistory={handleRemoveDirectoryHistory}
        onOpenDirectory={openDirectory}
        onSelectScope={selectScope}
        onScrollToTop={scrollMessageListToTop}
        onClearDirectory={clearDirectory}
        onRefreshCatalog={refreshCatalog}
        onOpenSearch={openSearch}
        onBuildIndex={buildViewerIndex}
        onToggleSortOrder={toggleMessageSortOrder}
        onClose={closeDialog}
        {scopeLabel}
      />

      <div class="grid min-h-0 flex-1 overflow-hidden grid-cols-[42%_1fr]">
        <section class="flex min-h-0 flex-col border-r border-border">
          <div class="flex h-10 shrink-0 items-center bg-muted/20 px-4">
            <span class="min-w-0 truncate text-sm font-semibold text-foreground" title={scopeLabel(selectedScope)}>
              {scopeLabel(selectedScope)}
            </span>
          </div>
          {#if loadingCatalog || loadingMessages}
            <div class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
              <Icon icon="mdi:loading" class="mr-2 animate-spin" width="18" height="18" />
              {$_('backupViewer.loading')}
            </div>
          {:else if !directory}
            <div class="flex min-h-0 flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">{$_('backupViewer.noDirectory')}</div>
          {:else if catalog && catalog.messageCount === 0}
            <div class="flex min-h-0 flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">{$_('backupViewer.noBackup')}</div>
          {:else if visibleMessages.length === 0}
            <div class="flex min-h-0 flex-1 items-center justify-center px-6 text-center text-sm text-muted-foreground">{$_('backupViewer.noMessages')}</div>
          {:else}
            <div class="flex min-h-0 flex-1 flex-col">
              <div
                bind:this={messageListEl}
                role="listbox"
                aria-label={$_('backupViewer.title')}
                tabindex={hasValidMessageSelection ? -1 : 0}
                class="min-h-0 flex-1 overflow-y-auto scrollbar-thin outline-none"
                onfocus={handleMessageListFocus}
                onkeydown={handleMessageListKeydown}
              >
                {#each visibleMessages as message (message.key)}
                  {@const hasAttachments = messageAttachmentCount(message) > 0}
                  <button
                    type="button"
                    data-backup-message-key={message.key}
                    role="option"
                    aria-selected={selectedMessageKey === message.key}
                    tabindex={selectedMessageKey === message.key ? 0 : -1}
                    class="relative grid w-full grid-cols-[1rem_minmax(0,1fr)_auto] items-start gap-x-3 border-b border-border py-3 pl-4 pr-6 text-left transition-colors {selectedMessageKey === message.key ? 'keyboard-selected-item bg-primary/15' : 'hover:bg-muted/40'}"
                    onclick={() => selectMessage(message.key)}
                  >
                    <Icon icon="mdi:email-outline" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-sm font-semibold text-foreground">{message.subject || $_('backupViewer.unknownSubject')}</span>
                      <span class="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                        <span class="truncate">{message.accountEmail}</span>
                        {#if message.folderPath}
                          <span class="shrink-0">/</span>
                          <span class="truncate">{message.folderPath}</span>
                        {/if}
                      </span>
                    </span>
                    <time datetime={message.date} class="shrink-0 whitespace-nowrap text-right text-xs tabular-nums text-muted-foreground">{formatShortDate(message.date)}</time>
                    <span class="pointer-events-none absolute bottom-0 left-0 top-0 w-[5px] {hasAttachments ? 'bg-amber-500' : 'bg-transparent'}" aria-hidden="true"></span>
                  </button>
                {/each}
              </div>
              <div class="flex h-11 shrink-0 items-center justify-between gap-3 border-t border-border bg-muted/20 px-4 text-xs text-muted-foreground">
                <span class="min-w-0 truncate">
                  {$_('backupViewer.loadedMessages', { values: { loaded: visibleMessages.length, total: messagesTotal } })}
                </span>
                {#if hasMoreMessages}
                  <button
                    type="button"
                    class="inline-flex h-8 shrink-0 items-center rounded-md px-3 text-sm font-medium text-foreground transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={loadingMoreMessages}
                    onclick={() => loadMessagePage()}
                  >
                    {#if loadingMoreMessages}
                      <Icon icon="mdi:loading" class="mr-2 h-4 w-4 animate-spin" />
                    {/if}
                    {$_('backupViewer.loadMore')}
                  </button>
                {:else}
                  <span class="shrink-0">{$_('backupViewer.allLoaded')}</span>
                {/if}
              </div>
            </div>
          {/if}
        </section>

        <BackupViewerMessageDetail
          {detail}
          {loadingDetail}
          {detailHeaderTitle}
          bind:attachmentsExpanded
          {savingAttachmentIndexes}
          {darkFilterStyle}
          {darkFilterEnabled}
          onSaveAttachment={saveBackupAttachment}
          {formatDate}
        />
      </div>
      </div>
  </ModalFrame>

  <BackupViewerSearchOverlay
    open={searchOpen}
    {accountScopes}
    {searchScopeEmail}
    bind:searchQuery
    {searchResults}
    {searchLoading}
    bind:searchActiveIndex
    bind:searchInputEl
    onClose={closeSearch}
    onSelectSearchScope={selectSearchScope}
    onSearchInput={onSearchInput}
    onCompositionStart={onCompositionStart}
    onCompositionEnd={onCompositionEnd}
    onSearchKeydown={onSearchKeydown}
    onSelectSearchResult={selectSearchResult}
    {messageAttachmentCount}
    {formatShortDate}
  />
{/if}
