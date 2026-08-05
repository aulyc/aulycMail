// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetConversations: vi.fn(),
  GetConversationCount: vi.fn(),
  SyncFolder: vi.fn(),
  ForceSyncFolder: vi.fn(),
  CancelFolderSync: vi.fn(),
  SetMessageListSortOrder: vi.fn(),
  GetUnifiedInboxConversations: vi.fn(),
  GetUnifiedInboxCount: vi.fn(),
  GetFTSIndexStatus: vi.fn(),
  IsFTSIndexing: vi.fn(),
  SearchConversations: vi.fn(),
  GetSearchCount: vi.fn(),
  SearchUnifiedInbox: vi.fn(),
  GetSearchCountUnifiedInbox: vi.fn(),
  IMAPSearchFolder: vi.fn(),
  Trash: vi.fn(),
  DeletePermanently: vi.fn(),
  EmptyTrash: vi.fn(),
  Undo: vi.fn(),
  FetchServerMessage: vi.fn(),
  Star: vi.fn(),
  Unstar: vi.fn(),
}))
const runtime = vi.hoisted(() => ({
  handlers: new Map(),
  EventsOn: vi.fn((name, handler) => {
    runtime.handlers.set(name, handler)
    return () => runtime.handlers.delete(name)
  }),
}))
const settings = vi.hoisted(() => ({ sortOrder: 'newest' }))
const layout = vi.hoisted(() => ({ mode: 'full', hideViewer: vi.fn() }))
const toast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
  subscribe(run) {
    run([])
    return () => {}
  },
}))
const accountStore = vi.hoisted(() => ({
  accounts: [{
    account: {
      id: 'account-1',
      syncAllFolders: false,
      syncFoldersEnabled: false,
    },
    folders: [{
      folder: { id: 'inbox-1', type: 'inbox', subscribed: true },
      children: [],
    }],
  }],
  syncProgress: {},
  isOnline: true,
  getAccount(id) {
    return this.accounts.find((entry) => entry.account.id === id)
  },
  getFolderUnreadCount: vi.fn(() => 7),
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime.js', () => runtime)
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$lib/stores/settings.svelte', () => ({
  getMessageListDensity: () => 'compact',
  getMessageListSortOrder: () => settings.sortOrder,
  setMessageListSortOrder: (value) => { settings.sortOrder = value },
  getAccentBarUnread: () => true,
  getCurrentDateFnsLocale: () => undefined,
  getEnhancedKeyboardNavigation: () => true,
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/layout.svelte', () => ({
  getLayoutMode: () => layout.mode,
  hideViewer: layout.hideViewer,
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({ focusPane: vi.fn(() => true) }))

import '../src/lib/iconify-offline'
import MessageList from '../src/lib/components/list/MessageList.svelte'

const mounted = []

function conversation(id, overrides = {}) {
  return {
    threadId: id,
    subject: `Subject ${id}`,
    snippet: `Snippet ${id}`,
    participants: [{ name: `Sender ${id}`, email: `${id}@example.test` }],
    latestDate: '2026-08-01T08:00:00Z',
    messageCount: 1,
    unreadCount: 1,
    isStarred: false,
    hasAttachments: false,
    messageIds: [`message-${id}`],
    accountId: 'account-1',
    folderId: 'inbox-1',
    ...overrides,
  }
}

function buttonWithText(root, text) {
  return [...root.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function renderList(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(MessageList, {
    target,
    props: {
      accountId: 'account-1',
      folderId: 'inbox-1',
      folderName: 'Inbox',
      folderType: 'inbox',
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

beforeEach(() => {
  document.body.innerHTML = ''
  settings.sortOrder = 'newest'
  layout.mode = 'full'
  layout.hideViewer.mockReset()
  accountStore.accounts = [{
    account: {
      id: 'account-1',
      syncAllFolders: false,
      syncFoldersEnabled: false,
    },
    folders: [{
      folder: { id: 'inbox-1', type: 'inbox', subscribed: true },
      children: [],
    }],
  }]
  accountStore.syncProgress = {}
  accountStore.isOnline = true
  backend.GetConversations.mockReset().mockResolvedValue([])
  backend.GetConversationCount.mockReset().mockResolvedValue(0)
  backend.SyncFolder.mockReset().mockResolvedValue(undefined)
  backend.ForceSyncFolder.mockReset().mockResolvedValue(undefined)
  backend.CancelFolderSync.mockReset().mockResolvedValue(undefined)
  backend.SetMessageListSortOrder.mockReset().mockResolvedValue(undefined)
  backend.GetUnifiedInboxConversations.mockReset().mockResolvedValue([])
  backend.GetUnifiedInboxCount.mockReset().mockResolvedValue(0)
  backend.GetFTSIndexStatus.mockReset().mockResolvedValue({
    isComplete: true,
    indexedCount: 0,
    totalCount: 0,
  })
  backend.IsFTSIndexing.mockReset().mockResolvedValue(false)
  backend.SearchConversations.mockReset().mockResolvedValue([])
  backend.GetSearchCount.mockReset().mockResolvedValue(0)
  backend.SearchUnifiedInbox.mockReset().mockResolvedValue([])
  backend.GetSearchCountUnifiedInbox.mockReset().mockResolvedValue(0)
  backend.IMAPSearchFolder.mockReset().mockResolvedValue({ results: [], totalCount: 0 })
  backend.Trash.mockReset().mockResolvedValue(true)
  backend.DeletePermanently.mockReset().mockResolvedValue(undefined)
  backend.EmptyTrash.mockReset().mockResolvedValue(undefined)
  backend.Undo.mockReset().mockResolvedValue('delete')
  backend.FetchServerMessage.mockReset().mockResolvedValue(null)
  backend.Star.mockReset().mockResolvedValue(undefined)
  backend.Unstar.mockReset().mockResolvedValue(undefined)
  runtime.EventsOn.mockClear()
  runtime.handlers.clear()
  accountStore.getFolderUnreadCount.mockClear()
  for (const method of ['success', 'error', 'info', 'warning']) toast[method].mockReset()
  if (!HTMLElement.prototype.scrollTo) {
    HTMLElement.prototype.scrollTo = function scrollTo(options) {
      if (typeof options === 'object' && options?.top != null) this.scrollTop = options.top
    }
  }
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
})

test('loads a folder into real DOM, selects the first row, and supports keyboard selection APIs', async () => {
  const rows = [conversation('thread-1'), conversation('thread-2', { unreadCount: 0, isStarred: true })]
  backend.GetConversations.mockResolvedValue(rows)
  backend.GetConversationCount.mockResolvedValue(2)
  const onConversationFocus = vi.fn()
  const onConversationSelect = vi.fn()

  const { instance, target } = await renderList({ onConversationFocus, onConversationSelect })

  assert.match(target.textContent, /Subject thread-1/)
  assert.match(target.textContent, /Subject thread-2/)
  assert.match(target.textContent, /messageList\.unread:\{"count":7\}/)
  assert.equal(instance.getSelectedThreadId(), 'thread-1')
  assert.deepEqual(instance.getSelectedMessageIds(), ['message-thread-1'])
  assert.deepEqual(instance.getSelectedConversationInfo(), { accountId: 'account-1', folderId: 'inbox-1' })
  assert.equal(onConversationFocus.mock.calls.at(-1)?.[0], 'thread-1')

  await instance.selectNext()
  assert.equal(instance.getSelectedThreadId(), 'thread-2')
  assert.equal(instance.isSelectedStarred(), true)
  assert.equal(onConversationFocus.mock.calls.at(-1)?.[0], 'thread-2')

  instance.openSelected()
  assert.deepEqual(onConversationSelect.mock.calls.at(-1), ['thread-2', 'inbox-1', 'account-1'])

  instance.toggleCheck()
  assert.equal(instance.hasCheckedMessages(), true)
  assert.deepEqual(instance.getCheckedMessageIds(), ['message-thread-2'])
  instance.selectAll()
  assert.deepEqual(instance.getCheckedMessageIds().sort(), ['message-thread-1', 'message-thread-2'])
  assert.equal(instance.getCheckedHasUnstarred(), true)
  assert.equal(instance.getCheckedHasUnread(), true)
  instance.clearChecked()
  assert.equal(instance.hasCheckedMessages(), false)
})

test('renders empty and error states, retries, and reports an authoritative empty folder', async () => {
  const onEmptyFolder = vi.fn()
  const { target } = await renderList({ onEmptyFolder })

  assert.match(target.textContent, /messageList\.noMessages/)
  assert.equal(onEmptyFolder.mock.calls.length, 1)

  backend.GetConversations.mockRejectedValueOnce(new Error('database unavailable'))
  const retryButton = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('messageList.syncNow'))
  assert.ok(retryButton)
  retryButton.click()
  await flushAsync()
  assert.match(target.textContent, /viewer\.failedToLoadMessages/)
  assert.match(target.textContent, /messageList\.tryAgain/)

  backend.GetConversations.mockResolvedValueOnce([conversation('recovered')])
  backend.GetConversationCount.mockResolvedValueOnce(1)
  const tryAgain = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('messageList.tryAgain'))
  assert.ok(tryAgain)
  tryAgain.click()
  await flushAsync()
  assert.match(target.textContent, /Subject recovered/)
})

test('opens and closes search with actual focus, changes sort order, and cancels active sync', async () => {
  vi.useFakeTimers()
  accountStore.syncProgress = { 'account-1': { 'inbox-1': { phase: 'headers', percentage: 40 } } }
  backend.GetConversations.mockResolvedValue([conversation('thread-1')])
  backend.GetConversationCount.mockResolvedValue(1)
  const { instance, target } = await renderList()

  instance.toggleSearchFocus()
  await tick()
  vi.advanceTimersByTime(50)
  await tick()
  const search = target.querySelector('input[placeholder="messageList.searchMessages"]')
  assert.ok(search)
  assert.equal(document.activeElement, search)

  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  await flushAsync()
  assert.equal(target.querySelector('input[placeholder="messageList.searchMessages"]'), null)

  const sort = target.querySelector('button[title="messageList.showingNewest"]')
  assert.ok(sort)
  sort.click()
  await flushAsync()
  assert.deepEqual(backend.SetMessageListSortOrder.mock.calls.at(-1), ['oldest'])
  assert.equal(settings.sortOrder, 'oldest')

  const cancel = [...target.querySelectorAll('button')].find((button) => button.title.includes('sidebar.clickToCancel'))
  assert.ok(cancel)
  cancel.click()
  await flushAsync()
  assert.deepEqual(backend.CancelFolderSync.mock.calls.at(-1), ['account-1', 'inbox-1'])
})

test('uses unified inbox APIs and never attempts a direct folder sync', async () => {
  backend.GetUnifiedInboxConversations.mockResolvedValue([conversation('unified', {
    accountId: 'account-2',
    folderId: 'inbox-2',
  })])
  backend.GetUnifiedInboxCount.mockResolvedValue(1)

  const { instance, target } = await renderList({ accountId: 'unified', folderId: 'inbox' })

  assert.match(target.textContent, /Subject unified/)
  assert.deepEqual(instance.getSelectedConversationInfo(), { accountId: 'account-2', folderId: 'inbox-2' })
  await instance.syncFolder()
  assert.equal(backend.SyncFolder.mock.calls.length, 0)
  assert.equal(backend.GetUnifiedInboxConversations.mock.calls.length, 1)
})

test('searches locally with pagination, switches to server results, and fetches a non-local hit', async () => {
  vi.useFakeTimers()
  backend.GetConversations.mockResolvedValue([conversation('folder-row')])
  backend.GetConversationCount.mockResolvedValue(1)
  const localFirst = conversation('local-1', {
    highlightedSubject: '<mark>Local</mark> result',
    highlightedSnippet: 'matched locally',
    folderName: 'Inbox',
    folderType: 'inbox',
  })
  const localSecond = conversation('local-2', {
    highlightedSubject: '<mark>Local</mark> second',
    folderName: 'Archive',
    folderType: 'archive',
  })
  backend.SearchConversations.mockImplementation(async (_accountId, _folderId, _query, offset) => (
    offset === 0 ? [localFirst] : [localSecond]
  ))
  backend.GetSearchCount.mockResolvedValue(2)
  backend.IMAPSearchFolder.mockResolvedValue({
    results: [
      {
        uid: 101,
        messageId: 'server-local',
        subject: 'Already local server result',
        snippet: 'local snippet',
        isLocal: true,
        isRead: true,
        hasAttachments: false,
        isStarred: false,
        date: '2026-08-01T09:00:00Z',
        fromName: 'Local Sender',
        fromEmail: 'local@example.test',
        accountId: 'account-1',
        folderId: 'inbox-1',
      },
      {
        uid: 202,
        messageId: '',
        subject: 'Remote-only server result',
        isLocal: false,
        isRead: false,
        hasAttachments: true,
        isStarred: true,
        date: '2026-08-01T10:00:00Z',
        fromName: 'Remote Sender',
        fromEmail: 'remote@example.test',
        accountId: 'account-1',
        folderId: 'inbox-1',
      },
    ],
    totalCount: 3,
  })
  backend.FetchServerMessage.mockResolvedValue({
    id: 'fetched-message',
    threadId: 'fetched-thread',
    snippet: 'downloaded from server',
  })
  const onConversationSelect = vi.fn()
  const { instance, target } = await renderList({ onConversationSelect })

  instance.toggleSearchFocus()
  await vi.advanceTimersByTimeAsync(60)
  await flushAsync()
  const search = target.querySelector('input[placeholder="messageList.searchMessages"]')
  search.value = 'quarterly'
  search.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.match(target.textContent, /Local result/)
  assert.deepEqual(backend.SearchConversations.mock.calls[0].slice(0, 4), [
    'account-1',
    'inbox-1',
    'quarterly',
    0,
  ])

  const loadMore = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('messageList.loadMore'))
  assert.ok(loadMore)
  loadMore.click()
  await flushAsync()
  assert.match(target.textContent, /Local second/)
  assert.equal(backend.SearchConversations.mock.calls.at(-1)[3], 50)

  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(backend.IMAPSearchFolder.mock.calls.length, 1)
  assert.match(target.textContent, /Remote-only server result/)
  assert.match(target.textContent, /search\.serverResultsCapped/)

  const remoteRow = [...target.querySelectorAll('[data-conversation-row]')]
    .find((row) => row.textContent.includes('Remote-only server result'))
  assert.ok(remoteRow)
  remoteRow.click()
  await flushAsync()
  assert.deepEqual(backend.FetchServerMessage.mock.calls.at(-1), ['account-1', 'inbox-1', 202])
  assert.deepEqual(onConversationSelect.mock.calls.at(-1), ['fetched-thread', 'inbox-1', 'account-1'])

  const showAll = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('search.showAllResults'))
  assert.ok(showAll)
  showAll.click()
  await flushAsync()
  assert.equal(backend.IMAPSearchFolder.mock.calls.at(-1)[3], 0)

  search.value = 'changed query'
  search.dispatchEvent(new Event('input', { bubbles: true }))
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(backend.IMAPSearchFolder.mock.calls.at(-1)[2], 'changed query')
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  assert.match(target.textContent, /Local result/)
})

test('supports pointer range selection, keyboard range selection, row drag, draft open, and context menu', async () => {
  const rows = [conversation('one'), conversation('two'), conversation('three')]
  backend.GetConversations.mockResolvedValue(rows)
  backend.GetConversationCount.mockResolvedValue(rows.length)
  const onConversationSelect = vi.fn()
  const onOpenDraft = vi.fn()
  const { instance, target } = await renderList({
    folderType: 'drafts',
    onConversationSelect,
    onOpenDraft,
  })
  let renderedRows = target.querySelectorAll('[data-conversation-row]')

  renderedRows[1].dispatchEvent(new MouseEvent('click', { bubbles: true, metaKey: true }))
  await flushAsync()
  assert.deepEqual(instance.getCheckedMessageIds().sort(), ['message-one', 'message-two'])
  renderedRows[2].dispatchEvent(new MouseEvent('click', { bubbles: true, shiftKey: true }))
  await flushAsync()
  assert.deepEqual(instance.getCheckedMessageIds().sort(), ['message-one', 'message-three', 'message-two'])

  await instance.selectPrevious()
  instance.selectNextWithCheck()
  assert.deepEqual(instance.getCheckedMessageIds().sort(), ['message-one', 'message-two'])
  instance.selectPreviousWithCheck()
  assert.deepEqual(instance.getCheckedMessageIds().sort(), ['message-one', 'message-two'])
  instance.selectPreviousWithCheck()
  instance.selectAll()
  assert.equal(instance.getCheckedMessageIds().length, 3)

  renderedRows = target.querySelectorAll('[data-conversation-row]')
  const transferValues = new Map()
  const dataTransfer = {
    effectAllowed: 'none',
    setData(type, value) { transferValues.set(type, value) },
  }
  const drag = new Event('dragstart', { bubbles: true, cancelable: true })
  Object.defineProperty(drag, 'dataTransfer', { value: dataTransfer })
  renderedRows[1].dispatchEvent(drag)
  assert.equal(dataTransfer.effectAllowed, 'move')
  assert.deepEqual(JSON.parse(transferValues.get('application/x-aulycmail-messages')).messageIds.sort(), [
    'message-one',
    'message-three',
    'message-two',
  ])

  renderedRows[1].dispatchEvent(new MouseEvent('dblclick', { bubbles: true }))
  assert.deepEqual(onOpenDraft.mock.calls.at(-1), ['message-two'])
  renderedRows[1].dispatchEvent(new PointerEvent('pointermove', { bubbles: true, movementX: 0, movementY: 0 }))
  renderedRows[1].dispatchEvent(new PointerEvent('pointermove', { bubbles: true, movementX: 2, movementY: 1 }))
  renderedRows[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  await flushAsync()
  assert.deepEqual(onConversationSelect.mock.calls.at(-1), ['two', 'inbox-1', 'account-1'])

  await instance.openContextMenu()
  renderedRows[1].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
  await flushAsync()
})

test('deletes with undo, confirms permanent deletion, and empties trash with success and failure feedback', async () => {
  backend.GetConversations.mockResolvedValue([conversation('delete-me')])
  backend.GetConversationCount.mockResolvedValue(1)
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { instance } = await renderList()

  instance.requestDelete(['message-delete-me'])
  await flushAsync()
  assert.deepEqual(backend.Trash.mock.calls.at(-1), [['message-delete-me']])
  assert.equal(toast.success.mock.calls.length > 0, true)
  const undoAction = toast.success.mock.calls[0][1][0]
  assert.equal(undoAction.label, 'common.undo')
  await undoAction.onClick()
  assert.equal(backend.Undo.mock.calls.length, 1)

  backend.Trash.mockResolvedValueOnce(false)
  instance.requestDelete(['message-delete-me'])
  await flushAsync()
  assert.deepEqual(toast.success.mock.calls.at(-1)[1], [])

  instance.requestDelete(['message-delete-me'], true)
  await flushAsync()
  let confirm = document.querySelector('[role="alertdialog"] button:last-child')
  assert.ok(confirm)
  confirm.click()
  await flushAsync()
  assert.deepEqual(backend.DeletePermanently.mock.calls.at(-1), [['message-delete-me']])

  backend.Trash.mockRejectedValueOnce(new Error('trash failed'))
  instance.requestDelete(['message-delete-me'])
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.failedToDelete')

  backend.DeletePermanently.mockRejectedValueOnce(new Error('permanent failed'))
  instance.requestDelete(['message-delete-me'], true)
  await flushAsync()
  confirm = document.querySelector('[role="alertdialog"] button:last-child')
  confirm.click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.failedToDelete')
  assert.equal(error.mock.calls.length >= 2, true)

  while (mounted.length > 0) await unmount(mounted.pop())
  backend.GetConversations.mockResolvedValue([conversation('trash-row', { folderId: 'trash-1' })])
  backend.GetConversationCount.mockResolvedValue(1)
  const trash = await renderList({ folderId: 'trash-1', folderName: 'Trash', folderType: 'trash' })
  const emptyTrash = [...trash.target.querySelectorAll('button')].find((button) => button.textContent.includes('messageList.emptyTrash'))
  assert.ok(emptyTrash)
  emptyTrash.click()
  await flushAsync()
  confirm = document.querySelector('[role="alertdialog"] button:last-child')
  confirm.click()
  await flushAsync()
  assert.deepEqual(backend.EmptyTrash.mock.calls.at(-1), ['account-1', 'trash-1'])
})

test('applies runtime read and FTS events and coalesces folder refreshes', async () => {
  vi.useFakeTimers()
  backend.GetConversations.mockResolvedValue([conversation('event-row', { unreadCount: 1 })])
  backend.GetConversationCount.mockResolvedValue(1)
  backend.GetFTSIndexStatus.mockResolvedValue({ isComplete: false, indexedCount: 2, totalCount: 10 })
  backend.IsFTSIndexing.mockResolvedValue(true)
  const { instance, target } = await renderList()

  instance.toggleSearchFocus()
  await vi.advanceTimersByTimeAsync(60)
  await flushAsync()
  assert.match(target.textContent, /messageList\.ftsBuilding:\{"percentage":20\}/)

  runtime.handlers.get('fts:progress')({ folderId: 'other', percentage: 90 })
  runtime.handlers.get('fts:progress')({ folderId: 'inbox-1', percentage: 45 })
  await flushAsync()
  assert.match(target.textContent, /"percentage":45/)
  runtime.handlers.get('fts:complete')({ folderId: 'inbox-1' })
  runtime.handlers.get('fts:indexing')({ status: 'started' })
  runtime.handlers.get('fts:indexing')({ status: 'completed' })
  runtime.handlers.get('fts:indexing')({ status: 'ignored' })

  runtime.handlers.get('messages:readChanged')({ messageIds: ['message-event-row'], isRead: true })
  instance.selectAll()
  assert.equal(instance.getCheckedHasUnread(), false)

  const callsBeforeRefresh = backend.GetConversations.mock.calls.length
  runtime.handlers.get('folder:synced')({ accountId: 'other', folderId: 'other' })
  runtime.handlers.get('folder:synced')({ accountId: 'account-1', folderId: 'inbox-1' })
  runtime.handlers.get('messages:updated')({ accountId: 'account-1', folderId: 'inbox-1' })
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.equal(backend.GetConversations.mock.calls.length, callsBeforeRefresh + 1)
})

test('pages to a requested thread, tracks scrolling, and runs header actions', async () => {
  const firstPage = Array.from({ length: 50 }, (_, index) => conversation(`page-${index}`))
  const targetThread = conversation('requested')
  backend.GetConversations.mockImplementation(async (_accountId, _folderId, offset) => (
    offset === 0 ? firstPage : [targetThread]
  ))
  backend.GetConversationCount.mockResolvedValue(51)
  const onSearch = vi.fn()
  const onToggleSidebar = vi.fn()
  const { instance, target } = await renderList({ showFolderToggle: true, onSearch, onToggleSidebar })
  const scroller = target.querySelector('.flex-1.overflow-y-auto.scrollbar-thin')
  Object.defineProperty(scroller, 'clientHeight', { configurable: true, value: 120 })
  scroller.scrollTo = vi.fn(function scrollTo(options) { this.scrollTop = options.top })
  scroller.scrollTop = 88
  scroller.dispatchEvent(new Event('scroll', { bubbles: true }))

  target.querySelector('button[title="messageList.scrollToTop"]').click()
  assert.equal(scroller.scrollTop, 0)
  assert.deepEqual(scroller.scrollTo.mock.calls.at(-1), [{ top: 0, behavior: 'smooth' }])
  target.querySelector('button[title="common.search"]').click()
  target.querySelector('button[title="responsive.folders"]').click()
  assert.equal(onSearch.mock.calls.length, 1)
  assert.equal(onToggleSidebar.mock.calls.length, 1)

  await instance.selectThread('requested')
  await flushAsync()
  assert.equal(instance.getSelectedThreadId(), 'requested')
  assert.equal(backend.GetConversations.mock.calls.some((call) => call[2] === 50), true)
  assert.equal(scroller.scrollTop > 0, true)

  await instance.selectThread('missing-thread')
  assert.equal(instance.getSelectedThreadId(), 'missing-thread')
})

test('loads normal pagination and toggles idle and active folder synchronization', async () => {
  backend.GetConversations.mockImplementation(async (_accountId, _folderId, offset) => (
    offset === 0 ? [conversation('first-page')] : [conversation('second-page')]
  ))
  backend.GetConversationCount.mockResolvedValue(51)
  let rendered = await renderList()
  const loadMore = [...rendered.target.querySelectorAll('button')]
    .find((item) => item.textContent.includes('messageList.loadMore'))
  assert.ok(loadMore)
  loadMore.click()
  await flushAsync()
  assert.match(rendered.target.textContent, /Subject second-page/)
  assert.equal(backend.GetConversations.mock.calls.at(-1)[2], 50)

  await rendered.instance.toggleFolderSync()
  assert.deepEqual(backend.SyncFolder.mock.calls.at(-1), ['account-1', 'inbox-1'])

  await unmount(mounted.pop())
  accountStore.syncProgress = { 'account-1': { 'inbox-1': { phase: 'headers', percentage: 10 } } }
  rendered = await renderList()
  await rendered.instance.toggleFolderSync()
  assert.deepEqual(backend.CancelFolderSync.mock.calls.at(-1), ['account-1', 'inbox-1'])
})

test('renders the no-folder state and reports sync, cancel, force-sync, and sort failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  let rendered = await renderList({ accountId: null, folderId: null })
  assert.match(rendered.target.textContent, /messageList\.selectFolder/)
  await rendered.instance.syncFolder()
  await rendered.instance.cancelFolderSync()
  assert.equal(backend.SyncFolder.mock.calls.length, 0)
  assert.equal(backend.CancelFolderSync.mock.calls.length, 0)

  await unmount(mounted.pop())
  backend.GetConversations.mockResolvedValue([conversation('failure-row')])
  backend.GetConversationCount.mockResolvedValue(1)
  rendered = await renderList()
  backend.SyncFolder.mockRejectedValueOnce(new Error('sync unavailable'))
  await rendered.instance.syncFolder()
  assert.match(rendered.target.textContent, /viewer\.failedToLoadMessages/)

  backend.CancelFolderSync.mockRejectedValueOnce(new Error('cancel unavailable'))
  await rendered.instance.cancelFolderSync()
  backend.SetMessageListSortOrder.mockRejectedValueOnce(new Error('sort unavailable'))
  rendered.target.querySelector('button[title="messageList.showingNewest"]').click()
  await flushAsync()
  assert.equal(settings.sortOrder, 'newest')
  assert.equal(error.mock.calls.length >= 3, true)
})

test('uses the sync and filter menus for force refresh and every filter mode', async () => {
  backend.GetConversations.mockResolvedValue([conversation('menu-row')])
  backend.GetConversationCount.mockResolvedValue(1)
  const { target } = await renderList()
  const menuTriggers = target.querySelectorAll('[aria-haspopup="menu"]')
  assert.equal(menuTriggers.length, 2)

  menuTriggers[0].dispatchEvent(new PointerEvent('pointerdown', {
    button: 0, pointerType: 'mouse', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  const forceItem = [...document.querySelectorAll('[role="menuitem"]')]
    .find((item) => item.textContent.includes('messageList.forceResync'))
  assert.ok(forceItem)
  forceItem.click()
  await flushAsync()
  assert.deepEqual(backend.ForceSyncFolder.mock.calls.at(-1), ['account-1', 'inbox-1'])

  for (const [label, mode] of [
    ['messageList.filterUnread', 'unread'],
    ['messageList.filterStarred', 'starred'],
    ['messageList.filterAttachments', 'attachments'],
  ]) {
    menuTriggers[1].dispatchEvent(new PointerEvent('pointerdown', {
      button: 0, pointerType: 'mouse', bubbles: true, cancelable: true,
    }))
    await flushAsync()
    const item = [...document.querySelectorAll('[role="menuitem"]')]
      .find((candidate) => candidate.textContent.includes(label))
    assert.ok(item, `missing ${label}`)
    item.click()
    await flushAsync()
    assert.equal(backend.GetConversations.mock.calls.at(-1)[5], mode)
    assert.match(target.textContent, new RegExp(label.replace('.', '\\.')))
  }

  const activeFilter = [...target.querySelectorAll('button')]
    .find((item) => item.textContent.includes('messageList.filterAttachments'))
  assert.ok(activeFilter)
  activeFilter.click()
  await flushAsync()
  assert.equal(backend.GetConversations.mock.calls.at(-1)[5], '')
  assert.doesNotMatch(target.textContent, /messageList\.filterLabel/)
})

test('covers local and server search empty states, retries, and backend failures', async () => {
  vi.useFakeTimers()
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetConversations.mockResolvedValue([conversation('search-base')])
  backend.GetConversationCount.mockResolvedValue(1)
  const { instance, target } = await renderList()

  instance.toggleSearchFocus()
  await vi.advanceTimersByTimeAsync(60)
  await flushAsync()
  const search = target.querySelector('input[placeholder="messageList.searchMessages"]')
  search.value = 'failing local query'
  backend.SearchConversations.mockRejectedValueOnce(new Error('local index unavailable'))
  search.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.match(target.textContent, /viewer\.failedToLoadMessages/)

  backend.SearchConversations.mockResolvedValueOnce([])
  backend.GetSearchCount.mockResolvedValueOnce(0)
  buttonWithText(target, 'messageList.tryAgain')?.click()
  await flushAsync()
  assert.match(target.textContent, /messageList\.noResults/)

  backend.IMAPSearchFolder.mockRejectedValueOnce(new Error('server search unavailable'))
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  assert.match(target.textContent, /viewer\.failedToLoadMessages/)

  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  backend.IMAPSearchFolder.mockResolvedValueOnce({ results: [], totalCount: 0 })
  search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  assert.match(target.textContent, /messageList\.noResults/)

  search.value = '   '
  search.dispatchEvent(new Event('input', { bubbles: true }))
  await flushAsync()
  assert.equal(backend.SearchConversations.mock.calls.length >= 2, true)
  assert.equal(error.mock.calls.length >= 2, true)
})

test('reloads after actions in normal and search modes and hides the narrow viewer', async () => {
  vi.useFakeTimers()
  backend.GetConversations.mockResolvedValue([
    conversation('action-one'),
    conversation('action-two'),
  ])
  backend.GetConversationCount.mockResolvedValue(2)
  const onRowActionComplete = vi.fn()
  const rendered = await renderList({ onRowActionComplete })
  layout.mode = 'narrow'
  rendered.instance.selectAll()
  rendered.instance.handleActionComplete(true)
  await flushAsync()
  await vi.runAllTimersAsync()
  await flushAsync()
  assert.equal(onRowActionComplete.mock.calls.length, 1)
  assert.equal(layout.hideViewer.mock.calls.length, 1)
  assert.equal(backend.GetConversations.mock.calls.at(-1)[3], 50)

  rendered.instance.toggleSearchFocus()
  await vi.advanceTimersByTimeAsync(60)
  await flushAsync()
  const search = rendered.target.querySelector('input[placeholder="messageList.searchMessages"]')
  backend.SearchConversations.mockResolvedValue([conversation('search-action')])
  backend.GetSearchCount.mockResolvedValue(1)
  search.value = 'search action'
  search.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  rendered.instance.handleActionComplete(true)
  await flushAsync()
  await vi.runAllTimersAsync()
  await flushAsync()
  assert.equal(backend.SearchConversations.mock.calls.length >= 2, true)
  assert.equal(layout.hideViewer.mock.calls.length, 2)
})

test('automatically catches up optional folders and reloads unified inbox for nested inbox events', async () => {
  vi.useFakeTimers()
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  accountStore.accounts[0].folders.push({
    folder: { id: 'archive-1', type: 'archive', subscribed: false },
    children: [{
      folder: { id: 'nested-inbox', type: 'inbox', subscribed: true },
      children: [],
    }],
  })
  backend.GetConversations.mockResolvedValue([conversation('archive-row', { folderId: 'archive-1' })])
  backend.GetConversationCount.mockResolvedValue(1)
  await renderList({ folderId: 'archive-1', folderName: 'Archive', folderType: 'archive' })
  await flushAsync()
  assert.deepEqual(backend.SyncFolder.mock.calls.at(-1), ['account-1', 'archive-1'])
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.equal(backend.GetConversations.mock.calls.length >= 2, true)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.SyncFolder.mockRejectedValueOnce(new Error('automatic catch-up unavailable'))
  accountStore.accounts[0].folders.push({
    folder: { id: 'custom-2', type: 'folder', subscribed: false },
    children: [],
  })
  await renderList({ folderId: 'custom-2', folderName: 'Custom', folderType: 'folder' })
  await flushAsync()
  assert.equal(warn.mock.calls.length, 1)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetUnifiedInboxConversations.mockResolvedValue([conversation('unified-event')])
  backend.GetUnifiedInboxCount.mockResolvedValue(1)
  await renderList({ accountId: 'unified', folderId: 'inbox' })
  const before = backend.GetUnifiedInboxConversations.mock.calls.length
  runtime.handlers.get('folder:synced')({ accountId: 'account-1', folderId: 'missing' })
  runtime.handlers.get('folder:synced')({ accountId: 'account-1', folderId: 'nested-inbox' })
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.equal(backend.GetUnifiedInboxConversations.mock.calls.length, before + 1)
})
