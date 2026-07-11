<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu'
  import ConfirmDialog from '$lib/components/ui/confirm-dialog/ConfirmDialog.svelte'
  import { _ } from '$lib/i18n'
  import type { ActivityLogsStore } from './activityLogs.svelte'
  interface Props { store: ActivityLogsStore }
  let { store }: Props = $props()
  let confirm = $state<'current' | 'all' | null>(null)
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger class="inline-flex h-9 items-center justify-center rounded-md border border-input bg-background px-3 text-sm font-medium transition-colors hover:bg-accent disabled:pointer-events-none disabled:opacity-50" disabled={store.clearing || store.total === 0}>
    <Icon icon="mdi:delete-sweep-outline" class="mr-1.5 h-4 w-4" />{$_('activityLog.clear')}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end">
    <DropdownMenu.Item onSelect={() => confirm = 'current'}>{$_('activityLog.clearCurrent')}</DropdownMenu.Item>
    <DropdownMenu.Item class="text-destructive" onSelect={() => confirm = 'all'}>{$_('activityLog.clearAll')}</DropdownMenu.Item>
  </DropdownMenu.Content>
</DropdownMenu.Root>

<ConfirmDialog open={confirm !== null} title={confirm === 'all' ? $_('activityLog.clearAll') : $_('activityLog.clearCurrent')} description={$_('activityLog.clearConfirm')} confirmLabel={$_('activityLog.confirmClear')} cancelLabel={$_('common.cancel')} variant="destructive" loading={store.clearing} onConfirm={async () => { if (confirm === 'all') await store.clearAll(); else await store.clearCurrent(); confirm = null }} onCancel={() => confirm = null} />
