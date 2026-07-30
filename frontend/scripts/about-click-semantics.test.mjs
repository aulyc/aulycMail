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

test('about links share aligned icon and text columns', async () => {
  const aboutTab = await readFile(aboutTabPath, 'utf8')

  assert.match(aboutTab, /class="flex w-max flex-col items-stretch gap-2"/)
  assert.match(aboutTab, /class="flex w-full items-center justify-start gap-2 text-left[^"]*"/)
  assert.match(aboutTab, /icon="lucide:heart-handshake" class="h-5 w-5 shrink-0"/)
})

test('about information dialogs autofocus the primary close action for keyboard activation', async () => {
  const aboutInfoDialog = await readFile(aboutInfoDialogPath, 'utf8')

  assert.match(
    aboutInfoDialog,
    /function handleOpenAutoFocus[\s\S]*event\.preventDefault\(\)[\s\S]*if \(open\) primaryAction\?\.focus\(\{ preventScroll: true \}\)/,
  )
  assert.match(aboutInfoDialog, /onOpenAutoFocus=\{handleOpenAutoFocus\}/)
  assert.match(aboutInfoDialog, /bind:this=\{primaryAction\}/)
})
