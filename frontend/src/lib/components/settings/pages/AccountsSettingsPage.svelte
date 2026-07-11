<script lang="ts">
  import { _ } from '$lib/i18n'
  import AccountsTab from '../AccountsTab.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
</script>
<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.accounts')} />
  <AccountsTab />
</div>
