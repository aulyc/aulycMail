<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog'
  import AboutTab from './AboutTab.svelte'
  import { _ } from '$lib/i18n'

  interface Props {
    /** Whether the dialog is open */
    open?: boolean
    /** Callback when dialog should close */
    onClose?: () => void
  }

  let {
    open = $bindable(false),
    onClose,
  }: Props = $props()

  function handleOpenChange(isOpen: boolean) {
    open = isOpen
    if (!isOpen) onClose?.()
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="max-w-sm">
    <!-- Title kept for accessibility only; the popup just shows the About card. -->
    <Dialog.Title class="sr-only">{$_('settings.about')}</Dialog.Title>
    <AboutTab />
  </Dialog.Content>
</Dialog.Root>
