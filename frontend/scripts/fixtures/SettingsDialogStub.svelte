<script>
  let { open = $bindable(false), activePage = $bindable('general'), onClose } = $props()
</script>

{#if open}
  <div role="dialog" data-stub="settings-dialog" data-page={activePage}>
    <button data-app-action="close-settings" onclick={onClose}>Close settings</button>
  </div>
{/if}
