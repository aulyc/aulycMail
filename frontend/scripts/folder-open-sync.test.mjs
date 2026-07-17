import assert from 'node:assert/strict'
import test from 'node:test'
import {
  FOLDER_OPEN_SYNC_RETRY_COOLDOWN_MS,
  findFolderSyncDescriptor,
  shouldAutoSyncFolderOnOpen,
} from '../src/lib/components/list/folderOpenSync.ts'

const baseInput = {
  accountId: 'account-1',
  folderId: 'junk-1',
  folderType: 'spam',
  folderSubscribed: true,
  syncAllFolders: false,
  syncFoldersEnabled: false,
  isUnifiedView: false,
  isOnline: true,
  isSyncing: false,
  lastAttemptAt: null,
  now: Date.parse('2026-07-17T00:00:00Z'),
}

test('syncs an opened folder that the default background policy excludes', () => {
  assert.equal(shouldAutoSyncFolderOnOpen(baseInput), true)
})

test('does not duplicate background sync coverage', () => {
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, folderType: 'inbox' }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({
    ...baseInput,
    folderType: 'inbox',
    syncFoldersEnabled: true,
    folderSubscribed: false,
  }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, syncAllFolders: true }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, syncFoldersEnabled: true }), false)
})

test('still syncs an opened folder that is excluded from subscription-based sync', () => {
  assert.equal(shouldAutoSyncFolderOnOpen({
    ...baseInput,
    syncFoldersEnabled: true,
    folderSubscribed: false,
  }), true)
})

test('skips automatic sync when offline, already syncing, or using unified inbox', () => {
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, isOnline: false }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, isSyncing: true }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, isUnifiedView: true }), false)
})

test('throttles repeated open attempts without blocking a later retry', () => {
  const lastAttemptAt = baseInput.now - FOLDER_OPEN_SYNC_RETRY_COOLDOWN_MS + 1
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, lastAttemptAt }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({
    ...baseInput,
    lastAttemptAt: baseInput.now - FOLDER_OPEN_SYNC_RETRY_COOLDOWN_MS,
  }), true)
})

test('requires a concrete account and selectable folder description', () => {
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, accountId: null }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, folderId: null }), false)
  assert.equal(shouldAutoSyncFolderOnOpen({ ...baseInput, folderType: null }), false)
})

test('finds deeply nested folders without assuming a fixed tree depth', () => {
  const trees = [{
    children: [{
      children: [{
        folder: { id: 'junk-1', type: 'spam', subscribed: true },
      }],
    }],
  }]

  assert.deepEqual(findFolderSyncDescriptor(trees, 'junk-1'), {
    type: 'spam',
    subscribed: true,
  })
  assert.equal(findFolderSyncDescriptor(trees, 'missing'), null)
})
