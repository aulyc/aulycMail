<script lang="ts">
  import Icon from '@iconify/svelte'
  import { _ } from '$lib/i18n'
  import type { ActivityLog } from './activityLogTypes'
  import { activityStatusLabel, activitySummary, activityTime, activityTypeLabel, backupActivityDetails } from './activityLogFormat'
  interface Props { log: ActivityLog; expanded?: boolean; onToggle?: () => void }
  let { log, expanded = false, onToggle }: Props = $props()
  const backup = $derived(backupActivityDetails(log))
  const expandable = $derived(Boolean(backup || log.detail))
  const detailId = $derived(`activity-log-details-${log.id}`)
  const statusIcon = $derived(log.status === 'success' ? 'mdi:check-circle' : log.status === 'cancelled' ? 'mdi:cancel' : 'mdi:alert-circle')
  const statusClass = $derived(log.status === 'success' ? 'text-emerald-500' : log.status === 'partial' ? 'text-amber-500' : log.status === 'cancelled' ? 'text-muted-foreground' : 'text-destructive')
</script>

<article class="border-b border-border/70 py-3 last:border-b-0">
  <button
    type="button"
    class="flex w-full items-center gap-3 pr-4 text-left disabled:cursor-default"
    disabled={!expandable}
    aria-expanded={expandable ? expanded : undefined}
    aria-controls={expandable ? detailId : undefined}
    onclick={() => expandable && onToggle?.()}
  >
    <Icon icon={statusIcon} class="h-5 w-5 shrink-0 {statusClass}" />
    <span class="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">{activityTypeLabel(log.type)}</span>
    <span class="min-w-0 flex-1 truncate text-sm text-foreground" title={activitySummary(log)}>{activitySummary(log)}</span>
    <time class="shrink-0 text-xs tabular-nums text-muted-foreground/75">{activityTime(log.createdAt)}</time>
    <span class="w-14 shrink-0 text-right text-xs {statusClass}">{activityStatusLabel(log.status)}</span>
    {#if expandable}<Icon icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'} class="h-4 w-4 shrink-0 text-muted-foreground" />{/if}
  </button>
  {#if expanded && expandable}
    <div id={detailId} class="mt-3 mr-4 ml-8 rounded-lg border border-border/70 bg-muted/30 p-3">
      {#if backup}
        <div class="flex items-center justify-between gap-3 text-xs text-muted-foreground">
          <span>{backup.mode}</span>
          {#if backup.directory}<span class="min-w-0 truncate" title={backup.directory}>{$_('settingsBackup.directory')}：{backup.directory}</span>{/if}
        </div>

        <dl class="mt-3 grid grid-cols-3 gap-2">
          <div class="rounded-md bg-background px-3 py-2">
            <dt class="text-xs text-muted-foreground">{$_('settingsBackup.checked')}</dt>
            <dd class="mt-1 text-base font-semibold tabular-nums text-foreground">{backup.statistics.checked}</dd>
          </div>
          <div class="rounded-md bg-background px-3 py-2">
            <dt class="text-xs text-muted-foreground">{$_('settingsBackup.backedUp')}</dt>
            <dd class="mt-1 text-base font-semibold tabular-nums text-foreground">{backup.statistics.backedUp}</dd>
          </div>
          <div class="rounded-md bg-background px-3 py-2">
            <dt class="text-xs text-muted-foreground">{$_('settingsBackup.notBackedUp')}</dt>
            <dd class="mt-1 text-base font-semibold tabular-nums {backup.statistics.notBackedUp > 0 ? 'text-amber-500' : 'text-foreground'}">{backup.statistics.notBackedUp}</dd>
          </div>
        </dl>

        <div class="mt-2 grid grid-cols-2 gap-2">
          <section class="rounded-md bg-background px-3 py-2.5">
            <div class="flex items-baseline gap-2">
              <h4 class="shrink-0 text-xs font-medium text-foreground">{$_('settingsBackup.backedUp')}</h4>
              <p class="min-w-0 truncate text-[11px] text-muted-foreground">{$_('settingsBackup.backedUpComposition')}</p>
            </div>
            <dl class="mt-2 grid grid-cols-2 gap-2 text-xs">
              <div><dt class="text-muted-foreground">{$_('settingsBackup.newlyBackedUp')}</dt><dd class="mt-1 font-semibold tabular-nums text-foreground">{backup.statistics.newlyBackedUp}</dd></div>
              <div><dt class="text-muted-foreground">{$_('settingsBackup.previouslyBackedUp')}</dt><dd class="mt-1 font-semibold tabular-nums text-foreground">{backup.statistics.previouslyBackedUp}</dd></div>
            </dl>
          </section>
          <section class="rounded-md bg-background px-3 py-2.5">
            <div class="flex items-baseline gap-2">
              <h4 class="shrink-0 text-xs font-medium text-foreground">{$_('settingsBackup.notBackedUp')}</h4>
              <p class="min-w-0 truncate text-[11px] text-muted-foreground">{$_('settingsBackup.notBackedUpComposition')}</p>
            </div>
            <dl class="mt-2 grid grid-cols-3 gap-2 text-xs">
              <div><dt class="leading-4 text-muted-foreground">{$_('settingsBackup.serverNotReturned')}</dt><dd class="mt-1 font-semibold tabular-nums text-foreground">{backup.statistics.serverNotReturned}</dd></div>
              <div><dt class="leading-4 text-muted-foreground">{$_('settingsBackup.noReadableSource')}</dt><dd class="mt-1 font-semibold tabular-nums text-foreground">{backup.statistics.noReadableSource}</dd></div>
              <div><dt class="leading-4 text-muted-foreground">{$_('settingsBackup.processingFailed')}</dt><dd class="mt-1 font-semibold tabular-nums text-foreground">{backup.statistics.processingFailed}</dd></div>
            </dl>
          </section>
        </div>
      {/if}
      {#if log.detail}
        <p class="rounded-md bg-background p-2 text-xs leading-5 text-muted-foreground whitespace-pre-wrap {backup ? 'mt-2' : ''}">{log.detail}</p>
      {/if}
    </div>
  {/if}
</article>
