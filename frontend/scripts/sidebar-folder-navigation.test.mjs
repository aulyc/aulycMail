import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import {
  buildFolderNavigationList,
  nextSidebarAction,
  nextSidebarNavigationIndex,
} from '../src/lib/components/sidebar/folderNavigation.ts'

const sidebarPath = new URL('../src/lib/components/sidebar/Sidebar.svelte', import.meta.url)
const accountSectionPath = new URL('../src/lib/components/sidebar/AccountSection.svelte', import.meta.url)
const folderTreeItemPath = new URL('../src/lib/components/sidebar/FolderTreeItem.svelte', import.meta.url)
const globalShortcutsPath = new URL('../src/lib/keyboard/globalShortcuts.ts', import.meta.url)

const accounts = [
  {
    account: { id: 'account-a', name: 'Account A' },
    folders: [
      {
        folder: {
          id: 'trash',
          name: 'Deleted Messages',
          path: 'Deleted Messages',
          type: 'trash',
          noSelect: false,
        },
      },
      {
        folder: {
          id: 'other-folders',
          name: 'Other folders',
          path: 'Other folders',
          type: 'folder',
          noSelect: true,
        },
        children: [
          {
            folder: {
              id: 'receipts',
              name: 'Receipts',
              path: 'Other folders/Receipts',
              type: 'folder',
              noSelect: false,
            },
          },
        ],
      },
    ],
  },
  {
    account: { id: 'account-b', name: 'Account B' },
    folders: [],
  },
]

test('the compose and sync group leads the vertical sidebar order as one item', () => {
  const items = buildFolderNavigationList(
    accounts,
    { 'account-a': true, 'account-b': true },
    {},
  )

  assert.equal(items[0]?.type, 'sidebar-actions')
  assert.equal(items[1]?.type, 'account-header')
})

test('vertical sidebar navigation wraps through the action group at both boundaries', () => {
  assert.equal(nextSidebarNavigationIndex(0, 8, -1), 7)
  assert.equal(nextSidebarNavigationIndex(7, 8, 1), 0)
  assert.equal(nextSidebarNavigationIndex(-1, 8, 1), 0)
  assert.equal(nextSidebarNavigationIndex(-1, 8, -1), 7)
})

test('left and right wrap within the horizontal compose and sync group', () => {
  assert.equal(nextSidebarAction('compose', 1), 'sync')
  assert.equal(nextSidebarAction('sync', 1), 'compose')
  assert.equal(nextSidebarAction('sync', -1), 'compose')
  assert.equal(nextSidebarAction('compose', -1), 'sync')
})

test('collapsed directory-only groups remain in the visible arrow-key order', () => {
  const items = buildFolderNavigationList(
    accounts,
    { 'account-a': true, 'account-b': true },
    {},
  )
  const trashIndex = items.findIndex((item) => item.folderId === 'trash')

  assert.equal(items[trashIndex + 1]?.type, 'folder-group')
  assert.equal(items[trashIndex + 1]?.folderId, 'other-folders')
  assert.equal(items.some((item) => item.folderId === 'receipts'), false)
})

test('expanding a directory-only group places its child folders immediately after it', () => {
  const items = buildFolderNavigationList(
    accounts,
    { 'account-a': true, 'account-b': true },
    { 'other-folders': false },
  )
  const groupIndex = items.findIndex((item) => item.folderId === 'other-folders')

  assert.equal(items[groupIndex]?.type, 'folder-group')
  assert.equal(items[groupIndex + 1]?.type, 'folder')
  assert.equal(items[groupIndex + 1]?.folderId, 'receipts')
})

test('non-mailbox rows use one blue keyboard cursor without retaining the old folder highlight', async () => {
  const [sidebar, accountSection, folderTreeItem, globalShortcuts] = await Promise.all([
    readFile(sidebarPath, 'utf8'),
    readFile(accountSectionPath, 'utf8'),
    readFile(folderTreeItemPath, 'utf8'),
    readFile(globalShortcutsPath, 'utf8'),
  ])

  assert.match(sidebar, /item\.type === 'folder-group'[\s\S]*focusedFolderGroupId = item\.folderId/)
  assert.match(sidebar, /export function hasFocusedFolderGroup\(\)/)
  assert.match(sidebar, /showFolderSelection=\{!sidebarActionsFocused && focusedAccountId === null && focusedFolderGroupId === null\}/)
  assert.match(accountSection, /isHeaderFocused \? 'bg-primary\/10 text-primary'/)
  assert.match(accountSection, /\{showFolderSelection\}/)
  assert.match(folderTreeItem, /return showFolderSelection && selectionSource === 'account'/)
  assert.match(folderTreeItem, /isFolderGroupFocused\(tree\.folder\.id\)[\s\S]*bg-primary\/10 text-primary font-medium/)
  assert.match(globalShortcuts, /hasFocusedFolderGroup\(\)[\s\S]*toggleFocusedFolderGroup\(\)/)
})

test('sidebar action buttons switch horizontally and activate with Enter or Space', async () => {
  const [sidebar, globalShortcuts] = await Promise.all([
    readFile(sidebarPath, 'utf8'),
    readFile(globalShortcutsPath, 'utf8'),
  ])

  assert.match(sidebar, /data-sidebar-action="compose"[\s\S]*data-sidebar-action="sync"/)
  assert.match(sidebar, /showFolderSelection=\{!sidebarActionsFocused/)
  assert.match(sidebar, /export function activateFocusedSidebarAction\(\)/)
  assert.match(globalShortcuts, /e\.key === 'ArrowLeft'[\s\S]*moveFocusedSidebarAction\(-1\)/)
  assert.match(globalShortcuts, /e\.key === 'ArrowRight'[\s\S]*moveFocusedSidebarAction\(1\)/)
  assert.match(globalShortcuts, /case 'Enter':[\s\S]*hasFocusedSidebarAction\(\)[\s\S]*activateFocusedSidebarAction\(\)/)
  assert.match(globalShortcuts, /case ' ':[\s\S]*hasFocusedSidebarAction\(\)[\s\S]*activateFocusedSidebarAction\(\)/)
})

test('spam folder unread badges use a neutral tone while ordinary unread stays primary', async () => {
  const folderTreeItem = await readFile(folderTreeItemPath, 'utf8')

  assert.match(folderTreeItem, /let isSpamFolder = \$derived\(tree\.folder\?\.type === 'spam'\)/)
  assert.match(
    folderTreeItem,
    /isSpamFolder\s*\? 'bg-muted text-muted-foreground'\s*: 'bg-primary text-primary-foreground'/,
  )
})
