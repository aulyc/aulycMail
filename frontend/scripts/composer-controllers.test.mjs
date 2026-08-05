import assert from 'node:assert/strict'
import { test, vi } from 'vitest'

import { createDraftSaveController } from '../src/lib/components/composer/composerDraftController.ts'
import {
  clampMentionSelection,
  createMentionSearchController,
  moveMentionPointerSelection,
  selectMentionIndex,
} from '../src/lib/components/composer/composerMentionController.ts'
import {
  registerInlineImage,
  replaceInlineImageSourcesWithCids,
} from '../src/lib/components/composer/composerInlinePipeline.ts'
import { getComposerSendBlocker } from '../src/lib/components/composer/composerSendController.ts'

test('mention selection controller keeps the active row visible and ignores stationary pointer events', () => {
  const initial = { selectedIndex: 0, windowStart: 0, keyboardMode: false, pointerX: -1, pointerY: -1 }
  const keyboard = selectMentionIndex(initial, 6, 5, 'keyboard', 4)
  assert.deepEqual(keyboard, { ...initial, selectedIndex: 5, windowStart: 2, keyboardMode: true })

  const pointer = moveMentionPointerSelection(keyboard, 6, 3, 20, 30, 4)
  assert.equal(pointer.changed, true)
  assert.equal(pointer.state.selectedIndex, 3)
  assert.equal(pointer.state.keyboardMode, false)
  const stationary = moveMentionPointerSelection(pointer.state, 6, 4, 20, 30, 4)
  assert.equal(stationary.changed, false)
  assert.deepEqual(stationary.state, pointer.state)

  assert.deepEqual(clampMentionSelection({ ...keyboard, selectedIndex: 5, windowStart: 2 }, 2, 4), {
    ...keyboard,
    selectedIndex: 1,
    windowStart: 0,
  })
})

test('mention search controller debounces requests and ignores stale responses', async () => {
  vi.useFakeTimers()
  const pending = new Map()
  const published = []
  const controller = createMentionSearchController({
    delayMs: 50,
    search: vi.fn((query) => new Promise((resolve) => pending.set(query, resolve))),
    onResults: (query, results) => published.push([query, results]),
  })

  controller.schedule('a')
  controller.schedule('alice')
  await vi.advanceTimersByTimeAsync(50)
  assert.equal(pending.has('a'), false)
  pending.get('alice')([{ email: 'alice@example.test' }])
  await Promise.resolve()
  assert.deepEqual(published, [['alice', [{ email: 'alice@example.test' }]]])

  controller.schedule('old')
  await vi.advanceTimersByTimeAsync(50)
  controller.schedule('new')
  await vi.advanceTimersByTimeAsync(50)
  pending.get('new')([{ email: 'new@example.test' }])
  await Promise.resolve()
  pending.get('old')([{ email: 'old@example.test' }])
  await Promise.resolve()
  assert.deepEqual(published.at(-1), ['new', [{ email: 'new@example.test' }]])
  controller.destroy()
  vi.useRealTimers()
})

test('draft save controller debounces, de-duplicates, waits for in-flight work, and suppresses saves while discarding', async () => {
  vi.useFakeTimers()
  let hash = 'first'
  let hasDraft = false
  const statuses = []
  const save = vi.fn(async () => { hasDraft = true })
  const controller = createDraftSaveController({
    delayMs: 100,
    hasContent: () => true,
    getContentHash: () => hash,
    hasPersistedDraft: () => hasDraft,
    getStatus: () => statuses.at(-1) || 'idle',
    setStatus: (status) => statuses.push(status),
    save,
  })

  controller.schedule()
  controller.schedule()
  await vi.advanceTimersByTimeAsync(99)
  assert.equal(save.mock.calls.length, 0)
  await vi.advanceTimersByTimeAsync(1)
  assert.equal(save.mock.calls.length, 1)
  assert.deepEqual(statuses.slice(-2), ['saving', 'saved'])

  controller.schedule()
  await vi.advanceTimersByTimeAsync(100)
  assert.equal(save.mock.calls.length, 1)

  hash = 'second'
  controller.setDiscarding(true)
  assert.equal(await controller.saveNow(), false)
  controller.setDiscarding(false)
  assert.equal(await controller.saveNow(), true)
  await controller.waitForIdle()
  assert.equal(save.mock.calls.length, 2)
  controller.destroy()
  vi.useRealTimers()
})

test('inline image pipeline de-duplicates content and rewrites every registered source to CID', () => {
  const first = { cid: 'one@example', dataUrl: 'data:image/png;base64,AA==', contentType: 'image/png', data: 'AA==', filename: 'one.png' }
  const duplicate = { ...first, cid: 'duplicate@example', filename: 'duplicate.png' }
  const second = { cid: 'two@example', dataUrl: 'data:image/png;base64,BB==', contentType: 'image/png', data: 'BB==', filename: 'two.png' }

  const initial = registerInlineImage([], first)
  assert.equal(initial.added, true)
  const reused = registerInlineImage(initial.images, duplicate)
  assert.equal(reused.added, false)
  assert.equal(reused.image.cid, first.cid)
  const completed = registerInlineImage(reused.images, second)
  assert.equal(completed.images.length, 2)
  assert.equal(
    replaceInlineImageSourcesWithCids(`<img src="${first.dataUrl}"><img src="${second.dataUrl}">`, completed.images),
    '<img src="cid:one@example"><img src="cid:two@example">',
  )
})

test('send controller preserves user-facing validation order', () => {
  const valid = { recipientCount: 1, hasIdentity: true, attachmentCount: 1, mentionsAttachment: false, subject: 'Hello' }
  assert.equal(getComposerSendBlocker({ ...valid, recipientCount: 0 }), 'no-recipients')
  assert.equal(getComposerSendBlocker({ ...valid, hasIdentity: false }), 'missing-identity')
  assert.equal(getComposerSendBlocker({ ...valid, attachmentCount: 0, mentionsAttachment: true, subject: '' }), 'missing-attachment')
  assert.equal(getComposerSendBlocker({ ...valid, subject: '   ' }), 'empty-subject')
  assert.equal(getComposerSendBlocker(valid), null)
})
