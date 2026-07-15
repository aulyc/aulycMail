<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { ActivityLogsStore } from './activityLogs.svelte'
  import ActivityLogItem from './ActivityLogItem.svelte'
  interface Props { store: ActivityLogsStore }
  let { store }: Props = $props()
</script>

<div class="min-h-0 flex-1 overflow-y-auto border-y border-border/70 scrollbar-thin">
  {#if store.loading && store.entries.length === 0}
    <div class="flex justify-center py-12"><Icon icon="mdi:loading" class="h-6 w-6 animate-spin text-muted-foreground" /></div>
  {:else if store.loadFailed && store.entries.length === 0}
    <div class="py-12 text-center text-sm text-destructive">{$_('activityLog.loadFailed')}</div>
  {:else if store.entries.length === 0}
    <div class="py-12 text-center text-sm text-muted-foreground">{$_('activityLog.empty')}</div>
  {:else}
    {#each store.entries as log (log.id)}<ActivityLogItem {log} />{/each}
    {#if store.hasMore}
      <div class="flex justify-center p-3"><Button variant="ghost" size="sm" onclick={() => store.loadMore()} disabled={store.loading}>{$_('activityLog.loadMore')}</Button></div>
    {/if}
  {/if}
</div>
