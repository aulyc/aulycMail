<script lang="ts">
  // Command-palette style search overlay. Opened with the `/` shortcut: a
  // dimmed full-window backdrop with a centered search box and a live results
  // list below it. Searches mail or contacts depending on the
  // active rail. Selecting a result navigates to it via the parent's callbacks.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import ModalFrame from '$lib/components/ui/ModalFrame.svelte'
  import SearchScopeCarousel from '$lib/components/search/SearchScopeCarousel.svelte'
  import {
    SEARCH_RESULT_VIEWPORT_HEIGHT_PX,
    resultRowHighlightClass,
    searchResultScrollTopForIndex,
    shouldActivatePointerResult,
    type SearchResultInputMode,
  } from '$lib/components/search/searchResultHighlight'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { formatRelativeDateTime } from '$lib/utils/date'
  import { createDebouncer } from '$lib/utils/debounce'
  import { shouldShowContactEmail } from '$contacts/utils/contactPresentation'
  // @ts-ignore - wailsjs path
  import { SearchMailInAccount, Contacts_ListContactsForBrowse } from '../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import type { contactdto } from '../../../wailsjs/go/models'

  // Substring mail-search result (mirrors Go message.ContactMessage).
  interface MailResult {
    id: string
    threadId: string
    accountId: string
    folderId: string
    subject: string
    fromName: string
    fromEmail: string
    date: string
    isRead: boolean
    incoming: boolean
    snippet: string
  }

  interface Props {
    open?: boolean
    /** Which dataset to search — follows the active rail. */
    mode: 'mail' | 'contacts'
    onClose: () => void
    onSelectMail?: (r: MailResult) => void
    onSelectContact?: (c: contactdto.Contact) => void
  }

  interface SearchScope {
    id: string
    label: string
  }

  let { open = $bindable(false), mode, onClose, onSelectMail, onSelectContact }: Props = $props()

  let query = $state('')
  let mailResults = $state<MailResult[]>([])
  let contactResults = $state<contactdto.Contact[]>([])
  let loading = $state(false)
  let activeIndex = $state(0)
  let resultInputMode = $state<SearchResultInputMode>('keyboard')
  let selectedScopeId = $state('')
  let inputEl = $state<HTMLInputElement | null>(null)
  let resultListEl = $state<HTMLDivElement | null>(null)
  let pointerActivationSuppressedUntil = 0
  const searchDebouncer = createDebouncer(200)
  let searchSeq = 0
  // True while an IME is composing (e.g. typing pinyin before picking a hanzi).
  let composing = false

  const resultCount = $derived(mode === 'mail' ? mailResults.length : contactResults.length)
  const scopes = $derived.by<SearchScope[]>(() => [
    {
      id: '',
      label: mode === 'mail' ? $_('search.scopeAllMail') : $_('search.scopeAllContacts'),
    },
    ...accountStore.accounts
      .filter((item) => !!item.account?.id)
      .map((item) => ({
        id: item.account.id,
        label: item.account.email || item.account.name || item.account.id,
      })),
  ])

  // Reset + focus whenever the overlay opens.
  $effect(() => {
    if (open) {
      query = ''
      mailResults = []
      contactResults = []
      activeIndex = 0
      resultInputMode = 'keyboard'
      selectedScopeId = ''
      loading = false
      setTimeout(() => inputEl?.focus(), 30)
    } else {
      searchDebouncer.cancel()
    }
  })

  $effect(() => {
    const hasSelectedScope = scopes.some((scope) => scope.id === selectedScopeId)
    if (!hasSelectedScope) selectedScopeId = ''
  })

  function runSearch() {
    const q = query.trim()
    if (!q) {
      mailResults = []
      contactResults = []
      loading = false
      return
    }
    loading = true
    const seq = ++searchSeq
    const currentMode = mode
    const scopeID = selectedScopeId
    const p: Promise<any> = currentMode === 'mail'
      ? SearchMailInAccount(scopeID, q, 50)
      : Contacts_ListContactsForBrowse(q, scopeID ? `account:${scopeID}` : '', 50, 0)
    p.then((r: any) => {
      if (seq !== searchSeq) return // stale
      if (currentMode === 'mail') mailResults = r || []
      else contactResults = r || []
      activeIndex = 0
      resultInputMode = 'keyboard'
      requestAnimationFrame(() => {
        if (resultListEl) resultListEl.scrollTop = 0
      })
    }).catch((err: unknown) => {
      if (seq !== searchSeq) return
      console.error('Search failed:', err)
      if (currentMode === 'mail') mailResults = []
      else contactResults = []
    }).finally(() => {
      if (seq === searchSeq) loading = false
    })
  }

  function onInput() {
    // While an IME is composing pinyin, the in-progress romaji ("liao") sits in
    // the input but isn't a real query yet — don't search on it. We search on
    // compositionend instead, once the hanzi is committed.
    if (composing) return
    searchDebouncer.schedule(runSearch)
  }

  function onCompositionStart() {
    composing = true
  }

  function onCompositionEnd() {
    composing = false
    // The committed text is now in the input — search it.
    searchDebouncer.schedule(runSearch)
  }

  function selectScope(scopeID: string) {
    if (selectedScopeId === scopeID) {
      inputEl?.focus()
      return
    }
    selectedScopeId = scopeID
    activeIndex = 0
    searchDebouncer.cancel()
    runSearch()
    setTimeout(() => inputEl?.focus(), 0)
  }

  function moveScope(delta: number) {
    if (scopes.length === 0) return
    const currentIndex = Math.max(0, scopes.findIndex((scope) => scope.id === selectedScopeId))
    const nextIndex = (currentIndex + delta + scopes.length) % scopes.length
    selectScope(scopes[nextIndex].id)
  }

  function selectIndex(i: number) {
    if (mode === 'mail') {
      const r = mailResults[i]
      if (r) onSelectMail?.(r)
    } else {
      const c = contactResults[i]
      if (c) onSelectContact?.(c)
    }
    onClose()
  }

  function onKeydown(e: KeyboardEvent) {
    // While an IME is composing (e.g. typing pinyin), let Enter/arrows commit or
    // navigate the candidate window — don't hijack them for the results list.
    if (e.isComposing || e.keyCode === 229) return
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      onClose()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      e.stopPropagation()
      moveActiveResult(1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      e.stopPropagation()
      moveActiveResult(-1)
    } else if (e.key === 'Tab') {
      e.preventDefault()
      e.stopPropagation()
      moveScope(e.shiftKey ? -1 : 1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      if (resultCount > 0 && activeIndex >= 0) selectIndex(activeIndex)
    } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'a') {
      // Select all of the input's text. The native Edit menu has no Select All
      // item (so macOS won't do this), and the global Cmd+A would otherwise be
      // suppressed while the overlay is open — handle it here.
      e.preventDefault()
      e.stopPropagation()
      inputEl?.select()
    }
  }

  function contactEmail(c: contactdto.Contact): string {
    return c.emails && c.emails.length > 0 ? c.emails[0] : ''
  }

  function activatePointerResult(index: number, event: MouseEvent) {
    if (!shouldActivatePointerResult(
      event.movementX,
      event.movementY,
      performance.now(),
      pointerActivationSuppressedUntil,
    )) return
    resultInputMode = 'pointer'
    activeIndex = index
  }

  function clearStaleResultHighlight() {
    resultInputMode = 'idle'
    activeIndex = -1
  }

  function moveActiveResult(delta: -1 | 1) {
    if (resultCount <= 0) return
    resultInputMode = 'keyboard'
    // WebKit can dispatch mousemove while programmatic scrolling moves a new
    // row underneath a stationary pointer. Keep keyboard ownership briefly so
    // that synthetic event cannot replace the active index with the hovered row.
    pointerActivationSuppressedUntil = performance.now() + 250

    const baseIndex = activeIndex >= 0 ? activeIndex : (delta > 0 ? -1 : 1)
    const nextIndex = Math.max(0, Math.min(baseIndex + delta, resultCount - 1))
    activeIndex = nextIndex

    requestAnimationFrame(() => {
      if (!resultListEl) return
      resultListEl.scrollTop = searchResultScrollTopForIndex(
        nextIndex,
        resultListEl.scrollTop,
        resultListEl.clientHeight,
      )
    })
  }
</script>

{#if open}
  <ModalFrame
    {open}
    onClose={onClose}
    containerClass="z-[100]"
    panelClass="mx-auto mt-[calc(12vh+52px)] w-[min(90vw,640px)]"
  >
      <!-- Search scope tags -->
      <div class="mb-3 overflow-hidden">
        <SearchScopeCarousel scopes={scopes} selectedId={selectedScopeId} onSelect={selectScope} />
      </div>

      <div class="bg-popover border border-border rounded-xl shadow-2xl overflow-hidden">
        <!-- Search input -->
        <div class="flex items-center gap-2 px-4 py-3 {query.trim() || resultCount > 0 ? 'border-b border-border' : ''}">
          <Icon icon="mdi:magnify" class="w-5 h-5 text-muted-foreground flex-shrink-0" />
          <input
            bind:this={inputEl}
            bind:value={query}
            oninput={onInput}
            oncompositionstart={onCompositionStart}
            oncompositionend={onCompositionEnd}
            onkeydown={onKeydown}
            placeholder={mode === 'mail' ? $_('search.overlayMail') : $_('search.overlayContacts')}
            class="flex-1 min-w-0 bg-transparent border-none outline-none text-base text-foreground"
          />
          {#if loading}
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin text-muted-foreground flex-shrink-0" />
          {/if}
          <button class="p-1 rounded hover:bg-muted flex-shrink-0" onclick={onClose} aria-label="Close">
            <Icon icon="mdi:close" class="w-5 h-5 text-muted-foreground" />
          </button>
        </div>

        <!-- Results -->
        {#if query.trim() && resultCount === 0 && !loading}
          <div class="px-4 py-6 text-center text-sm text-muted-foreground">{$_('search.overlayNoResults')}</div>
        {:else if resultCount > 0}
          <div
            bind:this={resultListEl}
            class="snap-y snap-mandatory overflow-x-hidden overflow-y-auto overscroll-contain scrollbar-thin"
            style={`max-height: ${SEARCH_RESULT_VIEWPORT_HEIGHT_PX}px`}
            onwheel={clearStaleResultHighlight}
          >
            {#if mode === 'mail'}
              {#each mailResults as r, i (r.threadId + '-' + i)}
                <button
                  class="flex h-[52px] w-full snap-start items-center gap-3 px-4 text-left {resultRowHighlightClass(i, activeIndex, resultInputMode)}"
                  onclick={() => selectIndex(i)}
                  onmousemove={(event) => activatePointerResult(i, event)}
                >
                  <Icon
                    icon={r.incoming ? 'mdi:email-arrow-left-outline' : 'mdi:email-arrow-right-outline'}
                    class="h-4 w-4 flex-shrink-0 text-muted-foreground"
                  />
                  <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span class="flex min-w-0 items-baseline">
                      <span class="min-w-0 flex-1 truncate text-sm text-foreground">{r.subject || $_('viewer.noSubject')}</span>
                      <span class="ml-4 max-w-40 flex-shrink-0 truncate text-right text-xs text-muted-foreground">
                        {r.fromName || r.fromEmail}
                      </span>
                      <span class="ml-4 flex-shrink-0 whitespace-nowrap text-xs text-muted-foreground">
                        {r.date ? formatRelativeDateTime(new Date(r.date)) : ''}
                      </span>
                    </span>
                    <span class="truncate text-xs text-muted-foreground/80">{r.snippet || '\u00A0'}</span>
                  </span>
                </button>
              {/each}
            {:else}
              {#each contactResults as c, i (c.id + '-' + i)}
                <button
                  class="flex h-[52px] w-full snap-start items-center gap-3 px-4 text-left {resultRowHighlightClass(i, activeIndex, resultInputMode)}"
                  onclick={() => selectIndex(i)}
                  onmousemove={(event) => activatePointerResult(i, event)}
                >
                  <Icon icon="mdi:account-outline" class="w-4 h-4 flex-shrink-0 text-muted-foreground" />
                  <span class="flex flex-col min-w-0 flex-1">
                    <span class="truncate text-sm text-foreground">{c.name || contactEmail(c) || $_('contacts.common.unnamed')}</span>
                    {#if shouldShowContactEmail(c.name, contactEmail(c))}
                      <span class="truncate text-xs text-muted-foreground">{contactEmail(c)}</span>
                    {/if}
                  </span>
                </button>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
  </ModalFrame>
{/if}
