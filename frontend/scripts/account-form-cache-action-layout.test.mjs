import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const accountFormPath = new URL(
  '../src/lib/components/settings/AccountForm.svelte',
  import.meta.url,
)

test('offline body cache action lives beside test connection in the pinned footer', async () => {
  const source = await readFile(accountFormPath, 'utf8')
  const footerStart = source.indexOf('<!-- Actions (pinned footer')
  const footerEnd = source.indexOf('</form>', footerStart)

  assert.notEqual(footerStart, -1)
  assert.notEqual(footerEnd, -1)

  const fields = source.slice(0, footerStart)
  const footer = source.slice(footerStart, footerEnd)

  assert.doesNotMatch(fields, /account\.clearOfflineBodyCache/)
  assert.match(
    footer,
    /account\.testConnection[\s\S]*account\.clearOfflineBodyCache/,
  )
  assert.match(
    footer,
    /\{#if editAccount\}[\s\S]*showClearOfflineBodyCacheConfirm = true[\s\S]*account\.clearOfflineBodyCache[\s\S]*\{\/if\}/,
  )
})
