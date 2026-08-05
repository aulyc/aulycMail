import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  buildFolderNavigationList,
  nextSidebarAction,
  nextSidebarNavigationIndex,
} from '../src/lib/components/sidebar/folderNavigation.ts'

const accounts = [
  {
    account: { id: 'account-a', name: 'Account A' },
    folders: [
      {
        folder: {
          id: 'trash', name: 'Deleted Messages', path: 'Deleted Messages',
          type: 'trash', noSelect: false,
        },
      },
      {
        folder: {
          id: 'other-folders', name: 'Other folders', path: 'Other folders',
          type: 'folder', noSelect: true,
        },
        children: [
          {
            folder: {
              id: 'receipts', name: 'Receipts', path: 'Other folders/Receipts',
              type: 'folder', noSelect: false,
            },
          },
        ],
      },
    ],
  },
  { account: { id: 'account-b', name: 'Account B' }, folders: [] },
]

test('the compose and sync group leads the vertical sidebar order as one item', () => {
  const items = buildFolderNavigationList(accounts, { 'account-a': true, 'account-b': true }, {})
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

test('directory groups stay ordered and reveal children only when expanded', () => {
  const collapsed = buildFolderNavigationList(accounts, { 'account-a': true, 'account-b': true }, {})
  const collapsedIndex = collapsed.findIndex((item) => item.folderId === 'other-folders')
  assert.equal(collapsed[collapsedIndex]?.type, 'folder-group')
  assert.equal(collapsed.some((item) => item.folderId === 'receipts'), false)

  const expanded = buildFolderNavigationList(
    accounts,
    { 'account-a': true, 'account-b': true },
    { 'other-folders': false },
  )
  const expandedIndex = expanded.findIndex((item) => item.folderId === 'other-folders')
  assert.equal(expanded[expandedIndex + 1]?.type, 'folder')
  assert.equal(expanded[expandedIndex + 1]?.folderId, 'receipts')
})
