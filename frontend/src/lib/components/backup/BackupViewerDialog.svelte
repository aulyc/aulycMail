<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import * as Select from '$lib/components/ui/select'
  import BackupDirectoryPicker from '$lib/components/backup/BackupDirectoryPicker.svelte'
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
  import { getThemeMode } from '$lib/stores/settings.svelte'
  import { buildDarkMailFilterStyles } from '$lib/utils/dark-mail'
  import { formatFileSize } from '$lib/utils/fileSize'
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

  let searchOpen = $state(false)
  let searchQuery = $state('')
  let searchResults = $state<app.BackupViewerMessageSummary[]>([])
  let searchLoading = $state(false)
  let searchActiveIndex = $state(0)
  let searchScopeEmail = $state('')
  let searchInputEl = $state<HTMLInputElement | null>(null)
  let searchScopeStripEl = $state<HTMLDivElement | null>(null)
  let searchDebounce: ReturnType<typeof setTimeout> | null = null
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
  const searchScopeIndex = $derived(Math.max(0, accountScopes.findIndex((scope) => scope.id === searchScopeEmail)))
  const darkFilterStyle = $derived.by(() => {
    void getThemeMode()
    if (!darkFilterEnabled) return ''
    const styles = buildDarkMailFilterStyles()
    return `background-color: ${styles.surfaceBackground}; --backup-viewer-content-filter: ${styles.contentFilter}; --backup-viewer-media-filter: ${styles.mediaFilter};`
  })
  const visibleMessages = $derived.by(() => {
    const source = catalog?.messages ?? []
    if (!selectedAccountEmail) return source
    return source.filter((message) => message.accountEmail === selectedAccountEmail)
  })

  $effect(() => {
    if (open) {
      void initialize()
    } else {
      resetViewerContent()
    }
  })

  $effect(() => {
    if (!searchOpen) return
    void centerScope(searchScopeStripEl, searchScopeIndex)
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
    try {
      const loadedDetail = await GetBackupViewerMessage(directory, key)
      detail = loadedDetail
      updateMessageAttachmentCount(key, loadedDetail?.attachments?.length ?? 0)
    } catch (err) {
      console.error('Failed to load backup message:', err)
      detail = null
      errorMessage = $_('backupViewer.messageLoadFailed')
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
    if (searchDebounce) {
      clearTimeout(searchDebounce)
      searchDebounce = null
    }
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
    if (searchDebounce) clearTimeout(searchDebounce)
    searchDebounce = setTimeout(runSearch, 200)
  }

  function onCompositionStart() {
    composing = true
  }

  function onCompositionEnd() {
    composing = false
    if (searchDebounce) clearTimeout(searchDebounce)
    searchDebounce = setTimeout(runSearch, 200)
  }

  function selectSearchScope(scopeID: string) {
    searchScopeEmail = scopeID
    searchActiveIndex = 0
    void centerScope(searchScopeStripEl, searchScopeIndex)
    if (searchDebounce) clearTimeout(searchDebounce)
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
    await selectMessage(message.key)
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

  async function centerScope(strip: HTMLDivElement | null, scopeIndex: number) {
    await tick()
    await nextAnimationFrame()
    if (!strip) return
    const button = strip.querySelector<HTMLElement>(`[data-scope-index="${scopeIndex}"]`)
    if (!button) return
    const stripRect = strip.getBoundingClientRect()
    const buttonRect = button.getBoundingClientRect()
    const buttonCenter = buttonRect.left - stripRect.left + strip.scrollLeft + buttonRect.width / 2
    const targetLeft = buttonCenter - strip.clientWidth / 2
    const maxLeft = Math.max(0, strip.scrollWidth - strip.clientWidth)
    strip.scrollTo({ left: Math.min(Math.max(0, targetLeft), maxLeft), behavior: 'auto' })
  }

  function nextAnimationFrame(): Promise<void> {
    return new Promise((resolve) => requestAnimationFrame(() => resolve()))
  }

  function parseViewerDate(value: string): Date | null {
    if (!value) return null
    const trimmed = value.trim()
    const goTimeMatch = trimmed.match(/^(\d{4}-\d{2}-\d{2})\s+(\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+([+-]\d{4})\s+\S+$/)
    const normalized = goTimeMatch
      ? `${goTimeMatch[1]}T${goTimeMatch[2]}${goTimeMatch[3].slice(0, 3)}:${goTimeMatch[3].slice(3)}`
      : trimmed
    const date = new Date(normalized)
    return Number.isNaN(date.getTime()) ? null : date
  }

  function formatDate(value: string): string {
    if (!value) return ''
    const date = parseViewerDate(value)
    if (!date) return value
    return date.toLocaleString()
  }

  function formatShortDate(value: string): string {
    if (!value) return ''
    const date = parseViewerDate(value)
    if (!date) return value
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
  }

  function scopeLabel(scope: Scope | undefined): string {
    return scope?.label ?? $_('backupViewer.scopeAll')
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[90] flex items-center justify-center bg-black/80 p-5" onclick={closeDialog}>
    <div
      class="flex h-[min(86vh,760px)] w-[min(94vw,1180px)] flex-col overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-2xl"
      onclick={(event) => event.stopPropagation()}
    >
      <header class="flex items-center justify-between border-b border-border px-5 py-4">
        <h2 class="text-lg font-semibold">{$_('backupViewer.title')}</h2>
        <button
          type="button"
          class="rounded p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
          aria-label={$_('common.close')}
          onclick={closeDialog}
        >
          <Icon icon="mdi:close" width="20" height="20" />
        </button>
      </header>

      <div class="relative z-20 flex h-14 shrink-0 items-center gap-2 overflow-visible border-b border-border px-4">
        <div class="w-[340px] shrink-0">
          <BackupDirectoryPicker
            bind:menuOpen={directoryMenuOpen}
            {directory}
            placeholder={$_('backupViewer.directoryPlaceholder')}
            onChoose={(path) => loadCatalog(path, { remember: true })}
            onChooseError={handleChooseDirectoryError}
            onSelectHistory={(path) => loadCatalog(path, { fromHistory: true, remember: true })}
            onRemoveHistory={handleRemoveDirectoryHistory}
            onOpenDirectory={() => openDirectory()}
          />
        </div>

        <Select.Root
          value={selectedAccountEmail}
          onValueChange={(value) => void selectScope(value)}
          disabled={!catalog?.messageCount}
        >
          <Select.Trigger class="h-10 w-[300px] shrink-0 border-border bg-background px-3 py-2 text-sm font-semibold shadow-none hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-0">
            <Select.Value class="min-w-0 flex-1" placeholder={$_('backupViewer.scopeAll')}>
              <span class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-4">
                <span class="truncate">{scopeLabel(selectedScope)}</span>
                {#if typeof selectedScope?.count === 'number'}
                  <span class="shrink-0 tabular-nums text-muted-foreground">{selectedScope.count}</span>
                {/if}
              </span>
            </Select.Value>
          </Select.Trigger>
          <Select.Content class="z-[130] w-[300px]">
            {#each accountScopes as scope (scope.id || 'all')}
              <Select.Item value={scope.id} label={scope.label} class="pr-3">
                <span class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-4">
                  <span class="truncate">{scope.label}</span>
                  {#if typeof scope.count === 'number'}
                    <span class="shrink-0 tabular-nums text-muted-foreground">{scope.count}</span>
                  {/if}
                </span>
              </Select.Item>
            {/each}
          </Select.Content>
        </Select.Root>

        <div class="flex shrink-0 items-center gap-0.5" role="toolbar" aria-label={$_('backupViewer.title')}>
          <button
            type="button"
            class="rounded-md p-1.5 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!directory}
            title={$_('backupViewer.clearDirectory')}
            aria-label={$_('backupViewer.clearDirectory')}
            onclick={clearDirectory}
          >
            <Icon icon="mdi:folder-remove-outline" class="h-5 w-5 text-muted-foreground" />
          </button>
          <button
            type="button"
            class="rounded-md p-1.5 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!directory || loadingCatalog}
            title={$_('backupViewer.refresh')}
            aria-label={$_('backupViewer.refresh')}
            onclick={refreshCatalog}
          >
            <Icon icon="mdi:refresh" class="h-5 w-5 text-muted-foreground {loadingCatalog ? 'animate-spin' : ''}" />
          </button>
          <button
            type="button"
            class="rounded-md p-1.5 transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
            disabled={!catalog?.messageCount}
            title={$_('backupViewer.search')}
            aria-label={$_('backupViewer.search')}
            onclick={openSearch}
          >
            <Icon icon="mdi:magnify" class="h-5 w-5 text-muted-foreground" />
          </button>
          <button
            type="button"
            class="rounded-md p-1.5 transition-colors hover:bg-muted"
            aria-pressed={darkFilterEnabled}
            aria-label={$_('backupViewer.darkFilter')}
            title={$_('backupViewer.darkFilter')}
            onclick={() => darkFilterEnabled = !darkFilterEnabled}
          >
            <Icon icon={darkFilterEnabled ? 'mdi:weather-night' : 'mdi:white-balance-sunny'} class="h-5 w-5 text-muted-foreground" />
          </button>
        </div>

        {#if errorMessage}
          <span class="max-w-[220px] shrink truncate text-sm text-destructive" title={errorMessage}>{errorMessage}</span>
        {/if}

        <span class="min-w-0 flex-1" aria-hidden="true"></span>
      </div>

      <div class="grid min-h-0 flex-1 overflow-hidden grid-cols-[42%_1fr]">
        <section class="flex min-h-0 flex-col border-r border-border">
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
            <div class="min-h-0 flex-1 overflow-y-auto scrollbar-thin">
              {#each visibleMessages as message (message.key)}
                <button
                  type="button"
                  class="flex w-full items-start gap-3 border-b border-border px-4 py-3 text-left transition-colors {selectedMessageKey === message.key ? 'bg-primary/15' : 'hover:bg-muted/40'}"
                  onclick={() => selectMessage(message.key)}
                >
                  <Icon icon="mdi:email-outline" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="min-w-0 flex-1">
                    <span class="flex items-baseline gap-2">
                      <span class="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{message.subject || $_('backupViewer.unknownSubject')}</span>
                      {#if messageAttachmentCount(message) > 0}
                        <span class="shrink-0 text-muted-foreground" title={$_('backupViewer.attachments')}>
                          <Icon icon="mdi:paperclip" class="h-3.5 w-3.5" />
                        </span>
                      {/if}
                      <span class="shrink-0 text-xs text-muted-foreground">{formatShortDate(message.date)}</span>
                    </span>
                    <span class="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                      <span class="truncate">{message.accountEmail}</span>
                      {#if message.folderPath}
                        <span class="shrink-0">/</span>
                        <span class="truncate">{message.folderPath}</span>
                      {/if}
                    </span>
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </section>

        <section class="flex min-h-0 flex-col overflow-hidden">
          {#if loadingDetail}
            <div class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
              <Icon icon="mdi:loading" class="mr-2 animate-spin" width="18" height="18" />
              {$_('backupViewer.loading')}
            </div>
          {:else if !detail}
            <div class="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 text-muted-foreground">
              <Icon icon="mdi:email-open-outline" width="42" height="42" />
              <p class="text-sm">{$_('backupViewer.selectMessage')}</p>
            </div>
          {:else}
            <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5 scrollbar-thin">
              <div class="mb-5 space-y-1 rounded-md border border-border bg-muted/20 p-3 text-sm">
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.subject')}</span>
                  <span class="min-w-0 break-words font-semibold">{detailHeaderTitle}</span>
                </div>
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.from')}</span>
                  <span class="min-w-0 break-words">{detail.from?.join(', ') || '-'}</span>
                </div>
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.to')}</span>
                  <span class="min-w-0 break-words">{detail.to?.join(', ') || '-'}</span>
                </div>
                {#if detail.cc?.length}
                  <div class="grid grid-cols-[64px_1fr] gap-2">
                    <span class="text-muted-foreground">{$_('backupViewer.cc')}</span>
                    <span class="min-w-0 break-words">{detail.cc.join(', ')}</span>
                  </div>
                {/if}
                {#if detail.bcc?.length}
                  <div class="grid grid-cols-[64px_1fr] gap-2">
                    <span class="text-muted-foreground">{$_('backupViewer.bcc')}</span>
                    <span class="min-w-0 break-words">{detail.bcc.join(', ')}</span>
                  </div>
                {/if}
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.date')}</span>
                  <span>{formatDate(detail.date)}</span>
                </div>
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.folder')}</span>
                  <span>{detail.accountEmail}{detail.folderPath ? ` / ${detail.folderPath}` : ''}</span>
                </div>
                <div class="grid grid-cols-[64px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.size')}</span>
                  <span>{formatFileSize(detail.size)}</span>
                </div>
              </div>

              {#if detail.attachments?.length}
                <div class="mb-5">
                  <h3 class="mb-2 text-sm font-semibold">{$_('backupViewer.attachments')}</h3>
                  <div class="space-y-2">
                    {#each detail.attachments as attachment, index (attachment.filename + '-' + index)}
                      {@const attachmentIndex = typeof attachment.index === 'number' ? attachment.index : index}
                      {@const isSavingAttachment = savingAttachmentIndexes.has(attachmentIndex)}
                      <button
                        type="button"
                        class="flex w-full items-center gap-3 rounded-md border border-border bg-muted/20 px-3 py-2 text-left transition hover:bg-muted/40 disabled:cursor-wait disabled:opacity-70"
                        disabled={isSavingAttachment}
                        title={$_('attachment.download')}
                        onclick={() => saveBackupAttachment(attachment, index)}
                      >
                        {#if isSavingAttachment}
                          <Icon icon="mdi:loading" class="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
                        {:else}
                          <Icon icon="mdi:paperclip" class="h-4 w-4 shrink-0 text-muted-foreground" />
                        {/if}
                        <span class="min-w-0 flex-1 truncate text-sm">{attachment.filename}</span>
                        <span class="text-xs text-muted-foreground">{attachment.contentType}</span>
                        <span class="text-xs text-muted-foreground">{formatFileSize(attachment.size)}</span>
                        <Icon icon="mdi:download" class="h-4 w-4 shrink-0 text-muted-foreground" />
                      </button>
                    {/each}
                  </div>
                </div>
              {/if}

              <div class="backup-viewer-body rounded-md border border-border bg-background p-4" style={darkFilterStyle}>
                <div class="backup-viewer-mail-content {darkFilterEnabled ? 'backup-viewer-dark-filter' : ''}">
                  {#if detail.hasHTML}
                    <!-- eslint-disable-next-line svelte/no-at-html-tags -- backup viewer HTML is sanitized in Go before it reaches the UI -->
                    {@html detail.bodyHTML}
                  {:else if detail.bodyText}
                    <pre class="whitespace-pre-wrap break-words font-sans text-sm leading-6">{detail.bodyText}</pre>
                  {:else}
                    <p class="text-sm text-muted-foreground">{$_('backupViewer.noBody')}</p>
                  {/if}
                </div>
              </div>
            </div>
          {/if}
        </section>
      </div>
    </div>
  </div>

  {#if searchOpen}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="fixed inset-0 z-[120] bg-black/75" onclick={closeSearch}>
      <div
        class="mx-auto mt-[calc(12vh+52px)] w-[min(90vw,680px)]"
        onclick={(event) => event.stopPropagation()}
      >
        <div class="mb-3 overflow-hidden">
          <div
            bind:this={searchScopeStripEl}
            class="flex items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          >
            <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
            {#each accountScopes as scope, index (scope.id || 'all')}
              <button
                type="button"
                tabindex="-1"
                aria-pressed={searchScopeEmail === scope.id}
                data-scope-index={index}
                class="max-w-[240px] shrink-0 rounded-md border px-3 py-1.5 text-xs font-semibold shadow-sm backdrop-blur-sm transition-colors truncate
                  {searchScopeEmail === scope.id
                    ? 'border-primary/75 bg-primary/15 text-primary'
                    : 'border-border/70 bg-transparent text-muted-foreground hover:border-border hover:bg-muted/20 hover:text-foreground'}"
                onmousedown={(event) => event.preventDefault()}
                onclick={() => selectSearchScope(scope.id)}
              >
                {scope.label}
              </button>
            {/each}
            <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
          </div>
        </div>

        <div class="overflow-hidden rounded-xl border border-border bg-popover shadow-2xl">
          <div class="flex items-center gap-2 border-b border-border px-4 py-3">
            <Icon icon="mdi:magnify" class="h-5 w-5 shrink-0 text-muted-foreground" />
            <input
              bind:this={searchInputEl}
              bind:value={searchQuery}
              oninput={onSearchInput}
              oncompositionstart={onCompositionStart}
              oncompositionend={onCompositionEnd}
              onkeydown={onSearchKeydown}
              placeholder={$_('backupViewer.searchPlaceholder')}
              class="min-w-0 flex-1 border-none bg-transparent text-base text-foreground outline-none"
            />
            {#if searchLoading}
              <Icon icon="mdi:loading" class="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
            {/if}
            <button type="button" class="shrink-0 rounded p-1 hover:bg-muted" onclick={closeSearch} aria-label={$_('common.close')}>
              <Icon icon="mdi:close" class="h-5 w-5 text-muted-foreground" />
            </button>
          </div>

          {#if searchQuery.trim() && searchResults.length === 0 && !searchLoading}
            <div class="px-4 py-6 text-center text-sm text-muted-foreground">{$_('backupViewer.searchNoResults')}</div>
          {:else if searchResults.length > 0}
            <div class="max-h-[55vh] overflow-y-auto py-1 scrollbar-thin">
              {#each searchResults as result, index (result.key)}
                <button
                  type="button"
                  class="flex w-full items-start gap-3 px-4 py-2 text-left {index === searchActiveIndex ? 'bg-muted' : 'hover:bg-muted/50'}"
                  onclick={() => selectSearchResult(index)}
                  onmousemove={() => searchActiveIndex = index}
                >
                  <Icon icon="mdi:email-outline" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="min-w-0 flex-1">
                    <span class="flex min-w-0 items-baseline gap-2">
                      <span class="min-w-0 flex-1 truncate text-sm text-foreground">{result.subject || $_('backupViewer.unknownSubject')}</span>
                      {#if messageAttachmentCount(result) > 0}
                        <span class="shrink-0 text-muted-foreground" title={$_('backupViewer.attachments')}>
                          <Icon icon="mdi:paperclip" class="h-3.5 w-3.5" />
                        </span>
                      {/if}
                      <span class="shrink-0 text-xs text-muted-foreground">{formatShortDate(result.date)}</span>
                    </span>
                    <span class="truncate text-xs text-muted-foreground">{result.accountEmail}{result.folderPath ? ` / ${result.folderPath}` : ''}</span>
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
{/if}

<style>
  :global(.backup-viewer-body) {
    max-width: 100%;
    overflow-x: auto;
    color: hsl(var(--foreground));
    font-size: 0.875rem;
    line-height: 1.6;
  }

  :global(.backup-viewer-mail-content) {
    min-height: 2rem;
    max-width: 100%;
    overflow-wrap: anywhere;
  }

  :global(.backup-viewer-body p) {
    margin: 0 0 0.75rem;
  }

  :global(.backup-viewer-body a) {
    color: hsl(var(--primary));
    text-decoration: underline;
  }

  :global(.backup-viewer-body img) {
    max-width: 100%;
    height: auto;
  }

  :global(.backup-viewer-body table) {
    width: auto !important;
    max-width: 100% !important;
    table-layout: auto;
  }

  :global(.backup-viewer-body th),
  :global(.backup-viewer-body td) {
    max-width: 100%;
    white-space: normal !important;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter) {
    background: #fff;
    color: #1a1a0a;
    color-scheme: dark;
    filter: var(--backup-viewer-content-filter);
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter a) {
    color: #2563eb;
  }

  :global(.backup-viewer-mail-content.backup-viewer-dark-filter img:not([data-blocked-src])),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter video),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter iframe),
  :global(.backup-viewer-mail-content.backup-viewer-dark-filter [data-no-invert]) {
    filter: var(--backup-viewer-media-filter);
  }
</style>
