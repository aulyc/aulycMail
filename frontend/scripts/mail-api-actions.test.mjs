import assert from 'node:assert/strict'
import { beforeEach, test, vi } from 'vitest'

const wails = vi.hoisted(() => ({
  Archive: vi.fn(),
  CopyToFolder: vi.fn(),
  DeleteDraft: vi.fn(),
  DeletePermanently: vi.fn(),
  GetAccount: vi.fn(),
  GetAllAccountIdentities: vi.fn(),
  GetIdentities: vi.fn(),
  GetSearchCount: vi.fn(),
  GetSearchCountUnifiedInbox: vi.fn(),
  IMAPSearchFolder: vi.fn(),
  MarkAsNotSpam: vi.fn(),
  MarkAsRead: vi.fn(),
  MarkAsSpam: vi.fn(),
  MarkAsUnread: vi.fn(),
  MoveToFolder: vi.fn(),
  PickAttachmentFiles: vi.fn(),
  ReadFileAsAttachment: vi.fn(),
  SaveDraft: vi.fn(),
  SearchContacts: vi.fn(),
  SearchConversations: vi.fn(),
  SearchUnifiedInbox: vi.fn(),
  SendMessage: vi.fn(),
  Star: vi.fn(),
  Trash: vi.fn(),
  Undo: vi.fn(),
  Unstar: vi.fn(),
}))

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => wails)
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))

import { COMPOSER_API_KEY, createMainWindowApi } from '../src/lib/composerApi.ts'
import {
  loadMoreLocalMessageListSearch,
  searchLocalMessageList,
  searchServerMessageList,
} from '../src/lib/components/list/messageListSearch.ts'
import {
  archiveMessages,
  copyMessagesToFolder,
  deleteMessagesPermanently,
  moveMessagesToFolder,
  setReadStateMessages,
  toggleSpamMessages,
  toggleStarMessages,
  trashMessages,
  undoLastMailAction,
} from '../src/lib/mailActions.ts'

beforeEach(() => {
  vi.clearAllMocks()
  for (const fn of Object.values(wails)) fn.mockResolvedValue(undefined)
})

test('main-window composer API delegates every operation and normalizes missing results', async () => {
  const message = { subject: 'Hello' }
  const identities = [{ id: 'identity' }]
  const attachments = [{ filename: 'a.pdf' }]
  const account = { id: 'account' }
  const groups = [{ accountId: 'account' }]
  wails.SearchContacts.mockResolvedValue(null)
  wails.GetIdentities.mockResolvedValue(identities)
  wails.SaveDraft.mockResolvedValue(null)
  wails.PickAttachmentFiles.mockResolvedValue(attachments)
  wails.GetAccount.mockResolvedValue(account)
  wails.ReadFileAsAttachment.mockResolvedValue(null)
  wails.GetAllAccountIdentities.mockResolvedValue(groups)

  const api = createMainWindowApi()
  assert.equal(COMPOSER_API_KEY, 'composer-api')
  await api.sendMessage('account', message)
  assert.deepEqual(await api.searchContacts('ali', 5), [])
  assert.equal(await api.getIdentities('account'), identities)
  assert.deepEqual(await api.saveDraft('account', message, ''), { id: '', syncStatus: 'pending' })
  await api.deleteDraft('draft')
  assert.equal(await api.pickAttachmentFiles(), attachments)
  assert.equal(await api.getAccount('account'), account)
  assert.equal(await api.readFileAsAttachment('/tmp/a.pdf'), null)
  assert.equal(await api.getAllAccountIdentities(), groups)

  assert.deepEqual(wails.SendMessage.mock.calls[0], ['account', message])
  assert.deepEqual(wails.SearchContacts.mock.calls[0], ['ali', 5])
  assert.deepEqual(wails.SaveDraft.mock.calls[0], ['account', message, ''])
  assert.deepEqual(wails.DeleteDraft.mock.calls[0], ['draft'])
  assert.deepEqual(wails.ReadFileAsAttachment.mock.calls[0], ['/tmp/a.pdf'])
})

test('composer API preserves draft identity and sync state returned by the backend', async () => {
  wails.SaveDraft.mockResolvedValue({ draft: { id: 'draft-1', syncStatus: 'synced' } })
  assert.deepEqual(await createMainWindowApi().saveDraft('account', {}, 'draft-1'), {
    id: 'draft-1', syncStatus: 'synced',
  })
})

test('local search handles unified, account, and incomplete folder contexts', async () => {
  wails.SearchUnifiedInbox.mockResolvedValue(null)
  wails.GetSearchCountUnifiedInbox.mockResolvedValue(4)
  assert.deepEqual(await searchLocalMessageList({
    isUnifiedView: true, accountId: null, folderId: null, query: 'hello', offset: 0, limit: 20, filterMode: 'all',
  }), { results: [], count: 4 })

  assert.deepEqual(await searchLocalMessageList({
    isUnifiedView: false, accountId: null, folderId: 'inbox', query: 'hello', offset: 0, limit: 20, filterMode: 'all',
  }), { results: [], count: 0 })

  wails.SearchConversations.mockResolvedValue([{ threadId: 'thread' }])
  wails.GetSearchCount.mockResolvedValue(1)
  assert.deepEqual(await searchLocalMessageList({
    isUnifiedView: false, accountId: 'account', folderId: 'inbox', query: 'hello', offset: 20, limit: 20, filterMode: 'unread',
  }), { results: [{ threadId: 'thread' }], count: 1 })
  assert.deepEqual(wails.SearchConversations.mock.calls[0], ['account', 'inbox', 'hello', 20, 20, 'unread'])
})

test('local search pagination returns empty arrays for missing or null backend results', async () => {
  wails.SearchUnifiedInbox.mockResolvedValue(null)
  assert.deepEqual(await loadMoreLocalMessageListSearch({
    isUnifiedView: true, accountId: null, folderId: null, query: '', offset: 20, limit: 20, filterMode: 'all',
  }), [])
  assert.deepEqual(await loadMoreLocalMessageListSearch({
    isUnifiedView: false, accountId: 'account', folderId: null, query: '', offset: 20, limit: 20, filterMode: 'all',
  }), [])
  wails.SearchConversations.mockResolvedValue(null)
  assert.deepEqual(await loadMoreLocalMessageListSearch({
    isUnifiedView: false, accountId: 'account', folderId: 'inbox', query: '', offset: 20, limit: 20, filterMode: 'all',
  }), [])
})

test('server search adapts local and remote messages and derives a missing total', async () => {
  wails.IMAPSearchFolder.mockResolvedValue({
    results: [
      {
        uid: 1, messageId: 'message-1', subject: 'Local', snippet: 'preview', isLocal: true,
        isRead: false, hasAttachments: true, isStarred: true, date: '2026-01-01',
        fromName: 'Alice', fromEmail: 'alice@example.com', accountId: 'account', folderId: 'inbox',
      },
      {
        uid: 2, messageId: '', subject: 'Remote', snippet: 'hidden', isLocal: false,
        isRead: true, hasAttachments: false, isStarred: false, date: '2026-01-02',
        fromName: '', fromEmail: 'bob@example.com', accountId: 'account', folderId: 'inbox',
      },
    ],
  })
  const result = await searchServerMessageList({ accountId: 'account', folderId: 'inbox', query: 'q', limit: 10 })
  assert.equal(result.totalCount, 2)
  assert.deepEqual(result.results[0], {
    threadId: 'message-1', subject: 'Local', snippet: 'preview', messageCount: 1, unreadCount: 1,
    hasAttachments: true, isStarred: true, latestDate: '2026-01-01',
    participants: [{ name: 'Alice', email: 'alice@example.com' }], messageIds: ['message-1'],
    accountId: 'account', folderId: 'inbox', _isLocal: true, _uid: 1,
  })
  assert.equal(result.results[1].threadId, 'server-uid-2')
  assert.equal(result.results[1].snippet, '')
  assert.deepEqual(result.results[1].messageIds, [])

  wails.IMAPSearchFolder.mockResolvedValue(null)
  assert.deepEqual(await searchServerMessageList({ accountId: 'account', folderId: 'inbox', query: '', limit: 10 }), {
    results: [], totalCount: 0,
  })
})

test('undo reports success and failure through user-visible state and callbacks', async () => {
  const onSuccess = vi.fn()
  const onError = vi.fn()
  wails.Undo.mockResolvedValue('Archived 2 messages')
  await undoLastMailAction({ onSuccess, onError })
  assert.deepEqual(toast.success.mock.calls[0], [
    'toast.undone:{"description":"Archived 2 messages"}',
  ])
  assert.equal(onSuccess.mock.calls.length, 1)

  const error = new Error('undo failed')
  wails.Undo.mockRejectedValue(error)
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  await undoLastMailAction({ onSuccess, onError })
  log.mockRestore()
  assert.deepEqual(toast.error.mock.calls.at(-1), ['toast.undoFailed'])
  assert.deepEqual(onError.mock.calls.at(-1), [error])
})

test('archive, trash, and permanent delete apply default selection and undo semantics', async () => {
  const onUndo = vi.fn()
  const onSuccess = vi.fn()
  await archiveMessages(['m1'], { onUndo, onSuccess, successKey: 'toast.customArchive' })
  assert.deepEqual(wails.Archive.mock.calls[0], [['m1']])
  assert.equal(toast.success.mock.calls[0][0], 'toast.customArchive')
  assert.equal(toast.success.mock.calls[0][1].length, 1)
  toast.success.mock.calls[0][1][0].onClick()
  await Promise.resolve()
  assert.equal(onUndo.mock.calls.length, 1)
  assert.deepEqual(onSuccess.mock.calls[0], [true])

  wails.Trash.mockResolvedValueOnce(true).mockResolvedValueOnce(false)
  await trashMessages(['m1'], { onUndo, onSuccess })
  await trashMessages(['m2'], { onUndo, onSuccess, autoSelectNext: false })
  assert.equal(toast.success.mock.calls[1][0], 'toast.movedToTrash')
  assert.equal(toast.success.mock.calls[1][1].length, 1)
  assert.equal(toast.success.mock.calls[2][0], 'toast.deletedFromFolder')
  assert.deepEqual(toast.success.mock.calls[2][1], [])
  assert.deepEqual(onSuccess.mock.calls.at(-1), [false])

  await deleteMessagesPermanently(['m3'], { onSuccess })
  assert.equal(toast.success.mock.calls.at(-1)[0], 'toast.permanentlyDeleted')
  assert.deepEqual(onSuccess.mock.calls.at(-1), [true])
})

test('spam actions cover restore, forced wording, and backend move results', async () => {
  const onUndo = vi.fn()
  const onSuccess = vi.fn()
  await toggleSpamMessages(['m1'], true, { onUndo, onSuccess })
  assert.deepEqual(wails.MarkAsNotSpam.mock.calls[0], [['m1']])
  assert.equal(toast.success.mock.calls[0][0], 'toast.markedAsNotSpam')

  wails.MarkAsSpam.mockResolvedValueOnce(false).mockResolvedValueOnce(false).mockResolvedValueOnce(true)
  await toggleSpamMessages(['m2'], false, { onUndo, onSuccess, spamSuccessMode: 'alwaysMarked' })
  await toggleSpamMessages(['m3'], false, { onUndo, onSuccess })
  await toggleSpamMessages(['m4'], false, { onUndo, onSuccess })
  assert.equal(toast.success.mock.calls[1][0], 'toast.markedAsSpam')
  assert.equal(toast.success.mock.calls[2][0], 'toast.deletedFromFolder')
  assert.deepEqual(toast.success.mock.calls[2][1], [])
  assert.equal(toast.success.mock.calls[3][0], 'toast.markedAsSpam')
})

test('star, read, move, and copy actions report their concrete results', async () => {
  const onSuccess = vi.fn()
  const onUndo = vi.fn()
  await toggleStarMessages(['m1'], true, { onSuccess })
  await toggleStarMessages(['m1'], false, { onSuccess, unstarSuccessKey: 'toast.customUnstar' })
  assert.equal(toast.success.mock.calls[0][0], 'toast.starred')
  assert.equal(toast.success.mock.calls[1][0], 'toast.customUnstar')

  await setReadStateMessages(['m1'], true, { onSuccess })
  await setReadStateMessages(['m1'], false, { onSuccess })
  assert.equal(toast.success.mock.calls[2][0], 'toast.markedAsRead')
  assert.equal(toast.success.mock.calls[3][0], 'toast.markedAsUnread')

  await moveMessagesToFolder(['m1'], 'archive', 'Archive', { onSuccess, onUndo })
  await copyMessagesToFolder(['m1'], 'copy', 'Copy', { onSuccess })
  assert.deepEqual(wails.MoveToFolder.mock.calls[0], [['m1'], 'archive'])
  assert.deepEqual(wails.CopyToFolder.mock.calls[0], [['m1'], 'copy'])
  assert.equal(toast.success.mock.calls[4][0], 'toast.movedTo:{"folder":"Archive"}')
  assert.equal(toast.success.mock.calls[4][1].length, 1)
  assert.equal(toast.success.mock.calls[5][0], 'toast.copyingTo:{"folder":"Copy"}')
})

test.each([
  ['archive', 'failedToArchive', () => archiveMessages(['m'], {})],
  ['Trash', 'failedToDelete', () => trashMessages(['m'], {})],
  ['DeletePermanently', 'failedToDelete', () => deleteMessagesPermanently(['m'], {})],
  ['MarkAsSpam', 'failedToMarkAsSpam', () => toggleSpamMessages(['m'], false, {})],
  ['MarkAsNotSpam', 'failedToMarkAsNotSpam', () => toggleSpamMessages(['m'], true, {})],
  ['Star', 'failedToUpdateStar', () => toggleStarMessages(['m'], true, {})],
  ['MarkAsRead', 'customReadError', () => setReadStateMessages(['m'], true, { errorKey: 'toast.customReadError' })],
  ['MoveToFolder', 'failedToMove', () => moveMessagesToFolder(['m'], 'folder', 'Folder', {})],
  ['CopyToFolder', 'failedToCopy', () => copyMessagesToFolder(['m'], 'folder', 'Folder', {})],
])('%s failure invokes the error path without escaping', async (method, expectedKey, invoke) => {
  const backendMethod = method === 'archive' ? 'Archive' : method
  const error = new Error(`${method} failed`)
  wails[backendMethod].mockRejectedValue(error)
  const onError = vi.fn()
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})

  if (method === 'archive') await archiveMessages(['m'], { onError })
  else if (method === 'Trash') await trashMessages(['m'], { onError })
  else if (method === 'DeletePermanently') await deleteMessagesPermanently(['m'], { onError })
  else if (method === 'MarkAsSpam') await toggleSpamMessages(['m'], false, { onError })
  else if (method === 'MarkAsNotSpam') await toggleSpamMessages(['m'], true, { onError })
  else if (method === 'Star') await toggleStarMessages(['m'], true, { onError })
  else if (method === 'MarkAsRead') await setReadStateMessages(['m'], true, { onError, errorKey: 'toast.customReadError' })
  else if (method === 'MoveToFolder') await moveMessagesToFolder(['m'], 'folder', 'Folder', { onError })
  else await copyMessagesToFolder(['m'], 'folder', 'Folder', { onError })

  log.mockRestore()
  assert.deepEqual(onError.mock.calls[0], [error])
  assert.equal(toast.error.mock.calls[0][0], `toast.${expectedKey}`)
  assert.equal(typeof invoke, 'function')
})
