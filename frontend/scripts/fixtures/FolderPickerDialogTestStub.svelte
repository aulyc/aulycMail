<script lang="ts">
  let {
    open = $bindable(false),
    title,
    accounts = [],
    initialAccountId,
    excludeFolderId,
    onSelect,
  } = $props()

  function select() {
    onSelect?.('destination-folder', 'Destination', 'destination-account')
  }
</script>

{#if open}
  <section
    data-folder-picker
    data-title={title}
    data-account-count={accounts.length}
    data-initial-account={initialAccountId}
    data-excluded-folder={excludeFolderId}
  >
    <button type="button" data-folder-picker-select onclick={select}>select folder</button>
    <button type="button" data-folder-picker-close onclick={() => (open = false)}>close picker</button>
  </section>
{/if}
