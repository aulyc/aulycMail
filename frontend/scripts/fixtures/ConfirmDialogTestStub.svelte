<script lang="ts">
  let {
    open = $bindable(false),
    title,
    description,
    confirmLabel = 'Confirm',
    cancelLabel = 'Cancel',
    loading = false,
    loadingPresentation = 'button',
    loadingLabel = '',
    loadingProgress = 0,
    onConfirm,
    onCancel,
  }: {
    open: boolean
    title: string
    description: string
    confirmLabel?: string
    cancelLabel?: string
    loading?: boolean
    loadingPresentation?: 'button' | 'footer-progress'
    loadingLabel?: string
    loadingProgress?: number
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
    {#if loading && loadingPresentation === 'footer-progress'}
      <div data-confirm-progress>
        <span data-confirm-progress-label>{loadingLabel}</span>
        <span data-confirm-progress-percent>{loadingProgress}%</span>
        <div role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={loadingProgress}>
          <div data-confirm-progress-fill style:width={`${loadingProgress}%`}></div>
        </div>
      </div>
    {/if}
    <button type="button" data-confirm-action="cancel" onclick={cancel} disabled={loading}>{cancelLabel}</button>
    <button
      type="button"
      data-confirm-action="confirm"
      data-confirm-fixed-width={loadingPresentation === 'footer-progress'}
      onclick={confirm}
      disabled={loading}
    >{confirmLabel}</button>
  </div>
{/if}
