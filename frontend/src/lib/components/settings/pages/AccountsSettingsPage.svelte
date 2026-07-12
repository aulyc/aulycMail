<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import AccountsTab from '../AccountsTab.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  let accountsTab: AccountsTab | null = null
</script>
<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.accounts')}>
    {#snippet action()}
      <button type="button" class="flex h-6 w-6 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-muted hover:text-foreground" title={$_('settingsAccounts.addAccount')} aria-label={$_('settingsAccounts.addAccount')} onclick={() => accountsTab?.openAdd()}>
        <Icon icon="mdi:plus" class="h-5 w-5" />
      </button>
    {/snippet}
  </SettingsPageHeader>
  <AccountsTab bind:this={accountsTab} />
</div>
