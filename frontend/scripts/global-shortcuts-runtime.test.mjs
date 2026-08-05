import assert from 'node:assert/strict'
import { beforeEach, test, vi } from 'vitest'

const keyboard = vi.hoisted(() => ({
  focusNext: vi.fn(),
  focusPrevious: vi.fn(),
  focusCurrent: vi.fn(),
  focusedPane: vi.fn(() => 'messageList'),
  paneNav: vi.fn(),
  input: vi.fn(() => false),
}))
const guard = vi.hoisted(() => ({ active: vi.fn(() => false) }))
const pane = vi.hoisted(() => ({ dispatch: vi.fn(() => false) }))
const ui = vi.hoisted(() => ({ active: vi.fn(() => 'mail'), set: vi.fn() }))
const layout = vi.hoisted(() => ({
  mode: vi.fn(() => 'full'),
  view: vi.fn(() => 'default'),
  responsive: vi.fn(() => false),
  hideSidebar: vi.fn(),
  hideViewer: vi.fn(),
  showSidebar: vi.fn(),
  showViewer: vi.fn(),
}))
const settings = vi.hoisted(() => ({ enhanced: vi.fn(() => true) }))

vi.mock('$lib/stores/keyboard.svelte', () => ({
  focusNextPane: keyboard.focusNext,
  focusPreviousPane: keyboard.focusPrevious,
  focusCurrentPane: keyboard.focusCurrent,
  getFocusedPane: keyboard.focusedPane,
  getPaneNav: keyboard.paneNav,
  isInputElement: keyboard.input,
}))
vi.mock('$lib/stores/dialogGuard', () => ({ isDialogGuardActive: guard.active }))
vi.mock('$lib/stores/paneShortcuts.svelte', () => ({ dispatchPaneShortcut: pane.dispatch }))
vi.mock('$lib/stores/uiState.svelte', () => ({ getActivePane: ui.active, setActivePane: ui.set }))
vi.mock('$lib/stores/layout.svelte', () => ({
  getLayoutMode: layout.mode,
  getResponsiveView: layout.view,
  hideSidebar: layout.hideSidebar,
  hideViewer: layout.hideViewer,
  isResponsive: layout.responsive,
  showSidebar: layout.showSidebar,
  showViewer: layout.showViewer,
}))
vi.mock('$lib/stores/settings.svelte', () => ({
  getEnhancedKeyboardNavigation: settings.enhanced,
}))

import { handleGlobalShortcut } from '../src/lib/keyboard/globalShortcuts.ts'

function event(key, overrides = {}) {
  return {
    key,
    code: overrides.code ?? '',
    keyCode: 0,
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    defaultPrevented: false,
    isComposing: false,
    target: null,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
    ...overrides,
  }
}

function context(overrides = {}) {
  const sidebarRef = {
    toggleSync: vi.fn(),
    selectPreviousFolder: vi.fn(),
    selectNextFolder: vi.fn(),
    hasFocusedSidebarAction: vi.fn(() => false),
    hasFocusedAccount: vi.fn(() => false),
    hasFocusedFolderGroup: vi.fn(() => false),
    hasSelectedFolderWithChildren: vi.fn(() => false),
    moveFocusedSidebarAction: vi.fn(),
    activateFocusedSidebarAction: vi.fn(),
    toggleFocusedAccount: vi.fn(),
    toggleFocusedFolderGroup: vi.fn(),
    toggleSelectedFolderCollapse: vi.fn(),
  }
  const messageListRef = {
    toggleFolderSync: vi.fn(),
    selectAll: vi.fn(),
    hasCheckedMessages: vi.fn(() => false),
    getCheckedMessageIds: vi.fn(() => ['checked']),
    getSelectedMessageIds: vi.fn(() => ['selected']),
    getCheckedHasUnstarred: vi.fn(() => true),
    isSelectedStarred: vi.fn(() => false),
    openContextMenu: vi.fn(),
    clearChecked: vi.fn(),
    selectPrevious: vi.fn(),
    selectPreviousWithCheck: vi.fn(),
    selectNext: vi.fn(),
    selectNextWithCheck: vi.fn(),
    openSelected: vi.fn(),
    toggleCheck: vi.fn(),
    requestDelete: vi.fn(),
  }
  const viewerRef = {
    hasFocusedMessage: vi.fn(() => false),
    replyAll: vi.fn(),
    reply: vi.fn(),
    isImagesLoaded: vi.fn(() => true),
    selectAllText: vi.fn(),
    openAlwaysLoadDropdown: vi.fn(),
    loadImages: vi.fn(),
    openContextMenu: vi.fn(),
    scrollUp: vi.fn(),
    scrollDown: vi.fn(),
    getFocusedMessageId: vi.fn(() => 'focused-message'),
    deletePermanently: vi.fn(),
    trash: vi.fn(),
  }
  return {
    leftAltHeld: false,
    showSearchOverlay: false,
    showComposer: false,
    selectedThreadId: 'thread',
    selectedAccountId: 'account',
    selectedFolderId: 'folder',
    focusMode: 'off',
    sidebarRef,
    messageListRef,
    viewerRef,
    setSearchOverlay: vi.fn(),
    setFocusMode: vi.fn(),
    setFocusedMessageIdInFocus: vi.fn(),
    focusContextMenu: vi.fn(),
    openRegionActionMenu: vi.fn(),
    handleQuit: vi.fn(),
    handleCompose: vi.fn(),
    handleReply: vi.fn(),
    getLastMessageId: vi.fn(() => 'last-message'),
    handleBulkMarkRead: vi.fn(),
    handleBulkMarkUnread: vi.fn(),
    handleBulkArchive: vi.fn(),
    handleBulkSpam: vi.fn(),
    handleBulkToggleStar: vi.fn(),
    toggleThreadFocus: vi.fn(),
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  keyboard.focusedPane.mockReturnValue('messageList')
  keyboard.input.mockReturnValue(false)
  guard.active.mockReturnValue(false)
  pane.dispatch.mockReturnValue(false)
  ui.active.mockReturnValue('mail')
  layout.mode.mockReturnValue('full')
  layout.view.mockReturnValue('default')
  layout.responsive.mockReturnValue(false)
  settings.enhanced.mockReturnValue(true)
  vi.stubGlobal('document', { querySelector: vi.fn(() => null), activeElement: null })
  vi.stubGlobal('MouseEvent', class MouseEvent {
    constructor(type, init = {}) {
      this.type = type
      Object.assign(this, init)
    }
  })
  vi.stubGlobal('requestAnimationFrame', (callback) => callback())
})

test('global guards leave handled, composing, overlay, menu, and dialog events untouched', () => {
  const ctx = context()
  handleGlobalShortcut(event('x', { defaultPrevented: true }), ctx)
  handleGlobalShortcut(event('x', { isComposing: true }), ctx)
  handleGlobalShortcut(event('x', { keyCode: 229 }), ctx)
  handleGlobalShortcut(event('x'), context({ showSearchOverlay: true }))
  document.querySelector.mockReturnValueOnce({})
  handleGlobalShortcut(event('x'), ctx)
  guard.active.mockReturnValueOnce(true)
  handleGlobalShortcut(event('x'), ctx)
  assert.equal(ctx.setSearchOverlay.mock.calls.length, 0)
})

test('search remains available without enhanced navigation while native and composer keys are preserved', () => {
  settings.enhanced.mockReturnValue(false)
  const ctx = context()
  const search = event('f', { metaKey: true })
  handleGlobalShortcut(search, ctx)
  assert.deepEqual(ctx.setSearchOverlay.mock.calls[0], [true])
  assert.equal(search.preventDefault.mock.calls.length, 1)

  const native = event('n', { metaKey: true })
  handleGlobalShortcut(native, ctx)
  assert.equal(native.preventDefault.mock.calls.length, 0)

  settings.enhanced.mockReturnValue(true)
  const reload = event('r', { metaKey: true })
  handleGlobalShortcut(reload, context({ showComposer: true }))
  assert.equal(reload.preventDefault.mock.calls.length, 1)
})

test('pane dispatch, Tab cycling, slash search, quitting, rail switching, composing, and replying work', () => {
  const ctx = context()
  pane.dispatch.mockReturnValueOnce(true)
  const local = event('x')
  handleGlobalShortcut(local, ctx)
  assert.equal(local.stopPropagation.mock.calls.length, 1)

  handleGlobalShortcut(event('Tab'), ctx)
  handleGlobalShortcut(event('Tab', { shiftKey: true }), ctx)
  assert.equal(keyboard.focusNext.mock.calls.length, 1)
  assert.equal(keyboard.focusPrevious.mock.calls.length, 1)
  handleGlobalShortcut(event('/'), ctx)
  assert.equal(ctx.setSearchOverlay.mock.calls.length, 1)

  handleGlobalShortcut(event('q', { metaKey: true }), ctx)
  assert.equal(ctx.handleQuit.mock.calls.length, 1)
  handleGlobalShortcut(event('Tab', { metaKey: true }), ctx)
  handleGlobalShortcut(event('`', { metaKey: true }), ctx)
  assert.deepEqual(ui.set.mock.calls.map((call) => call[0]), ['contacts', 'contacts'])
  handleGlobalShortcut(event('n', { metaKey: true }), ctx)
  assert.equal(ctx.handleCompose.mock.calls.length, 1)

  handleGlobalShortcut(event('r', { metaKey: true }), ctx)
  assert.deepEqual(ctx.handleReply.mock.calls[0], ['reply', 'last-message', true])
  keyboard.focusedPane.mockReturnValue('viewer')
  ctx.viewerRef.hasFocusedMessage.mockReturnValue(true)
  handleGlobalShortcut(event('r', { metaKey: true, shiftKey: true }), ctx)
  assert.equal(ctx.viewerRef.replyAll.mock.calls.length, 1)
})

test('mail command shortcuts sync, select, load images, and operate on checked or selected messages', () => {
  const ctx = context()
  handleGlobalShortcut(event('s', { metaKey: true, shiftKey: true }), ctx)
  handleGlobalShortcut(event('a', { metaKey: true, shiftKey: true }), ctx)
  assert.equal(ctx.messageListRef.toggleFolderSync.mock.calls.length, 1)
  assert.equal(ctx.sidebarRef.toggleSync.mock.calls.length, 1)

  handleGlobalShortcut(event('a', { metaKey: true }), ctx)
  assert.equal(ctx.messageListRef.selectAll.mock.calls.length, 1)
  keyboard.focusedPane.mockReturnValue('viewer')
  handleGlobalShortcut(event('a', { metaKey: true }), ctx)
  assert.equal(ctx.viewerRef.selectAllText.mock.calls.length, 1)
  handleGlobalShortcut(event('l', { metaKey: true }), ctx)
  handleGlobalShortcut(event('l', { metaKey: true, shiftKey: true }), ctx)
  assert.equal(ctx.viewerRef.loadImages.mock.calls.length, 1)
  assert.equal(ctx.viewerRef.openAlwaysLoadDropdown.mock.calls.length, 1)

  keyboard.focusedPane.mockReturnValue('messageList')
  ctx.messageListRef.hasCheckedMessages.mockReturnValue(true)
  handleGlobalShortcut(event('u', { metaKey: true }), ctx)
  handleGlobalShortcut(event('u', { metaKey: true, shiftKey: true }), ctx)
  handleGlobalShortcut(event('k', { metaKey: true }), ctx)
  handleGlobalShortcut(event('j', { metaKey: true }), ctx)
  assert.deepEqual(ctx.handleBulkMarkRead.mock.calls[0], [['checked']])
  assert.deepEqual(ctx.handleBulkMarkUnread.mock.calls[0], [['checked']])
  assert.deepEqual(ctx.handleBulkArchive.mock.calls[0], [['checked']])
  assert.deepEqual(ctx.handleBulkSpam.mock.calls[0], [['checked']])
})

test('context menus and responsive Alt navigation select the correct region behavior', () => {
  const ctx = context()
  keyboard.focusedPane.mockReturnValue('messageList')
  handleGlobalShortcut(event('ContextMenu'), ctx)
  assert.equal(ctx.messageListRef.openContextMenu.mock.calls.length, 1)
  assert.equal(ctx.focusContextMenu.mock.calls.length, 1)
  keyboard.focusedPane.mockReturnValue('viewer')
  handleGlobalShortcut(event('Alt', { code: 'AltRight' }), ctx)
  assert.equal(ctx.viewerRef.openContextMenu.mock.calls.length, 1)

  layout.responsive.mockReturnValue(true)
  layout.view.mockReturnValue('viewer')
  handleGlobalShortcut(event('h', { altKey: true }), ctx)
  assert.equal(layout.hideViewer.mock.calls.length, 1)
  layout.view.mockReturnValue('default')
  layout.mode.mockReturnValue('narrow')
  handleGlobalShortcut(event('h', { altKey: true }), ctx)
  assert.equal(layout.showSidebar.mock.calls.length, 1)
  layout.view.mockReturnValue('sidebar')
  handleGlobalShortcut(event('l', { altKey: true }), ctx)
  assert.equal(layout.hideSidebar.mock.calls.length, 1)
  layout.view.mockReturnValue('default')
  handleGlobalShortcut(event('l', { altKey: true }), ctx)
  assert.equal(layout.showViewer.mock.calls.length, 1)
})

test('Alt sidebar navigation delegates by active pane and toggles focused groups', () => {
  const ctx = context()
  keyboard.focusedPane.mockReturnValue('sidebar')
  handleGlobalShortcut(event('k', { altKey: true }), ctx)
  handleGlobalShortcut(event('j', { altKey: true }), ctx)
  assert.equal(ctx.sidebarRef.selectPreviousFolder.mock.calls.length, 1)
  assert.equal(ctx.sidebarRef.selectNextFolder.mock.calls.length, 1)

  const navigatePrev = vi.fn()
  const navigateNext = vi.fn()
  ui.active.mockReturnValue('contacts')
  keyboard.paneNav.mockReturnValue({ navigatePrev, navigateNext })
  handleGlobalShortcut(event('k', { altKey: true }), ctx)
  handleGlobalShortcut(event('j', { altKey: true }), ctx)
  assert.equal(navigatePrev.mock.calls.length, 1)
  assert.equal(navigateNext.mock.calls.length, 1)

  ui.active.mockReturnValue('mail')
  ctx.sidebarRef.hasFocusedAccount.mockReturnValue(true)
  handleGlobalShortcut(event('Enter', { altKey: true }), ctx)
  assert.equal(ctx.sidebarRef.toggleFocusedAccount.mock.calls.length, 1)
})

test('input Escape, region menu, responsive Escape, and checked-message Escape are isolated', () => {
  const blur = vi.fn()
  keyboard.input.mockReturnValueOnce(true)
  const inputEscape = event('Escape', { target: { blur } })
  handleGlobalShortcut(inputEscape, context())
  assert.equal(blur.mock.calls.length, 1)
  assert.equal(keyboard.focusCurrent.mock.calls.length, 1)

  const ctx = context()
  handleGlobalShortcut(event('F10', { shiftKey: true }), ctx)
  assert.equal(ctx.openRegionActionMenu.mock.calls.length, 1)
  handleGlobalShortcut(event('Escape'), context({ focusMode: 'message', setFocusMode: ctx.setFocusMode, setFocusedMessageIdInFocus: ctx.setFocusedMessageIdInFocus }))
  assert.deepEqual(ctx.setFocusMode.mock.calls[0], ['off'])

  layout.responsive.mockReturnValue(true)
  layout.view.mockReturnValue('viewer')
  handleGlobalShortcut(event('Escape'), ctx)
  assert.equal(layout.hideViewer.mock.calls.length > 0, true)
  layout.responsive.mockReturnValue(false)
  ctx.messageListRef.hasCheckedMessages.mockReturnValue(true)
  handleGlobalShortcut(event('Escape'), ctx)
  assert.equal(ctx.messageListRef.clearChecked.mock.calls.length, 1)
})

test('mail navigation, activation, starring, focus modes, and deletion route to focused controls', () => {
  const ctx = context()
  keyboard.focusedPane.mockReturnValue('sidebar')
  ctx.sidebarRef.hasFocusedSidebarAction.mockReturnValue(true)
  handleGlobalShortcut(event('ArrowLeft'), ctx)
  handleGlobalShortcut(event('ArrowRight'), ctx)
  assert.deepEqual(ctx.sidebarRef.moveFocusedSidebarAction.mock.calls.map((call) => call[0]), [-1, 1])
  handleGlobalShortcut(event('Enter'), ctx)
  assert.equal(ctx.sidebarRef.activateFocusedSidebarAction.mock.calls.length, 1)

  keyboard.focusedPane.mockReturnValue('messageList')
  handleGlobalShortcut(event('k'), ctx)
  handleGlobalShortcut(event('J', { shiftKey: true }), ctx)
  handleGlobalShortcut(event('Enter'), ctx)
  handleGlobalShortcut(event(' '), ctx)
  handleGlobalShortcut(event('v'), ctx)
  assert.equal(ctx.messageListRef.selectPrevious.mock.calls.length, 1)
  assert.equal(ctx.messageListRef.selectNextWithCheck.mock.calls.length, 1)
  assert.equal(ctx.messageListRef.openSelected.mock.calls.length, 2)
  assert.equal(ctx.messageListRef.toggleCheck.mock.calls.length, 1)

  handleGlobalShortcut(event('s'), ctx)
  assert.deepEqual(ctx.handleBulkToggleStar.mock.calls[0], [['selected'], true])
  handleGlobalShortcut(event('f'), ctx)
  assert.equal(ctx.toggleThreadFocus.mock.calls.length, 1)
  handleGlobalShortcut(event('F'), ctx)
  assert.deepEqual(ctx.setFocusMode.mock.calls.at(-1), ['message'])
  assert.deepEqual(ctx.setFocusedMessageIdInFocus.mock.calls.at(-1), ['last-message'])

  handleGlobalShortcut(event('Delete'), ctx)
  assert.deepEqual(ctx.messageListRef.requestDelete.mock.calls[0], [['selected'], false])
  keyboard.focusedPane.mockReturnValue('viewer')
  ctx.viewerRef.hasFocusedMessage.mockReturnValue(true)
  handleGlobalShortcut(event('Backspace'), ctx)
  handleGlobalShortcut(event('Delete', { shiftKey: true }), ctx)
  assert.equal(ctx.viewerRef.trash.mock.calls.length, 1)
  assert.equal(ctx.viewerRef.deletePermanently.mock.calls.length, 1)
})

test('command shortcuts cover missing selections, native input selection, and inactive mail panes', () => {
  const ctx = context({ selectedThreadId: null })
  const noConversationReply = event('r', { metaKey: true })
  handleGlobalShortcut(noConversationReply, ctx)
  assert.equal(noConversationReply.preventDefault.mock.calls.length, 0)

  ctx.selectedThreadId = 'thread'
  ctx.getLastMessageId.mockReturnValueOnce(null)
  handleGlobalShortcut(event('r', { metaKey: true }), ctx)
  assert.equal(ctx.handleReply.mock.calls.length, 0)
  keyboard.focusedPane.mockReturnValue('viewer')
  ctx.viewerRef.hasFocusedMessage.mockReturnValue(true)
  handleGlobalShortcut(event('r', { metaKey: true }), ctx)
  assert.equal(ctx.viewerRef.reply.mock.calls.length, 1)

  handleGlobalShortcut(event('s', { metaKey: true }), ctx)
  assert.equal(ctx.messageListRef.toggleFolderSync.mock.calls.length, 0)
  keyboard.input.mockReturnValue(true)
  const select = vi.fn()
  const inputSelect = event('a', { metaKey: true, target: { select } })
  handleGlobalShortcut(inputSelect, ctx)
  assert.equal(select.mock.calls.length, 1)

  keyboard.input.mockReturnValue(false)
  keyboard.focusedPane.mockReturnValue('messageList')
  ctx.messageListRef.getSelectedMessageIds.mockReturnValue([])
  for (const key of ['u', 'k', 'j']) {
    handleGlobalShortcut(event(key, { metaKey: true }), ctx)
  }
  assert.equal(ctx.handleBulkMarkRead.mock.calls.length, 0)
  assert.equal(ctx.handleBulkArchive.mock.calls.length, 0)
  assert.equal(ctx.handleBulkSpam.mock.calls.length, 0)

  ui.active.mockReturnValue('contacts')
  handleGlobalShortcut(event('n', { metaKey: true }), ctx)
  assert.equal(ctx.handleCompose.mock.calls.length, 0)
  ui.active.mockReturnValue('not-a-pane')
  handleGlobalShortcut(event('Tab', { metaKey: true }), ctx)
  assert.equal(ui.set.mock.calls.length, 1)

  handleGlobalShortcut(event('x'), context({ showComposer: true }))
})

test('sidebar context menu guards and activation dispatch use the selected folder element', () => {
  keyboard.focusedPane.mockReturnValue('sidebar')
  const actionFocused = context()
  actionFocused.sidebarRef.hasFocusedSidebarAction.mockReturnValue(true)
  handleGlobalShortcut(event('ContextMenu'), actionFocused)
  assert.equal(actionFocused.focusContextMenu.mock.calls.length, 0)

  const noSelection = context({ selectedFolderId: null })
  handleGlobalShortcut(event('ContextMenu'), noSelection)
  assert.equal(noSelection.focusContextMenu.mock.calls.length, 0)

  handleGlobalShortcut(event('ContextMenu'), context())
  const dispatched = []
  const folderElement = {
    getBoundingClientRect: () => ({ right: 80, top: 20, height: 24 }),
    dispatchEvent: (value) => dispatched.push(value),
  }
  document.querySelector.mockImplementation((selector) => selector === '[role="menu"]' ? null : folderElement)
  const selected = context()
  handleGlobalShortcut(event('ContextMenu'), selected)
  assert.equal(dispatched.length, 1)
  assert.equal(selected.focusContextMenu.mock.calls.length, 1)

  keyboard.focusedPane.mockReturnValue('viewer')
  const leftAlt = context({ leftAltHeld: true })
  handleGlobalShortcut(event('Alt', { code: 'AltRight' }), leftAlt)
  assert.equal(dispatched.length, 2)
  const withoutViewer = context({ viewerRef: null })
  handleGlobalShortcut(event('ContextMenu'), withoutViewer)
  assert.equal(withoutViewer.focusContextMenu.mock.calls.length, 1)
})

test('Alt navigation covers desktop fallbacks, missing conversations, and folder hierarchy toggles', () => {
  const ctx = context({ focusMode: 'thread' })
  handleGlobalShortcut(event('h', { altKey: true }), ctx)
  assert.equal(keyboard.focusPrevious.mock.calls.length, 0)

  ctx.focusMode = 'off'
  handleGlobalShortcut(event('h', { altKey: true }), ctx)
  handleGlobalShortcut(event('l', { altKey: true }), ctx)
  assert.equal(keyboard.focusPrevious.mock.calls.length, 1)
  assert.equal(keyboard.focusNext.mock.calls.length, 1)

  layout.responsive.mockReturnValue(true)
  layout.mode.mockReturnValue('compact')
  layout.view.mockReturnValue('default')
  const noConversation = context({ selectedThreadId: null })
  handleGlobalShortcut(event('h', { altKey: true }), noConversation)
  handleGlobalShortcut(event('l', { altKey: true }), noConversation)
  assert.equal(keyboard.focusPrevious.mock.calls.length, 2)
  assert.equal(keyboard.focusNext.mock.calls.length, 2)

  layout.responsive.mockReturnValue(false)
  keyboard.focusedPane.mockReturnValue('sidebar')
  const group = context()
  group.sidebarRef.hasFocusedFolderGroup.mockReturnValue(true)
  handleGlobalShortcut(event('Enter', { altKey: true }), group)
  assert.equal(group.sidebarRef.toggleFocusedFolderGroup.mock.calls.length, 1)
  const children = context()
  children.sidebarRef.hasSelectedFolderWithChildren.mockReturnValue(true)
  handleGlobalShortcut(event('Enter', { altKey: true }), children)
  assert.equal(children.sidebarRef.toggleSelectedFolderCollapse.mock.calls.length, 1)

  ui.active.mockReturnValue('contacts')
  keyboard.paneNav.mockReturnValue(null)
  handleGlobalShortcut(event('k', { altKey: true }), context())
  handleGlobalShortcut(event('j', { altKey: true }), context())
})

test('list navigation and activation cover every focused pane and native button exemption', () => {
  const ctx = context()
  keyboard.focusedPane.mockReturnValue('viewer')
  handleGlobalShortcut(event('k'), ctx)
  handleGlobalShortcut(event('j'), ctx)
  assert.equal(ctx.viewerRef.scrollUp.mock.calls.length, 1)
  assert.equal(ctx.viewerRef.scrollDown.mock.calls.length, 1)

  keyboard.focusedPane.mockReturnValue('sidebar')
  handleGlobalShortcut(event('j'), ctx)
  assert.equal(ctx.sidebarRef.selectNextFolder.mock.calls.length, 1)
  keyboard.focusedPane.mockReturnValue('messageList')
  handleGlobalShortcut(event('K', { shiftKey: true }), ctx)
  handleGlobalShortcut(event('j'), ctx)
  assert.equal(ctx.messageListRef.selectPreviousWithCheck.mock.calls.length, 1)
  assert.equal(ctx.messageListRef.selectNext.mock.calls.length, 1)

  keyboard.input.mockReturnValue(true)
  handleGlobalShortcut(event('x', { target: { blur: vi.fn() } }), ctx)
  keyboard.input.mockReturnValue(false)
  layout.responsive.mockReturnValue(true)
  layout.view.mockReturnValue('sidebar')
  handleGlobalShortcut(event('Escape'), ctx)
  assert.equal(layout.hideSidebar.mock.calls.length, 1)
  layout.responsive.mockReturnValue(false)
  ui.active.mockReturnValue('contacts')
  handleGlobalShortcut(event('x'), ctx)

  ui.active.mockReturnValue('mail')
  const nativeButton = { tagName: 'BUTTON', closest: () => ({}) }
  document.activeElement = nativeButton
  handleGlobalShortcut(event('Enter'), ctx)
  handleGlobalShortcut(event(' '), ctx)
  assert.equal(ctx.messageListRef.openSelected.mock.calls.length, 0)
  const unrelatedButton = { tagName: 'BUTTON', closest: () => null }
  document.activeElement = unrelatedButton
  handleGlobalShortcut(event('Enter'), ctx)
  assert.equal(ctx.messageListRef.openSelected.mock.calls.length, 1)

  document.activeElement = null
  keyboard.focusedPane.mockReturnValue('sidebar')
  for (const [predicate, action] of [
    ['hasFocusedAccount', 'toggleFocusedAccount'],
    ['hasFocusedFolderGroup', 'toggleFocusedFolderGroup'],
    ['hasSelectedFolderWithChildren', 'toggleSelectedFolderCollapse'],
  ]) {
    const local = context()
    local.sidebarRef[predicate].mockReturnValue(true)
    handleGlobalShortcut(event('Enter'), local)
    handleGlobalShortcut(event(' '), local)
    assert.equal(local.sidebarRef[action].mock.calls.length, 2)
  }
})

test('star, focus, and delete shortcuts cover checked, empty, and focused-message alternatives', () => {
  const checked = context()
  checked.messageListRef.hasCheckedMessages.mockReturnValue(true)
  handleGlobalShortcut(event('s'), checked)
  assert.deepEqual(checked.handleBulkToggleStar.mock.calls[0], [['checked'], true])

  const empty = context({ selectedThreadId: null })
  empty.messageListRef.getSelectedMessageIds.mockReturnValue([])
  handleGlobalShortcut(event('s'), empty)
  handleGlobalShortcut(event('f'), empty)
  handleGlobalShortcut(event('F'), empty)
  handleGlobalShortcut(event('d'), empty)
  assert.equal(empty.handleBulkToggleStar.mock.calls.length, 0)
  assert.equal(empty.toggleThreadFocus.mock.calls.length, 0)
  assert.equal(empty.messageListRef.requestDelete.mock.calls.length, 0)

  const starred = context()
  starred.messageListRef.isSelectedStarred.mockReturnValue(true)
  handleGlobalShortcut(event('s'), starred)
  assert.deepEqual(starred.handleBulkToggleStar.mock.calls[0], [['selected'], false])

  const focusedMode = context({ focusMode: 'message' })
  handleGlobalShortcut(event('F'), focusedMode)
  assert.deepEqual(focusedMode.setFocusMode.mock.calls[0], ['off'])
  assert.deepEqual(focusedMode.setFocusedMessageIdInFocus.mock.calls[0], [null])

  keyboard.focusedPane.mockReturnValue('viewer')
  const viewerFocus = context()
  viewerFocus.viewerRef.hasFocusedMessage.mockReturnValue(true)
  handleGlobalShortcut(event('F'), viewerFocus)
  assert.deepEqual(viewerFocus.setFocusedMessageIdInFocus.mock.calls[0], ['focused-message'])

  keyboard.focusedPane.mockReturnValue('messageList')
  const noFocusTarget = context()
  noFocusTarget.getLastMessageId.mockReturnValue(null)
  handleGlobalShortcut(event('F'), noFocusTarget)
  assert.equal(noFocusTarget.setFocusMode.mock.calls.length, 0)

  const deleteChecked = context()
  deleteChecked.messageListRef.hasCheckedMessages.mockReturnValue(true)
  handleGlobalShortcut(event('Delete', { shiftKey: true }), deleteChecked)
  assert.deepEqual(deleteChecked.messageListRef.requestDelete.mock.calls[0], [['checked'], true])
})
