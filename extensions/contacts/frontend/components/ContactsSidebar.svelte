<script lang="ts">
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import Icon from '@iconify/svelte'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getAccountColor } from '$lib/utils/accountColor'
  import { contactsView, selectSource, reloadContacts } from '$contacts/frontend/stores/contactsView.svelte'
  import {
    contactAccountGroups,
    loadContactAccountGroups,
  } from '$contacts/frontend/stores/contactAccountGroups.svelte'
  import {
    beginContactRefresh,
    completeContactRefresh,
    failContactRefresh,
    initContactRefreshEvents,
  } from '$contacts/frontend/stores/contactRefresh.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { RefreshContactsFromMail } from '$wailsjs/go/app/App'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  let refreshing = $state(false)
  let accountGroups = $derived(contactAccountGroups.groups)

  function accountSourceID(accountID: string): string {
    return `account:${accountID}`
  }

  onMount(() => {
    initContactRefreshEvents()
    void loadContactAccountGroups()
  })

  // Re-scan every account's mail and re-collect participants into the local
  // address book (senders / recipients / cc-bcc), then reload the list.
  async function runRefresh() {
    if (refreshing) return
    refreshing = true
    beginContactRefresh()
    try {
      const count = await RefreshContactsFromMail()
      await loadContactAccountGroups({ force: true })
      await reloadContacts()
      completeContactRefresh(count)
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
  //   'account:<accountID>:<role>'    → account-scoped sender/recipient/cc/bcc
  type SidebarItem = {
    id: string
    label: string
    count?: number
    accountID?: string
    accountIndex?: number
  }

  // Reactive — re-runs when locale changes because $_ is referenced inside.
  const sections = $derived.by(() => {
    const builtins: SidebarItem[] = [{
      id: '',
      label: $_('contacts.sidebar.all'),
    }]

    for (const [accountIndex, account] of accountGroups.entries()) {
      builtins.push({
        id: accountSourceID(account.accountId),
        label: account.email || account.name || $_('contacts.common.unnamed'),
        count: account.count,
        accountID: account.accountId,
        accountIndex,
      })
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
    <button
      class="p-1 rounded hover:bg-muted/40 text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
      title={$_('contacts.sidebar.refresh')}
      aria-label={$_('contacts.sidebar.refresh')}
      onclick={runRefresh}
      disabled={refreshing}
      type="button"
    >
      <Icon icon="mdi:refresh" class="w-5 h-5 {refreshing ? 'animate-spin' : ''}" />
    </button>
  {/snippet}

  {#snippet item(it: SidebarItem, { active })}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_interactive_supports_focus -->
    <div
      role="option"
      aria-selected={active}
      class="flex items-center gap-2 mx-2 py-1.5 pr-2 text-sm rounded-md text-left transition-colors cursor-pointer select-none {active
        ? 'bg-primary/10 text-primary font-medium'
        : 'text-foreground hover:bg-muted/50'}"
      style="padding-left: 0.75rem"
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
        <span class="text-xs text-muted-foreground flex-shrink-0">{it.count}</span>
      {/if}
    </div>
  {/snippet}
</SourceSidebar>
