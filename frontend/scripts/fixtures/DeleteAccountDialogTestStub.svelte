<script lang="ts">
  let {
    open = $bindable(false),
    account,
    onClose,
  }: {
    open?: boolean
    account?: { email?: string } | null
    onClose?: () => void
  } = $props()

  function close() {
    open = false
    onClose?.()
  }
</script>

{#if open}
  <section data-delete-account-dialog>
    <span>{account?.email ?? ''}</span>
    <button type="button" data-delete-account-close onclick={close}>close delete dialog</button>
  </section>
{/if}
