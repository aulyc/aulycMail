import assert from 'node:assert/strict'
import test from 'node:test'
import { shouldClearRestoredFolderSelection } from '../src/lib/stores/restoredFolderSelection.ts'

test('waits for account folders before validating restored folder state', () => {
  assert.equal(shouldClearRestoredFolderSelection(true, false, null), false)
})

test('clears missing and hierarchy-only restored folders after loading', () => {
  assert.equal(shouldClearRestoredFolderSelection(false, true, null), true)
  assert.equal(shouldClearRestoredFolderSelection(true, true, null), true)
  assert.equal(shouldClearRestoredFolderSelection(true, true, { noSelect: true }), true)
})

test('keeps a selectable restored folder', () => {
  assert.equal(shouldClearRestoredFolderSelection(true, true, { noSelect: false }), false)
})
