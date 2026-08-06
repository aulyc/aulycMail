<script lang="ts">
  import Icon from '@iconify/svelte'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { _ } from '$lib/i18n'
  import { updateStore } from '$lib/stores/update.svelte'
  import { addToast } from '$lib/stores/toast'

  interface Props { compact?: boolean }
  let { compact = false }: Props = $props()
  let confirmOpen = $state(false)

  updateStore.start()

  const busy = $derived(['checking', 'downloading', 'verifying', 'installing'].includes(updateStore.status.state))
  const installingUpdate = $derived(['downloading', 'verifying', 'installing'].includes(updateStore.status.state))
  const installProgress = $derived(Math.max(0, Math.min(100, Math.round(updateStore.status.progress ?? 0))))
  const statusText = $derived.by(() => {
    const status = updateStore.status
    switch (status.state) {
      case 'checking': return $_('settingsUpdate.checking')
      case 'upToDate': return $_('settingsUpdate.upToDate')
      case 'available': return $_('settingsUpdate.available', { values: { version: status.latestVersion ?? '' } })
      case 'downloading': return $_('settingsUpdate.downloading', { values: { progress: status.progress ?? 0 } })
      case 'verifying': return $_('settingsUpdate.verifying')
      case 'installing': return $_('settingsUpdate.installing')
      case 'failed': return status.failureOperation === 'install'
        ? $_('settingsUpdate.installFailedRetry')
        : $_('settingsUpdate.failed')
      default: return compact ? $_('settingsUpdate.checkNow') : ''
    }
  })
  const compactStatusText = $derived(
    updateStore.status.state === 'available'
      ? $_('settingsUpdate.availableCompact')
      : statusText,
  )
  const installProgressText = $derived.by(() => {
    switch (updateStore.status.state) {
      case 'downloading': return $_('settingsUpdate.downloadingProgress')
      case 'verifying': return $_('settingsUpdate.verifying')
      case 'installing': return $_('settingsUpdate.installing')
      default: return ''
    }
  })
  const statusClass = $derived(
    updateStore.status.state === 'upToDate'
      ? 'text-emerald-500'
      : updateStore.status.state === 'available'
        ? 'text-red-500'
        : updateStore.status.state === 'failed'
          ? 'text-amber-500'
          : 'text-muted-foreground',
  )

  async function activate() {
    if (busy) return
    if (updateStore.status.state === 'available' && updateStore.status.canInstall) {
      confirmOpen = true
      return
    }
    try {
      await updateStore.check()
    } catch (error) {
      console.error('Failed to check for updates:', error)
      addToast({ type: 'error', message: $_('settingsUpdate.checkFailed') })
    }
  }

  async function install() {
    try {
      await updateStore.install()
    } catch (error) {
      console.error('Failed to install update:', error)
      addToast({ type: 'error', message: $_('settingsUpdate.installFailed') })
    }
  }
</script>

{#if compact}
  <div class="flex w-full items-center gap-2 py-1.5 text-xs">
    <span data-update-title class="shrink-0 font-semibold text-muted-foreground">{$_('settingsUpdate.systemUpdate')}</span>
    <button
      type="button"
      class="inline-flex min-h-7 min-w-0 items-center justify-center gap-1.5 rounded-md border border-input bg-background px-2.5 py-1 font-medium shadow-sm transition-colors hover:bg-accent focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-default disabled:opacity-60"
      disabled={busy}
      onclick={activate}
      aria-label={statusText || $_('settingsUpdate.checkNow')}
    >
      {#if busy}<Icon icon="mdi:loading" class="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" />{/if}
      <span data-update-status class="truncate {statusClass}">{compactStatusText}</span>
    </button>
  </div>
{:else}
  <button
    type="button"
    data-settings-focus-style="link"
    class="-mx-2 inline-flex min-h-7 w-fit items-center gap-2 rounded-md px-2 text-left text-sm text-primary transition-colors hover:bg-primary/5 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-default disabled:no-underline"
    disabled={busy}
    onclick={activate}
  >
    <span>{$_('settingsUpdate.softwareUpdate')}</span>
    {#if statusText}<span class="ml-1 text-xs no-underline {statusClass}">{statusText}</span>{/if}
  </button>
{/if}

<ConfirmDialog
  bind:open={confirmOpen}
  title={$_('settingsUpdate.confirmTitle')}
  description={$_('settingsUpdate.confirmDescription', { values: { version: updateStore.status.latestVersion ?? '' } })}
  confirmLabel={$_('settingsUpdate.updateAndRestart')}
  cancelLabel={$_('common.cancel')}
  loading={installingUpdate}
  loadingPresentation="footer-progress"
  loadingLabel={installProgressText}
  loadingProgress={installProgress}
  onConfirm={install}
  onCancel={() => confirmOpen = false}
/>
