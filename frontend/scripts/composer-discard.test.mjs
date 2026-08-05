import assert from 'node:assert/strict'
import { test } from 'vitest'
import { discardDraftBeforeClose } from '../src/lib/components/composer/composerDiscard.ts'

test('discard closes only after the draft deletion succeeds', async () => {
  const calls = []

  await discardDraftBeforeClose(
    'draft-1',
    async id => { calls.push(`delete:${id}`) },
    () => { calls.push('close') },
  )

  assert.deepEqual(calls, ['delete:draft-1', 'close'])
})

test('discard keeps the composer open when draft deletion fails', async () => {
  let closed = false

  await assert.rejects(
    discardDraftBeforeClose(
      'draft-1',
      async () => { throw new Error('delete failed') },
      () => { closed = true },
    ),
    /delete failed/,
  )

  assert.equal(closed, false)
})

test('discard closes an unsaved composer without calling delete', async () => {
  let deleteCalled = false
  let closed = false

  await discardDraftBeforeClose(
    null,
    async () => { deleteCalled = true },
    () => { closed = true },
  )

  assert.equal(deleteCalled, false)
  assert.equal(closed, true)
})
