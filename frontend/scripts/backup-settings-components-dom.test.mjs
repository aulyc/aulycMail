// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  ChooseBackupDirectory: vi.fn(),
  GetBackupRunState: vi.fn(),
  GetBackupStatus: vi.fn(),
  GetLatestActivityLog: vi.fn(),
  OpenBackupDirectory: vi.fn(),
  StartEmailBackup: vi.fn(),
}))
const runtime = vi.hoisted(() => ({ callbacks: new Map(), unsubscribe: vi.fn() }))
const accounts = vi.hoisted(() => ({ accounts: [], loading: false }))
const toast = vi.hoisted(() => ({ add: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn(name, callback) {
    runtime.callbacks.set(name, callback)
    return runtime.unsubscribe
  },
}))
vi.mock('$lib/stores/accounts.svelte', () => ({ accountStore: accounts }))
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/dialog', async () => {
  const root = (await import('./fixtures/DialogRootTestStub.svelte')).default
  const content = (await import('./fixtures/DialogContentTestStub.svelte')).default
  const snippet = (await import('./fixtures/SnippetTestStub.svelte')).default
  return {
    Root: root,
    Content: content,
    Header: snippet,
    Title: snippet,
    Description: snippet,
  }
})

import {
  rememberBackupDirectory,
} from '../src/lib/utils/backup-directory-history.ts'
import BackupDirectoryPicker from '../src/lib/components/backup/BackupDirectoryPicker.svelte'
import BackupProgressDialog from '../src/lib/components/settings/backup/BackupProgressDialog.svelte'
import BackupScopePicker from '../src/lib/components/settings/backup/BackupScopePicker.svelte'
import BackupSettingsPage from '../src/lib/components/settings/backup/BackupSettingsPage.svelte'
import RecentBackupLog from '../src/lib/components/settings/backup/RecentBackupLog.svelte'
import { SettingsDraft } from '../src/lib/components/settings/settingsDraft.svelte.ts'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 8; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(component, props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return target
}

function buttonWithText(target, text) {
  const button = [...target.querySelectorAll('button')].find((item) => item.textContent.includes(text))
  assert.ok(button, `missing button containing ${text}`)
  return button
}

function backupLog(status = 'success', overrides = {}) {
  return {
    id: `log-${status}`,
    type: 'backup',
    status,
    createdAt: '2026-08-01T03:04:05Z',
    title: 'Backup',
    summary: 'Synthetic backup',
    payload: {
      mode: 'incremental',
      total: 8,
      backedUp: 6,
      added: 4,
      skipped: 2,
      missing: 1,
      unavailable: 0,
      failed: 1,
      directory: '/synthetic/backup',
    },
    ...overrides,
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  localStorage.clear()
  runtime.callbacks.clear()
  runtime.unsubscribe.mockReset()
  accounts.accounts = [
    { account: { id: 'a', email: 'a@example.test' } },
    { account: { id: 'b', email: 'b@example.test' } },
    { account: { id: 'shared', email: 'shared@example.test', sharedMailboxParentId: 'a' } },
  ]
  accounts.loading = false
  toast.add.mockReset()
  for (const fn of Object.values(backend)) fn.mockReset()
  backend.ChooseBackupDirectory.mockResolvedValue('/chosen/backup')
  backend.GetBackupRunState.mockResolvedValue({ running: false, progress: null })
  backend.GetBackupStatus.mockResolvedValue({ hasIndex: false })
  backend.GetLatestActivityLog.mockResolvedValue(null)
  backend.OpenBackupDirectory.mockResolvedValue(undefined)
  backend.StartEmailBackup.mockResolvedValue({
    running: false,
    progress: {
      phase: 'done', current: 8, total: 8, exported: 4, skipped: 2,
      missing: 1, unavailable: 0, failed: 1,
    },
  })
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
  localStorage.clear()
})

test('directory picker chooses, opens, reuses, removes, and dismisses history', async () => {
  rememberBackupDirectory('/history/one')
  rememberBackupDirectory('/history/two')
  const onChoose = vi.fn()
  const onSelectHistory = vi.fn()
  const onRemoveHistory = vi.fn()
  let resolveOpen
  const onOpenDirectory = vi.fn(() => new Promise((resolve) => { resolveOpen = resolve }))
  const target = await render(BackupDirectoryPicker, {
    directory: ' /current/backup ',
    onChoose,
    onSelectHistory,
    onRemoveHistory,
    onOpenDirectory,
  })

  const toggle = target.querySelector('[data-settings-keyboard-order="2"]')
  toggle.click()
  await flushAsync()
  assert.equal(toggle.getAttribute('aria-expanded'), 'true')
  assert.match(target.textContent, /\/history\/two/)

  buttonWithText(target, '/history/one').click()
  await flushAsync()
  assert.deepEqual(onSelectHistory.mock.calls.at(-1), ['/history/one'])
  assert.equal(target.querySelector('[role="listbox"]'), null)

  toggle.click()
  await flushAsync()
  const removeButtons = target.querySelectorAll('[aria-label="backupViewer.removeDirectoryHistory"]')
  removeButtons[0].click()
  await flushAsync()
  assert.deepEqual(onRemoveHistory.mock.calls.at(-1), ['/history/two'])
  assert.doesNotMatch(target.textContent, /\/history\/two/)

  buttonWithText(target, 'backupViewer.loadNewDirectory').click()
  await flushAsync()
  assert.deepEqual(onChoose.mock.calls.at(-1), ['/chosen/backup'])

  const openButton = target.querySelector('[aria-label="backupViewer.openDirectory"]')
  openButton.click()
  openButton.click()
  await flushAsync()
  assert.deepEqual(onOpenDirectory.mock.calls, [['/current/backup']])
  resolveOpen()
  await flushAsync()

  toggle.click()
  await flushAsync()
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.equal(target.querySelector('[role="listbox"]'), null)

  toggle.click()
  await flushAsync()
  document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
  await flushAsync()
  assert.equal(target.querySelector('[role="listbox"]'), null)
})

test('directory picker reports chooser errors and respects disabled states', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.ChooseBackupDirectory.mockRejectedValueOnce(new Error('picker unavailable'))
  const onChooseError = vi.fn()
  const target = await render(BackupDirectoryPicker, {
    directory: '',
    placeholder: 'Select a directory',
    onChooseError,
  })
  assert.equal(target.querySelector('[aria-label="backupViewer.openDirectory"]').disabled, true)
  target.querySelector('[data-settings-keyboard-order="2"]').click()
  await flushAsync()
  buttonWithText(target, 'backupViewer.loadNewDirectory').click()
  await flushAsync()
  assert.equal(onChooseError.mock.calls.length, 1)
  assert.equal(error.mock.calls.length, 1)

  const disabled = await render(BackupDirectoryPicker, { directory: '/backup', disabled: true })
  const disabledToggle = disabled.querySelector('[data-settings-keyboard-order="2"]')
  assert.equal(disabledToggle.disabled, true)
  assert.equal(disabled.querySelector('[aria-label="backupViewer.openDirectory"]').disabled, true)
})

test('scope picker filters shared mailboxes and updates all and selected labels', async () => {
  const target = await render(BackupScopePicker, {
    scope: 'selected',
    selectedAccountIds: ['a'],
  })
  const toggle = target.querySelector('button')
  assert.match(toggle.textContent, /a@example\.test/)
  toggle.click()
  await flushAsync()
  assert.doesNotMatch(target.textContent, /shared@example\.test/)

  buttonWithText(target, 'b@example.test').click()
  await flushAsync()
  assert.match(toggle.textContent, /settingsBackup\.scopeAll/)

  const menu = target.querySelector('.absolute')
  assert.ok(menu)
  buttonWithText(menu, 'settingsBackup.scopeAll').click()
  await flushAsync()
  assert.match(toggle.textContent, /settingsBackup\.selectedMailboxes:.*"count":0/)

  buttonWithText(menu, 'settingsBackup.scopeAll').click()
  await flushAsync()
  assert.match(toggle.textContent, /settingsBackup\.scopeAll/)

  buttonWithText(target, 'a@example.test').click()
  await flushAsync()
  assert.match(toggle.textContent, /b@example\.test/)

  toggle.click()
  await flushAsync()
  document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
  await flushAsync()
  assert.equal(target.querySelectorAll('button').length, 1)
})

test('progress dialog distinguishes preparing, running, completed-with-issues, and failed runs', async () => {
  const runInBackground = vi.fn()
  const preparing = await render(BackupProgressDialog, {
    open: true,
    store: { running: false, progress: null },
    preparing: true,
    onRunInBackground: runInBackground,
  })
  assert.match(preparing.textContent, /settingsBackup\.preparingBackup/)
  assert.ok(preparing.querySelector('.backup-progress-indeterminate'))
  buttonWithText(preparing, 'settingsBackup.runInBackground').click()
  assert.equal(runInBackground.mock.calls.length, 1)

  const checking = await render(BackupProgressDialog, {
    open: true,
    store: {
      running: true,
      progress: {
        phase: 'running', stage: 'checking', accountEmail: 'a@example.test', folderPath: 'Inbox',
        current: 17539, total: 21535, exported: 0, skipped: 17539, missing: 0, unavailable: 0, failed: 0,
      },
    },
  })
  assert.match(checking.textContent, /settingsBackup\.checkingExistingDescription/)
  assert.match(checking.textContent, /settingsBackup\.checkingExisting/)
  assert.doesNotMatch(checking.textContent, /17539/)
  assert.doesNotMatch(checking.textContent, /settingsBackup\.backedUpComposition/)
  assert.ok(checking.querySelector('.backup-progress-indeterminate'))
  assert.equal(checking.querySelector('[role="progressbar"]').hasAttribute('aria-valuenow'), false)

  const waitingForFirstBatch = await render(BackupProgressDialog, {
    open: true,
    store: {
      running: true,
      progress: {
        phase: 'running', stage: 'exporting', accountEmail: 'a@example.test', folderPath: 'Inbox',
        current: 14861, total: 14890, stageCurrent: 0, stageTotal: 29,
        exported: 0, skipped: 14861, missing: 0, unavailable: 0, failed: 0,
      },
    },
  })
  assert.equal(waitingForFirstBatch.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'), '0')
  assert.match(waitingForFirstBatch.textContent, /settingsBackup\.working/)
  assert.ok(waitingForFirstBatch.querySelector('.backup-working-indicator'))
  assert.ok(waitingForFirstBatch.querySelector('.backup-working-ellipsis'))
  assert.match(waitingForFirstBatch.textContent, /0%/)

  const running = await render(BackupProgressDialog, {
    open: true,
    store: {
      running: true,
      progress: {
        phase: 'running', stage: 'exporting', accountEmail: 'a@example.test', folderPath: 'Inbox',
        current: 21537, total: 21542, stageCurrent: 9, stageTotal: 14,
        exported: 5, skipped: 21528, missing: 4, unavailable: 0, failed: 0,
      },
    },
  })
  assert.equal(running.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'), '64')
  assert.match(running.textContent, /a@example\.test \/ Inbox/)
  assert.match(running.textContent, /64%/)
  assert.doesNotMatch(running.textContent, /100%/)

  const completed = await render(BackupProgressDialog, {
    open: true,
    store: {
      running: false,
      progress: {
        phase: 'done', current: 8, total: 8, exported: 4, skipped: 2,
        missing: 1, unavailable: 0, failed: 1,
      },
    },
  })
  assert.match(completed.textContent, /settingsBackup\.backupFinishedWithIssues/)
  assert.doesNotMatch(completed.textContent, /settingsBackup\.working/)
  assert.match(completed.textContent, /100%/)
  const completedText = completed.textContent.replace(/\s+/g, ' ')
  assert.match(completedText, /settingsBackup\.backedUpComposition 6/)
  assert.match(completedText, /settingsBackup\.notBackedUpComposition 2/)
  buttonWithText(completed, 'common.close').click()
  await flushAsync()
  assert.equal(completed.querySelector('[role="dialog"]'), null)

  const failed = await render(BackupProgressDialog, {
    open: true,
    store: { running: false, progress: { phase: 'error', current: 0, total: 0 } },
  })
  assert.match(failed.textContent, /settingsBackup\.backupFailedDescription/)
  assert.doesNotMatch(failed.textContent, /settingsBackup\.backedUpComposition/)
})

test('recent log renders success, refreshes on events, opens logs, and handles empty and failure states', async () => {
  const onOpenLogs = vi.fn()
  backend.GetLatestActivityLog.mockResolvedValueOnce(backupLog('success'))
  backend.GetBackupStatus.mockResolvedValueOnce({ hasIndex: true })
  const success = await render(RecentBackupLog, { directory: ' /synthetic/backup ', onOpenLogs })
  assert.deepEqual(backend.GetLatestActivityLog.mock.calls.at(-1), ['backup', '/synthetic/backup'])
  assert.match(success.textContent, /settingsBackup\.recentSuccess/)
  buttonWithText(success, 'settingsBackup.viewLogs').click()
  assert.equal(onOpenLogs.mock.calls.length, 1)

  backend.GetLatestActivityLog.mockResolvedValueOnce(backupLog('partial'))
  backend.GetBackupStatus.mockResolvedValueOnce({ hasIndex: true })
  runtime.callbacks.get('activity-log:created')({ type: 'backup' })
  await flushAsync()
  assert.match(success.textContent, /settingsBackup\.recentPartial/)

  const empty = await render(RecentBackupLog, { directory: '/indexed' })
  backend.GetLatestActivityLog.mockResolvedValueOnce(null)
  backend.GetBackupStatus.mockResolvedValueOnce({ hasIndex: true })
  runtime.callbacks.get('activity-log:created')({ type: 'backup' })
  await flushAsync()
  assert.match(empty.textContent, /settingsBackup\.indexWithoutLog/)

  backend.GetLatestActivityLog.mockRejectedValueOnce(new Error('database unavailable'))
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const failed = await render(RecentBackupLog, { directory: '/failed' })
  assert.match(failed.textContent, /activityLog\.loadFailed/)
  assert.ok(error.mock.calls.length >= 1)

  const noDirectory = await render(RecentBackupLog, { directory: '' })
  assert.match(noDirectory.textContent, /settingsBackup\.noRecentLog/)
})

test('backup settings page saves dirty settings, starts backup, and reports success and failure', async () => {
  backend.GetLatestActivityLog.mockResolvedValue(backupLog('success'))
  const draft = new SettingsDraft()
  draft.backupDirectory = '/synthetic/backup'
  draft.backupScope = 'selected'
  draft.backupSelectedAccountIds = ['a']
  const saveBackup = vi.spyOn(draft, 'saveBackup').mockResolvedValue(undefined)
  const target = await render(BackupSettingsPage, { draft, onOpenActivityLog: vi.fn() })
  backend.GetBackupRunState.mockResolvedValue({
    running: false,
    progress: {
      phase: 'done', current: 8, total: 8, exported: 4, skipped: 2,
      missing: 1, unavailable: 0, failed: 1,
    },
  })
  buttonWithText(target, 'settingsBackup.saveAndStart').click()
  await flushAsync()
  assert.equal(saveBackup.mock.calls.length, 1)
  assert.deepEqual(backend.StartEmailBackup.mock.calls.at(-1), [{
    directory: '/synthetic/backup',
    scope: 'selected',
    selectedAccountIds: ['a'],
  }])
  assert.match(target.textContent, /settingsBackup\.backupFinishedWithIssues/)

  backend.StartEmailBackup.mockRejectedValueOnce(new Error('backup failed'))
  backend.GetBackupRunState.mockResolvedValue({ running: false, progress: null })
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  buttonWithText(target, 'settingsBackup.saveAndStart').click()
  await flushAsync()
  assert.match(target.textContent, /settingsBackup\.backupFailedDescription/)
  assert.ok(toast.add.mock.calls.some(([entry]) => entry.type === 'error'))
  assert.ok(error.mock.calls.length >= 1)
})

test('backup settings page reopens an active run and sends it to background', async () => {
  backend.GetBackupRunState.mockResolvedValue({
    running: true,
    progress: {
      phase: 'running', current: 1, total: 4, exported: 1, skipped: 0,
      missing: 0, unavailable: 0, failed: 0,
    },
  })
  const draft = new SettingsDraft()
  draft.backupDirectory = '/synthetic/backup'
  draft.backupScope = 'all'
  draft.backupSelectedAccountIds = []
  const saveBackup = vi.spyOn(draft, 'saveBackup').mockResolvedValue(undefined)
  const target = await render(BackupSettingsPage, { draft })
  buttonWithText(target, 'settingsBackup.viewProgress').click()
  await flushAsync()
  buttonWithText(target, 'settingsBackup.runInBackground').click()
  await flushAsync()
  assert.equal(target.querySelector('[role="dialog"]'), null)
  assert.ok(toast.add.mock.calls.some(([entry]) => entry.type === 'success'))
  assert.equal(saveBackup.mock.calls.length, 0)
})
