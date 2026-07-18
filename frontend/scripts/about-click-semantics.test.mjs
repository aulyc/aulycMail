import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const aboutTabPath = new URL('../src/lib/components/settings/AboutTab.svelte', import.meta.url)
const aboutInfoDialogPath = new URL('../src/lib/components/settings/AboutInfoDialog.svelte', import.meta.url)

test('about actions use standard click activation instead of pointer-down activation', async () => {
  const [aboutTab, aboutInfoDialog] = await Promise.all([
    readFile(aboutTabPath, 'utf8'),
    readFile(aboutInfoDialogPath, 'utf8'),
  ])

  assert.doesNotMatch(aboutTab, /onpointerdown=/)
  assert.doesNotMatch(aboutInfoDialog, /onpointerdown=/)
  assert.match(aboutTab, /onclick=\{\(\) => openInfo\('product'\)\}/)
  assert.match(aboutInfoDialog, /onclick=\{close\}/)
})
