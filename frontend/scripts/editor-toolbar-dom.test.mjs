// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const keyboard = vi.hoisted(() => ({ enhanced: true }))

vi.mock('$lib/stores/settings.svelte', () => ({
  getEnhancedKeyboardNavigation: () => keyboard.enhanced,
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({
  default: (await import('./fixtures/StaticStub.svelte')).default,
}))

import EditorToolbar from '../src/lib/components/composer/EditorToolbar.svelte'

const mounted = []

function createEditor() {
  const calls = []
  const listeners = new Map()
  const active = new Set(['bold', 'underline', 'bulletList', 'link'])
  const attributes = { color: '#2563eb', fontSize: '18px' }
  let alignment = 'center'

  const chain = {}
  for (const name of [
    'focus', 'toggleBold', 'toggleItalic', 'toggleUnderline', 'toggleStrike',
    'toggleBulletList', 'toggleOrderedList', 'toggleBlockquote', 'unsetColor', 'run',
  ]) {
    chain[name] = (...args) => {
      calls.push([name, ...args])
      return chain
    }
  }
  for (const name of ['setLink', 'setColor', 'setFontSize', 'setTextAlign']) {
    chain[name] = (...args) => {
      calls.push([name, ...args])
      return chain
    }
  }

  return {
    calls,
    active,
    attributes,
    set alignment(value) { alignment = value },
    chain: () => chain,
    isActive(value) {
      if (typeof value === 'string') return active.has(value)
      return value?.textAlign === alignment
    },
    getAttributes: () => attributes,
    on(event, handler) { listeners.set(event, handler) },
    off(event, handler) {
      if (listeners.get(event) === handler) listeners.delete(event)
    },
    emitTransaction() { listeners.get('transaction')?.() },
    hasListener(event) { return listeners.has(event) },
  }
}

async function flushAsync() {
  for (let index = 0; index < 4; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function render(props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(EditorToolbar, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function button(target, title) {
  const result = target.querySelector(`button[title="${title}"]`)
  assert.ok(result, `missing button ${title}`)
  return result
}

function key(value) {
  window.dispatchEvent(new KeyboardEvent('keydown', {
    key: value,
    bubbles: true,
    cancelable: true,
  }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  keyboard.enhanced = true
  Element.prototype.scrollIntoView = vi.fn()
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

test('renders editor state and runs every formatting, color, size, alignment, and display action', async () => {
  const editor = createEditor()
  const onTogglePlainText = vi.fn()
  const onInsertImage = vi.fn()
  const onToggleDarkFilter = vi.fn()
  const { target } = await render({
    editor,
    onTogglePlainText,
    onInsertImage,
    showDarkFilter: true,
    darkFilterEnabled: true,
    onToggleDarkFilter,
  })

  assert.equal(editor.hasListener('transaction'), true)
  assert.equal(button(target, 'editor.bold').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.underline').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.bulletList').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.insertLink').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.fontSize').textContent.trim(), '18px')
  assert.equal(button(target, 'editor.alignCenter').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.disableDarkFilter').getAttribute('aria-pressed'), 'true')

  for (const [title, method] of [
    ['editor.bold', 'toggleBold'],
    ['editor.italic', 'toggleItalic'],
    ['editor.underline', 'toggleUnderline'],
    ['editor.strikethrough', 'toggleStrike'],
    ['editor.bulletList', 'toggleBulletList'],
    ['editor.numberedList', 'toggleOrderedList'],
    ['editor.quote', 'toggleBlockquote'],
  ]) {
    button(target, title).click()
    assert.equal(editor.calls.some(([name]) => name === method), true, `${method} was not run`)
  }

  vi.stubGlobal('prompt', vi.fn().mockReturnValue('https://example.test/message'))
  button(target, 'editor.insertLink').click()
  assert.deepEqual(editor.calls.find(([name]) => name === 'setLink'), [
    'setLink', { href: 'https://example.test/message' },
  ])
  prompt.mockReturnValueOnce('')
  const linkCallCount = editor.calls.filter(([name]) => name === 'setLink').length
  button(target, 'editor.insertLink').click()
  assert.equal(editor.calls.filter(([name]) => name === 'setLink').length, linkCallCount)

  button(target, 'editor.textColor').click()
  await flushAsync()
  assert.ok(button(target, '#dc2626'))
  button(target, '#dc2626').click()
  await flushAsync()
  assert.deepEqual(editor.calls.find(([name]) => name === 'setColor'), ['setColor', '#dc2626'])
  assert.equal(target.querySelector('button[title="#dc2626"]'), null)

  button(target, 'editor.textColor').click()
  await flushAsync()
  const colorInput = target.querySelector('input[type="color"]')
  colorInput.value = '#16a34a'
  colorInput.dispatchEvent(new Event('change', { bubbles: true }))
  assert.deepEqual(editor.calls.filter(([name]) => name === 'setColor').at(-1), ['setColor', '#16a34a'])

  button(target, 'editor.textColor').click()
  await flushAsync()
  ;[...target.querySelectorAll('button')].find((item) => item.textContent.includes('editor.reset')).click()
  assert.equal(editor.calls.some(([name]) => name === 'unsetColor'), true)

  button(target, 'editor.fontSize').click()
  await flushAsync()
  const sizeButton = [...target.querySelectorAll('button')].find((item) => item.textContent.trim() === '24px')
  assert.ok(sizeButton)
  sizeButton.click()
  assert.deepEqual(editor.calls.find(([name]) => name === 'setFontSize'), ['setFontSize', '24px'])

  button(target, 'editor.alignLeft').click()
  button(target, 'editor.alignCenter').click()
  button(target, 'editor.alignRight').click()
  assert.deepEqual(editor.calls.filter(([name]) => name === 'setTextAlign'), [
    ['setTextAlign', 'left'], ['setTextAlign', 'center'], ['setTextAlign', 'right'],
  ])

  button(target, 'editor.insertImage').click()
  button(target, 'editor.switchToPlainText').click()
  button(target, 'editor.disableDarkFilter').click()
  assert.equal(onInsertImage.mock.calls.length, 1)
  assert.equal(onTogglePlainText.mock.calls.length, 1)
  assert.equal(onToggleDarkFilter.mock.calls.length, 1)

  editor.active.clear()
  editor.active.add('italic')
  editor.attributes.color = ''
  editor.attributes.fontSize = ''
  editor.alignment = 'right'
  editor.emitTransaction()
  await flushAsync()
  assert.equal(button(target, 'editor.italic').classList.contains('bg-muted'), true)
  assert.equal(button(target, 'editor.bold').classList.contains('bg-muted'), false)
  assert.equal(button(target, 'editor.fontSize').textContent.trim(), '14px')
  assert.equal(button(target, 'editor.alignRight').classList.contains('bg-muted'), true)
})

test('supports numbered hints, cyclic arrow selection, activation, escape, and outside dismissal', async () => {
  const editor = createEditor()
  const onReturnFocus = vi.fn()
  const { target, instance } = await render({ editor, onReturnFocus })
  const toolbar = target.querySelector('[role="toolbar"]')

  instance.focus()
  await flushAsync()
  assert.equal(document.activeElement, toolbar)
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected="true"]').title, 'editor.bold')
  assert.match(target.textContent, /1/)

  key('ArrowLeft')
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected="true"]').title, 'editor.switchToPlainText')
  key('ArrowRight')
  key('ArrowRight')
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected="true"]').title, 'editor.italic')
  key('Enter')
  assert.equal(editor.calls.some(([name]) => name === 'toggleItalic'), true)
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected]'), null)
  assert.equal(onReturnFocus.mock.calls.length, 1)

  instance.focus()
  key('c')
  assert.equal(editor.calls.some(([name]) => name === 'toggleBlockquote'), true)
  assert.equal(onReturnFocus.mock.calls.length, 2)

  instance.focus()
  key('Escape')
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected]'), null)
  assert.equal(onReturnFocus.mock.calls.length, 3)

  instance.focus()
  document.body.click()
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected]'), null)

  instance.focus()
  instance.focus()
  assert.equal(onReturnFocus.mock.calls.length, 4)
})

test('plain-text and disabled keyboard modes expose only safe controls', async () => {
  const onTogglePlainText = vi.fn()
  const onToggleDarkFilter = vi.fn()
  const onReturnFocus = vi.fn()
  const { target, instance } = await render({
    editor: null,
    isPlainTextMode: true,
    showDarkFilter: true,
    darkFilterEnabled: false,
    onTogglePlainText,
    onToggleDarkFilter,
    onReturnFocus,
  })

  assert.equal(button(target, 'editor.bold').disabled, true)
  assert.equal(button(target, 'editor.insertImage').disabled, true)
  assert.equal(button(target, 'editor.switchToRichText').disabled, false)
  assert.equal(button(target, 'editor.enableDarkFilter').getAttribute('aria-pressed'), 'false')

  instance.focus()
  await flushAsync()
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected="true"]').title, 'editor.switchToRichText')
  key('Enter')
  assert.equal(onTogglePlainText.mock.calls.length, 1)
  assert.equal(onReturnFocus.mock.calls.length, 1)

  instance.focus()
  key('ArrowDown')
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected="true"]').title, 'editor.enableDarkFilter')
  key(' ')
  assert.equal(onToggleDarkFilter.mock.calls.length, 1)

  keyboard.enhanced = false
  instance.focus()
  key('ArrowRight')
  assert.equal(target.querySelector('[data-keyboard-toolbar-selected]'), null)
  assert.equal(onReturnFocus.mock.calls.length, 2)
})
