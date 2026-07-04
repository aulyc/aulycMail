<script lang="ts">
  // Command-palette style search overlay. Opened with the `/` shortcut: a
  // dimmed full-window backdrop with a centered search box and a live results
  // list below it. Searches mail or contacts depending on the
  // active rail. Selecting a result navigates to it via the parent's callbacks.
  import { tick } from 'svelte'
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { formatRelativeDateTime } from '$lib/utils/date'
  // @ts-ignore - wailsjs path
  import { SearchMailInAccount, Contacts_ListContactsForBrowse } from '../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import type { v1 } from '../../../wailsjs/go/models'

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
    onSelectContact?: (c: v1.Contact) => void
  }

  interface SearchScope {
    id: string
    label: string
  }

  let { open = $bindable(false), mode, onClose, onSelectMail, onSelectContact }: Props = $props()

  let query = $state('')
  let mailResults = $state<MailResult[]>([])
  let contactResults = $state<v1.Contact[]>([])
  let loading = $state(false)
  let activeIndex = $state(0)
  let selectedScopeId = $state('')
  let inputEl = $state<HTMLInputElement | null>(null)
  let scopeStripEl = $state<HTMLDivElement | null>(null)
  let debounce: ReturnType<typeof setTimeout> | null = null
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
  const selectedScopeIndex = $derived(Math.max(0, scopes.findIndex((scope) => scope.id === selectedScopeId)))

  // Reset + focus whenever the overlay opens.
  $effect(() => {
    if (open) {
      query = ''
      mailResults = []
      contactResults = []
      activeIndex = 0
      selectedScopeId = ''
      loading = false
      setTimeout(() => inputEl?.focus(), 30)
    } else if (debounce) {
      clearTimeout(debounce)
    }
  })

  $effect(() => {
    const hasSelectedScope = scopes.some((scope) => scope.id === selectedScopeId)
    if (!hasSelectedScope) selectedScopeId = ''
  })

  $effect(() => {
    if (!open) return
    void centerSelectedScope(selectedScopeId, scopes.length)
  })

  async function centerSelectedScope(scopeID: string, scopeCount: number) {
    await tick()
    await nextAnimationFrame()
    await nextAnimationFrame()
    if (!open || scopeID !== selectedScopeId || scopeCount !== scopes.length) return
    const strip = scopeStripEl
    if (!strip) return
    const button = strip.querySelector<HTMLElement>(`[data-scope-index="${selectedScopeIndex}"]`)
    if (!button) return

    const stripRect = strip.getBoundingClientRect()
    const buttonRect = button.getBoundingClientRect()
    const buttonCenter = buttonRect.left - stripRect.left + strip.scrollLeft + buttonRect.width / 2
    const targetLeft = buttonCenter - strip.clientWidth / 2
    const maxLeft = Math.max(0, strip.scrollWidth - strip.clientWidth)
    strip.scrollTo({ left: Math.min(Math.max(0, targetLeft), maxLeft), behavior: 'auto' })
  }

  function nextAnimationFrame(): Promise<void> {
    return new Promise((resolve) => {
      requestAnimationFrame(() => resolve())
    })
  }

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
    if (debounce) clearTimeout(debounce)
    debounce = setTimeout(runSearch, 200)
  }

  function onCompositionStart() {
    composing = true
  }

  function onCompositionEnd() {
    composing = false
    // The committed text is now in the input — search it.
    if (debounce) clearTimeout(debounce)
    debounce = setTimeout(runSearch, 200)
  }

  function selectScope(scopeID: string) {
    if (selectedScopeId === scopeID) {
      inputEl?.focus()
      return
    }
    selectedScopeId = scopeID
    activeIndex = 0
    void centerSelectedScope(scopeID, scopes.length)
    if (debounce) clearTimeout(debounce)
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
      activeIndex = Math.min(activeIndex + 1, resultCount - 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      e.stopPropagation()
      activeIndex = Math.max(activeIndex - 1, 0)
    } else if (e.key === 'Tab') {
      e.preventDefault()
      e.stopPropagation()
      moveScope(e.shiftKey ? -1 : 1)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      if (resultCount > 0) selectIndex(activeIndex)
    } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'a') {
      // Select all of the input's text. The native Edit menu has no Select All
      // item (so macOS won't do this), and the global Cmd+A would otherwise be
      // suppressed while the overlay is open — handle it here.
      e.preventDefault()
      e.stopPropagation()
      inputEl?.select()
    }
  }

  function contactEmail(c: v1.Contact): string {
    return c.emails && c.emails.length > 0 ? c.emails[0] : ''
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[100] bg-black/80" onclick={onClose}>
    <div
      class="mx-auto mt-[calc(12vh+52px)] w-[min(90vw,640px)]"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Search scope tags -->
      <div class="mb-3 overflow-hidden">
        <div
          bind:this={scopeStripEl}
          class="flex items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
          {#each scopes as scope, i (scope.id || 'all')}
            <button
              type="button"
              tabindex="-1"
              aria-pressed={selectedScopeId === scope.id}
              data-scope-index={i}
              class="max-w-[220px] shrink-0 rounded-md border px-3 py-1.5 text-xs font-semibold shadow-sm backdrop-blur-sm transition-colors truncate
                {selectedScopeId === scope.id
                  ? 'border-primary/75 bg-primary/15 text-primary'
                  : 'border-border/70 bg-transparent text-muted-foreground hover:border-border hover:bg-muted/20 hover:text-foreground'}"
              onmousedown={(e) => e.preventDefault()}
              onclick={() => selectScope(scope.id)}
            >
              {scope.label}
            </button>
          {/each}
          <span class="shrink-0 basis-1/2" aria-hidden="true"></span>
        </div>
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
          <div class="max-h-[60vh] overflow-y-auto scrollbar-thin py-1">
            {#if mode === 'mail'}
              {#each mailResults as r, i (r.threadId + '-' + i)}
                <button
                  class="w-full flex items-start gap-3 px-4 py-2 text-left {i === activeIndex ? 'bg-muted' : 'hover:bg-muted/50'}"
                  onclick={() => selectIndex(i)}
                  onmousemove={() => activeIndex = i}
                >
                  <Icon
                    icon={r.incoming ? 'mdi:email-arrow-left-outline' : 'mdi:email-arrow-right-outline'}
                    class="w-4 h-4 flex-shrink-0 text-muted-foreground mt-0.5"
                  />
                  <span class="flex flex-col min-w-0 flex-1">
                    <span class="flex items-baseline gap-2 min-w-0">
                      <span class="truncate text-sm text-foreground flex-1 min-w-0">{r.subject || $_('viewer.noSubject')}</span>
                      <span class="flex-shrink-0 text-xs text-muted-foreground whitespace-nowrap">
                        {r.date ? formatRelativeDateTime(new Date(r.date)) : ''}
                      </span>
                    </span>
                    <span class="truncate text-xs text-muted-foreground">{r.fromName || r.fromEmail}</span>
                    {#if r.snippet}
                      <span class="truncate text-xs text-muted-foreground/80">{r.snippet}</span>
                    {/if}
                  </span>
                </button>
              {/each}
            {:else}
              {#each contactResults as c, i (c.id + '-' + i)}
                <button
                  class="w-full flex items-center gap-3 px-4 py-2 text-left {i === activeIndex ? 'bg-muted' : 'hover:bg-muted/50'}"
                  onclick={() => selectIndex(i)}
                  onmousemove={() => activeIndex = i}
                >
                  <Icon icon="mdi:account-outline" class="w-4 h-4 flex-shrink-0 text-muted-foreground" />
                  <span class="flex flex-col min-w-0 flex-1">
                    <span class="truncate text-sm text-foreground">{c.name || contactEmail(c) || $_('contacts.common.unnamed')}</span>
                    {#if c.name && contactEmail(c)}
                      <span class="truncate text-xs text-muted-foreground">{contactEmail(c)}</span>
                    {/if}
                  </span>
                </button>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
