<script lang="ts">
  import Icon from '@iconify/svelte'
  import * as Dialog from '$lib/components/ui/dialog'
  import { Button } from '$lib/components/ui/button'
  import { dialogGuardOpen, dialogGuardClose } from '$lib/stores/dialogGuard'
  import { _ } from '$lib/i18n'
  import BackupTab from '$lib/components/settings/BackupTab.svelte'

  interface Props {
    open?: boolean
    onClose?: () => void
  }

  let { open = $bindable(false), onClose }: Props = $props()

  $effect(() => {
    if (open) {
      dialogGuardOpen()
      return () => dialogGuardClose()
    }
  })

  function closeDialog() {
    open = false
    onClose?.()
  }

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) onClose?.()
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  {#if open}
    <Dialog.Content
      class="flex max-h-[85vh] w-[min(92vw,760px)] max-w-2xl flex-col overflow-hidden"
      preventCloseAutoFocus
      onInteractOutside={(e) => e.preventDefault()}
    >
      <Dialog.Header>
        <Dialog.Title class="flex items-center gap-2">
          <Icon icon="lucide:archive" class="h-5 w-5" />
          {$_('settingsBackup.title')}
        </Dialog.Title>
      </Dialog.Header>

      <div class="h-[min(64vh,430px)] min-h-0 overflow-y-auto pl-1 pr-3">
        <BackupTab />
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-border pt-4">
        <Button variant="ghost" onclick={closeDialog}>
          {$_('common.close')}
        </Button>
      </div>
    </Dialog.Content>
  {/if}
</Dialog.Root>
