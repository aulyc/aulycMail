// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetAttachments: vi.fn(),
  SaveAttachmentAs: vi.fn(),
  SaveAllAttachments: vi.fn(),
  OpenAttachment: vi.fn(),
  OpenFile: vi.fn(),
  OpenFolder: vi.fn(),
}))
const toast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
}))

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

import '../src/lib/iconify-offline'
import AttachmentList from '../src/lib/components/viewer/AttachmentList.svelte'

const mounted = []

function attachment(id, contentType, overrides = {}) {
  return {
    id,
    messageId: 'message-1',
    filename: `${id}.dat`,
    contentType,
    size: 2048,
    contentId: '',
    isInline: false,
    localPath: '',
    ...overrides,
  }
}

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function render(messageId = 'message-1') {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(AttachmentList, { target, props: { messageId } })
  mounted.push(instance)
  await flushAsync()
  return target
}

function buttonByTitle(target, title) {
  return [...target.querySelectorAll('button')].find((button) => button.title === title)
}

function buttonWithText(target, text) {
  return [...target.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.GetAttachments.mockReset().mockResolvedValue([])
  backend.SaveAttachmentAs.mockReset().mockResolvedValue('')
  backend.SaveAllAttachments.mockReset().mockResolvedValue('')
  backend.OpenAttachment.mockReset().mockResolvedValue(undefined)
  backend.OpenFile.mockReset().mockResolvedValue(undefined)
  backend.OpenFolder.mockReset().mockResolvedValue(undefined)
  toast.success.mockReset()
  toast.error.mockReset()
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('renders every attachment icon class and completes open, save, and save-all actions', async () => {
  const attachments = [
    attachment('image', 'image/png', { filename: 'image.png', isInline: true }),
    attachment('video', 'video/mp4'),
    attachment('audio', 'audio/mpeg'),
    attachment('pdf', 'application/pdf'),
    attachment('word', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'),
    attachment('excel', 'application/vnd.ms-excel'),
    attachment('powerpoint', 'application/vnd.ms-powerpoint'),
    attachment('archive', 'application/zip'),
    attachment('text', 'text/plain'),
    attachment('html', 'text/html'),
    attachment('unknown', 'application/octet-stream'),
  ]
  backend.GetAttachments.mockResolvedValue(attachments)
  backend.SaveAttachmentAs.mockResolvedValue('/tmp/image.png')
  backend.SaveAllAttachments.mockResolvedValue('/tmp/all-attachments')
  const target = await render()

  assert.match(target.textContent, /image\.png/)
  assert.match(target.textContent, /attachment\.inline/)
  assert.equal(target.querySelectorAll('[data-keyboard-action-context]').length, attachments.length)

  buttonByTitle(target, 'attachment.open').click()
  await flushAsync()
  assert.deepEqual(backend.OpenAttachment.mock.calls, [['image']])

  buttonByTitle(target, 'attachment.download').click()
  await flushAsync()
  assert.deepEqual(backend.SaveAttachmentAs.mock.calls, [['image']])
  assert.equal(toast.success.mock.calls.length, 1)
  const saveActions = toast.success.mock.calls[0][1]
  assert.equal(saveActions.length, 2)
  saveActions[0].onClick()
  saveActions[1].onClick()
  assert.deepEqual(backend.OpenFile.mock.calls, [['/tmp/image.png']])
  assert.deepEqual(backend.OpenFolder.mock.calls, [['/tmp/image.png']])

  buttonWithText(target, 'attachment.saveAll').click()
  await flushAsync()
  assert.deepEqual(backend.SaveAllAttachments.mock.calls, [['message-1']])
  assert.equal(toast.success.mock.calls.length, 2)
  toast.success.mock.calls[1][1][0].onClick()
  assert.deepEqual(backend.OpenFolder.mock.calls.at(-1), ['/tmp/all-attachments'])
})

test('exposes loading states and treats canceled file and folder choices as no-ops', async () => {
  let resolveLoad
  backend.GetAttachments.mockReturnValue(new Promise((resolve) => { resolveLoad = resolve }))
  const target = await render()
  assert.match(target.textContent, /Loading attachments/)
  resolveLoad([
    attachment('first', 'text/plain'),
    attachment('second', 'application/pdf'),
  ])
  await flushAsync()

  let resolveSave
  backend.SaveAttachmentAs.mockReturnValue(new Promise((resolve) => { resolveSave = resolve }))
  const firstRow = target.querySelector('[data-keyboard-action-context="first.dat"]')
  firstRow.querySelector('button[title="attachment.download"]').click()
  await tick()
  assert.equal(firstRow.querySelector('button[title="attachment.download"]'), null)
  resolveSave('')
  await flushAsync()
  assert.equal(toast.success.mock.calls.length, 0)

  let resolveAll
  backend.SaveAllAttachments.mockReturnValue(new Promise((resolve) => { resolveAll = resolve }))
  buttonWithText(target, 'attachment.saveAll').click()
  await tick()
  assert.match(target.textContent, /attachment\.saving/)
  resolveAll('')
  await flushAsync()
  assert.equal(toast.success.mock.calls.length, 0)
})

test('reports backend failures and clears stale or absent attachment lists', async () => {
  backend.GetAttachments.mockResolvedValue(null)
  let target = await render()
  assert.equal(target.querySelectorAll('[data-keyboard-action-context]').length, 0)

  backend.GetAttachments.mockRejectedValueOnce(new Error('load failed'))
  target = await render('message-with-load-error')
  assert.equal(target.querySelectorAll('[data-keyboard-action-context]').length, 0)
  assert.match(console.error.mock.calls[0][0], /Failed to load attachments/)

  backend.GetAttachments.mockResolvedValue([
    attachment('first', 'text/plain'),
    attachment('second', 'application/pdf'),
  ])
  target = await render('message-actions')
  backend.OpenAttachment.mockRejectedValueOnce(new Error('open failed'))
  buttonByTitle(target, 'attachment.open').click()
  await flushAsync()
  assert.match(toast.error.mock.calls[0][0], /failedToOpenAttachment/)

  backend.SaveAttachmentAs.mockRejectedValueOnce(new Error('save failed'))
  buttonByTitle(target, 'attachment.download').click()
  await flushAsync()
  assert.match(toast.error.mock.calls[1][0], /failedToSaveAttachment/)

  backend.SaveAllAttachments.mockRejectedValueOnce(new Error('save all failed'))
  buttonWithText(target, 'attachment.saveAll').click()
  await flushAsync()
  assert.match(toast.error.mock.calls[2][0], /failedToSaveAttachments/)

  backend.GetAttachments.mockClear()
  target = await render('')
  assert.equal(backend.GetAttachments.mock.calls.length, 0)
  assert.equal(target.textContent.trim(), '')
})
