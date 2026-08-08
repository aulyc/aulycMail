import assert from 'node:assert/strict'
import { test } from 'vitest'
import { backupProgressPercent, backupStatistics } from '../src/lib/backup/backupStatistics.ts'

test('presents backup counters as coverage and mutually exclusive outcomes', () => {
  assert.deepEqual(backupStatistics({
    total: 21438,
    exported: 27,
    skipped: 21407,
    missing: 4,
    unavailable: 0,
    failed: 0,
  }), {
    checked: 21438,
    backedUp: 21434,
    notBackedUp: 4,
    newlyBackedUp: 27,
    previouslyBackedUp: 21407,
    serverNotReturned: 4,
    noReadableSource: 0,
    processingFailed: 0,
  })
})

test('derives the checked total for legacy activity payloads', () => {
  assert.deepEqual(backupStatistics({
    exported: 2,
    skipped: 3,
    missing: 1,
    unavailable: 1,
    failed: 1,
  }), {
    checked: 8,
    backedUp: 5,
    notBackedUp: 3,
    newlyBackedUp: 2,
    previouslyBackedUp: 3,
    serverNotReturned: 1,
    noReadableSource: 1,
    processingFailed: 1,
  })
})

test('honours the legacy completed counter when detailed success counters are absent', () => {
  const result = backupStatistics({ total: 10, completed: 8, missing: 2 })

  assert.equal(result.backedUp, 8)
  assert.equal(result.notBackedUp, 2)
  assert.equal(result.checked, 10)
})

test('calculates a bounded determinate backup percentage', () => {
  assert.equal(backupProgressPercent(25, 100), 25)
  assert.equal(backupProgressPercent(21537, 21542), 99)
  assert.equal(backupProgressPercent(21542, 21542), 100)
  assert.equal(backupProgressPercent(200, 100), 100)
  assert.equal(backupProgressPercent(-1, 100), 0)
  assert.equal(backupProgressPercent(0, 0), null)
})
