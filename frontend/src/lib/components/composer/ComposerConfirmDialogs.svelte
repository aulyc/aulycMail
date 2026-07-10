<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { ThreeOptionDialog } from '$lib/components/ui/confirm-dialog'
  import { _ } from '$lib/i18n'

  interface Props {
    showEmptySubjectDialog: boolean
    showMissingAttachmentDialog: boolean
    showFlatpakDndDialog: boolean
    showCloseConfirm: boolean
    closeLoading?: 'discard' | 'save' | null
    onConfirmEmptySubject: () => void
    onConfirmMissingAttachment: () => void
    onDiscardAndClose: () => void | Promise<void>
    onSaveAndClose: () => void | Promise<void>
    onKeepEditing: () => void
  }

  let {
    showEmptySubjectDialog = $bindable(false),
    showMissingAttachmentDialog = $bindable(false),
    showFlatpakDndDialog = $bindable(false),
    showCloseConfirm = $bindable(false),
    closeLoading = null,
    onConfirmEmptySubject,
    onConfirmMissingAttachment,
    onDiscardAndClose,
    onSaveAndClose,
    onKeepEditing,
  }: Props = $props()
</script>

<AlertDialog.Root bind:open={showEmptySubjectDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.emptySubjectTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.emptySubjectDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={onConfirmEmptySubject}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showMissingAttachmentDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.missingAttachmentTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        {$_('composer.missingAttachmentDescription')}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{$_('common.cancel')}</AlertDialog.Cancel>
      <AlertDialog.Action onclick={onConfirmMissingAttachment}>{$_('composer.sendAnywayGeneric')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showFlatpakDndDialog}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{$_('composer.flatpakDndTitle')}</AlertDialog.Title>
      <AlertDialog.Description>
        <p class="mb-3">{$_('composer.flatpakDndDescription')}</p>
        <p class="mb-2">{$_('composer.flatpakDndGrantExample')}</p>
        <code class="block bg-muted px-3 py-2 rounded text-sm font-mono mb-3 select-all overflow-x-auto">flatpak override --user --filesystem=home com.aulyc.aulycmail</code>
        <p class="mb-3 text-sm text-destructive">{$_('composer.flatpakDndSecurityWarning')}</p>
        <p class="text-sm text-muted-foreground">{$_('composer.flatpakDndAlternative')}</p>
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Action onclick={() => showFlatpakDndDialog = false}>{$_('common.ok')}</AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<ThreeOptionDialog
  bind:open={showCloseConfirm}
  title={$_('composer.closeTitle')}
  description={$_('composer.closeDescription')}
  option1Label={$_('composer.discardDraft')}
  option2Label={$_('composer.saveAndClose')}
  option3Label={$_('composer.keepEditing')}
  option1Variant="destructive"
  option2Variant="default"
  loading={closeLoading === 'discard' ? 'option1' : closeLoading === 'save' ? 'option2' : null}
  onOption1={onDiscardAndClose}
  onOption2={onSaveAndClose}
  onOption3={onKeepEditing}
/>
