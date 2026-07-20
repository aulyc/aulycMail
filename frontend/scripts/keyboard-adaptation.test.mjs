import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import {
  MAIN_KEYBOARD_REGION_ORDER,
  nextRovingIndex,
  nextVisibleRegion,
} from '../src/lib/keyboard/regionNavigation.ts'
import { resolveRequiredSelectionIndex } from '../src/lib/components/list/requiredSelection.ts'

const modalFramePath = new URL('../src/lib/components/ui/ModalFrame.svelte', import.meta.url)
const messageListPath = new URL('../src/lib/components/list/MessageList.svelte', import.meta.url)
const folderPickerPath = new URL('../src/lib/components/common/FolderPickerDialog.svelte', import.meta.url)
const globalShortcutsPath = new URL('../src/lib/keyboard/globalShortcuts.ts', import.meta.url)
const keyboardStorePath = new URL('../src/lib/stores/keyboard.svelte.ts', import.meta.url)
const appPath = new URL('../src/App.svelte', import.meta.url)
const appStylesPath = new URL('../src/app.css', import.meta.url)
const activityRailPath = new URL('../src/lib/components/rail/ActivityRail.svelte', import.meta.url)
const listRowPath = new URL('../src/lib/components/kit/ListRow.svelte', import.meta.url)
const sourceSidebarPath = new URL('../src/lib/components/kit/SourceSidebar.svelte', import.meta.url)
const folderTreeItemPath = new URL('../src/lib/components/sidebar/FolderTreeItem.svelte', import.meta.url)
const accountSectionPath = new URL('../src/lib/components/sidebar/AccountSection.svelte', import.meta.url)
const contactListPath = new URL('../src/lib/contacts/components/ContactList.svelte', import.meta.url)
const contactStorePath = new URL('../src/lib/contacts/stores/contactsView.svelte.ts', import.meta.url)
const viewerPath = new URL('../src/lib/components/viewer/ConversationViewer.svelte', import.meta.url)
const settingsPath = new URL('../src/lib/components/settings/SettingsDialog.svelte', import.meta.url)
const backupViewerPath = new URL('../src/lib/components/backup/BackupViewerDialog.svelte', import.meta.url)
const globalSearchPath = new URL('../src/lib/components/SearchOverlay.svelte', import.meta.url)

test('custom modal frames close on Escape, contain Tab focus, and restore prior focus', async () => {
  const modalFrame = await readFile(modalFramePath, 'utf8')

  assert.match(modalFrame, /bind:this=\{panelEl\}/)
  assert.match(modalFrame, /onkeydown=\{handlePanelKeydown\}/)
  assert.match(modalFrame, /event\.key === 'Escape'[\s\S]*onClose\?\.\(\)/)
  assert.match(modalFrame, /event\.key !== 'Tab'/)
  assert.match(modalFrame, /previouslyFocused[\s\S]*\.focus\(\{ preventScroll: true \}\)/)
})

test('mail-list search Escape closes search and returns focus to the list', async () => {
  const messageList = await readFile(messageListPath, 'utf8')

  assert.match(
    messageList,
    /function handleSearchKeydown[\s\S]*event\.key === 'Escape'[\s\S]*clearSearch\(\)[\s\S]*listContainerRef\?\.focus\(\)/,
  )
})

test('folder-picker arrows and Enter only run from its search or folder list', async () => {
  const folderPicker = await readFile(folderPickerPath, 'utf8')

  assert.match(folderPicker, /function eventTargetsFolderNavigation/)
  assert.match(folderPicker, /target === searchInput/)
  assert.match(folderPicker, /listEl\?\.contains\(target\)/)
  assert.match(folderPicker, /if \(!active \|\| !eventTargetsFolderNavigation\(e\)\) return/)
})

test('four visible regions cycle forward, backward, and across hidden regions', () => {
  assert.deepEqual(MAIN_KEYBOARD_REGION_ORDER, ['featureNav', 'sidebar', 'messageList', 'viewer'])
  assert.equal(nextVisibleRegion('featureNav', 1, MAIN_KEYBOARD_REGION_ORDER), 'sidebar')
  assert.equal(nextVisibleRegion('viewer', 1, MAIN_KEYBOARD_REGION_ORDER), 'featureNav')
  assert.equal(nextVisibleRegion('featureNav', -1, MAIN_KEYBOARD_REGION_ORDER), 'viewer')
  assert.equal(nextVisibleRegion('viewer', -1, ['featureNav', 'messageList', 'viewer']), 'messageList')
  assert.equal(nextVisibleRegion('featureNav', 1, ['featureNav', 'viewer']), 'viewer')
})

test('main Tab routing uses one focused region and preserves input Tab', async () => {
  const [globalShortcuts, keyboardStore, app, activityRail] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(keyboardStorePath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /e\.key === 'Tab' && !inInput[\s\S]*e\.shiftKey\) focusPreviousPane\(\)[\s\S]*focusNextPane\(\)/)
  assert.match(keyboardStore, /let focusedPane = \$state<FocusablePane>/)
  assert.match(keyboardStore, /dataset\.keyboardRegionVisible === 'false'/)
  assert.match(keyboardStore, /nextVisibleRegion\(focusedPane, -1, getVisiblePanes\(\)\)/)
  assert.match(app, /data-keyboard-region="sidebar"[\s\S]*data-keyboard-region="messageList"[\s\S]*data-keyboard-region="viewer"/)
  assert.match(activityRail, /data-keyboard-region="featureNav"/)
})

test('region indicator stays top-only while selections use component backgrounds', async () => {
  const [styles, app, activityRail, listRow] = await Promise.all([
    readFile(appStylesPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
    readFile(listRowPath, 'utf8'),
  ])

  assert.match(styles, /\.keyboard-region \{[\s\S]*border-top: 3px solid transparent/)
  assert.match(styles, /\.keyboard-region\[data-region-active='true'\][\s\S]*#f97316/)
  assert.doesNotMatch(styles, /keyboard-selected-item|inset 0 0 0 2px #cbd5e1/)
  assert.match(app, /isMainKeyboardScope\(\) && getActivePane\(\) === 'mail' && getFocusedPane\(\) === 'messageList'/)
  assert.match(activityRail, /selectedFeature === 'settings' \? 'bg-accent\/40 text-primary'/)
  assert.match(listRow, /selected[\s\S]*bg-primary\/20/)
  assert.doesNotMatch(activityRail + listRow, /keyboard-selected-item/)
})

test('mouse interaction claims each main region', async () => {
  const [app, activityRail, sourceSidebar, contactList] = await Promise.all([
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
    readFile(sourceSidebarPath, 'utf8'),
    readFile(contactListPath, 'utf8'),
  ])

  assert.match(app, /onmousedown=\{\(event\) => handlePaneMouseDown\('sidebar', event\)\}/)
  assert.match(app, /onmousedown=\{\(\) => handlePaneClick\('messageList'\)\}/)
  assert.match(app, /onmousedown=\{\(\) => handlePaneClick\('viewer'\)\}/)
  assert.match(activityRail, /onmousedown=\{claimRegion\}/)
  assert.match(sourceSidebar, /onmousedown=\{handleMouseDown\}/)
  assert.match(contactList, /function claimListRegion[\s\S]*setFocusedPane\('messageList'\)/)
  assert.match(contactList, /onmousedown=\{claimListRegion\}/)
  assert.match(contactList, /data-keyboard-region-visible=\{!isResponsive\(\) \|\| getResponsiveView\(\) === 'default'\}/)
})

test('mail sidebar rows leave DOM focus on the keyboard region', async () => {
  const [app, folderTreeItem, accountSection] = await Promise.all([
    readFile(appPath, 'utf8'),
    readFile(folderTreeItemPath, 'utf8'),
    readFile(accountSectionPath, 'utf8'),
  ])

  assert.match(
    app,
    /function handlePaneMouseDown[\s\S]*pane !== 'sidebar'[\s\S]*closest\('\[data-sidebar-item\]'\)[\s\S]*event\.preventDefault\(\)[\s\S]*region\?\.focus\(\{ preventScroll: true \}\)/,
  )
  assert.match(folderTreeItem, /tabindex="-1"[\s\S]*data-sidebar-item="folder"/)
  assert.match(folderTreeItem, /transition-colors focus:outline-none/)
  assert.match(accountSection, /tabindex="-1"[\s\S]*data-sidebar-item="account-header"/)
  assert.match(accountSection, /transition-colors focus:outline-none/)
  assert.doesNotMatch(folderTreeItem + accountSection, /keyboard-selected-item/)
})

test('input Escape returns to its region while search scope Tab remains unchanged', async () => {
  const [globalShortcuts, globalSearch, backupViewer, contactList] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(globalSearchPath, 'utf8'),
    readFile(backupViewerPath, 'utf8'),
    readFile(contactListPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /if \(inInput\)[\s\S]*e\.key === 'Escape'[\s\S]*focusCurrentPane\(\)/)
  assert.match(globalSearch, /e\.key === 'Tab'[\s\S]*moveScope\(e\.shiftKey \? -1 : 1\)/)
  assert.match(backupViewer, /event\.key === 'Tab'[\s\S]*moveSearchScope\(event\.shiftKey \? -1 : 1\)/)
  assert.match(contactList, /e\.key === 'Escape'[\s\S]*focusPane\('messageList'\)/)
})

test('ConversationViewer no longer owns Tab message navigation', async () => {
  const viewer = await readFile(viewerPath, 'utf8')

  assert.doesNotMatch(viewer, /e\.key === 'Tab'/)
  assert.match(viewer, /Region-level Tab[\s\S]*global shortcut dispatcher/)
  assert.match(viewer, /isInputElement\(e\.target\) \|\| isInputElement\(document\.activeElement\)/)
})

test('backup message list uses roving tabindex and complete directional navigation', async () => {
  const backupViewer = await readFile(backupViewerPath, 'utf8')

  assert.equal(nextRovingIndex('ArrowDown', 0, 3), 1)
  assert.equal(nextRovingIndex('ArrowUp', 0, 3), 0)
  assert.equal(nextRovingIndex('Home', 2, 3), 0)
  assert.equal(nextRovingIndex('End', 0, 3), 2)
  assert.match(backupViewer, /tabindex=\{selectedMessageKey === message\.key \? 0 : -1\}/)
  assert.match(backupViewer, /\['ArrowUp', 'ArrowDown', 'Home', 'End'\]/)
  assert.match(backupViewer, /scrollIntoView\(\{ block, behavior: 'smooth' \}\)/)
  assert.match(backupViewer, /event\.key === 'Enter'[\s\S]*selectMessage\(selectedMessageKey\)/)
  assert.match(backupViewer, /handleMessageListFocus[\s\S]*visibleMessages\[0\][\s\S]*selectMessage\(first\.key\)[\s\S]*focusMessageRow\(first\.key\)/)
  assert.match(backupViewer, /void selectMessage\(next\.key\)[\s\S]*scrollMessageIntoView\(next\.key\)[\s\S]*focusMessageRow\(next\.key\)/)
})

test('ordinary Escape cannot clear the selected mail detail', async () => {
  const [globalShortcuts, app] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(appPath, 'utf8'),
  ])

  assert.doesNotMatch(globalShortcuts, /clearConversation/)
  assert.match(globalShortcuts, /e\.key === 'Escape'[\s\S]*hasCheckedMessages\(\)[\s\S]*clearChecked\(\)/)
  assert.doesNotMatch(app, /clearConversation:/)
})

test('mail selection remains required across refresh, folder switch, and deletion', async () => {
  const messageList = await readFile(messageListPath, 'utf8')

  assert.equal(resolveRequiredSelectionIndex([], 'missing'), -1)
  assert.equal(resolveRequiredSelectionIndex(['a', 'b'], 'b'), 1)
  assert.equal(resolveRequiredSelectionIndex(['a', 'b'], 'missing'), 0)
  assert.equal(resolveRequiredSelectionIndex(['a', 'c'], 'deleted-b', 1), 1)
  assert.equal(resolveRequiredSelectionIndex(['a'], 'deleted-b', 1), 0)
  assert.match(messageList, /resolveRequiredSelectionIndex\([\s\S]*conversations\.map/)
  assert.match(messageList, /loadConversations\(totalLoaded, autoSelectNext \? currentIndex : 0\)/)
  assert.match(messageList, /folderChanged \|\| selectionChanged[\s\S]*emitConversationSelection/)
  assert.match(messageList, /conversations\.length === 0[\s\S]*onEmptyFolder\?\.\(\)/)
})

test('settings dialog cycles two regions, traps focus, and restores the settings entry', async () => {
  const [settings, app, activityRail] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(activityRailPath, 'utf8'),
  ])

  assert.match(settings, /settingsRegion = \$state<'navigation' \| 'content'>/)
  assert.match(settings, /event\.key === 'Tab' && !inputState[\s\S]*settingsRegion === 'navigation' \? 'content' : 'navigation'/)
  assert.match(settings, /event\.key === 'Escape' && inputState[\s\S]*focusSettingsRegion\('content'\)/)
  assert.match(settings, /setKeyboardScope\('settings'\)[\s\S]*setKeyboardScope\('main'\)/)
  assert.match(settings, /onOpenAutoFocus=\{handleOpenAutoFocus\}/)
  assert.match(app, /requestAnimationFrame\(\(\) => activityRailRef\?\.focusSettings\(\)\)/)
  assert.match(activityRail, /export function focusSettings\(\)/)
})

test('IME composition and dialog priority stay ahead of region shortcuts', async () => {
  const [globalShortcuts, settings, backupViewer] = await Promise.all([
    readFile(globalShortcutsPath, 'utf8'),
    readFile(settingsPath, 'utf8'),
    readFile(backupViewerPath, 'utf8'),
  ])

  assert.match(globalShortcuts, /e\.isComposing \|\| e\.keyCode === 229/)
  assert.ok(globalShortcuts.indexOf('if (ctx.showSearchOverlay) return') < globalShortcuts.indexOf("e.key === 'Tab'"))
  assert.ok(globalShortcuts.indexOf('if (isDialogGuardActive()) return') < globalShortcuts.indexOf("e.key === 'Tab'"))
  assert.match(settings, /event\.isComposing \|\| event\.keyCode === 229 \|\| showRestartDialog/)
  assert.match(backupViewer, /event\.isComposing \|\| event\.keyCode === 229/)
})

test('contact arrows update detail without forcing responsive activation', async () => {
  const [contactList, contactStore] = await Promise.all([
    readFile(contactListPath, 'utf8'),
    readFile(contactStorePath, 'utf8'),
  ])

  assert.match(contactList, /onSelect=\{\(id\) => focusContact\(id\)\}/)
  assert.match(contactList, /onActivate=\{\(id\) => activateContact\(id\)\}/)
  assert.match(contactStore, /focusContact[\s\S]*selectContact\(id, false\)/)
  assert.match(contactStore, /activateContact[\s\S]*selectContact\(id, true\)/)
})
