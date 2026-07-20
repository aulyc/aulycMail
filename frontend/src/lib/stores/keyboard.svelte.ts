/**
 * Keyboard and pane focus management store
 *
 * Tracks which pane is focused and manages focus state for keyboard navigation.
 */

import {
  MAIN_KEYBOARD_REGION_ORDER,
  nextVisibleRegion,
  type MainKeyboardRegion,
} from '$lib/keyboard/regionNavigation'

export type FocusablePane = MainKeyboardRegion

// Pane cycle order for navigation
const PANE_ORDER = MAIN_KEYBOARD_REGION_ORDER

// Reactive state using Svelte 5 runes
let focusedPane = $state<FocusablePane>('messageList')
let keyboardScope = $state<'main' | 'settings'>('main')

/**
 * Get the currently focused pane
 */
export function getFocusedPane(): FocusablePane {
  return focusedPane
}

/**
 * Set the focused pane.
 */
export function setFocusedPane(pane: FocusablePane) {
  focusedPane = pane
}

export function setKeyboardScope(scope: 'main' | 'settings'): void {
  keyboardScope = scope
}

export function isMainKeyboardScope(): boolean {
  return keyboardScope === 'main'
}

function isVisibleRegionElement(element: HTMLElement): boolean {
  if (element.hidden || element.getAttribute('aria-hidden') === 'true') return false
  if (element.dataset.keyboardRegionVisible === 'false') return false
  const style = window.getComputedStyle(element)
  return style.display !== 'none' && style.visibility !== 'hidden' && element.getClientRects().length > 0
}

function getRegionElement(pane: FocusablePane): HTMLElement | null {
  const matches = document.querySelectorAll<HTMLElement>(`[data-keyboard-region="${pane}"]`)
  return [...matches].find(isVisibleRegionElement) ?? null
}

function getVisiblePanes(): FocusablePane[] {
  return PANE_ORDER.filter((pane) => getRegionElement(pane) !== null)
}

/** Focus the pane-level keyboard target without traversing its child controls. */
export function focusPane(pane: FocusablePane): boolean {
  const region = getRegionElement(pane)
  if (!region) return false
  setFocusedPane(pane)
  const target = region.matches('[data-keyboard-region-focus-target]')
    ? region
    : region.querySelector<HTMLElement>('[data-keyboard-region-focus-target]') ?? region
  target.focus({ preventScroll: true })
  return true
}

export function focusCurrentPane(): boolean {
  return focusPane(focusedPane)
}

/**
 * Focus the previous pane in the cycle: viewer -> messageList -> sidebar -> viewer
 */
export function focusPreviousPane() {
  const previous = nextVisibleRegion(focusedPane, -1, getVisiblePanes())
  focusPane(previous)
}

/**
 * Focus the next pane in the cycle: sidebar -> messageList -> viewer -> sidebar
 */
export function focusNextPane() {
  const next = nextVisibleRegion(focusedPane, 1, getVisiblePanes())
  focusPane(next)
}

/**
 * Check if the event target is an input field
 * Used to disable single-key shortcuts when typing
 */
export function isInputElement(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) {
    return false
  }

  return Boolean(target.closest([
    'input',
    'textarea',
    'select',
    '[contenteditable="true"]',
    '[role="textbox"]',
    '[role="searchbox"]',
    '[role="combobox"]',
    '[data-keyboard-input="true"]',
  ].join(',')))
}

/**
 * Pane navigation registry — lets the global keyboard handler dispatch
 * Alt+J/K (and similar pane-targeted shortcuts) to whichever component
 * currently owns a given slot. Mail's panes don't use this (they're wired
 * directly via concrete refs in App.svelte); secondary pane kit components
 * register on mount so Alt+J/K navigates the kit's SourceSidebar /
 * ListPane uniformly with how Alt+J/K navigates mail's folder list.
 */
export interface PaneNavTarget {
  navigateNext?: () => void
  navigatePrev?: () => void
  activate?: () => void
  /** Move keyboard focus to this pane's search input (Ctrl+S). */
  focusSearch?: () => void
}

const paneNavTargets: Partial<Record<FocusablePane, PaneNavTarget>> = {}

export function registerPaneNav(slot: FocusablePane, target: PaneNavTarget): () => void {
  paneNavTargets[slot] = target
  return () => {
    if (paneNavTargets[slot] === target) {
      delete paneNavTargets[slot]
    }
  }
}

export function getPaneNav(slot: FocusablePane): PaneNavTarget | undefined {
  return paneNavTargets[slot]
}

// Composer open state — used to suppress viewer shortcuts (Delete/Backspace)
// during the composer's mount→focus race, where a keystroke can fire before
// TipTap claims focus and would otherwise trash the focused message.
let composerOpen = $state(false)

export function setComposerOpen(open: boolean): void {
  composerOpen = open
}

export function isComposerOpen(): boolean {
  return composerOpen
}
