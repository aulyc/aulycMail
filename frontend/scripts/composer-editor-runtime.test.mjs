// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { afterEach, beforeEach, test, vi } from 'vitest'

vi.mock('svelte-i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))

import { createComposerEditor } from '../src/lib/components/composer/composerEditor.ts'

let editors = []

function createEditor(handlers = {}) {
  const element = document.createElement('div')
  document.body.appendChild(element)
  const editor = createComposerEditor(element, { onUpdate: () => {}, ...handlers })
  editors.push(editor)
  return editor
}

function pasteEvent({ items, files }) {
  return {
    clipboardData: { items, files },
    preventDefault: vi.fn(),
  }
}

function dropEvent({ files = [], uriList = '', text = '' } = {}) {
  return {
    dataTransfer: {
      files,
      getData: vi.fn((type) => type === 'text/uri-list' ? uriList : type === 'text/plain' ? text : ''),
    },
    preventDefault: vi.fn(),
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  editors = []
})

afterEach(() => {
  for (const editor of editors) editor.destroy()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('creates a real TipTap editor and preserves legacy mail markup', () => {
  const onUpdate = vi.fn()
  const editor = createEditor({ onUpdate, getDarkFilterMode: () => 'reply' })

  editor.commands.setContent(`
    <p><font color="#ff0000">Legacy red</font></p>
    <p><span data-contact-mention data-label="Ada">@Ada</span></p>
    <table style="width: 80%"><tbody><tr><th style="color: blue">Head</th><td style="padding: 4px">Cell</td></tr></tbody></table>
    <p><img src="data:image/png;base64,AA==" data-original-src="https://images.example.test/a.png"></p>
    <blockquote><p>Quoted body</p></blockquote>
  `)

  const html = editor.getHTML()
  assert.match(html, /color: (?:#ff0000|rgb\(255, 0, 0\))/)
  assert.match(html, /data-contact-mention=""/)
  assert.match(html, /data-label="Ada"/)
  assert.match(html, /style="width: 80%/)
  assert.match(html, /data-original-src="https:\/\/images\.example\.test\/a\.png"/)
  assert.match(editor.getText(), /@Ada/)
  assert.ok(editor.view.dom.querySelector('blockquote.composer-dark-filter-content'))
  editor.commands.insertContent('!')
  assert.ok(onUpdate.mock.calls.length > 0)
})

test('routes keyboard, paste, and file-drop behavior through configured handlers', () => {
  const handlers = {
    onMentionKeyDown: vi.fn(() => false),
    onPasteImage: vi.fn(),
    onDropImage: vi.fn(),
    onDropFile: vi.fn(),
    onDropFilePaths: vi.fn(),
    onShiftTab: vi.fn(),
    isEnhancedKeyboardNavigationEnabled: vi.fn(() => true),
  }
  const editor = createEditor(handlers)
  const editorProps = editor.options.editorProps
  const image = new File(['image'], 'inline.png', { type: 'image/png' })
  const documentFile = new File(['pdf'], 'report.pdf', { type: 'application/pdf' })

  const itemPaste = pasteEvent({
    items: [{ type: 'image/png', getAsFile: () => image }],
    files: [],
  })
  assert.equal(editorProps.handlePaste(editor.view, itemPaste), true)
  assert.deepEqual(handlers.onPasteImage.mock.calls.at(-1), [image])
  assert.equal(itemPaste.preventDefault.mock.calls.length, 1)

  const fallbackPaste = pasteEvent({ items: [], files: [image] })
  assert.equal(editorProps.handlePaste(editor.view, fallbackPaste), true)
  assert.equal(handlers.onPasteImage.mock.calls.length, 2)

  const textPaste = pasteEvent({ items: [], files: [documentFile] })
  assert.equal(editorProps.handlePaste(editor.view, textPaste), false)

  const fileDrop = dropEvent({ files: [image, documentFile] })
  assert.equal(editorProps.handleDrop(editor.view, fileDrop, null, false), true)
  assert.deepEqual(handlers.onDropImage.mock.calls.at(-1), [image])
  assert.deepEqual(handlers.onDropFile.mock.calls.at(-1), [documentFile])
  assert.equal(fileDrop.preventDefault.mock.calls.length, 1)

  const uriDrop = dropEvent({ uriList: 'file:///tmp/report%20one.pdf\nfile:///tmp/image.png' })
  assert.equal(editorProps.handleDrop(editor.view, uriDrop, null, false), true)
  assert.deepEqual(handlers.onDropFilePaths.mock.calls.at(-1), [
    ['/tmp/report one.pdf', '/tmp/image.png'],
  ])
  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, true), false)

  const shiftTab = new KeyboardEvent('keydown', { key: 'Tab', code: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
  editor.view.dom.dispatchEvent(shiftTab)
  assert.equal(handlers.onShiftTab.mock.calls.length, 1)
  assert.equal(shiftTab.defaultPrevented, true)
})

test('removes WebKit-inserted file URIs without deleting adjacent user text', async () => {
  vi.useFakeTimers()
  const onDropFilePaths = vi.fn()
  const editor = createEditor({ onDropFilePaths })
  const editorProps = editor.options.editorProps
  editor.commands.setContent('<p>HelloWorld</p>')

  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  editor.commands.setContent('<p>Hellofile:///tmp/report%20one.pdfWorld</p>')
  await vi.advanceTimersByTimeAsync(200)

  assert.equal(editor.getText(), 'HelloWorld')
  assert.deepEqual(onDropFilePaths.mock.calls.at(-1), [['/tmp/report one.pdf']])

  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  editor.commands.setContent('<p>HelloWorldfile:///tmp/final.txt</p>')
  await vi.advanceTimersByTimeAsync(200)
  assert.equal(editor.getText(), 'HelloWorld')
  assert.deepEqual(onDropFilePaths.mock.calls.at(-1), [['/tmp/final.txt']])
})
