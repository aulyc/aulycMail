// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { createRawSnippet, mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const keyboard = vi.hoisted(() => ({
  focused: vi.fn(() => 'viewer'),
  setFocused: vi.fn(),
  register: vi.fn(() => () => {}),
}))
const settings = vi.hoisted(() => ({ enhanced: vi.fn(() => true) }))

vi.mock('$lib/stores/keyboard.svelte', () => ({
  getFocusedPane: keyboard.focused,
  setFocusedPane: keyboard.setFocused,
  registerPaneNav: keyboard.register,
}))
vi.mock('$lib/stores/settings.svelte', () => ({
  getEnhancedKeyboardNavigation: settings.enhanced,
}))

import ListPane from '../src/lib/components/kit/ListPane.svelte'

const mounted = []

const rowSnippet = createRawSnippet((getItem, getState) => ({
  render: () => `<div data-row-id="${getItem().id}" aria-selected="${getState().selected}">${getItem().id}</div>`,
}))
const emptySnippet = createRawSnippet(() => ({ render: () => '<p data-empty>Nothing synthetic</p>' }))
const loadingSnippet = createRawSnippet(() => ({ render: () => '<p data-loading>Loading synthetic</p>' }))

async function renderList(overrides = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const props = {
    items: [{ id: 'one' }, { id: 'two' }, { id: 'three' }],
    selectedId: 'two',
    row: rowSnippet,
    onSelect: vi.fn(),
    ...overrides,
  }
  const instance = mount(ListPane, { target, props })
  mounted.push(instance)
  await tick()
  await Promise.resolve()
  return {
    target,
    props,
    container: target.querySelector('[role="listbox"]'),
    scroll: target.querySelector('[role="listbox"] > div'),
  }
}

function press(target, key, overrides = {}) {
  const event = new KeyboardEvent('keydown', {
    key,
    bubbles: true,
    cancelable: true,
    ...overrides,
  })
  target.dispatchEvent(event)
  return event
}

beforeEach(() => {
  document.body.innerHTML = ''
  keyboard.focused.mockReset().mockReturnValue('viewer')
  keyboard.setFocused.mockReset()
  keyboard.register.mockReset().mockReturnValue(() => {})
  settings.enhanced.mockReset().mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(() => {})
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('navigates, extends ranges, activates, checks, selects all, and deletes locally', async () => {
  const onSelect = vi.fn()
  const onActivate = vi.fn()
  const onToggleCheck = vi.fn()
  const onSelectAll = vi.fn()
  const onRangeNext = vi.fn()
  const onRangePrev = vi.fn()
  const onDelete = vi.fn()
  const rendered = await renderList({
    onSelect,
    onActivate,
    onToggleCheck,
    onSelectAll,
    onRangeNext,
    onRangePrev,
    onDelete,
    label: 'Synthetic list',
  })
  assert.equal(rendered.container.getAttribute('aria-label'), 'Synthetic list')

  press(rendered.container, 'j')
  press(rendered.container, 'k')
  assert.deepEqual(onSelect.mock.calls.slice(0, 2).map((call) => call[0]), ['three', 'one'])
  press(rendered.container, 'J', { shiftKey: true })
  press(rendered.container, 'K', { shiftKey: true })
  assert.deepEqual(onRangeNext.mock.calls[0], ['two', 'three'])
  assert.deepEqual(onRangePrev.mock.calls[0], ['two', 'one'])

  press(rendered.container, 'Enter')
  press(rendered.container, ' ')
  press(rendered.container, 'a', { ctrlKey: true })
  press(rendered.container, 'Delete')
  press(rendered.container, 'Backspace')
  assert.deepEqual(onActivate.mock.calls[0], ['two'])
  assert.deepEqual(onToggleCheck.mock.calls[0], ['two'])
  assert.equal(onSelectAll.mock.calls.length, 1)
  assert.deepEqual(onDelete.mock.calls.map((call) => call[0]), ['two', 'two'])

  rendered.container.dispatchEvent(new FocusEvent('focus'))
  assert.deepEqual(keyboard.setFocused.mock.calls.at(-1), ['messageList'])
  const focus = vi.spyOn(rendered.container, 'focus')
  rendered.container.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
  assert.equal(focus.mock.calls.length, 1)
})

test('guards boundaries, optional handlers, composition, and disabled enhanced navigation', async () => {
  const onSelect = vi.fn()
  const first = await renderList({ selectedId: 'one', onSelect })
  press(first.container, 'K', { shiftKey: true })
  press(first.container, 'Enter')
  press(first.container, ' ')
  press(first.container, 'a', { ctrlKey: true })
  press(first.container, 'Delete')
  assert.equal(onSelect.mock.calls.length, 0)

  const last = await renderList({ selectedId: 'three', onSelect })
  press(last.container, 'J', { shiftKey: true })
  assert.equal(onSelect.mock.calls.length, 0)

  const none = await renderList({ selectedId: null, onSelect })
  press(none.container, 'j')
  press(none.container, 'k')
  press(none.container, 'Enter')
  press(none.container, ' ')
  assert.deepEqual(onSelect.mock.calls.map((call) => call[0]), ['one', 'three'])

  const empty = await renderList({ items: [], selectedId: null, onSelect })
  press(empty.container, 'j')
  press(empty.container, 'J', { shiftKey: true })
  press(empty.container, 'K', { shiftKey: true })
  assert.equal(onSelect.mock.calls.length, 2)

  settings.enhanced.mockReturnValue(false)
  const disabled = press(last.container, 'j')
  assert.equal(disabled.defaultPrevented, false)
  settings.enhanced.mockReturnValue(true)
  const composing = press(last.container, 'j', { isComposing: true })
  assert.equal(composing.defaultPrevented, false)
  const processKey = press(last.container, 'j', { keyCode: 229 })
  assert.equal(processKey.defaultPrevented, false)

  keyboard.focused.mockReturnValue('messageList')
  last.container.dispatchEvent(new FocusEvent('focus'))
  assert.equal(keyboard.setFocused.mock.calls.length, 0)
  document.activeElement?.blur()
  keyboard.focused.mockReturnValue('viewer')
})

test('renders loading and empty states, scrolls selections, reports reach-end, and registers pane navigation', async () => {
  const customLoading = await renderList({ loading: true, loadingSnippet })
  assert.ok(customLoading.target.querySelector('[data-loading]'))
  const defaultLoading = await renderList({ loading: true })
  assert.match(defaultLoading.target.textContent, /Loading…/)
  const customEmpty = await renderList({ items: [], selectedId: null, empty: emptySnippet })
  assert.ok(customEmpty.target.querySelector('[data-empty]'))
  const defaultEmpty = await renderList({ items: [], selectedId: null })
  assert.match(defaultEmpty.target.textContent, /No items\./)

  const onReachEnd = vi.fn()
  const onActivate = vi.fn()
  const onFocusSearch = vi.fn()
  const onSelect = vi.fn()
  const rendered = await renderList({
    selectedScrollSignal: 4,
    selectedScrollBlock: 'center',
    onReachEnd,
    reachEndThreshold: 20,
    onActivate,
    onFocusSearch,
    onSelect,
    focusSlot: 'viewer',
  })
  assert.equal(HTMLElement.prototype.scrollIntoView.mock.calls.length > 0, true)
  Object.defineProperties(rendered.scroll, {
    scrollHeight: { configurable: true, value: 200 },
    clientHeight: { configurable: true, value: 80 },
    scrollTop: { configurable: true, writable: true, value: 80 },
  })
  rendered.scroll.dispatchEvent(new Event('scroll'))
  assert.equal(onReachEnd.mock.calls.length, 0)
  rendered.scroll.scrollTop = 105
  rendered.scroll.dispatchEvent(new Event('scroll'))
  assert.equal(onReachEnd.mock.calls.length, 1)

  const navigation = keyboard.register.mock.calls.at(-1)
  assert.equal(navigation[0], 'viewer')
  navigation[1].navigateNext()
  navigation[1].navigatePrev()
  navigation[1].activate()
  navigation[1].focusSearch()
  assert.deepEqual(onSelect.mock.calls.slice(0, 2).map((call) => call[0]), ['three', 'one'])
  assert.deepEqual(onActivate.mock.calls[0], ['two'])
  assert.equal(onFocusSearch.mock.calls.length, 1)

  const fallbackSelect = vi.fn()
  await renderList({ selectedId: 'one', onSelect: fallbackSelect })
  keyboard.register.mock.calls.at(-1)[1].activate()
  assert.deepEqual(fallbackSelect.mock.calls.at(-1), ['one'])
  const noSelection = await renderList({ selectedId: null })
  keyboard.register.mock.calls.at(-1)[1].activate()
  noSelection.scroll.dispatchEvent(new Event('scroll'))
})
