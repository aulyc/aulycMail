// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const ui = vi.hoisted(() => ({ active: 'mail', setActive: vi.fn() }))
const keyboard = vi.hoisted(() => ({ focused: 'messageList', main: true, set: vi.fn() }))
const settings = vi.hoisted(() => ({ enabled: true }))
const backend = vi.hoisted(() => ({
  GetAttachments: vi.fn(), SaveAttachmentAs: vi.fn(), SaveAllAttachments: vi.fn(),
  OpenAttachment: vi.fn(), OpenFile: vi.fn(), OpenFolder: vi.fn(),
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))

vi.mock('$lib/stores/uiState.svelte', () => ({ getActivePane: () => ui.active, setActivePane: ui.setActive }))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  getFocusedPane: () => keyboard.focused,
  isMainKeyboardScope: () => keyboard.main,
  setFocusedPane: (pane) => { keyboard.focused = pane; keyboard.set(pane) },
}))
vi.mock('$lib/stores/settings.svelte', () => ({ getEnhancedKeyboardNavigation: () => settings.enabled }))
vi.mock('../wailsjs/go/app/App.js', () => backend)
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

import ActivityRail from '../src/lib/components/rail/ActivityRail.svelte'
import AttachmentList from '../src/lib/components/viewer/AttachmentList.svelte'

const mounted = []

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

function attachments() {
  const types = [
    'image/png', 'video/mp4', 'audio/mpeg', 'application/pdf', 'application/msword',
    'application/vnd.ms-excel', 'application/vnd.ms-powerpoint', 'application/zip', 'text/plain', 'text/html',
    'application/octet-stream',
  ]
  return types.map((contentType, index) => ({
    id: `attachment-${index}`, filename: `file-${index}.dat`, contentType, size: 1024 * (index + 1), isInline: index === 0,
  }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  ui.active = 'mail'
  ui.setActive.mockReset()
  keyboard.focused = 'messageList'
  keyboard.main = true
  keyboard.set.mockReset()
  settings.enabled = true
  backend.GetAttachments.mockReset().mockResolvedValue(attachments())
  backend.SaveAttachmentAs.mockReset().mockResolvedValue('/tmp/synthetic.dat')
  backend.SaveAllAttachments.mockReset().mockResolvedValue('/tmp/synthetic-folder')
  backend.OpenAttachment.mockReset().mockResolvedValue(undefined)
  backend.OpenFile.mockReset().mockResolvedValue(undefined)
  backend.OpenFolder.mockReset().mockResolvedValue(undefined)
  toast.success.mockReset()
  toast.error.mockReset()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('ActivityRail selects panes/settings and supports wrapped keyboard navigation', async () => {
  const onOpenSettings = vi.fn()
  const { target, instance } = await render(ActivityRail, { onOpenSettings })
  const nav = target.querySelector('nav')
  assert.ok(target.querySelector('button[title="Mail"]'))
  const contacts = target.querySelector('button[title="contacts.sidebar.title"]')
  contacts.click()
  assert.deepEqual(ui.setActive.mock.calls.at(-1), ['contacts'])
  assert.deepEqual(keyboard.set.mock.calls.at(-1), ['featureNav'])
  assert.equal(document.activeElement, nav)

  target.querySelector('button[title="sidebar.settings"]').click()
  assert.equal(onOpenSettings.mock.calls.length, 1)
  key(nav, 'ArrowDown')
  assert.deepEqual(ui.setActive.mock.calls.at(-1), ['mail'])
  key(nav, 'ArrowUp')
  assert.equal(onOpenSettings.mock.calls.length, 2)
  key(nav, 'Enter')
  assert.equal(onOpenSettings.mock.calls.length, 3)

  instance.selectSettingsEntry()
  instance.focusSettings()
  assert.equal(document.activeElement, nav)
  settings.enabled = false
  const before = ui.setActive.mock.calls.length
  key(nav, 'ArrowDown')
  assert.equal(ui.setActive.mock.calls.length, before)
  key(nav, 'ArrowDown', { isComposing: true })
  nav.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))
  nav.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  assert.equal(keyboard.focused, 'featureNav')
})

test('AttachmentList loads every file type and opens/downloads/saves attachments', async () => {
  const { target } = await render(AttachmentList, { messageId: 'message-1' })
  assert.deepEqual(backend.GetAttachments.mock.calls[0], ['message-1'])
  assert.equal(target.querySelectorAll('[data-keyboard-action-context]').length, 11)
  assert.match(target.textContent, /attachment\.inline/)

  target.querySelector('button[title="attachment.open"]').click()
  await flushAsync()
  assert.deepEqual(backend.OpenAttachment.mock.calls[0], ['attachment-0'])
  const downloadButtons = target.querySelectorAll('button[title="attachment.download"]')
  downloadButtons[0].click()
  await flushAsync()
  assert.deepEqual(backend.SaveAttachmentAs.mock.calls[0], ['attachment-0'])
  const downloadToast = toast.success.mock.calls.at(-1)
  assert.match(downloadToast[0], /toast\.attachmentSaved/)
  downloadToast[1][0].onClick()
  downloadToast[1][1].onClick()
  assert.deepEqual(backend.OpenFile.mock.calls[0], ['/tmp/synthetic.dat'])
  assert.deepEqual(backend.OpenFolder.mock.calls[0], ['/tmp/synthetic.dat'])

  const saveAll = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('attachment.saveAll'))
  saveAll.click()
  await flushAsync()
  assert.deepEqual(backend.SaveAllAttachments.mock.calls[0], ['message-1'])
  const allToast = toast.success.mock.calls.at(-1)
  allToast[1][0].onClick()
  assert.deepEqual(backend.OpenFolder.mock.calls.at(-1), ['/tmp/synthetic-folder'])
})

test('AttachmentList contains load/open/download/save-all errors and empty paths', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.OpenAttachment.mockRejectedValueOnce(new Error('open unavailable'))
  backend.SaveAttachmentAs.mockRejectedValueOnce(new Error('save unavailable'))
  backend.SaveAllAttachments.mockRejectedValueOnce(new Error('save all unavailable'))
  const { target } = await render(AttachmentList, { messageId: 'message-1' })
  target.querySelector('button[title="attachment.open"]').click()
  await flushAsync()
  target.querySelector('button[title="attachment.download"]').click()
  await flushAsync()
  const saveAll = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('attachment.saveAll'))
  saveAll.click()
  await flushAsync()
  assert.deepEqual(toast.error.mock.calls.map((call) => call[0]), [
    'toast.failedToOpenAttachment:{"filename":"file-0.dat"}',
    'toast.failedToSaveAttachment:{"filename":"file-0.dat"}',
    'toast.failedToSaveAttachments',
  ])
  assert.equal(error.mock.calls.length, 3)
  await unmount(mounted.pop())

  backend.GetAttachments.mockRejectedValueOnce(new Error('load unavailable'))
  let rendered = await render(AttachmentList, { messageId: 'message-error' })
  assert.equal(rendered.target.querySelectorAll('[data-keyboard-action-context]').length, 0)
  assert.equal(error.mock.calls.length, 4)
  await unmount(mounted.pop())

  await render(AttachmentList, { messageId: '' })
  assert.equal(backend.GetAttachments.mock.calls.some((call) => call[0] === ''), false)
})
