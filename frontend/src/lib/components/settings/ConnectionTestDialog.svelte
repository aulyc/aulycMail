<script lang="ts">
  // ConnectionTestDialog — a small controlled popup that shows the result of a
  // mail connection test (spinner while testing, then success/failure). Shared
  // by the per-account "test connection" button (AccountsTab) and the add/edit
  // account form so both report results the same way.
  import { _ } from '$lib/i18n'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import Icon from '@iconify/svelte'

  interface Props {
    open: boolean
    testing: boolean
    result: { success: boolean; message: string } | null
    onClose?: () => void
  }

  let { open = $bindable(false), testing, result, onClose }: Props = $props()

  function close() {
    open = false
    onClose?.()
  }
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) close() }}>
  <Dialog.Content class="max-w-sm">
    <Dialog.Header>
      <Dialog.Title>{$_('account.testConnection')}</Dialog.Title>
    </Dialog.Header>

    <div class="py-3 flex items-start gap-3 min-h-[48px]">
      {#if testing}
        <Icon icon="mdi:loading" class="w-6 h-6 flex-shrink-0 animate-spin text-muted-foreground" />
        <span class="text-sm text-muted-foreground">{$_('account.testing')}</span>
      {:else if result}
        <Icon
          icon={result.success ? 'mdi:check-circle' : 'mdi:alert-circle'}
          class="w-6 h-6 flex-shrink-0 {result.success ? 'text-green-500' : 'text-destructive'}"
        />
        <span class="text-sm {result.success ? 'text-green-600 dark:text-green-400' : 'text-destructive'}">
          {result.message}
        </span>
      {/if}
    </div>

    <div class="flex justify-end pt-2">
      <Button onclick={close} disabled={testing}>{$_('common.close')}</Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
