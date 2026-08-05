import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  MAIN_KEYBOARD_REGION_ORDER,
  nextRovingIndex,
  nextVisibleRegion,
} from '../src/lib/keyboard/regionNavigation.ts'
import {
  isOptionCodeShortcut,
  resolveAppKeyboardPolicy,
} from '../src/lib/keyboard/keyboardPolicy.ts'

function keyboardEvent(overrides = {}) {
  return {
    key: '',
    code: '',
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    ...overrides,
  }
}

test('enhanced keyboard policy leaves native keys alone while keeping Command-F search', () => {
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent({ key: 'Tab' }), false), 'native')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent({ key: 'Tab' }), true), 'enhanced')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent({ key: 'f', metaKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent({ key: 'F', ctrlKey: true }), false), 'search')
  assert.equal(resolveAppKeyboardPolicy(keyboardEvent({ key: 'f', metaKey: true, altKey: true }), false), 'native')
})

test('macOS Option-letter shortcuts match physical codes instead of produced symbols', () => {
  assert.equal(isOptionCodeShortcut(keyboardEvent({ key: '†', code: 'KeyT', altKey: true }), 'KeyT'), true)
  assert.equal(isOptionCodeShortcut(keyboardEvent({ key: 'å', code: 'KeyA', altKey: true }), 'KeyA'), true)
  assert.equal(isOptionCodeShortcut(keyboardEvent({ key: 't', code: 'KeyY', altKey: true }), 'KeyT'), false)
})

test('visible keyboard regions cycle in both directions and skip hidden regions', () => {
  assert.deepEqual(MAIN_KEYBOARD_REGION_ORDER, ['featureNav', 'sidebar', 'messageList', 'viewer'])
  const visible = ['featureNav', 'sidebar', 'viewer']
  assert.equal(nextVisibleRegion('featureNav', 1, visible), 'sidebar')
  assert.equal(nextVisibleRegion('sidebar', 1, visible), 'viewer')
  assert.equal(nextVisibleRegion('featureNav', -1, visible), 'viewer')
  assert.equal(nextVisibleRegion('messageList', 1, visible), 'viewer')
  assert.equal(nextRovingIndex('ArrowUp', 0, 4, true), 3)
  assert.equal(nextRovingIndex('ArrowDown', 3, 4, true), 0)
})
