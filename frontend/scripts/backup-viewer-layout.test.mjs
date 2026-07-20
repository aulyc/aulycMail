import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const toolbarPath = new URL('../src/lib/components/backup/BackupViewerToolbar.svelte', import.meta.url)
const dialogPath = new URL('../src/lib/components/backup/BackupViewerDialog.svelte', import.meta.url)
const detailPath = new URL('../src/lib/components/backup/BackupViewerMessageDetail.svelte', import.meta.url)
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
  assert.match(dialog, /absolute bottom-0 left-0 top-0 w-\[5px\][^"]*\{hasAttachments \? 'bg-amber-500' : 'bg-transparent'\}/)
  assert.doesNotMatch(dialog, /absolute bottom-0 right-0 top-0 w-\[5px\]/)
})

test('backup viewer toolbar controls align with the message detail cards', async () => {
  const [toolbar, dialog, detail] = await Promise.all([
    readFile(toolbarPath, 'utf8'),
    readFile(dialogPath, 'utf8'),
    readFile(detailPath, 'utf8'),
  ])

  assert.match(toolbar, /grid-cols-\[42%_minmax\(0,1fr\)\]/)
  assert.match(dialog, /grid-cols-\[42%_1fr\]/)
  assert.match(toolbar, /data-backup-viewer-detail-toolbar[^>]*class="[^"]*pl-6/)
  assert.match(detail, /class="min-h-0 flex-1 overflow-y-auto px-6 py-5 scrollbar-thin"/)
})

test('backup viewer directory picker ends at the message list edge', async () => {
  const [toolbar, dialog] = await Promise.all([
    readFile(toolbarPath, 'utf8'),
    readFile(dialogPath, 'utf8'),
  ])

  assert.match(dialog, /grid-cols-\[42%_1fr\]/)
  assert.match(toolbar, /grid-cols-\[42%_minmax\(0,1fr\)\]/)
  assert.match(toolbar, /data-backup-viewer-list-toolbar[^>]*class="[^"]*pl-4/)
  assert.doesNotMatch(toolbar, /data-backup-viewer-list-toolbar[^>]*class="[^"]*pr-/)
  assert.match(toolbar, /<div class="min-w-\[240px\] flex-1">\s*<BackupDirectoryPicker/)
  assert.doesNotMatch(toolbar, /w-\[340px\]/)
})
