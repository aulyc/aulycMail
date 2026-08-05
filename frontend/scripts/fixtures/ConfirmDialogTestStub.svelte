<script lang="ts">
  let {
    open = $bindable(false),
    title,
    description,
    confirmLabel = 'Confirm',
    cancelLabel = 'Cancel',
    onConfirm,
    onCancel,
  }: {
    open: boolean
    title: string
    description: string
    confirmLabel?: string
    cancelLabel?: string
    onConfirm: () => void | Promise<void>
    onCancel?: () => void
  } = $props()

  async function confirm() {
    await onConfirm()
    open = false
  }

  function cancel() {
    onCancel?.()
    open = false
  }
</script>

{#if open}
  <div role="alertdialog" data-confirm-dialog>
    <h2>{title}</h2>
    <p>{description}</p>
    <button type="button" data-confirm-action="cancel" onclick={cancel}>{cancelLabel}</button>
    <button type="button" data-confirm-action="confirm" onclick={confirm}>{confirmLabel}</button>
  </div>
{/if}
