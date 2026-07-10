export type ConversationSelectionLike = {
  threadId?: string
  messageIds?: string[]
  messages?: Array<{ id: string }>
  isStarred?: boolean
  unreadCount?: number
}

export type RowContextMenuState = {
  messageIds: string[]
  accountId: string
  folderId: string
  folderType: string
  isStarred: boolean
  isRead: boolean
  allowReply: boolean
}

function getConversationMessageIds(conversation: ConversationSelectionLike): string[] {
  return conversation.messageIds || conversation.messages?.map((m) => m.id) || []
}

function getSelectedConversations<T extends ConversationSelectionLike>(
  conversations: T[],
  checkedThreadIds: Set<string>,
): T[] {
  return conversations.filter((c) => !!c.threadId && checkedThreadIds.has(c.threadId))
}

export function getSelectedMessageIds(
  conversations: ConversationSelectionLike[],
  checkedThreadIds: Set<string>,
): string[] {
  return [
    ...new Set(
      getSelectedConversations(conversations, checkedThreadIds).flatMap(getConversationMessageIds),
    ),
  ]
}

export function hasSelectedUnstarred(
  conversations: ConversationSelectionLike[],
  checkedThreadIds: Set<string>,
): boolean {
  return getSelectedConversations(conversations, checkedThreadIds).some((c) => !c.isStarred)
}

export function hasSelectedUnread(
  conversations: ConversationSelectionLike[],
  checkedThreadIds: Set<string>,
): boolean {
  return getSelectedConversations(conversations, checkedThreadIds).some((c) => (c.unreadCount || 0) > 0)
}

export function toggleSetEntry<T>(set: Set<T>, key: T): void {
  if (set.has(key)) {
    set.delete(key)
    return
  }
  set.add(key)
}

export function createSingleRowContextMenu(input: {
  conversation: ConversationSelectionLike
  accountId: string
  folderId: string
  folderType: string
}): RowContextMenuState {
  return {
    messageIds: getConversationMessageIds(input.conversation),
    accountId: input.accountId,
    folderId: input.folderId,
    folderType: input.folderType,
    isStarred: input.conversation.isStarred ?? false,
    isRead: (input.conversation.unreadCount || 0) === 0,
    allowReply: true,
  }
}

export function createMultiRowContextMenu(input: {
  messageIds: string[]
  accountId: string
  folderId: string
  folderType: string
  hasUnstarred: boolean
  hasUnread: boolean
}): RowContextMenuState {
  return {
    messageIds: input.messageIds,
    accountId: input.accountId,
    folderId: input.folderId,
    folderType: input.folderType,
    isStarred: !input.hasUnstarred,
    isRead: !input.hasUnread,
    allowReply: false,
  }
}
