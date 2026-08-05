<script>
  let {
    accountId,
    folderId,
    onConversationSelect,
    onConversationFocus,
    onEmptyFolder,
    onOpenDraft,
    onSearch,
  } = $props()
  let selected = $state('')
  let actionCount = $state(0)

  export async function selectThread(threadId) { selected = threadId }
  export function handleActionComplete() { actionCount += 1 }
  export function clearChecked() {}
  export function toggleFolderSync() {}
  export function toggleSearchFocus() {}
</script>

<div data-stub="message-list" data-account={accountId || ''} data-folder={folderId || ''} data-selected={selected} data-actions={actionCount}>
  <button data-app-action="focus-conversation" onclick={() => onConversationFocus?.('thread-1', 'inbox-1', 'account-1')}>Focus conversation</button>
  <button data-app-action="open-conversation" onclick={() => onConversationSelect?.('thread-1', 'inbox-1', 'account-1')}>Open conversation</button>
  <button data-app-action="empty-folder" onclick={onEmptyFolder}>Empty folder</button>
  <button data-app-action="open-draft" onclick={() => onOpenDraft?.('draft-1')}>Open draft</button>
  <button data-app-action="search" onclick={onSearch}>Search</button>
</div>
