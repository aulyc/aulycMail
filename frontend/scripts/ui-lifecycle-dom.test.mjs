// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  MarkAllFolderMessagesAsRead: vi.fn(),
  MarkAllFolderMessagesAsUnread: vi.fn(),
  Undo: vi.fn(),
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('bits-ui', async () => ({
  ContextMenu: {
    Root: (await import('./fixtures/ContextMenuRootTestStub.svelte')).default,
    Trigger: (await import('./fixtures/SnippetTestStub.svelte')).default,
  },
}))
vi.mock('$lib/components/ui/context-menu', async () => ({
  ContextMenuContent: (await import('./fixtures/SnippetTestStub.svelte')).default,
  ContextMenuItem: (await import('./fixtures/DropdownItemTestStub.svelte')).default,
}))
vi.mock('$lib/stores/toast', () => ({ toasts: toast }))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key, options) => options?.values ? `${key}:${JSON.stringify(options.values)}` : key)
      return () => {}
    },
  },
}))

import FolderContextMenuHarness from './fixtures/FolderContextMenuHarness.svelte'
import ModalFrameHarness from './fixtures/ModalFrameHarness.svelte'

const mounted = []
let originalAnimationFrame

async function flushAsync() {
  for (let index = 0; index < 4; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(component, props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(component, { target, props })
  mounted.push(instance)
  await flushAsync()
  return target
}

function buttonWithText(target, text) {
  const found = [...target.querySelectorAll('button')].find((button) => button.textContent.includes(text))
  assert.ok(found, `missing button containing ${text}`)
  return found
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.MarkAllFolderMessagesAsRead.mockReset().mockResolvedValue(undefined)
  backend.MarkAllFolderMessagesAsUnread.mockReset().mockResolvedValue(undefined)
  backend.Undo.mockReset().mockResolvedValue('restored messages')
  toast.success.mockReset()
  toast.error.mockReset()
  originalAnimationFrame = globalThis.requestAnimationFrame
  globalThis.requestAnimationFrame = (callback) => {
    callback(performance.now())
    return 1
  }
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  globalThis.requestAnimationFrame = originalAnimationFrame
  vi.restoreAllMocks()
})

test('folder context menu executes both mutations, undo, disabled rendering, and all failures', async () => {
  let target = await render(FolderContextMenuHarness)
  assert.ok(target.querySelector('[data-context-menu-root]'))
  assert.ok(target.querySelector('[data-folder-child]'))

  buttonWithText(target, 'contextMenu.markAllAsRead').click()
  await flushAsync()
  assert.deepEqual(backend.MarkAllFolderMessagesAsRead.mock.calls, [['folder-1']])
  assert.equal(toast.success.mock.calls[0][0], 'toast.markedAllAsRead')
  await toast.success.mock.calls[0][1][0].onClick()
  assert.equal(backend.Undo.mock.calls.length, 1)
  assert.match(toast.success.mock.calls.at(-1)[0], /toast\.undone/)

  buttonWithText(target, 'contextMenu.markAllAsUnread').click()
  await flushAsync()
  assert.deepEqual(backend.MarkAllFolderMessagesAsUnread.mock.calls, [['folder-1']])
  assert.equal(toast.success.mock.calls.at(-1)[0], 'toast.markedAllAsUnread')

  target = await render(FolderContextMenuHarness, { disabled: true, folderId: 'disabled-folder' })
  assert.ok(target.querySelector('[data-folder-child]'))
  assert.equal(target.querySelector('[data-context-menu-root]'), null)

  const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.MarkAllFolderMessagesAsRead.mockRejectedValueOnce(new Error('read failed'))
  backend.MarkAllFolderMessagesAsUnread.mockRejectedValueOnce(new Error('unread failed'))
  backend.Undo.mockRejectedValueOnce(new Error('undo failed'))
  target = await render(FolderContextMenuHarness, { folderId: 'failure-folder' })
  buttonWithText(target, 'contextMenu.markAllAsRead').click()
  buttonWithText(target, 'contextMenu.markAllAsUnread').click()
  await flushAsync()
  assert.deepEqual(toast.error.mock.calls.slice(0, 2).map((call) => call[0]), [
    'toast.failedToMarkAllAsRead',
    'toast.failedToMarkAllAsUnread',
  ])

  backend.MarkAllFolderMessagesAsRead.mockResolvedValueOnce(undefined)
  buttonWithText(target, 'contextMenu.markAllAsRead').click()
  await flushAsync()
  await toast.success.mock.calls.at(-1)[1][0].onClick()
  assert.equal(toast.error.mock.calls.at(-1)[0], 'toast.undoFailed')
  assert.equal(consoleError.mock.calls.length, 3)
})

test('modal frame closes from backdrop and Escape while insulating panel clicks and ignored keys', async () => {
  const onClose = vi.fn()
  const target = await render(ModalFrameHarness, { onClose })
  const backdrop = target.querySelector('.fixed.inset-0')
  const panel = target.querySelector('[role="dialog"]')
  assert.equal(document.activeElement, panel)

  panel.click()
  assert.equal(onClose.mock.calls.length, 0)
  backdrop.click()
  assert.equal(onClose.mock.calls.length, 1)

  const ignored = [
    new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true, isComposing: true }),
    new KeyboardEvent('keydown', { key: 'Process', keyCode: 229, bubbles: true, cancelable: true }),
    new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }),
  ]
  const alreadyPrevented = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
  alreadyPrevented.preventDefault()
  ignored.push(alreadyPrevented)
  for (const event of ignored) panel.dispatchEvent(event)
  assert.equal(onClose.mock.calls.length, 1)

  const escape = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
  panel.dispatchEvent(escape)
  assert.equal(escape.defaultPrevented, true)
  assert.equal(onClose.mock.calls.length, 2)
})

test('modal frame traps Tab in both empty and populated panels and restores previous focus', async () => {
  const outside = document.createElement('button')
  document.body.appendChild(outside)
  outside.focus()

  let target = await render(ModalFrameHarness, { mode: 'none' })
  let panel = target.querySelector('[role="dialog"]')
  const emptyTab = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
  panel.dispatchEvent(emptyTab)
  assert.equal(emptyTab.defaultPrevented, true)
  assert.equal(document.activeElement, panel)

  await unmount(mounted.pop())
  await flushAsync()
  assert.equal(document.activeElement, outside)

  target = await render(ModalFrameHarness)
  panel = target.querySelector('[role="dialog"]')
  const first = target.querySelector('[data-first]')
  const last = target.querySelector('[data-last]')

  panel.focus()
  let tab = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
  panel.dispatchEvent(tab)
  assert.equal(tab.defaultPrevented, true)
  assert.equal(document.activeElement, first)

  last.focus()
  tab = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
  panel.dispatchEvent(tab)
  assert.equal(document.activeElement, first)

  first.focus()
  const shiftTab = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
  panel.dispatchEvent(shiftTab)
  assert.equal(shiftTab.defaultPrevented, true)
  assert.equal(document.activeElement, last)
})
