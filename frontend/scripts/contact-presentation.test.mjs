import assert from 'node:assert/strict'
import test from 'node:test'
import { shouldShowContactEmail } from '../src/lib/contacts/utils/contactPresentation.ts'

test('hides an email subtitle when the display name is the same address', () => {
  assert.equal(shouldShowContactEmail('user@example.com', 'user@example.com'), false)
  assert.equal(shouldShowContactEmail(' USER@example.com ', 'user@example.com'), false)
})

test('shows an email subtitle only when it adds information', () => {
  assert.equal(shouldShowContactEmail('Example User', 'user@example.com'), true)
  assert.equal(shouldShowContactEmail('', 'user@example.com'), false)
  assert.equal(shouldShowContactEmail('Example User', ''), false)
})
