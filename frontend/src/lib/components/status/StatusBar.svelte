<script lang="ts">
  import Icon from '@iconify/svelte'
  import { formatDistanceToNow } from 'date-fns'
  import { _ } from '$lib/i18n'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'
  import { toasts, type Toast } from '$lib/stores/toast'

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

  <div
    class="h-full min-w-0 w-[360px] max-w-[42vw] px-3
           flex items-center text-left"
  >
    <span class="min-w-0 truncate">
      {#if syncStatus.accountName}
        <span class="text-foreground">{syncStatus.accountName}</span>
        {' '}
      {/if}
      {syncStatus.label}
    </span>
  </div>

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
      </div>
    {/if}
  </div>
</footer>
