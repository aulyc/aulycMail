<script lang="ts">
  import { _ } from '$lib/i18n'
  import type { ActivityLogsStore } from './activityLogs.svelte'
  interface Props { store: ActivityLogsStore }
  let { store }: Props = $props()
  const filters = $derived([
    { key: 'all', label: $_('activityLog.filters.all'), type: '', problem: false },
    { key: 'sync', label: $_('activityLog.filters.sync'), type: 'sync', problem: false },
    { key: 'backup', label: $_('activityLog.filters.backup'), type: 'backup', problem: false },
    { key: 'failed', label: $_('activityLog.filters.failed'), type: '', problem: true },
  ])
  const active = $derived(store.problemOnly ? 'failed' : store.type || 'all')
</script>

<div class="flex flex-wrap items-center justify-between gap-3">
  <div class="inline-flex rounded-lg bg-muted p-1">
    {#each filters as filter (filter.key)}
      <button type="button" class="rounded-md px-3 py-1.5 text-xs font-medium transition {active === filter.key ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}" onclick={() => store.setFilter(filter.type, filter.problem)}>{filter.label}</button>
    {/each}
  </div>
  <label class="flex items-center gap-2 text-xs text-muted-foreground">
    <span>{$_('activityLog.date')}</span>
    <input type="date" value={store.date} onchange={(event) => { store.date = event.currentTarget.value; void store.refresh() }} class="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground" />
    {#if store.date}<button type="button" class="hover:text-foreground" onclick={() => { store.date = ''; void store.refresh() }}>{$_('activityLog.allDates')}</button>{/if}
  </label>
</div>
