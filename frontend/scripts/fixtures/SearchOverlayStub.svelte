<script>
  let { open = $bindable(false), mode, onClose, onSelectMail, onSelectContact } = $props()
</script>

{#if open}
  <div role="dialog" data-stub="search-overlay" data-mode={mode}>
    <button data-app-action="close-search" onclick={onClose}>Close search</button>
    <button data-app-action="select-mail-search" onclick={() => onSelectMail?.({ threadId: 'thread-search', accountId: 'account-1', folderId: 'inbox-1' })}>Select mail</button>
    <button data-app-action="select-mail-search-fallback" onclick={() => onSelectMail?.({ threadId: 'thread-fallback' })}>Select mail with current folder</button>
    <button data-app-action="select-contact-search" onclick={() => onSelectContact?.({ id: 'contact-1' })}>Select contact</button>
  </div>
{/if}
