import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const dialogPath = new URL('../src/lib/components/settings/backup/BackupProgressDialog.svelte', import.meta.url)

test('backup progress dialog presents progress as a horizontal bar', async () => {
  const dialog = await readFile(dialogPath, 'utf8')

  assert.doesNotMatch(dialog, /<svg/)
  assert.match(dialog, /class="relative h-2\.5 overflow-hidden rounded-full bg-muted"/)
  assert.match(dialog, /backup-progress-indeterminate/)
  assert.match(dialog, /style=\{`width: \$\{displayedPercent\}%`\}/)
})

test('backup progress dialog reserves the target row in every state', async () => {
  const dialog = await readFile(dialogPath, 'utf8')

  assert.doesNotMatch(dialog, /\{#if active && target\}\s*<p/)
  assert.match(dialog, /aria-hidden=\{!\(active && target\)\}/)
  assert.match(dialog, /active && target \? '' : 'invisible'/)
})
