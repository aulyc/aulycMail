// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({
  GetInlineAttachments: vi.fn(),
  AddImageAllowlist: vi.fn(),
  OpenURL: vi.fn(),
}))
const cache = vi.hoisted(() => ({ value: null, get: vi.fn(), set: vi.fn() }))
const imagePolicy = vi.hoisted(() => ({ allowed: false, refresh: vi.fn() }))
const settings = vi.hoisted(() => ({ alwaysLoad: false, enhanced: true, theme: 'dark' }))
const keyboard = vi.hoisted(() => ({
  setFocusedPane: vi.fn(),
  previous: vi.fn(),
  next: vi.fn(),
}))
const toast = vi.hoisted(() => ({ info: vi.fn() }))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('$lib/stores/inlineAttachmentCache', () => ({
  getCached: (id) => cache.get(id),
  setCache: (id, value) => cache.set(id, value),
}))
vi.mock('$lib/stores/imageAllowlist.svelte', () => ({
  isImageAllowedSync: () => imagePolicy.allowed,
  refreshImageAllowlist: imagePolicy.refresh,
}))
vi.mock('$lib/stores/keyboard.svelte', () => ({
  setFocusedPane: keyboard.setFocusedPane,
  focusPreviousPane: keyboard.previous,
  focusNextPane: keyboard.next,
}))
vi.mock('$lib/stores/settings.svelte', () => ({
  getAlwaysLoadImages: () => settings.alwaysLoad,
  getEnhancedKeyboardNavigation: () => settings.enhanced,
  getThemeMode: () => settings.theme,
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
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('$lib/components/ui/dropdown-menu', async () => ({
  Root: (await import('./fixtures/DropdownRootTestStub.svelte')).default,
  Trigger: (await import('./fixtures/DropdownTriggerTestStub.svelte')).default,
  Content: (await import('./fixtures/SnippetTestStub.svelte')).default,
  Item: (await import('./fixtures/DropdownItemTestStub.svelte')).default,
}))

import EmailBody from '../src/lib/components/viewer/EmailBody.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 5; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderEmail(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(EmailBody, {
    target,
    props: {
      messageId: 'message-1',
      fromEmail: 'sender@example.test',
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return { instance, target }
}

function findButton(root, text) {
  return [...root.querySelectorAll('button')].find((button) => button.textContent.includes(text))
}

function sendIframeMessage(iframe, data, source = iframe.contentWindow) {
  window.dispatchEvent(new MessageEvent('message', { data, source }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  settings.alwaysLoad = false
  settings.enhanced = true
  settings.theme = 'dark'
  imagePolicy.allowed = false
  cache.value = null
  cache.get.mockReset().mockImplementation(() => cache.value)
  cache.set.mockReset()
  imagePolicy.refresh.mockReset()
  backend.GetInlineAttachments.mockReset().mockResolvedValue({})
  backend.AddImageAllowlist.mockReset().mockResolvedValue(undefined)
  backend.OpenURL.mockReset().mockResolvedValue(undefined)
  keyboard.setFocusedPane.mockReset()
  keyboard.previous.mockReset()
  keyboard.next.mockReset()
  toast.info.mockReset()
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  })
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('escapes and linkifies plain text while routing mailto and web links safely', async () => {
  const onCompose = vi.fn()
  const { target } = await renderEmail({
    bodyText: '<unsafe> https://example.test/path?q=1 sender@example.test',
    onCompose,
  })

  assert.match(target.innerHTML, /&lt;unsafe&gt;/)
  assert.equal(target.querySelector('unsafe'), null)
  const links = [...target.querySelectorAll('a')]
  assert.equal(links.length, 2)

  links.find((link) => link.href.startsWith('mailto:')).click()
  assert.deepEqual(onCompose.mock.calls.at(-1), ['sender@example.test'])

  links.find((link) => link.href.startsWith('https:')).dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.deepEqual(backend.OpenURL.mock.calls.at(-1), ['https://example.test/path?q=1'])
  assert.deepEqual(toast.info.mock.calls.at(-1), ['toast.linkOpened'])
})

test('blocks remote HTML resources, embeds resolved inline images, and supports one-time loading', async () => {
  backend.GetInlineAttachments.mockResolvedValue({ cid1: 'data:image/png;base64,AA==' })
  const onImagesLoaded = vi.fn()
  const { target } = await renderEmail({
    bodyHtml: '<img src="https://images.example.test/a.png"><div style="background:url(https://images.example.test/b.png)"></div><table background="https://images.example.test/c.png"></table><img src="cid:cid1">',
    onImagesLoaded,
    darken: true,
  })

  const iframe = target.querySelector('iframe')
  assert.ok(iframe)
  assert.match(target.textContent, /viewer\.remoteImagesBlocked/)
  assert.match(iframe.srcdoc, /data-blocked-src=/)
  assert.match(iframe.srcdoc, /background:url\(\)/)
  assert.match(iframe.srcdoc, /background=""/)
  assert.match(iframe.srcdoc, /data:image\/png;base64,AA==/)
  assert.doesNotMatch(iframe.srcdoc, /data-cid="cid1"/)
  assert.doesNotMatch(iframe.srcdoc, /Loading\.\.\./)
  assert.match(iframe.srcdoc, /composer|content-filter|filter:/)
  assert.deepEqual(backend.GetInlineAttachments.mock.calls.at(-1), ['message-1'])
  assert.deepEqual(cache.set.mock.calls.at(-1), ['message-1', { cid1: 'data:image/png;base64,AA==' }])

  iframe.dispatchEvent(new Event('load'))
  await flushAsync()
  assert.equal(iframe.getAttribute('aria-busy'), 'false')

  findButton(target, 'viewer.loadImages').click()
  await flushAsync()
  assert.equal(target.textContent.includes('viewer.remoteImagesBlocked'), false)
  assert.equal(onImagesLoaded.mock.calls.length, 1)
  assert.match(iframe.srcdoc, /https:\/\/images\.example\.test\/a\.png/)
  assert.doesNotMatch(iframe.srcdoc, /data-blocked-src=/)
})

test('shows a bounded loading placeholder only while inline attachments are pending', async () => {
  let resolveInlineAttachments
  backend.GetInlineAttachments.mockReturnValue(new Promise((resolve) => {
    resolveInlineAttachments = resolve
  }))

  const { target } = await renderEmail({
    bodyHtml: '<img width="1200" height="900" src="cid:cid-with-special@host.test.png">',
  })
  const iframe = target.querySelector('iframe')
  assert.match(iframe.srcdoc, /Loading\.\.\./)
  assert.match(iframe.srcdoc, /data-inline-placeholder="loading"/)
  assert.match(iframe.srcdoc, /width: 120px !important/)

  resolveInlineAttachments({
    'cid-with-special@host.test.png': 'data:image/png;base64,AA==',
  })
  await flushAsync()

  assert.match(iframe.srcdoc, /data:image\/png;base64,AA==/)
  assert.doesNotMatch(iframe.srcdoc, /Loading\.\.\./)
  assert.doesNotMatch(iframe.srcdoc, /data-inline-placeholder=/)
})

test('ends inline loading when the requested CID is absent from cached content', async () => {
  backend.GetInlineAttachments.mockResolvedValue({})
  const { target } = await renderEmail({ bodyHtml: '<img src="cid:not-cached">' })
  const iframe = target.querySelector('iframe')

  assert.match(iframe.srcdoc, /Image unavailable/)
  assert.match(iframe.srcdoc, /data-inline-placeholder="unavailable"/)
  assert.doesNotMatch(iframe.srcdoc, /Loading\.\.\./)
})

test('ends inline loading when the local attachment bridge does not respond', async () => {
  vi.useFakeTimers()
  try {
    backend.GetInlineAttachments.mockReturnValue(new Promise(() => {}))
    const { target } = await renderEmail({ bodyHtml: '<img src="cid:bridge-timeout">' })
    const iframe = target.querySelector('iframe')
    assert.match(iframe.srcdoc, /Loading\.\.\./)

    await vi.advanceTimersByTimeAsync(5000)
    await flushAsync()

    assert.match(iframe.srcdoc, /Image unavailable/)
    assert.doesNotMatch(iframe.srcdoc, /Loading\.\.\./)
  } finally {
    vi.useRealTimers()
  }
})

test('honors global and sender image policies and updates the allowlist', async () => {
  settings.alwaysLoad = true
  let rendered = await renderEmail({ bodyHtml: '<img src="https://images.example.test/a.png">' })
  assert.equal(rendered.target.textContent.includes('viewer.remoteImagesBlocked'), false)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  settings.alwaysLoad = false
  imagePolicy.allowed = true
  rendered = await renderEmail({ bodyHtml: '<img src="https://images.example.test/a.png">' })
  assert.equal(rendered.target.textContent.includes('viewer.remoteImagesBlocked'), false)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  imagePolicy.allowed = false
  rendered = await renderEmail({ bodyHtml: '<img src="https://images.example.test/a.png">' })
  findButton(rendered.target, 'viewer.forDomain').click()
  await flushAsync()
  assert.deepEqual(backend.AddImageAllowlist.mock.calls.at(-1), ['domain', 'example.test'])
  assert.equal(imagePolicy.refresh.mock.calls.length, 1)

  await unmount(mounted.pop())
  document.body.innerHTML = ''
  rendered = await renderEmail({ bodyHtml: '<img src="https://images.example.test/a.png">' })
  findButton(rendered.target, 'viewer.forSender').click()
  await flushAsync()
  assert.deepEqual(backend.AddImageAllowlist.mock.calls.at(-1), ['sender', 'sender@example.test'])
})

test('validates iframe messages and routes focus, navigation, tooltips, and context actions', async () => {
  const onCompose = vi.fn()
  const shortcut = vi.fn()
  window.addEventListener('keydown', shortcut)
  const { target } = await renderEmail({ bodyHtml: '<p>Hello</p>', onCompose })
  const iframe = target.querySelector('iframe')
  assert.ok(iframe)
  vi.spyOn(iframe, 'getBoundingClientRect').mockReturnValue({
    left: 100, top: 200, right: 500, bottom: 500, width: 400, height: 300, x: 100, y: 200, toJSON() {},
  })
  const postMessage = vi.spyOn(iframe.contentWindow, 'postMessage')

  sendIframeMessage(iframe, { type: 'iframe-height', height: 321 })
  assert.equal(iframe.style.height, '341px')
  sendIframeMessage(iframe, { type: 'iframe-height', height: Infinity })
  assert.equal(iframe.style.height, '341px')
  sendIframeMessage(iframe, { type: 'iframe-focus' })
  assert.deepEqual(keyboard.setFocusedPane.mock.calls.at(-1), ['viewer'])

  sendIframeMessage(iframe, { type: 'open-link', url: 'mailto:friend@example.test' })
  assert.deepEqual(onCompose.mock.calls.at(-1), ['friend@example.test'])
  sendIframeMessage(iframe, { type: 'open-link', url: 'javascript:alert(1)' })
  await flushAsync()
  assert.equal(backend.OpenURL.mock.calls.length, 0)
  sendIframeMessage(iframe, { type: 'open-link', url: 'https://safe.example.test/path?token=synthetic' })
  await flushAsync()
  assert.deepEqual(backend.OpenURL.mock.calls.at(-1), ['https://safe.example.test/path?token=synthetic'])

  sendIframeMessage(iframe, {
    type: 'iframe-keydown', key: 'ArrowLeft', code: 'ArrowLeft', altKey: true,
    ctrlKey: false, metaKey: false, shiftKey: false,
  })
  sendIframeMessage(iframe, {
    type: 'iframe-keydown', key: 'l', code: 'KeyL', altKey: true,
    ctrlKey: false, metaKey: false, shiftKey: false,
  })
  assert.equal(keyboard.previous.mock.calls.length, 1)
  assert.equal(keyboard.next.mock.calls.length, 1)

  sendIframeMessage(iframe, {
    type: 'iframe-keydown', key: 'f', code: 'KeyF', altKey: false,
    ctrlKey: true, metaKey: false, shiftKey: false,
  })
  assert.ok(shortcut.mock.calls.some(([event]) => event.key === 'f' && event.ctrlKey))

  sendIframeMessage(iframe, { type: 'link-hover', url: 'https://safe.example.test', x: 10, y: 20 })
  await flushAsync()
  assert.match(target.textContent, /https:\/\/safe\.example\.test/)
  const tooltip = [...target.querySelectorAll('div')].find((element) => element.textContent === 'https://safe.example.test')
  assert.match(tooltip.getAttribute('style'), /left: 110px/)
  sendIframeMessage(iframe, { type: 'link-hover-end' })
  await flushAsync()
  assert.equal(target.textContent.includes('https://safe.example.test'), false)

  sendIframeMessage(iframe, {
    type: 'contextmenu', text: 'Selected synthetic text', url: 'https://safe.example.test', x: 5, y: 6,
  })
  await flushAsync()
  const menu = target.querySelector('[role="menu"]')
  assert.ok(menu)
  findButton(menu, 'viewer.copy').click()
  await flushAsync()
  assert.deepEqual(navigator.clipboard.writeText.mock.calls.at(-1), ['Selected synthetic text'])

  sendIframeMessage(iframe, {
    type: 'contextmenu', text: '', url: 'https://safe.example.test', x: 5, y: 6,
  })
  await flushAsync()
  findButton(target.querySelector('[role="menu"]'), 'viewer.copyLink').click()
  await flushAsync()
  assert.deepEqual(navigator.clipboard.writeText.mock.calls.at(-1), ['https://safe.example.test'])

  sendIframeMessage(iframe, {
    type: 'contextmenu', text: '', url: '', x: 5, y: 6,
  })
  await flushAsync()
  findButton(target.querySelector('[role="menu"]'), 'viewer.selectAll').click()
  assert.deepEqual(postMessage.mock.calls.at(-1), [{ type: 'select-all' }, '*'])

  const priorHeight = iframe.style.height
  sendIframeMessage(iframe, { type: 'iframe-height', height: 999 }, window)
  assert.equal(iframe.style.height, priorHeight)
  window.removeEventListener('keydown', shortcut)
})

test('handles image and clipboard failures without exposing broken UI state', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetInlineAttachments.mockRejectedValue(new Error('synthetic inline failure'))
  const { target } = await renderEmail({
    bodyHtml: '<img src="cid:missing"><img src="https://images.example.test/a.png">',
    fromEmail: 'invalid-address',
  })
  assert.equal(backend.GetInlineAttachments.mock.calls.length, 1)
  const iframe = target.querySelector('iframe')
  assert.match(iframe.srcdoc, /Image unavailable/)
  assert.match(iframe.srcdoc, /data-inline-placeholder="unavailable"/)
  assert.doesNotMatch(iframe.srcdoc, /Loading\.\.\./)
  findButton(target, 'viewer.forDomain').click()
  await flushAsync()
  assert.equal(backend.AddImageAllowlist.mock.calls.length, 0)

  backend.AddImageAllowlist.mockRejectedValue(new Error('synthetic allowlist failure'))
  findButton(target, 'viewer.forSender').click()
  await flushAsync()
  assert.equal(target.textContent.includes('viewer.remoteImagesBlocked'), true)

  sendIframeMessage(iframe, { type: 'contextmenu', text: 'copy failure', url: '', x: 0, y: 0 })
  await flushAsync()
  navigator.clipboard.writeText.mockRejectedValueOnce(new Error('clipboard unavailable'))
  findButton(target.querySelector('[role="menu"]'), 'viewer.copy').click()
  await flushAsync()
  assert.ok(error.mock.calls.length >= 2)
})
