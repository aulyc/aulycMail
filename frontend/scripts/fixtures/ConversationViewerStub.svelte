<script>
  let {
    threadId,
    accountId,
    folderId,
    onReply,
    onComposeToAddress,
    onEditDraft,
    onActionComplete,
    onToggleThreadFocus,
  } = $props()

  export function getLastMessageId() { return threadId ? 'message-last' : null }
  export function refreshFlags() {}
  export function hasFocusedMessage() { return false }
  export function isImagesLoaded() { return false }
  export function reply() { onReply?.('reply', 'message-last') }
  export function replyAll() { onReply?.('reply-all', 'message-last') }
</script>

<div data-stub="conversation-viewer" data-thread={threadId || ''} data-account={accountId || ''} data-folder={folderId || ''}>
  <button data-app-action="reply" onclick={() => onReply?.('reply', 'message-last', true)}>Reply</button>
  <button data-app-action="reply-default-images" onclick={() => onReply?.('reply', 'message-last')}>Reply with default image state</button>
  <button data-app-action="compose-address" onclick={() => onComposeToAddress?.('person@example.test')}>Compose address</button>
  <button data-app-action="edit-draft" onclick={() => onEditDraft?.('draft-1')}>Edit draft</button>
  <button data-app-action="complete-action" onclick={() => onActionComplete?.(true)}>Complete action</button>
  <button data-app-action="toggle-focus" onclick={onToggleThreadFocus}>Toggle focus</button>
</div>
