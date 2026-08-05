// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { afterEach, beforeEach, test, vi } from 'vitest'

const boot = vi.hoisted(() => ({
  initI18n: vi.fn(), mount: vi.fn(), IsReady: vi.fn(), EventsOn: vi.fn(),
  WindowCenter: vi.fn(), WindowSetSize: vi.fn(), WindowShow: vi.fn(), WindowUnmaximise: vi.fn(),
  readyHandler: null,
}))

vi.mock('../src/lib/i18n', () => ({ initI18n: boot.initI18n }))
vi.mock('../src/App.svelte', () => ({ default: { name: 'MockApp' } }))
vi.mock('svelte', () => ({ mount: boot.mount }))
vi.mock('../wailsjs/go/app/App.js', () => ({ IsReady: boot.IsReady }))
vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn: boot.EventsOn,
  WindowCenter: boot.WindowCenter,
  WindowSetSize: boot.WindowSetSize,
  WindowShow: boot.WindowShow,
  WindowUnmaximise: boot.WindowUnmaximise,
}))

async function flushAsync() {
  for (let index = 0; index < 12; index += 1) await Promise.resolve()
}

async function importMain() {
  await import('../src/main.ts')
  await flushAsync()
}

beforeEach(() => {
  vi.resetModules()
  vi.useFakeTimers()
  document.body.innerHTML = '<div id="app"></div>'
  Object.defineProperty(window, 'runtime', { configurable: true, writable: true, value: { WindowShow() {} } })
  boot.readyHandler = null
  for (const fn of [
    boot.initI18n, boot.mount, boot.IsReady, boot.EventsOn, boot.WindowCenter,
    boot.WindowSetSize, boot.WindowShow, boot.WindowUnmaximise,
  ]) fn.mockReset()
  boot.initI18n.mockResolvedValue(undefined)
  boot.IsReady.mockResolvedValue(true)
  boot.EventsOn.mockImplementation((name, callback) => {
    if (name === 'app:ready') boot.readyHandler = callback
    return vi.fn()
  })
})

afterEach(async () => {
  await vi.runOnlyPendingTimersAsync()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('bootstraps in order, restores the startup frame, and mounts after backend readiness', async () => {
  await importMain()
  await vi.advanceTimersByTimeAsync(100)
  await flushAsync()

  assert.equal(boot.WindowShow.mock.calls.length, 1)
  assert.equal(boot.WindowUnmaximise.mock.calls.length, 2)
  assert.deepEqual(boot.WindowSetSize.mock.calls, [[1300, 800], [1300, 800]])
  assert.equal(boot.WindowCenter.mock.calls.length, 1)
  assert.equal(boot.initI18n.mock.calls.length, 1)
  assert.deepEqual(boot.EventsOn.mock.calls[0].slice(0, 1), ['app:ready'])
  assert.equal(boot.IsReady.mock.calls.length, 1)
  assert.equal(boot.mount.mock.calls.length, 1)
  assert.equal(boot.mount.mock.calls[0][1].target, document.getElementById('app'))

  boot.readyHandler()
  boot.readyHandler()
  assert.equal(boot.mount.mock.calls.length, 1)
})

test('waits for the app-ready event when the readiness fallback is false or rejects', async () => {
  boot.IsReady.mockResolvedValueOnce(false)
  await importMain()
  await vi.advanceTimersByTimeAsync(100)
  await flushAsync()
  assert.equal(boot.mount.mock.calls.length, 0)
  boot.readyHandler()
  await flushAsync()
  assert.equal(boot.mount.mock.calls.length, 1)

  vi.resetModules()
  boot.mount.mockClear()
  boot.readyHandler = null
  boot.IsReady.mockRejectedValueOnce(new Error('binding unavailable'))
  await importMain()
  await vi.advanceTimersByTimeAsync(100)
  await flushAsync()
  assert.equal(boot.mount.mock.calls.length, 0)
  boot.readyHandler()
  await flushAsync()
  assert.equal(boot.mount.mock.calls.length, 1)
})

test('times out a missing injected runtime without wedging startup', async () => {
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  window.runtime = undefined
  await importMain()
  await vi.advanceTimersByTimeAsync(2200)
  await flushAsync()
  assert.equal(warn.mock.calls.length, 1)
  assert.match(warn.mock.calls[0][0], /Wails runtime never injected/)
  assert.equal(boot.WindowShow.mock.calls.length, 1)
  assert.equal(boot.mount.mock.calls.length, 1)
})
