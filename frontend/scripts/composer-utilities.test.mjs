import assert from 'node:assert/strict'
import { test, vi } from 'vitest'

import {
  fileToComposerAttachment,
  fileToDataUrl,
  selectedFileToComposerAttachment,
} from '../src/lib/components/composer/composerAttachments.ts'
import { handleComposerTabNavigation } from '../src/lib/components/composer/composerFocus.ts'
import {
  createInlineImageCID,
  createInlineImageFromAttachment,
  createInlineImageFromDataUrl,
} from '../src/lib/components/composer/composerInlineImages.ts'
import {
  INLINE_IMAGE_ATTACHMENT_SIZE,
  INLINE_IMAGE_WARNING_SIZE,
  base64DecodedSize,
  evaluateInlineImageBatch,
  formatInlineImageSize,
} from '../src/lib/components/composer/composerInlineImagePolicy.ts'
import {
  buildComposeMessage,
  restoreBlockedRemoteImages,
  toSmtpAddress,
} from '../src/lib/components/composer/composerMessage.ts'
import {
  addParagraphStyles,
  getFileIcon,
  parseFileUris,
  plainTextToHtml,
  readFileAsBase64,
  readFileAsDataUrl,
  stripParagraphStyles,
  textMentionsAttachment,
} from '../src/lib/components/composer/composerUtils.ts'

test('composer file icons cover known and fallback MIME families', () => {
  assert.equal(getFileIcon('image/png'), 'mdi:file-image')
  assert.equal(getFileIcon('video/mp4'), 'mdi:file-video')
  assert.equal(getFileIcon('audio/mpeg'), 'mdi:file-music')
  assert.equal(getFileIcon('application/pdf'), 'mdi:file-pdf-box')
  assert.equal(getFileIcon('application/vnd.ms-excel'), 'mdi:file-excel')
  assert.equal(getFileIcon('application/msword'), 'mdi:file-word')
  assert.equal(getFileIcon('application/vnd.ms-powerpoint'), 'mdi:file-powerpoint')
  assert.equal(getFileIcon('application/zip'), 'mdi:folder-zip')
  assert.equal(getFileIcon('text/plain'), 'mdi:file-document')
  assert.equal(getFileIcon('application/octet-stream'), 'mdi:file')
})

test('plain-text and paragraph conversions escape content and round-trip editor spacing', () => {
  assert.equal(plainTextToHtml('A&B <C>'), '<p>A&amp;B &lt;C&gt;</p>')
  assert.equal(plainTextToHtml('first\nsecond'), '<p>first<br>second</p>')
  assert.equal(plainTextToHtml('first\n\nsecond'), '<p>first</p><p>second</p>')

  assert.equal(
    addParagraphStyles('<p>One</p><p></p><p style="color:red">Two</p>'),
    '<div style="line-height:1.25"><p style="margin:0">One</p><br><p style="margin:0;color:red">Two</p></div>',
  )
  assert.equal(
    stripParagraphStyles('<div style="line-height:1.25"><p style="margin:0">One</p><br><p style="margin:0;color:red">Two</p></div>'),
    '<p>One</p><p></p><p style="color:red">Two</p>',
  )
})

test('attachment wording uses whole keywords and accepts straight or curly apostrophes', () => {
  assert.equal(textMentionsAttachment('Please see attached.'), true)
  assert.equal(textMentionsAttachment("I've attached the invoice"), true)
  assert.equal(textMentionsAttachment('I’ve attached the invoice'), true)
  assert.equal(textMentionsAttachment('The word reattachment is unrelated'), false)
  assert.equal(textMentionsAttachment('No files mentioned'), false)
})

test('file URI parsing decodes local paths and rejects comments and remote URLs', () => {
  assert.deepEqual(parseFileUris([
    '# drag payload',
    'file:///Users/test/My%20File.pdf',
    '/tmp/report.txt',
    'https://example.com/file.pdf',
    '',
  ].join('\r\n')), ['/Users/test/My File.pdf', '/tmp/report.txt'])
})

test('file readers resolve data URLs and base64 or reject reader failures', async () => {
  class TestFileReader {
    result = null
    error = null
    onload = null
    onerror = null

    readAsDataURL(file) {
      if (file.fail) {
        this.error = new Error('read failed')
        this.onerror?.()
        return
      }
      this.result = file.dataUrl
      this.onload?.()
    }
  }
  vi.stubGlobal('FileReader', TestFileReader)
  try {
    const file = { name: 'image.png', type: 'image/png', size: 3, dataUrl: 'data:image/png;base64,YWJj' }
    assert.equal(await readFileAsDataUrl(file), file.dataUrl)
    assert.equal(await readFileAsBase64(file), 'YWJj')
    assert.equal(await fileToDataUrl(file), file.dataUrl)
    assert.deepEqual(await fileToComposerAttachment(file), {
      filename: 'image.png', contentType: 'image/png', size: 3, data: 'YWJj',
    })
    assert.deepEqual(await selectedFileToComposerAttachment(file), {
      filename: 'image.png', contentType: 'image/png', size: 3, data: 'YWJj',
    })
    assert.deepEqual(await fileToComposerAttachment({ ...file, type: '' }), {
      filename: 'image.png', contentType: 'application/octet-stream', size: 3, data: 'YWJj',
    })
    assert.equal(await selectedFileToComposerAttachment({ ...file, dataUrl: 'invalid' }), null)
    await assert.rejects(readFileAsDataUrl({ fail: true }), /read failed/)
    await assert.rejects(readFileAsBase64({ fail: true }), /read failed/)
  } finally {
    vi.unstubAllGlobals()
  }
})

test('inline image factories validate data URLs and derive stable metadata', () => {
  assert.equal(createInlineImageCID(2, 1234), 'image2-1234@aulycmail')
  assert.equal(createInlineImageFromDataUrl({ cid: 'cid', dataUrl: 'invalid', counter: 1 }), null)
  assert.deepEqual(createInlineImageFromDataUrl({
    cid: 'cid',
    dataUrl: 'data:image/jpeg;base64,YWJj',
    counter: 3,
  }), {
    cid: 'cid',
    dataUrl: 'data:image/jpeg;base64,YWJj',
    contentType: 'image/jpeg',
    data: 'YWJj',
    filename: 'image3.jpeg',
    size: 3,
  })
  assert.equal(createInlineImageFromDataUrl({
    cid: 'cid',
    dataUrl: 'data:custom;base64,YWJj',
    counter: 4,
    fallbackPrefix: 'pasted-',
  })?.filename, 'pasted-4.png')
  assert.equal(createInlineImageFromDataUrl({
    cid: 'cid',
    dataUrl: 'data:image/png;base64,YWJj',
    counter: 1,
    filename: 'chosen.png',
  })?.filename, 'chosen.png')
  assert.deepEqual(createInlineImageFromAttachment({
    cid: 'cid', dataUrl: 'data:image/png;base64,AA==', contentType: 'image/png', data: 'AA==', filename: 'a.png',
  }), {
    cid: 'cid', dataUrl: 'data:image/png;base64,AA==', contentType: 'image/png', data: 'AA==', filename: 'a.png', size: 1,
  })
})

test('inline image policy applies exact 5 MiB and 10 MiB cumulative boundaries', () => {
  assert.equal(INLINE_IMAGE_WARNING_SIZE, 5 * 1024 * 1024)
  assert.equal(INLINE_IMAGE_ATTACHMENT_SIZE, 10 * 1024 * 1024)

  assert.equal(evaluateInlineImageBatch([], [
    { data: 'five', size: INLINE_IMAGE_WARNING_SIZE },
  ]).decision, 'inline')
  assert.equal(evaluateInlineImageBatch([], [
    { data: 'over-five', size: INLINE_IMAGE_WARNING_SIZE + 1 },
  ]).decision, 'confirm')
  assert.equal(evaluateInlineImageBatch([], [
    { data: 'ten', size: INLINE_IMAGE_ATTACHMENT_SIZE },
  ]).decision, 'confirm')
  assert.equal(evaluateInlineImageBatch([], [
    { data: 'over-ten', size: INLINE_IMAGE_ATTACHMENT_SIZE + 1 },
  ]).decision, 'attachment')
})

test('inline image policy counts current and batched unique image bytes once', () => {
  const MiB = 1024 * 1024
  const current = [{ data: 'existing', size: 4 * MiB }]
  const batch = [
    { data: 'new-image', size: 2 * MiB },
    { data: 'new-image', size: 2 * MiB },
    { data: 'existing', size: 4 * MiB },
  ]

  assert.deepEqual(evaluateInlineImageBatch(current, batch), {
    decision: 'confirm',
    currentBytes: 4 * MiB,
    batchBytes: 2 * MiB,
    projectedBytes: 6 * MiB,
  })
  assert.equal(evaluateInlineImageBatch(
    [{ data: 'existing', size: 9 * MiB }],
    [{ data: 'new-image', size: 2 * MiB }],
  ).decision, 'attachment')
})

test('inline image policy derives decoded bytes accurately from padded base64', () => {
  assert.equal(base64DecodedSize(''), 0)
  assert.equal(base64DecodedSize('YQ=='), 1)
  assert.equal(base64DecodedSize('YWI='), 2)
  assert.equal(base64DecodedSize('YWJj'), 3)
  assert.equal(base64DecodedSize('YWJj\r\n'), 3)
  assert.equal(formatInlineImageSize(6 * 1024 * 1024), '6.0 MB')
})

test('composer message assembly restores remote images and serializes attachment payloads', () => {
  const restored = restoreBlockedRemoteImages(
    '<p><img src="data:image/svg+xml,placeholder" data-original-src="https://img.example.test/a.png"></p>',
  )
  assert.equal(restored, '<p><img src="https://img.example.test/a.png"></p>')

  const fromEmail = toSmtpAddress({ name: 'Ada', email: 'ada@example.test' })
  assert.equal(fromEmail.name, 'Ada')
  assert.equal(fromEmail.address, 'ada@example.test')

  const message = buildComposeMessage({
    identity: { name: 'Sender', email: 'sender@example.test' },
    to: [toSmtpAddress({ address: 'to@example.test' })],
    cc: [],
    bcc: [],
    subject: 'Subject',
    htmlBody: restored,
    textBody: 'Body',
    attachments: [{ filename: 'report.pdf', contentType: 'application/pdf', size: 3, data: 'YWJj' }],
    inlineImages: [{
      cid: 'image@example.test', dataUrl: 'data:image/png;base64,AA==',
      contentType: 'image/png', data: 'AA==', filename: 'inline.png', size: 1,
    }],
    inReplyTo: 'parent@example.test',
    references: ['root@example.test'],
    sourceMessageId: 'source',
    replyType: 'reply',
    requestReadReceipt: true,
  })
  assert.equal(message.from.address, 'sender@example.test')
  assert.equal(message.attachments.length, 2)
  assert.equal(message.attachments[0].content_base64, 'YWJj')
  assert.equal(message.attachments[1].content_id, 'image@example.test')
  assert.equal(message.attachments[1].inline, true)
  assert.equal(message.request_read_receipt, true)
})

function focusFixture(activeTarget, isPlainTextMode = false) {
  const active = { id: activeTarget }
  const focus = Object.fromEntries(['from', 'to', 'cc', 'bcc', 'subject', 'plain', 'rich'].map((key) => [key, vi.fn()]))
  const container = (name) => ({ contains: (element) => element === active && activeTarget === name })
  return {
    active,
    focus,
    refs: {
      fromFieldElement: {
        ...container('from'),
        querySelector: () => ({ focus: focus.from }),
      },
      toFieldElement: container('to'),
      ccFieldElement: container('cc'),
      bccFieldElement: container('bcc'),
      subjectInputElement: activeTarget === 'subject' ? active : { focus: focus.subject },
      composerBodyElement: container('body'),
      plainTextRef: { focus: focus.plain },
      toInputRef: { focus: focus.to },
      ccInputRef: { focus: focus.cc },
      bccInputRef: { focus: focus.bcc },
      editor: { commands: { focus: focus.rich } },
      isPlainTextMode,
    },
  }
}

function tabEvent(shiftKey = false) {
  return { shiftKey, preventDefault: vi.fn(), stopPropagation: vi.fn() }
}

test('composer Tab navigation follows visible fields and wraps in both directions', () => {
  const disabled = focusFixture('from')
  assert.equal(handleComposerTabNavigation(tabEvent(), disabled.refs, {
    showCc: true, showBcc: true, disabled: true, activeElement: disabled.active,
  }), false)

  const from = focusFixture('from')
  const forward = tabEvent()
  assert.equal(handleComposerTabNavigation(forward, from.refs, {
    showCc: false, showBcc: false, disabled: false, activeElement: from.active,
  }), true)
  assert.equal(from.focus.to.mock.calls.length, 1)
  assert.equal(forward.preventDefault.mock.calls.length, 1)
  assert.equal(forward.stopPropagation.mock.calls.length, 1)

  const to = focusFixture('to')
  handleComposerTabNavigation(tabEvent(), to.refs, {
    showCc: true, showBcc: true, disabled: false, activeElement: to.active,
  })
  assert.equal(to.focus.cc.mock.calls.length, 1)

  const body = focusFixture('body')
  handleComposerTabNavigation(tabEvent(), body.refs, {
    showCc: true, showBcc: true, disabled: false, activeElement: body.active,
  })
  assert.equal(body.focus.from.mock.calls.length, 1)

  const reverse = focusFixture('from')
  handleComposerTabNavigation(tabEvent(true), reverse.refs, {
    showCc: false, showBcc: false, disabled: false, activeElement: reverse.active,
  })
  assert.equal(reverse.focus.rich.mock.calls.length, 1)

  const plain = focusFixture('subject', true)
  handleComposerTabNavigation(tabEvent(), plain.refs, {
    showCc: false, showBcc: false, disabled: false, activeElement: plain.active,
  })
  assert.equal(plain.focus.plain.mock.calls.length, 1)
})
