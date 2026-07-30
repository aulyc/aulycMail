<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { _ } from '$lib/i18n'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import ActivityLogFilters from './ActivityLogFilters.svelte'
  import ActivityLogList from './ActivityLogList.svelte'
  import { ActivityLogsStore } from './activityLogs.svelte'
  interface Props {
    initialType?: string
    onLoadMoreFinished?: (hasMore: boolean) => void
  }
  let { initialType = '', onLoadMoreFinished }: Props = $props()
  const store = new ActivityLogsStore()
  onMount(() => { store.type = initialType; store.start(); void store.refresh() })
  onDestroy(() => store.stop())
</script>

<div class="flex h-full min-h-0 flex-col gap-5">
  <div class="shrink-0"><SettingsPageHeader description={$_('activityLog.description')} /></div>
  <div class="shrink-0"><ActivityLogFilters {store} /></div>
  <ActivityLogList {store} {onLoadMoreFinished} />
</div>
