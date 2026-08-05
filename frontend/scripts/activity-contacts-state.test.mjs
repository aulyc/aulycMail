import assert from 'node:assert/strict'
import { beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  ClearActivityLogs: vi.fn(),
  ListActivityLogs: vi.fn(),
  GetBackupRunState: vi.fn(),
  StartEmailBackup: vi.fn(),
  Contacts_GetContactAccountGroups: vi.fn(),
  Contacts_BrowseContacts: vi.fn(),
  Contacts_GetContactDetail: vi.fn(),
  Contacts_UpdateContact: vi.fn(),
  Contacts_DeleteLocalContact: vi.fn(),
  Contacts_CreateContact: vi.fn(),
}))
const runtime = vi.hoisted(() => ({ EventsOn: vi.fn() }))
const toast = vi.hoisted(() => ({ add: vi.fn() }))
const layout = vi.hoisted(() => ({
  responsive: vi.fn(() => false),
  showViewer: vi.fn(),
  hideSidebar: vi.fn(),
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime.js', () => runtime)
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/layout.svelte', () => ({
  isResponsive: layout.responsive,
  showViewer: layout.showViewer,
  hideSidebar: layout.hideSidebar,
}))

import { ActivityLogsStore } from '../src/lib/components/settings/activity/activityLogs.svelte.ts'
import { BackupRunStore } from '../src/lib/components/settings/backup/backupRun.svelte.ts'
import {
  beginContactRefresh,
  completeContactRefresh,
  contactRefresh,
  failContactRefresh,
  initContactRefreshEvents,
} from '../src/lib/contacts/stores/contactRefresh.svelte.ts'
import {
  contactAccountGroups,
  loadContactAccountGroups,
  preloadContactAccountGroups,
} from '../src/lib/contacts/stores/contactAccountGroups.svelte.ts'
import {
  activateContact,
  activateContactFromGlobalSearch,
  contactsView,
  createContact,
  deleteLocalContact,
  focusContact,
  loadMoreContacts,
  reloadContacts,
  selectSource,
  setSearchQuery,
  setSortOrder,
  updateContact,
} from '../src/lib/contacts/stores/contactsView.svelte.ts'

beforeEach(() => {
  vi.clearAllMocks()
  for (const fn of Object.values(backend)) fn.mockResolvedValue(undefined)
  runtime.EventsOn.mockReturnValue(vi.fn())
  layout.responsive.mockReturnValue(false)
})

test('activity log store subscribes once, queries filters, paginates, clears, and reports failures', async () => {
  const unsubscribe = vi.fn()
  let createdListener
  runtime.EventsOn.mockImplementation((event, listener) => {
    createdListener = listener
    return unsubscribe
  })
  backend.ListActivityLogs
    .mockResolvedValueOnce({ entries: [{ id: 'first' }], total: 2 })
    .mockResolvedValueOnce({ entries: [{ id: 'second' }], total: 2 })
    .mockResolvedValueOnce({ entries: [], total: 0 })
    .mockResolvedValueOnce({ entries: [], total: 0 })

  const store = new ActivityLogsStore()
  store.type = 'sync'
  store.problemOnly = true
  store.date = '2026-08-01'
  store.start()
  store.start()
  assert.equal(runtime.EventsOn.mock.calls.length, 1)

  await store.refresh()
  assert.deepEqual(store.entries, [{ id: 'first' }])
  assert.equal(store.total, 2)
  assert.equal(store.hasMore, true)
  assert.deepEqual(backend.ListActivityLogs.mock.calls[0][0], {
    type: 'sync', problemOnly: true, date: '2026-08-01',
    timezoneOffsetMinutes: new Date().getTimezoneOffset(), directory: '', limit: 50, offset: 0,
  })

  await store.loadMore()
  assert.deepEqual(store.entries.map((entry) => entry.id), ['first', 'second'])
  assert.equal(store.hasMore, false)
  await store.loadMore()
  assert.equal(backend.ListActivityLogs.mock.calls.length, 2)

  await store.setFilter('backup', false)
  assert.equal(store.type, 'backup')
  await store.clearCurrent()
  assert.equal(store.clearing, false)
  assert.equal(backend.ClearActivityLogs.mock.calls[0][0].type, 'backup')

  backend.ListActivityLogs.mockRejectedValueOnce(new Error('load failed'))
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  await store.refresh()
  assert.equal(store.loadFailed, true)

  store.entries = [{ id: 'one' }]
  store.total = 2
  backend.ListActivityLogs.mockRejectedValueOnce(new Error('more failed'))
  await store.loadMore()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'activityLog.loadFailed')

  backend.ClearActivityLogs.mockRejectedValueOnce(new Error('clear failed'))
  await store.clearAll()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'activityLog.clearFailed')
  log.mockRestore()

  await createdListener()
  store.stop()
  assert.equal(unsubscribe.mock.calls.length, 1)
})

test('backup run store tracks backend state, progress events, success, and start failures', async () => {
  const unsubscribe = vi.fn()
  let progressListener
  runtime.EventsOn.mockImplementation((event, listener) => {
    progressListener = listener
    return unsubscribe
  })
  backend.GetBackupRunState.mockResolvedValue({
    running: false,
    progress: { phase: 'complete', current: 2, total: 2, exported: 2, skipped: 0, failed: 0 },
  })
  const store = new BackupRunStore()
  await store.startListening()
  await store.startListening()
  assert.equal(runtime.EventsOn.mock.calls.length, 1)
  assert.equal(store.running, false)
  assert.equal(store.progress.phase, 'complete')

  progressListener({ phase: 'running', current: 1, total: 3, exported: 1, skipped: 0, failed: 0 })
  assert.equal(store.running, true)
  assert.equal(store.progress.current, 1)

  backend.StartEmailBackup.mockResolvedValue({
    running: true,
    progress: { phase: 'running', current: 0, total: 3, exported: 0, skipped: 0, failed: 0 },
  })
  backend.GetBackupRunState.mockResolvedValueOnce({ running: true, progress: null })
  await store.start('/backup', 'selected', ['account'])
  assert.deepEqual(backend.StartEmailBackup.mock.calls[0][0], {
    directory: '/backup', scope: 'selected', selectedAccountIds: ['account'],
  })
  assert.equal(store.loading, false)
  assert.equal(store.running, true)

  backend.StartEmailBackup.mockRejectedValueOnce(new Error('start failed'))
  backend.GetBackupRunState.mockResolvedValueOnce({ running: false, progress: null })
  await assert.rejects(store.start('/backup', 'all', []), /start failed/)
  assert.equal(store.loading, false)
  assert.equal(store.running, false)
  store.stopListening()
  assert.equal(unsubscribe.mock.calls.length, 1)
})

test('contact refresh normalizes progress, computes percentage, and installs one listener', () => {
  let listener
  runtime.EventsOn.mockImplementation((event, callback) => {
    listener = callback
    return vi.fn()
  })
  initContactRefreshEvents()
  initContactRefreshEvents()
  assert.equal(runtime.EventsOn.mock.calls.length, 1)

  beginContactRefresh()
  assert.deepEqual({ active: contactRefresh.active, scanned: contactRefresh.scanned, total: contactRefresh.total }, {
    active: true, scanned: 0, total: 0,
  })
  assert.equal(contactRefresh.percentage, null)
  listener({ phase: 'scanning', scanned: 4.8, total: 10 })
  assert.equal(contactRefresh.percentage, 40)
  listener({ phase: 'scanning', scanned: 20, total: 10 })
  assert.equal(contactRefresh.active, false)
  assert.equal(contactRefresh.percentage, null)

  completeContactRefresh(Number.NaN, -1)
  assert.equal(contactRefresh.scanned, 0)
  assert.equal(contactRefresh.total, 0)
  beginContactRefresh()
  failContactRefresh()
  assert.equal(contactRefresh.active, false)
})

test('contact account groups cache, coalesce, force refresh, and preserve loaded data on errors', async () => {
  let resolveFirst
  backend.Contacts_GetContactAccountGroups.mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
  const first = loadContactAccountGroups()
  const coalesced = loadContactAccountGroups()
  assert.equal(backend.Contacts_GetContactAccountGroups.mock.calls.length, 1)
  resolveFirst([{ accountId: 'one', email: 'one@example.com', count: 1 }])
  await Promise.all([first, coalesced])
  assert.equal(contactAccountGroups.loaded, true)
  assert.equal(contactAccountGroups.loading, false)
  assert.equal(contactAccountGroups.groups[0].accountId, 'one')

  await loadContactAccountGroups()
  assert.equal(backend.Contacts_GetContactAccountGroups.mock.calls.length, 1)
  backend.Contacts_GetContactAccountGroups.mockResolvedValueOnce(null)
  await loadContactAccountGroups({ force: true })
  assert.deepEqual(contactAccountGroups.groups, [])

  backend.Contacts_GetContactAccountGroups.mockRejectedValueOnce(new Error('groups failed'))
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  await loadContactAccountGroups({ force: true })
  log.mockRestore()
  assert.equal(contactAccountGroups.loaded, true)
  assert.deepEqual(contactAccountGroups.groups, [])
  preloadContactAccountGroups()
})

test('contacts view covers source, search, sort, list pagination, detail, responsive activation, and mutations', async () => {
  selectSource('local:manual')
  assert.equal(contactsView.selectedSourceId, 'local:manual')
  assert.equal(contactsView.searchQuery, '')
  assert.equal(layout.hideSidebar.mock.calls.length, 1)
  setSearchQuery('alice')
  setSortOrder('name-desc')

  const first = { id: 'alice@example.com', name: 'Alice' }
  const second = { id: 'bob@example.com', name: 'Bob' }
  backend.Contacts_BrowseContacts.mockResolvedValueOnce({ items: [first], total: 2 })
  backend.Contacts_GetContactDetail.mockResolvedValueOnce({ id: first.id, name: 'Alice Detail' })
  await reloadContacts(1)
  await vi.waitFor(() => assert.equal(contactsView.detail?.id, first.id))
  assert.equal(contactsView.loading, false)
  assert.equal(contactsView.hasMore, true)
  assert.equal(contactsView.remaining, 1)
  assert.deepEqual(backend.Contacts_BrowseContacts.mock.calls[0], ['alice', 'local:manual', 'name-desc', 1, 0])

  backend.Contacts_BrowseContacts.mockResolvedValueOnce({ items: [first, second], total: 2 })
  await loadMoreContacts(2)
  assert.deepEqual(contactsView.contacts.map((contact) => contact.id), [first.id, second.id])
  assert.equal(contactsView.hasMore, false)

  layout.responsive.mockReturnValue(true)
  backend.Contacts_GetContactDetail.mockResolvedValueOnce({ id: second.id })
  await activateContact(second.id)
  assert.equal(layout.showViewer.mock.calls.length, 1)
  assert.equal(contactsView.detail.id, second.id)
  await focusContact(null)
  assert.equal(contactsView.detail, null)

  backend.Contacts_GetContactDetail.mockRejectedValueOnce(new Error('detail failed'))
  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  await focusContact(first.id)
  assert.equal(contactsView.detailLoadError, true)

  backend.Contacts_BrowseContacts.mockRejectedValueOnce(new Error('list failed'))
  await reloadContacts()
  assert.equal(contactsView.loadError, true)

  backend.Contacts_BrowseContacts.mockResolvedValue({ items: [first, second], total: 2 })
  backend.Contacts_GetContactDetail.mockResolvedValue({ id: first.id })
  await updateContact(first.id, { name: 'Updated' })
  assert.deepEqual(backend.Contacts_UpdateContact.mock.calls[0], [first.id, { name: 'Updated' }])

  await deleteLocalContact(first.id)
  assert.deepEqual(backend.Contacts_DeleteLocalContact.mock.calls[0], [first.id])

  backend.Contacts_CreateContact.mockResolvedValue('new@example.com')
  const input = { email: 'new@example.com', name: 'New' }
  assert.equal(await createContact(input), 'new@example.com')
  assert.deepEqual(backend.Contacts_CreateContact.mock.calls[0], [input])

  backend.Contacts_BrowseContacts.mockResolvedValueOnce({ items: [], total: 0 })
  backend.Contacts_GetContactDetail.mockResolvedValueOnce({ id: second.id })
  await activateContactFromGlobalSearch(second.id)
  assert.equal(contactsView.selectedSourceId, '')
  assert.equal(contactsView.selectedContactScrollTopSignal > 0, true)
  log.mockRestore()
})
