<script lang="ts">
  import Icon from '@iconify/svelte'
  import { accountStore } from '$lib/stores/accounts.svelte'
  import { _ } from '$lib/i18n'
  import type { BackupScope } from '../settingsDraft.svelte'
  interface Props { scope: BackupScope; selectedAccountIds: string[]; disabled?: boolean }
  let { scope = $bindable(), selectedAccountIds = $bindable(), disabled = false }: Props = $props()
  let open = $state(false)
  let element = $state<HTMLDivElement | null>(null)
  const accounts = $derived(accountStore.accounts.filter(item => !item.account.sharedMailboxParentId))
  const accountIds = $derived(accounts.map(item => item.account.id))
  const selected = $derived(scope === 'all' ? accountIds : selectedAccountIds.filter(id => accountIds.includes(id)))
  const selectedSet = $derived(new Set(selected))
  const label = $derived(scope === 'all' ? $_('settingsBackup.scopeAll') : selected.length === 1 ? accounts.find(item => item.account.id === selected[0])?.account.email ?? $_('settingsBackup.scopeSelected') : $_('settingsBackup.selectedMailboxes', { values: { count: selected.length } }))

  $effect(() => {
    if (!open) return
    const close = (event: PointerEvent) => { if (!element?.contains(event.target as Node)) open = false }
    window.addEventListener('pointerdown', close, true)
    return () => window.removeEventListener('pointerdown', close, true)
  })

  function selectAll() { scope = 'all'; selectedAccountIds = accountIds }
  function toggle(id: string) {
    const next = new Set(selected)
    if (next.has(id)) next.delete(id); else next.add(id)
    selectedAccountIds = accountIds.filter(accountId => next.has(accountId))
    scope = selectedAccountIds.length === accountIds.length ? 'all' : 'selected'
  }
</script>

<div class="relative w-96 max-w-full" bind:this={element}>
  <button type="button" class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 text-sm hover:bg-muted/40 disabled:opacity-50" disabled={disabled || accounts.length === 0} onclick={() => open = !open}>
    <span class="truncate">{accounts.length === 0 ? $_('settingsBackup.noAccounts') : label}</span><Icon icon="mdi:chevron-down" class="h-4 w-4 text-muted-foreground" />
  </button>
  {#if open}
    <div class="absolute right-0 z-50 mt-1 max-h-64 w-full overflow-y-auto rounded-md border border-border bg-popover p-1 shadow-lg">
      <button type="button" class="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left text-sm hover:bg-accent" onclick={selectAll}><Icon icon={scope === 'all' ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'} class="h-4 w-4 text-primary" />{$_('settingsBackup.scopeAll')}</button>
      {#each accounts as item (item.account.id)}
        <button type="button" class="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left text-sm hover:bg-accent" onclick={() => toggle(item.account.id)}><Icon icon={selectedSet.has(item.account.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'} class="h-4 w-4 text-primary" /><span class="truncate">{item.account.email}</span></button>
      {/each}
    </div>
  {/if}
</div>
