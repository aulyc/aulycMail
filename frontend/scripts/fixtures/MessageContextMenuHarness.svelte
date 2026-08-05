<script lang="ts">
  import MessageContextMenu from '../../src/lib/components/common/MessageContextMenu.svelte'

  let {
    messageIds,
    accountId = 'account-1',
    currentFolderId = 'inbox-1',
    folderType = 'inbox',
    isStarred = false,
    isRead = false,
    onActionComplete,
    onReply,
    onOpenChange,
  } = $props()
</script>

<MessageContextMenu
  {messageIds}
  {accountId}
  {currentFolderId}
  {folderType}
  {isStarred}
  {isRead}
  {onActionComplete}
  {onReply}
  {onOpenChange}
>
  <button type="button" data-context-trigger>message</button>
</MessageContextMenu>
