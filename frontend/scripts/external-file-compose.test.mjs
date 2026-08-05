import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  normalizeExternalOpenFiles,
  toExternalSmtpAttachment,
} from '../src/lib/externalFileCompose.ts'

test('normalizes and deduplicates native file-open payloads', () => {
  assert.deepEqual(normalizeExternalOpenFiles({
    paths: ['/tmp/report.pdf', '/tmp/report.pdf', '', 42, '/tmp/photo.jpg'],
  }), ['/tmp/report.pdf', '/tmp/photo.jpg'])
  assert.deepEqual(normalizeExternalOpenFiles(null), [])
  assert.deepEqual(normalizeExternalOpenFiles({ paths: 'not-an-array' }), [])
})

test('maps a backend file read to a regular SMTP attachment', () => {
  assert.deepEqual(toExternalSmtpAttachment({
    filename: 'report.pdf',
    contentType: 'application/pdf',
    size: 3,
    data: 'YWJj',
  }), {
    filename: 'report.pdf',
    content_type: 'application/pdf',
    content: [],
    content_base64: 'YWJj',
    content_id: '',
    inline: false,
  })
})
