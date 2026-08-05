import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  SEARCH_RESULT_ROW_HEIGHT_PX,
  SEARCH_RESULT_VIEWPORT_HEIGHT_PX,
  SEARCH_RESULT_VISIBLE_ROWS,
  resultRowHighlightClass,
  searchResultScrollTopForIndex,
  shouldActivatePointerResult,
} from '../src/lib/components/search/searchResultHighlight.ts'

test('keyboard navigation renders exactly one persistent highlight', () => {
  assert.equal(resultRowHighlightClass(2, 2, 'keyboard'), 'bg-muted')
  assert.equal(resultRowHighlightClass(3, 2, 'keyboard'), '')
})

test('pointer mode uses hover while scrolling idle mode clears all highlights', () => {
  assert.equal(resultRowHighlightClass(2, 2, 'pointer'), 'hover:bg-muted/50')
  assert.equal(resultRowHighlightClass(3, -1, 'idle'), '')
})

test('the search viewport fits exactly eight fixed-height result rows', () => {
  assert.equal(SEARCH_RESULT_VISIBLE_ROWS, 8)
  assert.equal(SEARCH_RESULT_VIEWPORT_HEIGHT_PX, SEARCH_RESULT_ROW_HEIGHT_PX * 8)
  assert.equal(SEARCH_RESULT_VIEWPORT_HEIGHT_PX, 416)
})

test('keyboard navigation keeps the active result inside the eight-row viewport', () => {
  assert.equal(searchResultScrollTopForIndex(7, 0, SEARCH_RESULT_VIEWPORT_HEIGHT_PX), 0)
  assert.equal(searchResultScrollTopForIndex(8, 0, SEARCH_RESULT_VIEWPORT_HEIGHT_PX), 52)
  assert.equal(searchResultScrollTopForIndex(15, 52, SEARCH_RESULT_VIEWPORT_HEIGHT_PX), 416)
  assert.equal(searchResultScrollTopForIndex(0, 52, SEARCH_RESULT_VIEWPORT_HEIGHT_PX), 0)
})

test('programmatic scrolling cannot hand keyboard selection to a stationary pointer', () => {
  assert.equal(shouldActivatePointerResult(0, 0, 500, 0), false)
  assert.equal(shouldActivatePointerResult(4, 0, 100, 250), false)
  assert.equal(shouldActivatePointerResult(0, -3, 300, 250), true)
})
