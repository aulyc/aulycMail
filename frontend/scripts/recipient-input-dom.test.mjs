// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const fallbackApi = vi.hoisted(() => ({ searchContacts: vi.fn().mockResolvedValue([]) }))

vi.mock('$lib/composerApi', () => ({
  COMPOSER_API_KEY: Symbol.for('recipient-input-test-api'),
  createMainWindowApi: () => fallbackApi,
}))

import '../src/lib/iconify-offline'
import RecipientInputHarness from './fixtures/RecipientInputHarness.svelte'

const mounted = []

function address(name, value) {
  return { name, address: value }
}

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function renderHarness(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(RecipientInputHarness, { target, props })
  mounted.push(instance)
  await flushAsync()
  return { target, instance }
}

function inputFor(target, field = 'primary') {
  return target.querySelector(`[data-field="${field}"] input`)
}

function setInputValue(input, value) {
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

function press(input, key, options = {}) {
  input.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...options }))
}

function dispatchWithDataTransfer(node, type, dataTransfer) {
  const event = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'dataTransfer', { value: dataTransfer })
  node.dispatchEvent(event)
  return event
}

function createDataTransfer() {
  const values = new Map()
  return {
    types: [],
    effectAllowed: 'none',
    dropEffect: 'none',
    setData(type, value) {
      values.set(type, value)
      if (!this.types.includes(type)) this.types.push(type)
    },
    getData(type) {
      return values.get(type) || ''
    },
  }
}

beforeEach(() => {
  document.body.innerHTML = ''
  fallbackApi.searchContacts.mockReset().mockResolvedValue([])
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('inserts, deduplicates, pastes, navigates, and removes recipients at the visible caret', async () => {
  const { target, instance } = await renderHarness({
    primary: [address('Alice', 'alice@example.test'), address('Bob', 'bob@example.test')],
  })
  let input = inputFor(target)

  press(input, 'Home')
  await flushAsync()
  input = inputFor(target)
  setInputValue(input, 'Carol <carol@example.test>')
  press(input, 'Enter')
  await flushAsync()
  assert.deepEqual(instance.getPrimary().map((item) => item.address), [
    'carol@example.test',
    'alice@example.test',
    'bob@example.test',
  ])

  input = inputFor(target)
  setInputValue(input, 'ALICE@example.test')
  press(input, 'Enter')
  await flushAsync()
  assert.equal(instance.getPrimary().length, 3)
  assert.equal(input.value, '')

  const paste = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(paste, 'clipboardData', {
    value: { getData: () => 'Dan <dan@example.test>; eve@example.test, invalid-address' },
  })
  input.dispatchEvent(paste)
  await flushAsync()
  assert.equal(paste.defaultPrevented, true)
  assert.deepEqual(instance.getPrimary().map((item) => item.address), [
    'carol@example.test',
    'dan@example.test',
    'eve@example.test',
    'alice@example.test',
    'bob@example.test',
  ])

  input = inputFor(target)
  input.setSelectionRange(0, 0)
  press(input, 'ArrowLeft')
  await flushAsync()
  press(inputFor(target), 'ArrowRight')
  await flushAsync()
  press(inputFor(target), 'End')
  await flushAsync()
  press(inputFor(target), 'Backspace')
  await flushAsync()
  assert.equal(instance.getPrimary().some((item) => item.address === 'bob@example.test'), false)

  press(inputFor(target), 'Home')
  await flushAsync()
  press(inputFor(target), 'Delete')
  await flushAsync()
  assert.equal(instance.getPrimary()[0].address, 'dan@example.test')

  press(inputFor(target), 'End')
  await flushAsync()
  press(inputFor(target), 'Delete')
  await flushAsync()
  assert.equal(instance.getPrimary().some((item) => item.address === 'alice@example.test'), false)

  const closeButtons = target.querySelectorAll('[data-field="primary"] [role="listitem"] button')
  closeButtons[0].click()
  await flushAsync()
  assert.deepEqual(instance.getPrimary().map((item) => item.address), ['eve@example.test'])
})

test('searches contacts, windows keyboard selection, supports pointer selection, and handles failures', async () => {
  vi.useFakeTimers()
  const contacts = Array.from({ length: 6 }, (_, index) => ({
    display_name: `Contact ${index}`,
    email: `contact${index}@example.test`,
  }))
  const searchContactsFn = vi.fn().mockResolvedValue(contacts)
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const { target, instance } = await renderHarness({ searchContactsFn })
  let input = inputFor(target)

  setInputValue(input, 'contact')
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.deepEqual(searchContactsFn.mock.calls[0], ['contact', 100])
  assert.deepEqual(
    [...target.querySelectorAll('[data-field="primary"] [data-suggestion-index]')].map((node) => node.dataset.suggestionIndex),
    ['0', '1', '2', '3'],
  )

  for (let index = 0; index < 5; index += 1) press(input, 'ArrowDown')
  await flushAsync()
  assert.deepEqual(
    [...target.querySelectorAll('[data-field="primary"] [data-suggestion-index]')].map((node) => node.dataset.suggestionIndex),
    ['1', '2', '3', '4'],
  )
  press(input, 'ArrowUp')
  press(input, 'Enter')
  await flushAsync()
  assert.equal(instance.getPrimary()[0].address, 'contact3@example.test')

  input = inputFor(target)
  setInputValue(input, 'contact')
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  const suggestion = target.querySelector('[data-field="primary"] [data-suggestion-index="2"]')
  suggestion.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 20 }))
  suggestion.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 20 }))
  suggestion.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  await flushAsync()
  assert.equal(instance.getPrimary()[1].address, 'contact2@example.test')

  searchContactsFn.mockRejectedValueOnce(new Error('contact search unavailable'))
  input = inputFor(target)
  setInputValue(input, 'failure')
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(error.mock.calls.length, 1)
  assert.equal(target.querySelector('[data-field="primary"] [data-suggestion-index]'), null)

  setInputValue(input, '')
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  press(input, 'Escape')
})

test('honors composition, delimiter keys, focus reopening, and delayed blur commit rules', async () => {
  vi.useFakeTimers()
  const searchContactsFn = vi.fn().mockResolvedValue([{ display_name: 'Kana', email: 'kana@example.test' }])
  const { target, instance } = await renderHarness({ searchContactsFn })
  let input = inputFor(target)

  input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
  setInputValue(input, 'かな')
  await vi.advanceTimersByTimeAsync(250)
  assert.equal(searchContactsFn.mock.calls.length, 0)
  press(input, 'Enter', { isComposing: true })
  assert.equal(instance.getPrimary().length, 0)
  input.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }))
  await flushAsync()
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(searchContactsFn.mock.calls.length, 1)
  assert.ok(target.querySelector('[data-field="primary"] [data-suggestion-index]'))

  press(input, 'Escape')
  input.dispatchEvent(new FocusEvent('focus', { bubbles: true }))
  await flushAsync()
  assert.ok(target.querySelector('[data-field="primary"] [data-suggestion-index]'))

  setInputValue(input, 'comma@example.test')
  press(input, ',')
  await flushAsync()
  setInputValue(inputFor(target), 'semi@example.test')
  press(inputFor(target), ';')
  await flushAsync()
  setInputValue(inputFor(target), 'tab@example.test')
  press(inputFor(target), 'Tab')
  await flushAsync()
  assert.deepEqual(instance.getPrimary().slice(-3).map((item) => item.address), [
    'comma@example.test',
    'semi@example.test',
    'tab@example.test',
  ])

  input = inputFor(target)
  setInputValue(input, 'blur@example.test')
  input.dispatchEvent(new FocusEvent('blur', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(instance.getPrimary().at(-1).address, 'blur@example.test')

  input = inputFor(target)
  setInputValue(input, '   ')
  input.dispatchEvent(new FocusEvent('blur', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(input.value, '')

  setInputValue(input, 'not-an-address')
  input.dispatchEvent(new FocusEvent('blur', { bubbles: true }))
  await vi.advanceTimersByTimeAsync(210)
  await flushAsync()
  assert.equal(input.value, 'not-an-address')
})

test('reorders within a field and moves recipients across fields with drag payload validation', async () => {
  const { target, instance } = await renderHarness({
    primary: [
      address('Alice', 'alice@example.test'),
      address('Bob', 'bob@example.test'),
      address('Carol', 'carol@example.test'),
    ],
    secondary: [address('Dan', 'dan@example.test')],
  })

  let primaryItems = target.querySelectorAll('[data-field="primary"] [role="listitem"]')
  const primaryInput = inputFor(target)
  const reorderTransfer = createDataTransfer()
  dispatchWithDataTransfer(primaryItems[0], 'dragstart', reorderTransfer)
  dispatchWithDataTransfer(primaryInput, 'dragenter', reorderTransfer)
  dispatchWithDataTransfer(primaryInput, 'dragover', reorderTransfer)
  dispatchWithDataTransfer(primaryInput, 'drop', reorderTransfer)
  dispatchWithDataTransfer(primaryItems[0], 'dragend', reorderTransfer)
  await flushAsync()
  assert.deepEqual(instance.getPrimary().map((item) => item.address), [
    'bob@example.test',
    'carol@example.test',
    'alice@example.test',
  ])

  primaryItems = target.querySelectorAll('[data-field="primary"] [role="listitem"]')
  const secondaryInput = inputFor(target, 'secondary')
  const moveTransfer = createDataTransfer()
  dispatchWithDataTransfer(primaryItems[0], 'dragstart', moveTransfer)
  dispatchWithDataTransfer(secondaryInput, 'dragover', moveTransfer)
  dispatchWithDataTransfer(secondaryInput, 'drop', moveTransfer)
  dispatchWithDataTransfer(primaryItems[0], 'dragend', moveTransfer)
  await flushAsync()
  assert.deepEqual(instance.getPrimary().map((item) => item.address), [
    'carol@example.test',
    'alice@example.test',
  ])
  assert.deepEqual(instance.getSecondary().map((item) => item.address), [
    'dan@example.test',
    'bob@example.test',
  ])

  const malformed = createDataTransfer()
  malformed.setData('application/x-aulycmail-recipient', '{bad json')
  dispatchWithDataTransfer(secondaryInput, 'drop', malformed)
  const missingAddress = createDataTransfer()
  missingAddress.setData('application/x-aulycmail-recipient', JSON.stringify({ sourceId: -1, recipient: { name: 'Nobody' } }))
  dispatchWithDataTransfer(secondaryInput, 'drop', missingAddress)
  dispatchWithDataTransfer(secondaryInput, 'dragleave', createDataTransfer())
  dispatchWithDataTransfer(secondaryInput, 'drop', createDataTransfer())
  await flushAsync()
  assert.equal(instance.getSecondary().length, 2)
})
