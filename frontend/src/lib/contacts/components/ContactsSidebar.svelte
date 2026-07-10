<script lang="ts">
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import Icon from '@iconify/svelte'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getAccountColor } from '$lib/utils/accountColor'
  import { contactsView, selectSource, reloadContacts } from '$contacts/stores/contactsView.svelte'
  import {
    contactAccountGroups,
    loadContactAccountGroups,
  } from '$contacts/stores/contactAccountGroups.svelte'
  import {
    beginContactRefresh,
    completeContactRefresh,
    contactRefresh,
    failContactRefresh,
    initContactRefreshEvents,
  } from '$contacts/stores/contactRefresh.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { RefreshContactsFromMail } from '$wailsjs/go/app/App'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  let refreshing = $state(false)
  let accountGroups = $derived(contactAccountGroups.groups)
  let retriedAfterAccountsLoaded = $state(false)

  function accountSourceID(accountID: string): string {
    return `account:${accountID}`
  }

  onMount(() => {
    initContactRefreshEvents()
    void loadContactAccountGroups({ force: true })
    if (!accountStore.loading && accountStore.accounts.length === 0) {
      void accountStore.load()
    }
  })

  $effect(() => {
    if (
      !retriedAfterAccountsLoaded &&
      contactAccountGroups.loaded &&
      !contactAccountGroups.loading &&
      contactAccountGroups.groups.length === 0 &&
      accountStore.accounts.length > 0
    ) {
      retriedAfterAccountsLoaded = true
      void loadContactAccountGroups({ force: true })
    }
  })

  // Re-scan every account's mail and re-collect participants into the local
  // address book (senders / recipients / cc-bcc), then reload the list.
  async function runRefresh() {
    if (refreshing) return
    refreshing = true
    beginContactRefresh()
    try {
      const count = await RefreshContactsFromMail()
      completeContactRefresh(count, count)
      await reloadContacts()
      // Account group counts scan message/contact associations and can be slow
      // on large mailboxes. Refresh them after the list is usable.
      void loadContactAccountGroups({ force: true })
      toasts.success($_('contacts.toast.refreshed', { values: { count } }))
    } catch (err) {
      failContactRefresh()
      toasts.error((err as Error)?.message ?? String(err))
    } finally {
      refreshing = false
    }
  }

  // Source IDs:
  //   ''                              → all local contacts
  //   'account:<accountID>'           → contacts associated with one mail account
  type SidebarItem = {
    id: string
    label: string
    count?: number
    accountID?: string
    accountIndex?: number
  }

  // Reactive — re-runs when locale changes because $_ is referenced inside.
  const sections = $derived.by(() => {
    const builtins: SidebarItem[] = []

    if (accountGroups.length > 0) {
      for (const [accountIndex, account] of accountGroups.entries()) {
        builtins.push({
          id: accountSourceID(account.accountId),
          label: account.email || account.name || $_('contacts.common.unnamed'),
          count: account.count,
          accountID: account.accountId,
          accountIndex,
        })
      }
    } else {
      for (const [accountIndex, { account }] of accountStore.accounts.entries()) {
        builtins.push({
          id: accountSourceID(account.id),
          label: account.email || account.name || $_('contacts.common.unnamed'),
          accountID: account.id,
          accountIndex,
        })
      }
    }

    return [{ items: builtins }]
  })

  function pick(id: string) {
    selectSource(id)
    onSelect()
  }

  function sidebarAccountColor(it: SidebarItem): string {
    const found = accountStore.accounts.find(item => item.account.id === it.accountID)?.account
    return getAccountColor(found ?? { orderIndex: it.accountIndex ?? 0 })
  }
</script>

<SourceSidebar
  title={$_('contacts.sidebar.title')}
  {sections}
  selectedId={contactsView.selectedSourceId}
  onSelect={pick}
>
  {#snippet titleAction()}
    <div class="flex items-center gap-1 min-w-0">
      <button
        class="px-2 py-1 rounded-md text-sm font-medium transition-colors whitespace-nowrap {contactsView.selectedSourceId === ''
          ? 'bg-primary/10 text-primary'
          : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'}"
        aria-pressed={contactsView.selectedSourceId === ''}
        onclick={() => pick('')}
        type="button"
      >
        {$_('contacts.sidebar.allButton')}
      </button>
      <button
        class="p-1 rounded hover:bg-muted/40 text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
        title={$_('contacts.sidebar.refresh')}
        aria-label={$_('contacts.sidebar.refresh')}
        onclick={runRefresh}
        disabled={refreshing}
        type="button"
      >
        <Icon icon="mdi:refresh" class="w-5 h-5 {contactRefresh.active ? 'animate-spin' : ''}" />
      </button>
    </div>
  {/snippet}

  {#snippet item(it: SidebarItem, { active })}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_interactive_supports_focus -->
    <div
      role="option"
      aria-selected={active}
      class={it.accountID
        ? `w-full flex items-center gap-2 px-3 py-2 text-sm font-medium text-left transition-colors cursor-pointer select-none ${active
          ? 'bg-primary/10 text-primary'
          : 'text-foreground hover:bg-muted/50'}`
        : `flex items-center gap-2 mx-2 py-1.5 pr-2 text-sm rounded-md text-left transition-colors cursor-pointer select-none ${active
          ? 'bg-primary/10 text-primary font-medium'
          : 'text-foreground hover:bg-muted/50'}`}
      style={it.accountID ? undefined : 'padding-left: 0.75rem'}
      onclick={() => pick(it.id)}
    >
      {#if it.accountID}
        <span
          class="w-2 h-2 rounded-full flex-shrink-0"
          style="background-color: {sidebarAccountColor(it)}"
          aria-hidden="true"
        ></span>
      {/if}
      <span class="truncate min-w-0 flex-1">{it.label}</span>
      {#if it.count !== undefined}
        <span class="text-xs font-medium text-muted-foreground flex-shrink-0">{it.count}</span>
      {/if}
    </div>
  {/snippet}
</SourceSidebar>
