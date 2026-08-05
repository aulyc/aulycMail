<script>
  let { open = $bindable(false), onClose } = $props()
</script>

{#if open}
  <div role="dialog" data-stub="backup-viewer">
    <button data-app-action="close-backup" onclick={onClose}>Close backup</button>
  </div>
{/if}
