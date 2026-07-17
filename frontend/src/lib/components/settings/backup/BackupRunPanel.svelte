<script lang="ts">
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import { backupStatistics } from '$lib/backup/backupStatistics'
  import type { BackupRunStore } from './backupRun.svelte'
  interface Props { store: BackupRunStore; canStart: boolean; saveBeforeStart: boolean; onStart: () => void | Promise<void> }
  let { store, canStart, saveBeforeStart, onStart }: Props = $props()
  const percent = $derived(store.progress?.total ? Math.min(100, Math.round(store.progress.current / store.progress.total * 100)) : 0)
  const target = $derived([store.progress?.accountEmail, store.progress?.folderPath].filter(Boolean).join(' / '))
  const statistics = $derived(backupStatistics({
    total: store.progress?.total,
    exported: store.progress?.exported,
    skipped: store.progress?.skipped,
    missing: store.progress?.missing,
    unavailable: store.progress?.unavailable,
    failed: store.progress?.failed,
  }))
  const checked = $derived(store.running && store.progress?.total
    ? `${store.progress.current}/${store.progress.total}`
    : statistics.checked)
</script>

<section class="relative flex min-h-20 min-w-0 items-center gap-3 border-b border-border/75 py-2">
  <h3 class="shrink-0 text-sm font-semibold text-foreground">
    {store.running ? $_('settingsBackup.currentTask') : store.progress ? $_('settingsBackup.latestResult') : $_('settingsBackup.currentTask')}
  </h3>

  {#if !(store.running || store.progress)}
    <p class="min-w-0 flex-1 truncate text-sm text-muted-foreground">{$_('settingsBackup.noCurrentTask')}</p>
  {:else}
    <div class="min-w-0 flex-1">
      <dl class="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm text-foreground">
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.checked')}</dt><dd class="tabular-nums">{checked}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.backedUp')}</dt><dd class="tabular-nums">{statistics.backedUp}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.notBackedUp')}</dt><dd class="tabular-nums">{statistics.notBackedUp}</dd></div>
      </dl>
      <dl class="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs text-muted-foreground">
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.newlyBackedUp')}</dt><dd class="tabular-nums">{statistics.newlyBackedUp}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.previouslyBackedUp')}</dt><dd class="tabular-nums">{statistics.previouslyBackedUp}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.serverNotReturned')}</dt><dd class="tabular-nums">{statistics.serverNotReturned}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.noReadableSource')}</dt><dd class="tabular-nums">{statistics.noReadableSource}</dd></div>
        <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.processingFailed')}</dt><dd class="tabular-nums">{statistics.processingFailed}</dd></div>
      </dl>
      {#if target}<p class="mt-1 truncate text-xs text-muted-foreground" title={target}>{target}</p>{/if}
    </div>
  {/if}

  <Button size="sm" class="shrink-0" onclick={onStart} disabled={!canStart || store.running || store.loading}>
    {store.running ? $_('settingsBackup.backupRunning') : saveBeforeStart ? $_('settingsBackup.saveAndStart') : $_('settingsBackup.startBackup')}
  </Button>

  {#if store.running || store.progress}
    <div class="absolute inset-x-0 bottom-0 h-1 overflow-hidden rounded-full bg-muted" role="progressbar" aria-label={store.running ? $_('settingsBackup.currentTask') : $_('settingsBackup.latestResult')} aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent}>
      <div class="h-full rounded-full bg-primary transition-all" style={`width:${percent}%`}></div>
    </div>
  {/if}
</section>
