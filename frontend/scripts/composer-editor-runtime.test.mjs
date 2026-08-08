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

test('routes keyboard, paste, and file-drop behavior through configured handlers', async () => {
  const handlers = {
    onMentionKeyDown: vi.fn(() => false),
    onFiles: vi.fn(),
    onFilePaths: vi.fn(),
    readClipboardFilePaths: vi.fn().mockResolvedValue([]),
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
  assert.deepEqual(handlers.onFiles.mock.calls.at(-1), [[image]])
  assert.equal(itemPaste.preventDefault.mock.calls.length, 1)

  const fallbackPaste = pasteEvent({ items: [], files: [image] })
  assert.equal(editorProps.handlePaste(editor.view, fallbackPaste), true)
  assert.equal(handlers.onFiles.mock.calls.length, 2)

  const documentPaste = pasteEvent({ items: [], files: [documentFile] })
  assert.equal(editorProps.handlePaste(editor.view, documentPaste), true)
  assert.deepEqual(handlers.onFiles.mock.calls.at(-1), [[documentFile]])
  assert.equal(documentPaste.preventDefault.mock.calls.length, 1)

  const uriPaste = {
    clipboardData: {
      items: [],
      files: [],
      types: ['text/uri-list'],
      getData: vi.fn((type) => type === 'text/uri-list' ? 'file:///tmp/copied%20report.pdf' : ''),
    },
    preventDefault: vi.fn(),
  }
  assert.equal(editorProps.handlePaste(editor.view, uriPaste), true)
  assert.deepEqual(handlers.onFilePaths.mock.calls.at(-1), [['/tmp/copied report.pdf']])
  assert.equal(uriPaste.preventDefault.mock.calls.length, 1)

  handlers.readClipboardFilePaths.mockResolvedValueOnce(['/tmp/copied-one.pdf', '/tmp/copied-two.txt'])
  const nativeFilePaste = pasteEvent({ items: [], files: [] })
  assert.equal(editorProps.handlePaste(editor.view, nativeFilePaste), false)
  await vi.waitFor(() => {
    assert.deepEqual(handlers.onFilePaths.mock.calls.at(-1), [
      ['/tmp/copied-one.pdf', '/tmp/copied-two.txt'],
    ])
  })

  const fileDrop = dropEvent({ files: [image, documentFile] })
  assert.equal(editorProps.handleDrop(editor.view, fileDrop, null, false), true)
  assert.deepEqual(handlers.onFiles.mock.calls.at(-1), [[image, documentFile]])
  assert.equal(fileDrop.preventDefault.mock.calls.length, 1)

  const uriDrop = dropEvent({ uriList: 'file:///tmp/report%20one.pdf\nfile:///tmp/image.png' })
  assert.equal(editorProps.handleDrop(editor.view, uriDrop, null, false), true)
  assert.deepEqual(handlers.onFilePaths.mock.calls.at(-1), [
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
  const onFilePaths = vi.fn()
  const editor = createEditor({ onFilePaths })
  const editorProps = editor.options.editorProps
  editor.commands.setContent('<p>HelloWorld</p>')

  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  editor.commands.setContent('<p>Hellofile:///tmp/report%20one.pdfWorld</p>')
  await vi.advanceTimersByTimeAsync(200)

  assert.equal(editor.getText(), 'HelloWorld')
  assert.deepEqual(onFilePaths.mock.calls.at(-1), [['/tmp/report one.pdf']])

  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  editor.commands.setContent('<p>HelloWorldfile:///tmp/final.txt</p>')
  await vi.advanceTimersByTimeAsync(200)
  assert.equal(editor.getText(), 'HelloWorld')
  assert.deepEqual(onFilePaths.mock.calls.at(-1), [['/tmp/final.txt']])
})

test('covers optional editor callbacks, mention navigation, and paste fallbacks', () => {
  const mentionHandler = vi.fn(() => true)
  const editor = createEditor({ onMentionKeyDown: mentionHandler })
  const editorProps = editor.options.editorProps

  const handledMention = new KeyboardEvent('keydown', { key: '@', bubbles: true, cancelable: true })
  assert.equal(editorProps.handleKeyDown(editor.view, handledMention), true)
  assert.deepEqual(mentionHandler.mock.calls.at(-1), [handledMention])

  mentionHandler.mockReturnValue(false)
  editor.commands.setContent('<p><span data-contact-mention data-label="Ada">@Ada</span>tail</p>')
  editor.commands.setTextSelection(1)
  const right = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })
  assert.equal(editorProps.handleKeyDown(editor.view, right), true)
  assert.equal(right.defaultPrevented, true)
  assert.equal(editor.state.selection.from, 2)

  const left = new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true, cancelable: true })
  assert.equal(editorProps.handleKeyDown(editor.view, left), true)
  assert.equal(left.defaultPrevented, true)
  assert.equal(editor.state.selection.from, 1)

  editor.commands.setTextSelection({ from: 1, to: 2 })
  assert.equal(editorProps.handleKeyDown(editor.view, new KeyboardEvent('keydown', { key: 'ArrowRight' })), false)
  assert.equal(editorProps.handleKeyDown(editor.view, new KeyboardEvent('keydown', { key: 'Home' })), false)

  const image = new File(['image'], 'inline.png', { type: 'image/png' })
  const nullImageItem = pasteEvent({ items: [{ type: 'image/png', getAsFile: () => null }], files: [] })
  assert.equal(editorProps.handlePaste(editor.view, nullImageItem), true)
  assert.equal(nullImageItem.preventDefault.mock.calls.length, 1)
  assert.equal(editorProps.handlePaste(editor.view, { clipboardData: null, preventDefault: vi.fn() }), false)
  assert.equal(editorProps.handlePaste(editor.view, pasteEvent({ items: [], files: [image] })), true)

  const plainEditor = createEditor()
  const shiftTab = new KeyboardEvent('keydown', { key: 'Tab', code: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
  plainEditor.view.dom.dispatchEvent(shiftTab)
  assert.equal(shiftTab.defaultPrevented, true)
  const modEnter = new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', ctrlKey: true, bubbles: true, cancelable: true })
  plainEditor.view.dom.dispatchEvent(modEnter)
  assert.equal(modEnter.defaultPrevented, true)

  const disabledEditor = createEditor({
    onShiftTab: vi.fn(),
    isEnhancedKeyboardNavigationEnabled: () => false,
  })
  const disabledShiftTab = new KeyboardEvent('keydown', { key: 'Tab', code: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
  disabledEditor.view.dom.dispatchEvent(disabledShiftTab)
  assert.equal(disabledShiftTab.defaultPrevented, false)
})

test('covers drop text fallback, absent handlers, and unchanged WebKit state', async () => {
  vi.useFakeTimers()
  const onFilePaths = vi.fn()
  const editor = createEditor({ onFilePaths })
  const editorProps = editor.options.editorProps

  const textDrop = dropEvent({ text: 'file:///tmp/from%20plain.txt' })
  assert.equal(editorProps.handleDrop(editor.view, textDrop, null, false), true)
  assert.deepEqual(onFilePaths.mock.calls.at(-1), [['/tmp/from plain.txt']])

  const invalidTextDrop = dropEvent({ text: 'https://example.test/not-a-local-file' })
  assert.equal(editorProps.handleDrop(editor.view, invalidTextDrop, null, false), false)
  await vi.advanceTimersByTimeAsync(200)

  const image = new File(['image'], 'inline.png', { type: 'image/png' })
  const documentFile = new File(['pdf'], 'report.pdf', { type: 'application/pdf' })
  const noHandlersEditor = createEditor()
  const filesDrop = dropEvent({ files: [image, documentFile] })
  assert.equal(noHandlersEditor.options.editorProps.handleDrop(noHandlersEditor.view, filesDrop, null, false), true)
  assert.equal(filesDrop.preventDefault.mock.calls.length, 1)
  const uriDrop = dropEvent({ uriList: 'file:///tmp/no-handler.txt' })
  assert.equal(noHandlersEditor.options.editorProps.handleDrop(noHandlersEditor.view, uriDrop, null, false), true)

  editor.commands.setContent('<p>unchanged</p>')
  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  await vi.advanceTimersByTimeAsync(200)
  assert.equal(editor.getText(), 'unchanged')

  assert.equal(editorProps.handleDrop(editor.view, dropEvent(), null, false), false)
  editor.commands.setContent('<p>unchanged plus ordinary text</p>')
  await vi.advanceTimersByTimeAsync(200)
  assert.equal(editor.getText(), 'unchanged plus ordinary text')
  assert.equal(onFilePaths.mock.calls.length, 1)
})
