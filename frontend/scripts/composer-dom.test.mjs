// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  DeleteDraft: vi.fn(),
  GetAccount: vi.fn(),
  GetAllAccountIdentities: vi.fn(),
  GetIdentities: vi.fn(),
  PickAttachmentFiles: vi.fn(),
  ReadFileAsAttachment: vi.fn(),
  SaveDraft: vi.fn(),
  SearchContacts: vi.fn(),
  SendMessage: vi.fn(),
}))
const runtime = vi.hoisted(() => ({
  handlers: new Map(),
  EventsOn: vi.fn((name, handler) => {
    runtime.handlers.set(name, handler)
    return () => runtime.handlers.delete(name)
  }),
}))
const toast = vi.hoisted(() => ({ add: vi.fn() }))
const editorState = vi.hoisted(() => ({ editors: [] }))
const keyboardMenu = vi.hoisted(() => ({ showForRoot: vi.fn() }))
const composerSettings = vi.hoisted(() => ({
  alwaysLoadImages: false,
  darkMailContent: true,
  enhancedKeyboardNavigation: true,
  format: 'plain',
  isDark: true,
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime.js', () => runtime)
vi.mock('../src/lib/components/composer/composerEditor', () => ({
  createComposerEditor(element, options) {
    let html = ''
    const insertedImages = []
    const chain = {
      focus: () => chain,
      deleteRange: () => chain,
      insertContent: () => chain,
      setImage: (attributes) => {
        insertedImages.push(attributes)
        html += `<img src="${attributes.src}" alt="${attributes.alt || ''}">`
        element.innerHTML = html
        return chain
      },
      run: () => true,
    }
    const editor = {
      view: {
        dom: element,
        coordsAtPos: () => ({ left: 0, right: 0, top: 0, bottom: 0 }),
      },
      state: {
        doc: {
          content: { size: 0 },
          textBetween: () => '',
        },
        selection: {
          empty: true,
          from: 1,
          $from: { parent: { textBetween: () => '' }, parentOffset: 0 },
        },
      },
      commands: {
        focus: vi.fn(),
        setContent: vi.fn((value) => {
          html = value || ''
          element.innerHTML = html
        }),
      },
      chain: () => chain,
      getHTML: () => html,
      getText: () => element.textContent || '',
      isActive: () => false,
      getAttributes: () => ({}),
      on: vi.fn(),
      off: vi.fn(),
      isEmpty: true,
      destroy: vi.fn(),
      __options: options,
      __insertedImages: insertedImages,
    }
    editorState.editors.push(editor)
    return editor
  },
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/imageAllowlist.svelte', () => ({ isImageAllowedSync: () => false }))
vi.mock('$lib/stores/settings.svelte', () => ({
  getAlwaysLoadImages: () => composerSettings.alwaysLoadImages,
  getComposerFormat: () => composerSettings.format,
  getDarkMailContent: () => composerSettings.darkMailContent,
  getEnhancedKeyboardNavigation: () => composerSettings.enhancedKeyboardNavigation,
}))
vi.mock('$lib/stores/keyboardActionMenu.svelte', () => ({
  keyboardActionMenu: keyboardMenu,
}))
vi.mock('$lib/stores/toast', () => ({
  addToast: (value) => toast.add(value),
}))
vi.mock('$lib/stores/theme.svelte', () => ({ getIsDarkActive: () => composerSettings.isDark }))

import '../src/lib/iconify-offline'
import { smtp } from '../wailsjs/go/models'
import Composer from '../src/lib/components/composer/Composer.svelte'

const mounted = []

function identity(overrides = {}) {
  return {
    id: 'identity-1',
    accountId: 'account-1',
    name: 'Mail User',
    email: 'me@example.test',
    isDefault: true,
    signatureHtml: '',
    signatureText: '',
    signaturePosition: 'bottom',
    ...overrides,
  }
}

function composeMessage(overrides = {}) {
  return new smtp.ComposeMessage({
    from: new smtp.Address({ name: 'Mail User', address: 'me@example.test' }),
    to: [new smtp.Address({ name: 'Recipient', address: 'recipient@example.test' })],
    cc: [],
    bcc: [],
    subject: 'Quarterly update',
    text_body: 'Hello from the composer',
    html_body: '',
    attachments: [],
    references: [],
    request_read_receipt: false,
    ...overrides,
  })
}

function createApi(overrides = {}) {
  const userIdentity = identity()
  return {
    sendMessage: vi.fn().mockResolvedValue(undefined),
    searchContacts: vi.fn().mockResolvedValue([]),
    getIdentities: vi.fn().mockResolvedValue([userIdentity]),
    saveDraft: vi.fn().mockResolvedValue({ id: 'draft-saved', syncStatus: 'pending' }),
    deleteDraft: vi.fn().mockResolvedValue(undefined),
    pickAttachmentFiles: vi.fn().mockResolvedValue([]),
    getAccount: vi.fn().mockResolvedValue({ id: 'account-1', readReceiptRequestPolicy: 'ask' }),
    readFileAsAttachment: vi.fn().mockResolvedValue(null),
    getAllAccountIdentities: vi.fn().mockResolvedValue([{
      account: { id: 'account-1', noOutgoingServer: false },
      identities: [userIdentity],
    }]),
    ...overrides,
  }
}

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function renderComposer(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const api = props.api || createApi()
  const instance = mount(Composer, {
    target,
    props: {
      accountId: 'account-1',
      api,
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target, api }
}

function buttonWithText(root, text) {
  return [...root.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

function dispatchWithDataTransfer(node, type, dataTransfer) {
  const event = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'dataTransfer', { value: dataTransfer })
  node.dispatchEvent(event)
  return event
}

beforeEach(() => {
  document.body.innerHTML = ''
  editorState.editors.splice(0)
  composerSettings.alwaysLoadImages = false
  composerSettings.darkMailContent = true
  composerSettings.enhancedKeyboardNavigation = true
  composerSettings.format = 'plain'
  composerSettings.isDark = true
  toast.add.mockReset()
  keyboardMenu.showForRoot.mockReset()
  runtime.EventsOn.mockClear()
  runtime.handlers.clear()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('accepts recipients through the real input and sends the visible plain-text message', async () => {
  const onSent = vi.fn()
  const onClose = vi.fn()
  const { target, api } = await renderComposer({ onSent, onClose })
  assert.match(target.textContent, /composer\.newMessage/)

  const recipientInput = target.querySelector('input[placeholder="composer.addRecipients"]')
  assert.ok(recipientInput)
  recipientInput.value = 'Alice <alice@example.test>'
  recipientInput.dispatchEvent(new Event('input', { bubbles: true }))
  recipientInput.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  await flushAsync()
  assert.match(target.textContent, /Alice/)

  const subject = target.querySelector('#composer-subject')
  subject.value = 'DOM interaction report'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  const body = target.querySelector('textarea[placeholder="composer.writePlaceholder"]')
  body.value = 'This message was entered in the browser DOM.'
  body.dispatchEvent(new InputEvent('input', { bubbles: true, data: body.value }))
  await tick()

  const send = buttonWithText(target, 'composer.send')
  assert.ok(send)
  assert.equal(send.disabled, false)
  send.click()
  await flushAsync()

  assert.equal(api.sendMessage.mock.calls.length, 1)
  const [accountId, sent] = api.sendMessage.mock.calls[0]
  assert.equal(accountId, 'account-1')
  assert.equal(sent.to[0].address, 'alice@example.test')
  assert.equal(sent.subject, 'DOM interaction report')
  assert.equal(sent.text_body, 'This message was entered in the browser DOM.')
  assert.equal(sent.html_body, '')
  assert.equal(onSent.mock.calls.length, 1)
  assert.equal(onClose.mock.calls.length, 1)
  assert.deepEqual(toast.add.mock.calls.at(-1)[0], { type: 'success', message: 'composer.messageSent' })
})

test('requires empty-subject confirmation before sending and keeps the dialog keyboard-operable', async () => {
  const api = createApi()
  const { target } = await renderComposer({
    api,
    initialMessage: composeMessage({ subject: '' }),
  })
  const send = buttonWithText(target, 'composer.send')
  send.click()
  await flushAsync()

  assert.equal(api.sendMessage.mock.calls.length, 0)
  assert.match(document.body.textContent, /composer\.emptySubjectTitle/)
  const sendAnyway = buttonWithText(document, 'composer.sendAnywayGeneric')
  assert.ok(sendAnyway)
  sendAnyway.focus()
  sendAnyway.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  sendAnyway.click()
  await flushAsync()
  assert.equal(api.sendMessage.mock.calls.length, 1)
})

test('warns when the body mentions a missing attachment and sends only after confirmation', async () => {
  const api = createApi()
  const { target } = await renderComposer({
    api,
    initialMessage: composeMessage({ text_body: 'Please see the attached file.' }),
  })
  buttonWithText(target, 'composer.send').click()
  await flushAsync()

  assert.equal(api.sendMessage.mock.calls.length, 0)
  assert.match(document.body.textContent, /composer\.missingAttachmentTitle/)
  const sendAnyway = buttonWithText(document, 'composer.sendAnywayGeneric')
  assert.ok(sendAnyway)
  sendAnyway.click()
  await flushAsync()
  assert.equal(api.sendMessage.mock.calls.length, 1)
})

test('does not search mentions during IME composition and resumes after composition ends', async () => {
  const api = createApi({
    searchContacts: vi.fn().mockResolvedValue(Array.from({ length: 8 }, (_, index) => ({
      id: `contact-${index + 1}`,
      display_name: `Alice ${index + 1}`,
      email: `alice${index + 1}@example.test`,
    }))),
  })
  const { target } = await renderComposer({ api })
  vi.useFakeTimers()
  const body = target.querySelector('textarea[placeholder="composer.writePlaceholder"]')
  body.focus()
  body.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true, data: '阿' }))
  body.value = '@ali'
  body.setSelectionRange(4, 4)
  body.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'i', isComposing: true }))
  await tick()
  vi.advanceTimersByTime(300)
  await flushAsync()
  assert.equal(api.searchContacts.mock.calls.length, 0)

  body.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: 'ali' }))
  vi.advanceTimersByTime(0)
  await flushAsync()
  vi.advanceTimersByTime(150)
  await flushAsync()
  assert.deepEqual(api.searchContacts.mock.calls.at(-1), ['ali', 100])
  assert.match(target.textContent, /alice1@example.test/)

  const menu = target.querySelector('[data-composer-mention-menu]')
  const bubbledWheel = vi.fn()
  const bubbledTouch = vi.fn()
  document.body.addEventListener('wheel', bubbledWheel, { once: true })
  document.body.addEventListener('touchmove', bubbledTouch, { once: true })
  menu.dispatchEvent(new WheelEvent('wheel', { bubbles: true }))
  menu.dispatchEvent(new Event('touchmove', { bubbles: true }))
  assert.equal(bubbledWheel.mock.calls.length, 0)
  assert.equal(bubbledTouch.mock.calls.length, 0)

  let options = target.querySelectorAll('[role="option"]')
  options[1].dispatchEvent(new PointerEvent('pointermove', {
    clientX: 12, clientY: 18, bubbles: true,
  }))
  await tick()
  options = target.querySelectorAll('[role="option"]')
  assert.equal(options[1].getAttribute('aria-selected'), 'true')
  options[1].dispatchEvent(new PointerEvent('pointermove', {
    clientX: 12, clientY: 18, bubbles: true,
  }))
  body.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  await tick()
  options = target.querySelectorAll('[role="option"]')
  assert.equal(options[2].getAttribute('aria-selected'), 'true')
  body.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }))
  body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await tick()
  assert.match(target.textContent, /Alice 2/)
  assert.match(body.value, /@Alice 2/)
  await vi.advanceTimersByTimeAsync(1)
  await flushAsync()
})

test('offers keep-editing and discard choices without losing the current draft', async () => {
  const api = createApi()
  const onClose = vi.fn()
  const { target } = await renderComposer({ api, draftId: 'draft-existing', initialMessage: composeMessage(), onClose })
  buttonWithText(target, 'composer.close').click()
  await flushAsync()
  assert.match(document.body.textContent, /composer\.closeTitle/)

  const keepEditing = buttonWithText(document, 'composer.keepEditing')
  assert.ok(keepEditing)
  keepEditing.click()
  await tick()
  assert.equal(onClose.mock.calls.length, 0)
  assert.equal(document.body.textContent.includes('composer.closeTitle'), false)

  buttonWithText(target, 'composer.close').click()
  await flushAsync()
  const discard = buttonWithText(document, 'composer.discardDraft')
  assert.ok(discard)
  discard.click()
  await flushAsync()
  assert.deepEqual(api.deleteDraft.mock.calls.at(-1), ['draft-existing'])
  assert.equal(onClose.mock.calls.length, 1)
})

test('shows a non-destructive error and re-enables Send when delivery fails', async () => {
  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  const api = createApi({ sendMessage: vi.fn().mockRejectedValue(new Error('SMTP offline')) })
  const onClose = vi.fn()
  const { target } = await renderComposer({ api, initialMessage: composeMessage(), onClose })
  const send = buttonWithText(target, 'composer.send')
  send.click()
  await flushAsync()

  assert.equal(onClose.mock.calls.length, 0)
  assert.equal(send.disabled, false)
  assert.deepEqual(toast.add.mock.calls.at(-1)[0], { type: 'error', message: 'composer.failedToSend' })
  errorSpy.mockRestore()
})

test('auto-saves changed content, reports save failure, and saves explicitly before closing', async () => {
  vi.useFakeTimers()
  const api = createApi()
  const onClose = vi.fn()
  const { target } = await renderComposer({ api, initialMessage: composeMessage(), onClose })
  const subject = target.querySelector('#composer-subject')
  const body = target.querySelector('textarea[placeholder="composer.writePlaceholder"]')

  subject.value = 'Autosaved subject'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  body.value = 'Autosaved body'
  body.dispatchEvent(new InputEvent('input', { bubbles: true, data: body.value }))
  await flushAsync()
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()

  assert.equal(api.saveDraft.mock.calls.length, 1)
  assert.deepEqual(api.saveDraft.mock.calls[0].slice(0, 1), ['account-1'])
  assert.equal(api.saveDraft.mock.calls[0][1].subject, 'Autosaved subject')
  assert.equal(api.saveDraft.mock.calls[0][1].text_body, 'Autosaved body')
  assert.equal(api.saveDraft.mock.calls[0][2], '')

  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  api.saveDraft.mockRejectedValueOnce(new Error('draft store unavailable'))
  body.value = 'A second revision'
  body.dispatchEvent(new InputEvent('input', { bubbles: true, data: body.value }))
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(api.saveDraft.mock.calls.length, 2)
  assert.equal(error.mock.calls.some((call) => String(call[0]).includes('Failed to save draft')), true)

  api.saveDraft.mockResolvedValueOnce({ id: 'draft-final', syncStatus: 'synced' })
  subject.value = 'Save before close'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  buttonWithText(target, 'composer.close').click()
  await flushAsync()
  const saveAndClose = buttonWithText(document, 'composer.saveAndClose')
  assert.ok(saveAndClose)
  saveAndClose.click()
  await flushAsync()
  assert.equal(api.saveDraft.mock.calls.at(-1)[1].subject, 'Save before close')
  assert.equal(onClose.mock.calls.length, 1)
})

test('restores, removes, drops, and sends regular and inline attachments through real composer callbacks', async () => {
  composerSettings.format = 'rich'
  vi.spyOn(FileReader.prototype, 'readAsDataURL').mockImplementation(function (file) {
    const mime = file.type || 'application/octet-stream'
    const payload = file.name === 'inline.png' ? 'aW5saW5l' : 'ZHJvcHBlZA=='
    Object.defineProperty(this, 'result', { configurable: true, value: `data:${mime};base64,${payload}` })
    this.onload?.(new ProgressEvent('load'))
  })
  const api = createApi({
    readFileAsAttachment: vi.fn(async (path) => {
      if (path.endsWith('.png')) {
        return { filename: 'path-image.png', contentType: 'image/png', size: 3, data: 'aW1n' }
      }
      if (path.endsWith('.txt')) {
        return { filename: 'path-note.txt', contentType: 'text/plain', size: 4, data: 'bm90ZQ==' }
      }
      return null
    }),
  })
  const initialMessage = composeMessage({
    html_body: '<p>Rich attachment message</p><img src="cid:existing-inline">',
    text_body: 'Rich attachment message',
    attachments: [
      {
        filename: 'existing.txt',
        content_type: 'text/plain',
        content_base64: 'ZXhpc3Rpbmc=',
        content_id: '',
        inline: false,
      },
      {
        filename: 'existing.png',
        content_type: 'image/png',
        content_base64: 'aW1hZ2U=',
        content_id: 'existing-inline',
        inline: true,
      },
    ],
  })
  const { target } = await renderComposer({ api, initialMessage })
  assert.match(target.textContent, /existing\.txt/)
  assert.match(editorState.editors[0].getHTML(), /data:image\/png;base64,aW1hZ2U=/)

  target.querySelector('button[title="attachment.removeAttachment"]').click()
  await flushAsync()
  assert.equal(target.textContent.includes('existing.txt'), false)

  const root = target.querySelector('[role="region"]')
  const droppedFile = new File(['drop body'], 'dropped.txt', { type: 'text/plain' })
  const fileTransfer = {
    types: ['Files'],
    files: [droppedFile],
    dropEffect: 'none',
    getData: () => '',
  }
  const dragOver = dispatchWithDataTransfer(root, 'dragover', fileTransfer)
  await tick()
  assert.equal(dragOver.defaultPrevented, true)
  assert.equal(fileTransfer.dropEffect, 'copy')
  assert.match(target.textContent, /composer\.dropToAttach/)
  dispatchWithDataTransfer(root, 'drop', fileTransfer)
  await new Promise((resolve) => setTimeout(resolve, 0))
  await flushAsync()
  assert.match(target.textContent, /dropped\.txt/)

  const uriTransfer = {
    types: ['text/uri-list'],
    files: [],
    getData(type) {
      return type === 'text/uri-list' ? 'file:///tmp/path-note.txt' : ''
    },
  }
  dispatchWithDataTransfer(root, 'drop', uriTransfer)
  await flushAsync()
  assert.deepEqual(api.readFileAsAttachment.mock.calls.at(-1), ['/tmp/path-note.txt'])
  assert.match(target.textContent, /path-note\.txt/)

  const editor = editorState.editors[0]
  await editor.__options.onDropFilePaths(['/tmp/path-image.png', '/tmp/path-note.txt', '/tmp/ignored.bin'])
  await flushAsync()
  assert.equal(editor.__insertedImages.some((item) => item.alt === 'path-image.png'), true)

  const inlineFile = new File(['inline image'], 'inline.png', { type: 'image/png' })
  await editor.__options.onPasteImage(inlineFile)
  await editor.__options.onDropImage(inlineFile)
  await flushAsync()
  assert.equal(editor.__insertedImages.filter((item) => item.alt === 'inline.png').length, 2)

  buttonWithText(target, 'composer.send').click()
  await flushAsync()
  const sent = api.sendMessage.mock.calls.at(-1)[1]
  assert.equal(sent.attachments.some((item) => item.filename === 'dropped.txt' && !item.inline), true)
  assert.equal(sent.attachments.some((item) => item.filename === 'path-note.txt' && !item.inline), true)
  assert.equal(sent.attachments.filter((item) => item.filename === 'inline.png' && item.inline).length, 1)
  assert.equal(sent.attachments.some((item) => item.filename === 'path-image.png' && item.inline), true)
  assert.match(sent.html_body, /cid:/)
})

test('handles rich/plain display controls, Cc/Bcc, read receipts, and composer keyboard shortcuts', async () => {
  composerSettings.format = 'rich'
  const api = createApi({
    getAccount: vi.fn().mockResolvedValue({ id: 'account-1', readReceiptRequestPolicy: 'ask' }),
  })
  const onClose = vi.fn()
  const initialMessage = composeMessage({
    reply_type: 'reply',
    html_body: '<p>Reply content</p><blockquote>Quoted HTML</blockquote>',
    text_body: 'Reply content\n\n> Quoted plain text',
  })
  const { target } = await renderComposer({ api, initialMessage, onClose })
  assert.match(target.textContent, /composer\.reply/)

  const darkFilter = target.querySelector('button[aria-label="editor.disableDarkFilter"]')
  assert.ok(darkFilter)
  darkFilter.click()
  await flushAsync()
  assert.ok(target.querySelector('button[aria-label="editor.enableDarkFilter"]'))

  const switchToPlain = target.querySelector('button[title="editor.switchToPlainText"]')
  assert.ok(switchToPlain)
  switchToPlain.click()
  await flushAsync()
  assert.ok(target.querySelector('button[title="editor.switchToRichText"]'))
  target.querySelector('button[title="editor.switchToRichText"]').click()
  await flushAsync()
  assert.ok(target.querySelector('button[title="editor.switchToPlainText"]'))

  buttonWithText(target, 'composer.cc').click()
  buttonWithText(target, 'composer.bcc').click()
  await flushAsync()
  const cc = target.querySelector('input[placeholder="composer.addCcRecipients"]')
  const bcc = target.querySelector('input[placeholder="composer.addBccRecipients"]')
  cc.value = 'Copy <copy@example.test>'
  cc.dispatchEvent(new Event('input', { bubbles: true }))
  cc.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  bcc.value = 'Blind <blind@example.test>'
  bcc.dispatchEvent(new Event('input', { bubbles: true }))
  bcc.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  const receipt = target.querySelector('input[type="checkbox"]')
  receipt.click()
  await flushAsync()

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'F10', shiftKey: true, bubbles: true, cancelable: true }))
  assert.equal(keyboardMenu.showForRoot.mock.calls.length, 1)
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true }))
  await flushAsync()
  const sent = api.sendMessage.mock.calls.at(-1)[1]
  assert.equal(sent.cc[0].address, 'copy@example.test')
  assert.equal(sent.bcc[0].address, 'blind@example.test')
  assert.equal(sent.request_read_receipt, true)

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  await flushAsync()
  assert.match(document.body.textContent, /composer\.closeTitle/)
})

test('rejects oversized inline images and ignores recipient-chip drags at the composer drop surface', async () => {
  composerSettings.format = 'rich'
  const { target } = await renderComposer({ initialMessage: composeMessage() })
  const editor = editorState.editors[0]
  const oversized = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'huge.png', { type: 'image/png' })
  await editor.__options.onPasteImage(oversized)
  assert.deepEqual(toast.add.mock.calls.at(-1)[0], { type: 'error', message: 'composer.imageTooLarge' })

  const root = target.querySelector('[role="region"]')
  const recipientTransfer = {
    types: ['application/x-aulycmail-recipient'],
    files: [],
    getData: () => '',
  }
  const over = dispatchWithDataTransfer(root, 'dragover', recipientTransfer)
  assert.equal(over.defaultPrevented, false)
  dispatchWithDataTransfer(root, 'dragleave', recipientTransfer)
  const drop = dispatchWithDataTransfer(root, 'drop', recipientTransfer)
  assert.equal(drop.defaultPrevented, false)
  assert.equal(target.textContent.includes('composer.dropToAttach'), false)
})

test('restores blocked remote images and handles image and attachment file pickers', async () => {
  composerSettings.format = 'rich'
  vi.spyOn(FileReader.prototype, 'readAsDataURL').mockImplementation(function (file) {
    const payload = file.name.endsWith('.png') ? 'aW1hZ2U=' : 'YXR0YWNobWVudA=='
    Object.defineProperty(this, 'result', {
      configurable: true,
      value: `data:${file.type || 'application/octet-stream'};base64,${payload}`,
    })
    this.onload?.(new ProgressEvent('load'))
  })
  vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => {})

  const initialMessage = composeMessage({
    reply_type: 'reply',
    html_body: '<p>Quoted message</p><img src="data:image/svg+xml,blocked" data-original-src="https://images.example.test/pixel.png">',
    text_body: 'Quoted message',
  })
  const { target } = await renderComposer({ initialMessage })
  assert.match(target.textContent, /viewer\.remoteImagesBlocked/)
  const blockedImage = editorState.editors[0].view.dom.querySelector('img[data-original-src]')
  assert.ok(blockedImage)
  assert.equal(blockedImage.getAttribute('src').startsWith('data:image/svg+xml'), true)
  buttonWithText(target, 'viewer.loadImages').click()
  await flushAsync()
  assert.equal(blockedImage.getAttribute('src'), 'https://images.example.test/pixel.png')
  assert.equal(blockedImage.hasAttribute('data-original-src'), false)
  assert.doesNotMatch(target.textContent, /viewer\.remoteImagesBlocked/)

  target.querySelector('button[title="editor.insertImage"]').click()
  const imageInput = document.body.querySelector('input[type="file"][accept="image/*"]')
  assert.ok(imageInput)
  Object.defineProperty(imageInput, 'files', {
    configurable: true,
    value: [new File(['image'], 'picked.png', { type: 'image/png' })],
  })
  imageInput.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.equal(editorState.editors[0].__insertedImages.some((item) => item.alt === 'picked.png'), true)
  assert.equal(document.body.contains(imageInput), false)

  buttonWithText(target, 'composer.attachFiles').click()
  const attachmentInput = document.body.querySelector('input[type="file"][multiple]')
  assert.ok(attachmentInput)
  Object.defineProperty(attachmentInput, 'files', {
    configurable: true,
    value: [new File(['attachment'], 'picked.txt', { type: 'text/plain' })],
  })
  attachmentInput.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.match(target.textContent, /picked\.txt/)
  assert.equal(document.body.contains(attachmentInput), false)

  buttonWithText(target, 'composer.attachFiles').click()
  const emptyInput = document.body.querySelector('input[type="file"][multiple]')
  Object.defineProperty(emptyInput, 'files', { configurable: true, value: [] })
  emptyInput.dispatchEvent(new Event('change', { bubbles: true }))
  await flushAsync()
  assert.equal(document.body.contains(emptyInput), false)
})

test('switches identities across accounts, migrates the draft, and updates receipt policy', async () => {
  composerSettings.format = 'rich'
  const first = identity()
  const second = identity({
    id: 'identity-2',
    accountId: 'account-2',
    name: 'Second User',
    email: 'second@example.test',
    isDefault: true,
    signatureEnabled: true,
    signatureForNew: true,
    signatureHtml: '<p>Second signature</p>',
  })
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const api = createApi({
    getAllAccountIdentities: vi.fn().mockResolvedValue([
      { account: { id: 'account-1', noOutgoingServer: false }, identities: [first] },
      { account: { id: 'account-2', noOutgoingServer: false }, identities: [second] },
    ]),
    getAccount: vi.fn().mockImplementation(async (id) => ({
      id,
      readReceiptRequestPolicy: id === 'account-2' ? 'always' : 'ask',
    })),
    deleteDraft: vi.fn().mockRejectedValue(new Error('old draft unavailable')),
  })
  const { target } = await renderComposer({ api, draftId: 'old-account-draft', initialMessage: composeMessage() })
  const trigger = target.querySelector('[data-keyboard-select-trigger="true"]')
  assert.ok(trigger)
  trigger.dispatchEvent(new PointerEvent('pointerdown', {
    button: 0, pointerType: 'mouse', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  const secondOption = [...document.querySelectorAll('[role="option"]')]
    .find((option) => option.textContent.includes('Second User'))
  assert.ok(secondOption)
  secondOption.dispatchEvent(new PointerEvent('pointerup', {
    button: 0, pointerType: 'mouse', bubbles: true, cancelable: true,
  }))
  await flushAsync()

  assert.deepEqual(api.getAccount.mock.calls.at(-1), ['account-2'])
  assert.deepEqual(api.deleteDraft.mock.calls.at(-1), ['old-account-draft'])
  assert.equal(error.mock.calls.some((call) => String(call[0]).includes('Failed to delete old account draft')), true)
  assert.equal(target.querySelector('input[type="checkbox"]'), null)
  assert.match(editorState.editors[0].getHTML(), /Second signature/)
})

test('handles draft sync events and reports unavailable sender and account settings', async () => {
  const noIdentityApi = createApi({
    getAllAccountIdentities: vi.fn().mockResolvedValue([]),
    getAccount: vi.fn().mockRejectedValue(new Error('account unavailable')),
  })
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  await renderComposer({ api: noIdentityApi, draftId: 'tracked-draft', initialMessage: composeMessage() })
  runtime.handlers.get('draft:syncStatusChanged')?.({ draftId: 'other', syncStatus: 'failed' })
  runtime.handlers.get('draft:syncStatusChanged')?.({ draftId: 'tracked-draft', syncStatus: 'failed' })
  await flushAsync()
  assert.equal(error.mock.calls.some((call) => String(call[0]).includes('Failed to load account settings')), true)

  window.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'composer.selectSenderIdentity')
})

test('returns focus from toolbar hints and rejects shortcut sends without recipients', async () => {
  const { target } = await renderComposer()
  window.dispatchEvent(new KeyboardEvent('keydown', {
    key: 't', code: 'KeyT', altKey: true, bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.ok(target.querySelector('[data-keyboard-toolbar-selected="true"]'))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.ok(target.querySelector('button[title="editor.switchToPlainText"]'))
  assert.equal(editorState.editors[0].commands.focus.mock.calls.length > 0, true)

  window.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'composer.noRecipients')
})

test('revalidates a newly mentioned attachment after empty-subject confirmation', async () => {
  const api = createApi()
  const { target } = await renderComposer({
    api,
    initialMessage: composeMessage({ subject: '', text_body: 'Ordinary body' }),
  })
  buttonWithText(target, 'composer.send').click()
  await flushAsync()
  assert.match(document.body.textContent, /composer\.emptySubjectTitle/)

  const body = target.querySelector('textarea[placeholder="composer.writePlaceholder"]')
  body.value = 'Please see the attached report.'
  body.dispatchEvent(new InputEvent('input', { bubbles: true, data: body.value }))
  buttonWithText(document, 'composer.sendAnywayGeneric').click()
  await flushAsync()
  assert.match(document.body.textContent, /composer\.missingAttachmentTitle/)
  assert.equal(api.sendMessage.mock.calls.length, 0)

  buttonWithText(document, 'composer.sendAnywayGeneric').click()
  await flushAsync()
  assert.equal(api.sendMessage.mock.calls.length, 1)
})

test('keeps a failed discard recoverable and closes an empty composer without creating a draft', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const deleteDraft = vi.fn()
    .mockRejectedValueOnce(new Error('draft delete unavailable'))
    .mockResolvedValueOnce(undefined)
  const api = createApi({ deleteDraft })
  const onClose = vi.fn()
  let rendered = await renderComposer({ api, draftId: 'draft-retry', initialMessage: composeMessage(), onClose })
  buttonWithText(rendered.target, 'composer.close').click()
  await flushAsync()
  buttonWithText(document, 'composer.discardDraft').click()
  await flushAsync()
  assert.equal(onClose.mock.calls.length, 0)
  assert.deepEqual(toast.add.mock.calls.at(-1)[0], { type: 'error', message: 'composer.failedToDiscardDraft' })
  assert.match(document.body.textContent, /composer\.closeTitle/)

  buttonWithText(document, 'composer.discardDraft').click()
  await flushAsync()
  assert.equal(deleteDraft.mock.calls.length, 2)
  assert.equal(onClose.mock.calls.length, 1)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  const emptyApi = createApi()
  const emptyClose = vi.fn()
  rendered = await renderComposer({ api: emptyApi, onClose: emptyClose })
  buttonWithText(rendered.target, 'composer.close').click()
  await flushAsync()
  buttonWithText(document, 'composer.saveAndClose').click()
  await flushAsync()
  assert.equal(emptyApi.saveDraft.mock.calls.length, 0)
  assert.equal(emptyClose.mock.calls.length, 1)
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to delete draft:'))
})

test('supports detached identity loading and receive-only reply identity routing', async () => {
  const detachedIdentity = identity({ id: 'detached', name: 'Detached Sender', email: 'detached@example.test' })
  const detachedApi = createApi({
    getAllAccountIdentities: undefined,
    getIdentities: vi.fn().mockResolvedValue([detachedIdentity]),
  })
  let rendered = await renderComposer({ api: detachedApi, initialMessage: composeMessage({
    from: { name: 'Detached Sender', email: 'detached@example.test' },
  }) })
  buttonWithText(rendered.target, 'composer.send').click()
  await flushAsync()
  assert.deepEqual(detachedApi.getIdentities.mock.calls.at(-1), ['account-1'])
  assert.equal(detachedApi.sendMessage.mock.calls.at(-1)[1].from.address, 'detached@example.test')

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  const preferredIdentity = identity({
    id: 'preferred-identity', accountId: 'account-2', name: 'Preferred Sender', email: 'preferred@example.test',
  })
  const receiveOnlyApi = createApi({
    getAllAccountIdentities: vi.fn().mockResolvedValue([
      {
        account: { id: 'account-1', noOutgoingServer: true, replyForwardIdentityId: 'preferred-identity' },
        identities: [identity({ id: 'unusable-source' })],
      },
      {
        account: { id: 'account-2', noOutgoingServer: false },
        identities: [preferredIdentity],
      },
    ]),
    getAccount: vi.fn().mockResolvedValue({ id: 'account-2', readReceiptRequestPolicy: 'always' }),
  })
  rendered = await renderComposer({
    api: receiveOnlyApi,
    initialMessage: composeMessage({ from: { address: 'unmatched@example.test' }, reply_type: 'reply' }),
  })
  assert.equal(rendered.target.querySelector('input[type="checkbox"]'), null)
  buttonWithText(rendered.target, 'composer.send').click()
  await flushAsync()
  const [activeAccount, sent] = receiveOnlyApi.sendMessage.mock.calls.at(-1)
  assert.equal(activeAccount, 'account-2')
  assert.equal(sent.from.address, 'preferred@example.test')
  assert.equal(sent.request_read_receipt, true)
})

test('skips empty and duplicate draft saves and serializes changes behind an in-flight save', async () => {
  vi.useFakeTimers()
  let finishFirstSave
  const saveDraft = vi.fn()
    .mockReturnValueOnce(new Promise((resolve) => { finishFirstSave = resolve }))
    .mockResolvedValue({ id: 'draft-after-flight', syncStatus: 'synced' })
  const api = createApi({ saveDraft })
  const { target } = await renderComposer({ api })
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(saveDraft.mock.calls.length, 0)

  const subject = target.querySelector('#composer-subject')
  subject.value = 'First autosave'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(saveDraft.mock.calls.length, 1)

  subject.value = 'Edit while saving'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(saveDraft.mock.calls.length, 1)

  finishFirstSave({ id: 'draft-first', syncStatus: 'pending' })
  await flushAsync()
  subject.value = 'Saved after flight'
  subject.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(saveDraft.mock.calls.length, 2)

  subject.dispatchEvent(new Event('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(10_050)
  await flushAsync()
  assert.equal(saveDraft.mock.calls.length, 2)
})

test('searches and selects rich-text mentions and reports stale mention-search failures', async () => {
  vi.useFakeTimers()
  composerSettings.format = 'rich'
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const searchContacts = vi.fn()
    .mockResolvedValueOnce([{ id: 'rich-contact', display_name: 'Rich Alice', email: 'rich@example.test' }])
    .mockRejectedValueOnce(new Error('mention search unavailable'))
  const api = createApi({ searchContacts })
  const { target } = await renderComposer({ api })
  const editor = editorState.editors[0]
  editor.state.selection = {
    empty: true,
    from: 5,
    $from: { parent: { textBetween: () => '@ali' }, parentOffset: 4 },
  }
  editor.state.doc.content.size = 5
  editor.__options.onUpdate()
  await vi.advanceTimersByTimeAsync(150)
  await flushAsync()
  assert.match(target.textContent, /rich@example\.test/)

  editor.view.dom.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.match(target.textContent, /Rich Alice/)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  composerSettings.format = 'plain'
  const failingApi = createApi({ searchContacts: vi.fn().mockRejectedValue(new Error('mention search unavailable')) })
  const failed = await renderComposer({ api: failingApi })
  const plainBody = failed.target.querySelector('textarea[placeholder="composer.writePlaceholder"]')
  plainBody.value = '@bob'
  plainBody.setSelectionRange(4, 4)
  plainBody.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'b' }))
  await vi.advanceTimersByTimeAsync(150)
  await flushAsync()
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to search contacts for mention:'))
})
