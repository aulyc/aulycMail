// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  BuildBackupViewerIndex: vi.fn(),
  GetBackupSettings: vi.fn(),
  GetBackupViewerCatalog: vi.fn(),
  GetBackupViewerMessage: vi.fn(),
  ListBackupViewerMessages: vi.fn(),
  OpenBackupViewerDirectory: vi.fn(),
  SaveBackupViewerAttachmentAs: vi.fn(),
  SearchBackupViewerMessages: vi.fn(),
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
const history = vi.hoisted(() => ({ remember: vi.fn(), remove: vi.fn() }))
const guards = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))
const settings = vi.hoisted(() => ({ dark: true, enhanced: true, theme: 'dark' }))
const actionMenu = vi.hoisted(() => ({ showForRoot: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/utils/backup-directory-history', () => ({
  rememberBackupDirectory: history.remember,
  removeBackupDirectory: history.remove,
}))
vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guards.open,
  dialogGuardClose: guards.close,
}))
vi.mock('$lib/stores/settings.svelte', () => ({
  getDarkMailContent: () => settings.dark,
  getEnhancedKeyboardNavigation: () => settings.enhanced,
  getThemeMode: () => settings.theme,
}))
vi.mock('$lib/stores/keyboardActionMenu.svelte', () => ({ keyboardActionMenu: actionMenu }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/backup/BackupViewerToolbar.svelte', async () => ({
  default: (await import('./fixtures/BackupViewerToolbarTestStub.svelte')).default,
}))
vi.mock('$lib/components/backup/BackupViewerMessageDetail.svelte', async () => ({
  default: (await import('./fixtures/BackupViewerDetailTestStub.svelte')).default,
}))
vi.mock('$lib/components/backup/BackupViewerSearchOverlay.svelte', async () => ({
  default: (await import('./fixtures/BackupViewerSearchTestStub.svelte')).default,
}))

import BackupViewerDialog from '../src/lib/components/backup/BackupViewerDialog.svelte'

const mounted = []

function message(key, overrides = {}) {
  return {
    key,
    subject: `Subject ${key}`,
    accountEmail: 'first@example.test',
    folderPath: 'INBOX',
    date: '2026-08-01T08:00:00Z',
    attachmentCount: 0,
    ...overrides,
  }
}

function catalog(overrides = {}) {
  return {
    directory: '/synthetic/backup',
    messageCount: 3,
    needsIndex: true,
    accounts: [
      { accountEmail: 'first@example.test', messageCount: 2 },
      { accountEmail: 'second@example.test', messageCount: 1 },
    ],
    ...overrides,
  }
}

function detail(key, overrides = {}) {
  return {
    key,
    subject: `Detail ${key}`,
    date: '2026-08-01T08:00:00Z',
    attachments: [{ index: 1, filename: 'synthetic.pdf' }],
    ...overrides,
  }
}

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderViewer(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(BackupViewerDialog, {
    target,
    props: { open: true, ...props },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

function action(target, name) {
  const button = target.querySelector(`[data-backup-action="${name}"]`)
  assert.ok(button, `missing backup action ${name}`)
  button.click()
  return button
}

async function loadDefault(target) {
  action(target, 'load')
  await flushAsync()
}

beforeEach(() => {
  document.body.innerHTML = ''
  settings.dark = true
  settings.enhanced = true
  settings.theme = 'dark'
  backend.GetBackupSettings.mockReset().mockResolvedValue({ directory: '/last/backup' })
  backend.GetBackupViewerCatalog.mockReset().mockResolvedValue(catalog())
  backend.ListBackupViewerMessages.mockReset().mockImplementation(async (_dir, email, _sort, offset) => {
    if (email === 'second@example.test') return { messages: [message('second', { accountEmail: email })], total: 1 }
    if (offset > 0) return { messages: [message('third')], total: 3 }
    return { messages: [message('first', { attachmentCount: 1 }), message('second')], total: 3 }
  })
  backend.GetBackupViewerMessage.mockReset().mockImplementation(async (_dir, key) => detail(key))
  backend.BuildBackupViewerIndex.mockReset().mockResolvedValue(undefined)
  backend.OpenBackupViewerDirectory.mockReset().mockResolvedValue(undefined)
  backend.SaveBackupViewerAttachmentAs.mockReset().mockResolvedValue('/saved/synthetic.pdf')
  backend.SearchBackupViewerMessages.mockReset().mockResolvedValue({
    messages: [message('searched', { accountEmail: 'second@example.test', attachmentCount: 2 })],
    total: 1,
  })
  for (const fn of Object.values(toast)) fn.mockReset()
  for (const fn of Object.values(history)) fn.mockReset()
  for (const fn of Object.values(guards)) fn.mockReset()
  actionMenu.showForRoot.mockReset()
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
  if (!HTMLElement.prototype.scrollTo) HTMLElement.prototype.scrollTo = vi.fn()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('loads, pages, sorts, filters, navigates, and saves backup attachments', async () => {
  const { target } = await renderViewer()
  assert.deepEqual(history.remember.mock.calls.at(-1), ['/last/backup'])
  assert.equal(guards.open.mock.calls.length, 1)
  assert.match(target.textContent, /backupViewer\.noDirectory/)

  await loadDefault(target)
  assert.deepEqual(backend.GetBackupViewerCatalog.mock.calls.at(-1), ['/synthetic/backup'])
  assert.deepEqual(history.remember.mock.calls.at(-1), ['/synthetic/backup'])
  assert.deepEqual(backend.ListBackupViewerMessages.mock.calls.at(-1), [
    '/synthetic/backup', '', 'newest', 0, 200,
  ])
  assert.deepEqual(backend.GetBackupViewerMessage.mock.calls.at(-1), ['/synthetic/backup', 'first'])
  assert.equal(target.querySelectorAll('[data-backup-message-key]').length, 2)
  assert.equal(target.querySelector('[data-backup-message-key="first"]').getAttribute('aria-selected'), 'true')
  assert.equal(target.querySelector('[data-backup-detail]').dataset.key, 'first')

  action(target, 'refresh')
  await flushAsync()
  assert.equal(backend.GetBackupViewerCatalog.mock.calls.length >= 2, true)

  const list = target.querySelector('[role="listbox"]')
  list.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-detail]').dataset.key, 'second')
  assert.equal(document.activeElement.dataset.backupMessageKey, 'second')

  list.dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-detail]').dataset.key, 'first')
  list.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.deepEqual(backend.GetBackupViewerMessage.mock.calls.at(-1), ['/synthetic/backup', 'first'])

  const loadMore = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('backupViewer.loadMore'))
  assert.ok(loadMore)
  loadMore.click()
  await flushAsync()
  assert.equal(target.querySelectorAll('[data-backup-message-key]').length, 3)
  assert.deepEqual(backend.ListBackupViewerMessages.mock.calls.at(-1), [
    '/synthetic/backup', '', 'newest', 2, 200,
  ])

  action(target, 'sort')
  await flushAsync()
  assert.deepEqual(backend.ListBackupViewerMessages.mock.calls.at(-1), [
    '/synthetic/backup', '', 'oldest', 0, 200,
  ])
  action(target, 'scope')
  await flushAsync()
  assert.deepEqual(backend.ListBackupViewerMessages.mock.calls.at(-1), [
    '/synthetic/backup', 'second@example.test', 'oldest', 0, 200,
  ])
  assert.equal(target.querySelector('[data-backup-toolbar]').dataset.scope, 'second@example.test')

  target.querySelector('[data-backup-detail-action="save"]').click()
  await flushAsync()
  assert.deepEqual(backend.SaveBackupViewerAttachmentAs.mock.calls.at(-1), [
    '/synthetic/backup', 'second', 1,
  ])
  assert.match(toast.success.mock.calls.at(-1)[0], /toast\.attachmentSaved/)
  action(target, 'sort')
  await flushAsync()
  action(target, 'sort')
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-toolbar]').dataset.sort, 'oldest')
  action(target, 'remove-history')
  await flushAsync()
  assert.match(target.textContent, /backupViewer\.noDirectory/)
})

test('searches with debounce and IME guards, changes scope, and selects a result', async () => {
  vi.useFakeTimers()
  const { target } = await renderViewer()
  await loadDefault(target)
  action(target, 'search')
  await flushAsync()
  const input = target.querySelector('[data-backup-search-input]')
  assert.ok(input)

  input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
  input.value = 'ignored while composing'
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(250)
  assert.equal(backend.SearchBackupViewerMessages.mock.calls.length, 0)

  input.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(250)
  await flushAsync()
  assert.deepEqual(backend.SearchBackupViewerMessages.mock.calls.at(-1), [
    '/synthetic/backup', '', 'ignored while composing', 0, 50,
  ])
  assert.ok(target.querySelector('[data-backup-search-result="searched"]'))

  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-viewer-search]').dataset.scope, 'first@example.test')
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-viewer-search]'), null)
  assert.equal(target.querySelector('[data-backup-detail]').dataset.key, 'searched')
  assert.equal(target.querySelector('[data-backup-message-key="searched"]').getAttribute('aria-selected'), 'true')

  window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.ok(target.querySelector('[data-backup-viewer-search]'))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-viewer-search]'), null)

  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'F10', shiftKey: true, bubbles: true, cancelable: true }))
  assert.equal(actionMenu.showForRoot.mock.calls.length, 1)
})

test('reports catalog, index, directory, detail, attachment, and close failures safely', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  backend.GetBackupSettings.mockRejectedValueOnce(new Error('settings unavailable'))
  const onClose = vi.fn()
  const { target } = await renderViewer({ onClose })
  assert.equal(warn.mock.calls.length, 1)

  backend.GetBackupViewerCatalog.mockRejectedValueOnce(new Error('history missing'))
  action(target, 'load-history')
  await flushAsync()
  assert.deepEqual(history.remove.mock.calls.at(-1), ['/missing/backup'])
  assert.match(toast.error.mock.calls.at(-1)[0], /backupViewer\.historyPathMissing/)

  await loadDefault(target)
  backend.OpenBackupViewerDirectory.mockRejectedValueOnce(new Error('finder unavailable'))
  action(target, 'open-directory')
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.openDirectoryFailed/)

  backend.BuildBackupViewerIndex.mockRejectedValueOnce(new Error('index unavailable'))
  action(target, 'build')
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.buildIndexFailed/)

  backend.GetBackupViewerMessage.mockRejectedValueOnce(new Error('native detail failure'))
  target.querySelector('[data-backup-message-key="first"]').click()
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /native detail failure/)

  backend.GetBackupViewerMessage.mockRejectedValueOnce({ message: 'synthetic detail failure' })
  target.querySelector('[data-backup-message-key="second"]').click()
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /synthetic detail failure/)

  backend.GetBackupViewerMessage.mockResolvedValueOnce(detail('first'))
  target.querySelector('[data-backup-message-key="first"]').click()
  await flushAsync()
  backend.SaveBackupViewerAttachmentAs.mockRejectedValueOnce(new Error('save unavailable'))
  target.querySelector('[data-backup-detail-action="save"]').click()
  await flushAsync()
  assert.match(toast.error.mock.calls.at(-1)[0], /toast\.failedToSaveAttachment/)

  action(target, 'choose-error')
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.chooseDirectoryFailed/)

  action(target, 'close')
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-viewer-root]'), null)
  assert.equal(onClose.mock.calls.length, 1)
  assert.ok(guards.close.mock.calls.length >= 1)
  assert.ok(error.mock.calls.length >= 4)
})

test('guards empty actions and renders empty catalogs, plain failures, and busy index builds', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetBackupSettings.mockResolvedValueOnce({})
  const { target } = await renderViewer()
  action(target, 'open-directory')
  action(target, 'refresh')
  action(target, 'build')
  action(target, 'load-blank')
  target.querySelector('[data-backup-detail-action="save"]').click()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', metaKey: true, bubbles: true, cancelable: true }))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(backend.OpenBackupViewerDirectory.mock.calls.length, 0)
  assert.equal(backend.BuildBackupViewerIndex.mock.calls.length, 0)
  assert.equal(backend.GetBackupViewerCatalog.mock.calls.length, 0)
  assert.equal(backend.SaveBackupViewerAttachmentAs.mock.calls.length, 0)
  assert.equal(target.querySelector('[data-backup-viewer-search]'), null)

  backend.GetBackupViewerCatalog.mockResolvedValueOnce(catalog({
    directory: '', messageCount: 0, accounts: null, needsIndex: false,
  }))
  await loadDefault(target)
  assert.match(target.textContent, /backupViewer\.noBackup/)
  const listCalls = backend.ListBackupViewerMessages.mock.calls.length
  action(target, 'scope-all')
  await flushAsync()
  assert.equal(backend.ListBackupViewerMessages.mock.calls.length, listCalls)
  action(target, 'remove-other-history')
  assert.equal(target.querySelector('[data-backup-toolbar]').dataset.directory, '/synthetic/backup')
  action(target, 'open-directory')
  await flushAsync()
  assert.equal(backend.OpenBackupViewerDirectory.mock.calls.length, 1)

  let resolveBuild
  backend.BuildBackupViewerIndex.mockImplementationOnce(() => new Promise((resolve) => { resolveBuild = resolve }))
  action(target, 'build')
  action(target, 'build')
  await flushAsync()
  assert.equal(backend.BuildBackupViewerIndex.mock.calls.length, 1)
  resolveBuild()
  await flushAsync()

  backend.GetBackupViewerCatalog.mockRejectedValueOnce(new Error('plain catalog failure'))
  action(target, 'load-plain-failure')
  await flushAsync()
  assert.match(target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.loadFailed/)
  assert.equal(history.remove.mock.calls.length, 0)
  assert.equal(error.mock.calls.length >= 1, true)
})

test('handles empty pages, stale detail responses, error shapes, and fallback attachment indexes', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.ListBackupViewerMessages.mockResolvedValueOnce(null)
  const empty = await renderViewer()
  await loadDefault(empty.target)
  assert.match(empty.target.textContent, /backupViewer\.noMessages/)
  assert.equal(backend.GetBackupViewerMessage.mock.calls.length, 0)

  backend.ListBackupViewerMessages.mockRejectedValueOnce(new Error('message page unavailable'))
  await loadDefault(empty.target)
  assert.match(empty.target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.loadFailed/)

  let resolveFirst
  backend.GetBackupViewerMessage.mockReset().mockImplementation(async (_dir, key) => detail(key))
  backend.ListBackupViewerMessages.mockReset().mockResolvedValue({
    messages: [message('first', { attachmentCount: 1 }), message('second')],
    total: 3,
  })
  const race = await renderViewer()
  await loadDefault(race.target)
  backend.GetBackupViewerMessage.mockImplementation((_dir, key) => {
    if (key === 'first') return new Promise((resolve) => { resolveFirst = resolve })
    return Promise.resolve(detail(key, { subject: '' }))
  })
  race.target.querySelector('[data-backup-message-key="first"]').click()
  const secondRow = race.target.querySelector('[data-backup-message-key="second"]')
  assert.ok(secondRow)
  secondRow.click()
  await flushAsync()
  assert.equal(race.target.querySelector('[data-backup-detail]').dataset.key, 'second')
  assert.equal(race.target.querySelector('[data-backup-detail]').dataset.title, 'backupViewer.unknownSubject')
  resolveFirst(detail('first-stale'))
  await flushAsync()
  assert.equal(race.target.querySelector('[data-backup-detail]').dataset.key, 'second')

  for (const reason of [undefined, 'string failure', 42, { message: 99 }]) {
    backend.GetBackupViewerMessage.mockRejectedValueOnce(reason)
    race.target.querySelector('[data-backup-message-key="first"]').click()
    await flushAsync()
    assert.match(race.target.querySelector('[data-backup-toolbar]').dataset.error, /backupViewer\.messageLoadFailed/)
  }

  backend.GetBackupViewerMessage.mockResolvedValueOnce(detail('second'))
  race.target.querySelector('[data-backup-message-key="second"]').click()
  await flushAsync()
  backend.SaveBackupViewerAttachmentAs.mockResolvedValueOnce('')
  race.target.querySelector('[data-backup-detail-action="save-fallback"]').click()
  await flushAsync()
  assert.deepEqual(backend.SaveBackupViewerAttachmentAs.mock.calls.at(-1), [
    '/synthetic/backup', 'second', 7,
  ])
  assert.equal(toast.success.mock.calls.length, 0)
  assert.equal(error.mock.calls.length >= 5, true)
})

test('covers search failures, navigation bounds, native select-all, external targets, and menu Escape', async () => {
  vi.useFakeTimers()
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target } = await renderViewer()
  await loadDefault(target)
  action(target, 'search')
  await vi.advanceTimersByTimeAsync(35)
  await flushAsync()
  const input = target.querySelector('[data-backup-search-input]')
  const select = vi.spyOn(input, 'select')

  input.value = '   '
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  assert.equal(backend.SearchBackupViewerMessages.mock.calls.length, 0)

  backend.SearchBackupViewerMessages.mockRejectedValueOnce(new Error('search unavailable'))
  input.value = 'failure'
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(error.mock.calls.length, 1)

  backend.SearchBackupViewerMessages.mockResolvedValueOnce({
    messages: [message('first'), message('second')], total: 2,
  })
  input.value = 'existing'
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }))
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }))
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }))
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', metaKey: true, bubbles: true, cancelable: true }))
  assert.equal(select.mock.calls.length, 1)
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-detail]').dataset.key, 'first')

  action(target, 'search')
  await flushAsync()
  const reopened = target.querySelector('[data-backup-search-input]')
  reopened.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', isComposing: true, bubbles: true }))
  reopened.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', keyCode: 229, bubbles: true }))
  reopened.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true }))
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', metaKey: true, bubbles: true, cancelable: true }))
  assert.equal(document.activeElement, reopened)
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
  await flushAsync()

  const outside = document.createElement('button')
  document.body.appendChild(outside)
  outside.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true }))
  assert.equal(target.querySelector('[data-backup-viewer-search]'), null)

  action(target, 'toggle-menu')
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-toolbar]').dataset.menuOpen, 'true')
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[data-backup-toolbar]').dataset.menuOpen, 'false')

  settings.enhanced = false
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', bubbles: true, cancelable: true }))
  settings.enhanced = true
})
