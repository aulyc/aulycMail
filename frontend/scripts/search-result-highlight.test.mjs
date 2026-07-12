import assert from 'node:assert/strict'
import test from 'node:test'
import { resultRowHighlightClass } from '../src/lib/components/search/searchResultHighlight.ts'

test('keyboard navigation renders exactly one persistent highlight', () => {
  assert.equal(resultRowHighlightClass(2, 2, 'keyboard'), 'bg-muted')
  assert.equal(resultRowHighlightClass(3, 2, 'keyboard'), '')
})

test('pointer and idle modes use hover without a stale persistent highlight', () => {
  assert.equal(resultRowHighlightClass(2, 2, 'pointer'), 'hover:bg-muted/50')
  assert.equal(resultRowHighlightClass(3, -1, 'idle'), 'hover:bg-muted/50')
})
