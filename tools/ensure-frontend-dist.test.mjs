import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { ensureFrontendDist } from './ensure-frontend-dist.mjs'

function withTempRoot(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aulycmail-frontend-dist-test-'))
  t.after(() => fs.rmSync(root, { recursive: true, force: true }))
  return root
}

test('creates an embed-safe placeholder for a clean checkout', (t) => {
  const root = withTempRoot(t)

  assert.equal(ensureFrontendDist(root), true)
  assert.equal(fs.existsSync(path.join(root, 'frontend', 'dist', 'index.html')), true)
})

test('preserves an existing frontend build without adding a placeholder', (t) => {
  const root = withTempRoot(t)
  const asset = path.join(root, 'frontend', 'dist', 'assets', 'main.js')
  fs.mkdirSync(path.dirname(asset), { recursive: true })
  fs.writeFileSync(asset, 'existing-build')

  assert.equal(ensureFrontendDist(root), false)
  assert.equal(fs.readFileSync(asset, 'utf8'), 'existing-build')
  assert.equal(fs.existsSync(path.join(root, 'frontend', 'dist', 'index.html')), false)
})
