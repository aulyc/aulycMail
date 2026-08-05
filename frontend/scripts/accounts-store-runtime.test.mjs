import assert from 'node:assert/strict'
import { test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetAccounts: vi.fn(),
  GetFolderTree: vi.fn(),
  SyncAccountComplete: vi.fn(),
  SyncAllComplete: vi.fn(),
  CancelAllSyncs: vi.fn(),
  AddAccount: vi.fn(),
  UpdateAccount: vi.fn(),
  RemoveAccount: vi.fn(),
  TestConnection: vi.fn(),
  TestAccountConnection: vi.fn(),
  GetAccountConnOK: vi.fn(),
  ReorderAccounts: vi.fn(),
}))
const runtime = vi.hoisted(() => ({ EventsOn: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime.js', () => runtime)

import { accountStore } from '../src/lib/stores/accounts.svelte.ts'

test('account store manages loading, trees, live events, sync lifecycle, mutations, and connection delegation', async () => {
  const events = new Map()
  const windowEvents = new Map()
  runtime.EventsOn.mockImplementation((name, listener) => {
    events.set(name, listener)
    return vi.fn()
  })
  vi.stubGlobal('navigator', { onLine: false })
  vi.stubGlobal('window', {
    addEventListener: vi.fn((name, listener) => windowEvents.set(name, listener)),
  })

  const accountA = { id: 'a', email: 'a@example.com', name: 'Account A', enabled: true }
  const accountB = { id: 'b', email: 'b@example.com', name: 'Account B', enabled: true }
  const inbox = { folder: { id: 'inbox', type: 'inbox', unreadCount: 2 }, children: [] }
  const drafts = { folder: { id: 'drafts', type: 'drafts', unreadCount: 1 }, children: [] }
  const parent = { folder: { id: 'parent', type: 'normal', unreadCount: 0 }, children: [drafts] }
  const blocked = { folder: { id: 'blocked', type: 'normal', unreadCount: 0, noSelect: true }, children: [] }

  backend.GetAccounts.mockResolvedValue([accountA, accountB])
  backend.GetFolderTree.mockImplementation(async (id) => id === 'a' ? [inbox, parent, blocked] : [])
  await accountStore.load()
  assert.equal(accountStore.loading, false)
  assert.equal(accountStore.error, null)
  assert.equal(accountStore.accounts.length, 2)
  assert.equal(runtime.EventsOn.mock.calls.length, 7)
  assert.equal(window.addEventListener.mock.calls.length, 2)
  assert.equal(accountStore.isOnline, false)
  assert.equal(accountStore.getFolderUnreadCount('drafts'), 1)
  assert.equal(accountStore.getFolderUnreadCount('missing'), 0)
  assert.equal(accountStore.getFolderUnreadCount(null), 0)

  accountStore.selectFolder('a', 'inbox', 'Inbox', 'Inbox')
  assert.equal(accountStore.selectedFolder.folderId, 'inbox')
  accountStore.selectFolder('a', 'blocked', 'Blocked', 'Blocked')
  assert.equal(accountStore.selectedFolder.folderId, 'inbox')
  assert.equal(accountStore.getAccount('a').account.email, 'a@example.com')

  events.get('folders:countsChanged')({ inbox: 9, drafts: 4 })
  assert.equal(accountStore.getFolderUnreadCount('inbox'), 9)
  assert.equal(accountStore.getFolderUnreadCount('drafts'), 4)

  events.get('sync:progress')({ accountId: 'a', folderId: '', fetched: 0, total: 0, phase: 'folders' })
  events.get('sync:progress')({ accountId: 'a', folderId: 'inbox', fetched: 8, total: 10, phase: 'headers' })
  events.get('sync:progress')({ accountId: 'a', folderId: 'drafts', fetched: 20, total: 10, phase: 'bodies' })
  assert.equal(accountStore.isAnySyncing, true)
  assert.equal(accountStore.getSyncProgress('a').folderId, 'inbox')
  assert.equal(accountStore.getSyncProgress('a').percentage, 80)

  const log = vi.spyOn(console, 'error').mockImplementation(() => {})
  events.get('folder:syncError')({ accountId: 'a', folderId: 'inbox', error: 'server rejected' })
  assert.deepEqual(accountStore.getSyncError('a'), { folderId: 'inbox', error: 'server rejected' })
  assert.equal(accountStore.getAccount('a').syncing, false)
  accountStore.clearSyncError('a')
  assert.equal(accountStore.getSyncError('a'), null)

  events.get('sync:progress')({ accountId: 'a', folderId: 'drafts', fetched: 1, total: 2, phase: 'headers' })
  events.get('folder:synced')({ accountId: 'a', folderId: 'drafts' })
  await vi.waitFor(() => assert.equal(backend.GetFolderTree.mock.calls.length >= 3, true))
  assert.equal(accountStore.getSyncProgress('a'), null)
  assert.equal(accountStore.getAccount('a').syncing, false)

  events.get('sync:progress')({ accountId: 'a', folderId: 'inbox', fetched: 1, total: 2, phase: 'headers' })
  events.get('sync:accountFinished')({ accountId: 'a', succeeded: true })
  assert.equal(accountStore.getAccount('a').syncing, false)
  assert.equal(accountStore.getAccount('a').lastCompleteSync instanceof Date, true)
  windowEvents.get('online')()
  assert.equal(accountStore.isOnline, true)
  windowEvents.get('offline')()
  assert.equal(accountStore.isOnline, false)
  events.get('network:online')()
  assert.equal(accountStore.isOnline, true)
  events.get('network:offline')()
  assert.equal(accountStore.isOnline, false)

  backend.SyncAccountComplete.mockResolvedValue(undefined)
  await accountStore.syncAccount('a')
  assert.equal(accountStore.getAccount('a').lastCompleteSync instanceof Date, true)
  assert.equal(accountStore.getAccount('a').syncing, true)
  await accountStore.syncAccount('missing')
  backend.SyncAccountComplete.mockRejectedValueOnce(new Error('single sync failed'))
  await assert.rejects(accountStore.syncAccount('a'), /single sync failed/)
  assert.equal(accountStore.getAccount('a').syncing, false)
  assert.equal(accountStore.getAccount('a').error, 'single sync failed')

  backend.SyncAllComplete.mockResolvedValueOnce(undefined)
  await accountStore.syncAllComplete()
  assert.equal(accountStore.accounts.every((item) => item.lastCompleteSync instanceof Date), true)
  const oldest = new Date('2026-01-01T00:00:00Z')
  const newer = new Date('2026-01-02T00:00:00Z')
  accountStore.accounts[0].lastCompleteSync = newer
  accountStore.accounts[1].lastCompleteSync = oldest
  assert.equal(accountStore.lastCompleteSyncTime.toISOString(), oldest.toISOString())
  accountStore.accounts[1].lastCompleteSync = null
  assert.equal(accountStore.lastCompleteSyncTime, null)

  backend.SyncAllComplete.mockRejectedValueOnce(new Error('sync errors: b@example.com: unavailable'))
  await assert.rejects(accountStore.syncAllComplete(), /unavailable/)
  assert.equal(accountStore.getAccount('b').syncing, false)
  assert.match(accountStore.getAccount('b').error, /unavailable/)
  await accountStore.cancelAllSyncs()
  assert.equal(accountStore.accounts.every((item) => !item.syncing), true)

  const accountC = { id: 'c', email: 'c@example.com', name: 'Account C', enabled: true }
  backend.AddAccount.mockResolvedValue(accountC)
  backend.SyncAccountComplete.mockResolvedValue(undefined)
  backend.GetFolderTree.mockResolvedValue([])
  assert.equal(await accountStore.addAccount({ email: accountC.email }), accountC)
  await vi.waitFor(() => assert.equal(backend.SyncAccountComplete.mock.calls.at(-1)[0], 'c'))
  assert.equal(accountStore.getAccount('c').account, accountC)

  const updatedC = { ...accountC, name: 'Updated C' }
  backend.UpdateAccount.mockResolvedValue(updatedC)
  assert.equal(await accountStore.updateAccount('c', { email: accountC.email }), updatedC)
  assert.equal(accountStore.getAccount('c').account.name, 'Updated C')

  accountStore.selectFolder('c', 'inbox', 'Inbox', 'Inbox')
  await accountStore.removeAccount('c')
  assert.equal(accountStore.getAccount('c'), undefined)
  assert.equal(accountStore.selectedFolder, null)

  backend.TestConnection.mockResolvedValue({ success: true })
  backend.TestAccountConnection.mockResolvedValue({ success: false, error: 'no connection' })
  backend.GetAccountConnOK.mockResolvedValue('2026-08-01T00:00:00Z')
  assert.deepEqual(await accountStore.testConnection({ email: 'test@example.com' }), { success: true })
  assert.deepEqual(await accountStore.testAccountConnection('a'), { success: false, error: 'no connection' })
  assert.equal(await accountStore.getAccountConnOK('a'), '2026-08-01T00:00:00Z')

  backend.GetAccounts.mockResolvedValue([accountB, accountA])
  await accountStore.reorderAccounts(['b', 'a'])
  assert.deepEqual(backend.ReorderAccounts.mock.calls[0], [['b', 'a']])
  assert.deepEqual(accountStore.accounts.map((item) => item.account.id), ['b', 'a'])

  backend.GetAccounts.mockRejectedValueOnce(new Error('accounts unavailable'))
  await accountStore.load()
  assert.equal(accountStore.loading, false)
  assert.equal(accountStore.error, 'accounts unavailable')
  log.mockRestore()
  vi.unstubAllGlobals()
})
