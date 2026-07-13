import assert from 'node:assert/strict'
import test from 'node:test'
import { getComposerDarkFilterTargetIndexes } from '../src/lib/components/composer/composerDarkFilter.ts'

test('forward dark filter excludes typing area, signature, and forward header', () => {
  const nodes = [
    { type: 'paragraph', textContent: 'New text' },
    { type: 'paragraph', textContent: 'Signature' },
    { type: 'paragraph', textContent: '---------- Forwarded message ----------\nFrom: sender@example.com' },
    { type: 'paragraph', textContent: '' },
    { type: 'paragraph', textContent: 'Original message' },
    { type: 'table', textContent: 'Original table' },
  ]

  assert.deepEqual(getComposerDarkFilterTargetIndexes('forward', nodes), [3, 4, 5])
})

test('reply dark filter targets only the quoted block', () => {
  const nodes = [
    { type: 'paragraph', textContent: 'New text' },
    { type: 'paragraph', textContent: 'Signature' },
    { type: 'paragraph', textContent: 'On Monday, sender wrote:' },
    { type: 'blockquote', textContent: 'Original message' },
  ]

  assert.deepEqual(getComposerDarkFilterTargetIndexes('reply', nodes), [3])
  assert.deepEqual(getComposerDarkFilterTargetIndexes('reply-all', nodes), [3])
})

test('dark filter does not guess when a quote boundary is absent', () => {
  const nodes = [{ type: 'paragraph', textContent: 'Draft content' }]

  assert.deepEqual(getComposerDarkFilterTargetIndexes('new', nodes), [])
  assert.deepEqual(getComposerDarkFilterTargetIndexes('forward', nodes), [])
})
