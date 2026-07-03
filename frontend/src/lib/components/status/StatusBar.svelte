<script lang="ts">
  import Icon from '@iconify/svelte'
  import { formatDistanceToNow } from 'date-fns'
  import { _ } from '$lib/i18n'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'
  import { toasts, type Toast, type ToastAction } from '$lib/stores/toast'

  const toastIcons = {
    success: 'mdi:check-circle',
    error: 'mdi:alert-circle',
    info: 'mdi:information',
    warning: 'mdi:alert',
  } as const

  const toastIconClasses = {
    success: 'text-green-500',
    error: 'text-red-500',
    info: 'text-blue-500',
    warning: 'text-yellow-500',
  } as const

  let latestToast = $derived<Toast | null>($toasts[$toasts.length - 1] ?? null)

  let syncStatus = $derived.by<{ accountName: string | null; label: string; percentage: number | null }>(() => {
    if (accountStore.isAnySyncing) {
      const syncingAccount = accountStore.accounts.find((account) => account.syncing)
      if (syncingAccount) {
        const accountName = syncingAccount.account.name
        const progress = accountStore.getSyncProgress(syncingAccount.account.id)

        if (progress) {
          if (progress.phase === 'folders') {
            return { accountName, label: $_('sidebar.syncingFolders'), percentage: null }
          }
          if (progress.phase === 'messages') {
            return { accountName, label: $_('sidebar.fetchingMessageList'), percentage: null }
          }
          if (progress.phase === 'headers') {
            return {
              accountName,
              label: $_('sidebar.fetchingHeaders', { values: { percentage: progress.percentage } }),
              percentage: progress.percentage,
            }
          }
          return {
            accountName,
            label: $_('sidebar.syncingContent', { values: { percentage: progress.percentage } }),
            percentage: progress.percentage,
          }
        }

        return { accountName, label: $_('sidebar.syncing'), percentage: null }
      }

      return { accountName: null, label: $_('sidebar.syncing'), percentage: null }
    }

    if (!accountStore.isOnline) {
      return { accountName: null, label: $_('sidebar.offline'), percentage: null }
    }

    if (!accountStore.lastSyncTime) {
      return { accountName: null, label: $_('sidebar.notSynced'), percentage: null }
    }

    return {
      accountName: null,
      label: $_('sidebar.synced', {
        values: {
          time: formatDistanceToNow(accountStore.lastSyncTime, {
            addSuffix: true,
            locale: getCurrentDateFnsLocale(),
          }),
        },
      }),
      percentage: null,
    }
  })

  async function handleSyncClick() {
    try {
      if (accountStore.isAnySyncing) {
        await accountStore.cancelAllSyncs()
        return
      }
      await accountStore.syncAllComplete()
    } catch (err) {
      console.error('Status bar sync failed:', err)
    }
  }

  function handleToastAction(toast: Toast, action: ToastAction) {
    action.onClick()
    toasts.remove(toast.id)
  }
</script>

<footer
  class="relative h-8 shrink-0 border-t border-border bg-muted/40 text-xs text-muted-foreground
         flex items-center overflow-hidden"
  aria-label={$_('aria.statusBar')}
>
  {#if syncStatus.percentage !== null}
    <div class="absolute top-0 left-0 right-0 h-0.5 bg-muted overflow-hidden">
      <div
        class="h-full bg-primary transition-all duration-300 ease-out"
        style="width: {syncStatus.percentage}%"
      ></div>
    </div>
  {/if}

  <button
    type="button"
    class="h-full min-w-0 w-[360px] max-w-[42vw] border-r border-border px-3
           flex items-center gap-2 hover:text-foreground transition-colors text-left"
    onclick={handleSyncClick}
    title={$_(accountStore.isAnySyncing ? 'sidebar.clickToCancel' : 'sidebar.syncAllAccounts')}
  >
    <Icon
      icon="mdi:sync"
      class="w-4 h-4 shrink-0 {accountStore.isAnySyncing ? 'animate-spin text-primary' : ''}"
    />
    <span class="min-w-0 truncate">
      {#if syncStatus.accountName}
        <span class="text-foreground">{syncStatus.accountName}</span>
        <span class="mx-1 text-muted-foreground">·</span>
      {/if}
      {syncStatus.label}
    </span>
  </button>

  <div class="min-w-0 flex-1 px-3 flex items-center justify-end">
    {#if latestToast}
      <div
        class="min-w-0 max-w-full flex items-center gap-2"
        role={latestToast.type === 'error' ? 'alert' : 'status'}
      >
        <Icon
          icon={toastIcons[latestToast.type]}
          class="w-4 h-4 shrink-0 {toastIconClasses[latestToast.type]}"
        />
        <span class="truncate text-foreground">{latestToast.message}</span>

        {#if latestToast.actions && latestToast.actions.length > 0}
          <div class="shrink-0 flex items-center gap-1">
            {#each latestToast.actions as action (action.label)}
              <button
                type="button"
                class="px-2 py-0.5 rounded text-primary hover:bg-primary/10 transition-colors font-medium"
                onclick={() => handleToastAction(latestToast, action)}
              >
                {action.label}
              </button>
            {/each}
          </div>
        {/if}

        <button
          type="button"
          class="p-0.5 rounded hover:bg-accent hover:text-foreground transition-colors shrink-0"
          onclick={() => toasts.remove(latestToast.id)}
          aria-label={$_('aria.dismiss')}
        >
          <Icon icon="mdi:close" class="w-4 h-4" />
        </button>
      </div>
    {/if}
  </div>
</footer>
