// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const guards = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))

vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: guards.open,
  dialogGuardClose: guards.close,
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

import KeyboardActionMenu from '../src/lib/components/keyboard/KeyboardActionMenu.svelte'
import { keyboardActionMenu } from '../src/lib/stores/keyboardActionMenu.svelte.ts'

const mounted = []
const actionElements = []

async function flushAsync() {
  for (let index = 0; index < 7; index += 1) {
    await Promise.resolve()
    await tick()
  }
}

function makeActions() {
  return ['First action', 'Second action', 'Third action'].map((label, index) => {
    const element = document.createElement('button')
    element.textContent = `source ${index + 1}`
    element.addEventListener('click', vi.fn())
    document.body.appendChild(element)
    actionElements.push(element)
    return { id: `action-${index + 1}`, label, element }
  })
}

async function renderMenu(actions = makeActions()) {
  keyboardActionMenu.actions = actions
  keyboardActionMenu.open = true
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(KeyboardActionMenu, { target })
  mounted.push(instance)
  await flushAsync()
  return { target, actions }
}

function inputText(input, value) {
  input.value = value
  input.dispatchEvent(new InputEvent('input', { bubbles: true }))
}

beforeEach(() => {
  document.body.innerHTML = ''
  actionElements.length = 0
  keyboardActionMenu.close()
  guards.open.mockReset()
  guards.close.mockReset()
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
    callback(performance.now())
    return 1
  })
  if (!HTMLElement.prototype.scrollIntoView) HTMLElement.prototype.scrollIntoView = vi.fn()
})

afterEach(async () => {
  keyboardActionMenu.close()
  while (mounted.length > 0) await unmount(mounted.pop())
  vi.restoreAllMocks()
})

test('opens with focus and activates the keyboard-selected action', async () => {
  const { target, actions } = await renderMenu()
  const click = vi.spyOn(actions[1].element, 'click')
  assert.equal(guards.open.mock.calls.length, 1)
  assert.match(target.textContent, /First action/)
  assert.match(target.textContent, /Second action/)
  const input = target.querySelector('input')
  assert.equal(document.activeElement, input)

  input.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'ArrowDown', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.match(target.querySelector('[data-keyboard-action-index="1"]').className, /bg-muted/)

  input.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.equal(click.mock.calls.length, 1)
  assert.equal(keyboardActionMenu.open, false)
  assert.equal(target.querySelector('[role="dialog"]'), null)
  assert.ok(guards.close.mock.calls.length >= 1)
})

test('filters actions, supports pointer activation, and renders an empty result', async () => {
  const actions = makeActions()
  const click = vi.spyOn(actions[2].element, 'click')
  const { target } = await renderMenu(actions)
  const input = target.querySelector('input')
  inputText(input, '  THIRD  ')
  await flushAsync()
  assert.doesNotMatch(target.textContent, /First action/)
  assert.match(target.textContent, /Third action/)

  const visibleAction = target.querySelector('[data-keyboard-action-index="0"]')
  visibleAction.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }))
  visibleAction.click()
  await flushAsync()
  assert.equal(click.mock.calls.length, 1)
  assert.equal(keyboardActionMenu.open, false)

  keyboardActionMenu.actions = actions
  keyboardActionMenu.open = true
  await flushAsync()
  const reopenedInput = target.querySelector('input')
  inputText(reopenedInput, 'nothing matches')
  await flushAsync()
  assert.match(target.textContent, /keyboardActions\.empty/)
  reopenedInput.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'ArrowDown', bubbles: true, cancelable: true,
  }))
  reopenedInput.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', bubbles: true, cancelable: true,
  }))
  assert.equal(keyboardActionMenu.open, true)
})

test('wraps upward, ignores IME events, and closes from button, Escape, and backdrop', async () => {
  const actions = makeActions()
  const thirdClick = vi.spyOn(actions[2].element, 'click')
  const { target } = await renderMenu(actions)
  const input = target.querySelector('input')
  input.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'ArrowDown', isComposing: true, bubbles: true, cancelable: true,
  }))
  input.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'ArrowUp', bubbles: true, cancelable: true,
  }))
  input.dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Enter', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.equal(thirdClick.mock.calls.length, 1)

  keyboardActionMenu.actions = actions
  keyboardActionMenu.open = true
  await flushAsync()
  target.querySelector('[aria-label="common.close"]').click()
  await flushAsync()
  assert.equal(keyboardActionMenu.open, false)

  keyboardActionMenu.actions = actions
  keyboardActionMenu.open = true
  await flushAsync()
  target.querySelector('[role="dialog"]').dispatchEvent(new KeyboardEvent('keydown', {
    key: 'Escape', bubbles: true, cancelable: true,
  }))
  await flushAsync()
  assert.equal(keyboardActionMenu.open, false)

  keyboardActionMenu.actions = actions
  keyboardActionMenu.open = true
  await flushAsync()
  target.querySelector('[role="dialog"]').parentElement.click()
  await flushAsync()
  assert.equal(keyboardActionMenu.open, false)
})
