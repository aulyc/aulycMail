// @ts-ignore - wailsjs bindings
import {
  GetSearchCount,
  GetSearchCountUnifiedInbox,
  IMAPSearchFolder,
  SearchConversations,
  SearchUnifiedInbox,
} from '../../../../wailsjs/go/app/App'

export interface LocalSearchParams {
  isUnifiedView: boolean
  accountId: string | null
  folderId: string | null
  query: string
  offset: number
  limit: number
  filterMode: string
}

export interface LocalSearchResult {
  results: any[]
  count: number
}

export interface ServerSearchParams {
  accountId: string
  folderId: string
  query: string
  limit: number
}

export interface ServerSearchResult {
  results: any[]
  totalCount: number
}

export async function searchLocalMessageList(params: LocalSearchParams): Promise<LocalSearchResult> {
  if (params.isUnifiedView) {
    const [results, count] = await Promise.all([
      SearchUnifiedInbox(params.query, params.offset, params.limit, params.filterMode),
      GetSearchCountUnifiedInbox(params.query, params.filterMode),
    ])
    return { results: results || [], count }
  }

  if (!params.accountId || !params.folderId) {
    return { results: [], count: 0 }
  }

  const [results, count] = await Promise.all([
    SearchConversations(params.accountId, params.folderId, params.query, params.offset, params.limit, params.filterMode),
    GetSearchCount(params.accountId, params.folderId, params.query, params.filterMode),
  ])
  return { results: results || [], count }
}

export async function loadMoreLocalMessageListSearch(params: LocalSearchParams): Promise<any[]> {
  if (params.isUnifiedView) {
    return await SearchUnifiedInbox(params.query, params.offset, params.limit, params.filterMode) || []
  }

  if (!params.accountId || !params.folderId) {
    return []
  }

  return await SearchConversations(params.accountId, params.folderId, params.query, params.offset, params.limit, params.filterMode) || []
}

export async function searchServerMessageList(params: ServerSearchParams): Promise<ServerSearchResult> {
  const response = await IMAPSearchFolder(params.accountId, params.folderId, params.query, params.limit)
  const results = (response?.results || []).map(adaptServerResult)
  return {
    results,
    totalCount: response?.totalCount ?? results.length,
  }
}

function adaptServerResult(r: any): any {
  return {
    threadId: r.messageId || `server-uid-${r.uid}`,
    subject: r.subject,
    snippet: r.isLocal ? r.snippet : '',
    messageCount: 1,
    unreadCount: r.isRead ? 0 : 1,
    hasAttachments: r.hasAttachments,
    isStarred: r.isStarred,
    latestDate: r.date,
    participants: [{ name: r.fromName, email: r.fromEmail }],
    messageIds: r.messageId ? [r.messageId] : [],
    accountId: r.accountId,
    folderId: r.folderId,
    _isLocal: r.isLocal,
    _uid: r.uid,
  }
}
