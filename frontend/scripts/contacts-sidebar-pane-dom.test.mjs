// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const accountStore = vi.hoisted(() => ({ loading: false, accounts: [], load: vi.fn() }))
const contacts = vi.hoisted(() => ({
  view: { selectedSourceId: '', selectedContactId: 'contact-1', detail: { id: 'contact-1', name: 'Ada' } },
  selectSource: vi.fn(), reload: vi.fn(), activate: vi.fn(),
}))
const groups = vi.hoisted(() => ({ state: { groups: [], loaded: true, loading: false }, load: vi.fn() }))
const refresh = vi.hoisted(() => ({
  state: { active: false }, begin: vi.fn(), complete: vi.fn(), fail: vi.fn(), init: vi.fn(),
}))
const backend = vi.hoisted(() => ({ RefreshContactsFromMail: vi.fn() }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const keyboard = vi.hoisted(() => ({ focused: 'sidebar', nav: {}, set: vi.fn() }))
const layout = vi.hoisted(() => ({ narrow: false, view: 'default', hideSidebar: vi.fn() }))
const pane = vi.hoisted(() => ({ shortcuts: [], events: [], unsub: vi.fn() }))

vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$contacts/stores/contactsView.svelte', () => ({
  contactsView: contacts.view,
  selectSource: contacts.selectSource,
  reloadContacts: contacts.reload,
  activateContact: contacts.activate,
}))
vi.mock('$contacts/stores/contactAccountGroups.svelte', () => ({
  contactAccountGroups: groups.state,
  loadContactAccountGroups: groups.load,
}))
vi.mock('$contacts/stores/contactRefresh.svelte', () => ({
  contactRefresh: refresh.state,
  beginContactRefresh: refresh.begin,
  completeContactRefresh: refresh.complete,
  failContactRefresh: refresh.fail,
  initContactRefreshEvents: refresh.init,
}))
vi.mock('$wailsjs/go/app/App', () => backend)
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  setFocusedPane: (slot) => { keyboard.focused = slot; keyboard.set(slot) },
  getFocusedPane: () => keyboard.focused,
  isMainKeyboardScope: () => true,
  registerPaneNav: (slot, handlers) => {
    keyboard.nav[slot] = handlers
    return () => { delete keyboard.nav[slot] }
  },
}))
vi.mock('$lib/stores/settings.svelte', () => ({ getEnhancedKeyboardNavigation: () => true }))
vi.mock('$lib/stores/layout.svelte', () => ({
  getLayoutMode: () => layout.narrow ? 'narrow' : 'wide',
  getResponsiveView: () => layout.view,
  hideSidebar: layout.hideSidebar,
}))
vi.mock('$lib/stores/uiState.svelte', () => ({
  getUIState: () => ({ sidebarWidth: 240 }), getUIStateVersion: () => 1,
}))
vi.mock('$lib/stores/paneShortcuts.svelte', () => ({
  registerPaneShortcut: (paneId, predicate, callback) => {
    const unregister = vi.fn()
    pane.shortcuts.push({ paneId, predicate, callback, unregister })
    return unregister
  },
}))
vi.mock('$wailsjs/runtime/runtime', () => ({
  EventsOn: (name, callback) => {
    pane.events.push({ name, callback })
    return pane.unsub
  },
}))
vi.mock('svelte-i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))

import ContactsSidebar from '../src/lib/contacts/components/ContactsSidebar.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderSidebar(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(ContactsSidebar, { target, props: { onSelect: vi.fn(), ...props } })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function key(target, value, options = {}) {
  target.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true, ...options }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  accountStore.loading = false
  accountStore.accounts = [
    { account: { id: 'account-1', email: 'first@example.test', name: 'First', color: '#336699' } },
    { account: { id: 'account-2', email: '', name: 'Second', orderIndex: 1 } },
  ]
  accountStore.load.mockReset().mockResolvedValue(undefined)
  contacts.view.selectedSourceId = ''
  contacts.view.selectedContactId = 'contact-1'
  contacts.view.detail = { id: 'contact-1', name: 'Ada' }
  contacts.selectSource.mockReset()
  contacts.reload.mockReset().mockResolvedValue(undefined)
  contacts.activate.mockReset().mockResolvedValue(undefined)
  groups.state.groups = [{ accountId: 'account-1', email: 'group@example.test', name: 'Group', count: 7 }]
  groups.state.loaded = true
  groups.state.loading = false
  groups.load.mockReset().mockResolvedValue(undefined)
  refresh.state.active = false
  for (const fn of [refresh.begin, refresh.complete, refresh.fail, refresh.init]) fn.mockReset()
  backend.RefreshContactsFromMail.mockReset().mockResolvedValue(3)
  toast.success.mockReset()
  toast.error.mockReset()
  keyboard.focused = 'sidebar'
  keyboard.nav = {}
  keyboard.set.mockReset()
  layout.narrow = false
  layout.view = 'default'
  layout.hideSidebar.mockReset()
  pane.shortcuts = []
  pane.events = []
  pane.unsub.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('ContactsSidebar loads groups, selects all/account sources, and completes refresh', async () => {
  const onSelect = vi.fn()
  const { target } = await renderSidebar({ onSelect })
  assert.equal(refresh.init.mock.calls.length, 1)
  assert.equal(groups.load.mock.calls.length, 1)
  assert.match(target.textContent, /group@example\.test/)
  assert.match(target.textContent, /7/)

  target.querySelector('[role="option"]').click()
  assert.deepEqual(contacts.selectSource.mock.calls.at(-1), ['account:account-1'])
  assert.equal(onSelect.mock.calls.length, 1)
  target.querySelector('[data-source-sidebar-header-action="all"]').click()
  assert.deepEqual(contacts.selectSource.mock.calls.at(-1), [''])

  target.querySelector('[data-source-sidebar-header-action="refresh"]').click()
  await flushAsync()
  assert.equal(refresh.begin.mock.calls.length, 1)
  assert.deepEqual(refresh.complete.mock.calls[0], [3, 3])
  assert.equal(contacts.reload.mock.calls.length, 1)
  assert.ok(groups.load.mock.calls.some((call) => call[0]?.force === true))
  assert.match(toast.success.mock.calls[0][0], /contacts\.toast\.refreshed/)
})

test('SourceSidebar supports header-action and item keyboard navigation', async () => {
  const onSelect = vi.fn()
  const { target } = await renderSidebar({ onSelect })
  const aside = target.querySelector('aside')
  target.querySelector('[data-source-sidebar-header-action="all"]').click()
  await flushAsync()
  key(aside, 'ArrowRight')
  await flushAsync()
  assert.equal(target.querySelector('[data-source-sidebar-header-action="refresh"]').dataset.keyboardSelected, 'true')
  key(aside, 'Enter')
  await flushAsync()
  assert.equal(backend.RefreshContactsFromMail.mock.calls.length, 1)

  key(aside, 'ArrowDown')
  assert.deepEqual(contacts.selectSource.mock.calls.at(-1), ['account:account-1'])
  key(aside, 'Enter')
  key(aside, 'ArrowUp')
  target.querySelector('[data-source-sidebar-header-action="refresh"]').click()
  await flushAsync()
  key(aside, 'ArrowLeft')
  await flushAsync()
  assert.equal(target.querySelector('[data-source-sidebar-header-action="all"]').dataset.keyboardSelected, 'true')

  keyboard.nav.sidebar.navigateNext()
  keyboard.nav.sidebar.activate()
  assert.ok(contacts.selectSource.mock.calls.length >= 2)
})

test('ContactsSidebar reports refresh failures and loads accounts when none exist', async () => {
  backend.RefreshContactsFromMail.mockRejectedValueOnce(new Error('refresh unavailable'))
  accountStore.accounts = []
  const { target } = await renderSidebar()
  assert.equal(accountStore.load.mock.calls.length, 1)
  target.querySelector('[data-source-sidebar-header-action="refresh"]').click()
  await flushAsync()
  assert.equal(refresh.fail.mock.calls.length, 1)
  assert.deepEqual(toast.error.mock.calls.at(-1), ['refresh unavailable'])
})

test('ContactsSidebar falls back to account data, retries empty groups, and renders narrow back UI', async () => {
  groups.state.groups = []
  accountStore.accounts = [{ account: { id: 'account-2', email: '', name: 'Second', orderIndex: 1 } }]
  layout.narrow = true
  layout.view = 'sidebar'
  const { target } = await renderSidebar()
  assert.match(target.textContent, /Second/)
  assert.ok(groups.load.mock.calls.some((call) => call[0]?.force === true))
  assert.equal(target.querySelector('aside').dataset.keyboardRegionVisible, 'true')
  target.querySelector('button[aria-label="common.back"]').click()
  assert.equal(layout.hideSidebar.mock.calls.length, 1)
})

test('SourceSidebar ignores composing keys, handles mouse focus, and wraps navigation', async () => {
  const { target } = await renderSidebar()
  const aside = target.querySelector('aside')
  key(aside, 'ArrowDown', { isComposing: true })
  assert.equal(contacts.selectSource.mock.calls.length, 0)
  aside.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
  assert.equal(document.activeElement, aside)
  key(aside, 'ArrowUp')
  key(aside, ' ')
  assert.ok(contacts.selectSource.mock.calls.length >= 1)
})
