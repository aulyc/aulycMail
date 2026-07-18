<script lang="ts">
  import Icon from '@iconify/svelte'
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

  const radius = 54
  const circumference = 2 * Math.PI * radius
  const progress = $derived(preparing ? null : store.progress)
  const active = $derived(preparing || store.running)
  const failed = $derived(startFailed || (!active && progress?.phase === 'error'))
  const finished = $derived(!active && !failed && progress?.phase === 'done')
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
  const ringClass = $derived(failed
    ? 'text-destructive'
    : finished && statistics.notBackedUp > 0
      ? 'text-amber-500'
      : 'text-primary')

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
      <div class="flex flex-col items-center text-center">
        <div
          class="relative h-36 w-36"
          role="progressbar"
          aria-label={$_('settingsBackup.progressTitle')}
          aria-valuemin="0"
          aria-valuemax="100"
          aria-valuenow={percent ?? undefined}
        >
          <svg viewBox="0 0 128 128" class={`h-full w-full ${ringClass}`} aria-hidden="true">
            <circle cx="64" cy="64" r={radius} fill="none" stroke="currentColor" stroke-width="8" class="opacity-15" />
            {#if active && percent === null}
              <circle
                cx="64"
                cy="64"
                r={radius}
                fill="none"
                stroke="currentColor"
                stroke-width="8"
                stroke-linecap="round"
                stroke-dasharray={`${circumference * 0.24} ${circumference * 0.76}`}
                class="origin-center animate-spin"
              />
            {:else}
              <circle
                cx="64"
                cy="64"
                r={radius}
                fill="none"
                stroke="currentColor"
                stroke-width="8"
                stroke-linecap="round"
                stroke-dasharray={circumference}
                stroke-dashoffset={circumference * (1 - (failed ? 1 : (percent ?? (finished ? 100 : 0))) / 100)}
                transform="rotate(-90 64 64)"
                class="transition-[stroke-dashoffset] duration-300"
              />
            {/if}
          </svg>
          <div class="absolute inset-0 flex flex-col items-center justify-center">
            {#if failed}
              <Icon icon="mdi:alert-outline" class="h-9 w-9 text-destructive" />
            {:else if finished}
              <Icon icon={statistics.notBackedUp > 0 ? 'mdi:alert-circle-check-outline' : 'mdi:check'} class={`h-10 w-10 ${ringClass}`} />
            {:else if percent !== null}
              <span class="text-2xl font-semibold tabular-nums text-foreground">{percent}%</span>
            {:else}
              <span class="text-sm font-medium text-muted-foreground">{$_('settingsBackup.preparing')}</span>
            {/if}
          </div>
        </div>

        {#if active && (progress?.total ?? 0) > 0}
          <p class="mt-3 text-sm font-medium tabular-nums text-foreground">
            {$_('settingsBackup.checkedProgress', {
              values: { current: progress?.current ?? 0, total: progress?.total ?? 0 },
            })}
          </p>
        {:else if active}
          <p class="mt-3 text-sm text-muted-foreground">{$_('settingsBackup.calculatingScope')}</p>
        {:else if finished}
          <p class="mt-3 text-sm font-medium tabular-nums text-foreground">
            {$_('settingsBackup.checkedTotal', { values: { count: statistics.checked } })}
          </p>
        {/if}
        {#if active && target}
          <p class="mt-1 max-w-full truncate text-xs text-muted-foreground" title={target}>{target}</p>
        {/if}
      </div>

      {#if !failed}
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
