<script>
  let { onFolderSelect, onCompose } = $props()
  let revealed = $state('')

  export function revealFolder(accountId, folderId) {
    revealed = `${accountId}:${folderId}`
  }

  export function toggleSync() {}
</script>

<div data-stub="sidebar" data-revealed={revealed}>
  <button data-sidebar-item data-app-action="select-folder" onclick={() => onFolderSelect?.('account-1', 'inbox-1', 'INBOX', 'Inbox', 'inbox')}>Inbox</button>
  <button data-app-action="compose" onclick={onCompose}>Compose</button>
</div>
