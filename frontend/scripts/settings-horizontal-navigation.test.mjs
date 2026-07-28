import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveHorizontalActionIndex } from '../src/lib/keyboard/settingsHorizontalNavigation.ts'

const accountActions = (moveUpAvailable = true, moveDownAvailable = true) => [
  { action: 'test', available: true },
  { action: 'move-up', available: moveUpAvailable },
  { action: 'move-down', available: moveDownAvailable },
  { action: 'edit', available: true },
  { action: 'delete', available: true },
]

test('keeps the same account action selected after an ordinary reorder', () => {
  assert.equal(resolveHorizontalActionIndex(accountActions(), 'move-up'), 1)
  assert.equal(resolveHorizontalActionIndex(accountActions(), 'move-down'), 2)
})

test('selects the opposite reorder action when the original reaches a boundary', () => {
  assert.equal(resolveHorizontalActionIndex(accountActions(false, true), 'move-up'), 2)
  assert.equal(resolveHorizontalActionIndex(accountActions(true, false), 'move-down'), 1)
})

test('falls back to the nearest available action and reports an empty group', () => {
  assert.equal(
    resolveHorizontalActionIndex([
      { action: 'test', available: false },
      { action: 'move-up', available: false },
      { action: 'edit', available: true },
    ], 'move-up'),
    2,
  )
  assert.equal(resolveHorizontalActionIndex([], 'move-up'), -1)
})
