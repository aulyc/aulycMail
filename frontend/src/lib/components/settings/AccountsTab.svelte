<script lang="ts">
  import { untrack } from 'svelte'
  import Icon from '@iconify/svelte'
  import { formatDistanceToNow } from 'date-fns'
  import { Button } from '$lib/components/ui/button'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { getCurrentDateFnsLocale } from '$lib/stores/settings.svelte'
  import AccountDialog from './AccountDialog.svelte'
  import DeleteAccountDialog from './DeleteAccountDialog.svelte'
  import ConnectionTestDialog from './ConnectionTestDialog.svelte'
  // @ts-ignore - wailsjs path
  import type { account } from '../../../../wailsjs/go/models'
  import { _ } from '$lib/i18n'

  // Filter out shared mailboxes — they're managed from the parent account's Identity tab
  const regularAccounts = $derived(accountStore.accounts.filter(acc => !acc.account.sharedMailboxParentId))

  // Dialog state
  let showAccountDialog = $state(false)
  let editingAccount = $state<account.Account | null>(null)

  let showDeleteDialog = $state(false)
  let deletingAccount = $state<account.Account | null>(null)

  // Connection-test popup + per-account last-successful-connection timestamps.
  let showTestDialog = $state(false)
  let testing = $state(false)
  let testResult = $state<{ success: boolean; message: string } | null>(null)
  let connOk = $state<Record<string, string>>({})

  // Lazily load each account's last-OK timestamp. Tracks the account list so
  // new accounts get loaded; the connOk read/write is untracked to avoid a loop.
  $effect(() => {
    const ids = regularAccounts.map(a => a.account.id)
    untrack(() => {
      for (const id of ids) {
        if (!(id in connOk)) {
          accountStore.getAccountConnOK(id)
            .then(v => { connOk = { ...connOk, [id]: v } })
            .catch(() => {})
        }
      }
    })
  })

  function relTime(iso: string): string {
    try {
      return formatDistanceToNow(new Date(iso), { addSuffix: true, locale: getCurrentDateFnsLocale() })
    } catch {
      return ''
    }
  }

  async function runTest(accountId: string) {
    showTestDialog = true
    testing = true
    testResult = null
    try {
      const r = await accountStore.testAccountConnection(accountId)
      if (r.success) {
        testResult = { success: true, message: $_('account.connectionSuccessful') }
        connOk = { ...connOk, [accountId]: await accountStore.getAccountConnOK(accountId) }
      } else {
        testResult = { success: false, message: r.error || $_('account.connectionFailed') }
      }
    } catch (err) {
      testResult = { success: false, message: (err as Error)?.message ?? String(err) }
    } finally {
      testing = false
    }
  }

  function openEdit(acc: account.Account) {
    editingAccount = acc
    showAccountDialog = true
  }

  export function openAdd() {
    editingAccount = null
    showAccountDialog = true
  }

  function openDelete(acc: account.Account) {
    deletingAccount = acc
    showDeleteDialog = true
  }

  function handleDialogClose() {
    showAccountDialog = false
    editingAccount = null
  }

  function handleDeleteDialogClose() {
    showDeleteDialog = false
    deletingAccount = null
  }

  async function moveUp(index: number) {
    if (index <= 0) return
    const ids = accountStore.accounts.map(a => a.account.id)
    ;[ids[index - 1], ids[index]] = [ids[index], ids[index - 1]]
    await accountStore.reorderAccounts(ids)
  }

  async function moveDown(index: number) {
    if (index >= accountStore.accounts.length - 1) return
    const ids = accountStore.accounts.map(a => a.account.id)
    ;[ids[index], ids[index + 1]] = [ids[index + 1], ids[index]]
    await accountStore.reorderAccounts(ids)
  }
</script>

<div class="space-y-4">
  {#if accountStore.loading}
    <div class="flex items-center justify-center py-4">
      <Icon icon="mdi:loading" class="w-5 h-5 animate-spin text-muted-foreground" />
    </div>
  {:else if regularAccounts.length === 0}
    <div class="text-sm text-muted-foreground py-4 text-center">
      <p>{$_('settingsAccounts.noAccountsConfigured')}</p>
    </div>
  {:else}
    <div class="space-y-2">
      {#each regularAccounts as accWithFolders, index (accWithFolders.account.id)}
        {@const acc = accWithFolders.account}
        <div class="p-3 border border-border rounded-lg flex items-center gap-3">
          <!-- Order number -->
          <div class="w-6 h-6 rounded-full bg-muted flex items-center justify-center text-xs font-medium text-muted-foreground">
            {index + 1}
          </div>

          <!-- Account info (name == email now, so show the address only) -->
          <div class="flex-1 min-w-0">
            <div class="text-sm truncate">{acc.email}</div>
          </div>

          <!-- Last-OK hint + test connection -->
          <span class="text-xs text-muted-foreground whitespace-nowrap hidden sm:inline">
            {connOk[acc.id]
              ? $_('account.lastConnected', { values: { time: relTime(connOk[acc.id]) } })
              : $_('account.neverConnected')}
          </span>
          <div
            data-settings-horizontal-group="account-actions"
            data-keyboard-action-context={acc.email}
            class="flex items-center gap-3"
          >
            <Button
              data-settings-horizontal-action="test"
              size="icon"
              variant="ghost"
              class="h-7 w-7"
              onclick={() => runTest(acc.id)}
              title={$_('account.testConnection')}
            >
              <Icon icon="mdi:lan-connect" class="w-4 h-4" />
            </Button>

            <!-- Up/Down buttons -->
            <div class="flex items-center gap-1">
              <Button
                data-settings-horizontal-action="move-up"
                size="icon"
                variant="ghost"
                class="h-7 w-7"
                onclick={() => moveUp(index)}
                disabled={index === 0}
                title={$_('settingsAccounts.moveUp')}
              >
                <Icon icon="mdi:chevron-up" class="w-4 h-4" />
              </Button>
              <Button
                data-settings-horizontal-action="move-down"
                size="icon"
                variant="ghost"
                class="h-7 w-7"
                onclick={() => moveDown(index)}
                disabled={index === accountStore.accounts.length - 1}
                title={$_('settingsAccounts.moveDown')}
              >
                <Icon icon="mdi:chevron-down" class="w-4 h-4" />
              </Button>
            </div>

            <!-- Edit button -->
            <Button
              data-settings-horizontal-action="edit"
              size="icon"
              variant="ghost"
              class="h-7 w-7"
              onclick={() => openEdit(acc)}
              title={$_('settingsAccounts.editAccount')}
            >
              <Icon icon="mdi:pencil" class="w-4 h-4" />
            </Button>

            <!-- Delete button -->
            <Button
              data-settings-horizontal-action="delete"
              size="icon"
              variant="ghost"
              class="h-7 w-7 text-destructive hover:text-destructive hover:bg-destructive/10"
              onclick={() => openDelete(acc)}
              title={$_('settingsAccounts.deleteAccount')}
            >
              <Icon icon="mdi:delete-outline" class="w-4 h-4" />
            </Button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Account Dialog -->
<AccountDialog
  bind:open={showAccountDialog}
  editAccount={editingAccount}
  onClose={handleDialogClose}
/>

<!-- Delete confirmation (same dialog used by the sidebar 3-dot menu) -->
<DeleteAccountDialog
  bind:open={showDeleteDialog}
  account={deletingAccount}
  onClose={handleDeleteDialogClose}
/>

<!-- Connection test result popup -->
<ConnectionTestDialog bind:open={showTestDialog} {testing} result={testResult} />
