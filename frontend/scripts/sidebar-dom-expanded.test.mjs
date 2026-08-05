// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({ MoveToFolder: vi.fn(), Undo: vi.fn() }))
const accountStore = vi.hoisted(() => ({
  accounts: [],
  loading: false,
  isAnySyncing: false,
  selectedFolder: null,
  selectFolder: vi.fn(),
  syncAllComplete: vi.fn(),
  cancelAllSyncs: vi.fn(),
}))
const ui = vi.hoisted(() => ({
  version: 1,
  expanded: true,
  state: { collapsedFolders: {} },
  setAccountExpanded: vi.fn(),
  setFolderCollapsed: vi.fn(),
  saveUIState: vi.fn(),
}))
const keyboard = vi.hoisted(() => ({ setFocusedPane: vi.fn() }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$lib/stores/uiState.svelte', () => ({
  isAccountExpanded: () => ui.expanded,
  setAccountExpanded: ui.setAccountExpanded,
  setFolderCollapsed: ui.setFolderCollapsed,
  getUIState: () => ui.state,
  getUIStateVersion: () => ui.version,
  saveUIState: ui.saveUIState,
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({ setFocusedPane: keyboard.setFocusedPane }))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('../src/lib/components/sidebar/FolderContextMenu.svelte', async () => ({
  default: (await import('./fixtures/SnippetTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/settings/AccountDialog.svelte', async () => ({
  default: (await import('./fixtures/AccountDialogTestStub.svelte')).default,
}))

import Sidebar from '../src/lib/components/sidebar/Sidebar.svelte'

const mounted = []

function folder(id, name, type = 'folder', overrides = {}) {
  return { id, name, path: name, type, unreadCount: 0, totalCount: 0, noSelect: false, ...overrides }
}

function accountFixture() {
  return {
    account: { id: 'account-1', name: 'Synthetic Account', email: 'mail@example.test', color: '#336699' },
    loading: false,
    syncing: false,
    error: null,
    folders: [
      { folder: folder('inbox', 'Inbox', 'inbox', { unreadCount: 2 }), children: [] },
      {
        folder: folder('group', 'Other folders', 'folder', { noSelect: true }),
        children: [
          { folder: folder('archive', 'Archive', 'archive', { unreadCount: 3 }), children: [] },
          { folder: folder('drafts', 'Drafts', 'drafts', { totalCount: 4 }), children: [] },
        ],
      },
      { folder: folder('spam', 'Spam', 'spam', { unreadCount: 5 }), children: [] },
    ],
  }
}

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderSidebar(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(Sidebar, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function buttonFor(target, selector, text) {
  return [...target.querySelectorAll(selector)].find((button) => button.textContent.includes(text))
}

function dropEvent(raw, types = ['application/x-aulycmail-messages']) {
  const event = new Event('drop', { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'dataTransfer', {
    value: {
      types,
      files: [],
      dropEffect: 'none',
      getData: () => raw,
    },
  })
  return event
}

beforeEach(() => {
  document.body.innerHTML = ''
  Element.prototype.scrollIntoView = vi.fn()
  accountStore.accounts = [accountFixture()]
  accountStore.loading = false
  accountStore.isAnySyncing = false
  accountStore.selectedFolder = null
  accountStore.selectFolder.mockReset()
  accountStore.syncAllComplete.mockReset().mockResolvedValue(undefined)
  accountStore.cancelAllSyncs.mockReset().mockResolvedValue(undefined)
  ui.version += 1
  ui.expanded = true
  ui.state = { collapsedFolders: { group: false, stale: true } }
  ui.setAccountExpanded.mockReset()
  ui.setFolderCollapsed.mockReset()
  ui.saveUIState.mockReset()
  keyboard.setFocusedPane.mockReset()
  backend.MoveToFolder.mockReset().mockResolvedValue(undefined)
  backend.Undo.mockReset().mockResolvedValue(undefined)
  toast.success.mockReset()
  toast.error.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('renders account badges, selects folders, toggles trees, and prunes stale UI state', async () => {
  const onFolderSelect = vi.fn()
  const onCompose = vi.fn()
  const onBack = vi.fn()
  const { target } = await renderSidebar({
    selectedAccountId: 'account-1', selectedFolderId: 'inbox', selectionSource: 'account',
    onFolderSelect, onCompose, showBackButton: true, onBack,
  })

  assert.match(target.textContent, /Synthetic Account/)
  assert.match(target.textContent, /14/)
  assert.match(target.textContent, /Archive/)
  assert.deepEqual(ui.saveUIState.mock.calls[0], [{ collapsedFolders: { group: false } }])

  buttonFor(target, '[data-sidebar-action]', 'sidebar.compose').click()
  assert.equal(onCompose.mock.calls.length, 1)
  target.querySelector('button[aria-label="aria.closeSidebar"]').click()
  assert.equal(onBack.mock.calls.length, 1)

  buttonFor(target, '[data-sidebar-item="folder"]', 'Archive').click()
  assert.deepEqual(accountStore.selectFolder.mock.calls.at(-1), ['account-1', 'archive', 'Archive', 'Archive'])
  assert.deepEqual(onFolderSelect.mock.calls.at(-1), ['account-1', 'archive', 'Archive', 'Archive', 'archive'])

  buttonFor(target, '[data-sidebar-item="folder"]', 'Other folders').click()
  assert.deepEqual(ui.setFolderCollapsed.mock.calls.at(-1), ['group', true])

  target.querySelector('[data-sidebar-item="account-header"]').click()
  assert.deepEqual(ui.setAccountExpanded.mock.calls.at(-1), ['account-1', false])
})

test('exposes keyboard navigation and focused action/folder-group operations', async () => {
  const onFolderSelect = vi.fn()
  const onCompose = vi.fn()
  const { instance } = await renderSidebar({
    selectedAccountId: 'account-1', selectedFolderId: 'inbox', selectionSource: 'account',
    onFolderSelect, onCompose,
  })

  assert.equal(instance.hasSelectedFolderWithChildren(), false)
  instance.selectPreviousFolder()
  assert.equal(instance.hasFocusedAccount(), true)
  instance.toggleFocusedAccount()
  assert.deepEqual(ui.setAccountExpanded.mock.calls.at(-1), ['account-1', false])
  instance.toggleFocusedAccount()
  assert.deepEqual(ui.setAccountExpanded.mock.calls.at(-1), ['account-1', true])

  instance.selectPreviousFolder()
  await flushAsync()
  assert.equal(instance.hasFocusedSidebarAction(), true)
  instance.activateFocusedSidebarAction()
  assert.equal(onCompose.mock.calls.length, 1)
  instance.moveFocusedSidebarAction(1)
  instance.activateFocusedSidebarAction()
  await flushAsync()
  assert.equal(accountStore.syncAllComplete.mock.calls.length, 1)

  instance.selectNextFolder()
  assert.equal(instance.hasFocusedAccount(), true)
  instance.selectNextFolder()
  await flushAsync()
  assert.deepEqual(onFolderSelect.mock.calls.at(-1).slice(0, 2), ['account-1', 'inbox'])
  instance.selectNextFolder()
  assert.equal(instance.hasFocusedFolderGroup(), true)
  instance.toggleFocusedFolderGroup()
  assert.ok(ui.setFolderCollapsed.mock.calls.some((call) => call[0] === 'group'))

  instance.revealFolder('account-1', 'archive')
  await flushAsync()
  assert.ok(ui.setAccountExpanded.mock.calls.some((call) => call[0] === 'account-1' && call[1] === true))
  assert.ok(ui.setFolderCollapsed.mock.calls.some((call) => call[0] === 'group' && call[1] === false))
  instance.revealFolder('', '')
})

test('starts or cancels synchronization and contains sync failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target, instance } = await renderSidebar()
  target.querySelector('[data-sidebar-action="sync"]').click()
  await flushAsync()
  assert.equal(accountStore.syncAllComplete.mock.calls.length, 1)

  accountStore.isAnySyncing = true
  await instance.toggleSync()
  assert.equal(accountStore.cancelAllSyncs.mock.calls.length, 1)

  accountStore.syncAllComplete.mockRejectedValueOnce(new Error('sync failed'))
  await instance.syncAllAccounts()
  accountStore.cancelAllSyncs.mockRejectedValueOnce(new Error('cancel failed'))
  await instance.cancelSync()
  assert.equal(error.mock.calls.length, 2)
})

test('moves dropped messages, offers undo, and reports move and undo failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const onMessagesMoved = vi.fn()
  const { target } = await renderSidebar({
    selectedAccountId: 'account-1', selectedFolderId: 'inbox', selectionSource: 'account', onMessagesMoved,
  })
  const archive = buttonFor(target, '[data-sidebar-item="folder"]', 'Archive')

  archive.dispatchEvent(dropEvent(JSON.stringify({ messageIds: ['message-1'], sourceAccountId: 'account-1' })))
  await flushAsync()
  assert.deepEqual(backend.MoveToFolder.mock.calls[0], [['message-1'], 'archive'])
  assert.equal(onMessagesMoved.mock.calls.length, 1)
  assert.equal(toast.success.mock.calls.length, 1)
  await toast.success.mock.calls[0][1][0].onClick()
  assert.equal(backend.Undo.mock.calls.length, 1)

  backend.MoveToFolder.mockRejectedValueOnce(new Error('move unavailable'))
  archive.dispatchEvent(dropEvent(JSON.stringify({ messageIds: ['message-2'], sourceAccountId: 'account-1' })))
  await flushAsync()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.failedToMove')

  backend.Undo.mockRejectedValueOnce(new Error('undo unavailable'))
  await toast.success.mock.calls[0][1][0].onClick()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.undoFailed')
  assert.equal(error.mock.calls.length, 2)

  archive.dispatchEvent(dropEvent('{bad json'))
  archive.dispatchEvent(dropEvent(JSON.stringify({ messageIds: [] })))
  const beforeSameFolder = backend.MoveToFolder.mock.calls.length
  const inbox = buttonFor(target, '[data-sidebar-item="folder"]', 'Inbox')
  inbox.dispatchEvent(dropEvent(JSON.stringify({ messageIds: ['same'], sourceAccountId: 'account-1' })))
  await flushAsync()
  assert.equal(backend.MoveToFolder.mock.calls.length, beforeSameFolder)
})

test('shows loading, empty, and account error states and closes the add-account dialog', async () => {
  accountStore.loading = true
  let rendered = await renderSidebar()
  assert.doesNotMatch(rendered.target.textContent, /Synthetic Account/)
  await unmount(mounted.pop())

  accountStore.loading = false
  accountStore.accounts = []
  rendered = await renderSidebar()
  assert.match(rendered.target.textContent, /sidebar\.noAccountsYet/)
  buttonFor(rendered.target, 'button', 'sidebar.addAccount').click()
  await flushAsync()
  assert.ok(rendered.target.querySelector('[data-account-dialog]'))
  rendered.target.querySelector('[data-account-dialog-close]').click()
  assert.deepEqual(keyboard.setFocusedPane.mock.calls.at(-1), ['messageList'])
  await unmount(mounted.pop())

  accountStore.accounts = [{ ...accountFixture(), loading: true }]
  rendered = await renderSidebar()
  assert.match(rendered.target.textContent, /sidebar\.loadingFolders/)
  await unmount(mounted.pop())

  accountStore.accounts = [{ ...accountFixture(), folders: [], loading: false, error: 'Synthetic account error' }]
  rendered = await renderSidebar()
  assert.match(rendered.target.textContent, /sidebar\.noFoldersSynced/)
  assert.equal(rendered.target.querySelector('[title="Synthetic account error"]')?.getAttribute('title'), 'Synthetic account error')
})
