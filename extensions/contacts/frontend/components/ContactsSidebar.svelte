<script lang="ts">
  import { _ } from 'svelte-i18n'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import SourceItem from '$lib/components/kit/SourceItem.svelte'
  import { contactsView, selectSource } from '$extensions/contacts/frontend/stores/contactsView.svelte'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  // Source IDs (single local address book, classified by mail role):
  //   ''                  → all local contacts
  //   'role:sender'       → 发件人 (collected from received mail's From)
  //   'role:recipient'    → 收件人 (collected from sent mail's To)
  //   'role:ccbcc'        → 抄送密送 (collected from sent mail's Cc/Bcc)
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
      { id: 'role:ccbcc', label: $_('contacts.sidebar.roleCcBcc'), icon: 'mdi:email-multiple-outline' },
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
  {#snippet item(it: SidebarItem, { active })}
    <SourceItem icon={it.icon} label={it.label} {active} onclick={() => pick(it.id)} />
  {/snippet}
</SourceSidebar>
