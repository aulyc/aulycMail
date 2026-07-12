<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { BackupRunStore } from './backupRun.svelte'
  import SettingsSection from '../shared/SettingsSection.svelte'
  interface Props { store: BackupRunStore; canStart: boolean; saveBeforeStart: boolean; onStart: () => void | Promise<void> }
  let { store, canStart, saveBeforeStart, onStart }: Props = $props()
  const percent = $derived(store.progress?.total ? Math.min(100, Math.round(store.progress.current / store.progress.total * 100)) : 0)
  const target = $derived([store.progress?.accountEmail, store.progress?.folderPath].filter(Boolean).join(' / '))
  const hasIssues = $derived((store.progress?.missing ?? 0) > 0 || (store.progress?.failed ?? 0) > 0)
</script>

<SettingsSection title={$_('settingsBackup.currentTask')} framed={false}>
  <div class="space-y-3 py-3">
    {#if store.running || store.progress}
      <div class="flex items-center gap-2 text-sm"><Icon icon={store.running ? 'mdi:loading' : hasIssues ? 'mdi:alert-circle-outline' : 'mdi:check-circle-outline'} class="h-4 w-4 {store.running ? 'animate-spin text-primary' : hasIssues ? 'text-amber-500' : 'text-emerald-500'}" /><span class="font-medium tabular-nums">{store.progress?.current ?? 0}/{store.progress?.total ?? 0}</span>{#if target}<span class="truncate text-muted-foreground">· {target}</span>{/if}</div>
      <div class="h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full bg-primary transition-all" style={`width:${percent}%`}></div></div>
      <div class="flex flex-wrap gap-x-4 text-xs text-muted-foreground"><span>{$_('settingsBackup.progressExported')} {store.progress?.exported ?? 0}</span><span>{$_('settingsBackup.progressSkipped')} {store.progress?.skipped ?? 0}</span><span>{$_('settingsBackup.progressMissing')} {store.progress?.missing ?? 0}</span><span>{$_('settingsBackup.progressFailed')} {store.progress?.failed ?? 0}</span></div>
    {:else}<p class="text-sm text-muted-foreground">{$_('settingsBackup.noCurrentTask')}</p>{/if}
    <div class="flex justify-end"><Button onclick={onStart} disabled={!canStart || store.running || store.loading}>{#if store.running}<Icon icon="mdi:loading" class="mr-2 h-4 w-4 animate-spin" />{$_('settingsBackup.backupRunning')}{:else}<Icon icon="mdi:archive-arrow-down-outline" class="mr-2 h-4 w-4" />{saveBeforeStart ? $_('settingsBackup.saveAndStart') : $_('settingsBackup.startBackup')}{/if}</Button></div>
  </div>
</SettingsSection>
