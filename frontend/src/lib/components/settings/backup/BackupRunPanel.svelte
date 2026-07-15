<script lang="ts">
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { BackupRunStore } from './backupRun.svelte'
  interface Props { store: BackupRunStore; canStart: boolean; saveBeforeStart: boolean; onStart: () => void | Promise<void> }
  let { store, canStart, saveBeforeStart, onStart }: Props = $props()
  const percent = $derived(store.progress?.total ? Math.min(100, Math.round(store.progress.current / store.progress.total * 100)) : 0)
  const target = $derived([store.progress?.accountEmail, store.progress?.folderPath].filter(Boolean).join(' / '))
</script>

<section class="relative flex h-14 min-w-0 items-center gap-3 border-b border-border/75">
  <h3 class="shrink-0 text-sm font-semibold text-foreground">{$_('settingsBackup.currentTask')}</h3>

  {#if !(store.running || store.progress)}
    <p class="min-w-0 flex-1 truncate text-sm text-muted-foreground">{$_('settingsBackup.noCurrentTask')}</p>
  {/if}

  <dl class="flex shrink-0 items-baseline gap-x-3 text-xs text-muted-foreground">
    <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.progressExported')}</dt><dd class="tabular-nums">{store.progress?.exported ?? 0}</dd></div>
    <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.progressSkipped')}</dt><dd class="tabular-nums">{store.progress?.skipped ?? 0}</dd></div>
    <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.progressMissing')}</dt><dd class="tabular-nums">{store.progress?.missing ?? 0}</dd></div>
    <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.progressUnavailable')}</dt><dd class="tabular-nums">{store.progress?.unavailable ?? 0}</dd></div>
    <div class="flex items-baseline gap-1 whitespace-nowrap"><dt>{$_('settingsBackup.progressFailed')}</dt><dd class="tabular-nums">{store.progress?.failed ?? 0}</dd></div>
  </dl>

  {#if target}
    <span class="min-w-0 flex-1 truncate text-xs text-muted-foreground" title={target}>· {target}</span>
  {:else if store.running || store.progress}
    <span class="min-w-0 flex-1"></span>
  {/if}

  <Button size="sm" class="shrink-0" onclick={onStart} disabled={!canStart || store.running || store.loading}>
    {store.running ? $_('settingsBackup.backupRunning') : saveBeforeStart ? $_('settingsBackup.saveAndStart') : $_('settingsBackup.startBackup')}
  </Button>

  {#if store.running || store.progress}
    <div class="absolute inset-x-0 bottom-0 h-1 overflow-hidden rounded-full bg-muted" role="progressbar" aria-label={$_('settingsBackup.currentTask')} aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent}>
      <div class="h-full rounded-full bg-primary transition-all" style={`width:${percent}%`}></div>
    </div>
  {/if}
</section>
