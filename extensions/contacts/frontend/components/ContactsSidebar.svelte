<script lang="ts">
  import { _ } from 'svelte-i18n'
  import Icon from '@iconify/svelte'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import SourceItem from '$lib/components/kit/SourceItem.svelte'
  import { contactsView, selectSource, reloadContacts } from '$extensions/contacts/frontend/stores/contactsView.svelte'
  import { toasts } from '$lib/stores/toast'
  // @ts-ignore - wailsjs bindings
  import { RefreshContactsFromMail } from '$wailsjs/go/app/App'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  let refreshing = $state(false)

  // Re-scan every account's mail and re-collect participants into the local
  // address book (senders / recipients / cc-bcc), then reload the list.
  async function runRefresh() {
    if (refreshing) return
    refreshing = true
    try {
      const count = await RefreshContactsFromMail()
      await reloadContacts()
      toasts.success($_('contacts.toast.refreshed', { values: { count } }))
    } catch (err) {
      toasts.error((err as Error)?.message ?? String(err))
    } finally {
      refreshing = false
    }
  }

  // Source IDs (single local address book, classified by mail role):
  //   ''                  → all local contacts
  //   'role:sender'       → 发件人 (collected from received mail's From)
  //   'role:recipient'    → 收件人 (collected from sent mail's To)
  //   'role:cc'           → 抄送 (collected from sent mail's Cc)
  //   'role:bcc'          → 密送 (collected from sent mail's Bcc)
  type SidebarItem = {
    id: string
    label: string
    icon: string
  }

  // Reactive — re-runs when locale changes because $_ is referenced inside.
  const sections = $derived.by(() => {
    const builtins: SidebarItem[] = [
      { id: '', label: $_('contacts.sidebar.all'), icon: 'mdi:account-multiple' },
      { id: 'role:sender', label: $_('contacts.sidebar.roleSender'), icon: 'mdi:email-arrow-left-outline' },
      { id: 'role:recipient', label: $_('contacts.sidebar.roleRecipient'), icon: 'mdi:email-arrow-right-outline' },
      { id: 'role:cc', label: $_('contacts.sidebar.roleCc'), icon: 'mdi:email-multiple-outline' },
      { id: 'role:bcc', label: $_('contacts.sidebar.roleBcc'), icon: 'mdi:email-off-outline' },
    ]
    return [{ items: builtins }]
  })

  function pick(id: string) {
    selectSource(id)
    onSelect()
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
    <SourceItem icon={it.icon} label={it.label} {active} onclick={() => pick(it.id)} />
  {/snippet}
</SourceSidebar>
