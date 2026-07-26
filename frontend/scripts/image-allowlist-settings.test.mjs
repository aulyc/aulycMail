import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const imagesTabPath = new URL('../src/lib/components/settings/ImagesTab.svelte', import.meta.url)

test('image allowlist removal requires confirmation before the backend mutation', async () => {
  const source = await readFile(imagesTabPath, 'utf8')

  assert.match(source, /let pendingRemoval = \$state<settings\.AllowlistEntry \| null>\(null\)/)
  assert.match(source, /onclick=\{\(\) => requestRemove\(entry\)\}/)
  assert.doesNotMatch(source, /onclick=\{\(\) => (?:handle|confirm)Remove\(entry\.id\)\}/)
  assert.match(
    source,
    /async function confirmRemove\(\)[\s\S]*const entry = pendingRemoval[\s\S]*await RemoveImageAllowlist\(entry\.id\)/,
  )
  assert.match(
    source,
    /<ConfirmDialog[\s\S]*open=\{pendingRemoval !== null\}[\s\S]*title=\{\$_\('images\.removeConfirmTitle'\)\}[\s\S]*variant="destructive"[\s\S]*onConfirm=\{confirmRemove\}/,
  )
})
