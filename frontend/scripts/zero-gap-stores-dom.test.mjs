// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { afterEach, beforeEach, test, vi } from 'vitest'

const settings = vi.hoisted(() => ({ enabled: true }))
const backend = vi.hoisted(() => ({ GetImageAllowlist: vi.fn() }))

vi.mock('$lib/stores/settings.svelte', () => ({ getEnhancedKeyboardNavigation: () => settings.enabled }))
vi.mock('../wailsjs/go/app/App.js', () => backend)

import {
  focusCurrentPane,
  focusNextPane,
  focusPane,
  focusPreviousPane,
  getFocusedPane,
  getPaneNav,
  isComposerOpen,
  isInputElement,
  isMainKeyboardScope,
  registerPaneNav,
  setComposerOpen,
  setFocusedPane,
  setKeyboardScope,
} from '../src/lib/stores/keyboard.svelte.ts'
import { keyboardActionMenu } from '../src/lib/stores/keyboardActionMenu.svelte.ts'
import { isImageAllowedSync, loadImageAllowlist, refreshImageAllowlist } from '../src/lib/stores/imageAllowlist.svelte.ts'

function visibleRect() {
  return { x: 0, y: 0, width: 100, height: 30, top: 0, right: 100, bottom: 30, left: 0, toJSON() {} }
}

beforeEach(() => {
  document.body.innerHTML = ''
  settings.enabled = true
  setKeyboardScope('main')
  setFocusedPane('messageList')
  setComposerOpen(false)
  keyboardActionMenu.close()
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([visibleRect()])
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(visibleRect())
  backend.GetImageAllowlist.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('keyboard store focuses visible pane targets, cycles regions, and respects scope/settings', () => {
  const feature = document.createElement('nav')
  feature.dataset.keyboardRegion = 'featureNav'
  feature.dataset.keyboardRegionFocusTarget = ''
  feature.tabIndex = 0
  const sidebar = document.createElement('aside')
  sidebar.dataset.keyboardRegion = 'sidebar'
  const sidebarTarget = document.createElement('button')
  sidebarTarget.dataset.keyboardRegionFocusTarget = ''
  sidebar.appendChild(sidebarTarget)
  const list = document.createElement('section')
  list.dataset.keyboardRegion = 'messageList'
  list.dataset.keyboardRegionFocusTarget = ''
  list.tabIndex = 0
  const viewer = document.createElement('section')
  viewer.dataset.keyboardRegion = 'viewer'
  viewer.dataset.keyboardRegionVisible = 'false'
  document.body.append(feature, sidebar, list, viewer)

  assert.equal(focusPane('sidebar'), true)
  assert.equal(getFocusedPane(), 'sidebar')
  assert.equal(document.activeElement, sidebarTarget)
  focusNextPane()
  assert.equal(getFocusedPane(), 'messageList')
  focusPreviousPane()
  assert.equal(getFocusedPane(), 'sidebar')
  assert.equal(focusCurrentPane(), true)

  setKeyboardScope('settings')
  assert.equal(isMainKeyboardScope(), false)
  setKeyboardScope('main')
  settings.enabled = false
  assert.equal(isMainKeyboardScope(), false)
  assert.equal(focusPane('featureNav'), false)
  settings.enabled = true
  assert.equal(focusPane('viewer'), false)

  sidebar.hidden = true
  assert.equal(focusPane('sidebar'), false)
  sidebar.hidden = false
  sidebar.setAttribute('aria-hidden', 'true')
  assert.equal(focusPane('sidebar'), false)
})

test('keyboard store classifies inputs and manages pane registries and composer state', () => {
  const wrapper = document.createElement('div')
  const input = document.createElement('input')
  const textarea = document.createElement('textarea')
  const select = document.createElement('select')
  const editable = document.createElement('div')
  editable.contentEditable = 'true'
  const textbox = document.createElement('div')
  textbox.setAttribute('role', 'textbox')
  const searchbox = document.createElement('div')
  searchbox.setAttribute('role', 'searchbox')
  const combobox = document.createElement('div')
  combobox.setAttribute('role', 'combobox')
  const custom = document.createElement('div')
  custom.dataset.keyboardInput = 'true'
  wrapper.append(input, textarea, select, editable, textbox, searchbox, combobox, custom)
  document.body.appendChild(wrapper)
  for (const element of [input, textarea, select, editable, textbox, searchbox, combobox, custom]) {
    assert.equal(isInputElement(element), true)
  }
  assert.equal(isInputElement(wrapper), false)
  assert.equal(isInputElement(null), false)

  const target = { navigateNext: vi.fn() }
  const unregister = registerPaneNav('sidebar', target)
  assert.equal(getPaneNav('sidebar'), target)
  const replacement = { navigatePrev: vi.fn() }
  const unregisterReplacement = registerPaneNav('sidebar', replacement)
  unregister()
  assert.equal(getPaneNav('sidebar'), replacement)
  unregisterReplacement()
  assert.equal(getPaneNav('sidebar'), undefined)

  setComposerOpen(true)
  assert.equal(isComposerOpen(), true)
  setComposerOpen(false)
  assert.equal(isComposerOpen(), false)
})

test('keyboard action menu collects visible labeled actions, deduplicates, activates, and closes safely', async () => {
  vi.useFakeTimers()
  const root = document.createElement('section')
  root.dataset.keyboardRegion = 'viewer'
  const context = document.createElement('div')
  context.dataset.keyboardActionContext = 'ada@example.test'
  const first = document.createElement('button')
  first.textContent = 'Copy'
  const second = document.createElement('button')
  second.textContent = 'Copy'
  const link = document.createElement('a')
  link.href = '#test'
  link.title = 'Open'
  const checkbox = document.createElement('input')
  checkbox.type = 'checkbox'
  checkbox.setAttribute('aria-label', 'Choose')
  const roleButton = document.createElement('div')
  roleButton.setAttribute('role', 'button')
  roleButton.dataset.keyboardActionLabel = 'Role action'
  const disabled = document.createElement('button')
  disabled.disabled = true
  disabled.textContent = 'Disabled'
  const hidden = document.createElement('button')
  hidden.hidden = true
  hidden.textContent = 'Hidden'
  const long = document.createElement('button')
  long.textContent = 'x'.repeat(101)
  const ownMenu = document.createElement('div')
  ownMenu.dataset.keyboardActionMenu = ''
  ownMenu.innerHTML = '<button>Recursive</button>'
  context.append(first, second)
  root.append(context, link, checkbox, roleButton, disabled, hidden, long, ownMenu)
  document.body.appendChild(root)

  assert.equal(keyboardActionMenu.showForRegion('viewer'), true)
  assert.deepEqual(keyboardActionMenu.actions.map((action) => action.label), [
    'Copy — ada@example.test', 'Copy — ada@example.test (2)', 'Open', 'Choose', 'Role action',
  ])
  const click = vi.spyOn(first, 'click')
  keyboardActionMenu.activate(keyboardActionMenu.actions[0])
  assert.equal(keyboardActionMenu.open, false)
  await vi.runAllTimersAsync()
  assert.equal(click.mock.calls.length, 1)

  assert.equal(keyboardActionMenu.showForRoot(null), false)
  const empty = document.createElement('div')
  document.body.appendChild(empty)
  assert.equal(keyboardActionMenu.showForRoot(empty), false)
  assert.equal(keyboardActionMenu.showForRegion('missing'), false)

  keyboardActionMenu.showForRoot(root)
  const detached = keyboardActionMenu.actions[0]
  detached.element.remove()
  keyboardActionMenu.activate(detached)
  assert.equal(keyboardActionMenu.actions.length, 0)
})

test('image allowlist loads sender/domain rules, rejects malformed addresses, refreshes, and contains errors', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  assert.equal(isImageAllowedSync('person@example.test'), false)
  backend.GetImageAllowlist.mockResolvedValueOnce([
    { type: 'sender', value: 'person@example.test' },
    { type: 'domain', value: 'trusted.test' },
  ])
  await loadImageAllowlist()
  assert.equal(isImageAllowedSync(' Person@Example.Test '), true)
  assert.equal(isImageAllowedSync('other@trusted.test'), true)
  assert.equal(isImageAllowedSync('other@blocked.test'), false)
  assert.equal(isImageAllowedSync(''), false)
  assert.equal(isImageAllowedSync('malformed'), false)

  backend.GetImageAllowlist.mockResolvedValueOnce(null)
  refreshImageAllowlist()
  await Promise.resolve()
  await Promise.resolve()
  await Promise.resolve()
  assert.equal(isImageAllowedSync('person@example.test'), false)
  backend.GetImageAllowlist.mockRejectedValueOnce(new Error('allowlist unavailable'))
  await loadImageAllowlist()
  assert.equal(error.mock.calls.length, 1)
})
