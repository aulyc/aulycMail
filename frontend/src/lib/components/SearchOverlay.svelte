<script lang="ts">
  // Command-palette style search overlay. Opened with the `/` shortcut: a
  // dimmed full-window backdrop with a centered search box and a live results
  // list below it. Searches mail (current folder) or contacts depending on the
  // active rail. Selecting a result navigates to it via the parent's callbacks.
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import { formatRelativeDate } from '$lib/utils/date'
  // @ts-ignore - wailsjs path
  import { SearchConversations, Contacts_ListContactsForBrowse } from '../../../wailsjs/go/app/App'
  // @ts-ignore - wailsjs path
  import type { message, v1 } from '../../../wailsjs/go/models'

  interface Props {
    open?: boolean
    /** Which dataset to search — follows the active rail. */
    mode: 'mail' | 'contacts'
    /** Mail search scope (current folder). Ignored for contacts. */
    accountId?: string | null
    folderId?: string | null
    onClose: () => void
    onSelectMail?: (r: message.ConversationSearchResult) => void
    onSelectContact?: (c: v1.Contact) => void
  }

  let { open = $bindable(false), mode, accountId = null, folderId = null, onClose, onSelectMail, onSelectContact }: Props = $props()

  let query = $state('')
  let mailResults = $state<message.ConversationSearchResult[]>([])
  let contactResults = $state<v1.Contact[]>([])
  let loading = $state(false)
  let activeIndex = $state(0)
  let inputEl = $state<HTMLInputElement | null>(null)
  let debounce: ReturnType<typeof setTimeout> | null = null
  let searchSeq = 0

  const resultCount = $derived(mode === 'mail' ? mailResults.length : contactResults.length)

  // Reset + focus whenever the overlay opens.
  $effect(() => {
    if (open) {
      query = ''
      mailResults = []
      contactResults = []
      activeIndex = 0
      loading = false
      setTimeout(() => inputEl?.focus(), 30)
    } else if (debounce) {
      clearTimeout(debounce)
    }
  })

  function runSearch() {
    const q = query.trim()
    if (!q) {
      mailResults = []
      contactResults = []
      loading = false
      return
    }
    if (mode === 'mail' && (!accountId || !folderId)) {
      mailResults = []
      return
    }
    loading = true
    const seq = ++searchSeq
    const p: Promise<any> = mode === 'mail'
      ? SearchConversations(accountId!, folderId!, q, 0, 50, '')
      : Contacts_ListContactsForBrowse(q, '', 50, 0)
    p.then((r: any) => {
      if (seq !== searchSeq) return // stale
      if (mode === 'mail') mailResults = r || []
      else contactResults = r || []
      activeIndex = 0
    }).catch((err: unknown) => {
      if (seq !== searchSeq) return
      console.error('Search failed:', err)
      if (mode === 'mail') mailResults = []
      else contactResults = []
    }).finally(() => {
      if (seq === searchSeq) loading = false
    })
  }

  function onInput() {
    if (debounce) clearTimeout(debounce)
    debounce = setTimeout(runSearch, 200)
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
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      if (resultCount > 0) selectIndex(activeIndex)
    }
  }

  function mailSender(r: message.ConversationSearchResult): string {
    const p = r.participants && r.participants.length > 0 ? r.participants[0] : null
    return (p?.name || p?.email || '') as string
  }
  function contactEmail(c: v1.Contact): string {
    return c.emails && c.emails.length > 0 ? c.emails[0] : ''
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[100] bg-black/50" onclick={onClose}>
    <div
      class="mx-auto mt-[12vh] w-[min(90vw,640px)] bg-popover border border-border rounded-xl shadow-2xl overflow-hidden"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Search input -->
      <div class="flex items-center gap-2 px-4 py-3 border-b border-border">
        <Icon icon="mdi:magnify" class="w-5 h-5 text-muted-foreground flex-shrink-0" />
        <input
          bind:this={inputEl}
          bind:value={query}
          oninput={onInput}
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
                class="w-full flex items-center gap-3 px-4 py-2 text-left {i === activeIndex ? 'bg-muted' : 'hover:bg-muted/50'}"
                onclick={() => selectIndex(i)}
                onmousemove={() => activeIndex = i}
              >
                <Icon icon="mdi:email-outline" class="w-4 h-4 flex-shrink-0 text-muted-foreground" />
                <span class="flex flex-col min-w-0 flex-1">
                  <span class="truncate text-sm text-foreground">{r.subject || $_('viewer.noSubject')}</span>
                  <span class="truncate text-xs text-muted-foreground">{mailSender(r)}</span>
                </span>
                <span class="flex-shrink-0 text-xs text-muted-foreground whitespace-nowrap">
                  {r.latestDate ? formatRelativeDate(new Date(r.latestDate)) : ''}
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
      {:else}
        <div class="px-4 py-3 text-xs text-muted-foreground">{$_('search.overlayHint')}</div>
      {/if}
    </div>
  </div>
{/if}
