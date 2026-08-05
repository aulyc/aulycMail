// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  SearchMailInAccount: vi.fn(),
  Contacts_ListContactsForBrowse: vi.fn(),
}))
const accountStore = vi.hoisted(() => ({
  accounts: [
    { account: { id: 'account-1', email: 'first@example.test', name: 'First' } },
    { account: { id: 'account-2', email: '', name: 'Second' } },
    { account: { id: '', email: 'ignored@example.test', name: 'Ignored' } },
  ],
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$lib/utils/date', () => ({ formatRelativeDateTime: () => 'relative-date' }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))

import SearchOverlay from '../src/lib/components/SearchOverlay.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderOverlay(props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(SearchOverlay, { target, props: { open: true, ...props } })
  mounted.push(instance)
  await flushAsync()
  return { target, instance, input: target.querySelector('input') }
}

function inputText(input, value) {
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
}

function key(input, value, options = {}) {
  input.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true, ...options }))
}

async function runDebounce() {
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
}

beforeEach(() => {
  document.body.innerHTML = ''
  vi.useFakeTimers()
  backend.SearchMailInAccount.mockReset().mockResolvedValue([])
  backend.Contacts_ListContactsForBrowse.mockReset().mockResolvedValue([])
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('searches mail by scope and supports keyboard selection, select-all, and close', async () => {
  const onClose = vi.fn()
  const onSelectMail = vi.fn()
  const results = [
    {
      id: 'mail-1', threadId: 'thread-1', accountId: 'account-1', folderId: 'inbox',
      subject: '', fromName: '', fromEmail: 'sender@example.test', date: '', incoming: true,
      isRead: false, snippet: '',
    },
    {
      id: 'mail-2', threadId: 'thread-2', accountId: 'account-1', folderId: 'inbox',
      subject: 'Second result', fromName: 'Synthetic Sender', fromEmail: 'sender@example.test',
      date: '2026-08-01T09:00:00Z', incoming: false, isRead: true, snippet: 'Preview',
    },
  ]
  backend.SearchMailInAccount.mockResolvedValue(results)
  const { target, input } = await renderOverlay({ mode: 'mail', onClose, onSelectMail })

  assert.equal(input.placeholder, 'search.overlayMail')
  inputText(input, '  project  ')
  await runDebounce()
  assert.deepEqual(backend.SearchMailInAccount.mock.calls.at(-1), ['', 'project', 50])
  assert.match(target.textContent, /viewer\.noSubject/)
  assert.match(target.textContent, /Second result/)

  key(input, 'ArrowDown')
  await flushAsync()
  key(input, 'Enter')
  assert.deepEqual(onSelectMail.mock.calls[0], [results[1]])
  assert.equal(onClose.mock.calls.length, 1)

  input.focus()
  input.setSelectionRange(0, 0)
  key(input, 'a', { metaKey: true })
  assert.equal(input.selectionStart, 0)
  assert.equal(input.selectionEnd, input.value.length)

  key(input, 'Escape')
  assert.equal(onClose.mock.calls.length, 2)

  target.querySelector('button[aria-label="Close"]').click()
  assert.equal(onClose.mock.calls.length, 3)
})

test('defers IME input, wraps scopes with Tab, and ignores stale search results', async () => {
  const first = Promise.withResolvers()
  const second = Promise.withResolvers()
  backend.SearchMailInAccount.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
  const { target, input } = await renderOverlay({ mode: 'mail', onClose: vi.fn() })

  input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
  inputText(input, 'pin')
  await runDebounce()
  assert.equal(backend.SearchMailInAccount.mock.calls.length, 0)
  input.value = '项目'
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertCompositionText' }))
  input.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }))
  await runDebounce()
  assert.deepEqual(backend.SearchMailInAccount.mock.calls[0], ['', '项目', 50])

  key(input, 'Tab')
  await flushAsync()
  assert.deepEqual(backend.SearchMailInAccount.mock.calls[1], ['account-1', '项目', 50])
  assert.equal(target.querySelector('button[aria-pressed="true"]').textContent.trim(), 'first@example.test')

  second.resolve([{
    id: 'fresh', threadId: 'fresh', accountId: 'account-1', folderId: 'inbox', subject: 'Fresh',
    fromName: '', fromEmail: '', date: '', incoming: true, isRead: false, snippet: '',
  }])
  await flushAsync()
  first.resolve([{
    id: 'stale', threadId: 'stale', accountId: 'account-1', folderId: 'inbox', subject: 'Stale',
    fromName: '', fromEmail: '', date: '', incoming: true, isRead: false, snippet: '',
  }])
  await flushAsync()
  assert.match(target.textContent, /Fresh/)
  assert.doesNotMatch(target.textContent, /Stale/)

  key(input, 'Tab', { shiftKey: true })
  await flushAsync()
  assert.equal(target.querySelector('button[aria-pressed="true"]').textContent.trim(), 'search.scopeAllMail')
})

test('searches and selects contacts, clears pointer highlight, and reports failures as empty results', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const onClose = vi.fn()
  const onSelectContact = vi.fn()
  const contacts = [
    { id: 'contact-1', name: 'Ada', emails: ['ada@example.test'] },
    { id: 'contact-2', name: '', emails: [] },
  ]
  backend.Contacts_ListContactsForBrowse.mockResolvedValueOnce(contacts)
  const { target, input } = await renderOverlay({ mode: 'contacts', onClose, onSelectContact })

  inputText(input, 'ada')
  await runDebounce()
  assert.deepEqual(backend.Contacts_ListContactsForBrowse.mock.calls[0], ['ada', '', 50, 0])
  assert.match(target.textContent, /Ada/)
  assert.match(target.textContent, /ada@example\.test/)
  assert.match(target.textContent, /contacts\.common\.unnamed/)

  const resultButtons = [...target.querySelectorAll('button')].filter((button) => button.textContent.includes('Ada'))
  assert.ok(resultButtons.length > 0)
  resultButtons.at(-1).dispatchEvent(new MouseEvent('mousemove', { bubbles: true, movementX: 2 }))
  target.querySelector('[style*="max-height"]').dispatchEvent(new WheelEvent('wheel', { bubbles: true }))
  resultButtons.at(-1).click()
  assert.deepEqual(onSelectContact.mock.calls[0], [contacts[0]])
  assert.equal(onClose.mock.calls.length, 1)

  backend.Contacts_ListContactsForBrowse.mockRejectedValueOnce(new Error('search unavailable'))
  inputText(input, 'missing')
  await runDebounce()
  assert.match(target.textContent, /search\.overlayNoResults/)
  assert.equal(error.mock.calls.length, 1)

  inputText(input, '   ')
  await runDebounce()
  assert.doesNotMatch(target.textContent, /search\.overlayNoResults/)
})
