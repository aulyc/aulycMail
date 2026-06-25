<script lang="ts">
  import Icon from '@iconify/svelte'
  import RailButton from './RailButton.svelte'
  import { getRailTabs } from '$lib/stores/extensionRegistry.svelte'
  import { getActiveExtension, setActiveExtension } from '$lib/stores/uiState.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    // Opens the app Settings dialog. Wired by App.svelte so the gear works
    // from every view (mail + any extension pane).
    onOpenSettings?: () => void
  }

  const { onOpenSettings }: Props = $props()

  // Mail is always present and always first; extensions follow in their
  // registered Order. The rail always renders now — it hosts the global
  // Settings gear at the bottom, so it must be reachable from every view.
  let active = $derived(getActiveExtension())
  let tabs = $derived(getRailTabs())

  function select(name: string) {
    setActiveExtension(name)
  }
</script>

<nav
  class="flex flex-col items-stretch w-12 flex-shrink-0 bg-muted/30 border-r border-border pt-2"
  aria-label="Active extension"
>
  <RailButton
    icon="mdi:email"
    label="Mail"
    active={active === 'mail'}
    onclick={() => select('mail')}
  />
  {#each tabs as tab (tab.extensionId)}
    <RailButton
      icon={tab.icon || 'mdi:puzzle'}
      label={tab.label}
      active={active === tab.extensionId}
      onclick={() => select(tab.extensionId)}
    />
  {/each}

  <!-- Settings — pinned to the bottom, available from mail AND extension views. -->
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
