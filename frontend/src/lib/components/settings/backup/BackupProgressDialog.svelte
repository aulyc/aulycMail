<script lang="ts">
  import { backupProgressPercent, backupStatistics } from '$lib/backup/backupStatistics'
  import { Button } from '$lib/components/ui/button'
  import * as Dialog from '$lib/components/ui/dialog'
  import { _ } from '$lib/i18n'
  import type { BackupRunStore } from './backupRun.svelte'

  interface Props {
    open?: boolean
    store: BackupRunStore
    preparing?: boolean
    startFailed?: boolean
    onRunInBackground?: () => void
  }

  let {
    open = $bindable(false),
    store,
    preparing = false,
    startFailed = false,
    onRunInBackground,
  }: Props = $props()

  const progress = $derived(preparing ? null : store.progress)
  const active = $derived(preparing || store.running)
  const failed = $derived(startFailed || (!active && progress?.phase === 'error'))
  const finished = $derived(!active && !failed && progress?.phase === 'done')
  const checkingExisting = $derived(active && progress?.stage === 'checking')
  const percent = $derived(backupProgressPercent(progress?.current ?? 0, progress?.total ?? 0))
  const target = $derived([progress?.accountEmail, progress?.folderPath].filter(Boolean).join(' / '))
  const statistics = $derived(backupStatistics({
    total: progress?.total,
    exported: progress?.exported,
    skipped: progress?.skipped,
    missing: progress?.missing,
    unavailable: progress?.unavailable,
    failed: progress?.failed,
  }))
  const displayedPercent = $derived(finished ? 100 : (percent ?? 0))
  const progressClass = $derived(failed
    ? 'bg-destructive'
    : finished && statistics.notBackedUp > 0
      ? 'bg-amber-500'
      : 'bg-primary')

  function close() {
    if (active) {
      onRunInBackground?.()
      return
    }
    open = false
  }

  function handleOpenChange(next: boolean) {
    if (!next && active) {
      onRunInBackground?.()
      return
    }
    open = next
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-h-[88vh] w-[min(560px,92vw)] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0">
    <Dialog.Header class="border-b border-border px-6 py-5 pr-12">
      <Dialog.Title>{$_('settingsBackup.progressTitle')}</Dialog.Title>
      <Dialog.Description>
        {#if preparing}
          {$_('settingsBackup.preparingBackup')}
        {:else if failed}
          {$_('settingsBackup.backupFailedDescription')}
        {:else if checkingExisting}
          {$_('settingsBackup.checkingExistingDescription')}
        {:else if store.running}
          {$_('settingsBackup.backupInProgressDescription')}
        {:else if statistics.notBackedUp > 0}
          {$_('settingsBackup.backupFinishedWithIssues')}
        {:else}
          {$_('settingsBackup.backupFinished')}
        {/if}
      </Dialog.Description>
    </Dialog.Header>

    <div class="min-h-0 space-y-5 overflow-y-auto px-6 py-5 scrollbar-thin" aria-live="polite">
      <div class="space-y-3 py-1">
        <div class="flex items-center justify-between gap-4 text-sm">
          <p class="min-w-0 font-medium tabular-nums text-foreground">
            {#if checkingExisting}
              {$_('settingsBackup.checkingExisting')}
            {:else if active && (progress?.total ?? 0) > 0}
              {$_('settingsBackup.checkedProgress', {
                values: { current: progress?.current ?? 0, total: progress?.total ?? 0 },
              })}
            {:else if active}
              {$_('settingsBackup.calculatingScope')}
            {:else if finished}
              {$_('settingsBackup.checkedTotal', { values: { count: statistics.checked } })}
            {:else}
              {$_('settingsBackup.backupFailed')}
            {/if}
          </p>
          {#if !checkingExisting && (percent !== null || finished)}
            <span class="shrink-0 font-semibold tabular-nums text-foreground">{displayedPercent}%</span>
          {/if}
        </div>

        <div
          class="relative h-2.5 overflow-hidden rounded-full bg-muted"
          role="progressbar"
          aria-label={$_('settingsBackup.progressTitle')}
          aria-valuemin="0"
          aria-valuemax="100"
          aria-valuenow={checkingExisting ? undefined : (percent ?? (finished ? 100 : undefined))}
        >
          {#if active && (percent === null || checkingExisting)}
            <div class="backup-progress-indeterminate absolute inset-y-0 w-2/5 rounded-full bg-primary"></div>
          {:else}
            <div
              class={`h-full rounded-full transition-[width] duration-300 ${progressClass}`}
              style={`width: ${displayedPercent}%`}
            ></div>
          {/if}
        </div>

        <p
          class="max-w-full truncate text-xs text-muted-foreground {active && target ? '' : 'invisible'}"
          title={active && target ? target : undefined}
          aria-hidden={!(active && target)}
        >
          {active && target ? target : '\u00a0'}
        </p>
      </div>

      {#if !failed && !checkingExisting}
        <section class="rounded-lg border border-border bg-muted/20 p-4">
          <div class="flex items-baseline justify-between gap-4">
            <div>
              <h3 class="text-sm font-semibold text-foreground">{$_('settingsBackup.backedUp')}</h3>
              <p class="mt-0.5 text-xs text-muted-foreground">{$_('settingsBackup.backedUpComposition')}</p>
            </div>
            <span class="text-xl font-semibold tabular-nums text-foreground">{statistics.backedUp}</span>
          </div>
          <dl class="mt-3 grid grid-cols-2 gap-3">
            <div class="rounded-md bg-background px-3 py-2.5">
              <dt class="text-xs text-muted-foreground">{$_('settingsBackup.newlyBackedUp')}</dt>
              <dd class="mt-1 text-base font-medium tabular-nums text-foreground">{statistics.newlyBackedUp}</dd>
            </div>
            <div class="rounded-md bg-background px-3 py-2.5">
              <dt class="text-xs text-muted-foreground">{$_('settingsBackup.previouslyBackedUp')}</dt>
              <dd class="mt-1 text-base font-medium tabular-nums text-foreground">{statistics.previouslyBackedUp}</dd>
            </div>
          </dl>
        </section>

        <section class="rounded-lg border border-border bg-muted/20 p-4">
          <div class="flex items-baseline justify-between gap-4">
            <div>
              <h3 class="text-sm font-semibold text-foreground">{$_('settingsBackup.notBackedUp')}</h3>
              <p class="mt-0.5 text-xs text-muted-foreground">{$_('settingsBackup.notBackedUpComposition')}</p>
            </div>
            <span class="text-xl font-semibold tabular-nums {statistics.notBackedUp > 0 ? 'text-amber-500' : 'text-foreground'}">{statistics.notBackedUp}</span>
          </div>
          <dl class="mt-3 grid grid-cols-3 gap-3">
            <div class="rounded-md bg-background px-3 py-2.5">
              <dt class="text-xs leading-4 text-muted-foreground">{$_('settingsBackup.serverNotReturned')}</dt>
              <dd class="mt-1 text-base font-medium tabular-nums text-foreground">{statistics.serverNotReturned}</dd>
            </div>
            <div class="rounded-md bg-background px-3 py-2.5">
              <dt class="text-xs leading-4 text-muted-foreground">{$_('settingsBackup.noReadableSource')}</dt>
              <dd class="mt-1 text-base font-medium tabular-nums text-foreground">{statistics.noReadableSource}</dd>
            </div>
            <div class="rounded-md bg-background px-3 py-2.5">
              <dt class="text-xs leading-4 text-muted-foreground">{$_('settingsBackup.processingFailed')}</dt>
              <dd class="mt-1 text-base font-medium tabular-nums text-foreground">{statistics.processingFailed}</dd>
            </div>
          </dl>
        </section>
      {/if}
    </div>

    <footer class="flex justify-end border-t border-border px-6 py-4">
      <Button variant={active ? 'outline' : 'default'} onclick={close}>
        {active ? $_('settingsBackup.runInBackground') : $_('common.close')}
      </Button>
    </footer>
  </Dialog.Content>
</Dialog.Root>

<style>
  @keyframes backup-progress-slide {
    from { transform: translateX(-100%); }
    to { transform: translateX(250%); }
  }

  .backup-progress-indeterminate {
    animation: backup-progress-slide 1.2s ease-in-out infinite;
  }

  @media (prefers-reduced-motion: reduce) {
    .backup-progress-indeterminate {
      animation: none;
    }
  }
</style>
