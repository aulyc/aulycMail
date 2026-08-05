import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  clampMentionPosition,
  MENTION_MENU_HEIGHT,
  MENTION_VISIBLE_ROWS,
} from '../src/lib/components/composer/composerMentionLayout.ts'
import {
  filteredMentionResults,
  findMentionToken,
  getContactEmail,
  getMentionLabel,
  getPlainTextMentionSegments,
  hasRecipient,
  mentionKey,
} from '../src/lib/components/composer/composerMentions.ts'
import {
  buildSignatureHtml,
  getSignatureSeparator,
  hasSignatureMarker,
  insertSignatureIntoContent,
  removeSignatureFromContent,
  shouldAppendSignature,
} from '../src/lib/components/composer/composerSignature.ts'

const marker = '\u200B\u200B\u200B'

function identity(overrides = {}) {
  return {
    signatureEnabled: true,
    signatureHtml: '<p>Regards</p>',
    signatureText: '',
    signatureSeparator: false,
    signatureSeparatorStyle: '',
    signatureForNew: true,
    signatureForReply: false,
    signatureForForward: false,
    ...overrides,
  }
}

test('signature HTML handles disabled, text fallback, separators, and marker placement', () => {
  assert.equal(buildSignatureHtml(identity({ signatureEnabled: false })), '')
  assert.equal(buildSignatureHtml(identity({ signatureHtml: '', signatureText: '' })), '')
  assert.equal(
    buildSignatureHtml(identity({ signatureHtml: '', signatureText: 'A&B\n<C>' })),
    `<p>${marker}A&amp;B<br>&lt;C&gt;</p>`,
  )
  assert.equal(buildSignatureHtml(identity()), `<p>${marker}Regards</p>`)
  assert.equal(
    buildSignatureHtml(identity({ signatureHtml: '<div>Regards</div>' })),
    `<p>${marker}</p><div>Regards</div>`,
  )
  assert.equal(
    buildSignatureHtml(identity({ signatureSeparator: true })),
    `<p>${marker}-----</p><p>Regards</p>`,
  )
  assert.equal(getSignatureSeparator(identity({ signatureSeparatorStyle: '*****' })), '*****')
  assert.equal(getSignatureSeparator(identity({ signatureSeparatorStyle: 'custom', signatureSeparator: false })), '')
})

test('signature mode preferences are evaluated independently', () => {
  const configured = identity({ signatureForNew: true, signatureForReply: true, signatureForForward: false })
  assert.equal(shouldAppendSignature(configured, 'new'), true)
  assert.equal(shouldAppendSignature(configured, 'reply'), true)
  assert.equal(shouldAppendSignature(configured, 'reply-all'), true)
  assert.equal(shouldAppendSignature(configured, 'forward'), false)
  assert.equal(shouldAppendSignature(identity({ signatureEnabled: false }), 'new'), false)
})

test('signature insertion respects reply, forward, blockquote, empty, and append placements', () => {
  const signature = `<p>${marker}Regards</p>`
  assert.equal(
    insertSignatureIntoContent('<p>Alice wrote:</p><p>quoted</p>', signature, 'reply'),
    `<p></p><p></p>${signature}<p></p><p></p><p>Alice wrote:</p><p>quoted</p>`,
  )
  assert.equal(
    insertSignatureIntoContent('<p>---------- Forwarded message ----------</p>', signature, 'forward'),
    `<p></p><p></p>${signature}<p></p><p></p><p>---------- Forwarded message ----------</p>`,
  )
  assert.equal(
    insertSignatureIntoContent('<blockquote>quoted</blockquote>', signature, 'reply'),
    `<p></p><p></p>${signature}<p></p><p></p><blockquote>quoted</blockquote>`,
  )
  assert.equal(insertSignatureIntoContent('<p></p>', signature, 'new'), `<p></p><p></p>${signature}`)
  assert.equal(insertSignatureIntoContent('<p>Body</p>', signature, 'new'), `<p>Body</p><p></p>${signature}`)
  assert.equal(insertSignatureIntoContent('<p>Body</p>', signature, 'reply', 'below'), `<p>Body</p><p></p>${signature}`)
})

test('signature removal preserves quoted content and handles malformed markup', () => {
  const signature = `<p>${marker}Regards</p>`
  assert.equal(removeSignatureFromContent('<p>No signature</p>'), '<p>No signature</p>')
  assert.equal(removeSignatureFromContent(`prefix${marker}tail`), 'prefix')
  assert.equal(removeSignatureFromContent(`<p>Body</p><p></p>${signature}`), '<p>Body</p>')
  assert.equal(
    removeSignatureFromContent(`<p>Body</p>${signature}<p>Alice wrote:</p><blockquote>quoted</blockquote>`),
    '<p>Body</p><p>Alice wrote:</p><blockquote>quoted</blockquote>',
  )
  assert.equal(hasSignatureMarker(signature), true)
  assert.equal(hasSignatureMarker('<p>None</p>'), false)
})

test('mention identity helpers normalize contacts and recipients', () => {
  assert.equal(getMentionLabel({ display_name: ' Alice ', email: 'alice@example.com' }), 'Alice')
  assert.equal(getMentionLabel({ display_name: '', email: ' alice@example.com ' }), 'alice@example.com')
  assert.equal(getContactEmail({ email: ' ALICE@example.com ' }), 'alice@example.com')
  assert.equal(hasRecipient('Alice@example.com', [{ address: ' alice@example.com ' }]), true)
  assert.equal(hasRecipient('bob@example.com', [{ email: 'BOB@example.com' }]), true)
  assert.equal(hasRecipient(' ', [{ email: 'bob@example.com' }]), false)
  assert.equal(mentionKey('plain', 'ali', 2, 6), 'plain:2:6:ali')
})

test('mention token detection enforces boundaries and maximum query length', () => {
  assert.deepEqual(findMentionToken('@ali'), { query: 'ali', startOffset: 0 })
  assert.deepEqual(findMentionToken('hello (@张'), { query: '张', startOffset: 7 })
  assert.equal(findMentionToken('mail@example.com'), null)
  assert.equal(findMentionToken(`@${'a'.repeat(41)}`), null)
})

test('mention result filtering matches labels and emails while removing duplicates', () => {
  const contacts = [
    { display_name: 'Alice', email: 'alice@example.com' },
    { display_name: 'Alice duplicate', email: 'ALICE@example.com' },
    { display_name: 'Bob', email: 'bob@example.com' },
    { display_name: '', email: '' },
  ]
  assert.deepEqual(filteredMentionResults(contacts, ''), [])
  assert.deepEqual(filteredMentionResults(contacts, 'ali'), [contacts[0]])
  assert.deepEqual(filteredMentionResults(contacts, 'bob@'), [contacts[2]])
})

test('plain-text mention segmentation prefers selected labels and falls back safely', () => {
  assert.deepEqual(getPlainTextMentionSegments('', ['Alice']), [])
  assert.deepEqual(getPlainTextMentionSegments('Hi @Alice and @Bob!', ['Alice']), [
    { type: 'text', text: 'Hi ' },
    { type: 'mention', text: '@Alice' },
    { type: 'text', text: ' and ' },
    { type: 'mention', text: '@Bob!' },
  ])
  assert.deepEqual(getPlainTextMentionSegments('mail@example.com @A', ['Alice', 'A']), [
    { type: 'text', text: 'mail@example.com ' },
    { type: 'mention', text: '@A' },
  ])
})

test('mention popup positioning clamps horizontally and flips above near the bottom', () => {
  assert.equal(MENTION_VISIBLE_ROWS, 4)
  assert.equal(MENTION_MENU_HEIGHT, 210)
  assert.deepEqual(clampMentionPosition({ left: -10, top: 20, container: null }), { left: -10, top: 20 })

  const container = { scrollTop: 0, clientHeight: 500, clientWidth: 400 }
  assert.deepEqual(clampMentionPosition({ left: 200, top: 100, container }), { left: 104, top: 106 })
  assert.deepEqual(clampMentionPosition({ left: 20, top: 450, container }), { left: 20, top: 234 })

  const scrolled = { scrollTop: 100, clientHeight: 200, clientWidth: 200 }
  assert.deepEqual(clampMentionPosition({ left: -20, top: 105, container: scrolled }), { left: 8, top: 108 })
})
