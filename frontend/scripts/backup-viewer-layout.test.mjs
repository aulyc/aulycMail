import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const toolbarPath = new URL('../src/lib/components/backup/BackupViewerToolbar.svelte', import.meta.url)
const dialogPath = new URL('../src/lib/components/backup/BackupViewerDialog.svelte', import.meta.url)
const selectItemPath = new URL('../src/lib/components/ui/select/select-item.svelte', import.meta.url)

test('backup viewer account selection indicator stays at the left edge', async () => {
  const [toolbar, selectItem] = await Promise.all([
    readFile(toolbarPath, 'utf8'),
    readFile(selectItemPath, 'utf8'),
  ])

  assert.match(toolbar, /<Select\.Item[^>]*indicatorPosition="start"/)
  assert.match(selectItem, /indicatorPosition\?: 'start' \| 'end'/)
  assert.match(selectItem, /\{#if indicatorPosition === 'start'\}[\s\S]*class="min-w-0 flex-1"/)
})

test('backup viewer message rows keep time on one line and use only the edge attachment marker', async () => {
  const dialog = await readFile(dialogPath, 'utf8')

  assert.match(dialog, /grid-cols-\[1rem_minmax\(0,1fr\)_auto\]/)
  assert.match(dialog, /<time[^>]*whitespace-nowrap[^>]*>\{formatShortDate\(message\.date\)\}<\/time>/)
  assert.doesNotMatch(dialog, /icon="mdi:paperclip"/)
  assert.match(dialog, /w-\[5px\][^"]*\{hasAttachments \? 'bg-amber-500' : 'bg-transparent'\}/)
})
