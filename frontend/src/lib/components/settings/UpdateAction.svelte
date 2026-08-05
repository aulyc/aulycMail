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
  <button
    type="button"
    class="flex w-full flex-col items-center gap-1 rounded-md px-1 py-1.5 text-center text-xs transition-colors hover:bg-background/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-default"
    disabled={busy}
    onclick={activate}
    aria-label={statusText || $_('settingsUpdate.checkNow')}
  >
    {#if busy}<Icon icon="mdi:loading" class="h-3.5 w-3.5 animate-spin text-muted-foreground" />{/if}
    <span class={statusClass}>{statusText}</span>
  </button>
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
  loading={updateStore.status.state === 'downloading' || updateStore.status.state === 'verifying'}
  onConfirm={install}
  onCancel={() => confirmOpen = false}
/>
