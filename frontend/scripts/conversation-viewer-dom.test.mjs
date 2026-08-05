// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetConversation: vi.fn(),
  GetReadReceiptResponsePolicy: vi.fn(),
  SendReadReceipt: vi.fn(),
  IgnoreReadReceipt: vi.fn(),
  GetMarkAsReadDelay: vi.fn(),
  GetMessageSource: vi.fn(),
  FetchMessageBody: vi.fn(),
  OpenFile: vi.fn(),
  MarkAsRead: vi.fn(),
  GetInlineAttachments: vi.fn(),
  AddImageAllowlist: vi.fn(),
  OpenURL: vi.fn(),
  GetAccounts: vi.fn(),
  GetFolders: vi.fn(),
}))
const eventHandlers = vi.hoisted(() => new Map())
const runtime = vi.hoisted(() => ({
  EventsOn: vi.fn((name, handler) => {
    eventHandlers.set(name, handler)
    return () => eventHandlers.delete(name)
  }),
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const keyboard = vi.hoisted(() => ({
  composerOpen: false,
  setFocusedPane: vi.fn(),
}))
const dialogGuard = vi.hoisted(() => ({
  active: false,
  listeners: new Set(),
}))
const mailActions = vi.hoisted(() => {
  const complete = (name) => vi.fn(async (_ids, ...rest) => {
    const options = rest.at(-1)
    await options?.onSuccess?.(options?.autoSelectNext)
    return name
  })
  return {
    archiveMessages: complete('archive'),
    deleteMessagesPermanently: complete('delete'),
    setReadStateMessages: complete('read'),
    toggleSpamMessages: complete('spam'),
    toggleStarMessages: complete('star'),
    trashMessages: complete('trash'),
    undoLastMailAction: vi.fn(async (options) => options?.onSuccess?.()),
    copyMessagesToFolder: complete('copy'),
    moveMessagesToFolder: complete('move'),
  }
})

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime.js', () => runtime)
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/settings.svelte', () => ({
  getDarkMailContent: () => true,
  getDeveloperMode: () => true,
  getEnhancedKeyboardNavigation: () => true,
  getCurrentDateFnsLocale: () => undefined,
  getAlwaysLoadImages: () => false,
  getThemeMode: () => 'dark',
}))
vi.mock('$lib/stores/theme.svelte', () => ({ getIsDarkActive: () => true }))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  setFocusedPane: keyboard.setFocusedPane,
  isComposerOpen: () => keyboard.composerOpen,
  isInputElement: (target) => target instanceof HTMLElement && Boolean(target.closest('input,textarea,select,[contenteditable="true"],[role="textbox"]')),
  focusPreviousPane: vi.fn(),
  focusNextPane: vi.fn(),
}))
vi.mock('$lib/mailActions', () => mailActions)
vi.mock('$lib/stores/imageAllowlist.svelte', () => ({
  isImageAllowedSync: () => false,
  refreshImageAllowlist: vi.fn(),
}))
vi.mock('$lib/stores/dialogGuard', () => ({
  isDialogGuardActive: () => dialogGuard.active,
  dialogGuardOpen: vi.fn(),
  dialogGuardClose: vi.fn(),
  onDialogGuardChange: (listener) => {
    dialogGuard.listeners.add(listener)
    return () => dialogGuard.listeners.delete(listener)
  },
}))

import '../src/lib/iconify-offline'
import ConversationViewer from '../src/lib/components/viewer/ConversationViewer.svelte'

const mounted = []

function message(id, overrides = {}) {
  return {
    id,
    accountId: 'account-1',
    fromName: `Sender ${id}`,
    fromEmail: `${id}@example.test`,
    toList: JSON.stringify([{ name: 'Recipient', email: 'recipient@example.test' }]),
    ccList: '',
    bccList: '',
    replyTo: '',
    date: '2026-08-01T08:00:00Z',
    snippet: `Snippet ${id}`,
    bodyText: `Body ${id}`,
    bodyHtml: '',
    bodyFetched: true,
    isRead: true,
    isStarred: false,
    hasAttachments: false,
    readReceiptTo: '',
    readReceiptHandled: false,
    ...overrides,
  }
}

function conversation(messages = [message('message-1')], overrides = {}) {
  return {
    threadId: 'thread-1',
    subject: 'Quarterly update',
    unreadCount: messages.filter((item) => !item.isRead).length,
    messages,
    ...overrides,
  }
}

function titledButton(root, title) {
  return root.querySelector(`button[title="${title}"]`)
}

function buttonWithText(root, text) {
  return [...root.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function renderViewer(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(ConversationViewer, {
    target,
    props: {
      threadId: 'thread-1',
      folderId: 'inbox-1',
      folderType: 'inbox',
      accountId: 'account-1',
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

beforeEach(() => {
  document.body.innerHTML = ''
  eventHandlers.clear()
  dialogGuard.active = false
  dialogGuard.listeners.clear()
  keyboard.composerOpen = false
  keyboard.setFocusedPane.mockReset()
  backend.GetConversation.mockReset().mockResolvedValue(conversation())
  backend.GetReadReceiptResponsePolicy.mockReset().mockResolvedValue('ask')
  backend.SendReadReceipt.mockReset().mockResolvedValue(undefined)
  backend.IgnoreReadReceipt.mockReset().mockResolvedValue(undefined)
  backend.GetMarkAsReadDelay.mockReset().mockResolvedValue(-1)
  backend.GetMessageSource.mockReset().mockResolvedValue({ content: 'From: sender@example.test\nSubject: Raw', filePath: '' })
  backend.FetchMessageBody.mockReset().mockResolvedValue(null)
  backend.OpenFile.mockReset().mockResolvedValue(undefined)
  backend.MarkAsRead.mockReset().mockResolvedValue(undefined)
  backend.GetInlineAttachments.mockReset().mockResolvedValue({})
  backend.AddImageAllowlist.mockReset().mockResolvedValue(undefined)
  backend.OpenURL.mockReset().mockResolvedValue(undefined)
  backend.GetAccounts.mockReset().mockResolvedValue([])
  backend.GetFolders.mockReset().mockResolvedValue([])
  runtime.EventsOn.mockClear()
  toast.success.mockReset()
  toast.error.mockReset()
  for (const action of Object.values(mailActions)) action.mockReset?.()
  const clipboard = navigator.clipboard || { writeText: async () => {} }
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: clipboard })
  vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
  if (!HTMLElement.prototype.scrollBy) {
    HTMLElement.prototype.scrollBy = function scrollBy(options) {
      if (typeof options === 'object' && options?.top != null) this.scrollTop += options.top
    }
  }
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('renders newest-first messages and toggles a message with Enter and Space', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('older', { isRead: true }),
    message('newer', { isRead: false }),
  ]))
  const { instance, target } = await renderViewer()

  assert.match(target.textContent, /Quarterly update/)
  assert.equal(instance.getLastMessageId(), 'newer')
  const messageElements = [...target.querySelectorAll('[data-message-id]')]
  assert.deepEqual(messageElements.map((element) => element.dataset.messageId), ['newer', 'older'])
  const olderHeader = messageElements[1].querySelector('[data-keyboard-action-context="Sender older"]')
  assert.ok(olderHeader)
  assert.equal(messageElements[1].getAttribute('aria-expanded'), 'false')

  olderHeader.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  await tick()
  assert.equal(messageElements[1].getAttribute('aria-expanded'), 'true')
  olderHeader.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
  await tick()
  assert.equal(messageElements[1].getAttribute('aria-expanded'), 'false')
})

test('routes toolbar replies and action APIs using the focused message', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('older'),
    message('newer'),
  ]))
  const onReply = vi.fn()
  const onActionComplete = vi.fn()
  const { instance, target } = await renderViewer({ onReply, onActionComplete, isFocused: true })
  const older = target.querySelector('[data-message-id="older"]')
  assert.ok(older)
  older.focus()
  await tick()
  assert.equal(instance.getFocusedMessageId(), 'older')

  instance.reply()
  instance.replyAll()
  instance.forward()
  assert.deepEqual(onReply.mock.calls, [
    ['reply', 'older', false],
    ['reply-all', 'older', false],
    ['forward', 'older', false],
  ])

  await instance.archive()
  await instance.spam()
  await instance.toggleStar()
  await instance.markRead()
  assert.deepEqual(mailActions.archiveMessages.mock.calls.at(-1)[0], ['older', 'newer'])
  assert.deepEqual(mailActions.toggleSpamMessages.mock.calls.at(-1).slice(0, 2), [['older', 'newer'], false])
  assert.deepEqual(mailActions.toggleStarMessages.mock.calls.at(-1).slice(0, 2), [['older', 'newer'], true])
  assert.deepEqual(mailActions.setReadStateMessages.mock.calls.at(-1).slice(0, 2), [['older', 'newer'], false])
})

test('guards focused-message deletion during IME composition and while the composer is open', async () => {
  const { target } = await renderViewer({ isFocused: true })
  const messageElement = target.querySelector('[data-message-id="message-1"]')
  assert.ok(messageElement)
  messageElement.focus()
  await tick()

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Delete', bubbles: true, isComposing: true }))
  await flushAsync()
  assert.equal(mailActions.trashMessages.mock.calls.length, 0)

  keyboard.composerOpen = true
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Delete', bubbles: true }))
  await flushAsync()
  assert.equal(mailActions.trashMessages.mock.calls.length, 0)

  keyboard.composerOpen = false
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Delete', bubbles: true }))
  await flushAsync()
  assert.deepEqual(mailActions.trashMessages.mock.calls.at(-1)[0], ['message-1'])
})

test('copies an address with keyboard activation and handles requested read receipts', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('receipt', { isRead: false, readReceiptTo: 'receipt@example.test' }),
  ]))
  const { target } = await renderViewer()
  const address = target.querySelector('[data-keyboard-action-context="receipt@example.test"]')
  assert.ok(address)
  address.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  await flushAsync()
  assert.deepEqual(navigator.clipboard.writeText.mock.calls.at(-1), ['Sender receipt <receipt@example.test>'])
  assert.match(toast.success.mock.calls.at(-1)[0], /viewer\.copiedToClipboard/)

  const sendReceipt = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('viewer.sendReceipt'))
  assert.ok(sendReceipt)
  sendReceipt.click()
  await flushAsync()
  assert.deepEqual(backend.SendReadReceipt.mock.calls.at(-1), ['account-1', 'receipt'])
  assert.equal(target.textContent.includes('viewer.readReceiptRequested'), false)
})

test('surfaces body-fetch and conversation-load failures and recovers through retry', async () => {
  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetConversation.mockResolvedValue(conversation([
    message('missing-body', { bodyFetched: false, bodyText: '', bodyHtml: '' }),
  ]))
  backend.FetchMessageBody.mockRejectedValue(new Error('local message source unavailable'))
  const { target } = await renderViewer()
  await flushAsync()
  assert.match(target.textContent, /viewer\.localContentUnavailable/)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetConversation.mockRejectedValueOnce(new Error('database unavailable'))
  const failed = await renderViewer()
  assert.match(failed.target.textContent, /viewer\.failedToLoad/)

  backend.GetConversation.mockResolvedValueOnce(conversation([message('recovered')]))
  const retry = [...failed.target.querySelectorAll('button')].find((button) => button.textContent.includes('viewer.tryAgain'))
  assert.ok(retry)
  retry.click()
  await flushAsync()
  assert.match(failed.target.textContent, /Body recovered/)
  errorSpy.mockRestore()
})

test('toggles the dark-content mode and opens source in a real dialog portal', async () => {
  const { target } = await renderViewer()
  const darkToggle = target.querySelector('button[title="viewer.darkMailToLight"]')
  assert.ok(darkToggle)
  darkToggle.click()
  await tick()
  assert.ok(target.querySelector('button[title="viewer.darkMailToDark"]'))

  const source = target.querySelector('button[title="viewer.viewSource"]')
  assert.ok(source)
  source.click()
  await flushAsync()
  assert.deepEqual(backend.GetMessageSource.mock.calls.at(-1), ['message-1'])
  assert.match(document.body.textContent, /From: sender@example.test/)

  const close = document.querySelector('button[aria-label="common.close"]')
  assert.ok(close)
  close.click()
  await tick()
  assert.equal(document.body.textContent.includes('From: sender@example.test'), false)
})

test('expands and collapses the thread, scrolls, selects message text, and dispatches image controls', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('older', { bodyHtml: '<p>Older HTML</p>' }),
    message('newer', { bodyHtml: '<p>Newer HTML</p>' }),
  ]))
  const onBack = vi.fn()
  const onToggleThreadFocus = vi.fn()
  const loadImages = vi.fn()
  const openDropdown = vi.fn()
  window.addEventListener('load-remote-images', loadImages)
  window.addEventListener('open-always-load-dropdown', openDropdown)
  const { instance, target } = await renderViewer({
    showBackButton: true,
    onBack,
    onToggleThreadFocus,
    inFocusMode: true,
    focusModeKind: 'thread',
  })

  titledButton(target, 'viewer.expandAll').click()
  await flushAsync()
  assert.equal([...target.querySelectorAll('[data-message-id]')].every((node) => node.getAttribute('aria-expanded') === 'true'), true)
  titledButton(target, 'viewer.collapseAll').click()
  await flushAsync()
  assert.equal([...target.querySelectorAll('[data-message-id]')].every((node) => node.getAttribute('aria-expanded') === 'false'), true)

  const newer = target.querySelector('[data-message-id="newer"]')
  newer.focus()
  newer.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  await flushAsync()
  assert.equal(instance.hasFocusedMessage(), true)
  assert.equal(instance.getFocusedMessageId(), 'newer')
  const iframe = newer.querySelector('iframe')
  if (iframe?.contentWindow) {
    const postMessage = vi.spyOn(iframe.contentWindow, 'postMessage')
    instance.selectAllText()
    assert.deepEqual(postMessage.mock.calls.at(-1), [{ type: 'select-all' }, '*'])
  } else {
    instance.selectAllText()
  }

  const scrollContainer = target.querySelector('.overflow-y-auto')
  scrollContainer.scrollTop = 200
  const scrollBy = vi.spyOn(scrollContainer, 'scrollBy')
  instance.scrollDown()
  assert.deepEqual(scrollBy.mock.calls.at(-1), [{ top: 100, behavior: 'smooth' }])
  instance.scrollUp()
  assert.deepEqual(scrollBy.mock.calls.at(-1), [{ top: -100, behavior: 'smooth' }])

  instance.loadImages()
  instance.openAlwaysLoadDropdown()
  assert.equal(loadImages.mock.calls.length, 1)
  assert.equal(openDropdown.mock.calls.length, 1)
  instance.openContextMenu()

  titledButton(target, 'responsive.back').click()
  titledButton(target, 'viewer.exitFocus').click()
  assert.equal(onBack.mock.calls.length, 1)
  assert.equal(onToggleThreadFocus.mock.calls.length, 1)
  window.removeEventListener('load-remote-images', loadImages)
  window.removeEventListener('open-always-load-dropdown', openDropdown)
})

test('confirms whole-thread deletion in trash and targets a focused message for permanent deletion', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('older'),
    message('newer'),
  ]))
  const onActionComplete = vi.fn()
  const { instance, target } = await renderViewer({ folderType: 'trash', folderId: 'trash-1', onActionComplete })

  titledButton(target, 'viewer.deletePermanently').click()
  await flushAsync()
  let confirm = document.querySelector('[role="alertdialog"] button:last-child')
  assert.ok(confirm)
  confirm.click()
  await flushAsync()
  assert.deepEqual(mailActions.deleteMessagesPermanently.mock.calls.at(-1)[0], ['older', 'newer'])

  const older = target.querySelector('[data-message-id="older"]')
  older.focus()
  await tick()
  instance.deletePermanently()
  await flushAsync()
  assert.deepEqual(mailActions.deleteMessagesPermanently.mock.calls.at(-1)[0], ['older'])

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetConversation.mockResolvedValue(conversation([message('inbox-message')]))
  const inbox = await renderViewer({ onActionComplete })
  inbox.instance.trash()
  await flushAsync()
  assert.deepEqual(mailActions.trashMessages.mock.calls.at(-1)[0], ['inbox-message'])
})

test('ignores or auto-sends receipts and reports receipt failures without hiding other mail', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetConversation.mockResolvedValue(conversation([
    message('ignored', { readReceiptTo: 'receipt@example.test' }),
  ]))
  const first = await renderViewer()
  const ignore = [...first.target.querySelectorAll('button')].find((button) => button.textContent.includes('viewer.ignoreReceipt'))
  assert.ok(ignore)
  ignore.click()
  await flushAsync()
  assert.deepEqual(backend.IgnoreReadReceipt.mock.calls.at(-1), ['account-1', 'ignored'])
  assert.equal(first.target.textContent.includes('viewer.readReceiptRequested'), false)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('always')
  backend.GetConversation.mockResolvedValue(conversation([
    message('automatic', { readReceiptTo: 'auto@example.test' }),
  ]))
  const automatic = await renderViewer()
  const header = automatic.target.querySelector('[data-keyboard-action-context="Sender automatic"]')
  header.click()
  header.click()
  await flushAsync()
  assert.deepEqual(backend.SendReadReceipt.mock.calls.at(-1), ['account-1', 'automatic'])

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('ask')
  backend.SendReadReceipt.mockRejectedValueOnce(new Error('MDN rejected'))
  backend.IgnoreReadReceipt.mockRejectedValueOnce(new Error('ignore rejected'))
  backend.GetConversation.mockResolvedValue(conversation([
    message('failure', { readReceiptTo: 'failure@example.test' }),
  ]))
  const failed = await renderViewer()
  buttonWithText(failed.target, 'viewer.sendReceipt').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'viewer.failedToSendReceipt')
  buttonWithText(failed.target, 'viewer.ignoreReceipt').click()
  await flushAsync()
  assert.equal(error.mock.calls.length >= 2, true)
})

test('handles source-file results, source failures, copy failures, and draft editing controls', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('draft', {
      replyTo: 'reply@example.test',
      toList: '{invalid-json',
      ccList: JSON.stringify([{ name: 'Copy', email: 'copy@example.test' }]),
      bccList: JSON.stringify([{ name: '', email: 'blind@example.test' }]),
    }),
  ]))
  backend.GetMessageSource.mockResolvedValue({ content: '', filePath: '/tmp/large-message.eml' })
  const onEditDraft = vi.fn()
  const { target } = await renderViewer({ folderType: 'drafts', onEditDraft })
  titledButton(target, 'viewer.editDraft').click()
  assert.deepEqual(onEditDraft.mock.calls.at(-1), ['draft'])

  titledButton(target, 'viewer.viewSource').click()
  await flushAsync()
  const openFile = buttonWithText(document, 'viewer.openSourceFile')
  assert.ok(openFile)
  openFile.click()
  await flushAsync()
  assert.deepEqual(backend.OpenFile.mock.calls.at(-1), ['/tmp/large-message.eml'])

  backend.OpenFile.mockRejectedValueOnce(new Error('cannot open source'))
  openFile.click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'viewer.failedToLoadSource')
  document.querySelector('button[aria-label="common.close"]').click()
  await flushAsync()

  backend.GetMessageSource.mockResolvedValueOnce({ content: 'Raw source content', filePath: '' })
  titledButton(target, 'viewer.viewSource').click()
  await flushAsync()
  navigator.clipboard.writeText.mockRejectedValueOnce(new Error('clipboard denied'))
  document.querySelector('button[title="viewer.copySource"]').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'viewer.failedToCopy')
  document.querySelector('button[aria-label="common.close"]').click()
  await flushAsync()

  backend.GetMessageSource.mockRejectedValueOnce(new Error('source unavailable'))
  titledButton(target, 'viewer.viewSource').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'viewer.failedToLoadSource')
})

test('reacts to read, move, delete, undo, and coalesced sync events without stale refreshes', async () => {
  vi.useFakeTimers()
  const onActionComplete = vi.fn()
  backend.GetConversation.mockResolvedValue(conversation([
    message('one'),
    message('two'),
  ]))
  const first = await renderViewer({ onActionComplete })

  first.instance.markRead()
  await flushAsync()
  eventHandlers.get('messages:readChanged')({ messageIds: ['one', 'two'], isRead: false })
  await flushAsync()
  assert.match(first.target.textContent, /Quarterly update/)

  const initialLoads = backend.GetConversation.mock.calls.length
  eventHandlers.get('messages:updated')({ accountId: 'other', folderId: 'other' })
  eventHandlers.get('messages:updated')({ accountId: 'account-1', folderId: 'inbox-1' })
  eventHandlers.get('folder:synced')({ accountId: 'account-1', folderId: 'inbox-1' })
  eventHandlers.get('sent:synced')({ accountId: 'account-1' })
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.equal(backend.GetConversation.mock.calls.length, initialLoads + 1)

  eventHandlers.get('messages:moved')({ messageIds: ['one'], destFolderId: 'archive-1' })
  await flushAsync()
  assert.equal(backend.GetConversation.mock.calls.length >= initialLoads + 2, true)
  eventHandlers.get('undo:completed')()
  await flushAsync()

  eventHandlers.get('messages:moved')({ messageIds: ['one', 'two'], destFolderId: 'archive-1' })
  await flushAsync()
  assert.deepEqual(onActionComplete.mock.calls.at(-1), [true])

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetConversation.mockResolvedValue(conversation([message('delete-one')]))
  await renderViewer({ onActionComplete })
  eventHandlers.get('messages:deleted')(['delete-one'])
  await flushAsync()
  assert.deepEqual(onActionComplete.mock.calls.at(-1), [true])
})

test('renders all recipient fields and supports click, Enter, Space, and ignored recipient keys', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('rich-recipients', {
      fromName: '',
      fromEmail: 'sender@example.test',
      replyTo: 'reply@example.test',
      toList: JSON.stringify([
        { name: 'First recipient', email: 'first@example.test' },
        { name: '', email: 'second@example.test' },
      ]),
      ccList: JSON.stringify([
        { name: 'Copy one', email: 'copy-one@example.test' },
        { name: '', email: 'copy-two@example.test' },
      ]),
      bccList: JSON.stringify([
        { name: 'Blind one', email: 'blind-one@example.test' },
        { name: '', email: 'blind-two@example.test' },
      ]),
    }),
    message('invalid-recipients', {
      toList: '{invalid json',
      ccList: JSON.stringify({ email: 'not-an-array@example.test' }),
      bccList: '[]',
    }),
  ]))
  const { target } = await renderViewer()

  const activate = async (selector, key) => {
    const node = target.querySelector(`[data-keyboard-action-context="${selector}"]`)
    assert.ok(node, `missing recipient ${selector}`)
    node.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', bubbles: true, cancelable: true }))
    node.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
    await flushAsync()
  }

  await activate('reply@example.test', 'Enter')
  await activate('first@example.test', ' ')
  await activate('second@example.test', 'Enter')
  await activate('copy-one@example.test', ' ')
  await activate('copy-two@example.test', 'Enter')
  await activate('blind-one@example.test', ' ')
  await activate('blind-two@example.test', 'Enter')

  const sender = target.querySelector('span[data-keyboard-action-context="sender@example.test"]')
  assert.ok(sender)
  sender.click()
  await flushAsync()
  assert.deepEqual(navigator.clipboard.writeText.mock.calls.at(-1), ['sender@example.test'])
  assert.equal(navigator.clipboard.writeText.mock.calls.length >= 8, true)
})

test('applies automatic and manual read-receipt policies, including failures and in-flight deduplication', async () => {
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('always')
  backend.GetConversation.mockResolvedValue(conversation([
    message('always-receipt', { readReceiptTo: 'receipt@example.test' }),
  ]))
  let finishReceipt
  backend.SendReadReceipt.mockReturnValue(new Promise((resolve) => { finishReceipt = resolve }))
  let rendered = await renderViewer()
  const alwaysHeader = rendered.target.querySelector('[data-keyboard-action-context="Sender always-receipt"]')
  alwaysHeader.click()
  alwaysHeader.click()
  alwaysHeader.click()
  alwaysHeader.click()
  await flushAsync()
  assert.equal(backend.SendReadReceipt.mock.calls.length, 1)
  finishReceipt()
  await flushAsync()
  assert.match(toast.success.mock.calls.at(-1)[0], /viewer\.readReceiptSent/)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('ask')
  backend.GetConversation.mockResolvedValue(conversation([
    message('ignored-receipt', { readReceiptTo: 'ignore@example.test' }),
  ]))
  rendered = await renderViewer()
  const ignore = buttonWithText(rendered.target, 'viewer.ignoreReceipt')
  assert.ok(ignore)
  ignore.click()
  await flushAsync()
  assert.deepEqual(backend.IgnoreReadReceipt.mock.calls.at(-1), ['account-1', 'ignored-receipt'])
  assert.equal(rendered.target.textContent.includes('viewer.readReceiptRequested'), false)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetConversation.mockResolvedValue(conversation([
    message('failed-receipt', { readReceiptTo: 'failure@example.test' }),
  ]))
  backend.SendReadReceipt.mockRejectedValueOnce(new Error('receipt SMTP unavailable'))
  backend.IgnoreReadReceipt.mockRejectedValueOnce(new Error('ignore unavailable'))
  rendered = await renderViewer()
  buttonWithText(rendered.target, 'viewer.sendReceipt').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'viewer.failedToSendReceipt')
  buttonWithText(rendered.target, 'viewer.ignoreReceipt').click()
  await flushAsync()
  assert.equal(backend.IgnoreReadReceipt.mock.calls.length > 0, true)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  backend.GetReadReceiptResponsePolicy.mockResolvedValue('never')
  backend.GetConversation.mockResolvedValue(conversation([
    message('never-receipt', { readReceiptTo: 'never@example.test' }),
  ]))
  rendered = await renderViewer()
  assert.equal(rendered.target.textContent.includes('viewer.readReceiptRequested'), false)
})

test('classifies body download results and narrows message-focus rendering', async () => {
  backend.GetConversation.mockResolvedValue(conversation([
    message('body-success', { bodyFetched: false, bodyText: '', bodyHtml: '', isRead: false }),
    message('body-large', { bodyFetched: false, bodyText: '', bodyHtml: '', isRead: false }),
    message('body-generic', { bodyFetched: false, bodyText: '', bodyHtml: '', isRead: false }),
  ]))
  backend.FetchMessageBody.mockImplementation(async (id) => {
    if (id === 'body-success') return message(id, { bodyText: 'Downloaded body', bodyFetched: true, isRead: false })
    if (id === 'body-large') throw new Error('message body is too large')
    throw null
  })
  let rendered = await renderViewer({
    inFocusMode: true,
    focusModeKind: 'message',
    focusedMessageIdInFocus: 'body-large',
  })
  await flushAsync()
  assert.equal(rendered.target.querySelectorAll('[data-message-id]').length, 1)
  assert.ok(rendered.target.querySelector('[data-message-id="body-large"]'))
  assert.match(rendered.target.textContent, /viewer\.bodyTooLarge/)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  rendered = await renderViewer()
  await flushAsync()
  assert.match(rendered.target.textContent, /Downloaded body/)
  assert.match(rendered.target.textContent, /viewer\.bodyTooLarge/)
  assert.match(rendered.target.textContent, /viewer\.failedToFetchBody/)
})

test('handles the no-selection state and keeps exported actions safe without a conversation', async () => {
  const onReply = vi.fn()
  const onActionComplete = vi.fn()
  const { instance, target } = await renderViewer({
    threadId: null,
    folderId: null,
    accountId: null,
    onReply,
    onActionComplete,
  })
  assert.match(target.textContent, /viewer\.selectConversation/)
  assert.equal(instance.getLastMessageId(), null)
  assert.equal(instance.getFocusedMessageId(), null)
  assert.equal(instance.hasFocusedMessage(), false)
  assert.equal(instance.isImagesLoaded('missing'), false)

  instance.selectAllText()
  await instance.refreshFlags()
  instance.reply()
  instance.replyAll()
  instance.forward()
  await instance.archive()
  await instance.spam()
  instance.toggleStar()
  instance.markRead()
  instance.markUnread()
  instance.trash()
  instance.deletePermanently()
  instance.openContextMenu()
  instance.scrollUp()
  instance.scrollDown()
  await flushAsync()

  assert.equal(onReply.mock.calls.length, 0)
  assert.equal(onActionComplete.mock.calls.length, 0)
  assert.equal(backend.GetConversation.mock.calls.length, 0)
  assert.equal(mailActions.archiveMessages.mock.calls.length, 0)
})

test('marks unread mail immediately, consumes its own read event, and closes on an external unread event', async () => {
  backend.GetMarkAsReadDelay.mockResolvedValue(0)
  let finishConversation
  backend.GetConversation.mockReturnValue(new Promise((resolve) => { finishConversation = resolve }))
  const { target } = await renderViewer()
  finishConversation(conversation([message('unread-now', { isRead: false })]))
  await flushAsync()
  assert.deepEqual(backend.MarkAsRead.mock.calls.at(-1), [['unread-now']])

  eventHandlers.get('messages:readChanged')({ messageIds: ['unread-now'], isRead: true })
  await flushAsync()
  assert.ok(titledButton(target, 'viewer.markAsUnread'))

  eventHandlers.get('messages:readChanged')({ messageIds: ['unread-now'], isRead: false })
  await flushAsync()
  assert.equal(target.querySelector('[data-message-id="unread-now"]'), null)
})

test('reports settings and immediate mark-as-read failures', async () => {
  vi.useFakeTimers()
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetReadReceiptResponsePolicy.mockRejectedValueOnce(new Error('settings unavailable'))
  backend.GetMarkAsReadDelay.mockResolvedValueOnce(1000)
  backend.GetConversation.mockResolvedValue(conversation([
    message('mark-failure', { isRead: false }),
  ]))
  backend.MarkAsRead.mockRejectedValueOnce(new Error('mark unavailable'))
  await renderViewer()
  await flushAsync()
  await vi.advanceTimersByTimeAsync(1000)
  await flushAsync()
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to load settings:'))
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to mark messages as read:'))
})

test('defers refresh while a dialog is active and handles partial deletion, empty refresh, and refresh errors', async () => {
  vi.useFakeTimers()
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const onActionComplete = vi.fn()
  backend.GetConversation.mockResolvedValue(conversation([
    message('one'),
    message('two'),
  ]))
  const { target } = await renderViewer({ onActionComplete })
  const initialLoads = backend.GetConversation.mock.calls.length

  dialogGuard.active = true
  eventHandlers.get('messages:updated')({ accountId: 'account-1', folderId: 'inbox-1' })
  await vi.advanceTimersByTimeAsync(500)
  assert.equal(backend.GetConversation.mock.calls.length, initialLoads)

  dialogGuard.active = false
  for (const listener of dialogGuard.listeners) listener(false)
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.equal(backend.GetConversation.mock.calls.length, initialLoads + 1)

  backend.GetConversation.mockResolvedValueOnce(conversation([message('two')]))
  await eventHandlers.get('messages:deleted')(['one'])
  await flushAsync()
  assert.equal(target.querySelector('[data-message-id="one"]'), null)
  assert.ok(target.querySelector('[data-message-id="two"]'))

  backend.GetConversation.mockResolvedValueOnce(conversation([]))
  eventHandlers.get('folder:synced')({ accountId: 'account-1', folderId: 'inbox-1' })
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.deepEqual(onActionComplete.mock.calls.at(-1), [true])

  backend.GetConversation.mockRejectedValueOnce(new Error('refresh unavailable'))
  eventHandlers.get('sent:synced')({ accountId: 'account-1' })
  await vi.advanceTimersByTimeAsync(310)
  await flushAsync()
  assert.ok(error.mock.calls.some(([message]) => message === 'Failed to refresh conversation:'))
})
