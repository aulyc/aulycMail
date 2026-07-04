<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  // @ts-ignore - wailsjs path
  import {
    ChooseBackupDirectory,
    GetBackupSettings,
    GetBackupViewerCatalog,
    GetBackupViewerMessage,
    OpenBackupViewerDirectory,
    SearchBackupViewerMessages,
  } from '../../../../wailsjs/go/app/App.js'
  // @ts-ignore - wailsjs path
  import type { app } from '../../../../wailsjs/go/models'

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
  let catalog = $state<app.BackupViewerCatalog | null>(null)
  let selectedAccountEmail = $state('')
  let selectedMessageKey = $state('')
  let detail = $state<app.BackupViewerMessageDetail | null>(null)
  let loadingCatalog = $state(false)
  let loadingDetail = $state(false)
  let errorMessage = $state('')
  let mainScopeStripEl = $state<HTMLDivElement | null>(null)

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
  const selectedScopeIndex = $derived(Math.max(0, accountScopes.findIndex((scope) => scope.id === selectedAccountEmail)))
  const searchScopeIndex = $derived(Math.max(0, accountScopes.findIndex((scope) => scope.id === searchScopeEmail)))
  const visibleMessages = $derived.by(() => {
    const source = catalog?.messages ?? []
    if (!selectedAccountEmail) return source
    return source.filter((message) => message.accountEmail === selectedAccountEmail)
  })

  $effect(() => {
    if (open) {
      void initialize()
    } else {
      closeSearch()
    }
  })

  $effect(() => {
    if (!open) return
    void centerScope(mainScopeStripEl, selectedScopeIndex)
  })

  $effect(() => {
    if (!searchOpen) return
    void centerScope(searchScopeStripEl, searchScopeIndex)
  })

  async function initialize() {
    errorMessage = ''
    try {
      const settings = await GetBackupSettings()
      directory = settings?.directory || directory
      if (directory) {
        await loadCatalog(directory)
      } else {
        catalog = null
        selectedAccountEmail = ''
        selectedMessageKey = ''
        detail = null
      }
    } catch (err) {
      console.error('Failed to initialize backup viewer:', err)
      errorMessage = $_('backupViewer.loadFailed')
    }
  }

  async function loadCatalog(dir: string) {
    if (!dir.trim()) return
    loadingCatalog = true
    errorMessage = ''
    try {
      const result = await GetBackupViewerCatalog(dir)
      catalog = result
      directory = result?.directory || dir
      const availableScopes = new Set(['', ...((result?.accounts ?? []).map((account) => account.accountEmail))])
      if (!availableScopes.has(selectedAccountEmail)) {
        selectedAccountEmail = ''
      }
      await selectFirstVisibleMessage()
    } catch (err) {
      console.error('Failed to load backup catalog:', err)
      catalog = null
      detail = null
      selectedMessageKey = ''
      errorMessage = $_('backupViewer.loadFailed')
    } finally {
      loadingCatalog = false
    }
  }

  async function chooseDirectory() {
    try {
      const selected = await ChooseBackupDirectory()
      if (!selected) return
      directory = selected
      selectedAccountEmail = ''
      await loadCatalog(selected)
    } catch (err) {
      console.error('Failed to choose backup directory:', err)
      errorMessage = $_('backupViewer.chooseDirectoryFailed')
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
    void centerScope(mainScopeStripEl, selectedScopeIndex)
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
      detail = await GetBackupViewerMessage(directory, key)
    } catch (err) {
      console.error('Failed to load backup message:', err)
      detail = null
      errorMessage = $_('backupViewer.messageLoadFailed')
    } finally {
      loadingDetail = false
    }
  }

  function closeDialog() {
    open = false
    onClose?.()
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

  function formatDate(value: string): string {
    if (!value) return ''
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return date.toLocaleString()
  }

  function formatShortDate(value: string): string {
    if (!value) return ''
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return date.toLocaleDateString()
  }

  function formatSize(size: number): string {
    if (!size || size < 0) return ''
    const units = ['B', 'KB', 'MB', 'GB']
    let value = size
    let unit = 0
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024
      unit += 1
    }
    return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
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

      <section class="border-b border-border px-5 py-4">
        <div class="grid grid-cols-[120px_minmax(0,1fr)_auto_auto_auto_auto] items-center gap-3">
          <label class="text-sm font-semibold" for="backup-viewer-directory">{$_('backupViewer.directory')}</label>
          <input
            id="backup-viewer-directory"
            value={directory}
            readonly
            placeholder={$_('backupViewer.directoryPlaceholder')}
            class="h-10 min-w-0 rounded-md border border-border bg-background px-3 text-sm text-foreground outline-none"
          />
          <button type="button" class="inline-flex h-10 items-center gap-2 rounded-md border border-border px-3 text-sm font-semibold hover:bg-muted" onclick={chooseDirectory}>
            <Icon icon="mdi:folder-outline" width="18" height="18" />
            {$_('backupViewer.chooseDirectory')}
          </button>
          <button type="button" class="inline-flex h-10 items-center gap-2 rounded-md border border-border px-3 text-sm font-semibold hover:bg-muted disabled:opacity-50" disabled={!directory} onclick={openDirectory}>
            {$_('backupViewer.openDirectory')}
          </button>
          <button type="button" class="inline-flex h-10 items-center gap-2 rounded-md border border-border px-3 text-sm font-semibold hover:bg-muted disabled:opacity-50" disabled={!directory || loadingCatalog} onclick={refreshCatalog}>
            <Icon icon="mdi:refresh" width="18" height="18" class={loadingCatalog ? 'animate-spin' : ''} />
            {$_('backupViewer.refresh')}
          </button>
          <button type="button" class="inline-flex h-10 items-center gap-2 rounded-md bg-primary px-3 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50" disabled={!catalog?.messageCount} onclick={openSearch}>
            <Icon icon="mdi:magnify" width="18" height="18" />
            {$_('backupViewer.search')}
          </button>
        </div>
        {#if errorMessage}
          <p class="mt-3 text-sm text-destructive">{errorMessage}</p>
        {/if}
      </section>

      <section class="border-b border-border px-5 py-3">
        <div
          bind:this={mainScopeStripEl}
          class="flex items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
          {#each accountScopes as scope, index (scope.id || 'all')}
            <button
              type="button"
              data-scope-index={index}
              aria-pressed={selectedAccountEmail === scope.id}
              class="max-w-[260px] shrink-0 rounded-md border px-3 py-1.5 text-xs font-semibold transition-colors truncate
                {selectedAccountEmail === scope.id
                  ? 'border-primary/75 bg-primary/15 text-primary'
                  : 'border-border/70 bg-transparent text-muted-foreground hover:border-border hover:bg-muted/20 hover:text-foreground'}"
              onclick={() => selectScope(scope.id)}
            >
              <span>{scope.label}</span>
              {#if typeof scope.count === 'number'}
                <span class="ml-2 text-muted-foreground">{scope.count}</span>
              {/if}
            </button>
          {/each}
          <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
        </div>
      </section>

      <div class="grid min-h-0 flex-1 overflow-hidden grid-cols-[38%_1fr]">
        <section class="flex min-h-0 flex-col border-r border-border">
          <div class="flex h-11 shrink-0 items-center justify-between border-b border-border px-4">
            <h3 class="text-sm font-semibold">{$_('backupViewer.mailList')}</h3>
            <span class="text-xs text-muted-foreground">{visibleMessages.length}</span>
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
          <div class="flex h-11 shrink-0 items-center justify-between border-b border-border px-5">
            <h3 class="text-sm font-semibold">{$_('backupViewer.messageDetail')}</h3>
            {#if detail?.size}
              <span class="text-xs text-muted-foreground">{formatSize(detail.size)}</span>
            {/if}
          </div>
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
              <h2 class="mb-3 text-xl font-semibold">{detail.subject || $_('backupViewer.unknownSubject')}</h2>
              <div class="mb-5 space-y-1 rounded-md border border-border bg-muted/20 p-3 text-sm">
                <div class="grid grid-cols-[88px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.from')}</span>
                  <span class="min-w-0 break-words">{detail.from?.join(', ') || '-'}</span>
                </div>
                <div class="grid grid-cols-[88px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.to')}</span>
                  <span class="min-w-0 break-words">{detail.to?.join(', ') || '-'}</span>
                </div>
                {#if detail.cc?.length}
                  <div class="grid grid-cols-[88px_1fr] gap-2">
                    <span class="text-muted-foreground">{$_('backupViewer.cc')}</span>
                    <span class="min-w-0 break-words">{detail.cc.join(', ')}</span>
                  </div>
                {/if}
                {#if detail.bcc?.length}
                  <div class="grid grid-cols-[88px_1fr] gap-2">
                    <span class="text-muted-foreground">{$_('backupViewer.bcc')}</span>
                    <span class="min-w-0 break-words">{detail.bcc.join(', ')}</span>
                  </div>
                {/if}
                <div class="grid grid-cols-[88px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.date')}</span>
                  <span>{formatDate(detail.date)}</span>
                </div>
                <div class="grid grid-cols-[88px_1fr] gap-2">
                  <span class="text-muted-foreground">{$_('backupViewer.folder')}</span>
                  <span>{detail.accountEmail}{detail.folderPath ? ` / ${detail.folderPath}` : ''}</span>
                </div>
              </div>

              <div class="backup-viewer-body rounded-md border border-border bg-background p-4">
                {#if detail.hasHTML}
                  <!-- eslint-disable-next-line svelte/no-at-html-tags -- backup viewer HTML is sanitized in Go before it reaches the UI -->
                  {@html detail.bodyHTML}
                {:else if detail.bodyText}
                  <pre class="whitespace-pre-wrap break-words font-sans text-sm leading-6 text-foreground">{detail.bodyText}</pre>
                {:else}
                  <p class="text-sm text-muted-foreground">{$_('backupViewer.noBody')}</p>
                {/if}
              </div>

              {#if detail.attachments?.length}
                <div class="mt-5">
                  <h3 class="mb-2 text-sm font-semibold">{$_('backupViewer.attachments')}</h3>
                  <div class="space-y-2">
                    {#each detail.attachments as attachment, index (attachment.filename + '-' + index)}
                      <div class="flex items-center gap-3 rounded-md border border-border bg-muted/20 px-3 py-2">
                        <Icon icon="mdi:paperclip" class="h-4 w-4 text-muted-foreground" />
                        <span class="min-w-0 flex-1 truncate text-sm">{attachment.filename}</span>
                        <span class="text-xs text-muted-foreground">{attachment.contentType}</span>
                        <span class="text-xs text-muted-foreground">{formatSize(attachment.size)}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
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
    color: hsl(var(--foreground));
    font-size: 0.875rem;
    line-height: 1.6;
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
</style>
