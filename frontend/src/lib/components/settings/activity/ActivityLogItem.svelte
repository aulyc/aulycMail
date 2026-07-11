<script lang="ts">
  import Icon from '@iconify/svelte'
  import type { ActivityLog } from './activityLogTypes'
  import { activityStatusLabel, activitySummary, activityTime, activityTypeLabel } from './activityLogFormat'
  interface Props { log: ActivityLog }
  let { log }: Props = $props()
  let expanded = $state(false)
  const statusIcon = $derived(log.status === 'success' ? 'mdi:check-circle' : log.status === 'cancelled' ? 'mdi:cancel' : 'mdi:alert-circle')
  const statusClass = $derived(log.status === 'success' ? 'text-emerald-500' : log.status === 'partial' ? 'text-amber-500' : log.status === 'cancelled' ? 'text-muted-foreground' : 'text-destructive')
</script>

<article class="border-b border-border/70 px-4 py-3 last:border-b-0">
  <button type="button" class="flex w-full items-center gap-3 text-left disabled:cursor-default" disabled={!log.detail} onclick={() => log.detail && (expanded = !expanded)}>
    <Icon icon={statusIcon} class="h-5 w-5 shrink-0 {statusClass}" />
    <span class="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">{activityTypeLabel(log.type)}</span>
    <span class="min-w-0 flex-1 truncate text-sm text-foreground" title={activitySummary(log)}>{activitySummary(log)}</span>
    <time class="shrink-0 text-xs tabular-nums text-muted-foreground/75">{activityTime(log.createdAt)}</time>
    <span class="w-14 shrink-0 text-right text-xs {statusClass}">{activityStatusLabel(log.status)}</span>
    {#if log.detail}<Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="h-4 w-4 shrink-0 text-muted-foreground" />{/if}
  </button>
  {#if expanded && log.detail}<p class="mt-2 ml-8 rounded-md bg-muted/60 p-2 text-xs leading-5 text-muted-foreground whitespace-pre-wrap">{log.detail}</p>{/if}
</article>
