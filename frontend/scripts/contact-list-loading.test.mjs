import assert from 'node:assert/strict'
import { test } from 'vitest'
import { shouldBlockContactList } from '../src/lib/contacts/utils/contactLoadLifecycle.ts'

test('background refresh only blocks an empty contact list', () => {
  assert.equal(shouldBlockContactList(true, 0), true)
  assert.equal(shouldBlockContactList(true, 58), false)
  assert.equal(shouldBlockContactList(false, 0), false)
})
