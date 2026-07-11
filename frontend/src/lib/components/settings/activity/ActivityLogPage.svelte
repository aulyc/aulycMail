<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { _ } from '$lib/i18n'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import ActivityLogFilters from './ActivityLogFilters.svelte'
  import ActivityLogList from './ActivityLogList.svelte'
  import ActivityLogClearMenu from './ActivityLogClearMenu.svelte'
  import { ActivityLogsStore } from './activityLogs.svelte'
  const store = new ActivityLogsStore()
  onMount(() => { store.start(); void store.refresh() })
  onDestroy(() => store.stop())
</script>

<div class="space-y-5">
  <SettingsPageHeader title={$_('activityLog.title')} description={$_('activityLog.description')} />
  <ActivityLogFilters {store} />
  <ActivityLogList {store} />
  <div class="flex justify-end"><ActivityLogClearMenu {store} /></div>
</div>
