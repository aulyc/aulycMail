// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))

import SignatureEditor from '../src/lib/components/settings/account/SignatureEditor.svelte'
import SignatureEditorHarness from './fixtures/SignatureEditorHarness.svelte'

const mounted = []

async function flushAsync() {
  for (let index = 0; index < 9; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

async function renderEditor(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(SignatureEditor, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance, proseMirror: target.querySelector('.ProseMirror') }
}

async function renderHarness(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(SignatureEditorHarness, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance, proseMirror: target.querySelector('.ProseMirror') }
}

function titledButton(target, title) {
  const button = target.querySelector(`button[title="${title}"]`)
  assert.ok(button, `missing toolbar button ${title}`)
  return button
}

function inputValue(input, value) {
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
}

beforeEach(() => {
  document.body.innerHTML = ''
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('preserves legacy signature HTML and round-trips formatted raw HTML', async () => {
  const onchange = vi.fn()
  const value = '<div style="color: red"><font color="#00ff00">Legacy</font></div><table style="width: 80%"><tbody><tr><th style="color: blue">Head</th><td style="padding: 4px">Cell</td></tr></tbody></table>'
  const { target, proseMirror } = await renderHarness({ initialValue: value, onchange })
  assert.ok(proseMirror)
  assert.match(proseMirror.innerHTML, /Legacy/)
  assert.match(proseMirror.innerHTML, /color: (?:#00ff00|rgb\(0, 255, 0\))/)
  assert.match(proseMirror.innerHTML, /width: 80%/)
  assert.match(proseMirror.innerHTML, /padding: 4px/)

  titledButton(target, 'editor.htmlMode').click()
  await flushAsync()
  const raw = target.querySelector('textarea')
  assert.ok(raw)
  assert.match(raw.value, /<table style="width: 80%">/)
  assert.match(raw.value, /\n/)
  inputValue(raw, '<p>Changed</p>\n<div>Second</div>')
  await flushAsync()
  assert.deepEqual(onchange.mock.calls.at(-1), ['<p>Changed</p>\n<div>Second</div>'])

  titledButton(target, 'editor.visualMode').click()
  await flushAsync()
  assert.match(target.querySelector('.ProseMirror').textContent, /ChangedSecond/)
  assert.match(onchange.mock.calls.at(-1)[0], /Changed/)
})

test('executes formatting, color, size, alignment, table, link, and image URL actions', async () => {
  const prompt = vi.fn()
    .mockReturnValueOnce('https://example.test/link')
    .mockReturnValueOnce('https://example.test/image.png')
    .mockReturnValue(null)
  Object.defineProperty(globalThis, 'prompt', { configurable: true, value: prompt })
  const { target, proseMirror } = await renderEditor({ value: '<p>Signature</p>' })
  proseMirror.focus()

  for (const title of ['editor.bold', 'editor.italic', 'editor.underline', 'editor.strikethrough']) {
    titledButton(target, title).click()
  }
  titledButton(target, 'editor.alignCenter').click()
  titledButton(target, 'editor.alignRight').click()
  titledButton(target, 'editor.alignLeft').click()

  titledButton(target, 'editor.textColor').click()
  await flushAsync()
  titledButton(target, '#dc2626').click()
  titledButton(target, 'editor.textColor').click()
  await flushAsync()
  const customColor = target.querySelector('input[type="color"]')
  customColor.value = '#123456'
  customColor.dispatchEvent(new Event('change', { bubbles: true }))
  titledButton(target, 'editor.textColor').click()
  await flushAsync()
  const reset = [...target.querySelectorAll('button')].find((button) => button.textContent.includes('editor.reset'))
  reset.click()

  titledButton(target, 'editor.fontSize').click()
  await flushAsync()
  const sizeButton = [...target.querySelectorAll('button')].find((button) => button.textContent.trim() === '20px')
  sizeButton.click()

  titledButton(target, 'editor.insertTable').click()
  await flushAsync()
  assert.ok(target.querySelector('.ProseMirror table'))
  const deleteTable = target.querySelector('button[title="editor.deleteTable"]')
  if (deleteTable) {
    deleteTable.click()
    await flushAsync()
  }

  titledButton(target, 'editor.insertLink').click()
  await flushAsync()
  const removeLink = target.querySelector('button[title="editor.removeLink"]')
  removeLink?.click()
  titledButton(target, 'editor.insertImageUrl').click()
  await flushAsync()
  assert.ok(target.querySelector('.ProseMirror img[src="https://example.test/image.png"]'))
  titledButton(target, 'editor.insertLink').click()
  assert.equal(prompt.mock.calls.length, 3)

  titledButton(target, 'editor.textColor').click()
  titledButton(target, 'editor.fontSize').click()
  await flushAsync()
  target.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await flushAsync()
  assert.equal(target.querySelector('input[type="color"]'), null)
})

test('inserts image files from picker, paste, and drop and handles file-read failures', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const image = new File(['synthetic-image'], 'signature.png', { type: 'image/png' })
  const text = new File(['synthetic-text'], 'note.txt', { type: 'text/plain' })
  const readFile = vi.spyOn(FileReader.prototype, 'readAsDataURL').mockImplementation(function () {
    Object.defineProperty(this, 'result', { configurable: true, value: 'data:image/png;base64,AAAA' })
    this.onload?.(new ProgressEvent('load'))
  })
  const nativeClick = HTMLInputElement.prototype.click
  vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(function () {
    if (this.type !== 'file') return nativeClick.call(this)
    Object.defineProperty(this, 'files', { configurable: true, value: [image] })
    this.dispatchEvent(new Event('change', { bubbles: true }))
  })
  const { target, proseMirror } = await renderEditor({ value: '<p>Signature</p>' })

  titledButton(target, 'editor.insertImageFile').click()
  await flushAsync()
  assert.ok(target.querySelector('.ProseMirror img[src^="data:image/png"]'))

  const paste = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(paste, 'clipboardData', {
    value: {
      items: [{ type: 'image/png', getAsFile: () => image }],
      files: [image],
      getData: () => '',
    },
  })
  proseMirror.dispatchEvent(paste)
  await flushAsync()
  assert.equal(paste.defaultPrevented, true)

  const textPaste = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(textPaste, 'clipboardData', {
    value: {
      items: [{ type: 'text/plain', getAsFile: () => text }],
      files: [text],
      getData: () => '',
    },
  })
  proseMirror.dispatchEvent(textPaste)
  assert.equal(textPaste.defaultPrevented, false)

  const drop = new Event('drop', { bubbles: true, cancelable: true })
  Object.defineProperty(drop, 'dataTransfer', { value: { files: [image], getData: () => '', types: ['Files'] } })
  proseMirror.dispatchEvent(drop)
  await flushAsync()

  readFile.mockImplementation(function () {
    this.onerror?.(new ProgressEvent('error'))
  })
  titledButton(target, 'editor.insertImageFile').click()
  await flushAsync()
  assert.equal(error.mock.calls.length, 1)
})

test('maps Enter to a hard break and safely handles empty editor actions', async () => {
  Object.defineProperty(globalThis, 'prompt', { configurable: true, value: vi.fn(() => null) })
  const { target, proseMirror } = await renderEditor({ value: '' })
  proseMirror.focus()
  proseMirror.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', bubbles: true, cancelable: true }))
  await flushAsync()
  assert.ok(target.querySelector('.ProseMirror br'))
  titledButton(target, 'editor.insertImageUrl').click()
  titledButton(target, 'editor.insertLink').click()
})
