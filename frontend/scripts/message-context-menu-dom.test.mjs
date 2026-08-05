// @vitest-environment happy-dom

import assert from 'node:assert/strict'
import { mount, tick, unmount } from 'svelte'
import { afterEach, beforeEach, test, vi } from 'vitest'

const backend = vi.hoisted(() => ({ GetAccounts: vi.fn() }))
const dialogGuard = vi.hoisted(() => ({ open: vi.fn(), close: vi.fn() }))
const mailActions = vi.hoisted(() => ({
  archiveMessages: vi.fn(),
  copyMessagesToFolder: vi.fn(),
  deleteMessagesPermanently: vi.fn(),
  moveMessagesToFolder: vi.fn(),
  setReadStateMessages: vi.fn(),
  toggleSpamMessages: vi.fn(),
  toggleStarMessages: vi.fn(),
  trashMessages: vi.fn(),
  undoLastMailAction: vi.fn(),
}))

vi.mock('../wailsjs/go/app/App.js', () => backend)
vi.mock('../wailsjs/go/models.js', () => ({ account: { Account: class Account {} } }))
vi.mock('@iconify/svelte', async () => ({ default: (await import('./fixtures/StaticStub.svelte')).default }))
vi.mock('bits-ui', async () => ({
  ContextMenu: {
    Root: (await import('./fixtures/ContextMenuRootTestStub.svelte')).default,
    Trigger: (await import('./fixtures/SnippetTestStub.svelte')).default,
  },
}))
vi.mock('$lib/components/ui/context-menu', async () => ({
  ContextMenuContent: (await import('./fixtures/SnippetTestStub.svelte')).default,
  ContextMenuItem: (await import('./fixtures/DropdownItemTestStub.svelte')).default,
  ContextMenuSeparator: (await import('./fixtures/StaticStub.svelte')).default,
}))
vi.mock('$lib/components/ui/confirm-dialog', async () => ({
  ConfirmDialog: (await import('./fixtures/ConfirmDialogTestStub.svelte')).default,
}))
vi.mock('../src/lib/components/common/FolderPickerDialog.svelte', async () => ({
  default: (await import('./fixtures/FolderPickerDialogTestStub.svelte')).default,
}))
vi.mock('$lib/i18n', () => ({
  _: {
    subscribe(run) {
      run((key) => key)
      return () => {}
    },
  },
}))
vi.mock('$lib/stores/dialogGuard', () => ({
  dialogGuardOpen: dialogGuard.open,
  dialogGuardClose: dialogGuard.close,
}))
vi.mock('$lib/mailActions', () => mailActions)

import MessageContextMenuHarness from './fixtures/MessageContextMenuHarness.svelte'

const mounted = []

async function flushAsync() {
  await Promise.resolve()
  await tick()
  await Promise.resolve()
  await tick()
}

async function renderMenu(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const instance = mount(MessageContextMenuHarness, {
    target,
    props: {
      messageIds: ['message-1'],
      ...props,
    },
  })
  mounted.push(instance)
  await flushAsync()
  return target
}

function button(root, text) {
  const found = [...root.querySelectorAll('button')].find((item) => item.textContent.includes(text))
  assert.ok(found, `missing button containing ${text}`)
  return found
}

async function click(element) {
  element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await flushAsync()
}

beforeEach(() => {
  document.body.innerHTML = ''
  backend.GetAccounts.mockReset().mockResolvedValue([{ id: 'account-1' }])
  dialogGuard.open.mockReset()
  dialogGuard.close.mockReset()
  for (const action of Object.values(mailActions)) action.mockReset().mockResolvedValue(undefined)
})

afterEach(async () => {
  while (mounted.length > 0) await unmount(mounted.pop())
})

test('loads accounts once, reports open changes, and routes single-message actions', async () => {
  const replies = vi.fn()
  const completed = vi.fn()
  const openChanges = vi.fn()
  const target = await renderMenu({ onReply: replies, onActionComplete: completed, onOpenChange: openChanges })

  assert.ok(target.querySelector('[data-context-trigger]'))
  await click(target.querySelector('[data-context-menu-open]'))
  await click(target.querySelector('[data-context-menu-open]'))
  await click(target.querySelector('[data-context-menu-close]'))
  assert.equal(backend.GetAccounts.mock.calls.length, 1)
  assert.deepEqual(openChanges.mock.calls, [[true], [true], [false]])

  await click(button(target, 'contextMenu.reply'))
  await click(button(target, 'contextMenu.replyAll'))
  await click(button(target, 'contextMenu.forward'))
  assert.deepEqual(replies.mock.calls, [
    ['reply', 'message-1'],
    ['reply-all', 'message-1'],
    ['forward', 'message-1'],
  ])

  await click(button(target, 'contextMenu.archive'))
  const archiveOptions = mailActions.archiveMessages.mock.calls[0][1]
  assert.deepEqual(mailActions.archiveMessages.mock.calls[0][0], ['message-1'])
  assert.equal(archiveOptions.autoSelectNext, true)
  await archiveOptions.onUndo()
  await archiveOptions.onSuccess(true)
  assert.equal(mailActions.undoLastMailAction.mock.calls.length, 1)
  assert.deepEqual(completed.mock.calls, [[true]])

  await click(button(target, 'contextMenu.delete'))
  await click(button(target, 'contextMenu.markAsSpam'))
  await click(button(target, 'contextMenu.star'))
  await click(button(target, 'contextMenu.markAsRead'))
  assert.deepEqual(mailActions.trashMessages.mock.calls[0][0], ['message-1'])
  assert.deepEqual(mailActions.toggleSpamMessages.mock.calls[0].slice(0, 2), [['message-1'], false])
  assert.deepEqual(mailActions.toggleStarMessages.mock.calls[0].slice(0, 2), [['message-1'], true])
  assert.deepEqual(mailActions.setReadStateMessages.mock.calls[0].slice(0, 2), [['message-1'], true])
})

test('handles trash confirmation success and error, spam reversal, and active flags', async () => {
  const completed = vi.fn()
  const target = await renderMenu({ folderType: 'trash', isStarred: true, isRead: true, onActionComplete: completed })

  await click(button(target, 'contextMenu.deletePermanently'))
  assert.equal(dialogGuard.open.mock.calls.length, 1)
  await click(target.querySelector('[data-confirm-action="confirm"]'))
  const firstOptions = mailActions.deleteMessagesPermanently.mock.calls[0][1]
  await firstOptions.onSuccess(false)
  await flushAsync()
  assert.deepEqual(completed.mock.calls, [[false]])

  await click(button(target, 'contextMenu.deletePermanently'))
  await click(target.querySelector('[data-confirm-action="confirm"]'))
  const secondOptions = mailActions.deleteMessagesPermanently.mock.calls[1][1]
  secondOptions.onError()
  await flushAsync()
  assert.equal(target.querySelector('[data-confirm-dialog]'), null)

  await click(button(target, 'contextMenu.removeStar'))
  await click(button(target, 'contextMenu.markAsUnread'))
  assert.equal(mailActions.toggleStarMessages.mock.calls[0][1], false)
  assert.equal(mailActions.setReadStateMessages.mock.calls[0][1], false)

  const spamTarget = await renderMenu({ folderType: 'spam' })
  await click(button(spamTarget, 'contextMenu.markAsNotSpam'))
  assert.equal(mailActions.toggleSpamMessages.mock.calls.at(-1)[1], true)
})

test('moves and copies through the picker and tolerates missing optional callbacks', async () => {
  const target = await renderMenu({ messageIds: ['message-1', 'message-2'] })
  assert.equal([...target.querySelectorAll('button')].some((item) => item.textContent === 'contextMenu.reply'), false)

  await click(button(target, 'contextMenu.moveTo'))
  let picker = target.querySelector('[data-folder-picker]')
  assert.ok(picker)
  assert.equal(picker.dataset.title, 'contextMenu.moveTo')
  assert.equal(picker.dataset.accountCount, '0')
  assert.equal(dialogGuard.open.mock.calls.length, 1)
  await click(picker.querySelector('[data-folder-picker-select]'))
  assert.deepEqual(mailActions.moveMessagesToFolder.mock.calls[0].slice(0, 3), [
    ['message-1', 'message-2'], 'destination-folder', 'Destination',
  ])

  await click(button(target, 'contextMenu.copyTo'))
  picker = target.querySelector('[data-folder-picker]')
  assert.equal(picker.dataset.title, 'contextMenu.copyTo')
  await click(picker.querySelector('[data-folder-picker-select]'))
  assert.deepEqual(mailActions.copyMessagesToFolder.mock.calls[0], [
    ['message-1', 'message-2'], 'destination-folder', 'Destination',
  ])

  await click(button(target, 'contextMenu.archive'))
  await mailActions.archiveMessages.mock.calls[0][1].onSuccess?.(true)
  assert.equal(dialogGuard.close.mock.calls.length >= 1, true)
})

test('surfaces account load failure without blocking actions', async () => {
  const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
  backend.GetAccounts.mockRejectedValue(new Error('synthetic account failure'))
  const target = await renderMenu()
  await click(target.querySelector('[data-context-menu-open]'))
  assert.equal(consoleError.mock.calls.length, 1)
  await click(button(target, 'contextMenu.archive'))
  assert.equal(mailActions.archiveMessages.mock.calls.length, 1)
  consoleError.mockRestore()
})
