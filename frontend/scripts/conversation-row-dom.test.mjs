// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({ Star: vi.fn(), Unstar: vi.fn() }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const settings = vi.hoisted(() => ({ accentUnread: vi.fn(() => false) }))

vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/IconStub.svelte')).default }))
vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/settings.svelte', () => ({ getAccentBarUnread: settings.accentUnread }))
vi.mock('$lib/utils/date', () => ({ formatRelativeDate: () => 'RELATIVE-DATE' }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))

import ConversationRow from '../src/lib/components/list/ConversationRow.svelte'

const mounted = []

function conversation(overrides = {}) {
  return {
    id: 'thread-1',
    participants: [{ name: 'Alice', email: 'alice@example.test' }],
    unreadCount: 0,
    messageCount: 1,
    latestDate: '2026-08-01T12:00:00Z',
    subject: 'Quarterly report',
    snippet: 'Synthetic preview',
    isStarred: false,
    hasAttachments: false,
    isEncrypted: false,
    messageIds: ['message-1'],
    ...overrides,
  }
}

async function renderRow(overrides = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const props = {
    conversation: conversation(),
    selected: false,
    checked: false,
    accountId: 'account-1',
    folderId: 'inbox-1',
    selectedMessageIds: [],
    onSelect: vi.fn(),
    ...overrides,
  }
  const instance = mount(ConversationRow, { target, props })
  mounted.push(instance)
  await tick()
  return { target, props, row: target.querySelector('[data-conversation-row]') }
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.Star.mockReset().mockResolvedValue(undefined)
  backend.Unstar.mockReset().mockResolvedValue(undefined)
  toast.success.mockReset()
  toast.error.mockReset()
  settings.accentUnread.mockReset().mockReturnValue(false)
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('renders participant, density, selection, search, compose, and body-state variants', async () => {
  settings.accentUnread.mockReturnValue(true)
  const unknown = await renderRow({
    density: 'micro',
    selected: true,
    instantSelection: true,
    conversation: conversation({ participants: [], unreadCount: 2, subject: '', snippet: '', messageCount: 3 }),
  })
  assert.match(unknown.row.textContent, /viewer\.unknown/)
  assert.match(unknown.row.textContent, /viewer\.noSubject/)
  assert.match(unknown.row.textContent, /messageList\.noContent/)
  assert.match(unknown.row.className, /bg-primary\/20/)
  assert.match(unknown.row.className, /transition-none/)
  assert.match(unknown.row.className, /border-l-primary/)
  assert.match(unknown.row.getAttribute('style'), /height: 66px/)

  const twoPeople = await renderRow({
    density: 'compact',
    current: true,
    conversation: conversation({
      participants: [{ name: '', email: 'alice@example.test' }, { name: 'Bob', email: 'bob@example.test' }],
      isEncrypted: true,
      snippet: '',
    }),
  })
  assert.match(twoPeople.row.textContent, /alice, Bob/)
  assert.match(twoPeople.row.textContent, /messageList\.encryptedContent/)
  assert.match(twoPeople.row.className, /bg-primary\/10/)
  assert.match(twoPeople.row.getAttribute('style'), /height: 80px/)

  const manyPeople = await renderRow({
    showAccountIndicator: true,
    accountColor: '',
    conversation: conversation({
      participants: [
        { name: 'Alice', email: 'alice@example.test' },
        { name: 'Bob', email: 'bob@example.test' },
        { name: 'Carol', email: 'carol@example.test' },
      ],
      messageIds: null,
      messages: null,
      messageCount: 2,
    }),
  })
  assert.match(manyPeople.row.textContent, /Alice, Bob \+1/)
  assert.doesNotMatch(manyPeople.row.innerHTML, /background-color:/)

  const highlighted = await renderRow({
    density: 'large',
    showAccountIndicator: true,
    accountColor: '#123456',
    accountName: 'Primary',
    highlightedFromName: '<mark>Alice</mark>',
    highlightedSubject: '<mark>Quarterly</mark>',
    highlightedSnippet: '<mark>preview</mark>',
    searchFolderName: 'Archive',
    isNonLocal: true,
    conversation: conversation({
      participants: [
        { name: 'Alice', email: 'alice@example.test' },
        { name: 'Bob', email: 'bob@example.test' },
        { name: 'Carol', email: 'carol@example.test' },
      ],
      unreadCount: 1,
      hasAttachments: true,
      composeStatus: 'sent',
      composeAction: 'reply-all',
    }),
  })
  assert.match(highlighted.row.innerHTML, /<mark>Alice<\/mark>/)
  assert.match(highlighted.row.innerHTML, /<mark>Quarterly<\/mark>/)
  assert.match(highlighted.row.innerHTML, /Archive/)
  assert.match(highlighted.row.innerHTML, /background-color: #123456/)
  assert.match(highlighted.row.innerHTML, /messageList\.composeStatusSentReplyAll/)
  assert.match(highlighted.row.getAttribute('style'), /height: 120px/)
  const highlightedRead = await renderRow({
    highlightedFromName: '<mark>Read sender</mark>',
    highlightedSubject: '<mark>Read subject</mark>',
    conversation: conversation({ unreadCount: 0 }),
  })
  assert.doesNotMatch(highlightedRead.row.innerHTML, /font-semibold/)

  for (const [status, action, title] of [
    ['draft', '', 'messageList.composeStatusDraftReply'],
    ['sent', 'reply', 'messageList.composeStatusSentReply'],
    ['sent', 'forward', 'messageList.composeStatusSentForward'],
    ['sent', 'unknown', ''],
  ]) {
    const variant = await renderRow({
      density: 'standard',
      conversation: conversation({ composeStatus: status, composeAction: action }),
    })
    if (title) assert.match(variant.row.innerHTML, new RegExp(title))
    else assert.doesNotMatch(variant.row.innerHTML, /composeStatus/)
  }
})

test('stars and unstars only the row messages while preserving selection behavior', async () => {
  const complete = vi.fn()
  const select = vi.fn()
  const starred = await renderRow({ onSelect: select, onActionComplete: complete })
  starred.target.querySelector('button').click()
  await tick()
  assert.deepEqual(backend.Star.mock.calls[0], [['message-1']])
  assert.equal(toast.success.mock.calls[0][0], 'toast.starred')
  assert.equal(complete.mock.calls.length, 1)
  assert.equal(select.mock.calls.length, 0)

  const unstarred = await renderRow({
    conversation: conversation({ isStarred: true, messageIds: null, messages: [{ id: 'fallback-message' }] }),
  })
  unstarred.target.querySelector('button').click()
  await tick()
  assert.deepEqual(backend.Unstar.mock.calls[0], [['fallback-message']])
  assert.equal(toast.success.mock.calls.at(-1)[0], 'toast.starRemoved')

  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.Star.mockRejectedValueOnce(new Error('star unavailable'))
  const failed = await renderRow({ conversation: conversation({ messageIds: [] }) })
  failed.target.querySelector('button').click()
  await tick()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.failedToUpdateStar')
  assert.equal(errorSpy.mock.calls.length, 1)
})

test('routes pointer, keyboard, draft, context, and drag interactions with safe guards', async () => {
  const onSelect = vi.fn()
  const onPointerMove = vi.fn()
  const onContextMenu = vi.fn()
  const onOpenDraft = vi.fn()
  const rendered = await renderRow({
    checked: true,
    selectedMessageIds: ['checked-1', 'checked-2'],
    onSelect,
    onPointerMove,
    onContextMenu,
    onOpenDraft,
    rowIndex: 4,
  })
  rendered.row.click()
  rendered.row.dispatchEvent(new PointerEvent('pointermove', { bubbles: true }))
  rendered.row.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
  rendered.row.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }))
  rendered.row.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  rendered.row.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
  rendered.row.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  assert.equal(onSelect.mock.calls.length, 3)
  assert.equal(onPointerMove.mock.calls.length, 1)
  assert.equal(onContextMenu.mock.calls.length, 1)
  assert.equal(onOpenDraft.mock.calls.length, 1)
  assert.equal(rendered.row.dataset.rowIndex, '4')

  const transfer = { setData: vi.fn(), effectAllowed: '' }
  const drag = new Event('dragstart', { bubbles: true })
  Object.defineProperty(drag, 'dataTransfer', { value: transfer })
  rendered.row.dispatchEvent(drag)
  assert.equal(transfer.effectAllowed, 'move')
  assert.deepEqual(JSON.parse(transfer.setData.mock.calls[0][1]), {
    messageIds: ['checked-1', 'checked-2'],
    sourceAccountId: 'account-1',
  })

  rendered.row.dispatchEvent(new Event('dragstart', { bubbles: true }))
  const empty = await renderRow({ conversation: conversation({ messageIds: [] }) })
  const emptyTransfer = { setData: vi.fn(), effectAllowed: '' }
  const emptyDrag = new Event('dragstart', { bubbles: true })
  Object.defineProperty(emptyDrag, 'dataTransfer', { value: emptyTransfer })
  empty.row.dispatchEvent(emptyDrag)
  assert.equal(emptyTransfer.setData.mock.calls.length, 0)
})
