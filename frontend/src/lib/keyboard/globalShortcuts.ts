import { KEY } from './shortcuts'
import {
  focusNextPane,
  focusPreviousPane,
  focusCurrentPane,
  getFocusedPane,
  getPaneNav,
  isInputElement,
} from '$lib/stores/keyboard.svelte'
import { isDialogGuardActive } from '$lib/stores/dialogGuard'
import { dispatchPaneShortcut } from '$lib/stores/paneShortcuts.svelte'
import { RAIL_PANE_ORDER } from '$lib/rail/panes'
import { getActivePane, setActivePane } from '$lib/stores/uiState.svelte'
import {
  getLayoutMode,
  getResponsiveView,
  hideSidebar,
  hideViewer,
  isResponsive,
  showSidebar,
  showViewer,
} from '$lib/stores/layout.svelte'
import { getEnhancedKeyboardNavigation } from '$lib/stores/settings.svelte'
import { resolveAppKeyboardPolicy } from '$lib/keyboard/keyboardPolicy'

type FocusMode = 'off' | 'thread' | 'message'

export interface GlobalShortcutContext {
  leftAltHeld: boolean
  showSearchOverlay: boolean
  showComposer: boolean
  selectedThreadId: string | null
  selectedAccountId: string | null
  selectedFolderId: string | null
  focusMode: FocusMode
  sidebarRef: any
  messageListRef: any
  viewerRef: any
  setSearchOverlay(open: boolean): void
  setFocusMode(mode: FocusMode): void
  setFocusedMessageIdInFocus(messageId: string | null): void
  focusContextMenu(): void
  openRegionActionMenu(): void
  handleQuit(): void
  handleCompose(): void
  handleReply(mode: 'reply' | 'reply-all' | 'forward', messageId: string, imagesLoaded?: boolean): void
  getLastMessageId(): string | null
  handleBulkMarkRead(messageIds: string[]): void
  handleBulkMarkUnread(messageIds: string[]): void
  handleBulkArchive(messageIds: string[]): void
  handleBulkSpam(messageIds: string[]): void
  handleBulkToggleStar(messageIds: string[], shouldStar: boolean): void
  toggleThreadFocus(): void
}

export function handleGlobalShortcut(e: KeyboardEvent, ctx: GlobalShortcutContext): void {
  if (e.defaultPrevented) return
  if (e.isComposing || e.keyCode === 229) return

  const inInput = isInputElement(e.target)
  const focusedPane = getFocusedPane()
  const hasConversation = ctx.selectedThreadId !== null

  if (ctx.showSearchOverlay) return
  if (document.querySelector('[role="menu"]')) return
  if (isDialogGuardActive()) return

  // Command/Ctrl+F is the intentionally always-on baseline, including while
  // the composer is open.
  const keyboardPolicy = resolveAppKeyboardPolicy(e, getEnhancedKeyboardNavigation())
  if (keyboardPolicy === 'search') {
    e.preventDefault()
    ctx.setSearchOverlay(true)
    return
  }

  if (ctx.showComposer) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'r') {
      e.preventDefault()
      return
    }
    return
  }

  // "native" means the event is left untouched for WebKit/macOS when enhanced
  // navigation is disabled.
  if (keyboardPolicy === 'native') {
    return
  }

  if (!inInput && dispatchPaneShortcut(e)) {
    e.preventDefault()
    e.stopPropagation()
    return
  }

  if (e.key === 'Tab' && !inInput && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault()
    e.stopPropagation()
    if (e.shiftKey) focusPreviousPane()
    else focusNextPane()
    return
  }

  if (!inInput && e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault()
    ctx.setSearchOverlay(true)
    return
  }

  const isMailActive = () => getActivePane() === 'mail'
  if (e.ctrlKey || e.metaKey) {
    switch (e.key.toLowerCase()) {
      case 'q':
        e.preventDefault()
        ctx.handleQuit()
        return
      case 'f':
        e.preventDefault()
        ctx.setSearchOverlay(true)
        return
      case 'tab':
      case '`': {
        e.preventDefault()
        if (RAIL_PANE_ORDER.length <= 1) return
        const current = getActivePane()
        const idx = RAIL_PANE_ORDER.indexOf(current)
        const step = e.key === '`' ? -1 : 1
        const currentIndex = idx >= 0 ? idx : 0
        const next = (currentIndex + step + RAIL_PANE_ORDER.length) % RAIL_PANE_ORDER.length
        setActivePane(RAIL_PANE_ORDER[next])
        return
      }
    }

    if (!isMailActive()) return

    switch (e.key.toLowerCase()) {
      case 'n':
        e.preventDefault()
        ctx.handleCompose()
        return
      case 'r': {
        if (!hasConversation) return
        e.preventDefault()
        if (focusedPane === 'viewer' && ctx.viewerRef?.hasFocusedMessage()) {
          if (e.shiftKey) {
            ctx.viewerRef.replyAll()
            return
          }
          ctx.viewerRef.reply()
          return
        }
        const msgId = ctx.getLastMessageId()
        if (!msgId) return
        ctx.handleReply(e.shiftKey ? 'reply-all' : 'reply', msgId, ctx.viewerRef?.isImagesLoaded(msgId) || false)
        return
      }
      case 's':
        if (!e.shiftKey) return
        e.preventDefault()
        ctx.messageListRef?.toggleFolderSync()
        return
      case 'a':
        if (e.shiftKey) {
          e.preventDefault()
          ctx.sidebarRef?.toggleSync()
          return
        }
        if (inInput) {
          e.preventDefault()
          const el = e.target as HTMLInputElement | HTMLTextAreaElement
          if (typeof el.select === 'function') {
            el.select()
          } else {
            const range = document.createRange()
            range.selectNodeContents(el)
            const sel = window.getSelection()
            sel?.removeAllRanges()
            sel?.addRange(range)
          }
          return
        }
        e.preventDefault()
        if (focusedPane === 'viewer') {
          ctx.viewerRef?.selectAllText()
          return
        }
        ctx.messageListRef?.selectAll()
        return
      case 'l':
        e.preventDefault()
        if (e.shiftKey) {
          ctx.viewerRef?.openAlwaysLoadDropdown()
        } else {
          ctx.viewerRef?.loadImages()
        }
        return
      case 'u': {
        e.preventDefault()
        const messageIds = ctx.messageListRef?.hasCheckedMessages()
          ? ctx.messageListRef.getCheckedMessageIds()
          : ctx.messageListRef?.getSelectedMessageIds() ?? []
        if (messageIds.length === 0) return
        if (e.shiftKey) ctx.handleBulkMarkUnread(messageIds)
        else ctx.handleBulkMarkRead(messageIds)
        return
      }
      case 'k': {
        e.preventDefault()
        const messageIds = ctx.messageListRef?.hasCheckedMessages()
          ? ctx.messageListRef.getCheckedMessageIds()
          : ctx.messageListRef?.getSelectedMessageIds() ?? []
        if (messageIds.length > 0) ctx.handleBulkArchive(messageIds)
        return
      }
      case 'j': {
        e.preventDefault()
        const messageIds = ctx.messageListRef?.hasCheckedMessages()
          ? ctx.messageListRef.getCheckedMessageIds()
          : ctx.messageListRef?.getSelectedMessageIds() ?? []
        if (messageIds.length > 0) ctx.handleBulkSpam(messageIds)
        return
      }
    }
    return
  }

  if (e.key === 'ContextMenu' || (e.key === 'Alt' && e.code === 'AltRight')) {
    e.preventDefault()

    if (ctx.leftAltHeld || focusedPane === 'sidebar') {
      if (isMailActive() && ctx.sidebarRef?.hasFocusedSidebarAction()) return
      if (!ctx.selectedFolderId) return
      const folderEl = document.querySelector(
        `[data-sidebar-item="folder"][data-account-id="${ctx.selectedAccountId}"][data-folder-id="${ctx.selectedFolderId}"]`
      ) as HTMLElement | null
      if (!folderEl) return
      const rect = folderEl.getBoundingClientRect()
      folderEl.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: rect.right,
        clientY: rect.top + rect.height / 2,
      }))
      ctx.focusContextMenu()
      return
    }

    if (focusedPane === 'messageList') {
      ctx.messageListRef?.openContextMenu()
      ctx.focusContextMenu()
      return
    }
    if (focusedPane === 'viewer') {
      ctx.viewerRef?.openContextMenu()
      ctx.focusContextMenu()
    }
    return
  }

  if (e.altKey) {
    if (ctx.focusMode !== 'off') return

    if (KEY.PANE_FOCUS_PREV(e)) {
      e.preventDefault()
      if (isResponsive()) {
        const view = getResponsiveView()
        const mode = getLayoutMode()
        if (view === 'viewer') {
          hideViewer()
          return
        }
        if (mode === 'narrow' && view === 'default') {
          showSidebar()
          return
        }
      }
      focusPreviousPane()
      return
    }
    if (KEY.PANE_FOCUS_NEXT(e)) {
      e.preventDefault()
      if (isResponsive()) {
        const view = getResponsiveView()
        const mode = getLayoutMode()
        if (mode === 'narrow' && view === 'sidebar') {
          hideSidebar()
          return
        }
        if (view === 'default' && ctx.selectedThreadId) {
          showViewer()
          return
        }
      }
      focusNextPane()
      return
    }

    if (KEY.SIDEBAR_PREV(e)) {
      e.preventDefault()
      if (isMailActive()) {
        ctx.sidebarRef?.selectPreviousFolder()
        return
      }
      getPaneNav('sidebar')?.navigatePrev?.()
      return
    }
    if (KEY.SIDEBAR_NEXT(e)) {
      e.preventDefault()
      if (isMailActive()) {
        ctx.sidebarRef?.selectNextFolder()
        return
      }
      getPaneNav('sidebar')?.navigateNext?.()
      return
    }

    if (e.key === 'Enter') {
      if (ctx.sidebarRef?.hasFocusedAccount()) {
        e.preventDefault()
        ctx.sidebarRef.toggleFocusedAccount()
      } else if (ctx.sidebarRef?.hasFocusedFolderGroup()) {
        e.preventDefault()
        ctx.sidebarRef.toggleFocusedFolderGroup()
      } else if (ctx.sidebarRef?.hasSelectedFolderWithChildren()) {
        e.preventDefault()
        ctx.sidebarRef.toggleSelectedFolderCollapse()
      }
    }
    return
  }

  if (inInput) {
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      ;(e.target as HTMLElement | null)?.blur?.()
      requestAnimationFrame(() => focusCurrentPane())
    }
    return
  }

  if (e.key === 'F10' && e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault()
    e.stopPropagation()
    ctx.openRegionActionMenu()
    return
  }

  if (e.key === 'Escape') {
    if (ctx.focusMode !== 'off') {
      ctx.setFocusMode('off')
      ctx.setFocusedMessageIdInFocus(null)
      return
    }
    if (isResponsive() && getResponsiveView() === 'viewer') {
      hideViewer()
      return
    }
    if (isResponsive() && getResponsiveView() === 'sidebar') {
      hideSidebar()
      return
    }
    if (ctx.messageListRef?.hasCheckedMessages()) {
      ctx.messageListRef.clearChecked()
    }
    return
  }

  // Pane-local handlers own non-mail navigation and actions. Nothing below
  // this point may operate the always-mounted, currently hidden mail tree.
  if (!isMailActive()) return

  if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedSidebarAction()) {
    if (e.key === 'ArrowLeft' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      e.stopPropagation()
      ctx.sidebarRef.moveFocusedSidebarAction(-1)
      return
    }
    if (e.key === 'ArrowRight' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      e.preventDefault()
      e.stopPropagation()
      ctx.sidebarRef.moveFocusedSidebarAction(1)
      return
    }
  }

  if (KEY.LIST_PREV(e) || KEY.LIST_PREV_CHECK(e)) {
    e.preventDefault()
    if (focusedPane === 'sidebar') {
      ctx.sidebarRef?.selectPreviousFolder()
    } else if (focusedPane === 'messageList') {
      if (e.shiftKey) ctx.messageListRef?.selectPreviousWithCheck()
      else ctx.messageListRef?.selectPrevious()
    } else if (focusedPane === 'viewer') {
      ctx.viewerRef?.scrollUp()
    }
    return
  }
  if (KEY.LIST_NEXT(e) || KEY.LIST_NEXT_CHECK(e)) {
    e.preventDefault()
    if (focusedPane === 'sidebar') {
      ctx.sidebarRef?.selectNextFolder()
    } else if (focusedPane === 'messageList') {
      if (e.shiftKey) ctx.messageListRef?.selectNextWithCheck()
      else ctx.messageListRef?.selectNext()
    } else if (focusedPane === 'viewer') {
      ctx.viewerRef?.scrollDown()
    }
    return
  }

  switch (e.key) {
    case 'Enter':
      if (document.activeElement?.tagName === 'BUTTON') {
        const btn = document.activeElement as HTMLElement
        const inMessageList = btn.closest('[data-pane="messageList"]')
        const inViewer = btn.closest('[data-pane="viewer"]')
        if ((focusedPane === 'messageList' && inMessageList) ||
            (focusedPane === 'viewer' && inViewer)) {
          return
        }
        e.preventDefault()
      }
      if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedSidebarAction()) {
        e.preventDefault()
        ctx.sidebarRef.activateFocusedSidebarAction()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedAccount()) {
        e.preventDefault()
        ctx.sidebarRef.toggleFocusedAccount()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedFolderGroup()) {
        e.preventDefault()
        ctx.sidebarRef.toggleFocusedFolderGroup()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasSelectedFolderWithChildren()) {
        e.preventDefault()
        ctx.sidebarRef.toggleSelectedFolderCollapse()
      } else if (focusedPane === 'messageList') {
        e.preventDefault()
        ctx.messageListRef?.openSelected()
      }
      return
    case ' ':
      if (document.activeElement?.tagName === 'BUTTON') {
        const btn = document.activeElement as HTMLElement
        const inMessageList = btn.closest('[data-pane="messageList"]')
        const inViewer = btn.closest('[data-pane="viewer"]')
        if ((focusedPane === 'messageList' && inMessageList) ||
            (focusedPane === 'viewer' && inViewer)) {
          return
        }
        e.preventDefault()
      }
      e.preventDefault()
      if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedSidebarAction()) {
        ctx.sidebarRef.activateFocusedSidebarAction()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedAccount()) {
        ctx.sidebarRef.toggleFocusedAccount()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasFocusedFolderGroup()) {
        ctx.sidebarRef.toggleFocusedFolderGroup()
      } else if (focusedPane === 'sidebar' && ctx.sidebarRef?.hasSelectedFolderWithChildren()) {
        ctx.sidebarRef.toggleSelectedFolderCollapse()
      } else if (focusedPane === 'messageList') {
        ctx.messageListRef?.toggleCheck()
      }
      return
  }

  switch (e.key) {
    case 'v':
      if (focusedPane === 'messageList') {
        e.preventDefault()
        ctx.messageListRef?.openSelected()
      }
      return
    case 's':
      if (ctx.messageListRef?.hasCheckedMessages()) {
        ctx.handleBulkToggleStar(ctx.messageListRef.getCheckedMessageIds(), ctx.messageListRef.getCheckedHasUnstarred())
      } else {
        const focusedIds = ctx.messageListRef?.getSelectedMessageIds() ?? []
        if (focusedIds.length > 0) {
          const isStarred = ctx.messageListRef?.isSelectedStarred() ?? false
          ctx.handleBulkToggleStar(focusedIds, !isStarred)
        }
      }
      return
    case 'f':
      if (!hasConversation) return
      e.preventDefault()
      ctx.toggleThreadFocus()
      return
    case 'F': {
      if (!hasConversation) return
      e.preventDefault()
      if (ctx.focusMode === 'message') {
        ctx.setFocusMode('off')
        ctx.setFocusedMessageIdInFocus(null)
        return
      }
      const targetId = (focusedPane === 'viewer' && ctx.viewerRef?.hasFocusedMessage())
        ? ctx.viewerRef.getFocusedMessageId()
        : ctx.getLastMessageId()
      if (!targetId) return
      ctx.setFocusMode('message')
      ctx.setFocusedMessageIdInFocus(targetId)
      return
    }
    case 'd':
    case 'Backspace':
    case 'Delete': {
      if (focusedPane === 'viewer' && ctx.viewerRef?.hasFocusedMessage()) {
        if (e.shiftKey) {
          ctx.viewerRef.deletePermanently()
          return
        }
        ctx.viewerRef.trash()
        return
      }
      if (ctx.messageListRef?.hasCheckedMessages()) {
        ctx.messageListRef.requestDelete(ctx.messageListRef.getCheckedMessageIds(), e.shiftKey)
        return
      }
      const focusedMessageIds = ctx.messageListRef?.getSelectedMessageIds() ?? []
      if (focusedMessageIds.length > 0) {
        ctx.messageListRef?.requestDelete(focusedMessageIds, e.shiftKey)
      }
      return
    }
  }
}
