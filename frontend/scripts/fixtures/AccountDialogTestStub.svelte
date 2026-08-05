<script lang="ts">
  let {
    open = $bindable(false),
    onClose,
  }: {
    open?: boolean
    editAccount?: unknown
    onClose?: () => void
  } = $props()

  function close() {
    open = false
    onClose?.()
  }
</script>

{#if open}
  <div data-account-dialog>
    <button type="button" data-account-dialog-close onclick={close}>Close account dialog</button>
  </div>
{/if}
