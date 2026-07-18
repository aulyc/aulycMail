<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { addToast } from '$lib/stores/toast'
  import { _ } from '$lib/i18n'
  import type { SettingsDraft } from '../settingsDraft.svelte'
  import SettingsPageHeader from '../shared/SettingsPageHeader.svelte'
  import BackupConfigSection from './BackupConfigSection.svelte'
  import BackupProgressDialog from './BackupProgressDialog.svelte'
  import RecentBackupLog from './RecentBackupLog.svelte'
  import BackupRunPanel from './BackupRunPanel.svelte'
  import { BackupRunStore } from './backupRun.svelte'
  interface Props { draft: SettingsDraft; onOpenActivityLog?: () => void }
  let { draft, onOpenActivityLog }: Props = $props()
  const store = new BackupRunStore()
  const accountIds = $derived(accountStore.accounts.filter(item => !item.account.sharedMailboxParentId).map(item => item.account.id))
  const selectedIds = $derived(draft.backupScope === 'all' ? accountIds : draft.backupSelectedAccountIds.filter(id => accountIds.includes(id)))
  const canStart = $derived(Boolean(draft.backupDirectory.trim()) && selectedIds.length > 0)
  let progressDialogOpen = $state(false)
  let preparing = $state(false)
  let startFailed = $state(false)

  $effect(() => {
    if (accountStore.loading) return
    if (draft.backupScope === 'all') return
    const normalized = selectedIds
    if (normalized.length !== draft.backupSelectedAccountIds.length || normalized.some((id, index) => id !== draft.backupSelectedAccountIds[index])) {
      draft.backupSelectedAccountIds = normalized
    }
  })

  async function start() {
    progressDialogOpen = true
    startFailed = false
    if (store.running) return
    preparing = true
    try {
      if (draft.backupDirty) await draft.saveBackup()
      await store.start(draft.backupDirectory.trim(), draft.backupScope, selectedIds)
    } catch (error) {
      console.error('Failed to start backup:', error)
      if (!store.running) {
        startFailed = true
        addToast({ type: 'error', message: $_('settingsBackup.backupFailed') })
      }
    } finally {
      preparing = false
    }
  }

  function runInBackground() {
    progressDialogOpen = false
    addToast({ type: 'success', message: $_('settingsBackup.backupContinuesInBackground') })
  }
  onMount(() => {
    void store.startListening().catch((error) => {
      console.error('Failed to load backup run state:', error)
      addToast({ type: 'error', message: $_('settingsBackup.loadRunStateFailed') })
    })
  })
  onDestroy(() => store.stopListening())
</script>

<div class="space-y-6">
  <SettingsPageHeader description={$_('settingsDescriptions.backup')}>
    {#snippet action()}
      <BackupRunPanel {store} {canStart} saveBeforeStart={draft.backupDirty} onStart={start} />
    {/snippet}
  </SettingsPageHeader>
  <div>
    <BackupConfigSection {draft} running={store.running} />
    <RecentBackupLog directory={draft.backupDirectory} onOpenLogs={onOpenActivityLog} />
  </div>
  <BackupProgressDialog
    bind:open={progressDialogOpen}
    {store}
    {preparing}
    {startFailed}
    onRunInBackground={runInBackground}
  />
</div>
