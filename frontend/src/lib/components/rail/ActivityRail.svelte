<script lang="ts">
  import Icon from '@iconify/svelte'
  import RailButton from './RailButton.svelte'
  import { getActivePane, setActivePane } from '$lib/stores/uiState.svelte'
  import { BUILT_IN_RAIL_PANES } from '$lib/rail/panes'
  import { _ } from '$lib/i18n'

  interface Props {
    // Opens the app Settings dialog. Wired by App.svelte so the gear works
    // from every view (Mail + Contacts).
    onOpenSettings?: () => void
  }

  const { onOpenSettings }: Props = $props()

  // Mail is always present and always first; Contacts is a fixed built-in pane.
  // The rail always renders because it also hosts global Settings.
  let active = $derived(getActivePane())

  function select(name: string) {
    setActivePane(name)
  }

</script>

<nav
  class="flex flex-col items-stretch w-12 flex-shrink-0 bg-muted/30 border-r border-border pt-2"
  aria-label="Active rail pane"
>
  <RailButton
    icon="mdi:email"
    label="Mail"
    active={active === 'mail'}
    onclick={() => select('mail')}
  />
  {#each BUILT_IN_RAIL_PANES as pane (pane.id)}
    <RailButton
      icon={pane.icon}
      label={$_(pane.labelKey)}
      active={active === pane.id}
      onclick={() => select(pane.id)}
    />
  {/each}

  <!-- Settings: pinned to the bottom, available from Mail and Contacts. -->
  <button
    class="mt-auto mb-2 flex items-center justify-center w-12 h-12 border-l-[3px] border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30 transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
    type="button"
    title={$_('sidebar.settings')}
    aria-label={$_('sidebar.settings')}
    onclick={() => onOpenSettings?.()}
  >
    <Icon icon="mdi:cog" width="22" height="22" />
  </button>
</nav>
