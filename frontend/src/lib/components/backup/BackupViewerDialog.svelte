<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ModalFrame from '$lib/components/ui/ModalFrame.svelte'
  import BackupViewerMessageDetail from '$lib/components/backup/BackupViewerMessageDetail.svelte'
  import BackupViewerSearchOverlay from '$lib/components/backup/BackupViewerSearchOverlay.svelte'
  import BackupViewerToolbar from '$lib/components/backup/BackupViewerToolbar.svelte'
  // @ts-ignore - wailsjs path
  import {
    GetBackupSettings,
    GetBackupViewerCatalog,
    GetBackupViewerMessage,
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
  let selectedAccountEmail = $state('')
  let selectedMessageKey = $state('')
  let detail = $state<app.BackupViewerMessageDetail | null>(null)
  let loadingCatalog = $state(false)
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
  let composing = false

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
  const darkFilterStyle = $derived.by(() => {
    void getThemeMode()
    if (!darkFilterEnabled) return ''
    const styles = buildDarkMailFilterStyles()
    return `background-color: ${styles.surfaceBackground}; --backup-viewer-content-filter: ${styles.contentFilter}; --backup-viewer-media-filter: ${styles.mediaFilter};`
  })
  const visibleMessages = $derived.by(() => {
    const source = catalog?.messages ?? []
    const filtered = selectedAccountEmail
      ? source.filter((message) => message.accountEmail === selectedAccountEmail)
      : source
    return [...filtered].sort(compareMessagesByDate)
  })

  $effect(() => {
    if (open) {
      void initialize()
    } else {
      resetViewerContent()
    }
  })

  $effect(() => {
    if (!open) return
    window.addEventListener('keydown', onDialogKeydown, { capture: true })
    return () => window.removeEventListener('keydown', onDialogKeydown, { capture: true })
  })

  function onDialogKeydown(event: KeyboardEvent) {
    if (event.key !== 'Escape') return
    event.preventDefault()
    event.stopPropagation()
    event.stopImmediatePropagation()
    if (directoryMenuOpen) {
      directoryMenuOpen = false
      return
    }
    if (searchOpen) {
      closeSearch()
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
    selectedAccountEmail = ''
    selectedMessageKey = ''
    detail = null
    loadingCatalog = false
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
    detail = null
    selectedMessageKey = ''
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
      await selectFirstVisibleMessage()
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

  async function selectScope(scopeID: string) {
    selectedAccountEmail = scopeID
    await selectFirstVisibleMessage()
  }

  function clearDirectory() {
    resetViewerContent()
  }

  async function selectFirstVisibleMessage() {
    await tick()
    const current = visibleMessages.find((message) => message.key === selectedMessageKey)
    if (current) return
    const first = visibleMessages[0]
    if (!first) {
      selectedMessageKey = ''
      detail = null
      return
    }
    await selectMessage(first.key)
  }

  async function selectMessage(key: string) {
    if (!directory.trim() || !key) return
    selectedMessageKey = key
    loadingDetail = true
    errorMessage = ''
    try {
      const loadedDetail = await GetBackupViewerMessage(directory, key)
      detail = loadedDetail
      errorMessage = ''
      attachmentsExpanded = true
      updateMessageAttachmentCount(key, loadedDetail?.attachments?.length ?? 0)
    } catch (err) {
      console.error('Failed to load backup message:', err)
      detail = null
      const reason = describeError(err)
      errorMessage = reason ? `${$_('backupViewer.messageLoadFailed')}: ${reason}` : $_('backupViewer.messageLoadFailed')
    } finally {
      loadingDetail = false
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

  function compareMessagesByDate(left: app.BackupViewerMessageSummary, right: app.BackupViewerMessageSummary): number {
    const leftTime = parseFlexibleDate(left.date)?.getTime() ?? 0
    const rightTime = parseFlexibleDate(right.date)?.getTime() ?? 0
    if (leftTime !== rightTime) {
      return messageSortOrder === 'newest' ? rightTime - leftTime : leftTime - rightTime
    }
    return messageSortOrder === 'newest'
      ? right.key.localeCompare(left.key)
      : left.key.localeCompare(right.key)
  }

  function scrollMessageListToTop() {
    messageListEl?.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function scrollMessageIntoView(key: string) {
    if (!messageListEl || !key) return
    const row = [...messageListEl.querySelectorAll<HTMLElement>('[data-backup-message-key]')]
      .find((element) => element.dataset.backupMessageKey === key)
    row?.scrollIntoView({ block: 'start', behavior: 'smooth' })
  }

  function toggleMessageSortOrder() {
    messageSortOrder = messageSortOrder === 'newest' ? 'oldest' : 'newest'
    scrollMessageListToTop()
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
    SearchBackupViewerMessages(directory, searchScopeEmail, query, 50)
      .then((result: app.BackupViewerMessageSummary[]) => {
        if (seq !== searchSeq) return
        searchResults = result || []
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
    selectedAccountEmail = message.accountEmail
    closeSearch()
    await tick()
    await selectMessage(message.key)
    await tick()
    scrollMessageIntoView(message.key)
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
      <BackupViewerToolbar
        {directory}
        bind:directoryMenuOpen
        {catalog}
        {selectedAccountEmail}
        {selectedScope}
        {accountScopes}
        {loadingCatalog}
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
          {#if loadingCatalog}
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
            <div bind:this={messageListEl} class="min-h-0 flex-1 overflow-y-auto scrollbar-thin">
              {#each visibleMessages as message (message.key)}
                {@const hasAttachments = messageAttachmentCount(message) > 0}
                <button
                  type="button"
                  data-backup-message-key={message.key}
                  class="relative flex w-full items-start gap-3 border-b border-border py-3 pl-4 pr-6 text-left transition-colors {selectedMessageKey === message.key ? 'bg-primary/15' : 'hover:bg-muted/40'}"
                  onclick={() => selectMessage(message.key)}
                >
                  <Icon icon="mdi:email-outline" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-2">
                      <span class="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{message.subject || $_('backupViewer.unknownSubject')}</span>
                      {#if hasAttachments}
                        <span class="shrink-0 text-primary" title={$_('backupViewer.attachments')}>
                          <Icon icon="mdi:paperclip" class="h-4 w-4" />
                        </span>
                      {/if}
                      <span class="w-[96px] shrink-0 text-right text-xs tabular-nums text-muted-foreground">{formatShortDate(message.date)}</span>
                    </span>
                    <span class="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                      <span class="truncate">{message.accountEmail}</span>
                      {#if message.folderPath}
                        <span class="shrink-0">/</span>
                        <span class="truncate">{message.folderPath}</span>
                      {/if}
                    </span>
                  </span>
                  <span class="pointer-events-none absolute bottom-0 right-0 top-0 w-[5px] {hasAttachments ? 'bg-amber-500' : 'bg-transparent'}" aria-hidden="true"></span>
                </button>
              {/each}
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
