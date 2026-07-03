<script lang="ts">
  import { onMount } from 'svelte'
  import { _ } from 'svelte-i18n'
  import Icon from '@iconify/svelte'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import { contactsView, selectSource, reloadContacts } from '$extensions/contacts/frontend/stores/contactsView.svelte'
  import {
    contactAccountGroups,
    loadContactAccountGroups,
  } from '$extensions/contacts/frontend/stores/contactAccountGroups.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { RefreshContactsFromMail } from '$wailsjs/go/app/App'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  let refreshing = $state(false)
  let expandedAccounts = $state<Record<string, boolean>>({})
  let accountGroups = $derived(contactAccountGroups.groups)

  const roleItems = $derived.by(() => [
    { role: 'sender', label: $_('contacts.sidebar.roleSender'), icon: 'mdi:email-arrow-left-outline', countKey: 'senderCount' },
    { role: 'recipient', label: $_('contacts.sidebar.roleRecipient'), icon: 'mdi:email-arrow-right-outline', countKey: 'recipientCount' },
    { role: 'cc', label: $_('contacts.sidebar.roleCc'), icon: 'mdi:email-multiple-outline', countKey: 'ccCount' },
    { role: 'bcc', label: $_('contacts.sidebar.roleBcc'), icon: 'mdi:email-off-outline', countKey: 'bccCount' },
  ] as const)

  function accountSourceID(accountID: string): string {
    return `account:${accountID}`
  }

  function accountRoleSourceID(accountID: string, role: string): string {
    return `account:${accountID}:${role}`
  }

  onMount(() => {
    void loadContactAccountGroups()
  })

  // Re-scan every account's mail and re-collect participants into the local
  // address book (senders / recipients / cc-bcc), then reload the list.
  async function runRefresh() {
    if (refreshing) return
    refreshing = true
    try {
      const count = await RefreshContactsFromMail()
      await loadContactAccountGroups({ force: true })
      await reloadContacts()
      toasts.success($_('contacts.toast.refreshed', { values: { count } }))
    } catch (err) {
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
    icon?: string
    depth: 0 | 1
    count?: number
    expandable?: boolean
    expanded?: boolean
    accountID?: string
  }

  // Reactive — re-runs when locale changes because $_ is referenced inside.
  const sections = $derived.by(() => {
    const builtins: SidebarItem[] = [{
      id: '',
      label: $_('contacts.sidebar.all'),
      depth: 0,
    }]

    for (const account of accountGroups) {
      const expanded = expandedAccounts[account.accountId] === true
      builtins.push({
        id: accountSourceID(account.accountId),
        label: account.email || account.name || $_('contacts.common.unnamed'),
        depth: 0,
        count: account.count,
        expandable: true,
        expanded,
        accountID: account.accountId,
      })
      if (expanded) {
        for (const roleItem of roleItems) {
          builtins.push({
            id: accountRoleSourceID(account.accountId, roleItem.role),
            label: roleItem.label,
            icon: roleItem.icon,
            depth: 1,
            count: account[roleItem.countKey],
            accountID: account.accountId,
          })
        }
      }
    }

    return [{ items: builtins }]
  })

  function pick(id: string) {
    selectSource(id)
    onSelect()
  }

  function toggleItem(it: SidebarItem, e: MouseEvent) {
    e.stopPropagation()
    if (it.accountID) {
      expandedAccounts = {
        ...expandedAccounts,
        [it.accountID]: !(expandedAccounts[it.accountID] === true),
      }
    }
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
      style="padding-left: {it.depth === 0 ? 0.75 : 1.5}rem"
      onclick={() => pick(it.id)}
    >
      {#if it.icon}
        <Icon icon={it.icon} class="w-4 h-4 flex-shrink-0" />
      {/if}
      <span class="truncate min-w-0 flex-1">{it.label}</span>
      {#if it.count !== undefined}
        <span class="text-xs text-muted-foreground flex-shrink-0">{it.count}</span>
      {/if}
      {#if it.expandable}
        <button
          type="button"
          class="p-0.5 rounded hover:bg-muted text-muted-foreground hover:text-foreground flex-shrink-0"
          aria-label={it.expanded ? $_('contacts.sidebar.collapse') : $_('contacts.sidebar.expand')}
          title={it.expanded ? $_('contacts.sidebar.collapse') : $_('contacts.sidebar.expand')}
          onclick={(e) => toggleItem(it, e)}
        >
          <Icon icon={it.expanded ? 'mdi:chevron-down' : 'mdi:chevron-right'} class="w-4 h-4" />
        </button>
      {/if}
    </div>
  {/snippet}
</SourceSidebar>
