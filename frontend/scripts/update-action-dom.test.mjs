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
  backend.getStatus.mockResolvedValue({ state: 'idle', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39, canInstall: false })
  backend.check.mockResolvedValue({ state: 'upToDate', currentVersion: '0.7.0', currentBuildNumber: 40, latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false })
  backend.install.mockResolvedValue({ state: 'installing', currentVersion: '0.6.0-beta.23', currentBuildNumber: 39, latestVersion: '0.7.0', latestBuildNumber: 40, canInstall: false })

  const compact = render({ compact: true })
  await flushAsync()
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
  assert.ok(compact.querySelector('.text-red-500'))

  const about = render()
  await flushAsync()
  about.querySelector('button').click()
  await flushAsync()
  assert.match(about.textContent, /settingsUpdate\.confirmTitle/)
  about.querySelector('[data-confirm-action="confirm"]').click()
  await flushAsync()
  assert.equal(backend.install.mock.calls.length, 1)
})
