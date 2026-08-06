// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const contacts = vi.hoisted(() => ({
  view: {
    contacts: [], selectedContactId: null, selectedContactScrollTopSignal: 0,
    selectedSourceId: '', listResetSignal: 0, sortOrder: 'name-asc', total: 0,
    hasMore: false, loading: false, loadingMore: false, loadError: null,
    detail: null, detailLoading: false, detailLoadError: null,
  },
  reload: vi.fn(), loadMore: vi.fn(), focus: vi.fn(), activate: vi.fn(),
  setSearch: vi.fn(), setSort: vi.fn(), deleteLocal: vi.fn(),
}))
const keyboard = vi.hoisted(() => ({
  focused: 'messageList', nav: {}, set: vi.fn(), focus: vi.fn(),
}))
const layout = vi.hoisted(() => ({ responsive: false, view: 'default', hideViewer: vi.fn(), hideSidebar: vi.fn() }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const backend = vi.hoisted(() => ({ GetContactMessages: vi.fn() }))
const runtime = vi.hoisted(() => ({ EventsEmit: vi.fn() }))

vi.mock('$contacts/stores/contactsView.svelte', () => ({
  contactsView: contacts.view,
  reloadContacts: contacts.reload,
  loadMoreContacts: contacts.loadMore,
  focusContact: contacts.focus,
  activateContact: contacts.activate,
  setSearchQuery: contacts.setSearch,
  setSortOrder: contacts.setSort,
  deleteLocalContact: contacts.deleteLocal,
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  setFocusedPane: (slot) => { keyboard.focused = slot; keyboard.set(slot) },
  getFocusedPane: () => keyboard.focused,
  isMainKeyboardScope: () => true,
  focusPane: keyboard.focus,
  registerPaneNav: (slot, handlers) => {
    keyboard.nav[slot] = handlers
    return () => { delete keyboard.nav[slot] }
  },
}))
vi.mock('$lib/stores/settings.svelte', () => ({ getEnhancedKeyboardNavigation: () => true }))
vi.mock('$lib/stores/layout.svelte', () => ({
  isResponsive: () => layout.responsive,
  getResponsiveView: () => layout.view,
  hideViewer: layout.hideViewer,
  getLayoutMode: () => layout.responsive ? 'narrow' : 'wide',
  hideSidebar: layout.hideSidebar,
}))
vi.mock('$lib/stores/uiState.svelte', () => ({
  getUIState: () => ({ listWidth: 360, sidebarWidth: 240 }),
  getUIStateVersion: () => 1,
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/utils/date', () => ({
  formatLocalDateTime: (value) => `local:${value}`,
  formatRelativeDate: () => 'relative-date',
}))
vi.mock('$wailsjs/go/app/App', () => backend)
vi.mock('$wailsjs/runtime/runtime', () => runtime)
vi.mock('svelte-i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/confirm-dialog/ConfirmDialog.svelte', async () => ({
  default: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))

import ContactList from '../src/lib/contacts/components/ContactList.svelte'
import ContactDetail from '../src/lib/contacts/components/ContactDetail.svelte'

const mounted = []

function listContact(id, name, email) {
  return { id, name, emails: email ? [email] : [] }
}

function fullContact(overrides = {}) {
  return {
    id: 'contact-1', name: 'Ada Lovelace', emails: ['ada@example.test'],
    emailItems: [{ email: 'ada@example.test', type: 'work' }],
    associatedAccounts: [{ accountId: 'account-1', email: 'mail@example.test', name: 'Mail' }],
    phones: [{ number: '+1-555-0100', type: 'mobile', isPrimary: true }],
    addresses: [{ type: 'home', street: '1 Test St', city: 'London', region: 'LDN', postcode: '10000', country: 'UK' }],
    org: 'Analytical Engines', title: 'Programmer', bday: '1815-12-10', nickname: 'Ada',
    urls: [{ url: 'https://example.test/ada', type: 'home' }],
    impps: [{ handle: 'ada-chat', type: 'matrix' }], categories: ['friend'], note: 'Synthetic note',
    updatedAt: '2026-08-01T09:00:00Z',
    ...overrides,
  }
}

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(Component, props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(Component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function key(target, value, options = {}) {
  target.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true, ...options }))
}

function namedButton(target, text) {
  return [...target.querySelectorAll('button')].find((button) => button.textContent.includes(text) || button.title.includes(text))
}

beforeEach(() => {
  document.body.innerHTML = ''
  vi.useFakeTimers()
  Element.prototype.scrollIntoView = vi.fn()
  Element.prototype.scrollBy = vi.fn()
  Object.assign(contacts.view, {
    contacts: [listContact('contact-1', 'Ada', 'ada@example.test'), listContact('contact-2', '', '')],
    selectedContactId: 'contact-1', selectedContactScrollTopSignal: 1,
    selectedSourceId: '', listResetSignal: 0, sortOrder: 'name-asc', total: 2,
    hasMore: true, loading: false, loadingMore: false, loadError: null,
    detail: fullContact(), detailLoading: false, detailLoadError: null,
  })
  for (const fn of [contacts.reload, contacts.loadMore, contacts.focus, contacts.activate, contacts.setSearch, contacts.setSort, contacts.deleteLocal]) {
    fn.mockReset().mockResolvedValue(undefined)
  }
  keyboard.focused = 'messageList'
  keyboard.nav = {}
  keyboard.set.mockReset()
  keyboard.focus.mockReset()
  layout.responsive = false
  layout.view = 'default'
  layout.hideViewer.mockReset()
  layout.hideSidebar.mockReset()
  toast.success.mockReset()
  toast.error.mockReset()
  backend.GetContactMessages.mockReset().mockResolvedValue([
    { id: 'message-1', threadId: 'thread-1', accountId: 'account-1', folderId: 'inbox',
      subject: 'Related message', accountEmail: 'mail@example.test', fromName: 'Ada', fromEmail: 'ada@example.test',
      date: '2026-08-01T09:00:00Z', incoming: true, isRead: false },
  ])
  runtime.EventsEmit.mockReset()
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  })
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('ContactList renders rows and drives list keyboard, sort, add, pagination, and deletion', async () => {
  const onAdd = vi.fn()
  const { target } = await render(ContactList, { onAdd })
  const listbox = target.querySelector('[role="listbox"]')
  assert.equal(target.querySelector('[role="region"]').style.width, '360px')
  assert.match(target.textContent, /Ada/)
  assert.match(target.textContent, /contacts\.common\.unnamed/)
  assert.equal(target.querySelector('[aria-selected="true"]').textContent.includes('Ada'), true)

  target.querySelector('[aria-selected="true"]').click()
  assert.deepEqual(contacts.activate.mock.calls.at(-1), ['contact-1'])
  key(listbox, 'ArrowDown')
  assert.deepEqual(contacts.focus.mock.calls.at(-1), ['contact-2'])
  key(listbox, 'Enter')
  assert.deepEqual(contacts.activate.mock.calls.at(-1), ['contact-1'])

  namedButton(target, 'contacts.list.sortAsc').click()
  assert.deepEqual(contacts.setSort.mock.calls.at(-1), ['name-desc'])
  assert.equal(contacts.reload.mock.calls.length, 1)
  namedButton(target, 'contacts.list.addTooltip').click()
  assert.equal(onAdd.mock.calls.length, 1)

  const scroller = listbox.firstElementChild
  Object.defineProperties(scroller, {
    scrollHeight: { configurable: true, value: 500 },
    scrollTop: { configurable: true, value: 410 },
    clientHeight: { configurable: true, value: 50 },
  })
  scroller.dispatchEvent(new Event('scroll', { bubbles: true }))
  assert.equal(contacts.loadMore.mock.calls.length, 1)

  key(listbox, 'Delete')
  await flushAsync()
  const confirm = target.querySelector('[data-confirm-dialog]')
  assert.ok(confirm)
  confirm.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(contacts.deleteLocal.mock.calls.at(-1), ['contact-1'])
  assert.equal(toast.success.mock.calls.at(-1)[0], 'contacts.toast.deleted')
})

test('ContactList owns debounced search focus and handles clear, Enter, Escape, and delete errors', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  contacts.deleteLocal.mockRejectedValueOnce(new Error('delete unavailable'))
  const { target } = await render(ContactList)
  keyboard.nav.messageList.focusSearch()
  await vi.advanceTimersByTimeAsync(60)
  await flushAsync()
  const input = target.querySelector('input')
  assert.equal(document.activeElement, input)
  input.value = 'ada'
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.deepEqual(contacts.setSearch.mock.calls.at(-1), ['ada'])
  assert.equal(contacts.reload.mock.calls.length, 1)

  key(input, 'Enter')
  await flushAsync()
  assert.deepEqual(keyboard.focus.mock.calls.at(-1), ['messageList'])
  keyboard.nav.messageList.focusSearch()
  assert.equal(document.activeElement, input)
  keyboard.nav.messageList.focusSearch()
  await flushAsync()
  assert.deepEqual(contacts.setSearch.mock.calls.at(-1), [''])

  keyboard.nav.messageList.focusSearch()
  await vi.advanceTimersByTimeAsync(60)
  input.value = 'again'
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  key(input, 'Escape')
  await flushAsync()
  assert.deepEqual(contacts.setSearch.mock.calls.at(-1), [''])

  const listbox = target.querySelector('[role="listbox"]')
  key(listbox, 'Backspace')
  await flushAsync()
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'contacts.toast.failedDelete')
  assert.equal(error.mock.calls.length, 1)
})

test('ContactList renders loading, retry, empty-search, and loading-more states', async () => {
  contacts.view.contacts = []
  contacts.view.loading = true
  let rendered = await render(ContactList)
  assert.match(rendered.target.textContent, /Loading…/)
  await unmount(mounted.pop())

  contacts.view.loading = false
  contacts.view.loadError = 'failed'
  rendered = await render(ContactList)
  assert.match(rendered.target.textContent, /contacts\.list\.loadFailed/)
  namedButton(rendered.target, 'contacts.list.retry').click()
  assert.equal(contacts.reload.mock.calls.length, 1)
  await unmount(mounted.pop())

  contacts.view.loadError = null
  contacts.view.loadingMore = true
  rendered = await render(ContactList)
  assert.match(rendered.target.textContent, /contacts\.list\.empty/)
  assert.match(rendered.target.textContent, /common\.loading/)
})

test('ContactList covers category labels, descending sort, region focus, and pagination guards', async () => {
  const sourceLabels = [
    ['', 'contacts.sidebar.all'],
    ['role:sender', 'contacts.sidebar.roleSender'],
    ['role:recipient', 'contacts.sidebar.roleRecipient'],
    ['role:cc', 'contacts.sidebar.roleCc'],
    ['role:bcc', 'contacts.sidebar.roleBcc'],
    ['local', 'contacts.sidebar.localAll'],
    ['local:manual', 'contacts.sidebar.localManual'],
    ['local:collected', 'contacts.sidebar.localCollected'],
    ['unknown-source', 'contacts.list.header'],
  ]
  for (const [sourceID, label] of sourceLabels) {
    contacts.view.selectedSourceId = sourceID
    const rendered = await render(ContactList)
    assert.equal(rendered.target.querySelector('[role="region"]').getAttribute('aria-label'), label)
    await unmount(mounted.pop())
  }

  contacts.view.selectedSourceId = ''
  contacts.view.sortOrder = 'name-desc'
  contacts.view.contacts = [
    listContact('contact-email-only', '', 'only@example.test'),
    listContact('contact-named', 'Named', 'named@example.test'),
  ]
  const { target } = await render(ContactList)
  assert.equal(namedButton(target, 'contacts.list.addTooltip'), undefined)
  namedButton(target, 'contacts.list.sortDesc').click()
  assert.deepEqual(contacts.setSort.mock.calls.at(-1), ['name-asc'])
  assert.match(target.textContent, /only@example\.test/)

  const region = target.querySelector('[role="region"]')
  const focusTarget = region.querySelector('[data-keyboard-region-focus-target]')
  assert.ok(focusTarget)
  region.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  assert.equal(document.activeElement, focusTarget)
  assert.deepEqual(keyboard.set.mock.calls.at(-1), ['messageList'])

  const scroller = target.querySelector('[role="listbox"]').firstElementChild
  Object.defineProperties(scroller, {
    scrollHeight: { configurable: true, value: 500 },
    scrollTop: { configurable: true, value: 480 },
    clientHeight: { configurable: true, value: 50 },
  })
  contacts.view.hasMore = false
  scroller.dispatchEvent(new Event('scroll', { bubbles: true }))
  contacts.view.hasMore = true
  contacts.view.loading = true
  scroller.dispatchEvent(new Event('scroll', { bubbles: true }))
  contacts.view.loading = false
  contacts.view.loadingMore = true
  scroller.dispatchEvent(new Event('scroll', { bubbles: true }))
  assert.equal(contacts.loadMore.mock.calls.length, 0)
})

test('ContactDetail renders all contact fields, copies email, opens mail, edits, and deletes', async () => {
  const onEdit = vi.fn()
  const { target } = await render(ContactDetail, { onEdit })
  assert.deepEqual(backend.GetContactMessages.mock.calls[0], ['ada@example.test', 50])
  assert.match(target.textContent, /Ada Lovelace/)
  for (const text of ['Analytical Engines', 'Programmer', '1815-12-10', 'ada-chat', 'Synthetic note', 'mail@example.test', 'Related message']) {
    assert.match(target.textContent, new RegExp(text))
  }

  const email = target.querySelector('[data-keyboard-action-context="ada@example.test"]')
  email.click()
  await flushAsync()
  assert.deepEqual(navigator.clipboard.writeText.mock.calls.at(-1), ['ada@example.test'])
  assert.match(toast.success.mock.calls.at(-1)[0], /contacts\.toast\.emailCopied/)
  key(email, ' ')
  await flushAsync()
  assert.equal(navigator.clipboard.writeText.mock.calls.length, 2)

  namedButton(target, 'Related message').click()
  assert.deepEqual(runtime.EventsEmit.mock.calls[0], ['mail:openConversation', {
    accountId: 'account-1', folderId: 'inbox', threadId: 'thread-1',
  }])
  namedButton(target, 'contacts.detail.edit').click()
  assert.deepEqual(onEdit.mock.calls[0], [contacts.view.detail])

  namedButton(target, 'contacts.common.delete').click()
  await flushAsync()
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.deepEqual(contacts.deleteLocal.mock.calls.at(-1), ['contact-1'])
  assert.equal(toast.success.mock.calls.at(-1)[0], 'contacts.toast.deleted')

  const section = target.querySelector('section')
  section.focus()
  key(section, 'ArrowDown')
  assert.ok(Element.prototype.scrollBy.mock.calls.length > 0)
})

test('ContactDetail handles copy, related-mail, and deletion failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  navigator.clipboard.writeText.mockRejectedValueOnce(new Error('clipboard unavailable'))
  backend.GetContactMessages.mockRejectedValueOnce(new Error('messages unavailable'))
  contacts.deleteLocal.mockRejectedValueOnce(new Error('delete unavailable'))
  const { target } = await render(ContactDetail)
  target.querySelector('[data-keyboard-action-context]').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls[0][0], 'contacts.toast.emailCopyFailed')
  assert.match(target.textContent, /contacts\.detail\.relatedMailEmpty/)

  namedButton(target, 'contacts.common.delete').click()
  await flushAsync()
  target.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'contacts.toast.failedDelete')
  assert.equal(error.mock.calls.length, 2)
})

test('ContactDetail renders empty loading/error/retry and responsive back states', async () => {
  contacts.view.detail = null
  contacts.view.detailLoading = true
  let rendered = await render(ContactDetail)
  assert.match(rendered.target.textContent, /contacts\.detail\.loading/)
  await unmount(mounted.pop())

  contacts.view.detailLoading = false
  contacts.view.detailLoadError = 'failed'
  contacts.view.selectedContactId = 'contact-missing'
  rendered = await render(ContactDetail)
  assert.match(rendered.target.textContent, /contacts\.detail\.loadFailed/)
  namedButton(rendered.target, 'contacts.detail.retry').click()
  assert.deepEqual(contacts.focus.mock.calls.at(-1), ['contact-missing'])
  await unmount(mounted.pop())

  contacts.view.detailLoadError = null
  layout.responsive = true
  layout.view = 'viewer'
  rendered = await render(ContactDetail)
  assert.ok(rendered.target.querySelector('button[aria-label="common.back"]'))
  rendered.target.querySelector('button[aria-label="common.back"]').click()
  assert.equal(layout.hideViewer.mock.calls.length, 1)
  assert.match(rendered.target.textContent, /contacts\.detail\.emptyState/)
})
