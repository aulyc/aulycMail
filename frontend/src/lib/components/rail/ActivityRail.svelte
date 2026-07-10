<script lang="ts">
  import Icon from '@iconify/svelte'
  import RailButton from './RailButton.svelte'
  import { getActivePane, setActivePane } from '$lib/stores/uiState.svelte'
  import { BUILT_IN_RAIL_PANES } from '$lib/rail/panes'
  import { _ } from '$lib/i18n'
  import { syncLog } from '$lib/stores/syncLog.svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'

  interface Props {
    // Opens the app Settings dialog. Wired by App.svelte so the gear works
    // from every view (Mail + Contacts).
    onOpenSettings?: () => void
    // Opens the sync/connection log dialog.
    onOpenLog?: () => void
  }

  const { onOpenSettings, onOpenLog }: Props = $props()

  // Mail is always present and always first; Contacts is a fixed built-in pane.
  // The rail always renders because it also hosts global Settings and sync log.
  let active = $derived(getActivePane())

  function select(name: string) {
    setActivePane(name)
  }

  async function toggleSync() {
    try {
      if (accountStore.isAnySyncing) {
        await accountStore.cancelAllSyncs()
        return
      }
      await accountStore.syncAllComplete()
    } catch (err) {
      console.error('Rail sync failed:', err)
    }
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

  <!-- Sync: pinned above the sync log and Settings. -->
  <button
    class="mt-auto relative flex items-center justify-center w-12 h-12 border-l-[3px] border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30 transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
    type="button"
    title={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
    aria-label={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
    onclick={toggleSync}
  >
    <Icon
      icon="mdi:sync"
      width="22"
      height="22"
      class={accountStore.isAnySyncing ? 'animate-spin text-primary' : ''}
    />
  </button>

  <!-- Sync/connection log: pinned to the bottom, just above Settings. -->
  <button
    class="relative flex items-center justify-center w-12 h-12 border-l-[3px] border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30 transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
    type="button"
    title={$_('syncLog.title')}
    aria-label={$_('syncLog.title')}
    onclick={() => onOpenLog?.()}
  >
    <Icon icon="mdi:history" width="22" height="22" />
    {#if syncLog.unseenErrors > 0}
      <span class="absolute top-2 right-2 w-2 h-2 rounded-full bg-destructive"></span>
    {/if}
  </button>

  <!-- Settings: pinned to the bottom, available from Mail and Contacts. -->
  <button
    class="mb-2 flex items-center justify-center w-12 h-12 border-l-[3px] border-l-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30 transition-colors duration-150 cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
    type="button"
    title={$_('sidebar.settings')}
    aria-label={$_('sidebar.settings')}
    onclick={() => onOpenSettings?.()}
  >
    <Icon icon="mdi:cog" width="22" height="22" />
  </button>
</nav>
