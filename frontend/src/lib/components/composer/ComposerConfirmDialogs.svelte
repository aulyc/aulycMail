<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog'
  import { ThreeOptionDialog } from '$lib/components/ui/confirm-dialog'
  import { _ } from '$lib/i18n'

  interface Props {
    showEmptySubjectDialog: boolean
    showMissingAttachmentDialog: boolean
    showCloseConfirm: boolean
    showInlineImageSizeDialog: boolean
    inlineImageSizeDescription: string
    closeLoading?: 'discard' | 'save' | null
    onConfirmEmptySubject: () => void
    onConfirmMissingAttachment: () => void
    onKeepImagesInline: () => void
    onAttachImagesInstead: () => void
    onCancelInlineImages: () => void
    onDiscardAndClose: () => void | Promise<void>
    onSaveAndClose: () => void | Promise<void>
    onKeepEditing: () => void
  }

  let {
    showEmptySubjectDialog = $bindable(false),
    showMissingAttachmentDialog = $bindable(false),
    showCloseConfirm = $bindable(false),
    showInlineImageSizeDialog = $bindable(false),
    inlineImageSizeDescription,
    closeLoading = null,
    onConfirmEmptySubject,
    onConfirmMissingAttachment,
    onKeepImagesInline,
    onAttachImagesInstead,
    onCancelInlineImages,
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

<ThreeOptionDialog
  bind:open={showInlineImageSizeDialog}
  title={$_('composer.inlineImageSizeTitle')}
  description={inlineImageSizeDescription}
  option1Label={$_('composer.keepImagesInline')}
  option2Label={$_('composer.attachImagesInstead')}
  option3Label={$_('common.cancel')}
  option1Variant="default"
  option2Variant="default"
  onOption1={onKeepImagesInline}
  onOption2={onAttachImagesInstead}
  onOption3={onCancelInlineImages}
/>

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
