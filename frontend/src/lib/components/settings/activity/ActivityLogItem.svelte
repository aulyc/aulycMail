<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import type { ActivityLog } from './activityLogTypes'
  import { activityStatusLabel, activitySummary, activityTime, activityTitle, activityTypeLabel } from './activityLogFormat'
  interface Props { log: ActivityLog }
  let { log }: Props = $props()
  let expanded = $state(false)
  const statusIcon = $derived(log.status === 'success' ? 'mdi:check-circle' : log.status === 'cancelled' ? 'mdi:cancel' : 'mdi:alert-circle')
  const statusClass = $derived(log.status === 'success' ? 'text-emerald-500' : log.status === 'partial' ? 'text-amber-500' : log.status === 'cancelled' ? 'text-muted-foreground' : 'text-destructive')
</script>

<article class="border-b border-border/70 px-4 py-3 last:border-b-0">
  <button type="button" class="flex w-full items-start gap-3 text-left" onclick={() => log.detail && (expanded = !expanded)}>
    <Icon icon={statusIcon} class="mt-0.5 h-5 w-5 shrink-0 {statusClass}" />
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <span class="rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">{activityTypeLabel(log.type)}</span>
        <span class="min-w-0 flex-1 truncate text-sm font-medium">{activityTitle(log)}</span>
        <span class="text-xs {statusClass}">{activityStatusLabel(log.status)}</span>
      </div>
      <p class="mt-1 text-sm text-muted-foreground">{activitySummary(log)}</p>
      <p class="mt-1 text-xs text-muted-foreground/75">{activityTime(log.createdAt)}</p>
      {#if expanded && log.detail}<p class="mt-2 rounded-md bg-muted/60 p-2 text-xs leading-5 text-muted-foreground whitespace-pre-wrap">{log.detail}</p>{/if}
    </div>
    {#if log.detail}<Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="mt-1 h-4 w-4 text-muted-foreground" />{/if}
  </button>
</article>
