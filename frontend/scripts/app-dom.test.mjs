// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  PrepareReply: vi.fn(),
  GetPendingMailto: vi.fn(),
  GetDraft: vi.fn(),
  GetTermsAccepted: vi.fn(),
  SetTermsAccepted: vi.fn(),
  RefreshWindowConstraints: vi.fn(),
  AcceptCertificate: vi.fn(),
  GetStartHiddenActive: vi.fn(),
  QuitApp: vi.fn(),
  GetSystemTheme: vi.fn(),
  NotifyStartupComplete: vi.fn(),
  ReadFileAsAttachment: vi.fn(),
}))
const eventHandlers = vi.hoisted(() => new Map())
const runtime = vi.hoisted(() => ({
  WindowShow: vi.fn(),
  WindowHide: vi.fn(),
  WindowSetMinSize: vi.fn(),
  EventsOn: vi.fn((name, handler) => {
    eventHandlers.set(name, handler)
    return () => eventHandlers.delete(name)
  }),
}))
const toast = vi.hoisted(() => ({ add: vi.fn() }))
const theme = vi.hoisted(() => ({
  init: vi.fn(),
  apply: vi.fn(),
  system: vi.fn(),
  media: vi.fn(),
}))
const ui = vi.hoisted(() => ({
  activePane: 'mail',
  saves: [],
  state: null,
}))
const layout = vi.hoisted(() => ({
  init: vi.fn(),
  mode: 'full',
  view: 'default',
  showViewer: vi.fn(),
  hideViewer: vi.fn(),
  showSidebar: vi.fn(),
  hideSidebar: vi.fn(),
}))
const keyboard = vi.hoisted(() => ({ pane: 'messageList', composerOpen: false }))
const contacts = vi.hoisted(() => ({ preload: vi.fn(), activate: vi.fn() }))
const accountStore = vi.hoisted(() => ({
  accounts: [{
    account: { id: 'account-1', imapHost: 'imap.example.test' },
    folders: [{
      folder: {
        id: 'inbox-1',
        name: 'Inbox',
        path: 'INBOX',
        type: 'inbox',
        noSelect: false,
      },
      children: [],
    }],
    loading: false,
  }],
  loading: false,
  selectedFolder: null,
  load: vi.fn(),
  syncAllComplete: vi.fn(),
  selectFolder: vi.fn(),
}))
const mailActions = vi.hoisted(() => ({
  archiveMessages: vi.fn(),
  setReadStateMessages: vi.fn(),
  toggleSpamMessages: vi.fn(),
  toggleStarMessages: vi.fn(),
  undoLastMailAction: vi.fn(),
}))
const keyboardActionMenu = vi.hoisted(() => ({ showForRegion: vi.fn() }))

vi.mock('../src/lib/components/sidebar/Sidebar.svelte', async () => ({ default: (await import('./fixtures/SidebarStub.svelte')).default }))
vi.mock('../src/lib/components/list/MessageList.svelte', async () => ({ default: (await import('./fixtures/MessageListStub.svelte')).default }))
vi.mock('../src/lib/components/viewer/ConversationViewer.svelte', async () => ({ default: (await import('./fixtures/ConversationViewerStub.svelte')).default }))
vi.mock('../src/lib/components/composer/Composer.svelte', async () => ({ default: (await import('./fixtures/ComposerStub.svelte')).default }))
vi.mock('../src/lib/components/status/StatusBar.svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('../src/lib/components/TermsDialog.svelte', async () => ({ default: (await import('./fixtures/TermsDialogStub.svelte')).default }))
vi.mock('../src/lib/components/settings/CertificateDialog.svelte', async () => ({ default: (await import('./fixtures/CertificateDialogStub.svelte')).default }))
vi.mock('../src/lib/components/rail/ActivityRail.svelte', async () => ({ default: (await import('./fixtures/ActivityRailStub.svelte')).default }))
vi.mock('../src/lib/components/settings/SettingsDialog.svelte', async () => ({ default: (await import('./fixtures/SettingsDialogStub.svelte')).default }))
vi.mock('../src/lib/components/SearchOverlay.svelte', async () => ({ default: (await import('./fixtures/SearchOverlayStub.svelte')).default }))
vi.mock('../src/lib/components/backup/BackupViewerDialog.svelte', async () => ({ default: (await import('./fixtures/BackupViewerStub.svelte')).default }))
vi.mock('../src/lib/components/keyboard/KeyboardActionMenu.svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$contacts/components/ContactsPane.svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))

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
vi.mock('$contacts/stores/contactsView.svelte', () => ({
  activateContactFromGlobalSearch: contacts.activate,
}))
vi.mock('$contacts/stores/contactAccountGroups.svelte', () => ({
  preloadContactAccountGroups: contacts.preload,
}))
vi.mock('$lib/keyboard/globalShortcuts', () => ({
  handleGlobalShortcut(event, context) {
    if (event.isComposing || event.keyCode === 229) return
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'q') {
      event.preventDefault()
      context.handleQuit()
    } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'n') {
      event.preventDefault()
      context.handleCompose()
    } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
      event.preventDefault()
      context.setSearchOverlay(true)
    } else if (event.key === '1') {
      context.handleBulkArchive(['bulk-message'])
    } else if (event.key === '2') {
      context.handleBulkSpam(['bulk-message'])
    } else if (event.key === '3') {
      context.handleBulkMarkRead(['bulk-message'])
    } else if (event.key === '4') {
      context.handleBulkMarkUnread(['bulk-message'])
    } else if (event.key === '5') {
      context.handleBulkToggleStar(['bulk-message'], true)
    } else if (event.key === '6') {
      context.openRegionActionMenu()
    } else if (event.key === '7') {
      context.focusContextMenu()
    } else if (event.key === 't') {
      context.toggleThreadFocus()
    }
  },
}))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore }))
vi.mock('$lib/stores/toast', () => ({ addToast: (value) => toast.add(value) }))
vi.mock('$lib/stores/settings.svelte', () => ({
  loadSettings: vi.fn().mockResolvedValue('system'),
  getThemeMode: () => 'system',
}))
vi.mock('$lib/stores/imageAllowlist.svelte', () => ({ loadImageAllowlist: vi.fn() }))
vi.mock('$lib/stores/theme.svelte', () => ({
  initTheme: theme.init,
  applyThemeFromMode: theme.apply,
  handleSystemThemeEvent: theme.system,
  handleMediaQueryChange: theme.media,
}))
vi.mock('$lib/stores/uiState.svelte', () => ({
  DEFAULT_LIST_WIDTH: 420,
  DEFAULT_SIDEBAR_WIDTH: 336,
  loadUIState: async () => ui.state,
  saveUIState: (value) => ui.saves.push(value),
  getActivePane: () => ui.activePane,
  setActivePane: (value) => { ui.activePane = value },
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  getFocusedPane: () => keyboard.pane,
  isMainKeyboardScope: () => true,
  setFocusedPane: (value) => { keyboard.pane = value },
  setComposerOpen: (value) => { keyboard.composerOpen = value },
}))
vi.mock('$lib/stores/layout.svelte', () => ({
  initLayout: layout.init,
  getLayoutMode: () => layout.mode,
  getResponsiveView: () => layout.view,
  showViewer: layout.showViewer,
  hideViewer: layout.hideViewer,
  showSidebar: layout.showSidebar,
  hideSidebar: layout.hideSidebar,
  isResponsive: () => layout.mode !== 'full',
}))
vi.mock('$lib/mailActions', () => mailActions)
vi.mock('$lib/stores/keyboardActionMenu.svelte', () => ({
  keyboardActionMenu,
}))

import App from '../src/App.svelte'

const mounted = []

function defaultUIState() {
  return {
    selectedAccountId: null,
    selectedFolderId: null,
    selectedFolderName: 'Inbox',
    selectedFolderType: 'inbox',
    selectedThreadId: null,
    selectedConversationAccountId: null,
    selectedConversationFolderId: null,
    sidebarWidth: 336,
    listWidth: 420,
    expandedAccounts: {},
    unifiedInboxExpanded: true,
    collapsedFolders: {},
    activeExtension: 'mail',
  }
}

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderApp() {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(App, { target })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

function clickAction(name, root = document) {
  const button = root.querySelector(`[data-app-action="${name}"]`)
  assert.ok(button, `missing action ${name}`)
  button.click()
  return button
}

beforeEach(() => {
  document.body.innerHTML = '<div id="boot-splash"></div>'
  eventHandlers.clear()
  ui.activePane = 'mail'
  ui.saves.splice(0)
  ui.state = defaultUIState()
  keyboard.pane = 'messageList'
  keyboard.composerOpen = false
  layout.mode = 'full'
  layout.view = 'default'
  accountStore.accounts = [{
    account: { id: 'account-1', imapHost: 'imap.example.test' },
    folders: [{
      folder: { id: 'inbox-1', name: 'Inbox', path: 'INBOX', type: 'inbox', noSelect: false },
      children: [],
    }],
    loading: false,
  }]
  accountStore.loading = false
  accountStore.selectedFolder = null
  accountStore.load.mockReset().mockResolvedValue(undefined)
  accountStore.syncAllComplete.mockReset().mockResolvedValue(undefined)
  accountStore.selectFolder.mockReset()
  backend.PrepareReply.mockReset().mockResolvedValue({ subject: 'Re: Quarterly update', to: [], attachments: [] })
  backend.GetPendingMailto.mockReset().mockResolvedValue(null)
  backend.GetDraft.mockReset().mockResolvedValue({ subject: 'Saved draft', to: [], attachments: [] })
  backend.GetTermsAccepted.mockReset().mockResolvedValue(true)
  backend.SetTermsAccepted.mockReset().mockResolvedValue(undefined)
  backend.RefreshWindowConstraints.mockReset()
  backend.AcceptCertificate.mockReset().mockResolvedValue(undefined)
  backend.GetStartHiddenActive.mockReset().mockResolvedValue(false)
  backend.QuitApp.mockReset()
  backend.GetSystemTheme.mockReset().mockResolvedValue('light')
  backend.NotifyStartupComplete.mockReset()
  backend.ReadFileAsAttachment.mockReset().mockResolvedValue({
    filename: 'report.pdf',
    contentType: 'application/pdf',
    size: 4,
    data: 'ZGF0YQ==',
  })
  for (const fn of Object.values(runtime)) fn.mockClear?.()
  toast.add.mockReset()
  theme.init.mockReset().mockResolvedValue(undefined)
  theme.apply.mockReset()
  theme.system.mockReset()
  theme.media.mockReset()
  contacts.preload.mockReset()
  contacts.activate.mockReset().mockResolvedValue(undefined)
  keyboardActionMenu.showForRegion.mockReset()
  for (const fn of Object.values(layout)) fn?.mockClear?.()
  for (const fn of Object.values(mailActions)) fn.mockReset().mockResolvedValue(undefined)
  vi.spyOn(window, 'focus').mockImplementation(() => {})
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('completes startup, enforces terms acceptance, and supports capture-phase Select All', async () => {
  backend.GetTermsAccepted.mockResolvedValue(false)
  const { target } = await renderApp()

  assert.equal(accountStore.load.mock.calls.length, 1)
  assert.equal(theme.init.mock.calls.length, 1)
  assert.equal(contacts.preload.mock.calls.length, 1)
  assert.deepEqual(runtime.WindowSetMinSize.mock.calls.at(-1), [1354, 400])
  assert.equal(runtime.WindowShow.mock.calls.length, 1)
  assert.equal(runtime.WindowHide.mock.calls.length, 0)
  assert.equal(backend.NotifyStartupComplete.mock.calls.length, 1)
  assert.equal(backend.RefreshWindowConstraints.mock.calls.length, 1)
  assert.equal(target.querySelector('[data-stub="terms-dialog"]') !== null, true)

  clickAction('accept-terms', target)
  await flushAsync()
  assert.deepEqual(backend.SetTermsAccepted.mock.calls.at(-1), [true])
  assert.equal(target.querySelector('[data-stub="terms-dialog"]'), null)

  const input = document.createElement('input')
  input.value = 'select all of me'
  document.body.appendChild(input)
  input.focus()
  input.setSelectionRange(3, 3)
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', metaKey: true, bubbles: true }))
  assert.equal(input.selectionStart, 0)
  assert.equal(input.selectionEnd, input.value.length)
})

test('routes native settings, About, backup, theme, and window events to visible UI', async () => {
  const { target } = await renderApp()
  clickAction('open-settings', target)
  await tick()
  assert.ok(target.querySelector('[data-stub="settings-dialog"]'))
  eventHandlers.get('menu:openSettings')()
  await tick()
  let settingsDialog = target.querySelector('[data-stub="settings-dialog"]')
  assert.ok(settingsDialog)
  assert.equal(settingsDialog.dataset.page, 'general')
  assert.equal(target.querySelector('[data-app-action="open-settings"]').dataset.selected, 'true')

  clickAction('close-settings', target)
  await new Promise((resolve) => requestAnimationFrame(resolve))
  assert.equal(target.querySelector('[data-stub="settings-dialog"]'), null)
  assert.equal(document.activeElement, target.querySelector('[data-app-action="open-settings"]'))

  eventHandlers.get('menu:openAbout')()
  await tick()
  settingsDialog = target.querySelector('[data-stub="settings-dialog"]')
  assert.equal(settingsDialog.dataset.page, 'about')
  eventHandlers.get('menu:openBackupViewer')()
  await tick()
  assert.equal(target.querySelector('[data-stub="settings-dialog"]'), null)
  assert.ok(target.querySelector('[data-stub="backup-viewer"]'))
  clickAction('close-backup', target)
  await tick()
  assert.equal(target.querySelector('[data-stub="backup-viewer"]'), null)

  eventHandlers.get('theme:system-preference')('dark')
  assert.deepEqual(theme.system.mock.calls.at(-1), ['dark'])
  eventHandlers.get('window:show')()
  assert.equal(window.focus.mock.calls.length, 1)
})

test('opens and closes composers from the sidebar, mailto, draft, address, and reply flows', async () => {
  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target } = await renderApp()
  clickAction('compose', target)
  await tick()
  let composer = target.querySelector('[data-stub="composer"]')
  assert.ok(composer)
  assert.equal(composer.dataset.account, 'account-1')
  clickAction('composer-close', target)
  await flushAsync()
  assert.equal(target.querySelector('[data-stub="composer"]'), null)

  eventHandlers.get('mailto:external')({
    to: ['mailto@example.test'],
    subject: 'Mailto subject',
    body: 'Mailto body',
  })
  await tick()
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, 'Mailto subject')
  assert.equal(composer.querySelector('[data-composer-to]').textContent, 'mailto@example.test')
  clickAction('composer-close', target)
  await flushAsync()

  clickAction('open-draft', target)
  await flushAsync()
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.dataset.draft, 'draft-1')
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, 'Saved draft')
  clickAction('composer-close', target)
  await flushAsync()

  clickAction('compose-address', target)
  await tick()
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.querySelector('[data-composer-to]').textContent, 'person@example.test')
  clickAction('composer-close', target)
  await flushAsync()

  clickAction('reply', target)
  await flushAsync()
  assert.deepEqual(backend.PrepareReply.mock.calls.at(-1), ['message-last', 'reply'])
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, 'Re: Quarterly update')
  assert.equal(composer.dataset.imagesLoaded, 'true')

  clickAction('composer-close', target)
  await flushAsync()
  backend.PrepareReply.mockRejectedValueOnce(new Error('reply preparation failed'))
  clickAction('reply', target)
  await flushAsync()
  assert.match(toast.add.mock.calls.at(-1)[0].message, /toast\.failedToPrepare/)
  assert.ok(target.querySelector('[data-stub="composer"]'))
  errorSpy.mockRestore()
})

test('queues Finder attachments behind an open composer and processes them after close', async () => {
  const { target } = await renderApp()
  clickAction('compose', target)
  await tick()
  eventHandlers.get('files:openAsAttachments')({ paths: ['/private/report.pdf', '/private/report.pdf'] })
  await flushAsync()
  assert.equal(backend.ReadFileAsAttachment.mock.calls.length, 0)
  assert.match(toast.add.mock.calls.at(-1)[0].message, /toast\.externalFilesQueued/)

  clickAction('composer-close', target)
  await flushAsync()
  assert.deepEqual(backend.ReadFileAsAttachment.mock.calls.at(-1), ['/private/report.pdf'])
  const composer = target.querySelector('[data-stub="composer"]')
  assert.ok(composer)
  assert.equal(composer.querySelector('[data-composer-attachments]').textContent, '1')
})

test('handles certificate and backup events and applies global shortcut IME guards', async () => {
  const { target } = await renderApp()
  eventHandlers.get('certificate:untrusted')({
    accountId: 'account-1',
    certificate: { host: 'imap.example.test', fingerprint: 'synthetic' },
  })
  await tick()
  assert.ok(target.querySelector('[data-stub="certificate-dialog"]'))
  clickAction('accept-cert-once', target)
  await flushAsync()
  assert.deepEqual(backend.AcceptCertificate.mock.calls.at(-1), [
    'imap.example.test',
    { host: 'imap.example.test', fingerprint: 'synthetic' },
    false,
  ])
  assert.equal(target.querySelector('[data-stub="certificate-dialog"]'), null)

  eventHandlers.get('backup:progress')({
    phase: 'done', current: 3, total: 3, exported: 2, skipped: 1, failed: 0,
  })
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'success')
  eventHandlers.get('backup:progress')({
    phase: 'error', current: 0, total: 1, exported: 0, skipped: 0, failed: 1, message: 'disk full',
  })
  assert.deepEqual(toast.add.mock.calls.at(-1)[0], {
    type: 'error',
    message: 'settingsBackup.backupFailed: disk full',
  })

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'q', ctrlKey: true, bubbles: true, isComposing: true }))
  assert.equal(backend.QuitApp.mock.calls.length, 0)
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'q', ctrlKey: true, bubbles: true }))
  await tick()
  assert.equal(backend.QuitApp.mock.calls.length, 1)
  assert.match(target.textContent, /window\.shuttingDown/)

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, bubbles: true }))
  await tick()
  assert.ok(target.querySelector('[data-stub="search-overlay"]'))
})

test('restores and changes folder/conversation state through list, notification, and search routes', async () => {
  ui.state = {
    ...defaultUIState(),
    selectedAccountId: 'account-1',
    selectedFolderId: 'inbox-1',
    selectedFolderName: 'Inbox',
    selectedFolderType: 'inbox',
    selectedThreadId: 'restored-thread',
    selectedConversationAccountId: 'account-1',
    selectedConversationFolderId: 'inbox-1',
  }
  const { target } = await renderApp()
  const list = target.querySelector('[data-stub="message-list"]')
  const viewer = target.querySelector('[data-stub="conversation-viewer"]')
  assert.equal(list.dataset.account, 'account-1')
  assert.equal(list.dataset.folder, 'inbox-1')
  assert.equal(viewer.dataset.thread, 'restored-thread')

  clickAction('focus-conversation', target)
  await flushAsync()
  assert.equal(viewer.dataset.thread, 'thread-1')
  clickAction('complete-action', target)
  clickAction('search', target)
  await tick()
  assert.ok(target.querySelector('[data-stub="search-overlay"]'))
  clickAction('close-search', target)
  window.dispatchEvent(new CustomEvent('escape-iframe-focus'))
  assert.equal(document.activeElement, target.querySelector('[data-pane="messageList"]'))
  clickAction('open-conversation', target)
  await flushAsync()
  assert.equal(layout.showViewer.mock.calls.length > 0, true)
  clickAction('empty-folder', target)
  await flushAsync()
  assert.equal(viewer.dataset.thread, '')

  eventHandlers.get('notification:clicked')({
    accountId: 'account-1',
    folderId: 'inbox-1',
    threadId: 'notification-thread',
  })
  await new Promise((resolve) => setTimeout(resolve, 110))
  await flushAsync()
  assert.equal(list.dataset.selected, 'notification-thread')
  assert.deepEqual(accountStore.selectFolder.mock.calls.at(-1), ['account-1', 'inbox-1', 'INBOX', 'Inbox'])

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, bubbles: true }))
  await flushAsync()
  clickAction('select-mail-search', target)
  await new Promise((resolve) => setTimeout(resolve, 110))
  await flushAsync()
  assert.equal(list.dataset.selected, 'thread-search')

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, bubbles: true }))
  await flushAsync()
  clickAction('select-contact-search', target)
  await flushAsync()
  assert.deepEqual(contacts.activate.mock.calls.at(-1), ['contact-1'])
  assert.equal(ui.activePane, 'contacts')
})

test('handles hidden startup, pending mailto, certificate choices, and partial external-file failures', async () => {
  vi.useFakeTimers()
  backend.GetStartHiddenActive.mockResolvedValue(true)
  backend.GetPendingMailto.mockResolvedValue({
    to: ['launch@example.test'],
    cc: ['copy@example.test'],
    bcc: ['blind@example.test'],
    subject: 'Launch mail',
    body: 'Opened from command line',
  })
  const { target } = await renderApp()
  await vi.advanceTimersByTimeAsync(110)
  await flushAsync()
  assert.equal(runtime.WindowHide.mock.calls.length, 1)
  assert.equal(runtime.WindowShow.mock.calls.length, 0)
  const composer = target.querySelector('[data-stub="composer"]')
  assert.ok(composer)
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, 'Launch mail')
  clickAction('composer-close', target)
  await flushAsync()

  eventHandlers.get('certificate:untrusted')({
    accountId: 'account-1',
    certificate: { host: 'imap.example.test', fingerprint: 'permanent' },
  })
  await flushAsync()
  clickAction('accept-cert-always', target)
  await flushAsync()
  assert.deepEqual(backend.AcceptCertificate.mock.calls.at(-1), [
    'imap.example.test',
    { host: 'imap.example.test', fingerprint: 'permanent' },
    true,
  ])

  eventHandlers.get('certificate:untrusted')({
    accountId: 'account-1',
    certificate: { host: 'imap.example.test', fingerprint: 'declined' },
  })
  await flushAsync()
  clickAction('decline-cert', target)
  await flushAsync()
  assert.equal(target.querySelector('[data-stub="certificate-dialog"]'), null)

  backend.ReadFileAsAttachment.mockImplementation(async (path) => {
    if (path.endsWith('missing.pdf')) return null
    if (path.endsWith('broken.pdf')) throw new Error('read failed')
    return { filename: 'good.pdf', contentType: 'application/pdf', size: 4, data: 'ZGF0YQ==' }
  })
  eventHandlers.get('files:openAsAttachments')({
    paths: ['/private/good.pdf', '/private/missing.pdf', '/private/broken.pdf'],
  })
  await flushAsync()
  assert.equal(backend.ReadFileAsAttachment.mock.calls.length, 3)
  assert.equal(toast.add.mock.calls.some((call) => call[0].type === 'error' && call[0].message.includes('toast.externalFilesFailed')), true)
  assert.ok(target.querySelector('[data-stub="composer"]'))
})

test('routes bulk mail actions, undo callbacks, region menus, and thread focus through the global dispatcher', async () => {
  const complete = (withUndo = false) => async (...args) => {
    const options = args.at(-1)
    await options?.onSuccess?.(true)
    if (withUndo) await options?.onUndo?.()
  }
  mailActions.archiveMessages.mockImplementation(complete(true))
  mailActions.toggleSpamMessages.mockImplementation(complete())
  mailActions.setReadStateMessages.mockImplementation(complete())
  mailActions.toggleStarMessages.mockImplementation(complete())
  mailActions.undoLastMailAction.mockImplementation(async (options) => options?.onSuccess?.())
  const { target } = await renderApp()

  for (const key of ['1', '2', '3', '4', '5']) {
    window.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
    await flushAsync()
  }
  assert.deepEqual(mailActions.archiveMessages.mock.calls.at(-1)[0], ['bulk-message'])
  assert.deepEqual(mailActions.toggleSpamMessages.mock.calls.at(-1).slice(0, 2), [['bulk-message'], false])
  assert.deepEqual(mailActions.setReadStateMessages.mock.calls[0].slice(0, 2), [['bulk-message'], true])
  assert.deepEqual(mailActions.setReadStateMessages.mock.calls[1].slice(0, 2), [['bulk-message'], false])
  assert.deepEqual(mailActions.toggleStarMessages.mock.calls.at(-1).slice(0, 2), [['bulk-message'], true])
  assert.equal(mailActions.undoLastMailAction.mock.calls.length, 1)
  assert.equal(Number(target.querySelector('[data-stub="message-list"]').dataset.actions) >= 5, true)

  window.dispatchEvent(new KeyboardEvent('keydown', { key: '6', bubbles: true }))
  assert.deepEqual(keyboardActionMenu.showForRegion.mock.calls.at(-1), ['messageList'])
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 't', bubbles: true }))
  await flushAsync()
  assert.equal(keyboard.pane, 'viewer')
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 't', bubbles: true }))
  await flushAsync()
  assert.equal(keyboard.pane, 'messageList')
})

test('fails safely when terms, draft loading, certificates, or account-dependent compose paths are unavailable', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetTermsAccepted.mockRejectedValueOnce(new Error('terms store unavailable'))
  backend.SetTermsAccepted.mockRejectedValueOnce(new Error('terms write unavailable'))
  backend.GetDraft.mockRejectedValueOnce(new Error('draft unavailable'))
  backend.AcceptCertificate.mockRejectedValueOnce(new Error('certificate rejected'))
  const { target } = await renderApp()
  assert.ok(target.querySelector('[data-stub="terms-dialog"]'))
  clickAction('accept-terms', target)
  await flushAsync()
  assert.ok(target.querySelector('[data-stub="terms-dialog"]'))

  clickAction('open-draft', target)
  await flushAsync()
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'composer.failedToLoadDraft')
  eventHandlers.get('certificate:untrusted')({
    accountId: 'missing-account',
    certificate: { host: 'unknown.example.test', fingerprint: 'failure' },
  })
  await flushAsync()
  clickAction('accept-cert-once', target)
  await flushAsync()
  assert.equal(target.querySelector('[data-stub="certificate-dialog"]'), null)

  accountStore.accounts = []
  eventHandlers.get('mailto:external')({ to: ['nobody@example.test'] })
  eventHandlers.get('files:openAsAttachments')({ paths: ['/private/no-account.pdf'] })
  await flushAsync()
  assert.equal(toast.add.mock.calls.filter((call) => call[0].message === 'toast.noAccountConfigured').length, 2)
  assert.equal(error.mock.calls.length >= 3, true)
})

test('restores nested folders and clears hierarchy-only or missing selections', async () => {
  accountStore.accounts[0].folders = [{
    folder: { id: 'parent-1', name: 'Parent', path: 'Parent', type: '', noSelect: true },
    children: [{
      folder: { id: 'nested-1', name: 'Nested', path: 'Parent/Nested', type: 'archive', noSelect: false },
      children: [],
    }],
  }]
  ui.state = {
    ...defaultUIState(),
    selectedAccountId: 'account-1',
    selectedFolderId: 'nested-1',
    selectedFolderName: '',
    selectedFolderType: 'archive',
    selectedThreadId: 'nested-thread',
  }
  const nested = await renderApp()
  assert.equal(nested.target.querySelector('[data-stub="message-list"]').dataset.folder, 'nested-1')
  assert.match(nested.target.querySelector('[data-stub="message-list"]').textContent, /Open conversation/)

  ui.saves.splice(0)
  ui.state = {
    ...defaultUIState(),
    selectedAccountId: 'account-1',
    selectedFolderId: 'parent-1',
    selectedFolderName: 'Parent',
  }
  const hierarchyOnly = await renderApp()
  assert.equal(hierarchyOnly.target.querySelector('[data-stub="message-list"]').dataset.folder, '')
  assert.equal(ui.saves.some((state) => state.selectedAccountId === null && state.selectedFolderId === null), true)

  ui.saves.splice(0)
  ui.state = {
    ...defaultUIState(),
    selectedAccountId: 'missing-account',
    selectedFolderId: 'missing-folder',
  }
  const missing = await renderApp()
  assert.equal(missing.target.querySelector('[data-stub="message-list"]').dataset.account, '')
  assert.equal(ui.saves.some((state) => state.selectedAccountId === null), true)
})

test('rejects hierarchy-only folder activation and uses current folder for partial search results', async () => {
  accountStore.accounts[0].folders[0].folder.noSelect = true
  const blocked = await renderApp()
  clickAction('select-folder', blocked.target)
  eventHandlers.get('notification:clicked')({
    accountId: 'account-1',
    folderId: 'inbox-1',
    threadId: 'blocked-thread',
  })
  await flushAsync()
  assert.equal(accountStore.selectFolder.mock.calls.length, 0)
  assert.equal(layout.hideSidebar.mock.calls.length, 0)
  assert.equal(blocked.target.querySelector('[data-stub="conversation-viewer"]').dataset.thread, '')

  accountStore.accounts[0].folders[0].folder.noSelect = false
  ui.state = {
    ...defaultUIState(),
    selectedAccountId: 'account-1',
    selectedFolderId: 'inbox-1',
    selectedFolderName: 'Inbox',
    selectedFolderType: 'inbox',
  }
  const fallback = await renderApp()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, bubbles: true }))
  await flushAsync()
  clickAction('select-mail-search-fallback', fallback.target)
  await new Promise((resolve) => setTimeout(resolve, 110))
  await flushAsync()
  assert.deepEqual(accountStore.selectFolder.mock.calls.at(-1), ['account-1', 'inbox-1', 'INBOX', 'Inbox'])
  assert.equal(fallback.target.querySelector('[data-stub="message-list"]').dataset.selected, 'thread-fallback')

  eventHandlers.get('mail:openConversation')({
    accountId: 'account-1',
    folderId: 'new-server-folder',
    threadId: 'new-folder-thread',
  })
  await new Promise((resolve) => setTimeout(resolve, 110))
  await flushAsync()
  assert.deepEqual(accountStore.selectFolder.mock.calls.at(-1), ['account-1', 'new-server-folder', '', 'Inbox'])
  assert.equal(fallback.target.querySelector('[data-stub="message-list"]').dataset.selected, 'new-folder-thread')
})

test('preserves a composer opened while Finder attachments are still loading', async () => {
  let resolveFirstRead
  backend.ReadFileAsAttachment.mockImplementationOnce(() => new Promise((resolve) => {
    resolveFirstRead = resolve
  }))
  const { target } = await renderApp()
  eventHandlers.get('files:openAsAttachments')({ paths: ['/private/race.pdf'] })
  await flushAsync()
  assert.equal(typeof resolveFirstRead, 'function')

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'n', ctrlKey: true, bubbles: true }))
  await tick()
  assert.ok(target.querySelector('[data-stub="composer"]'))
  resolveFirstRead({ filename: 'race.pdf', contentType: 'application/pdf', size: 4, data: 'ZGF0YQ==' })
  await flushAsync()
  assert.match(toast.add.mock.calls.at(-1)[0].message, /toast\.externalFilesQueued/)
  assert.equal(target.querySelector('[data-composer-attachments]').textContent, '0')

  clickAction('composer-close', target)
  await flushAsync()
  assert.equal(backend.ReadFileAsAttachment.mock.calls.length, 2)
  assert.equal(target.querySelector('[data-composer-attachments]').textContent, '1')
})

test('continues after an attachment batch has no usable files and applies compose defaults', async () => {
  backend.ReadFileAsAttachment.mockResolvedValueOnce(null)
  const { target } = await renderApp()
  eventHandlers.get('files:openAsAttachments')({ paths: ['/private/missing.pdf'] })
  await flushAsync()
  assert.equal(target.querySelector('[data-stub="composer"]'), null)
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'error')

  backend.GetDraft.mockResolvedValueOnce(null)
  clickAction('open-draft', target)
  await flushAsync()
  let composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.dataset.draft, 'draft-1')
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, '')
  clickAction('composer-close', target)
  await flushAsync()

  clickAction('select-folder', target)
  eventHandlers.get('mailto:external')({})
  await tick()
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.dataset.account, 'account-1')
  assert.equal(composer.querySelector('[data-composer-to]').textContent, '')
  assert.equal(composer.querySelector('[data-composer-subject]').textContent, '')
  clickAction('composer-close', target)
  await flushAsync()

  clickAction('reply-default-images', target)
  await flushAsync()
  composer = target.querySelector('[data-stub="composer"]')
  assert.equal(composer.dataset.imagesLoaded, 'false')
})

test('guards every account-dependent entry point when no account exists', async () => {
  accountStore.accounts = []
  backend.GetPendingMailto.mockResolvedValue({ to: [], subject: '', body: '' })
  const { target } = await renderApp()
  eventHandlers.get('files:openAsAttachments')({ paths: [] })
  clickAction('compose', target)
  clickAction('open-draft', target)
  clickAction('compose-address', target)
  clickAction('reply', target)
  await flushAsync()
  assert.equal(target.querySelector('[data-stub="composer"]'), null)
  assert.equal(backend.GetDraft.mock.calls.length, 0)
  assert.equal(backend.PrepareReply.mock.calls.length, 0)
  assert.equal(toast.add.mock.calls.length, 0)
})

test('handles duplicate certificate prompts, backup edge results, narrow panes, and context focus', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  layout.mode = 'narrow'
  layout.view = 'sidebar'
  backend.AcceptCertificate.mockRejectedValueOnce(new Error('permanent trust failed'))
  const { target } = await renderApp()

  eventHandlers.get('certificate:untrusted')({
    accountId: 'missing-account',
    certificate: { host: 'first.example.test', fingerprint: 'first' },
  })
  eventHandlers.get('certificate:untrusted')({
    accountId: 'account-1',
    certificate: { host: 'second.example.test', fingerprint: 'second' },
  })
  await flushAsync()
  const accept = clickAction('accept-cert-always', target)
  await flushAsync()
  assert.deepEqual(backend.AcceptCertificate.mock.calls.at(-1), [
    '',
    { host: 'first.example.test', fingerprint: 'first' },
    true,
  ])
  accept.click()
  await flushAsync()
  assert.equal(backend.AcceptCertificate.mock.calls.length, 1)

  eventHandlers.get('backup:progress')({
    phase: 'done', current: 5, total: 5, exported: 2, skipped: 1, missing: 1, unavailable: 1, failed: 0,
  })
  assert.equal(toast.add.mock.calls.at(-1)[0].type, 'warning')
  eventHandlers.get('backup:progress')({
    phase: 'error', current: 0, total: 1, exported: 0, skipped: 0, failed: 1,
  })
  assert.equal(toast.add.mock.calls.at(-1)[0].message, 'settingsBackup.backupFailed')

  const sidebar = target.querySelector('[data-keyboard-region="sidebar"]')
  const sidebarItem = target.querySelector('[data-sidebar-item]')
  const down = new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0 })
  assert.equal(sidebarItem.dispatchEvent(down), false)
  assert.equal(document.activeElement, sidebar)
  sidebar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 1 }))
  target.querySelector('[data-pane="messageList"]').dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  assert.equal(keyboard.pane, 'messageList')
  target.querySelector('.responsive-scrim').click()
  assert.equal(layout.hideSidebar.mock.calls.length, 1)

  const menu = document.createElement('div')
  menu.setAttribute('role', 'menu')
  menu.tabIndex = -1
  const item = document.createElement('button')
  item.setAttribute('role', 'menuitem')
  menu.appendChild(item)
  document.body.appendChild(menu)
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '7', bubbles: true }))
  await new Promise((resolve) => requestAnimationFrame(resolve))
  assert.equal(document.activeElement, item)
  window.dispatchEvent(new KeyboardEvent('keydown', { code: 'AltLeft', key: 'Alt', bubbles: true }))
  window.dispatchEvent(new KeyboardEvent('keyup', { code: 'AltLeft', key: 'Alt', bubbles: true }))
  assert.equal(error.mock.calls.length, 1)
})
