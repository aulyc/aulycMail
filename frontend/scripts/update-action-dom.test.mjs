// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  getStatus: vi.fn(),
  check: vi.fn(),
  install: vi.fn(),
}))
const runtime = vi.hoisted(() => ({ updateListener: null }))
const toast = vi.hoisted(() => ({ add: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => ({
  GetUpdateStatus: backend.getStatus,
  CheckForUpdates: backend.check,
  InstallAvailableUpdate: backend.install,
}))
vi.mock('../wailsjs/runtime/runtime.js', () => ({
  EventsOn: (_name, callback) => {
    runtime.updateListener = callback
    return () => {}
  },
}))
vi.mock('$lib/stores/toast', () => ({ addToast: toast.add }))
vi.mock('$lib/i18n', () => ({
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

import UpdateAction from '../src/lib/components/settings/UpdateAction.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

function render(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(UpdateAction, { target, props })
  mounted.push(instance)
  return target
}

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  document.body.innerHTML = ''
  vi.clearAllMocks()
})

test('shares latest/update state, manually checks, and confirms installation', async () => {
  let finishInstall
  backend.getStatus.mockResolvedValue({ state: 'idle', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39, canInstall: false })
  backend.check.mockResolvedValue({ state: 'upToDate', currentVersion: '0.7.0', currentBuildNumber: 40, latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false })
  backend.install.mockImplementation(() => new Promise((resolve) => { finishInstall = resolve }))

  const compact = render({ compact: true })
  await flushAsync()
  const updateTitle = compact.querySelector('[data-update-title]')
  assert.match(updateTitle.textContent, /settingsUpdate\.systemUpdate/)
  assert.ok(updateTitle.classList.contains('text-muted-foreground'))
  assert.equal(updateTitle.classList.contains('text-foreground'), false)
  assert.match(compact.querySelector('[data-update-status]').textContent, /settingsUpdate\.checkNow/)
  assert.ok(compact.querySelector('button').classList.contains('items-start'))
  assert.ok(compact.querySelector('button').classList.contains('text-left'))
  assert.match(compact.textContent, /settingsUpdate\.checkNow/)
  compact.querySelector('button').click()
  await flushAsync()
  assert.equal(backend.check.mock.calls.length, 1)
  assert.match(compact.textContent, /settingsUpdate\.upToDate/)
  assert.ok(compact.querySelector('.text-emerald-500'))

  runtime.updateListener({
    state: 'available', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: true,
  })
  await flushAsync()
  assert.match(compact.textContent, /settingsUpdate\.available/)
  assert.match(compact.querySelector('[data-update-status]').textContent, /settingsUpdate\.availableCompact/)
  assert.doesNotMatch(compact.querySelector('[data-update-status]').textContent, /0\.7\.0/)
  assert.ok(compact.querySelector('.text-red-500'))

  const about = render()
  await flushAsync()
  about.querySelector('button').click()
  await flushAsync()
  assert.match(about.textContent, /settingsUpdate\.confirmTitle/)
  about.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(backend.install.mock.calls.length, 1)

  runtime.updateListener({
    state: 'downloading', progress: 37, currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false,
  })
  await flushAsync()
  let progress = about.querySelector('[data-confirm-progress]')
  assert.ok(progress)
  assert.match(progress.textContent, /settingsUpdate\.downloadingProgress/)
  assert.equal(progress.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'), '37')
  assert.equal(about.querySelector('[data-confirm-action="confirm"]').dataset.confirmFixedWidth, 'true')

  runtime.updateListener({
    state: 'verifying', progress: 82, currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false,
  })
  await flushAsync()
  progress = about.querySelector('[data-confirm-progress]')
  assert.match(progress.textContent, /settingsUpdate\.verifying/)
  assert.equal(progress.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'), '82')

  runtime.updateListener({
    state: 'installing', progress: 100, currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false,
  })
  await flushAsync()
  progress = about.querySelector('[data-confirm-progress]')
  assert.match(progress.textContent, /settingsUpdate\.installing/)
  assert.equal(progress.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'), '100')

  finishInstall({ state: 'installing', progress: 100, currentVersion: '0.6.0-beta.23', currentBuildNumber: 39, latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false })
  await flushAsync()

  runtime.updateListener({
    state: 'failed', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, failureOperation: 'install', canInstall: false,
  })
  await flushAsync()
  assert.match(compact.textContent, /settingsUpdate\.installFailedRetry/)

  runtime.updateListener({
    state: 'failed', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39,
    latestVersion: '0.7.0', latestBuildNumber: 40, failureOperation: 'check', canInstall: false,
  })
  await flushAsync()
  assert.match(compact.textContent, /settingsUpdate\.failed/)
})
