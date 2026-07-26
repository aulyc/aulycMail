<script lang="ts">
  // ContactList — mirrors mail's MessageList toolbar pattern:
  //
  //   [Title  |  Search input (when open)]      [Search]  [Sort]
  //
  // Search bar is HIDDEN by default and toggled open via the search button
  // or Ctrl+S. toggleSearchFocus() is exposed to ListPane's onFocusSearch
  // so the global Ctrl+S handler cycles through the same three states mail
  // uses: closed → focused → closed.
  //
  // Sort is owned by the contacts store and applied by the backend before
  // pagination. That keeps the visible first row, default selection, and every
  // infinite-scroll page in one stable order.

  import Icon from '@iconify/svelte'
  import { tick } from 'svelte'
  import { _ } from 'svelte-i18n'
  import ListPane from '$lib/components/kit/ListPane.svelte'
  import ListRow from '$lib/components/kit/ListRow.svelte'
  import ConfirmDialog from '$lib/components/kit/ConfirmDialog.svelte'
  import { contactsView, reloadContacts, loadMoreContacts, focusContact, activateContact, setSearchQuery, setSortOrder, deleteLocalContact } from '$contacts/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  import { createDebouncer } from '$lib/utils/debounce'
  import { shouldShowContactEmail } from '$contacts/utils/contactPresentation'
  import { shouldBlockContactList } from '$contacts/utils/contactLoadLifecycle'
  // Canonical list toolbar — owns hamburger placement, title styling, count
  // badge and search-mode swap. Contacts supplies label/count plus search
  // markup and trailing action buttons.
  import ListHeader from '$lib/components/kit/ListHeader.svelte'
  import { getUIState, getUIStateVersion } from '$lib/stores/uiState.svelte'
  import { focusPane, getFocusedPane, isMainKeyboardScope, setFocusedPane } from '$lib/stores/keyboard.svelte'
  import { getResponsiveView, isResponsive } from '$lib/stores/layout.svelte'
  // @ts-ignore - wailsjs bindings
  import type { contactdto } from '$wailsjs/go/models'

  // Match mail's middle list column width so switching panes keeps the same
  // second-column footprint.
  const listWidth = $derived.by(() => { getUIStateVersion(); return getUIState().listWidth })

  interface Props {
    onAdd?: () => void
  }

  let { onAdd }: Props = $props()

  let showSearch = $state(false)
  let searchInput = $state('')
  // Plain `let` (not $state) — same as App.svelte's component refs. The
  // ref is only read inside event handlers (focus / blur / select / equality
  // check against document.activeElement), never in a reactive context, so
  // making it $state adds overhead without benefit.
  let searchInputEl: HTMLInputElement | null = null
  let regionEl = $state<HTMLElement | null>(null)

  // Delete-confirmation state for keyboard-triggered deletes. ContactDetail
  // has its own button-triggered confirm dialog; this one fires when the user
  // has the LIST focused and hits Delete/Backspace on the highlighted row.
  let showDeleteConfirm = $state(false)
  let pendingDelete = $state<contactdto.ContactListItem | null>(null)
  let deleting = $state(false)

  function requestDelete(id: string) {
    const found = contactsView.contacts.find(c => c.id === id)
    if (!found) return
    pendingDelete = found
    showDeleteConfirm = true
  }

  async function confirmDelete() {
    if (!pendingDelete) return
    deleting = true
    try {
      await deleteLocalContact(pendingDelete!.id)
      toasts.success($_('contacts.toast.deleted'))
    } catch (err) {
      console.error('Failed to delete contact:', err)
      toasts.error($_('contacts.toast.failedDelete'))
    } finally {
      deleting = false
      pendingDelete = null
    }
  }

  const searchDebouncer = createDebouncer(200)
  function onSearchInput(e: Event) {
    searchInput = (e.currentTarget as HTMLInputElement).value
    searchDebouncer.schedule(() => {
      setSearchQuery(searchInput)
      reloadContacts()
    })
  }

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      // Match mail: Enter blurs the input and hands focus to the list so j/k
      // navigation works immediately on filtered results.
      e.preventDefault()
      searchInputEl?.blur()
      void tick().then(() => focusPane('messageList'))
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      clearSearch()
      void tick().then(() => focusPane('messageList'))
    }
  }

  function clearSearch() {
    searchInput = ''
    setSearchQuery('')
    showSearch = false
    searchDebouncer.cancel()
    reloadContacts()
  }

  // When the selected category changes, reset the search UI so the new category
  // shows all its contacts. selectSource() already cleared the store's
  // searchQuery; this keeps the local input + open state in sync.
  let lastSourceId = contactsView.selectedSourceId
  let lastResetSignal = contactsView.listResetSignal
  $effect(() => {
    const sel = contactsView.selectedSourceId
    if (sel !== lastSourceId) {
      lastSourceId = sel
      searchInput = ''
      showSearch = false
      searchDebouncer.cancel()
    }
  })

  $effect(() => {
    const signal = contactsView.listResetSignal
    if (signal !== lastResetSignal) {
      lastResetSignal = signal
      searchInput = ''
      showSearch = false
      searchDebouncer.cancel()
    }
  })

  // Three-state Ctrl+S toggle (matches MessageList.toggleSearchFocus):
  //   closed                  → open + focus
  //   open but unfocused      → focus
  //   open and focused        → close
  function toggleSearchFocus() {
    if (!showSearch) {
      showSearch = true
      setTimeout(() => {
        searchInputEl?.focus()
        searchInputEl?.select()
      }, 50)
      return
    }
    if (document.activeElement !== searchInputEl) {
      searchInputEl?.focus()
      searchInputEl?.select()
      return
    }
    clearSearch()
  }

  function toggleSort() {
    setSortOrder(contactsView.sortOrder === 'name-asc' ? 'name-desc' : 'name-asc')
    void reloadContacts()
  }

  function handleReachEnd() {
    if (!contactsView.hasMore || contactsView.loading || contactsView.loadingMore) return
    void loadMoreContacts()
  }

  function primaryEmail(c: contactdto.ContactListItem): string {
    return c.emails && c.emails.length > 0 ? c.emails[0] : ''
  }

  // Header label tracks the sidebar's selected category — mirrors mail's
  // MessageList showing the active folder name. Unknown ids fall back to the
  // generic "Contacts" label so the header is never empty.
  const headerLabel = $derived.by(() => {
    const sel = contactsView.selectedSourceId
    if (sel === '') return $_('contacts.sidebar.all')
    if (sel === 'role:sender') return $_('contacts.sidebar.roleSender')
    if (sel === 'role:recipient') return $_('contacts.sidebar.roleRecipient')
    if (sel === 'role:cc') return $_('contacts.sidebar.roleCc')
    if (sel === 'role:bcc') return $_('contacts.sidebar.roleBcc')
    if (sel === 'local') return $_('contacts.sidebar.localAll')
    if (sel === 'local:manual') return $_('contacts.sidebar.localManual')
    if (sel === 'local:collected') return $_('contacts.sidebar.localCollected')
    return $_('contacts.list.header')
  })

  const headerCount = $derived(contactsView.total)

  function claimListRegion(event: MouseEvent) {
    setFocusedPane('messageList')
    if (!(event.target instanceof Element)) return
    if (event.target.closest('button, a, input, textarea, select, [contenteditable="true"]')) return
    regionEl?.querySelector<HTMLElement>('[data-keyboard-region-focus-target]')?.focus({ preventScroll: true })
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  bind:this={regionEl}
  role="region"
  aria-label={headerLabel}
  data-keyboard-region="messageList"
  data-keyboard-region-visible={!isResponsive() || getResponsiveView() === 'default'}
  data-region-active={isMainKeyboardScope() && getFocusedPane() === 'messageList'}
  class="keyboard-region flex-shrink-0 min-h-0 flex flex-col border-r border-border bg-background"
  style="width: {listWidth}px"
  onmousedown={claimListRegion}
>
  <ListHeader
    label={headerLabel}
    count={headerCount}
    searchMode={showSearch}
  >
    {#snippet search()}
      <div class="flex items-center gap-1 bg-muted rounded-md px-2 flex-1 min-w-0">
        <Icon icon="mdi:magnify" class="w-4 h-4 text-muted-foreground flex-shrink-0" />
        <input
          bind:this={searchInputEl}
          type="text"
          placeholder={$_('contacts.list.searchPlaceholder')}
          class="bg-transparent border-none outline-none text-sm py-1.5 w-full min-w-[200px] text-foreground"
          value={searchInput}
          oninput={onSearchInput}
          onkeydown={handleSearchKeydown}
        />
        {#if searchInput}
          <button
            onclick={clearSearch}
            class="p-0.5 hover:bg-muted-foreground/20 rounded"
            title={$_('contacts.list.searchClear')}
            type="button"
          >
            <Icon icon="mdi:close" class="w-4 h-4 text-muted-foreground" />
          </button>
        {/if}
      </div>
    {/snippet}

    {#snippet actions()}
      <button
        class="p-2 rounded-md hover:bg-muted transition-colors"
        title={contactsView.sortOrder === 'name-asc' ? $_('contacts.list.sortAsc') : $_('contacts.list.sortDesc')}
        onclick={toggleSort}
        type="button"
      >
        <Icon
          icon={contactsView.sortOrder === 'name-asc' ? 'mdi:sort-alphabetical-ascending' : 'mdi:sort-alphabetical-descending'}
          class="w-5 h-5 text-muted-foreground"
        />
      </button>
      {#if onAdd}
        <button
          class="p-2 rounded-md hover:bg-muted transition-colors"
          title={$_('contacts.list.addTooltip')}
          onclick={onAdd}
          type="button"
        >
          <Icon icon="mdi:plus" class="w-5 h-5 text-muted-foreground" />
        </button>
      {/if}
    {/snippet}
  </ListHeader>

  <ListPane
    items={contactsView.contacts}
    selectedId={contactsView.selectedContactId}
    focusSlot="messageList"
    label={$_('contacts.list.label')}
    loading={shouldBlockContactList(contactsView.loading, contactsView.contacts.length)}
    selectedScrollSignal={contactsView.selectedContactScrollTopSignal}
    selectedScrollBlock="start"
    onSelect={(id) => focusContact(id)}
    onActivate={(id) => activateContact(id)}
    onDelete={requestDelete}
    onFocusSearch={toggleSearchFocus}
    onReachEnd={handleReachEnd}
  >
    {#snippet row(c: contactdto.ContactListItem, { selected })}
      <ListRow {selected} onclick={() => activateContact(c.id)}>
        <span class="flex flex-col min-w-0 flex-1">
          <span class="font-medium truncate text-foreground">{c.name || primaryEmail(c) || $_('contacts.common.unnamed')}</span>
          {#if shouldShowContactEmail(c.name, primaryEmail(c))}
            <span class="text-xs text-muted-foreground truncate">{primaryEmail(c)}</span>
          {/if}
        </span>
      </ListRow>
    {/snippet}

    {#snippet empty()}
      {#if contactsView.loadError}
        <div class="m-4 flex flex-col items-start gap-2 text-sm text-muted-foreground">
          <p>{$_('contacts.list.loadFailed')}</p>
          <button
            class="rounded text-primary hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2"
            type="button"
            onclick={() => reloadContacts()}
          >
            {$_('contacts.list.retry')}
          </button>
        </div>
      {:else}
        <p class="m-4 text-sm text-muted-foreground">
          {searchInput ? $_('contacts.list.emptySearch') : $_('contacts.list.empty')}
        </p>
      {/if}
    {/snippet}
  </ListPane>

  {#if contactsView.loadingMore}
    <div class="shrink-0 border-t border-border bg-muted/20 px-4 py-2">
      <div class="flex items-center justify-center gap-2 text-sm text-muted-foreground">
        <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
        <span>{$_('common.loading')}</span>
      </div>
    </div>
  {/if}
</div>

<ConfirmDialog
  bind:open={showDeleteConfirm}
  title={$_('contacts.delete.title')}
  description={pendingDelete
    ? $_('contacts.delete.descriptionLocal', {
      values: {
        name: pendingDelete.name || (pendingDelete.emails && pendingDelete.emails[0]) || $_('contacts.common.unnamed'),
      },
    })
    : ''}
  confirmLabel={$_('contacts.common.delete')}
  cancelLabel={$_('contacts.common.cancel')}
  variant="destructive"
  loading={deleting}
  onConfirm={confirmDelete}
/>
