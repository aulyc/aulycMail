import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  backendAttachmentToComposerAttachment,
  estimateBase64DecodedSize,
  hasFileDropPayload,
  isRecipientChipDrag,
} from '../src/lib/components/composer/composerAttachments.ts'
import {
  composerHasContent,
  formatIdentityLabel,
  getComposerDisplayMode,
  widenComposerLabel,
} from '../src/lib/components/composer/composerDisplay.ts'
import {
  buildDraftContentHash,
  getDraftStatusMeta,
} from '../src/lib/components/composer/composerDraft.ts'
import {
  findPlainQuoteBoundary,
  findRichQuoteBoundary,
  hasPlainQuoteBoundary,
} from '../src/lib/components/composer/composerQuoteBoundaries.ts'
import {
  createMultiRowContextMenu,
  createSingleRowContextMenu,
  getSelectedMessageIds,
  hasSelectedUnread,
  hasSelectedUnstarred,
  toggleSetEntry,
} from '../src/lib/components/list/messageListSelection.ts'
import {
  getMessageListRowHeight,
  getVirtualWindow,
} from '../src/lib/components/list/messageListVirtual.ts'
import { getAccountColor } from '../src/lib/utils/accountColor.ts'
import { isEmailAddress, parseEmailAddress } from '../src/lib/utils/email.ts'
import { formatFileSize } from '../src/lib/utils/fileSize.ts'
import {
  dialogGuardClose,
  dialogGuardOpen,
  isDialogGuardActive,
  onDialogGuardChange,
} from '../src/lib/stores/dialogGuard.ts'
import { getCached, setCache } from '../src/lib/stores/inlineAttachmentCache.ts'

test('message selection deduplicates IDs and derives aggregate state', () => {
  const conversations = [
    { threadId: 'a', messageIds: ['m1', 'm2'], isStarred: true, unreadCount: 0 },
    { threadId: 'b', messages: [{ id: 'm2' }, { id: 'm3' }], isStarred: false, unreadCount: 2 },
    { messageIds: ['ignored'], isStarred: false, unreadCount: 1 },
  ]
  const selected = new Set(['a', 'b'])

  assert.deepEqual(getSelectedMessageIds(conversations, selected), ['m1', 'm2', 'm3'])
  assert.equal(hasSelectedUnstarred(conversations, selected), true)
  assert.equal(hasSelectedUnread(conversations, selected), true)
  assert.equal(hasSelectedUnread(conversations, new Set(['a'])), false)
})

test('selection toggles and context menus preserve row semantics', () => {
  const selected = new Set(['a'])
  toggleSetEntry(selected, 'a')
  toggleSetEntry(selected, 'b')
  assert.deepEqual([...selected], ['b'])

  assert.deepEqual(createSingleRowContextMenu({
    conversation: { messages: [{ id: 'm1' }], unreadCount: 1 },
    accountId: 'account',
    folderId: 'inbox',
    folderType: 'inbox',
  }), {
    messageIds: ['m1'],
    accountId: 'account',
    folderId: 'inbox',
    folderType: 'inbox',
    isStarred: false,
    isRead: false,
    allowReply: true,
  })

  assert.deepEqual(createMultiRowContextMenu({
    messageIds: ['m1', 'm2'],
    accountId: 'account',
    folderId: 'inbox',
    folderType: 'inbox',
    hasUnstarred: false,
    hasUnread: true,
  }), {
    messageIds: ['m1', 'm2'],
    accountId: 'account',
    folderId: 'inbox',
    folderType: 'inbox',
    isStarred: true,
    isRead: false,
    allowReply: false,
  })
})

test('virtual message windows clamp empty, middle, and tail ranges', () => {
  assert.equal(getMessageListRowHeight('micro'), 66)
  assert.equal(getMessageListRowHeight('large'), 120)
  assert.equal(getMessageListRowHeight('unknown'), 94)
  assert.deepEqual(getVirtualWindow([], 400, 0, 80), { rows: [], topHeight: 0, bottomHeight: 0 })

  const items = Array.from({ length: 100 }, (_, index) => `row-${index}`)
  const middle = getVirtualWindow(items, 200, 1000, 20)
  assert.equal(middle.rows[0].index, 42)
  assert.equal(middle.rows.at(-1).index, 67)
  assert.equal(middle.topHeight, 840)
  assert.equal(middle.bottomHeight, 640)

  const tail = getVirtualWindow(items, 200, 99999, 20)
  assert.equal(tail.rows[0].index, 74)
  assert.equal(tail.rows.at(-1).index, 99)
  assert.equal(tail.bottomHeight, 0)
})

test('composer display helpers classify modes and user-facing labels', () => {
  assert.equal(widenComposerLabel('回复'), '回　复')
  assert.equal(widenComposerLabel('Reply'), 'Reply')
  assert.equal(formatIdentityLabel(null), '')
  assert.equal(formatIdentityLabel({ name: 'Me', email: 'me@example.com' }), 'Me <me@example.com>')
  assert.equal(formatIdentityLabel({ name: 'ME@EXAMPLE.COM', email: 'me@example.com' }), 'me@example.com')

  assert.equal(getComposerDisplayMode({ replyType: '' }), 'new')
  assert.equal(getComposerDisplayMode({ initialMessage: { subject: 'Hello' }, replyType: 'forward' }), 'forward')
  assert.equal(getComposerDisplayMode({ initialMessage: { subject: 'Fwd: Hello' }, replyType: '' }), 'forward')
  assert.equal(getComposerDisplayMode({ initialMessage: { in_reply_to: 'id', to: [{}, {}] }, replyType: '' }), 'reply-all')
  assert.equal(getComposerDisplayMode({ initialMessage: { in_reply_to: 'id', to: [{}] }, replyType: '' }), 'reply')
  assert.equal(getComposerDisplayMode({ initialMessage: { subject: 'Draft' }, replyType: '' }), 'new')
})

test('composer content detection covers recipients, body, subject, and attachments', () => {
  const empty = {
    toCount: 0,
    ccCount: 0,
    bccCount: 0,
    subject: ' ',
    isPlainTextMode: true,
    plainTextContent: ' ',
    editor: null,
    attachments: [],
  }
  assert.equal(composerHasContent(empty), false)
  assert.equal(composerHasContent({ ...empty, toCount: 1 }), true)
  assert.equal(composerHasContent({ ...empty, subject: 'Subject' }), true)
  assert.equal(composerHasContent({ ...empty, plainTextContent: 'Body' }), true)
  assert.equal(composerHasContent({ ...empty, isPlainTextMode: false, editor: { getText: () => 'Rich body' } }), true)
  assert.equal(composerHasContent({ ...empty, attachments: [{}] }), true)
})

test('draft status and content hashing represent persistence state', () => {
  assert.deepEqual(getDraftStatusMeta('saving', 'pending', false), {
    icon: 'mdi:loading', color: '', labelKey: 'composer.saving',
  })
  assert.equal(getDraftStatusMeta('error', 'failed', true).labelKey, 'composer.saveFailed')
  assert.deepEqual(getDraftStatusMeta('idle', 'pending', false), { icon: '', color: '', labelKey: '' })
  assert.equal(getDraftStatusMeta('saved', 'synced', true).labelKey, 'composer.synced')
  assert.equal(getDraftStatusMeta('saved', 'pending', true).labelKey, 'composer.savedLocally')
  assert.equal(getDraftStatusMeta('saved', 'failed', true).labelKey, 'composer.savedLocallyOffline')

  const input = {
    toCount: 1,
    ccCount: 0,
    bccCount: 0,
    subject: 'subject',
    bodyContent: 'body',
    attachmentNames: 'a.pdf',
    isPlainTextMode: false,
  }
  assert.equal(buildDraftContentHash(input), '1|0|0|subject|body|a.pdf|false')
  assert.notEqual(buildDraftContentHash(input), buildDraftContentHash({ ...input, subject: 'changed' }))
})

test('quote boundary detection chooses the earliest supported marker', () => {
  const plain = 'Intro\nAlice wrote:\nquoted\n---------- Forwarded message ----------'
  assert.equal(findPlainQuoteBoundary(plain), 6)
  assert.equal(hasPlainQuoteBoundary(plain), true)
  assert.equal(findPlainQuoteBoundary('No quote'), -1)
  assert.equal(findRichQuoteBoundary('<p>Intro</p><blockquote>quoted</blockquote>'), 12)
  assert.equal(findRichQuoteBoundary('<p>A wrote:</p><blockquote>quoted</blockquote>'), 5)
  assert.equal(findRichQuoteBoundary('<p>plain</p>'), -1)
})

test('attachment helpers preserve metadata and recognize drag payloads', () => {
  const attachment = { filename: 'a.pdf', contentType: 'application/pdf', size: 3, data: 'YWJj' }
  assert.deepEqual(backendAttachmentToComposerAttachment(attachment), attachment)
  assert.equal(estimateBase64DecodedSize('YWJj'), 3)
  assert.equal(estimateBase64DecodedSize('YQ=='), 1)
  assert.equal(isRecipientChipDrag({ dataTransfer: { types: ['application/x-aulycmail-recipient'] } }), true)
  assert.equal(isRecipientChipDrag({}), false)
  assert.equal(hasFileDropPayload({ dataTransfer: { types: ['Files'] } }), true)
  assert.equal(hasFileDropPayload({ dataTransfer: { types: ['text/uri-list'] } }), true)
  assert.equal(hasFileDropPayload({ dataTransfer: { types: ['text/plain'] } }), false)
})

test('email, file-size, and account-color helpers handle valid and invalid boundaries', () => {
  assert.equal(isEmailAddress(' user+tag@example.com '), true)
  assert.equal(isEmailAddress('invalid@example'), false)
  assert.deepEqual(parseEmailAddress('Alice <ALICE@example.com>'), { name: 'Alice', email: 'alice@example.com' })
  assert.deepEqual(parseEmailAddress('person@example.com'), { name: '', email: 'person@example.com' })
  assert.equal(parseEmailAddress('not an address'), null)

  assert.equal(formatFileSize(Number.NaN), '')
  assert.equal(formatFileSize(-1), '')
  assert.equal(formatFileSize(0), '0 B')
  assert.equal(formatFileSize(1023), '1023 B')
  assert.equal(formatFileSize(1536), '1.5 KB')
  assert.equal(formatFileSize(10 * 1024), '10 KB')
  assert.equal(formatFileSize(1024 ** 4), '1024 GB')

  assert.equal(getAccountColor({ color: '#123456', orderIndex: 3 }), '#123456')
  assert.equal(getAccountColor(null), '#3B82F6')
  assert.equal(getAccountColor({ orderIndex: -10 }), '#3B82F6')
  assert.equal(getAccountColor({ orderIndex: 8 }), '#3B82F6')
})

test('dialog guard only notifies when active state crosses zero', () => {
  while (isDialogGuardActive()) dialogGuardClose()
  const states = []
  const unsubscribe = onDialogGuardChange((active) => states.push(active))

  dialogGuardOpen()
  dialogGuardOpen()
  dialogGuardClose()
  dialogGuardClose()
  dialogGuardClose()
  unsubscribe()
  dialogGuardOpen()
  dialogGuardClose()

  assert.deepEqual(states, [true, false])
  assert.equal(isDialogGuardActive(), false)
})

test('inline attachment cache distinguishes missing and cached empty data', () => {
  assert.equal(getCached('missing'), null)
  setCache('message-1', {})
  setCache('message-2', { cid: 'data:image/png;base64,AA==' })
  assert.deepEqual(getCached('message-1'), {})
  assert.deepEqual(getCached('message-2'), { cid: 'data:image/png;base64,AA==' })
})
