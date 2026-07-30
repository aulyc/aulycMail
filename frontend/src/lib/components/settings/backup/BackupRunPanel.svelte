<script lang="ts">
  import Icon from '@iconify/svelte'
  import { Button } from '$lib/components/ui/button'
  import { _ } from '$lib/i18n'
  import type { BackupRunStore } from './backupRun.svelte'

  interface Props {
    store: BackupRunStore
    canStart: boolean
    saveBeforeStart: boolean
    onStart: () => void | Promise<void>
  }

  let { store, canStart, saveBeforeStart, onStart }: Props = $props()
</script>

<Button
  size="sm"
  data-settings-contrast-selection
  onclick={onStart}
  disabled={store.loading || (!store.running && !canStart)}
>
  <Icon icon={store.running ? 'mdi:progress-clock' : 'mdi:backup-restore'} class="mr-2 h-4 w-4" />
  {store.running
    ? $_('settingsBackup.viewProgress')
    : saveBeforeStart
      ? $_('settingsBackup.saveAndStart')
      : $_('settingsBackup.startBackup')}
</Button>
