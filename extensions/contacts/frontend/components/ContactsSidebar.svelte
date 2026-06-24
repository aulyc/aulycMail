<script lang="ts">
  import { _ } from 'svelte-i18n'
  import SourceSidebar from '$lib/components/kit/SourceSidebar.svelte'
  import SourceItem from '$lib/components/kit/SourceItem.svelte'
  import { contactsView, selectSource } from '$extensions/contacts/frontend/stores/contactsView.svelte'

  interface Props {
    onSelect: () => void
  }

  const { onSelect }: Props = $props()

  // Source IDs (single local address book):
  //   ''                  → all local contacts
  //   'local:manual'      → user-added local contacts
  //   'local:collected'   → auto-collected from mail
  type SidebarItem = {
    id: string
    label: string
    icon: string
  }

  // Reactive — re-runs when locale changes because $_ is referenced inside.
  const sections = $derived.by(() => {
    const builtins: SidebarItem[] = [
      { id: '', label: $_('contacts.sidebar.all'), icon: 'mdi:account-multiple' },
      { id: 'local:manual', label: $_('contacts.sidebar.localManual'), icon: 'mdi:account-plus-outline' },
      { id: 'local:collected', label: $_('contacts.sidebar.localCollected'), icon: 'mdi:email-outline' },
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
